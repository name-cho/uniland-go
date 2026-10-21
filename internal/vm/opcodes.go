package vm

import (
	"fmt"

	"github.com/name-cho/uniland-go/internal/ast"
	"github.com/name-cho/uniland-go/internal/interp/val"
)

type Op uint8

const (
	OP_HALT Op = iota
	OP_CONST
	OP_POP
	OP_GETVAR
	OP_SETVAR
	OP_DEFVAR
	OP_CLOSURE
	OP_ECHO
	OP_TEMPLATE
	OP_LIST
	OP_DICT
	OP_INDEX_GET
	OP_INDEX_SET
	OP_MEMBER_GET
	OP_MEMBER_SET
	OP_ADD
	OP_SUB
	OP_MUL
	OP_DIV
	OP_MOD
	OP_POW
	OP_EQ
	OP_NEQ
	OP_CMP
	OP_NEG
	OP_NOT
	OP_INCR
	OP_COMPOUND
	OP_AND
	OP_OR
	OP_JMP
	OP_JMP_F
	OP_RET
	OP_TRY
	OP_TEND
	OP_CEND
	OP_FEND
	OP_PYBLOCK
	OP_GOBLOCK
	OP_THROW
	OP_STR_ERR
	OP_BREAK
	OP_CONTINUE
	OP_FORIN_START
	OP_FORIN_NEXT
	OP_FORIN_END
	OP_BLOCK_ENTER
	OP_BLOCK_EXIT
	OP_LOOP_PUSH
	OP_LOOP_POP
	OP_CALL
	OP_PUTREG
	OP_GETREG
)

type Ins struct {
	Op Op
	A  uint16
	B  uint16
}

type TryMeta struct {
	CatchPc   int
	CatchVar  int
	FinallyPc int
	AfterPc   int
}

type Code struct {
	Name   string
	Params []string
	Consts []val.Value
	Ins    []Ins
	RegN   int

	astBody *ast.Block
}

func constIdx(c *Code, v val.Value) int {
	for i, existing := range c.Consts {
		if sameConst(existing, v) {
			return i
		}
	}
	if len(c.Consts) > 60000 {
		panic(fmt.Sprintf("vm: константный пул переполнен (%s)", c.Name))
	}
	c.Consts = append(c.Consts, v)
	return len(c.Consts) - 1
}

func sameConst(a, b val.Value) bool {
	switch x := a.(type) {
	case val.Str:
		y, ok := b.(val.Str)
		return ok && x == y
	case val.Bool:
		y, ok := b.(val.Bool)
		return ok && bool(x) == bool(y)
	case *val.Int:
		y, ok := b.(*val.Int)
		return ok && x.N.Cmp(y.N) == 0
	case *TryMeta:
		y, ok := b.(*TryMeta)
		return ok && x == y
	}
	return false
}
