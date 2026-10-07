package compiler

import (
	"fmt"
	"strings"
)

type signature struct {
	id         int
	returnType valueType
	parameters []valueType
	unsafe     bool
}
type checker struct {
	signatures             map[string]signature
	function               *astFunction
	scopes                 []map[string]int
	variables              []valueType
	unsafeDepth, loopDepth int
}

func analyze(parsed *astProgram) (*Program, error) {
	signatures := make(map[string]signature)
	entry := -1
	for id, fn := range parsed.functions {
		if strings.HasPrefix(fn.name, "Console.") || strings.HasPrefix(fn.name, "System.Console.") {
			return nil, diagnostic(fn.span, "Console is reserved for built-in output")
		}
		if fn.body == nil && fn.container != "" {
			return nil, diagnostic(fn.span, "declare extern C functions at file scope")
		}
		parameters := make([]valueType, len(fn.parameters))
		for i, p := range fn.parameters {
			if p.ty == voidType {
				return nil, diagnostic(p.span, "parameters cannot have type void")
			}
			parameters[i] = p.ty
		}
		if fn.body == nil {
			if fn.returnType == boolType {
				return nil, diagnostic(fn.span, "extern bool parameters and results are not supported; use int for C ABI flags")
			}
			for _, ty := range parameters {
				if ty == boolType {
					return nil, diagnostic(fn.span, "extern bool parameters and results are not supported; use int for C ABI flags")
				}
			}
			if strings.HasPrefix(fn.name, "gb_") || fn.name == "main" || fn.name == "printf" || fn.name == "puts" {
				return nil, diagnostic(fn.span, "this external symbol is reserved by the runtime")
			}
		}
		if _, exists := signatures[fn.name]; exists {
			return nil, diagnostic(fn.span, fmt.Sprintf("duplicate function '%s'; overloading is not supported", fn.name))
		}
		signatures[fn.name] = signature{id, fn.returnType, parameters, fn.unsafe || fn.body == nil}
		parts := strings.Split(fn.name, ".")
		if parts[len(parts)-1] == "Main" && fn.body != nil {
			if entry >= 0 {
				return nil, diagnostic(fn.span, "a program must have exactly one Main function")
			}
			entry = id
			if len(parameters) != 0 || (fn.returnType != intType && fn.returnType != voidType) || fn.unsafe {
				return nil, diagnostic(fn.span, "entry point must be a safe int Main() or void Main()")
			}
		}
	}
	if entry < 0 {
		return nil, diagnostic(Span{}, "missing entry point: declare int Main() or static int Main() inside a class")
	}
	program := &Program{entry: entry}
	for id, fn := range parsed.functions {
		c := &checker{signatures: signatures, function: fn, scopes: []map[string]int{{}}}
		if fn.unsafe || fn.body == nil {
			c.unsafeDepth = 1
		}
		if err := c.validateType(fn.returnType, fn.span); err != nil {
			return nil, err
		}
		checked := &function{name: fn.name, symbol: fmt.Sprintf("gb_%d", id), returnType: fn.returnType}
		if fn.body == nil {
			checked.symbol = fn.name
		}
		for _, p := range fn.parameters {
			if err := c.validateType(p.ty, p.span); err != nil {
				return nil, err
			}
			variable, err := c.declare(p.name, p.ty, p.span)
			if err != nil {
				return nil, err
			}
			checked.parameters = append(checked.parameters, variable)
		}
		if fn.body != nil {
			body, terminates, err := c.statement(fn.body)
			if err != nil {
				return nil, err
			}
			if fn.returnType != voidType && !terminates {
				return nil, diagnostic(fn.span, fmt.Sprintf("not all paths in '%s' return a value", fn.name))
			}
			checked.body = body
		}
		checked.variables = c.variables
		program.functions = append(program.functions, checked)
	}
	return program, nil
}
func (c *checker) requireUnsafe(span Span) error {
	if c.unsafeDepth == 0 {
		return diagnostic(span, "pointer operations and extern/unsafe calls require an unsafe block or unsafe function")
	}
	return nil
}
func (c *checker) validateType(ty valueType, span Span) error {
	if ty.depth > 0 {
		return c.requireUnsafe(span)
	}
	return nil
}
func (c *checker) declare(name string, ty valueType, span Span) (int, error) {
	scope := c.scopes[len(c.scopes)-1]
	if _, exists := scope[name]; exists {
		return 0, diagnostic(span, "duplicate local '"+name+"'")
	}
	id := len(c.variables)
	c.variables = append(c.variables, ty)
	scope[name] = id
	return id, nil
}
func (c *checker) lookup(name string, span Span) (int, error) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if id, ok := c.scopes[i][name]; ok {
			return id, nil
		}
	}
	return 0, diagnostic(span, "unknown variable '"+name+"'")
}
func (c *checker) scopedStatement(s *astStmt) (*stmt, bool, error) {
	c.scopes = append(c.scopes, make(map[string]int))
	defer func() { c.scopes = c.scopes[:len(c.scopes)-1] }()
	return c.statement(s)
}
func (c *checker) condition(e *astExpr) (*expr, error) {
	value, err := c.expression(e)
	if err != nil {
		return nil, err
	}
	return coerce(value, boolType, e.span)
}
func (c *checker) statement(s *astStmt) (*stmt, bool, error) {
	result := &stmt{kind: s.kind}
	terminates := false
	switch s.kind {
	case stmtBlock:
		c.scopes = append(c.scopes, make(map[string]int))
		defer func() { c.scopes = c.scopes[:len(c.scopes)-1] }()
		for _, child := range s.statements {
			checked, ends, err := c.statement(child)
			if err != nil {
				return nil, false, err
			}
			terminates = terminates || ends
			result.statements = append(result.statements, checked)
		}
	case stmtUnsafe:
		c.unsafeDepth++
		defer func() { c.unsafeDepth-- }()
		return c.scopedStatement(s.body)
	case stmtLet:
		value, err := c.expression(s.value)
		if err != nil {
			return nil, false, err
		}
		ty := value.ty
		if s.ty != nil {
			ty = *s.ty
		}
		if err := c.validateType(ty, s.span); err != nil {
			return nil, false, err
		}
		if ty == voidType {
			return nil, false, diagnostic(s.span, "a variable cannot have type void")
		}
		result.value, err = coerce(value, ty, s.value.span)
		if err != nil {
			return nil, false, err
		}
		result.variable, err = c.declare(s.name, ty, s.span)
		if err != nil {
			return nil, false, err
		}
	case stmtExpr:
		if s.value.kind != exprCall && s.value.kind != exprAssign && s.value.kind != exprUpdate {
			return nil, false, diagnostic(s.value.span, "an expression statement must be a call, assignment, or increment")
		}
		value, err := c.expression(s.value)
		if err != nil {
			return nil, false, err
		}
		result.value = value
	case stmtReturn:
		terminates = true
		if c.function.returnType == voidType {
			if s.value != nil {
				return nil, false, diagnostic(s.span, "a void function cannot return a value")
			}
		} else {
			if s.value == nil {
				return nil, false, diagnostic(s.span, fmt.Sprintf("expected a return value of type %s", c.function.returnType))
			}
			value, err := c.expression(s.value)
			if err != nil {
				return nil, false, err
			}
			result.value, err = coerce(value, c.function.returnType, s.value.span)
			if err != nil {
				return nil, false, err
			}
		}
	case stmtIf:
		condition, err := c.condition(s.condition)
		if err != nil {
			return nil, false, err
		}
		result.condition = condition
		body, thenEnds, err := c.scopedStatement(s.body)
		if err != nil {
			return nil, false, err
		}
		result.body = body
		if s.elseBody != nil {
			body, elseEnds, err := c.scopedStatement(s.elseBody)
			if err != nil {
				return nil, false, err
			}
			result.elseBody = body
			terminates = thenEnds && elseEnds
		}
	case stmtWhile:
		condition, err := c.condition(s.condition)
		if err != nil {
			return nil, false, err
		}
		result.condition = condition
		c.loopDepth++
		body, _, err := c.scopedStatement(s.body)
		c.loopDepth--
		if err != nil {
			return nil, false, err
		}
		result.body = body
	case stmtFor:
		c.scopes = append(c.scopes, make(map[string]int))
		defer func() { c.scopes = c.scopes[:len(c.scopes)-1] }()
		if s.initializer != nil {
			initializer, _, err := c.statement(s.initializer)
			if err != nil {
				return nil, false, err
			}
			result.initializer = initializer
		}
		if s.condition != nil {
			condition, err := c.condition(s.condition)
			if err != nil {
				return nil, false, err
			}
			result.condition = condition
		}
		if s.step != nil {
			step, err := c.expression(s.step)
			if err != nil {
				return nil, false, err
			}
			result.step = step
		}
		c.loopDepth++
		body, _, err := c.scopedStatement(s.body)
		c.loopDepth--
		if err != nil {
			return nil, false, err
		}
		result.body = body
	case stmtBreak, stmtContinue:
		if c.loopDepth == 0 {
			return nil, false, diagnostic(s.span, "break and continue are only valid inside a loop")
		}
		terminates = true
	case stmtEmpty:
	default:
		panic("invalid parsed statement")
	}
	return result, terminates, nil
}

