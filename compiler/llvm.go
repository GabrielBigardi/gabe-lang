package compiler

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type llvmModule struct {
	program  *Program
	strings  []string
	interned map[string]int
}
type llvmValue struct {
	ty      valueType
	operand string
}
type loopTargets struct{ end, step string }
type functionEmitter struct {
	module          *llvmModule
	function        *function
	output          strings.Builder
	register, block int
	currentBlock    string
	terminated      bool
	loops           []loopTargets
}

func emitLLVM(program *Program, triple string) string {
	module := &llvmModule{program: program, interned: make(map[string]int)}
	var functions strings.Builder
	for _, fn := range program.functions {
		if fn.body != nil {
			emitter := &functionEmitter{module: module, function: fn, currentBlock: "entry"}
			functions.WriteString(emitter.emit())
		} else {
			parameters := make([]string, len(fn.parameters))
			for i, id := range fn.parameters {
				parameters[i] = fn.variables[id].llvm()
			}
			fmt.Fprintf(&functions, "declare %s @%s(%s)\n\n", fn.returnType.llvm(), fn.symbol, strings.Join(parameters, ", "))
		}
	}
	entry := program.functions[program.entry]
	if entry.returnType == voidType {
		fmt.Fprintf(&functions, "define i32 @main() {\nentry:\n  call void @%s()\n  ret i32 0\n}\n", entry.symbol)
	} else {
		fmt.Fprintf(&functions, "define i32 @main() {\nentry:\n  %%result = call i32 @%s()\n  ret i32 %%result\n}\n", entry.symbol)
	}
	var output strings.Builder
	fmt.Fprintf(&output, "; Gabe LLVM IR (opaque pointers; LLVM 17+)\ntarget triple = \"%s\"\n\n", triple)
	for id, value := range module.strings {
		fmt.Fprintf(&output, "@gb_string_%d = private unnamed_addr constant [%d x i8] c\"", id, len(value)+1)
		for i := 0; i < len(value); i++ {
			fmt.Fprintf(&output, "\\%02X", value[i])
		}
		output.WriteString("\\00\"\n")
	}
	output.WriteString("\ndeclare i32 @printf(ptr, ...)\ndeclare void @llvm.trap() cold noreturn nounwind\n\n")
	output.WriteString(functions.String())
	return output.String()
}
func (m *llvmModule) stringConstant(value string) string {
	id, found := m.interned[value]
	if !found {
		id = len(m.strings)
		m.strings = append(m.strings, value)
		m.interned[value] = id
	}
	return fmt.Sprintf("getelementptr inbounds ([%d x i8], ptr @gb_string_%d, i64 0, i64 0)", len(value)+1, id)
}
func (f *functionEmitter) emit() string {
	parameters := make([]string, len(f.function.parameters))
	for i, id := range f.function.parameters {
		parameters[i] = fmt.Sprintf("%s %%arg%d", f.function.variables[id].llvm(), i)
	}
	fmt.Fprintf(&f.output, "; %s\ndefine %s @%s(%s) {\nentry:\n", f.function.name, f.function.returnType.llvm(), f.function.symbol, strings.Join(parameters, ", "))
	// All slots live in entry, including loop locals, so optimizers can promote them.
	for id, ty := range f.function.variables {
		f.instruction(fmt.Sprintf("%%v%d = alloca %s", id, ty.llvm()))
	}
	for i, id := range f.function.parameters {
		f.instruction(fmt.Sprintf("store %s %%arg%d, ptr %%v%d", f.function.variables[id].llvm(), i, id))
	}
	f.statement(f.function.body)
	if !f.terminated {
		if f.function.returnType != voidType {
			panic("analysis guarantees returns")
		}
		f.instruction("ret void")
	}
	f.output.WriteString("}\n\n")
	return f.output.String()
}
func (f *functionEmitter) instruction(instruction string) {
	fmt.Fprintf(&f.output, "  %s\n", instruction)
}
func (f *functionEmitter) result(instruction string) string {
	result := fmt.Sprintf("%%r%d", f.register)
	f.register++
	f.instruction(result + " = " + instruction)
	return result
}
func (f *functionEmitter) newBlock() string {
	label := fmt.Sprintf("block_%d", f.block)
	f.block++
	return label
}
func (f *functionEmitter) label(label string) {
	fmt.Fprintf(&f.output, "%s:\n", label)
	f.currentBlock, f.terminated = label, false
}
func (f *functionEmitter) branch(target string) {
	if !f.terminated {
		f.instruction("br label %" + target)
		f.terminated = true
	}
}
func (f *functionEmitter) conditional(value, yes, no string) {
	f.instruction(fmt.Sprintf("br i1 %s, label %%%s, label %%%s", value, yes, no))
	f.terminated = true
}
func (f *functionEmitter) statement(s *stmt) {
	if f.terminated {
		return
	}
	switch s.kind {
	case stmtBlock:
		for _, child := range s.statements {
			f.statement(child)
		}
	case stmtLet:
		value := f.expression(s.value)
		f.instruction(fmt.Sprintf("store %s %s, ptr %%v%d", value.ty.llvm(), value.operand, s.variable))
	case stmtExpr:
		f.expression(s.value)
	case stmtReturn:
		if s.value != nil {
			value := f.expression(s.value)
			f.instruction(fmt.Sprintf("ret %s %s", value.ty.llvm(), value.operand))
		} else {
			f.instruction("ret void")
		}
		f.terminated = true
	case stmtIf:
		condition := f.expression(s.condition)
		yes, no, end := f.newBlock(), f.newBlock(), f.newBlock()
		falseTarget := end
		if s.elseBody != nil {
			falseTarget = no
		}
		f.conditional(condition.operand, yes, falseTarget)
		f.label(yes)
		f.statement(s.body)
		yesEnds := f.terminated
		f.branch(end)
		noEnds := false
		if s.elseBody != nil {
			f.label(no)
			f.statement(s.elseBody)
			noEnds = f.terminated
			f.branch(end)
		}
		if yesEnds && noEnds {
			f.terminated = true
		} else {
			f.label(end)
		}
	case stmtWhile:
		test, run, end := f.newBlock(), f.newBlock(), f.newBlock()
		f.branch(test)
		f.label(test)
		condition := f.expression(s.condition)
		f.conditional(condition.operand, run, end)
		f.label(run)
		f.loops = append(f.loops, loopTargets{end, test})
		f.statement(s.body)
		f.loops = f.loops[:len(f.loops)-1]
		f.branch(test)
		f.label(end)
	case stmtFor:
		if s.initializer != nil {
			f.statement(s.initializer)
		}
		test, run, increment, end := f.newBlock(), f.newBlock(), f.newBlock(), f.newBlock()
		f.branch(test)
		f.label(test)
		if s.condition != nil {
			condition := f.expression(s.condition)
			f.conditional(condition.operand, run, end)
		} else {
			f.branch(run)
		}
		f.label(run)
		f.loops = append(f.loops, loopTargets{end, increment})
		f.statement(s.body)
		f.loops = f.loops[:len(f.loops)-1]
		f.branch(increment)
		f.label(increment)
		if s.step != nil {
			f.expression(s.step)
		}
		f.branch(test)
		f.label(end)
	case stmtBreak, stmtContinue:
		loop := f.loops[len(f.loops)-1]
		if s.kind == stmtBreak {
			f.branch(loop.end)
		} else {
			f.branch(loop.step)
		}
	case stmtEmpty:
	default:
		panic("invalid analyzed statement")
	}
}
func (f *functionEmitter) address(target *lvalue) string {
	if target.pointer != nil {
		return f.expression(target.pointer).operand
	}
	return fmt.Sprintf("%%v%d", target.variable)
}
func (f *functionEmitter) load(target *lvalue, address string) llvmValue {
	return llvmValue{target.ty, f.result(fmt.Sprintf("load %s, ptr %s", target.ty.llvm(), address))}
}
func doubleConstant(value float64) string { return fmt.Sprintf("0x%016X", math.Float64bits(value)) }

