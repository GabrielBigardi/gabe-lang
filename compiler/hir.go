package compiler

// Program is typed HIR. Only semantic analysis constructs its private nodes.
type Program struct {
	functions []*function
	entry     int
}
type function struct {
	name, symbol string
	returnType   valueType
	parameters   []int
	variables    []valueType
	body         *stmt
}
type stmt struct {
	kind                        stmtKind
	statements                  []*stmt
	variable                    int
	captures                    []int
	value, condition, step      *expr
	body, elseBody, initializer *stmt
}
type expr struct {
	kind               exprKind
	ty                 valueType
	literal            literal
	function           int
	arguments          []*expr
	value, left, right *expr
	target             *lvalue
	unary              unaryOp
	op                 binaryOp
	delta              int
	prefix, newline    bool
}
type lvalue struct {
	ty       valueType
	variable int
	pointer  *expr
}
