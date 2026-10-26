package std

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"strings"

	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/pyops"
)

type Native func(args []val.Value) (val.Value, error)

type Ctx struct {
	Out        io.Writer
	In         io.Reader
	RdIn       *bufio.Reader
	Call       func(fn val.Value, args []val.Value) (val.Value, error)
	OnExit     func(code int) error
	ScriptArgs []string
}

func need(args []val.Value, n int, name string) error {
	if len(args) != n {
		return fmt.Errorf("%s() expects %d argument(s), got %d", name, n, len(args))
	}
	return nil
}

func needRange(args []val.Value, lo, hi int, name string, form string) error {
	if len(args) < lo || len(args) > hi {
		return fmt.Errorf("%s() expects %s", name, form)
	}
	return nil
}

func arrArg(args []val.Value, name string, idx int) (*val.List, error) {
	if l, ok := args[idx].(*val.List); ok {
		return l, nil
	}
	return nil, fmt.Errorf("%s() argument must be an array", name)
}

func strAttr(v val.Value, attr string) error {
	return fmt.Errorf("'%s' object has no attribute '%s'", pyops.TypeName(v), attr)
}

func needStr(args []val.Value, idx int, attr string) (string, error) {
	if s, ok := args[idx].(val.Str); ok {
		return string(s), nil
	}
	return "", strAttr(args[idx], attr)
}

func bPrint(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = pyops.Str(a)
		}
		fmt.Fprintf(ctx.Out, "%s\n", strings.Join(parts, " "))
		return val.Nul, nil
	}
}

func logger(ctx *Ctx, prefix string) Native {
	return func(args []val.Value) (val.Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = pyops.Str(a)
		}
		fmt.Fprintf(ctx.Out, "%s %s\n", prefix, strings.Join(parts, " "))
		return val.Nul, nil
	}
}

func bTypeof(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "typeof"); err != nil {
		return nil, err
	}
	return val.Str(pyops.TypeOf(args[0])), nil
}

func bToString(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "to_string"); err != nil {
		return nil, err
	}
	return val.Str(pyops.Str(args[0])), nil
}

func bToNumber(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "to_number"); err != nil {
		return nil, err
	}
	v := args[0]
	if b, ok := v.(val.Bool); ok {
		if b {
			return val.Int64(1), nil
		}
		return val.Int64(0), nil
	}
	s := strings.TrimSpace(pyops.PyStr(v))
	if regexp.MustCompile(`^[+-]?\d+$`).MatchString(s) {
		n := new(big.Int)
		if _, ok := n.SetString(s, 10); ok {
			return val.NewInt(n), nil
		}
	}
	f, err := strconvParsePyFloat(s)
	if err != nil {
		return nil, fmt.Errorf("Cannot convert %s to number", pyops.Str(v))
	}
	return val.NewFloat(f), nil
}

func strconvParsePyFloat(s string) (float64, error) {
	f, err := parseFloat64(s)
	if err != nil {
		if isRangeError(err) {
			return f, nil
		}
		return 0, err
	}
	return f, nil
}

func bToBoolean(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "to_boolean"); err != nil {
		return nil, err
	}
	return val.Bool(pyops.Truthy(args[0])), nil
}

func isNumber(v val.Value) bool {
	switch x := v.(type) {
	case val.Bool:
		return false
	case *val.Int, val.Float:
		_ = x
		return true
	}
	return false
}

func isCheck(name string, fn func(val.Value) bool) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := need(args, 1, "<lambda>"); err != nil {
			return nil, err
		}
		return val.Bool(fn(args[0])), nil
	}
}

func bLength(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "length"); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case val.Str:
		return val.Int64(int64(len([]rune(string(x))))), nil
	case *val.List:
		return val.Int64(int64(len(x.Items))), nil
	case *val.Map:
		return val.Int64(int64(x.Len())), nil
	}
	return nil, fmt.Errorf("length() needs a string, array or object")
}