func (c *checker) lvalue(e *astExpr) (*lvalue, error) {
	if e.kind == exprName {
		id, err := c.lookup(e.name, e.span)
		if err != nil {
			return nil, err
		}
		ty := c.variables[id]
		if err := c.validateType(ty, e.span); err != nil {
			return nil, err
		}
		return &lvalue{ty: ty, variable: id}, nil
	}
	if e.kind == exprUnary && e.unary == dereference {
		if err := c.requireUnsafe(e.span); err != nil {
			return nil, err
		}
		pointer, err := c.expression(e.value)
		if err != nil {
			return nil, err
		}
		if pointer.ty.depth == 0 {
			return nil, diagnostic(e.span, "dereferencing requires a pointer")
		}
		ty := pointer.ty.element()
		if ty == voidType {
			return nil, diagnostic(e.span, "cannot dereference a void pointer")
		}
		return &lvalue{ty: ty, pointer: pointer}, nil
	}
	return nil, diagnostic(e.span, "expected a variable or dereferenced pointer")
}

func coerce(value *expr, target valueType, span Span) (*expr, error) {
	if value.ty == target {
		return value, nil
	}
	if (value.ty == intType && (target == longType || target == doubleType)) || (value.ty == longType && target == doubleType) {
		return &expr{kind: exprCast, value: value, ty: target}, nil
	}
	return nil, diagnostic(span, fmt.Sprintf("cannot convert %s to %s", value.ty, target))
}