func (f *functionEmitter) expression(e *expr) llvmValue {
	operand := ""
	switch e.kind {
	case exprLiteral:
		switch e.literal.ty {
		case intType, longType:
			operand = strconv.FormatInt(e.literal.integer, 10)
		case doubleType:
			operand = doubleConstant(e.literal.double)
		case boolType:
			operand = strconv.FormatBool(e.literal.boolean)
		case stringType:
			operand = f.module.stringConstant(e.literal.text)
		default:
			panic("invalid analyzed literal")
		}
	case exprLoad:
		address := f.address(e.target)
		operand = f.load(e.target, address).operand
	case exprAddress:
		operand = f.address(e.target)
	case exprCall:
		symbol := f.module.program.functions[e.function].symbol
		arguments := make([]string, len(e.arguments))
		for i, argument := range e.arguments {
			value := f.expression(argument)
			arguments[i] = value.ty.llvm() + " " + value.operand
		}
		call := fmt.Sprintf("call %s @%s(%s)", e.ty.llvm(), symbol, strings.Join(arguments, ", "))
		if e.ty == voidType {
			f.instruction(call)
		} else {
			operand = f.result(call)
		}
	case exprPrint:
		value := f.expression(e.value)
		format := ""
		switch value.ty {
		case intType:
			format = "%d"
		case longType:
			format = "%lld"
		case doubleType:
			format = "%.17g"
		case stringType:
			format = "%s"
		case boolType:
			yes, no := f.module.stringConstant("True"), f.module.stringConstant("False")
			value.operand = f.result(fmt.Sprintf("select i1 %s, ptr %s, ptr %s", value.operand, yes, no))
			value.ty, format = stringType, "%s"
		default:
			panic("analysis checks output types")
		}
		if e.newline {
			format += "\n"
		}
		format = f.module.stringConstant(format)
		f.instruction(fmt.Sprintf("call i32 (ptr, ...) @printf(ptr %s, %s %s)", format, value.ty.llvm(), value.operand))
	case exprUnary:
		value := f.expression(e.value)
		switch e.unary {
		case positive:
			operand = value.operand
		case negate:
			if value.ty == doubleType {
				operand = f.result("fneg double " + value.operand)
			} else {
				operand = f.result(fmt.Sprintf("sub %s 0, %s", value.ty.llvm(), value.operand))
			}
		case not:
			operand = f.result("xor i1 " + value.operand + ", true")
		default:
			panic("address/dereference are lowered separately")
		}
	case exprBinary:
		left := f.expression(e.left)
		if e.op == and || e.op == or {
			origin := f.currentBlock
			rhs, end := f.newBlock(), f.newBlock()
			constant := "true"
			if e.op == and {
				f.conditional(left.operand, rhs, end)
				constant = "false"
			} else {
				f.conditional(left.operand, end, rhs)
			}
			f.label(rhs)
			right := f.expression(e.right)
			// Division guards and nested boolean expressions can change the RHS block.
			rhsEnd := f.currentBlock
			f.branch(end)
			f.label(end)
			operand = f.result(fmt.Sprintf("phi i1 [ %s, %%%s ], [ %s, %%%s ]", constant, origin, right.operand, rhsEnd))
		} else {
			right := f.expression(e.right)
			operand = f.binary(e.op, left, right)
		}
	case exprCast:
		value := f.expression(e.value)
		instruction := "sitofp"
		if e.ty == longType {
			instruction = "sext"
		}
		operand = f.result(fmt.Sprintf("%s %s %s to %s", instruction, value.ty.llvm(), value.operand, e.ty.llvm()))
	case exprAssign:
		address := f.address(e.target)
		var previous llvmValue
		if e.op != binaryNone {
			previous = f.load(e.target, address)
		}
		value := f.expression(e.value)
		operand = value.operand
		if e.op != binaryNone {
			operand = f.binary(e.op, previous, value)
		}
		f.instruction(fmt.Sprintf("store %s %s, ptr %s", e.target.ty.llvm(), operand, address))
	case exprUpdate:
		address := f.address(e.target)
		old := f.load(e.target, address)
		step := strconv.Itoa(e.delta)
		if e.target.ty == doubleType {
			step = doubleConstant(float64(e.delta))
		}
		updated := f.binary(add, old, llvmValue{e.target.ty, step})
		f.instruction(fmt.Sprintf("store %s %s, ptr %s", e.target.ty.llvm(), updated, address))
		operand = old.operand
		if e.prefix {
			operand = updated
		}
	default:
		panic("invalid analyzed expression")
	}
	return llvmValue{e.ty, operand}
}
func (f *functionEmitter) binary(op binaryOp, left, right llvmValue) string {
	if (op == divide || op == remainder) && left.ty.integer() {
		zero := f.result(fmt.Sprintf("icmp eq %s %s, 0", right.ty.llvm(), right.operand))
		minimum := int64(math.MinInt64)
		if left.ty == intType {
			minimum = math.MinInt32
		}
		min := f.result(fmt.Sprintf("icmp eq %s %s, %d", left.ty.llvm(), left.operand, minimum))
		negativeOne := f.result(fmt.Sprintf("icmp eq %s %s, -1", right.ty.llvm(), right.operand))
		overflow := f.result("and i1 " + min + ", " + negativeOne)
		invalid := f.result("or i1 " + zero + ", " + overflow)
		trap, valid := f.newBlock(), f.newBlock()
		f.conditional(invalid, trap, valid)
		f.label(trap)
		f.instruction("call void @llvm.trap()")
		f.instruction("unreachable")
		f.terminated = true
		f.label(valid)
	}
	var instruction string
	if left.ty == doubleType {
		switch op {
		case add:
			instruction = "fadd"
		case subtract:
			instruction = "fsub"
		case multiply:
			instruction = "fmul"
		case divide:
			instruction = "fdiv"
		case remainder:
			instruction = "frem"
		case equal:
			instruction = "fcmp oeq"
		case notEqual:
			instruction = "fcmp une"
		case less:
			instruction = "fcmp olt"
		case lessEqual:
			instruction = "fcmp ole"
		case greater:
			instruction = "fcmp ogt"
		case greaterEqual:
			instruction = "fcmp oge"
		default:
			panic("logical operators are control flow")
		}
	} else {
		switch op {
		case add:
			instruction = "add"
		case subtract:
			instruction = "sub"
		case multiply:
			instruction = "mul"
		case divide:
			instruction = "sdiv"
		case remainder:
			instruction = "srem"
		case equal:
			instruction = "icmp eq"
		case notEqual:
			instruction = "icmp ne"
		case less:
			instruction = "icmp slt"
		case lessEqual:
			instruction = "icmp sle"
		case greater:
			instruction = "icmp sgt"
		case greaterEqual:
			instruction = "icmp sge"
		default:
			panic("logical operators are control flow")
		}
	}
	return f.result(fmt.Sprintf("%s %s %s, %s", instruction, left.ty.llvm(), left.operand, right.operand))
}
