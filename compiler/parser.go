package compiler

import (
	"math"
	"strconv"
	"strings"
)

type parser struct {
	tokens   []token
	position int
	err      *Diagnostic
}
type modifiers struct {
	unsafe, external, static bool
	access                   accessLevel
}

func parse(tokens []token) (*astProgram, error) {
	p := &parser{tokens: tokens}
	program := p.program()
	if p.err != nil {
		return nil, p.err
	}
	return program, nil
}
func (p *parser) current() token { return p.tokens[p.position] }
func (p *parser) previous() Span {
	if p.position == 0 {
		return p.current().span
	}
	return p.tokens[p.position-1].span
}
func (p *parser) fail(span Span, message string) {
	if p.err == nil {
		p.err = diagnostic(span, message)
	}
}
func (p *parser) at(text string) bool {
	t := p.current()
	return p.err == nil && (t.kind == tokenWord || t.kind == tokenSymbol) && t.text == text
}
func (p *parser) take(text string) bool {
	if !p.at(text) {
		return false
	}
	p.position++
	return true
}
func (p *parser) expect(text string) Span {
	span := p.current().span
	if !p.take(text) {
		p.fail(span, "expected '"+text+"'")
	}
	return span
}
func (p *parser) name() token {
	t := p.current()
	if p.err != nil {
		return t
	}
	if t.kind != tokenWord {
		p.fail(t.span, "expected an identifier")
		return t
	}
	switch t.text {
	case "class", "static", "public", "private", "internal", "unsafe", "extern", "using", "namespace", "return", "defer", "if", "else", "while", "for", "break", "continue", "true", "false", "var", "void", "int", "long", "double", "bool", "string":
		p.fail(t.span, "expected an identifier, found a keyword")
		return t
	}
	p.position++
	return t
}
func (p *parser) modifiers() modifiers {
	start := p.current().span
	var m modifiers
	for {
		span := p.current().span
		switch {
		case p.take("unsafe"):
			m.unsafe = true
		case p.take("extern"):
			m.external = true
		case p.take("static"):
			m.static = true
		case p.at("public") || p.at("private"):
			if m.access != accessUnspecified {
				p.fail(span, "declarations must have exactly one access modifier: 'public' or 'private'")
				return m
			}
			if p.take("public") {
				m.access = accessPublic
			} else {
				p.take("private")
				m.access = accessPrivate
			}
		case p.take("internal"):
			p.fail(span, "access modifier must be 'public' or 'private'; 'internal' is not supported")
			return m
		default:
			if m.access == accessUnspecified {
				p.fail(start, "declaration requires an explicit 'public' or 'private' access modifier")
			}
			return m
		}
	}
}
func (p *parser) ty() valueType {
	var ty valueType
	switch {
	case p.take("int"):
		ty = intType
	case p.take("long"):
		ty = longType
	case p.take("double"):
		ty = doubleType
	case p.take("bool"):
		ty = boolType
	case p.take("string"):
		ty = stringType
	case p.take("void"):
		ty = voidType
	default:
		p.fail(p.current().span, "expected a type (int, long, double, bool, string, or void)")
	}
	for p.take("*") {
		ty = ty.pointer()
	}
	return ty
}
func (p *parser) program() *astProgram {
	program := &astProgram{}
	for p.take("using") {
		name := p.name()
		if name.text != "System" {
			p.fail(name.span, "only 'using System;' is supported in v0.1")
		}
		p.expect(";")
	}
	for p.err == nil && p.current().kind != tokenEOF {
		start := p.current().span
		m := p.modifiers()
		if p.take("class") {
			if m.external {
				p.fail(start, "a class cannot be extern")
			}
			container := p.name().text
			p.expect("{")
			for p.err == nil && !p.take("}") {
				start := p.current().span
				inner := p.modifiers()
				if !inner.static {
					p.fail(start, "class methods must be static in v0.1")
				}
				inner.unsafe = inner.unsafe || m.unsafe
				program.functions = append(program.functions, p.function(container, inner, start))
			}
		} else {
			program.functions = append(program.functions, p.function("", m, start))
		}
	}
	return program
}
func (p *parser) function(container string, m modifiers, start Span) *astFunction {
	fn := &astFunction{container: container, access: m.access, returnType: p.ty(), unsafe: m.unsafe}
	fn.name = p.name().text
	if container != "" {
		fn.name = container + "." + fn.name
	}
	p.expect("(")
	if !p.take(")") {
		for p.err == nil {
			span := p.current().span
			ty := p.ty()
			name := p.name()
			fn.parameters = append(fn.parameters, astParameter{name.text, ty, span.join(name.span)})
			if p.take(")") {
				break
			}
			p.expect(",")
		}
	}
	if m.external {
		p.expect(";")
	} else {
		fn.body = p.block()
	}
	fn.span = start.join(p.previous())
	return fn
}
func (p *parser) block() *astStmt {
	start := p.expect("{")
	s := &astStmt{kind: stmtBlock}
	for p.err == nil && !p.at("}") {
		if p.current().kind == tokenEOF {
			p.fail(p.current().span, "expected '}' before end of file")
			break
		}
		s.statements = append(s.statements, p.statement())
	}
	s.span = start.join(p.expect("}"))
	return s
}
func (p *parser) statement() *astStmt {
	start := p.current().span
	s := &astStmt{}
	switch {
	case p.at("{"):
		return p.block()
	case p.take("unsafe"):
		s.kind, s.body = stmtUnsafe, p.block()
	case p.take("defer"):
		s.kind, s.value = stmtDefer, p.expression()
		p.expect(";")
	case p.take("return"):
		s.kind = stmtReturn
		if !p.at(";") {
			s.value = p.expression()
		}
		p.expect(";")
	case p.take("if"):
		s.kind = stmtIf
		p.expect("(")
		s.condition = p.expression()
		p.expect(")")
		s.body = p.statement()
		if p.take("else") {
			s.elseBody = p.statement()
		}
	case p.take("while"):
		s.kind = stmtWhile
		p.expect("(")
		s.condition = p.expression()
		p.expect(")")
		s.body = p.statement()
	case p.take("for"):
		s.kind = stmtFor
		p.expect("(")
		if !p.take(";") {
			s.initializer = p.simpleStatement()
		}
		if !p.at(";") {
			s.condition = p.expression()
		}
		p.expect(";")
		if !p.at(")") {
			s.step = p.expression()
		}
		p.expect(")")
		s.body = p.statement()
	case p.take("break"):
		s.kind = stmtBreak
		p.expect(";")
	case p.take("continue"):
		s.kind = stmtContinue
		p.expect(";")
	case p.take(";"):
		s.kind = stmtEmpty
	default:
		return p.simpleStatement()
	}
	s.span = start.join(p.previous())
	return s
}
func (p *parser) simpleStatement() *astStmt {
	start := p.current().span
	s := &astStmt{}
	if p.at("var") || p.at("int") || p.at("long") || p.at("double") || p.at("bool") || p.at("string") || p.at("void") {
		s.kind = stmtLet
		if !p.take("var") {
			ty := p.ty()
			s.ty = &ty
		}
		s.name = p.name().text
		p.expect("=")
		s.value = p.expression()
	} else {
		s.kind, s.value = stmtExpr, p.expression()
	}
	s.span = start.join(p.expect(";"))
	return s
}
func (p *parser) expression() *astExpr {
	target := p.binary(1)
	op, assignment := binaryNone, true
	switch {
	case p.take("="):
	case p.take("+="):
		op = add
	case p.take("-="):
		op = subtract
	case p.take("*="):
		op = multiply
	case p.take("/="):
		op = divide
	case p.take("%="):
		op = remainder
	default:
		assignment = false
	}
	if !assignment {
		return target
	}
	value := p.expression()
	return &astExpr{kind: exprAssign, target: target, value: value, op: op, span: target.span.join(value.span)}
}
func (p *parser) binaryOperator() (binaryOp, int) {
	if p.err != nil || p.current().kind != tokenSymbol {
		return binaryNone, 0
	}
	switch p.current().text {
	case "||":
		return or, 1
	case "&&":
		return and, 2
	case "==":
		return equal, 3
	case "!=":
		return notEqual, 3
	case "<":
		return less, 4
	case "<=":
		return lessEqual, 4
	case ">":
		return greater, 4
	case ">=":
		return greaterEqual, 4
	case "+":
		return add, 5
	case "-":
		return subtract, 5
	case "*":
		return multiply, 6
	case "/":
		return divide, 6
	case "%":
		return remainder, 6
	}
	return binaryNone, 0
}
func (p *parser) binary(minimum int) *astExpr {
	left := p.unary()
	for {
		op, precedence := p.binaryOperator()
		if precedence < minimum {
			return left
		}
		p.position++
		right := p.binary(precedence + 1)
		left = &astExpr{kind: exprBinary, op: op, left: left, right: right, span: left.span.join(right.span)}
	}
}
func (p *parser) unary() *astExpr {
	start := p.current().span
	if p.take("++") || p.take("--") {
		delta := 1
		if p.tokens[p.position-1].text == "--" {
			delta = -1
		}
		target := p.unary()
		return &astExpr{kind: exprUpdate, target: target, delta: delta, prefix: true, span: start.join(target.span)}
	}
	op := unaryNone
	switch {
	case p.take("-"):
		op = negate
	case p.take("+"):
		op = positive
	case p.take("!"):
		op = not
	case p.take("&"):
		op = address
	case p.take("*"):
		op = dereference
	}
	if op != unaryNone {
		if op == negate && p.current().kind == tokenNumber {
			t := p.current()
			p.position++
			span := start.join(t.span)
			return &astExpr{kind: exprLiteral, literal: p.number("-"+t.text, span), span: span}
		}
		value := p.unary()
		return &astExpr{kind: exprUnary, unary: op, value: value, span: start.join(value.span)}
	}
	value := p.primary()
	if p.take("++") || p.take("--") {
		t := p.tokens[p.position-1]
		delta := 1
		if t.text == "--" {
			delta = -1
		}
		value = &astExpr{kind: exprUpdate, target: value, delta: delta, span: value.span.join(t.span)}
	}
	return value
}
func (p *parser) primary() *astExpr {
	t := p.current()
	if p.take("(") {
		e := p.expression()
		e.span = t.span.join(p.expect(")"))
		return e
	}
	e := &astExpr{span: t.span}
	if p.err != nil {
		return e
	}
	switch t.kind {
	case tokenNumber:
		p.position++
		e.kind, e.literal = exprLiteral, p.number(t.text, t.span)
	case tokenString:
		p.position++
		e.kind, e.literal = exprLiteral, literal{ty: stringType, text: t.text}
	case tokenWord:
		if t.text == "true" || t.text == "false" {
			p.position++
			e.kind, e.literal = exprLiteral, literal{ty: boolType, boolean: t.text == "true"}
		} else {
			e.kind, e.name = exprName, p.name().text
			for p.take(".") {
				e.name += "." + p.name().text
			}
			if p.take("(") {
				e.kind = exprCall
				if !p.take(")") {
					for p.err == nil {
						e.arguments = append(e.arguments, p.expression())
						if p.take(")") {
							break
						}
						p.expect(",")
					}
				}
			}
		}
	default:
		p.fail(t.span, "expected an expression")
	}
	e.span = t.span.join(p.previous())
	return e
}
func (p *parser) number(number string, span Span) literal {
	long := strings.HasSuffix(number, "l") || strings.HasSuffix(number, "L")
	digits := number
	if long {
		digits = digits[:len(digits)-1]
	}
	if strings.ContainsAny(digits, ".eE") {
		if long {
			p.fail(span, "a floating-point literal cannot have the L suffix")
			return literal{}
		}
		value, err := strconv.ParseFloat(digits, 64)
		if math.IsInf(value, 0) || math.IsNaN(value) {
			p.fail(span, "double literal is out of range")
		} else if err != nil {
			p.fail(span, "invalid double literal")
		}
		return literal{ty: doubleType, double: value}
	}
	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		p.fail(span, "integer literal is outside the signed 64-bit range")
	}
	ty := longType
	if !long && value >= math.MinInt32 && value <= math.MaxInt32 {
		ty = intType
	}
	return literal{ty: ty, integer: value}
}
