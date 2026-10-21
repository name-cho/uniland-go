package vm

import (
	"fmt"
	"strings"

	"github.com/name-cho/uniland-go/internal/ast"
	"github.com/name-cho/uniland-go/internal/interp"
	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/pyops"
	"github.com/name-cho/uniland-go/internal/unierr"
)

type vmIter struct {
	items []val.Value
	pos   int
}

type loopTarget struct {
	savedEnv *interp.Environment
	envDepth int
	endPc    int
	contPc   int
}

type tryState struct {
	meta     *TryMeta
	code     *Code
	savedEnv *interp.Environment
	envDepth int
	stackLen int
	inCatch  bool
}

type envEntry struct{ savedEnv *interp.Environment }

type frame struct {
	code     *Code
	ip       int
	env      *interp.Environment
	stack    []val.Value
	regs     []val.Value
	envStack []envEntry
	loops    []loopTarget
	try      []tryState
	pending  error
}

type vmRetSig struct{ value val.Value }

func (s *vmRetSig) Error() string { return "return" }

type vmBreakSig struct{}

func (s *vmBreakSig) Error() string { return "break" }

type vmContinueSig struct{}

func (s *vmContinueSig) Error() string { return "continue" }

type Engine struct {
	it   *interp.Interp
	echo []string
}

func New(it *interp.Interp) *Engine { return &Engine{it: it} }

func (e *Engine) Call(fn *interp.UniFunction, args []val.Value, line int) (val.Value, error) {
	if len(args) != len(fn.Params) {
		msg := fmt.Sprintf("%s expects %d argument(s), got %d", fn.Name, len(fn.Params), len(args))
		if line >= 0 {
			return nil, unierr.AtLine(msg, line)
		}
		return nil, unierr.New(msg)
	}
	code, err := e.codeFor(fn)
	if err != nil {
		return nil, unierr.New(err.Error())
	}
	callEnv := interp.NewEnvironment(fn.Closure)
	for i, p := range fn.Params {
		callEnv.Define(p, args[i], false)
	}
	f := &frame{code: code, env: callEnv}
	if _, err := e.runFrame(f); err != nil {
		if s, ok := err.(*vmRetSig); ok {
			return s.value, nil
		}
		return nil, err
	}
	return val.Nul, nil
}

func (e *Engine) codeFor(fn *interp.UniFunction) (*Code, error) {
	if c, ok := fn.Compiled.(*Code); ok && c != nil {
		return c, nil
	}
	code, err := compileBlock(fn.Name, fn.Params, fn.Body)
	if err != nil {
		return nil, err
	}
	fn.Compiled = code
	return code, nil
}

func (e *Engine) Interpret(program *ast.Program) error {
	code, err := Compile(program, false)
	if err != nil {
		return unierr.New(err.Error())
	}
	f := &frame{code: code, env: e.it.GlobalEnv}
	if _, err := e.runFrame(f); err != nil {
		return e.topErr(err)
	}
	if v, gerr := e.it.GlobalEnv.Get("main"); gerr == nil {
		if fn, ok := v.(*interp.UniFunction); ok && len(fn.Params) == 0 {
			_, err := e.it.CallValue(fn, nil, -1)
			return e.topErr(err)
		}
	}
	return nil
}

func (e *Engine) EvalProgram(program *ast.Program) (string, error) {
	e.echo = nil
	code, err := Compile(program, true)
	if err != nil {
		return "", unierr.New(err.Error())
	}
	f := &frame{code: code, env: e.it.GlobalEnv}
	if _, err := e.runFrame(f); err != nil {
		return "", e.topErr(err)
	}
	return strings.Join(e.echo, "\n"), nil
}

func (e *Engine) topErr(err error) error {
	switch err.(type) {
	case nil:
		return nil
	case *interp.ExitSignal:
		return err
	case *vmRetSig:
		return unierr.New("return вне функции")
	case *vmBreakSig:
		return unierr.New("break вне цикла")
	case *vmContinueSig:
		return unierr.New("continue вне цикла")
	}
	return e.it.ToTopErr(err)
}

