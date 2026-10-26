package pyops

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strings"

	"github.com/name-cho/uniland-go/internal/interp/val"
)

func TypeName(v val.Value) string {
	switch v.(type) {
	case nil, val.Null:
		return "NoneType"
	case val.Bool:
		return "bool"
	case *val.Int:
		return "int"
	case val.Float:
		return "float"
	case val.Str:
		return "str"
	case *val.List:
		return "list"
	case *val.Map:
		return "dict"
	case *val.Opaque:
		return "object"
	default:
		return "function"
	}
}

func TypeOf(v val.Value) string {
	switch v.(type) {
	case nil, val.Null:
		return "null"
	case val.Bool:
		return "boolean"
	case *val.Int, val.Float:
		return "number"
	case val.Str:
		return "string"
	case *val.List:
		return "array"
	case *val.Map:
		return "object"
	case *val.Opaque:
		return "unknown"
	default:
		return "function"
	}
}

func Inner(v val.Value) string {
	switch x := v.(type) {
	case nil, val.Null:
		return "null"
	case val.Bool:
		if x {
			return "true"
		}
		return "false"
	case *val.Int:
		return x.N.String()
	case val.Float:
		return val.NumStr(float64(x))
	case val.Str:
		return quo(string(x))
	case *val.List:
		parts := make([]string, len(x.Items))
		for i, it := range x.Items {
			parts[i] = Inner(it)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *val.Map:
		parts := make([]string, 0, x.Len())
		for _, kv := range x.Ordered() {
			parts = append(parts, quo(kv.K)+": "+Inner(kv.V))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *val.Opaque:
		return "<object>"
	default:
		return "<function>"
	}
}

func quo(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

func Str(v val.Value) string {
	switch x := v.(type) {
	case nil, val.Null:
		return "null"
	case val.Bool:
		if x {
			return "true"
		}
		return "false"
	case *val.Int:
		return x.N.String()
	case val.Float:
		return val.NumStr(float64(x))
	case val.Str:
		return string(x)
	case *val.List, *val.Map:
		return Inner(v)
	case *val.Opaque:
		return "<object>"
	default:
		return "<function>"
	}
}

func Truthy(v val.Value) bool {
	switch x := v.(type) {
	case nil, val.Null:
		return false
	case val.Bool:
		return bool(x)
	case *val.Int:
		return x.N.Sign() != 0
	case val.Float:
		return float64(x) != 0
	case val.Str:
		return len(string(x)) > 0
	case *val.List:
		return len(x.Items) > 0
	case *val.Map:
		return x.Len() > 0
	default:
		return true
	}
}

func bigToFloat(n *big.Int) (float64, error) {
	f := new(big.Float).SetInt(n)
	res, _ := f.Float64()
	if math.IsInf(res, 0) {
		return 0, fmt.Errorf("int too large to convert to float")
	}
	return res, nil
}

func asRat(v val.Value) (*big.Rat, bool, bool) {
	f, ok := v.(val.Float)
	if ok {
		if math.IsNaN(float64(f)) {
			return nil, true, true
		}
		if math.IsInf(float64(f), 0) {
			return nil, true, true
		}
		r := new(big.Rat).SetFloat64(float64(f))
		return r, true, false
	}
	if n := val.IntVal(v); n != nil {
		return new(big.Rat).SetInt(n), true, false
	}
	return nil, false, false
}

func notNumbers(a, b val.Value, op string) error {
	return fmt.Errorf("unsupported operand type(s) for %s: '%s' and '%s'", op, TypeName(a), TypeName(b))
}

func Add(a, b val.Value) (val.Value, error) {
	ai := val.IntVal(a)
	bi := val.IntVal(b)
	if ai != nil && bi != nil {
		return val.NewInt(new(big.Int).Add(ai, bi)), nil
	}

	ra, aNum, aSpec := asRat(a)
	rb, bNum, bSpec := asRat(b)
	if aNum && bNum {
		if aSpec || bSpec {
			return val.NewFloat(math.NaN()), nil
		}
		f, _ := new(big.Rat).Add(ra, rb).Float64()
		return val.NewFloat(f), nil
	}
	if ok1, ok2 := asFloatOK(a), asFloatOK(b); ok1 != ok2 {
		return nil, notNumbers(a, b, "+")
	}
	return nil, notNumbers(a, b, "+")
}

func asFloatOK(v val.Value) bool {
	switch v.(type) {
	case val.Bool, *val.Int, val.Float:
		return true
	}
	return false
}

func Sub(a, b val.Value) (val.Value, error) {
	ai := val.IntVal(a)
	bi := val.IntVal(b)
	if ai != nil && bi != nil {
		return val.NewInt(new(big.Int).Sub(ai, bi)), nil
	}
	ra, aNum, aSpec := asRat(a)
	rb, bNum, bSpec := asRat(b)
	if aNum && bNum {
		if aSpec || bSpec {
			return val.NewFloat(math.NaN()), nil
		}
		f, _ := new(big.Rat).Sub(ra, rb).Float64()
		return val.NewFloat(f), nil
	}
	return nil, notNumbers(a, b, "-")
}

func Mul(a, b val.Value) (val.Value, error) {
	as, aIsStr := a.(val.Str)
	bs, bIsStr := b.(val.Str)
	al, aIsList := a.(*val.List)
	bl, bIsList := b.(*val.List)
	an := val.IntVal(a)
	bn := val.IntVal(b)

	seq, rhs := a, b
	if aIsStr || aIsList {
		seq, rhs = a, b
	} else if bIsStr || bIsList {
		seq, rhs = b, a
	} else {
		seq = nil
	}
	if seq != nil {
		if bnOfRhs := val.IntVal(rhs); bnOfRhs != nil {
			if _, isStr := seq.(val.Str); isStr {
				return val.Str(strings.Repeat(string(seq.(val.Str)), RepeatCount(bnOfRhs))), nil
			}
			return listRepeat(seq.(*val.List), bnOfRhs)
		}

		return nil, fmt.Errorf("can't multiply sequence by non-int of type '%s'", TypeName(rhs))
	}
	_ = as
	_ = bs
	_ = al
	_ = bl

	if an != nil && bn != nil {
		return val.NewInt(new(big.Int).Mul(an, bn)), nil
	}
	ra, aNum, _ := asRat(a)
	rb, bNum, _ := asRat(b)
	if aNum && bNum {
		f, _ := new(big.Rat).Mul(ra, rb).Float64()
		return val.NewFloat(f), nil
	}
	return nil, notNumbers(a, b, "*")
}

func RepeatCount(n *big.Int) int {
	if n.Sign() <= 0 {
		return 0
	}
	if n.IsInt64() {
		c := n.Int64()
		maxLen := int64(1 << 31)
		if c > maxLen {
			n.SetInt64(maxLen)
			return int(maxLen)
		}
		return int(c)
	}
	return int(maxRepeat)
}

const maxRepeat = int64(1 << 31)

func listRepeat(l *val.List, n *big.Int) (val.Value, error) {
	c := n.Int64()
	if n.Sign() <= 0 && !n.IsInt64() {
		c = 0
	} else if n.Sign() <= 0 {
		c = 0
	}
	if c <= 0 {
		return val.NewList(nil), nil
	}
	if d := len(l.Items); d > 0 {
		if c > maxRepeat/int64(d) {
			return nil, fmt.Errorf(":%s", "repetition too large")
		}
	}
	out := make([]val.Value, 0, len(l.Items)*int(c))
	for i := int64(0); i < c; i++ {
		out = append(out, l.Items...)
	}
	return val.NewList(out), nil
}

func Div(a, b val.Value) (val.Value, error) {
	ra, aNum, _ := asRat(a)
	rb, bNum, _ := asRat(b)
	if aNum && bNum {
		f, _ := new(big.Rat).Quo(ra, rb).Float64()
		if math.IsInf(f, 0) {
			return nil, fmt.Errorf("integer division result too large for a float")
		}
		return val.NewFloat(f), nil
	}
	return nil, notNumbers(a, b, "/")
}

func IsZeroNum(v val.Value) bool {
	r, num, spec := asRat(v)
	if !num || spec {
		return false
	}
	return r.Sign() == 0
}

func Mod(a, b val.Value) (val.Value, error) {
	ai := val.IntVal(a)
	bi := val.IntVal(b)
	if ai != nil && bi != nil {
		if bi.Sign() == 0 {
			return nil, fmt.Errorf("integer division or modulo by zero")
		}
		r := new(big.Int).Rem(ai, bi)
		if r.Sign() != 0 && (r.Sign() < 0) != (bi.Sign() < 0) {
			r.Add(r, bi)
		}
		return val.NewInt(r), nil
	}
	af, ok1, _ := asFloatSoft(a)
	bf, ok2, _ := asFloatSoft(b)
	if ok1 && ok2 {
		if bf == 0 {
			return nil, fmt.Errorf("float modulo")
		}
		r := math.Mod(af, bf)
		if r != 0 && (r < 0) != (bf < 0) {
			r += bf
		}
		return val.NewFloat(r), nil
	}
	return nil, notNumbers(a, b, "%")
}

func asFloatSoft(v val.Value) (float64, bool, bool) {
	switch x := v.(type) {
	case val.Bool:
		if x {
			return 1, true, false
		}
		return 0, true, false
	case *val.Int:
		f, ok := bigToFloat(x.N)
		if ok != nil {
			return 0, false, true
		}
		return f, true, false
	case val.Float:
		return float64(x), true, false
	}
	return 0, false, false
}

func Pow(a, b val.Value) (val.Value, error) {
	ai := val.IntVal(a)
	bi := val.IntVal(b)
	if ai != nil && bi != nil {
		if ai.Sign() == 0 && bi.Sign() > 0 {
			return val.Int64(0), nil
		}
		if ai.Sign() == 0 && bi.Sign() == 0 {
			return val.Int64(1), nil
		}
		if bi.Sign() < 0 {
			if ai.Sign() == 0 {
				return nil, fmt.Errorf("0.0 cannot be raised to a negative power")
			}
			af, err := bigToFloat(ai)
			if err != nil {
				return nil, err
			}
			bf, _ := new(big.Float).SetInt(bi).Float64()
			return val.NewFloat(math.Pow(af, bf)), nil
		}
		if ai.Sign() == 1 && ai.Cmp(big.NewInt(1)) == 0 {
			return val.Int64(1), nil
		}
		if ai.Sign() == -1 && ai.IsInt64() && ai.Int64() == -1 {
			if bi.Bit(0) == 1 {
				return val.Int64(-1), nil
			}
			return val.Int64(1), nil
		}

		est := uint64(ai.BitLen()) * uint64(bi.BitLen())
		if est > 1<<22 {
			return nil, fmt.Errorf("exponent too large")
		}
		res := new(big.Int).Exp(ai, bi, nil)
		return val.NewInt(res), nil
	}
	af, ok1, _ := asFloatSoft(a)
	bf, ok2, _ := asFloatSoft(b)
	if ok1 && ok2 {
		if af == 0 && bf < 0 {
			return nil, fmt.Errorf("0.0 cannot be raised to a negative power")
		}
		if af < 0 && bf != math.Trunc(bf) {
			return nil, fmt.Errorf("negative number cannot be raised to a fractional power")
		}
		return val.NewFloat(math.Pow(af, bf)), nil
	}
	return nil, notNumbers(a, b, "**")
}

func Negate(v val.Value) (val.Value, error) {
	if n := val.IntVal(v); n != nil {
		return val.NewInt(new(big.Int).Neg(n)), nil
	}
	if f, ok := v.(val.Float); ok {
		return val.NewFloat(-float64(f)), nil
	}
	return nil, fmt.Errorf("bad operand type for unary -: '%s'", TypeName(v))
}

func Abs(v val.Value) (val.Value, error) {
	switch x := v.(type) {
	case val.Bool:
		if x {
			return val.Int64(1), nil
		}
		return val.Int64(0), nil
	case *val.Int:
		return val.NewInt(new(big.Int).Abs(x.N)), nil
	case val.Float:
		return val.NewFloat(math.Abs(float64(x))), nil
	}
	if _, ok, _ := asFloatSoft(v); ok {
		if n := val.IntVal(v); n != nil {
			return val.NewInt(new(big.Int).Abs(n)), nil
		}
	}
	return nil, fmt.Errorf("bad operand type for abs(): '%s'", TypeName(v))
}

func Equal(a, b val.Value) bool {
	if isNull(a) && isNull(b) {
		return true
	}
	if isNull(a) != isNull(b) {
		return false
	}
	ra, aNum, aSpec := asRat(a)
	rb, bNum, bSpec := asRat(b)
	if aNum && bNum {
		if aSpec || bSpec {
			return infOrNaNEqual(a, b)
		}
		return ra.Cmp(rb) == 0
	}
	if aNum != bNum {
		return false
	}
	as, aStr := a.(val.Str)
	bs, bStr := b.(val.Str)
	if aStr && bStr {
		return as == bs
	}
	if aStr != bStr {
		return false
	}
	al, aL := a.(*val.List)
	bl, bL := b.(*val.List)
	if aL && bL {
		if len(al.Items) != len(bl.Items) {
			return false
		}
		for i := range al.Items {
			if !Equal(al.Items[i], bl.Items[i]) {
				return false
			}
		}
		return true
	}
	if aL != bL {
		return false
	}
	am, aM := a.(*val.Map)
	bm, bM := b.(*val.Map)
	if aM && bM {
		if am.Len() != bm.Len() {
			return false
		}
		for _, kv := range am.Ordered() {
			bv, ok := bm.GetOK(kv.K)
			if !ok || !Equal(kv.V, bv) {
				return false
			}
		}
		return true
	}
	if aM != bM {
		return false
	}

	return valueIdentity(a, b)
}

func isNull(v val.Value) bool {
	if v == nil {
		return true
	}
	_, ok := v.(val.Null)
	return ok
}

func infOrNaNEqual(a, b val.Value) bool {
	fa, _ := a.(val.Float)
	fb, _ := b.(val.Float)
	if math.IsNaN(float64(fa)) || math.IsNaN(float64(fb)) {
		return false
	}
	return float64(fa) == float64(fb)
}

func Cmp(op string, a, b val.Value) (bool, error) {
	switch op {
	case "<":
		return rawLess(a, b)
	case ">":
		return lessSwap(a, b)
	case "<=":
		lt, err := rawLess(a, b)
		if err != nil {
			return false, err
		}
		if lt {
			return true, nil
		}
		return Equal(a, b), nil
	case ">=":
		gt, err := lessSwap(a, b)
		if err != nil {
			return false, err
		}
		if gt {
			return true, nil
		}
		return Equal(a, b), nil
	}
	return false, fmt.Errorf("Unknown operator %s", op)
}

func lessSwap(a, b val.Value) (bool, error) {
	lt, err := rawLess(b, a)
	if err != nil {
		return false, err
	}
	return lt, nil
}

func Less(a, b val.Value) (bool, error) {
	return rawLess(a, b)
}

func rawLess(a, b val.Value) (bool, error) {
	ra, aNum, aSpec := asRat(a)
	rb, bNum, bSpec := asRat(b)
	if aNum && bNum {
		if aSpec || bSpec {
			return infNanLess(a, b)
		}
		return ra.Cmp(rb) < 0, nil
	}
	if aNum != bNum {
		return false, notComparable("<", a, b)
	}
	as, aStr := a.(val.Str)
	bs, bStr := b.(val.Str)
	if aStr && bStr {
		return as < bs, nil
	}
	al, aL := a.(*val.List)
	bl, bL := b.(*val.List)
	if aL && bL {
		n := len(al.Items)
		if len(bl.Items) < n {
			n = len(bl.Items)
		}
		for i := 0; i < n; i++ {
			if !Equal(al.Items[i], bl.Items[i]) {
				return rawLess(al.Items[i], bl.Items[i])
			}
		}
		return len(al.Items) < len(bl.Items), nil
	}
	return false, notComparable("<", a, b)
}

func infNanLess(a, b val.Value) (bool, error) {
	fa, aIsF := a.(val.Float)
	fb, bIsF := b.(val.Float)
	if !aIsF || !bIsF {
		if math.IsNaN(float64(fa)) || a == nil {
			return false, nil
		}
		if math.IsNaN(float64(fb)) {
			return false, nil
		}
	}
	nanA := math.IsNaN(float64(fa))
	nanB := math.IsNaN(float64(fb))
	if nanA || nanB {
		return false, nil
	}
	return float64(fa) < float64(fb), nil
}

func notComparable(op string, a, b val.Value) error {
	return fmt.Errorf("'%s' not supported between instances of '%s' and '%s'",
		op, TypeName(a), TypeName(b))
}

func valueIdentity(a, b val.Value) bool {
	if a == nil || b == nil {
		return false
	}
	ar := reflect.ValueOf(a)
	br := reflect.ValueOf(b)
	if ar.Kind() != br.Kind() {
		return false
	}
	switch ar.Kind() {
	case reflect.Ptr, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return ar.Pointer() == br.Pointer()
	}
	return false
}

func SortStable(items []val.Value) error {
	var sortErr error
	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(x, y int) bool {
		lt, err := rawLess(items[idx[x]], items[idx[y]])
		if err != nil {
			if sortErr == nil {
				sortErr = err
			}
			return false
		}
		return lt
	})
	if sortErr != nil {
		return fmt.Errorf("sort() cannot compare these elements")
	}
	out := make([]val.Value, len(items))
	for i, j := range idx {
		out[i] = items[j]
	}
	copy(items, out)
	return nil
}