func bSplit(args []val.Value) (val.Value, error) {
	if err := needRange(args, 1, 2, "split", "(string) or (string, separator)"); err != nil {
		return nil, err
	}
	s, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("split() first argument must be a string")
	}
	if len(args) == 1 {
		items := []val.Value{}
		for _, f := range strings.Fields(string(s)) {
			items = append(items, val.Str(f))
		}
		return val.NewList(items), nil
	}
	sep, ok := args[1].(val.Str)
	if !ok {
		return nil, fmt.Errorf("must be str or None, not %s", pyops.TypeName(args[1]))
	}
	if sep == "" {
		items := []val.Value{}
		for _, r := range []rune(string(s)) {
			items = append(items, val.Str(string(r)))
		}
		return val.NewList(items), nil
	}
	items := []val.Value{}
	for _, f := range strings.Split(string(s), string(sep)) {
		items = append(items, val.Str(f))
	}
	return val.NewList(items), nil
}

func bJoin(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "join"); err != nil {
		return nil, err
	}
	ary, ok := args[0].(*val.List)
	if !ok {
		return nil, fmt.Errorf("join() first argument must be an array")
	}
	sep, ok := args[1].(val.Str)
	if !ok {
		return nil, strAttr(args[1], "join")
	}
	parts := make([]string, len(ary.Items))
	for i, x := range ary.Items {
		parts[i] = pyops.Str(x)
	}
	return val.Str(strings.Join(parts, string(sep))), nil
}

func bReplace(args []val.Value) (val.Value, error) {
	if err := need(args, 3, "replace"); err != nil {
		return nil, err
	}
	s, ok := args[0].(val.Str)
	if !ok {
		return nil, strAttr(args[0], "replace")
	}
	old, err := needStr(args, 1, "")
	if err != nil {
		return nil, fmt.Errorf("replace() argument 1 must be str, not %s", pyops.TypeName(args[1]))
	}
	neu, err2 := needStr(args, 2, "")
	if err2 != nil {
		return nil, fmt.Errorf("replace() argument 2 must be str, not %s", pyops.TypeName(args[2]))
	}
	return val.Str(strings.ReplaceAll(string(s), old, neu)), nil
}

func bSubstring(args []val.Value) (val.Value, error) {
	if err := needRange(args, 2, 3, "substring", "(string, start[, end])"); err != nil {
		return nil, err
	}
	s, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("'%s' object is not subscriptable", pyops.TypeName(args[0]))
	}
	start, err := pyops.IntOf(args[1])
	if err != nil {
		return nil, err
	}
	end := new(big.Int).SetInt64(int64(len([]rune(string(s)))))
	if len(args) == 3 {
		if end, err = pyops.IntOf(args[2]); err != nil {
			return nil, err
		}
	}
	a, b := sliceBounds(start, end, int64(len([]rune(string(s)))))
	rs := []rune(string(s))
	return val.Str(string(rs[a:b])), nil
}

func bTrim(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "trim"); err != nil {
		return nil, err
	}
	if s, ok := args[0].(val.Str); ok {
		return val.Str(strings.TrimSpace(string(s))), nil
	}
	return nil, strAttr(args[0], "trim")
}

func bUpper(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "uppercase"); err != nil {
		return nil, err
	}
	if s, ok := args[0].(val.Str); ok {
		return val.Str(strings.ToUpper(string(s))), nil
	}
	return nil, strAttr(args[0], "upper")
}

func bLower(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "lowercase_"); err != nil {
		return nil, err
	}
	if s, ok := args[0].(val.Str); ok {
		return val.Str(strings.ToLower(string(s))), nil
	}
	return nil, strAttr(args[0], "lower")
}