func (e *Engine) runFrame(f *frame) (val.Value, error) {
	code := f.code
	if cap(f.regs) < code.RegN {
		f.regs = make([]val.Value, code.RegN)
	} else {
		f.regs = f.regs[:code.RegN]
		for i := range f.regs {
			f.regs[i] = nil
		}
	}
	for {
		ins := code.Ins[f.ip]
		jumped := false
		var err error

		switch ins.Op {
		case OP_HALT:
			return nil, nil

		case OP_CONST:
			f.stack = append(f.stack, code.Consts[ins.A])

		case OP_POP:
			f.stack = f.stack[:len(f.stack)-1]

		case OP_GETVAR:
			var v val.Value
			v, err = f.env.Get(string(code.Consts[ins.A].(val.Str)))
			f.stack = append(f.stack, v)

		case OP_SETVAR:
			err = f.env.Set(string(code.Consts[ins.A].(val.Str)), f.pop())

		case OP_DEFVAR:
			f.env.Define(string(code.Consts[ins.A].(val.Str)), f.pop(), ins.B == 1)

		case OP_CLOSURE:
			sub := code.Consts[ins.A].(*Code)
			f.stack = append(f.stack, &interp.UniFunction{
				Params: sub.Params, Body: sub.astBody, Compiled: sub,
				Closure: f.env, Name: sub.Name,
			})

		case OP_ECHO:
			if v := f.pop(); v != nil && v != val.Nul {
				e.echo = append(e.echo, pyops.Str(v))
			}

		case OP_TEMPLATE:
			b := strings.Builder{}
			start := len(f.stack) - int(ins.A)
			for i := start; i < len(f.stack); i++ {
				b.WriteString(pyops.Str(f.stack[i]))
			}
			f.stack = f.stack[:start]
			f.stack = append(f.stack, val.Str(b.String()))

		case OP_LIST:
			n := int(ins.A)
			start := len(f.stack) - n
			items := make([]val.Value, n)
			copy(items, f.stack[start:])
			f.stack = f.stack[:start]
			f.stack = append(f.stack, val.NewList(items))

		case OP_DICT:
			n := int(ins.A)
			start := len(f.stack) - n
			m := val.NewMap()
			for i := 0; i < n; i++ {
				m.Set(string(code.Consts[int(ins.B)+i].(val.Str)), f.stack[start+i])
			}
			f.stack = f.stack[:start]
			f.stack = append(f.stack, m)

		case OP_INDEX_GET:
			idx := f.pop()
			obj := f.pop()
			var v val.Value
			v, err = e.it.IndexValue(obj, idx, e.lineOf(code, ins))
			f.stack = append(f.stack, v)

		case OP_INDEX_SET:
			idx := f.pop()
			obj := f.pop()
			v := f.pop()
			err = e.assignIndex(obj, idx, v, e.lineOf(code, ins))

		case OP_MEMBER_GET:
			obj := f.pop()
			name := string(code.Consts[ins.A].(val.Str))
			if m, ok := obj.(*val.Map); ok {
				f.stack = append(f.stack, m.Get(name))
			} else {
				err = unierr.AtLine(fmt.Sprintf("Cannot read field '%s' of %s", name, pyops.TypeOf(obj)), e.lineOf(code, ins))
			}

		case OP_MEMBER_SET:
			obj := f.pop()
			v := f.pop()
			name := string(code.Consts[ins.A].(val.Str))
			if m, ok := obj.(*val.Map); ok {
				m.Set(name, v)
			} else {
				err = unierr.AtLine(fmt.Sprintf("Cannot assign field on %s", pyops.TypeOf(obj)), e.lineOf(code, ins))
			}

		case OP_ADD, OP_SUB, OP_MUL, OP_DIV, OP_MOD, OP_POW:
			rhs := f.pop()
			l := f.pop()
			var v val.Value
			v, err = e.it.ApplyBinary(binaryOpName(ins.Op), l, rhs, e.lineOf(code, ins))
			f.stack = append(f.stack, v)

		case OP_EQ:
			rhs := f.pop()
			l := f.pop()
			f.stack = append(f.stack, val.Bool(pyops.Equal(l, rhs)))

		case OP_NEQ:
			rhs := f.pop()
			l := f.pop()
			f.stack = append(f.stack, val.Bool(!pyops.Equal(l, rhs)))

		case OP_CMP:
			rhs := f.pop()
			l := f.pop()
			var v val.Value
			v, err = e.it.ApplyBinary(cmpOps[ins.A], l, rhs, e.lineOf(code, ins))
			f.stack = append(f.stack, v)

		case OP_NEG:
			v, e2 := pyops.Negate(f.pop())
			if e2 != nil {
				err = e2
			}
			f.stack = append(f.stack, v)

		case OP_NOT:
			f.stack = append(f.stack, val.Bool(!pyops.Truthy(f.pop())))

		case OP_INCR:
			v := f.pop()
			step := int64(1)
			if ins.A == 0 {
				step = -1
			}
			nv, e2 := pyops.Add(v, val.Int64(step))
			f.stack = append(f.stack, nv)
			if e2 != nil {
				if ue, ok := e2.(*unierr.Error); ok {
					err = ue
				} else {
					err = unierr.New(e2.Error())
				}
			}

		case OP_COMPOUND:
			rhs := f.pop()
			current := f.pop()
			var v val.Value
			v, err = e.it.ApplyBinary(compoundOps[ins.A], current, rhs, e.lineOf(code, ins))
			f.stack = append(f.stack, v)

		case OP_AND:
			v := f.pop()
			if !pyops.Truthy(v) {
				f.stack = append(f.stack, v)
				f.ip = int(ins.A)
				jumped = true
			}

		case OP_OR:
			v := f.pop()
			if pyops.Truthy(v) {
				f.stack = append(f.stack, v)
				f.ip = int(ins.A)
				jumped = true
			}

		case OP_JMP:
			f.ip = int(ins.A)
			jumped = true

		case OP_JMP_F:
			if !pyops.Truthy(f.pop()) {
				f.ip = int(ins.A)
				jumped = true
			}

		case OP_RET:

			err = &vmRetSig{value: f.pop()}

		case OP_PUTREG:
			f.regs[ins.A] = f.pop()

		case OP_GETREG:
			v := f.regs[ins.A]
			f.regs[ins.A] = nil
			f.stack = append(f.stack, v)

		case OP_CALL:
			n := int(ins.A)
			args := make([]val.Value, n)
			for i := n - 1; i >= 0; i-- {
				args[i] = f.pop()
			}
			callee := f.pop()
			var v val.Value
			v, err = e.it.CallValue(callee, args, e.lineOf(code, ins))
			f.stack = append(f.stack, v)

		case OP_LOOP_PUSH:
			f.loops = append(f.loops, loopTarget{
				savedEnv: f.env, envDepth: len(f.envStack),
				endPc: int(ins.A), contPc: int(ins.B),
			})

		case OP_LOOP_POP:
			if len(f.loops) == 0 {
				return nil, unierr.New("vm: внутренняя ошибка: LOOP_POP без цикла")
			}
			f.loops = f.loops[:len(f.loops)-1]

		case OP_BREAK:
			err = &vmBreakSig{}

		case OP_CONTINUE:
			err = &vmContinueSig{}

		case OP_TRY:
			f.try = append(f.try, tryState{
				meta: code.Consts[ins.A].(*TryMeta), code: code,
				savedEnv: f.env, envDepth: len(f.envStack),
				stackLen: len(f.stack),
			})

		case OP_TEND:
			st := f.try[len(f.try)-1]
			f.try = f.try[:len(f.try)-1]
			if st.meta.FinallyPc >= 0 {
				f.ip = st.meta.FinallyPc
			} else {
				f.ip = st.meta.AfterPc
			}
			jumped = true

		case OP_CEND:
			st := f.try[len(f.try)-1]
			f.try = f.try[:len(f.try)-1]
			f.env = st.savedEnv
			f.envStack = truncate(f.envStack, st.envDepth)
			if st.meta.FinallyPc >= 0 {
				f.ip = st.meta.FinallyPc
			} else {
				f.ip = st.meta.AfterPc
			}
			jumped = true

		case OP_FEND:
			if f.pending != nil {

				err = f.pending
				f.pending = nil
			} else {
				f.ip = int(e.metaOf(code, ins).AfterPc)
				jumped = true
			}

		case OP_PYBLOCK:
			err = e.it.PyExecBlock(string(code.Consts[ins.A].(val.Str)))

		case OP_GOBLOCK:
			err = e.it.GoExecBlock(string(code.Consts[ins.A].(val.Str)))

		case OP_THROW:
			err = unierr.AtLine(pyops.Str(f.pop()), e.lineOf(code, ins))

		case OP_FORIN_START:
			var items []val.Value
			items, err = e.it.IterableOf(f.pop())
			if err == nil {
				f.stack = append(f.stack, &vmIter{items: items})
			}

		case OP_FORIN_NEXT:
			cur := f.stack[len(f.stack)-1].(*vmIter)
			if cur.pos >= len(cur.items) {
				f.ip = int(ins.A)
				jumped = true
			} else {
				f.stack = append(f.stack, cur.items[cur.pos])
				cur.pos++
			}

		case OP_FORIN_END:
			_ = f.pop()

		case OP_BLOCK_ENTER:
			f.envStack = append(f.envStack, envEntry{savedEnv: f.env})
			f.env = interp.NewEnvironment(f.env)

		case OP_BLOCK_EXIT:
			if len(f.envStack) == 0 {
				return nil, unierr.New("vm: внутренняя ошибка: BLOCK_EXIT без блока")
			}
			f.env = f.envStack[len(f.envStack)-1].savedEnv
			f.envStack = f.envStack[:len(f.envStack)-1]

		default:
			return nil, unierr.New(fmt.Sprintf("vm: неизвестная инструкция %d", ins.Op))
		}

		if err != nil {
			pe := e.failFrame(f, err)
			if pe != nil {
				return nil, pe
			}
			continue
		}
		if !jumped {
			f.ip++
		}
	}
}

