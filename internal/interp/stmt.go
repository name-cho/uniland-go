package interp

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/name-cho/uniland-go/internal/ast"
	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/pyops"
	"github.com/name-cho/uniland-go/internal/unierr"
)

func (in *Interp) execute(node ast.Node) error {
	switch n := node.(type) {
	case *ast.Block:
		oldEnv := in.env
		in.env = NewEnvironment(oldEnv)
		defer func() { in.env = oldEnv }()
		for _, stmt := range n.Statements {
			if err := in.execute(stmt); err != nil {
				return err
			}
		}

	case *ast.VariableDeclaration:
		v, err := in.evaluate(n.Value)
		if err != nil {
			return err
		}
		in.env.Define(n.Name, v, n.IsConst)

	case *ast.FunctionDeclaration:
		fn := &UniFunction{Params: n.Params, Body: n.Body, Closure: in.env, Name: n.Name}
		in.env.Define(n.Name, fn, false)

	case *ast.ReturnStatement:
		if n.Value == nil {
			return &returnSignal{Value: val.Nul}
		}
		v, err := in.evaluate(n.Value)
		if err != nil {
			return err
		}
		return &returnSignal{Value: v}

	case *ast.IfStatement:
		cond, err := in.evaluate(n.Condition)
		if err != nil {
			return err
		}
		if pyops.Truthy(cond) {
			return in.execute(n.Then)
		} else if n.Else != nil {
			return in.execute(n.Else)
		}
		return nil

	case *ast.ForLoop:
		if n.Init != nil {
			if err := in.execute(n.Init); err != nil {
				return err
			}
		}
		for {
			cond, err := in.evaluate(n.Condition)
			if err != nil {
				return err
			}
			if !pyops.Truthy(cond) {
				break
			}
			if err := in.execute(n.Body); err != nil {
				if _, ok := err.(*breakSignal); ok {
					break
				}
				if _, ok := err.(*continueSignal); ok {

				} else {
					return err
				}
			}
			if n.Update != nil {
				if err := in.execute(n.Update); err != nil {
					return err
				}
			}
		}
		return nil

	case *ast.ForInLoop:
		iterable, err := in.evaluate(n.Iterable)
		if err != nil {
			return err
		}
		items, err := iterableOf(iterable)
		if err != nil {
			if e, ok := err.(*unierr.Error); ok {
				return e
			}
			return err
		}
		oldEnv := in.env
		for _, item := range items {
			in.env = NewEnvironment(oldEnv)
			in.env.Define(n.Var, item, false)
			err := in.execute(n.Body)
			if err != nil {
				if _, ok := err.(*breakSignal); ok {
					in.env = oldEnv
					return nil
				}
				if _, ok := err.(*continueSignal); ok {
					in.env = oldEnv
					continue
				}
				in.env = oldEnv
				return err
			}
			in.env = oldEnv
		}
		return nil

	case *ast.WhileLoop:
		for {
			cond, err := in.evaluate(n.Condition)
			if err != nil {
				return err
			}
			if !pyops.Truthy(cond) {
				break
			}
			if err := in.execute(n.Body); err != nil {
				if _, ok := err.(*breakSignal); ok {
					break
				}
				if _, ok := err.(*continueSignal); ok {
					continue
				}
				return err
			}
		}
		return nil

	case *ast.BreakStatement:
		return errBreak

	case *ast.ContinueStatement:
		return errContinue

	case *ast.Assignment:
		return in.execAssignment(n)

	case *ast.ExpressionStatement:
		_, err := in.evaluate(n.Expr)
		return err

	case *ast.PythonBlock:
		if in.PyBridge == nil {
			return unierr.New("Python worker недоступен: интерпретатор не подключён")
		}
		if err := in.PyBridge.Exec(dedent(n.Code)); err != nil {
			return wrapBridgeErr("pyexec", err)
		}
		return nil

	case *ast.GoBlock:
		if in.GoBridge == nil {
			return unierr.New("Go worker недоступен: интерпретатор не подключён")
		}
		if err := in.GoBridge.Exec(dedent(n.Code)); err != nil {
			return wrapBridgeErr("goexec", err)
		}
		return nil

	case *ast.TryCatch:
		return in.execTryCatch(n)

	case *ast.ThrowStatement:
		v, err := in.evaluate(n.Value)
		if err != nil {
			return err
		}
		return unierr.AtLine(pyops.Str(v), n.Line)

	default:
		return unierr.New(fmt.Sprintf("Cannot execute node: %s", nodeName(node)))
	}
	return nil
}

