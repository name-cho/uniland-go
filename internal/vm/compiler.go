package vm

import (
	"fmt"

	"github.com/name-cho/uniland-go/internal/ast"
	"github.com/name-cho/uniland-go/internal/interp/val"
)

type cstate struct {
	c     *Code
	nregs int
}

type compileError struct{ msg string }

func (e *compileError) Error() string { return e.msg }

func Compile(program *ast.Program, echo bool) (*Code, error) {
	c := &cstate{c: &Code{Name: "main"}}
	done := func() (code *Code, err error) {
		defer func() {
			if r := recover(); r != nil {
				if ce, ok := r.(*compileError); ok {
					code, err = nil, ce
					return
				}
				panic(r)
			}
		}()
		c.stmts(program.Statements, echo)
		c.emit(OP_HALT, 0, 0)
		return c.c, nil
	}
	return done()
}

func fail(format string, args ...any) {
	panic(&compileError{msg: fmt.Sprintf(format, args...)})
}

func (c *cstate) emit(op Op, a, b uint16) int {
	c.c.Ins = append(c.c.Ins, Ins{Op: op, A: a, B: b})
	return len(c.c.Ins) - 1
}

func (c *cstate) rawIdx(v val.Value) uint16 {
	if len(c.c.Consts) > 60000 {
		fail("vm: константный пул переполнен (%s)", c.c.Name)
	}
	c.c.Consts = append(c.c.Consts, v)
	return uint16(len(c.c.Consts) - 1)
}

func (c *cstate) idx(v val.Value) uint16 {
	i := constIdx(c.c, v)
	if i > 0xFFFF {
		fail("vm: индекс константы переполнен (%s)", c.c.Name)
	}
	return uint16(i)
}

func (c *cstate) lineIdx(line int) uint16 { return c.rawIdx(val.Int64(int64(line))) }

func (c *cstate) patch(pc int, target int) {
	if target > 0xFFFF || target < 0 {
		fail("vm: target переполнен (%s)", c.c.Name)
	}
	c.c.Ins[pc].A = uint16(target)
}

func (c *cstate) stmts(list []ast.Node, top bool) {
	for _, s := range list {
		c.stmt(s, top)
	}
}