func (e *Engine) failFrame(f *frame, err error) error {
	for len(f.try) > 0 {
		st := f.try[len(f.try)-1]
		f.try = f.try[:len(f.try)-1]
		f.stack = f.stack[:st.stackLen]
		f.envStack = truncate(f.envStack, st.envDepth)
		f.env = st.savedEnv
		ue, isUe := err.(*unierr.Error)
		if !st.inCatch {
			if isUe && st.meta.CatchPc >= 0 {

				f.env = interp.NewEnvironment(st.savedEnv)
				if st.meta.CatchVar >= 0 {
					f.env.Define(string(st.code.Consts[st.meta.CatchVar].(val.Str)), val.Str(ue.Msg), false)
				}
				st.inCatch = true
				f.try = append(f.try, st)
				f.ip = st.meta.CatchPc
				return nil
			}
			if st.meta.FinallyPc >= 0 {

				f.pending = err
				f.ip = st.meta.FinallyPc
				return nil
			}

			continue
		}

		if st.meta.FinallyPc >= 0 {
			f.pending = err
			f.ip = st.meta.FinallyPc
			return nil
		}
		continue
	}
	if len(f.loops) > 0 {
		if _, ok := err.(*vmBreakSig); ok {
			t := f.loops[len(f.loops)-1]
			f.env = t.savedEnv
			f.envStack = truncate(f.envStack, t.envDepth)
			f.ip = t.endPc
			return nil
		}
		if _, ok := err.(*vmContinueSig); ok {
			t := f.loops[len(f.loops)-1]
			f.env = t.savedEnv
			f.envStack = truncate(f.envStack, t.envDepth)
			f.ip = t.contPc
			return nil
		}
	}
	return err
}

