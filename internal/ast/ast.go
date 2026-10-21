package ast

type Node = any

type Program struct{ Statements []Node }

type VariableDeclaration struct {
	Name    string
	Value   Node
	IsConst bool
	Line    int
}

type FunctionDeclaration struct {
	Name   string
	Params []string
	Body   *Block
	Line   int
}

type FunctionExpression struct {
	Params []string
	Body   *Block
	Line   int
}

type ReturnStatement struct {
	Value Node
	Line  int
}

type IfStatement struct {
	Condition Node
	Then      *Block
	Else      Node
	Line      int
}

type ForLoop struct {
	Init, Condition, Update Node
	Body                    *Block
	Line                    int
}

type ForInLoop struct {
	Var      string
	Iterable Node
	Body     *Block
	Line     int
}

type WhileLoop struct {
	Condition Node
	Body      *Block
	Line      int
}

type BreakStatement struct{ Line int }
type ContinueStatement struct{ Line int }

type Block struct {
	Statements []Node
	Line       int
}

type ExpressionStatement struct {
	Expr Node
	Line int
}

type Assignment struct {
	Target Node
	Value  Node
	Op     string
	Line   int
}

type TryCatch struct {
	Try      *Block
	CatchVar *string
	Catch    *Block
	Finally  *Block
	Line     int
}

type ThrowStatement struct {
	Value Node
	Line  int
}

type PythonBlock struct {
	Code string
	Line int
}

type GoBlock struct {
	Code string
	Line int
}

type BinaryOp struct {
	Left, Right Node
	Op          string
	Line        int
}

type UnaryOp struct {
	Op   string
	Expr Node
	Line int
}

type CallExpression struct {
	Callee Node
	Args   []Node
	Line   int
}

type Identifier struct {
	Name string
	Line int
}

type Literal struct {
	Value any
	Line  int
}

type TemplateLiteral struct {
	Parts []TPLPart
	Line  int
}

type TPLPart struct {
	Str    string
	Expr   Node
	IsExpr bool
}

func StrPart(s string) TPLPart { return TPLPart{Str: s} }
func ExprPart(n Node) TPLPart  { return TPLPart{Expr: n, IsExpr: true} }

type ArrayLiteral struct {
	Elements []Node
	Line     int
}

type KeyValue struct {
	Key   string
	Value Node
}

type ObjectLiteral struct {
	Pairs []KeyValue
	Line  int
}

type IndexExpression struct {
	Obj   Node
	Index Node
	Line  int
}

type MemberExpression struct {
	Obj  Node
	Name string
	Line int
}