func bStartswith(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "startswith"); err != nil {
		return nil, err
	}
	if s, ok := args[0].(val.Str); ok {
		if p, ok := args[1].(val.Str); ok {
			return val.Bool(strings.HasPrefix(string(s), string(p))), nil
		}
		return nil, fmt.Errorf("startswith first arg must be str or a tuple of str, not %s", pyops.TypeName(args[1]))
	}
	return nil, strAttr(args[0], "startswith")
}

func bEndswith(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "endswith"); err != nil {
		return nil, err
	}
	if s, ok := args[0].(val.Str); ok {
		if p, ok := args[1].(val.Str); ok {
			return val.Bool(strings.HasSuffix(string(s), string(p))), nil
		}
		return nil, fmt.Errorf("endswith first arg must be str or a tuple of str, not %s", pyops.TypeName(args[1]))
	}
	return nil, strAttr(args[0], "endswith")
}

func bRepeat(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "repeat"); err != nil {
		return nil, err
	}
	n, err := pyops.IntOf(args[1])
	if err != nil {
		return nil, err
	}
	return pyops.Mul(args[0], val.NewInt(n))
}

func bCharCode(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "char_code"); err != nil {
		return nil, err
	}
	s, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("ord() expected string of length 1, but %s found", pyops.TypeName(args[0]))
	}
	rs := []rune(string(s))
	if len(rs) != 1 {
		return nil, fmt.Errorf("ord() expected a character, but string of length %d found", len(rs))
	}
	return val.Int64(int64(rs[0])), nil
}

func bFromCharCode(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "from_char_code"); err != nil {
		return nil, err
	}
	n, err := pyops.IntOf(args[0])
	if err != nil {
		return nil, err
	}
	if !n.IsInt64() || n.Int64() < 0 || n.Int64() > 0x10FFFF {
		return nil, fmt.Errorf("chr() arg not in range(0x110000)")
	}
	return val.Str(string(rune(n.Int64()))), nil
}

func bPush(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "push"); err != nil {
		return nil, err
	}
	ary, err := arrArg(args, "push", 0)
	if err != nil {
		return nil, err
	}
	ary.Items = append(ary.Items, args[1])
	return val.Int64(int64(len(ary.Items))), nil
}

func bPop(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "pop"); err != nil {
		return nil, err
	}
	ary, err := arrArg(args, "pop", 0)
	if err != nil {
		return nil, err
	}
	if len(ary.Items) == 0 {
		return nil, fmt.Errorf("pop() from empty array")
	}
	last := ary.Items[len(ary.Items)-1]
	ary.Items = ary.Items[:len(ary.Items)-1]
	return last, nil
}

func bShift(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "shift"); err != nil {
		return nil, err
	}
	ary, err := arrArg(args, "shift", 0)
	if err != nil {
		return nil, err
	}
	if len(ary.Items) == 0 {
		return nil, fmt.Errorf("shift() from empty array")
	}
	first := ary.Items[0]
	ary.Items = ary.Items[1:]
	return first, nil
}

func bUnshift(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "unshift"); err != nil {
		return nil, err
	}
	ary, err := arrArg(args, "unshift", 0)
	if err != nil {
		return nil, err
	}
	ary.Items = append([]val.Value{args[1]}, ary.Items...)
	return val.Int64(int64(len(ary.Items))), nil
}

func bSlice(args []val.Value) (val.Value, error) {
	if err := needRange(args, 2, 3, "slice", "(array, start[, end])"); err != nil {
		return nil, err
	}
	ary, err := arrArg(args, "slice", 0)
	if err != nil {
		return nil, err
	}
	start, err := pyops.IntOf(args[1])
	if err != nil {
		return nil, err
	}
	end := big.NewInt(int64(len(ary.Items)))
	if len(args) == 3 {
		if end, err = pyops.IntOf(args[2]); err != nil {
			return nil, err
		}
	}
	a, b := sliceBounds(start, end, int64(len(ary.Items)))
	out := make([]val.Value, b-a)
	copy(out, ary.Items[a:b])
	return val.NewList(out), nil
}

