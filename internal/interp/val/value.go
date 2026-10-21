package val

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

type Value = any

type Int struct{ N *big.Int }

func NewInt(n *big.Int) Value { return &Int{N: n} }

func Int64(v int64) Value { return &Int{N: big.NewInt(v)} }

func IntFromString(s string) Value {
	n := new(big.Int)
	_, ok := n.SetString(s, 10)
	if !ok {
		n.SetInt64(0)
	}
	return &Int{N: n}
}

type Float float64

func NewFloat(f float64) Value { return Float(f) }

type Str string
type Bool bool
type Null struct{}

var Nul Value = Null{}

type List struct{ Items []Value }

func NewList(items []Value) Value { return &List{Items: items} }

type KV struct {
	K string
	V Value
}

type Map struct {
	m    map[string]Value
	keys []string
}

func NewMap() *Map { return &Map{m: map[string]Value{}} }

func (m *Map) Set(k string, v Value) {
	if _, ok := m.m[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.m[k] = v
}

func (m *Map) Get(k string) Value { return m.m[k] }

func (m *Map) GetOK(k string) (Value, bool) {
	v, ok := m.m[k]
	return v, ok
}

func (m *Map) Len() int { return len(m.m) }

func (m *Map) Keys() []string { return append([]string(nil), m.keys...) }

func (m *Map) Ordered() []KV {
	out := make([]KV, 0, len(m.m))
	for _, k := range m.keys {
		out = append(out, KV{k, m.m[k]})
	}
	return out
}

func AsNum(v Value) (Value, bool) {
	switch x := v.(type) {
	case *Int, Float:
		return v, true
	case Bool:
		if x {
			return Int64(1), true
		}
		return Int64(0), true
	}
	return nil, false
}

func IntVal(v Value) *big.Int {
	if n, ok := AsNum(v); ok {
		if iv, ok := n.(*Int); ok {
			return iv.N
		}
	}
	return nil
}

func BoolOf(v Value) (bool, bool) { b, ok := v.(Bool); return bool(b), ok }

func FloatStr(f float64) string {
	if math.IsNaN(f) {
		return "nan"
	}
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	ei := strings.IndexByte(s, 'e')
	mant, expStr := s[:ei], s[ei+1:]
	exp, _ := strconv.Atoi(expStr)
	digits := strings.ReplaceAll(mant, ".", "")
	digits = strings.TrimRight(digits, "0")
	if digits == "" {
		digits = "0"
	}
	sign := ""
	if neg {
		sign = "-"
	}
	if exp < -4 || exp >= 16 {
		m := digits[:1]
		if len(digits) > 1 {
			m += "." + digits[1:]
		}
		es := fmt.Sprintf("%02d", exp)
		if exp < 0 {
			es = "-" + es[1:]
		} else {
			es = "+" + es
		}
		return sign + m + "e" + es
	}
	switch {
	case exp >= len(digits):
		return sign + digits + strings.Repeat("0", exp-len(digits)) + ".0"
	case exp >= 0:
		return sign + digits[:exp+1] + "." + digits[exp+1:]
	default:
		return sign + "0." + strings.Repeat("0", -exp-1) + digits
	}
}

func NumStr(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return FloatStr(f)
	}
	return strconv.FormatFloat(f, 'f', 0, 64)
}

func IntOrFloat(s string) Value {
	if strings.Contains(s, ".") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return NewFloat(math.NaN())
		}
		return NewFloat(f)
	}
	n := new(big.Int)
	if _, ok := n.SetString(s, 10); !ok {
		n.SetInt64(0)
	}
	return NewInt(n)
}

func PyRound(f float64, ndigits *int) Value {
	if ndigits == nil {
		r := math.RoundToEven(f)
		if math.IsNaN(r) {
			return NewFloat(r)
		}
		return Int64(int64(r))
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return NewFloat(f)
	}
	d := *ndigits
	rat := new(big.Rat).SetFloat64(f)
	if d >= 0 {
		scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(d)), nil)
		num := new(big.Rat).Mul(rat, new(big.Rat).SetInt(scale))
		q := roundHalfEvenRat(num)
		res := new(big.Rat).Quo(new(big.Rat).SetInt(q), new(big.Rat).SetInt(scale))
		fl, _ := res.Float64()
		return NewFloat(fl)
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-d)), nil)
	q := roundHalfEvenRat(new(big.Rat).Quo(rat, new(big.Rat).SetInt(scale)))
	n := new(big.Int).Mul(q, scale)
	return NewInt(n)
}

func roundHalfEvenRat(r *big.Rat) *big.Int {
	num, den := r.Num(), r.Denom()
	q, m := new(big.Int).QuoRem(num, den, new(big.Int))
	if m.Sign() == 0 {
		return q
	}
	m2 := new(big.Int).Abs(m)
	m2.Mul(m2, big.NewInt(2))
	cmp := m2.CmpAbs(den)
	if cmp > 0 {
		if num.Sign() > 0 {
			q.Add(q, big.NewInt(1))
		} else {
			q.Sub(q, big.NewInt(1))
		}
	} else if cmp == 0 {

		if q.Bit(0) == 1 {
			if num.Sign() > 0 {
				q.Add(q, big.NewInt(1))
			} else {
				q.Sub(q, big.NewInt(1))
			}
		}
	}
	return q
}
