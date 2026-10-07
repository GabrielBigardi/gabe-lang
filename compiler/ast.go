package compiler

import "strings"

type typeKind uint8

const (
	typeInvalid typeKind = iota
	typeInt
	typeLong
	typeDouble
	typeBool
	typeString
	typeVoid
)

// Pointer depth preserves Gabe pointee types independently of LLVM's opaque ptr.
type valueType struct {
	kind  typeKind
	depth int
}

var (
	intType    = valueType{kind: typeInt}
	longType   = valueType{kind: typeLong}
	doubleType = valueType{kind: typeDouble}
	boolType   = valueType{kind: typeBool}
	stringType = valueType{kind: typeString}
	voidType   = valueType{kind: typeVoid}
)

func (t valueType) numeric() bool {
	return t.depth == 0 && (t.kind == typeInt || t.kind == typeLong || t.kind == typeDouble)
}
func (t valueType) integer() bool      { return t == intType || t == longType }
func (t valueType) pointer() valueType { t.depth++; return t }
func (t valueType) element() valueType { t.depth--; return t }
func (t valueType) llvm() string {
	if t.depth > 0 {
		return "ptr"
	}
	switch t.kind {
	case typeInt:
		return "i32"
	case typeLong:
		return "i64"
	case typeDouble:
		return "double"
	case typeBool:
		return "i1"
	case typeString:
		return "ptr"
	case typeVoid:
		return "void"
	default:
		panic("invalid analyzed type")
	}
}
func (t valueType) String() string {
	names := [...]string{"<invalid>", "int", "long", "double", "bool", "string", "void"}
	return names[t.kind] + strings.Repeat("*", t.depth)
}

type unaryOp uint8

const (
	unaryNone unaryOp = iota
	negate
	positive
	not
	address
	dereference
)

type binaryOp uint8

const (
	binaryNone binaryOp = iota
	add
	subtract
	multiply
	divide
	remainder
	equal
	notEqual
	less
	lessEqual
	greater
	greaterEqual
	and
	or
)

func (op binaryOp) String() string {
	return [...]string{"None", "Add", "Subtract", "Multiply", "Divide", "Remainder", "Equal", "NotEqual", "Less", "LessEqual", "Greater", "GreaterEqual", "And", "Or"}[op]
}

type literal struct {
	ty      valueType
	integer int64
	double  float64
	boolean bool
	text    string
}
type exprKind uint8

const (
	exprLiteral exprKind = iota
	exprName
	exprCall
	exprUnary
	exprBinary
	exprAssign
	exprUpdate
	exprLoad
	exprAddress
	exprPrint
	exprCast
)

type astExpr struct {
	kind                       exprKind
	span                       Span
	literal                    literal
	name                       string
	arguments                  []*astExpr
	unary                      unaryOp
	op                         binaryOp
	value, left, right, target *astExpr
	delta                      int
	prefix                     bool
}
type stmtKind uint8

const (
	stmtBlock stmtKind = iota
	stmtUnsafe
	stmtLet
	stmtExpr
	stmtReturn
	stmtIf
	stmtWhile
	stmtFor
	stmtBreak
	stmtContinue
	stmtEmpty
)

type astStmt struct {
	kind                        stmtKind
	span                        Span
	statements                  []*astStmt
	name                        string
	ty                          *valueType
	value, condition, step      *astExpr
	body, elseBody, initializer *astStmt
}
type astParameter struct {
	name string
	ty   valueType
	span Span
}
type astFunction struct {
	name, container string
	returnType      valueType
	parameters      []astParameter
	body            *astStmt
	unsafe          bool
	span            Span
}
type astProgram struct{ functions []*astFunction }
