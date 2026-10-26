package pyops

import (
	"math/big"
	"strings"

	"github.com/name-cho/uniland-go/internal/interp/val"
)

func PyStr(v val.Value) string {
	switch x := v.(type) {
	case nil, val.Null:
		return "None"
	case val.Bool:
		if x {
			return "True"
		}
		return "False"
	case *val.Int:
		return x.N.String()
	case val.Float:
		return val.FloatStr(float64(x))
	case val.Str:
		return string(x)
	case *val.List:
		parts := make([]string, len(x.Items))
		for i, it := range x.Items {
			parts[i] = val.PyRepr(strForRepr(it))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *val.Map:
		parts := make([]string, 0, x.Len())
		for _, kv := range x.Ordered() {
			parts = append(parts, val.PyRepr(kv.K)+": "+val.PyRepr(strForRepr(kv.V)))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *val.Opaque:
		return "<object>"
	default:
		return "<function>"
	}
}

func strForRepr(v val.Value) string {
	switch x := v.(type) {
	case val.Str:
		return string(x)
	case *val.List:
		parts := make([]string, len(x.Items))
		for i, it := range x.Items {
			parts[i] = val.PyRepr(strForRepr(it))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *val.Map:
		parts := make([]string, 0, x.Len())
		for _, kv := range x.Ordered() {
			parts = append(parts, val.PyRepr(kv.K)+": "+val.PyRepr(strForRepr(kv.V)))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return PyStr(v)
	}
}

func IntOf(v val.Value) (*big.Int, error) {
	switch x := v.(type) {
	case val.Bool:
		if x {
			return big.NewInt(1), nil
		}
		return big.NewInt(0), nil
	case *val.Int:
		return x.N, nil
	case val.Float:
		f := new(big.Float).SetFloat64(float64(x))
		n, _ := f.Int(nil)
		return n, nil
	case val.Str:
		s := strings.TrimSpace(string(x))
		n := new(big.Int)
		if _, ok := n.SetString(s, 10); ok {
			return n, nil
		}
		return nil, &NumParseError{Literal: string(x)}
	}
	return nil, &NumParseError{Literal: strForRepr(v)}
}

type NumParseError struct{ Literal string }

func (e *NumParseError) Error() string {
	if e.Literal == "" {
		return "invalid literal for int() with base 10: ''"
	}
	return "invalid literal for int() with base 10: '" + e.Literal + "'"
}

func RangeOf(parts []val.Value) (val.Value, error) {
	get := func(i int) (*big.Int, error) { return IntOf(parts[i]) }
	var start, stop, step *big.Int
	var err error
	switch len(parts) {
	case 1:
		start = big.NewInt(0)
		if stop, err = get(0); err != nil {
			return nil, err
		}
		step = big.NewInt(1)
	case 2:
		if start, err = get(0); err != nil {
			return nil, err
		}
		if stop, err = get(1); err != nil {
			return nil, err
		}
		step = big.NewInt(1)
	case 3:
		if start, err = get(0); err != nil {
			return nil, err
		}
		if stop, err = get(1); err != nil {
			return nil, err
		}
		if step, err = get(2); err != nil {
			return nil, err
		}
		if step.Sign() == 0 {
			return nil, &ZeroStepError{}
		}
	default:
		return nil, nil
	}
	items := []val.Value{}
	sign := step.Sign()
	for cur := new(big.Int).Set(start); ; cur.Add(cur, step) {
		if sign > 0 && cur.Cmp(stop) >= 0 {
			break
		}
		if sign < 0 && cur.Cmp(stop) <= 0 {
			break
		}
		if len(items) > maxRangeItems {
			return nil, &RangeTooLargeError{}
		}
		items = append(items, val.NewInt(new(big.Int).Set(cur)))
	}
	return val.NewList(items), nil
}

const maxRangeItems = 1 << 26

type ZeroStepError struct{}

func (e *ZeroStepError) Error() string { return "range() arg 3 must not be zero" }

type RangeTooLargeError struct{}

func (e *RangeTooLargeError) Error() string { return "range() result too large" }