func sliceBounds(startI, endI *big.Int, size int64) (int64, int64) {
	bound := func(v *big.Int, dflt int64) int64 {
		var x int64
		if v.IsInt64() {
			x = v.Int64()
		} else if v.Sign() > 0 {
			x = math.MaxInt64
		} else {
			x = math.MinInt64
		}
		if x < 0 {
			x += size
			if x < 0 {
				x = -1 << 62
			}
		} else if x > size {
			x = size
		}
		if x == -1<<62 {
			return 0
		}
		return x
	}
	a := bound(startI, 0)
	b := bound(endI, size)
	if b < a {
		b = a
	}
	if a < 0 {
		a = 0
	}
	return a, b
}

func bSplice(args []val.Value) (val.Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("splice() expects (array, start[, count])")
	}
	ary, err := arrArg(args, "splice", 0)
	if err != nil {
		return nil, err
	}
	start, err := pyops.IntOf(args[1])
	if err != nil {
		return nil, err
	}

	n := int64(len(ary.Items))
	count := new(big.Int).Sub(big.NewInt(n), start)
	if len(args) >= 3 {
		if count, err = pyops.IntOf(args[2]); err != nil {
			return nil, err
		}
	}
	a, ae := sliceBounds(start, new(big.Int).Add(start, count), n)
	removed := make([]val.Value, ae-a)
	copy(removed, ary.Items[a:ae])
	rest := append(append([]val.Value{}, ary.Items[:a]...), ary.Items[ae:]...)
	ary.Items = rest
	return val.NewList(removed), nil
}

func bReverse(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "reverse"); err != nil {
		return nil, err
	}
	ary, err := arrArg(args, "reverse", 0)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(ary.Items)-1; i < j; i, j = i+1, j-1 {
		ary.Items[i], ary.Items[j] = ary.Items[j], ary.Items[i]
	}
	return ary, nil
}

func bSort(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := needRange(args, 1, 2, "sort", "(array[, keyfn])"); err != nil {
			return nil, err
		}
		ary, err := arrArg(args, "sort", 0)
		if err != nil {
			return nil, err
		}
		if len(args) == 2 {
			keys := make([]val.Value, len(ary.Items))
			for i, x := range ary.Items {
				k, err := ctx.Call(args[1], []val.Value{x})
				if err != nil {
					return nil, err
				}
				keys[i] = k
			}
			if err := sortPair(ary.Items, keys); err != nil {
				return nil, err
			}
			return ary, nil
		}
		if err := pyops.SortStable(ary.Items); err != nil {
			return nil, err
		}
		return ary, nil
	}
}

func sortPair(items, keys []val.Value) error {
	var sortErr error
	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	less := func(x, y int) bool {
		lt, err := cmpLess(keys[x], keys[y])
		if err != nil {
			if sortErr == nil {
				sortErr = err
			}
			return false
		}
		return lt
	}
	stableSort(idx, less)
	if sortErr != nil {
		return fmt.Errorf("sort() cannot compare these elements")
	}
	out := make([]val.Value, len(items))
	newKeys := make([]val.Value, len(items))
	for i, j := range idx {
		out[i] = items[j]
		newKeys[i] = keys[j]
	}
	copy(items, out)
	copy(keys, newKeys)
	return nil
}

func bIncludes(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "includes"); err != nil {
		return nil, err
	}
	switch coll := args[0].(type) {
	case *val.List:
		for _, x := range coll.Items {
			if pyops.Equal(x, args[1]) {
				return val.Bool(true), nil
			}
		}
		return val.Bool(false), nil
	case val.Str:
		n, ok := args[1].(val.Str)
		if !ok {
			return nil, fmt.Errorf("in <string> requires string as left operand, not %s", pyops.TypeName(args[1]))
		}
		return val.Bool(strings.Contains(string(coll), string(n))), nil
	}
	return nil, fmt.Errorf("includes() first argument must be an array or string")
}