func (f *frame) pop() val.Value {
	v := f.stack[len(f.stack)-1]
	f.stack = f.stack[:len(f.stack)-1]
	return v
}

func truncate[T any](s []T, n int) []T {
	if n > len(s) {
		n = len(s)
	}
	return s[:n]
}

func (e *Engine) lineOf(code *Code, ins Ins) int {
	return int(code.Consts[ins.B].(*val.Int).N.Int64())
}

func (e *Engine) metaOf(code *Code, ins Ins) *TryMeta {
	return code.Consts[ins.A].(*TryMeta)
}

func (e *Engine) assignIndex(obj, index, v val.Value, line int) error {
	switch o := obj.(type) {
	case *val.List:
		idx, ok := intIndex(index)
		if !ok {
			return unierr.AtLine("List index must be an integer", line)
		}
		if idx < 0 || int(idx) >= len(o.Items) {
			return unierr.AtLine("List index "+bigIdxStr(index)+" out of range", line)
		}
		o.Items[idx] = v
		return nil
	case *val.Map:
		o.Set(pyops.PyStr(index), v)
		return nil
	default:
		return unierr.AtLine(fmt.Sprintf("Cannot assign by index into %s", pyops.TypeOf(obj)), line)
	}
}

func intIndex(v val.Value) (int64, bool) {
	if _, isBool := v.(val.Bool); isBool {
		return 0, false
	}
	n := val.IntVal(v)
	if n == nil {
		return 0, false
	}
	if n.IsInt64() {
		return n.Int64(), true
	}
	if n.Sign() > 0 {
		return 1 << 62, true
	}
	return -(1 << 62), true
}

func bigIdxStr(v val.Value) string {
	n := val.IntVal(v)
	if n == nil {
		return pyops.PyStr(v)
	}
	return n.String()
}

func binaryOpName(op Op) string {
	switch op {
	case OP_ADD:
		return "+"
	case OP_SUB:
		return "-"
	case OP_MUL:
		return "*"
	case OP_DIV:
		return "/"
	case OP_MOD:
		return "%"
	case OP_POW:
		return "^"
	}
	return "?"
}

var cmpOps = [4]string{"<", ">", "<=", ">="}