func (c *checker) expression(e *astExpr) (*expr, error) {
	result := &expr{kind: e.kind}
	switch e.kind {
	case exprLiteral:
		result.literal, result.ty = e.literal, e.literal.ty
	case exprName:
		target, err := c.lvalue(e)
		if err != nil {
			return nil, err
		}
		result.kind, result.target, result.ty = exprLoad, target, target.ty
	case exprCall:
		switch e.name {
		case "Console.WriteLine", "System.Console.WriteLine", "Console.Write", "System.Console.Write":
			newline := strings.HasSuffix(e.name, "WriteLine")
			if len(e.arguments) > 1 || (len(e.arguments) == 0 && !newline) {
				return nil, diagnostic(e.span, "Console.Write takes one value; Console.WriteLine takes zero or one")
			}
			value := &expr{kind: exprLiteral, ty: stringType, literal: literal{ty: stringType}}
			if len(e.arguments) > 0 {
				var err error
				value, err = c.expression(e.arguments[0])
				if err != nil {
					return nil, err
				}
			}
			if !value.ty.numeric() && value.ty != boolType && value.ty != stringType {
				return nil, diagnostic(e.span, "Console output supports int, long, double, bool, and string")
			}
			result.kind, result.ty, result.value, result.newline = exprPrint, voidType, value, newline
		default:
			var sig signature
			found := false
			if c.function.container != "" {
				sig, found = c.signatures[c.function.container+"."+e.name]
			}
			if !found {
				sig, found = c.signatures[e.name]
			}
			if !found {
				return nil, diagnostic(e.span, "unknown function '"+e.name+"'")
			}
			if sig.unsafe {
				if err := c.requireUnsafe(e.span); err != nil {
					return nil, err
				}
			}
			if len(sig.parameters) != len(e.arguments) {
				return nil, diagnostic(e.span, fmt.Sprintf("'%s' expects %d arguments, got %d", e.name, len(sig.parameters), len(e.arguments)))
			}
			for i, argument := range e.arguments {
				value, err := c.expression(argument)
				if err != nil {
					return nil, err
				}
				value, err = coerce(value, sig.parameters[i], argument.span)
				if err != nil {
					return nil, err
				}
				result.arguments = append(result.arguments, value)
			}
			result.function, result.ty = sig.id, sig.returnType
		}
	case exprUnary:
		switch e.unary {
		case address:
			if err := c.requireUnsafe(e.span); err != nil {
				return nil, err
			}
			target, err := c.lvalue(e.value)
			if err != nil {
				return nil, err
			}
			result.kind, result.target, result.ty = exprAddress, target, target.ty.pointer()
		case dereference:
			target, err := c.lvalue(e)
			if err != nil {
				return nil, err
			}
			result.kind, result.target, result.ty = exprLoad, target, target.ty
		default:
			value, err := c.expression(e.value)
			if err != nil {
				return nil, err
			}
			if (e.unary == not && value.ty != boolType) || (e.unary != not && !value.ty.numeric()) {
				return nil, diagnostic(e.span, "unary ! requires bool; unary + and - require a number")
			}
			result.unary, result.value, result.ty = e.unary, value, value.ty
		}
	case exprBinary:
		left, err := c.expression(e.left)
		if err != nil {
			return nil, err
		}
		right, err := c.expression(e.right)
		if err != nil {
			return nil, err
		}
		return checkedBinary(e.op, left, right, e.span)
	case exprAssign:
		target, err := c.lvalue(e.target)
		if err != nil {
			return nil, err
		}
		value, err := c.expression(e.value)
		if err != nil {
			return nil, err
		}
		if e.op != binaryNone && (!target.ty.numeric() || !value.ty.numeric()) {
			return nil, diagnostic(e.span, "compound assignment requires numbers")
		}
		value, err = coerce(value, target.ty, e.value.span)
		if err != nil {
			return nil, err
		}
		result.target, result.value, result.op, result.ty = target, value, e.op, target.ty
	case exprUpdate:
		target, err := c.lvalue(e.target)
		if err != nil {
			return nil, err
		}
		if !target.ty.numeric() {
			return nil, diagnostic(e.span, "increment and decrement require a number")
		}
		result.target, result.delta, result.prefix, result.ty = target, e.delta, e.prefix, target.ty
	default:
		panic("invalid parsed expression")
	}
	return result, nil
}

func checkedBinary(op binaryOp, left, right *expr, span Span) (*expr, error) {
	logical := op == and || op == or
	equality := op == equal || op == notEqual
	comparison := equality || op == less || op == lessEqual || op == greater || op == greaterEqual
	if left.ty.numeric() && right.ty.numeric() && !logical {
		ty := intType
		if left.ty == doubleType || right.ty == doubleType {
			ty = doubleType
		} else if left.ty == longType || right.ty == longType {
			ty = longType
		}
		var err error
		left, err = coerce(left, ty, span)
		if err != nil {
			return nil, err
		}
		right, err = coerce(right, ty, span)
		if err != nil {
			return nil, err
		}
	} else if left.ty != right.ty || !((left.ty == boolType && (logical || equality)) || (left.ty.depth > 0 && equality)) {
		return nil, diagnostic(span, fmt.Sprintf("operator %s does not support %s and %s", op, left.ty, right.ty))
	}
	ty := left.ty
	if logical || comparison {
		ty = boolType
	}
	return &expr{kind: exprBinary, op: op, left: left, right: right, ty: ty}, nil
}