func bIndexOf(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "indexOf"); err != nil {
		return nil, err
	}
	switch coll := args[0].(type) {
	case *val.List:
		for i, x := range coll.Items {
			if pyops.Equal(x, args[1]) {
				return val.Int64(int64(i)), nil
			}
		}
		return val.Int64(int64(-1)), nil
	case val.Str:
		n, ok := args[1].(val.Str)
		if !ok {
			return nil, fmt.Errorf("must be str, not %s", pyops.TypeName(args[1]))
		}
		return val.Int64(int64(strings.Index(string(coll), string(n)))), nil
	}
	return nil, fmt.Errorf("indexOf() first argument must be an array or string")
}

func bMap(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := need(args, 2, "map"); err != nil {
			return nil, err
		}
		ary, err := arrArg(args, "map", 0)
		if err != nil {
			return nil, err
		}
		out := make([]val.Value, len(ary.Items))
		for i, x := range ary.Items {
			out[i], err = ctx.Call(args[1], []val.Value{x})
			if err != nil {
				return nil, err
			}
		}
		return val.NewList(out), nil
	}
}

func bFilter(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := need(args, 2, "filter"); err != nil {
			return nil, err
		}
		ary, err := arrArg(args, "filter", 0)
		if err != nil {
			return nil, err
		}
		out := []val.Value{}
		for _, x := range ary.Items {
			r, err := ctx.Call(args[1], []val.Value{x})
			if err != nil {
				return nil, err
			}
			if pyops.Truthy(r) {
				out = append(out, x)
			}
		}
		return val.NewList(out), nil
	}
}

func bReduce(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := needRange(args, 2, 3, "reduce", "(array, fn[, initial])"); err != nil {
			return nil, err
		}
		ary, err := arrArg(args, "reduce", 0)
		if err != nil {
			return nil, err
		}
		var acc val.Value
		var items []val.Value
		if len(args) == 3 {
			acc = args[2]
			items = ary.Items
		} else {
			if len(ary.Items) == 0 {
				return nil, fmt.Errorf("reduce() of empty array with no initial value")
			}
			acc = ary.Items[0]
			items = ary.Items[1:]
		}
		for _, x := range items {
			acc, err = ctx.Call(args[1], []val.Value{acc, x})
			if err != nil {
				return nil, err
			}
		}
		return acc, nil
	}
}

func bFind(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := need(args, 2, "find"); err != nil {
			return nil, err
		}
		ary, err := arrArg(args, "find", 0)
		if err != nil {
			return nil, err
		}
		for _, x := range ary.Items {
			r, err := ctx.Call(args[1], []val.Value{x})
			if err != nil {
				return nil, err
			}
			if pyops.Truthy(r) {
				return x, nil
			}
		}
		return val.Nul, nil
	}
}

func bEach(ctx *Ctx) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := need(args, 2, "each"); err != nil {
			return nil, err
		}
		ary, err := arrArg(args, "each", 0)
		if err != nil {
			return nil, err
		}
		for _, x := range ary.Items {
			if _, err := ctx.Call(args[1], []val.Value{x}); err != nil {
				return nil, err
			}
		}
		return val.Nul, nil
	}
}

func bSum(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "sum"); err != nil {
		return nil, err
	}
	ary, err := arrArg(args, "sum", 0)
	if err != nil {
		return nil, err
	}
	acc := val.Int64(0)
	for _, x := range ary.Items {
		acc, err = pyops.Add(acc, x)
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}

func bRange(args []val.Value) (val.Value, error) {
	switch len(args) {
	case 1, 2, 3:
		r, err := pyops.RangeOf(args)
		if err != nil {
			var rs *pyops.RangeTooLargeError
			var _ = rs
			return nil, err
		}
		return r, nil
	}
	return nil, fmt.Errorf("range() expects 1-3 arguments")
}

