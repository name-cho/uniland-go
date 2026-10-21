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

func (in *Interp) evaluate(node ast.Node) (val.Value, error) {
	switch n := node.(type) {
	case *ast.Literal:
		return n.Value, nil

	case *ast.TemplateLiteral:
		var b strings.Builder
		for _, part := range n.Parts {
			if !part.IsExpr {
				b.WriteString(part.Str)
				continue
			}
			v, err := in.evaluate(part.Expr)
			if err != nil {
				return nil, err
			}
			b.WriteString(pyops.Str(v))
		}
		return val.Str(b.String()), nil

	case *ast.Identifier:
		return in.env.Get(n.Name)

	case *ast.BinaryOp:
		if n.Op == "&&" {
			left, err := in.evaluate(n.Left)
			if err != nil {
				return nil, err
			}
			if !pyops.Truthy(left) {
				return left, nil
			}
			return in.evaluate(n.Right)
		}
		if n.Op == "||" {
			left, err := in.evaluate(n.Left)
			if err != nil {
				return nil, err
			}
			if pyops.Truthy(left) {
				return left, nil
			}
			return in.evaluate(n.Right)
		}
		left, err := in.evaluate(n.Left)
		if err != nil {
			return nil, err
		}
		right, err2 := in.evaluate(n.Right)
		if err2 != nil {
			return nil, err2
		}
		return in.applyBinary(n.Op, left, right, n.Line)

	case *ast.UnaryOp:
		v, err := in.evaluate(n.Expr)
		if err != nil {
			return nil, err
		}
		switch n.Op {
		case "-":
			return pyops.Negate(v)
		case "!":
			return val.Bool(!pyops.Truthy(v)), nil
		}
		return nil, unierr.AtLine(fmt.Sprintf("Unknown unary operator %s", n.Op), n.Line)

	case *ast.IndexExpression:
		obj, err := in.evaluate(n.Obj)
		if err != nil {
			return nil, err
		}
		index, err2 := in.evaluate(n.Index)
		if err2 != nil {
			return nil, err2
		}
		return in.indexValue(obj, index, n.Line)

	case *ast.MemberExpression:
		obj, err := in.evaluate(n.Obj)
		if err != nil {
			return nil, err
		}
		if m, ok := obj.(*val.Map); ok {
			return m.Get(n.Name), nil
		}
		return nil, unierr.AtLine(fmt.Sprintf("Cannot read field '%s' of %s", n.Name, pyTypeOf(obj)), n.Line)

	case *ast.CallExpression:
		fnVal, err := in.evaluate(n.Callee)
		if err != nil {
			return nil, err
		}
		args := make([]val.Value, len(n.Args))
		for i, a := range n.Args {
			args[i], err = in.evaluate(a)
			if err != nil {
				return nil, err
			}
		}
		return in.callValue(fnVal, args, n.Line)

	case *ast.FunctionExpression:
		return &UniFunction{Params: n.Params, Body: n.Body, Closure: in.env, Name: "<anonymous>"}, nil

	case *ast.ArrayLiteral:
		items := make([]val.Value, len(n.Elements))
		for i, e := range n.Elements {
			v, err := in.evaluate(e)
			if err != nil {
				return nil, err
			}
			items[i] = v
		}
		return val.NewList(items), nil

	case *ast.ObjectLiteral:
		obj := val.NewMap()
		for _, kv := range n.Pairs {
			v, err := in.evaluate(kv.Value)
			if err != nil {
				return nil, err
			}
			obj.Set(kv.Key, v)
		}
		return obj, nil
	}
	return nil, unierr.New(fmt.Sprintf("Cannot evaluate node: %s", nodeName(node)))
}

func (in *Interp) indexValue(obj, index val.Value, line int) (val.Value, error) {
	switch o := obj.(type) {
	case *val.List:
		idx, ok := intIndex(index)
		if !ok {
			return nil, unierr.AtLine("List index must be an integer", line)
		}
		if idx < 0 {
			idx += int64(len(o.Items))
		}
		if idx < 0 || idx >= int64(len(o.Items)) {
			return nil, unierr.AtLine(fmt.Sprintf("List index out of range (length %d)", len(o.Items)), line)
		}
		return o.Items[idx], nil
	case val.Str:
		idx, ok := intIndex(index)
		if !ok {
			return nil, unierr.AtLine("String index must be an integer", line)
		}
		rs := []rune(string(o))
		if idx < 0 {
			idx += int64(len(rs))
		}
		if idx < 0 || idx >= int64(len(rs)) {
			return nil, unierr.AtLine("String index out of range", line)
		}
		return val.Str(string(rs[idx])), nil
	case *val.Map:
		return o.Get(pyops.PyStr(index)), nil
	}
	return nil, unierr.AtLine(fmt.Sprintf("Cannot index %s", pyTypeOf(obj)), line)
}

func (in *Interp) applyBinary(op string, left, right val.Value, line int) (val.Value, error) {
	switch op {
	case "+":
		if _, lok := left.(val.Str); lok {
			return val.Str(pyops.Str(left) + pyops.Str(right)), nil
		}
		if _, rok := right.(val.Str); rok {
			return val.Str(pyops.Str(left) + pyops.Str(right)), nil
		}
		l, lok := left.(*val.List)
		r, rok := right.(*val.List)
		if lok && rok {
			items := make([]val.Value, 0, len(l.Items)+len(r.Items))
			items = append(items, l.Items...)
			items = append(items, r.Items...)
			return val.NewList(items), nil
		}
		return in.arith(pyops.Add, op, left, right, line)
	case "-":
		return in.arith(pyops.Sub, op, left, right, line)
	case "*":
		return in.arith(pyops.Mul, op, left, right, line)
	case "/":

		if pyops.IsZeroNum(right) {
			return nil, unierr.AtLine("Division by zero", line)
		}
		return in.arith(pyops.Div, op, left, right, line)
	case "%":
		return in.arith(pyops.Mod, op, left, right, line)
	case "^":
		return in.arith(pyops.Pow, op, left, right, line)
	case "==":
		return val.Bool(pyops.Equal(left, right)), nil
	case "!=":
		return val.Bool(!pyops.Equal(left, right)), nil
	case ">", "<", ">=", "<=":
		b, err := pyops.Cmp(op, left, right)
		if err != nil {
			return nil, unierr.AtLine(err.Error(), line)
		}
		return val.Bool(b), nil
	}
	return nil, unierr.AtLine(fmt.Sprintf("Unknown operator %s", op), line)
}

func (in *Interp) arith(f func(a, b val.Value) (val.Value, error), op string, left, right val.Value, line int) (val.Value, error) {
	r, err := f(left, right)
	if err != nil {
		if e, ok := err.(*unierr.Error); ok {
			return r, e
		}
		return r, unierr.New(err.Error())
	}
	return r, nil
}

var _ big.Int = big.Int{}