func (c *cstate) stmt(n ast.Node, top bool) {
	switch t := n.(type) {
	case *ast.Block:
		c.emit(OP_BLOCK_ENTER, 0, 0)
		c.stmts(t.Statements, false)
		c.emit(OP_BLOCK_EXIT, 0, 0)

	case *ast.VariableDeclaration:
		c.expr(t.Value)
		con := uint16(0)
		if t.IsConst {
			con = 1
		}
		c.emit(OP_DEFVAR, c.idx(val.Str(t.Name)), con)

	case *ast.FunctionDeclaration:
		code := compileFunc(t.Name, t.Params, t.Body)
		c.emit(OP_CLOSURE, c.idx(code), 0)
		c.emit(OP_DEFVAR, c.idx(val.Str(t.Name)), 0)

	case *ast.ReturnStatement:
		if t.Value == nil {
			c.emit(OP_CONST, c.idx(val.Nul), 0)
		} else {
			c.expr(t.Value)
		}
		c.emit(OP_RET, 0, 0)

	case *ast.IfStatement:
		c.expr(t.Condition)
		jf := c.emit(OP_JMP_F, 0, 0)
		c.stmt(t.Then, false)
		if t.Else != nil {
			jmp := c.emit(OP_JMP, 0, 0)
			c.patch(jf, len(c.c.Ins))
			c.stmt(t.Else, false)
			c.patch(jmp, len(c.c.Ins))
		} else {
			c.patch(jf, len(c.c.Ins))
		}

	case *ast.ForLoop:
		if t.Init != nil {
			c.stmt(t.Init, false)
		}
		push := c.emit(OP_LOOP_PUSH, 0, 0)
		topPc := len(c.c.Ins)
		c.expr(t.Condition)
		jf := c.emit(OP_JMP_F, 0, 0)
		c.emit(OP_BLOCK_ENTER, 0, 0)
		c.stmt(t.Body, false)
		c.emit(OP_BLOCK_EXIT, 0, 0)
		contPc := topPc
		if t.Update != nil {
			contPc = len(c.c.Ins)
			c.stmt(t.Update, false)
		}
		c.emit(OP_JMP, uint16(topPc), 0)
		endPc := len(c.c.Ins)
		c.c.Ins[push] = Ins{Op: OP_LOOP_PUSH, A: uint16(endPc), B: uint16(contPc)}
		c.patch(jf, endPc)
		c.emit(OP_LOOP_POP, 0, 0)

	case *ast.WhileLoop:
		push := c.emit(OP_LOOP_PUSH, 0, 0)
		topPc := len(c.c.Ins)
		c.expr(t.Condition)
		jf := c.emit(OP_JMP_F, 0, 0)
		c.emit(OP_BLOCK_ENTER, 0, 0)
		c.stmt(t.Body, false)
		c.emit(OP_BLOCK_EXIT, 0, 0)
		c.emit(OP_JMP, uint16(topPc), 0)
		endPc := len(c.c.Ins)
		c.c.Ins[push] = Ins{Op: OP_LOOP_PUSH, A: uint16(endPc), B: uint16(topPc)}
		c.patch(jf, endPc)
		c.emit(OP_LOOP_POP, 0, 0)

	case *ast.ForInLoop:
		c.expr(t.Iterable)
		c.emit(OP_FORIN_START, 0, 0)
		push := c.emit(OP_LOOP_PUSH, 0, 0)
		nextPc := len(c.c.Ins)
		fin := c.emit(OP_FORIN_NEXT, 0, 0)
		c.emit(OP_BLOCK_ENTER, 0, 0)
		c.emit(OP_DEFVAR, c.idx(val.Str(t.Var)), 0)
		c.stmt(t.Body, false)
		c.emit(OP_BLOCK_EXIT, 0, 0)
		c.emit(OP_JMP, uint16(nextPc), 0)
		endPc := len(c.c.Ins)
		c.c.Ins[push] = Ins{Op: OP_LOOP_PUSH, A: uint16(endPc), B: uint16(nextPc)}
		c.patch(fin, endPc)
		c.emit(OP_LOOP_POP, 0, 0)
		c.emit(OP_FORIN_END, 0, 0)

	case *ast.BreakStatement:
		c.emit(OP_BREAK, 0, 0)

	case *ast.ContinueStatement:
		c.emit(OP_CONTINUE, 0, 0)

	case *ast.Assignment:
		c.assign(t)

	case *ast.ExpressionStatement:
		c.expr(t.Expr)
		if top {
			c.emit(OP_ECHO, 0, 0)
		} else {
			c.emit(OP_POP, 0, 0)
		}

	case *ast.PythonBlock:
		c.emit(OP_PYBLOCK, c.idx(val.Str(t.Code)), 0)

	case *ast.GoBlock:
		c.emit(OP_GOBLOCK, c.idx(val.Str(t.Code)), 0)

	case *ast.TryCatch:
		c.tryCatch(t)

	case *ast.ThrowStatement:
		c.expr(t.Value)
		c.emit(OP_THROW, 0, c.lineIdx(t.Line))

	default:
		fail("vm: не умею компилировать statement %T", n)
	}
}

func (c *cstate) tryCatch(t *ast.TryCatch) {
	meta := &TryMeta{CatchPc: -1, CatchVar: -1, FinallyPc: -1}
	c.emit(OP_TRY, c.idx(meta), 0)
	c.stmt(t.Try, false)
	c.emit(OP_TEND, c.idx(meta), 0)
	if t.Catch != nil {
		meta.CatchPc = len(c.c.Ins)

		c.stmt(t.Catch, false)
		c.emit(OP_CEND, c.idx(meta), 0)
	}
	if t.CatchVar != nil {
		meta.CatchVar = int(c.idx(val.Str(*t.CatchVar)))
	}
	if t.Finally != nil {
		meta.FinallyPc = len(c.c.Ins)
		c.stmt(t.Finally, false)
		c.emit(OP_FEND, c.idx(meta), 0)
	}
	meta.AfterPc = len(c.c.Ins)
}