func bKeys(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "keys"); err != nil {
		return nil, err
	}
	m, ok := args[0].(*val.Map)
	if !ok {
		return nil, fmt.Errorf("keys() argument must be an object")
	}
	items := []val.Value{}
	for _, k := range m.Keys() {
		items = append(items, val.Str(k))
	}
	return val.NewList(items), nil
}

func bValues(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "values"); err != nil {
		return nil, err
	}
	m, ok := args[0].(*val.Map)
	if !ok {
		return nil, fmt.Errorf("values() argument must be an object")
	}
	items := []val.Value{}
	for _, kv := range m.Ordered() {
		items = append(items, kv.V)
	}
	return val.NewList(items), nil
}

func bEntries(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "entries"); err != nil {
		return nil, err
	}
	m, ok := args[0].(*val.Map)
	if !ok {
		return nil, fmt.Errorf("entries() argument must be an object")
	}
	items := []val.Value{}
	for _, kv := range m.Ordered() {
		items = append(items, val.NewList([]val.Value{val.Str(kv.K), kv.V}))
	}
	return val.NewList(items), nil
}

func bHas(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "has"); err != nil {
		return nil, err
	}
	m, ok := args[0].(*val.Map)
	if !ok {
		return nil, fmt.Errorf("has() first argument must be an object")
	}

	s, ok := args[1].(val.Str)
	if !ok {
		return val.Bool(false), nil
	}
	_, has := m.GetOK(string(s))
	return val.Bool(has), nil
}

func bMerge(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "merge"); err != nil {
		return nil, err
	}
	a, okA := args[0].(*val.Map)
	b, okB := args[1].(*val.Map)
	if !okA || !okB {
		return nil, fmt.Errorf("merge() needs two objects")
	}
	out := val.NewMap()
	for _, kv := range a.Ordered() {
		out.Set(kv.K, kv.V)
	}
	for _, kv := range b.Ordered() {
		out.Set(kv.K, kv.V)
	}
	return out, nil
}

func bClone(args []val.Value) (val.Value, error) {
	if err := need(args, 1, "clone"); err != nil {
		return nil, err
	}
	switch x := args[0].(type) {
	case *val.Map:
		out := val.NewMap()
		for _, kv := range x.Ordered() {
			out.Set(kv.K, kv.V)
		}
		return out, nil
	case *val.List:
		items := make([]val.Value, len(x.Items))
		copy(items, x.Items)
		return val.NewList(items), nil
	}
	return args[0], nil
}

func numGuard(args []val.Value, name string) (val.Value, error) {
	if err := need(args, 1, name); err != nil {
		return nil, err
	}
	if isNumber(args[0]) {
		return args[0], nil
	}
	return nil, fmt.Errorf("%s() argument must be a number", name)
}

func bAbs(args []val.Value) (val.Value, error) {
	v, err := numGuard(args, "abs")
	if err != nil {
		return nil, err
	}
	return pyops.Abs(v)
}

func bRound(args []val.Value) (val.Value, error) {
	if len(args) == 2 {
		n, err := pyops.IntOf(args[1])
		if err != nil {
			return nil, err
		}
		nd := int(n.Int64())
		if !n.IsInt64() {
			nd = 0
		}
		return roundValue(args[0], &nd)
	}
	v, err := numGuard(args, "round")
	if err != nil {
		return nil, err
	}
	return roundValue(v, nil)
}

func roundValue(v val.Value, nd *int) (val.Value, error) {
	switch x := v.(type) {
	case val.Bool, *val.Int:

		if n := val.IntVal(v); n != nil {
			return val.NewInt(new(big.Int).Set(n)), nil
		}
		return val.Int64(0), nil
	case val.Float:
		return val.PyRound(float64(x), nd), nil
	}
	return nil, fmt.Errorf("type %s doesn't define __round__ method", pyops.TypeName(v))
}