func (in *Interp) execTryCatch(n *ast.TryCatch) error {
	tryErr := func() error {
		if err := in.execute(n.Try); err != nil {
			if e, ok := err.(*unierr.Error); ok {
				return e
			}
			return err
		}
		return nil
	}()

	if tryErr == nil {

		if n.Finally != nil {
			if err := in.execute(n.Finally); err != nil {
				if e, ok := err.(*unierr.Error); ok {
					return e
				}
				return err
			}
		}
		return nil
	}

	ue, caught := tryErr.(*unierr.Error)
	if !caught {

		if n.Finally != nil {
			if err := in.execute(n.Finally); err != nil {
				return err
			}
		}
		return tryErr
	}

	if n.Catch != nil {
		oldEnv := in.env
		in.env = NewEnvironment(oldEnv)
		if n.CatchVar != nil {
			in.env.Define(*n.CatchVar, val.Str(ue.Msg), false)
		}
		catchErr := in.execute(n.Catch)
		in.env = oldEnv
		if n.Finally != nil {
			if err := in.execute(n.Finally); err != nil {
				if e, ok := err.(*unierr.Error); ok {
					return e
				}
				return err
			}
		}
		if catchErr != nil {
			if e, ok := catchErr.(*unierr.Error); ok {
				return e
			}
			return catchErr
		}
		return nil
	}

	if n.Finally != nil {
		if err := in.execute(n.Finally); err != nil {
			if e, ok := err.(*unierr.Error); ok {
				return e
			}
			return err
		}
	}
	return tryErr
}

func (in *Interp) execAssignment(n *ast.Assignment) error {
	var newValue val.Value
	switch n.Op {
	case "++", "--":
		current, err := in.evaluate(n.Target)
		if err != nil {
			return err
		}
		step := big.NewInt(1)
		if n.Op == "--" {
			step = big.NewInt(-1)
		}
		newValue, err = pyops.Add(current, val.NewInt(step))
		if err != nil {
			if e, ok := err.(*unierr.Error); ok {
				return e
			}
			return unierr.New(err.Error())
		}
	case "=":
		v, err := in.evaluate(n.Value)
		if err != nil {
			return err
		}
		newValue = v
	default:
		current, err := in.evaluate(n.Target)
		if err != nil {
			return err
		}
		rhs, err2 := in.evaluate(n.Value)
		if err2 != nil {
			return err2
		}
		newValue, err = in.applyBinary(opChar(n.Op), current, rhs, n.Line)
		if err != nil {
			return err
		}
	}
	return in.assignTo(n.Target, newValue, n.Line)
}

func opChar(op string) string { return op[:1] }

func (in *Interp) assignTo(target ast.Node, value val.Value, line int) error {
	switch t := target.(type) {
	case *ast.Identifier:
		if err := in.env.Set(t.Name, value); err != nil {
			return err
		}
		return nil
	case *ast.IndexExpression:
		obj, err := in.evaluate(t.Obj)
		if err != nil {
			return err
		}
		index, err2 := in.evaluate(t.Index)
		if err2 != nil {
			return err2
		}
		switch o := obj.(type) {
		case *val.List:
			idx, ok := intIndex(index)
			if !ok {
				return unierr.AtLine("List index must be an integer", line)
			}
			if idx < 0 || int(idx) >= len(o.Items) {
				return unierr.AtLine("List index "+bigIdxStr(index)+" out of range", line)
			}
			o.Items[idx] = value
			return nil
		case *val.Map:
			o.Set(pyops.PyStr(index), value)
			return nil
		default:
			return unierr.AtLine(fmt.Sprintf("Cannot assign by index into %s", pyTypeOf(obj)), line)
		}
	case *ast.MemberExpression:
		obj, err := in.evaluate(t.Obj)
		if err != nil {
			return err
		}
		if m, ok := obj.(*val.Map); ok {
			m.Set(t.Name, value)
			return nil
		}
		return unierr.AtLine(fmt.Sprintf("Cannot assign field on %s", pyTypeOf(obj)), line)
	}
	return unierr.AtLine("Invalid assignment target", line)
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

func iterableOf(v val.Value) ([]val.Value, error) {
	switch x := v.(type) {
	case *val.List:
		items := make([]val.Value, len(x.Items))
		copy(items, x.Items)
		return items, nil
	case val.Str:
		items := []val.Value{}
		for _, r := range []rune(string(x)) {
			items = append(items, val.Str(string(r)))
		}
		return items, nil
	case *val.Map:
		items := []val.Value{}
		for _, k := range x.Keys() {
			items = append(items, val.Str(k))
		}
		return items, nil
	}
	return nil, unierr.New(fmt.Sprintf("Cannot iterate over %s", pyTypeOf(v)))
}

func nodeName(node ast.Node) string {
	s := strings.TrimPrefix(strings.TrimPrefix(fmt.Sprintf("%T", node), "*ast."), "*interp.")
	return s
}

func dedent(s string) string {
	expanded := strings.Builder{}
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := 8 - col%8
			expanded.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		if r == '\n' {
			col = 0
			expanded.WriteRune(r)
			continue
		}
		col++
		expanded.WriteRune(r)
	}
	lines := strings.Split(expanded.String(), "\n")
	margin := -1
	for i, line := range lines {
		if i == 0 {
			continue
		}
		trimmed := len(line) - len(strings.TrimLeft(line, " "))
		if trimmed > 0 {
			if margin == -1 || trimmed < margin {
				margin = trimmed
			}
		}
	}
	if margin <= 0 {
		return s
	}
	for i, line := range lines {
		if i == 0 {
			continue
		}
		if len(line) >= margin {
			allSpaces := true
			for j := 0; j < margin; j++ {
				if line[j] != ' ' {
					allSpaces = false
					break
				}
			}
			if allSpaces {
				lines[i] = line[margin:]
			}
		}
	}
	return strings.Join(lines, "\n")
}