func (c *cstate) assign(t *ast.Assignment) {
	switch t.Op {
	case "++", "--":
		inc := uint16(1)
		if t.Op == "--" {
			inc = 0
		}
		switch tt := t.Target.(type) {
		case *ast.Identifier:
			c.emit(OP_GETVAR, c.idx(val.Str(tt.Name)), 0)
			c.emit(OP_INCR, inc, 0)
			c.emit(OP_SETVAR, c.idx(val.Str(tt.Name)), 0)
		case *ast.IndexExpression:
			c.expr(tt.Obj)
			c.expr(tt.Index)
			c.emit(OP_INDEX_GET, 0, c.lineIdx(t.Line))
			c.emit(OP_INCR, inc, 0)
			c.expr(tt.Obj)
			c.expr(tt.Index)
			c.emit(OP_INDEX_SET, 0, c.lineIdx(t.Line))
		case *ast.MemberExpression:
			c.expr(tt.Obj)
			c.emit(OP_MEMBER_GET, c.idx(val.Str(tt.Name)), c.lineIdx(t.Line))
			c.emit(OP_INCR, inc, 0)
			c.expr(tt.Obj)
			c.emit(OP_MEMBER_SET, c.idx(val.Str(tt.Name)), c.lineIdx(t.Line))
		default:
			fail("vm: ++/-- для %T", t.Target)
		}
	case "=":
		switch tt := t.Target.(type) {
		case *ast.Identifier:
			c.expr(t.Value)
			c.emit(OP_SETVAR, c.idx(val.Str(tt.Name)), 0)
		case *ast.IndexExpression:
			c.expr(t.Value)
			c.expr(tt.Obj)
			c.expr(tt.Index)
			c.emit(OP_INDEX_SET, 0, c.lineIdx(t.Line))
		case *ast.MemberExpression:
			c.expr(t.Value)
			c.expr(tt.Obj)
			c.emit(OP_MEMBER_SET, c.idx(val.Str(tt.Name)), c.lineIdx(t.Line))
		default:
			fail("vm: присваивание для %T", t.Target)
		}
	default:
		opIdx := compoundOpIdx(t.Op)
		line := c.lineIdx(t.Line)
		switch tt := t.Target.(type) {
		case *ast.Identifier:
			c.emit(OP_GETVAR, c.idx(val.Str(tt.Name)), 0)
			c.expr(t.Value)
			c.emit(OP_COMPOUND, uint16(opIdx), line)
			c.emit(OP_SETVAR, c.idx(val.Str(tt.Name)), 0)
		case *ast.IndexExpression:

			c.expr(tt.Obj)
			c.expr(tt.Index)
			c.emit(OP_INDEX_GET, 0, line)
			reg := c.allocReg()
			c.emit(OP_PUTREG, uint16(reg), 0)
			c.emit(OP_GETREG, uint16(reg), 0)
			c.expr(t.Value)
			c.emit(OP_COMPOUND, uint16(opIdx), line)
			c.expr(tt.Obj)
			c.expr(tt.Index)
			c.emit(OP_INDEX_SET, 0, line)
		case *ast.MemberExpression:
			c.expr(tt.Obj)
			c.emit(OP_MEMBER_GET, c.idx(val.Str(tt.Name)), line)
			reg := c.allocReg()
			c.emit(OP_PUTREG, uint16(reg), 0)
			c.emit(OP_GETREG, uint16(reg), 0)
			c.expr(t.Value)
			c.emit(OP_COMPOUND, uint16(opIdx), line)
			c.expr(tt.Obj)
			c.emit(OP_MEMBER_SET, c.idx(val.Str(tt.Name)), line)
		default:
			fail("vm: присваивание для %T", t.Target)
		}
	}
}

func compoundOpIdx(op string) int {
	switch op {
	case "+=":
		return 0
	case "-=":
		return 1
	case "*=":
		return 2
	case "/=":
		return 3
	case "%=":
		return 4
	case "^=":
		return 5
	}
	fail("vm: неизвестный compound-op %s", op)
	return 0
}

var compoundOps = [6]string{"+", "-", "*", "/", "%", "^"}

func regNo(n int) uint16 {
	if n > 0xFFF0 {
		fail("vm: переполнение регистров")
	}
	return uint16(n)
}

func (c *cstate) allocReg() int {
	c.nregs++
	if c.c.RegN < c.nregs {
		c.c.RegN = c.nregs
	}
	return c.nregs - 1
}