func bFloor(args []val.Value) (val.Value, error) {
	v, err := numGuard(args, "floor")
	if err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case val.Bool, *val.Int:
		if n := val.IntVal(v); n != nil {
			return val.NewInt(new(big.Int).Set(n)), nil
		}
	case val.Float:
		return floorCeil(float64(x), true)
	}
	return val.Int64(0), nil
}

func bCeil(args []val.Value) (val.Value, error) {
	v, err := numGuard(args, "ceil")
	if err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case val.Bool, *val.Int:
		if n := val.IntVal(v); n != nil {
			return val.NewInt(new(big.Int).Set(n)), nil
		}
	case val.Float:
		return floorCeil(float64(x), false)
	}
	return val.Int64(0), nil
}

func floorCeil(f float64, isFloor bool) (val.Value, error) {
	if math.IsNaN(f) {
		return nil, fmt.Errorf("cannot convert float NaN to integer")
	}
	if math.IsInf(f, 0) {
		return nil, fmt.Errorf("cannot convert float infinity to integer")
	}
	if isFloor {
		return val.Int64(int64(math.Floor(f))), nil
	}
	return val.Int64(int64(math.Ceil(f))), nil
}

func bSqrt(args []val.Value) (val.Value, error) {
	v, err := numGuard(args, "sqrt")
	if err != nil {
		return nil, err
	}
	f, ok, _ := asFloatOf(v)
	if !ok {
		return nil, fmt.Errorf("sqrt() argument must be a number")
	}
	if f < 0 {
		return nil, fmt.Errorf("math domain error")
	}
	return val.NewFloat(math.Sqrt(f)), nil
}

func asFloatOf(v val.Value) (float64, bool, bool) {
	switch x := v.(type) {
	case val.Bool:
		if x {
			return 1, true, false
		}
		return 0, true, false
	case *val.Int:
		if x.N.IsInt64() {
			return float64(x.N.Int64()), true, false
		}
		f := new(big.Float).SetInt(x.N)
		r, _ := f.Float64()
		return r, true, false
	case val.Float:
		return float64(x), true, false
	}
	return 0, false, false
}

func bMin(args []val.Value) (val.Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("min expected at least 1 argument, got 0")
	}
	seq := args
	if len(args) == 1 {
		if l, ok := args[0].(*val.List); ok {
			seq = l.Items
		}
	}
	if len(seq) == 0 {
		return nil, fmt.Errorf("min() arg is an empty sequence")
	}
	best := seq[0]
	for _, x := range seq[1:] {
		lt, err := cmpLess(x, best)
		if err != nil {
			return nil, err
		}
		if lt {
			best = x
		}
	}
	return best, nil
}

func bMax(args []val.Value) (val.Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("max expected at least 1 argument, got 0")
	}
	seq := args
	if len(args) == 1 {
		if l, ok := args[0].(*val.List); ok {
			seq = l.Items
		}
	}
	if len(seq) == 0 {
		return nil, fmt.Errorf("max() arg is an empty sequence")
	}
	best := seq[0]
	for _, x := range seq[1:] {
		gt, err := cmpLess(best, x)
		if err != nil {
			return nil, err
		}
		if gt {
			best = x
		}
	}
	return best, nil
}

func bPow(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "pow"); err != nil {
		return nil, err
	}
	return pyops.Pow(args[0], args[1])
}

func bRandom(rnd *randSrc) Native {
	return func(args []val.Value) (val.Value, error) {
		if len(args) == 0 {
			return val.NewFloat(rnd.random()), nil
		}
		if len(args) == 2 {
			a, err := pyops.IntOf(args[0])
			if err != nil {
				return nil, err
			}
			b, err := pyops.IntOf(args[1])
			if err != nil {
				return nil, err
			}
			iv := rnd.randint(a, b)
			return val.NewInt(iv), nil
		}
		return nil, fmt.Errorf("random() expects 0 or 2 arguments")
	}
}

func bRandomInt(rnd *randSrc) Native {
	return func(args []val.Value) (val.Value, error) {
		if err := need(args, 2, "random_int"); err != nil {
			return nil, err
		}
		a, err := pyops.IntOf(args[0])
		if err != nil {
			return nil, err
		}
		b, err := pyops.IntOf(args[1])
		if err != nil {
			return nil, err
		}
		return val.NewInt(rnd.randint(a, b)), nil
	}
}

func compileRE(pattern string) (*regexp.Regexp, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("re.error: %s", regexpErrText(err))
	}
	return re, nil
}

func regexpErrText(err error) string {
	s := err.Error()

	return s
}

func bRegexMatch(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "regex_match"); err != nil {
		return nil, err
	}
	p, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("first argument must be string or compiled pattern")
	}
	s, err2 := needStr(args, 1, "")
	if err2 != nil {
		return nil, fmt.Errorf("expected string or bytes-like object, got '%s'", pyops.TypeName(args[1]))
	}
	re, err := compileRE(string(p))
	if err != nil {
		return nil, err
	}
	return val.Bool(re.MatchString(s)), nil
}

func bRegexFindall(args []val.Value) (val.Value, error) {
	if err := need(args, 2, "regex_findall"); err != nil {
		return nil, err
	}
	p, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("pattern must be a string")
	}
	s, sErr := needStr(args, 1, "")
	if sErr != nil {
		return nil, fmt.Errorf("expected string or bytes-like object, got '%s'", pyops.TypeName(args[1]))
	}
	re, err := compileRE(string(p))
	if err != nil {
		return nil, err
	}
	matches := re.FindAllStringSubmatch(s, -1)
	groups := re.NumSubexp()
	items := []val.Value{}
	if groups == 0 {
		for _, m := range matches {
			items = append(items, val.Str(m[0]))
		}
	} else if groups == 1 {
		for _, m := range matches {
			items = append(items, val.Str(m[1]))
		}
	} else {
		for _, m := range matches {
			g := make([]val.Value, 0, len(m)-1)
			for _, x := range m[1:] {
				g = append(g, val.Str(x))
			}
			items = append(items, val.NewList(g))
		}
	}
	return val.NewList(items), nil
}

func bRegexReplace(args []val.Value) (val.Value, error) {
	if err := need(args, 3, "regex_replace"); err != nil {
		return nil, err
	}
	p, ok := args[0].(val.Str)
	if !ok {
		return nil, fmt.Errorf("pattern must be a string")
	}
	s, sErr := needStr(args, 1, "")
	if sErr != nil {
		return nil, fmt.Errorf("expected string or bytes-like object, got '%s'", pyops.TypeName(args[1]))
	}
	repl, ok3 := args[2].(val.Str)
	if !ok3 {
		return nil, fmt.Errorf("repl must be a string")
	}
	re, err := compileRE(string(p))
	if err != nil {
		return nil, err
	}
	return val.Str(re.ReplaceAllString(s, translateRepl(string(repl)))), nil
}

func translateRepl(repl string) string {
	var b strings.Builder
	i := 0
	for i < len(repl) {
		if repl[i] == '\\' && i+1 < len(repl) {
			c := repl[i+1]
			if c >= '0' && c <= '9' {
				b.WriteByte('$')
				b.WriteByte(c)
				i += 2
				continue
			}
			if strings.HasPrefix(repl[i:], `\g<`) {
				j := i + 3
				for j < len(repl) && (repl[j] >= '0' && repl[j] <= '9' || repl[j] == '_') {
					j++
				}
				if j < len(repl) && repl[j] == '>' {
					b.WriteString("${" + repl[i+3:j] + "}")
					i = j + 1
					continue
				}
			}
			b.WriteString(`\\`)
			b.WriteByte(c)
			i += 2
			continue
		}
		b.WriteByte(repl[i])
		i++
	}
	return b.String()
}