func (c *cstate) expr(n ast.Node) {
	switch t := n.(type) {
	case *ast.Literal:
		c.emit(OP_CONST, c.idx(t.Value), 0)

	case *ast.TemplateLiteral:
		for _, part := range t.Parts {
			if part.IsExpr {
				c.expr(part.Expr)
			} else {
				c.emit(OP_CONST, c.idx(val.Str(part.Str)), 0)
			}
		}
		c.emit(OP_TEMPLATE, uint16(len(t.Parts)), 0)

	case *ast.Identifier:
		c.emit(OP_GETVAR, c.idx(val.Str(t.Name)), 0)

	case *ast.BinaryOp:
		switch t.Op {
		case "&&":
			c.expr(t.Left)
			j := c.emit(OP_AND, 0, 0)
			c.expr(t.Right)
			c.patch(j, len(c.c.Ins))
		case "||":
			c.expr(t.Left)
			j := c.emit(OP_OR, 0, 0)
			c.expr(t.Right)
			c.patch(j, len(c.c.Ins))
		default:
			c.expr(t.Left)
			c.expr(t.Right)
			c.binary(t.Op, t.Line)
		}

	case *ast.UnaryOp:
		c.expr(t.Expr)
		switch t.Op {
		case "-":
			c.emit(OP_NEG, 0, 0)
		case "!":
			c.emit(OP_NOT, 0, 0)
		default:
			fail("vm: неизвестный унарный оператор %s", t.Op)
		}

	case *ast.IndexExpression:
		c.expr(t.Obj)
		c.expr(t.Index)
		c.emit(OP_INDEX_GET, 0, c.lineIdx(t.Line))

	case *ast.MemberExpression:
		c.expr(t.Obj)
		c.emit(OP_MEMBER_GET, c.idx(val.Str(t.Name)), c.lineIdx(t.Line))

	case *ast.CallExpression:
		c.expr(t.Callee)
		for _, a := range t.Args {
			c.expr(a)
		}
		c.emit(OP_CALL, uint16(len(t.Args)), c.lineIdx(t.Line))

	case *ast.FunctionExpression:
		code := compileFunc("<anonymous>", t.Params, t.Body)
		c.emit(OP_CLOSURE, c.idx(code), 0)

	case *ast.ArrayLiteral:
		for _, e := range t.Elements {
			c.expr(e)
		}
		c.emit(OP_LIST, uint16(len(t.Elements)), 0)

	case *ast.ObjectLiteral:
		base := uint16(c.keyBase(t.Pairs))
		for _, kv := range t.Pairs {
			c.expr(kv.Value)
		}
		c.emit(OP_DICT, uint16(len(t.Pairs)), base)

	default:
		fail("vm: не умею компилировать expression %T", n)
	}
}

func (c *cstate) keyBase(pairs []ast.KeyValue) int {
	base := len(c.c.Consts)
	for _, kv := range pairs {
		_ = c.rawIdx(val.Str(kv.Key))
	}
	return base
}

func (c *cstate) binary(op string, line int) {
	var opOp Op
	switch op {
	case "+":
		opOp = OP_ADD
	case "-":
		opOp = OP_SUB
	case "*":
		opOp = OP_MUL
	case "/":
		opOp = OP_DIV
	case "%":
		opOp = OP_MOD
	case "^":
		opOp = OP_POW
	case "==":
		c.emit(OP_EQ, 0, 0)
		return
	case "!=":
		c.emit(OP_NEQ, 0, 0)
		return
	case ">":
		c.emit(OP_CMP, 1, c.lineIdx(line))
		return
	case "<":
		c.emit(OP_CMP, 0, c.lineIdx(line))
		return
	case ">=":
		c.emit(OP_CMP, 3, c.lineIdx(line))
		return
	case "<=":
		c.emit(OP_CMP, 2, c.lineIdx(line))
		return
	default:
		fail("vm: неизвестный оператор %s", op)
	}
	c.emit(opOp, 0, c.lineIdx(line))
}

func compileFunc(name string, params []string, body *ast.Block) *Code {
	c := &cstate{c: &Code{Name: name, Params: append([]string(nil), params...)}}
	c.stmt(body, false)
	c.emit(OP_CONST, c.idx(val.Nul), 0)
	c.emit(OP_RET, 0, 0)
	if len(c.c.Ins) > 0xFFFF {
		fail("vm: тело %s слишком большое (pc > 65535)", name)
	}
	c.c.astBody = body
	return c.c
}

func compileBlock(name string, params []string, body *ast.Block) (code *Code, err error) {
	defer func() {
		if r := recover(); r != nil {
			if ce, ok := r.(*compileError); ok {
				code, err = nil, ce
				return
			}
			panic(r)
		}
	}()
	return compileFunc(name, params, body), nil
}
