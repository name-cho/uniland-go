package gobridge

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"strconv"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/pybridge"
	"github.com/name-cho/uniland-go/internal/pyops"
)

type UniHost interface {
	UniGet(name string) (val.Value, error)
	UniSet(name string, v val.Value) error
	UniCall(name string, args []val.Value) (val.Value, error)
}

type PyExecer interface {
	Exec(code string) error
	Eval(code string) (val.Value, error)
}

type Bridge struct {
	out, errW io.Writer
	host      UniHost
	py        PyExecer
}

func New(out, errW io.Writer) *Bridge { return &Bridge{out: out, errW: errW} }

func (b *Bridge) SetHost(h UniHost) { b.host = h }

func (b *Bridge) SetPy(p PyExecer) { b.py = p }

func fail(format string, a ...interface{}) {
	panic(fmt.Sprintf(format, a...))
}

func (b *Bridge) newInterp() *interp.Interpreter {
	u := &uniAPI{b: b}
	i := interp.New(interp.Options{
		Stdout: b.out,
		Stderr: b.errW,
	})

	_ = i.Use(stdlib.Symbols)

	for _, pkg := range []string{"fmt", "strings", "strconv", "math", "sort", "time", "math/big", "unicode", "regexp"} {
		_, _ = i.Eval(`import "` + pkg + `"`)
	}

	_ = i.Use(interp.Exports{
		"uniland/uni/uni": {
			"Uni": reflect.ValueOf(u),
		},
	})
	i.Eval(`import uniapi "uniland/uni"`)
	i.Eval(`var uni = uniapi.Uni`)
	return i
}

func (b *Bridge) Exec(code string) error {
	i := b.newInterp()
	if _, err := i.Eval("func _uniMain() {\n" + code + "\n}"); err != nil {
		return err
	}
	if _, err := i.Eval("_uniMain()"); err != nil {
		return err
	}
	return nil
}

func (b *Bridge) Eval(code string) (val.Value, error) {
	i := b.newInterp()
	rv, err := i.Eval(code)
	if err != nil {
		return nil, err
	}
	return rvToVal(rv), nil
}

func rvToVal(rv reflect.Value) val.Value {
	if !rv.IsValid() {
		return val.Nul
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int64(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n := rv.Uint()
		if n > uint64(math.MaxInt64) {
			return val.NewFloat(float64(n))
		}
		return val.Int64(int64(n))
	case reflect.Float32, reflect.Float64:
		return val.NewFloat(rv.Float())
	case reflect.String:
		return val.Str(rv.String())
	case reflect.Bool:
		return val.Bool(rv.Bool())
	}
	if ptr := rv; ptr.Kind() == reflect.Ptr && rv.IsNil() {
		return val.Nul
	}
	typeOfBig := reflect.TypeOf(big.NewInt(0))
	switch rv.Type() {
	case typeOfBig:
		if x, ok := rv.Interface().(*big.Int); ok {
			return val.NewInt(new(big.Int).Set(x))
		}
	}
	return val.Str(fmt.Sprintf("%v", rv.Interface()))
}

func jsonEnvelope(v val.Value) string {
	blob, _ := json.Marshal(pybridge.Encode(v))
	return string(blob)
}

func jsonToVal(raw string) (val.Value, error) {
	return pybridge.Decode([]byte(raw))
}

type uniAPI struct {
	b *Bridge
}

func (u *uniAPI) get(name string) val.Value {
	if u.b.host == nil {
		fail("uni недоступен: интерпретатор не подключён")
	}
	v, err := u.b.host.UniGet(name)
	if err != nil {
		fail("%s", err.Error())
	}
	return v
}

func (u *uniAPI) set(name string, v val.Value) {
	if u.b.host == nil {
		fail("uni недоступен: интерпретатор не подключён")
	}
	if err := u.b.host.UniSet(name, v); err != nil {
		fail("%s", err.Error())
	}
}

func (u *uniAPI) SetInt(name string, v int64)     { u.set(name, val.Int64(v)) }
func (u *uniAPI) SetFloat(name string, v float64) { u.set(name, val.NewFloat(v)) }
func (u *uniAPI) SetStr(name string, v string)    { u.set(name, val.Str(v)) }
func (u *uniAPI) SetBool(name string, v bool)     { u.set(name, val.Bool(v)) }

func (u *uniAPI) SetBig(name string, decimal string) {
	n, ok := new(big.Int).SetString(decimal, 10)
	if !ok {
		fail("invalid decimal number: '%s'", decimal)
	}
	u.set(name, val.NewInt(n))
}

func (u *uniAPI) GetInt(name string) int64 {
	n, err := pyops.IntOf(u.get(name))
	if err != nil {
		fail("%s", err.Error())
	}

	if n.IsInt64() {
		return n.Int64()
	}
	if n.Sign() > 0 {
		return math.MaxInt64
	}
	return math.MinInt64
}

func (u *uniAPI) GetFloat(name string) float64 {
	v := u.get(name)
	switch x := v.(type) {
	case val.Bool:
		if x {
			return 1
		}
		return 0
	case val.Float:
		return float64(x)
	case *val.Int:
		f, _ := new(big.Float).SetInt(x.N).Float64()
		return f
	case val.Str:
		f, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			fail("can't convert '%s' to float", string(x))
		}
		return f
	}
	fail("can't convert %s to float", pyops.TypeName(v))
	return 0
}

func (u *uniAPI) GetStr(name string) string { return pyops.PyStr(u.get(name)) }
func (u *uniAPI) GetBool(name string) bool  { return pyops.Truthy(u.get(name)) }
func (u *uniAPI) GetBig(name string) string {
	v := u.get(name)
	n := val.IntVal(v)
	if n == nil {
		fail("'%s' is not an integer", pyops.TypeName(v))
	}
	return n.String()
}

func (u *uniAPI) Call(name string, args ...string) string {
	vals := make([]val.Value, len(args))
	for i, a := range args {
		vals[i] = coerceArg(a)
	}
	r, err := u.call(name, vals)
	if err != nil {
		fail("%s", err.Error())
	}
	return pyops.PyStr(r)
}

func coerceArg(s string) val.Value {
	if s == "true" {
		return val.Bool(true)
	}
	if s == "false" {
		return val.Bool(false)
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val.Int64(n)
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return val.NewFloat(f)
	}
	return val.Str(s)
}

func (u *uniAPI) CallJSON(name string, argsJSON []string) string {
	vals := make([]val.Value, len(argsJSON))
	for i, raw := range argsJSON {
		v, err := jsonToVal(raw)
		if err != nil {
			fail("%s", err.Error())
		}
		vals[i] = v
	}
	r, err := u.call(name, vals)
	if err != nil {
		fail("%s", err.Error())
	}
	return jsonEnvelope(r)
}

func (u *uniAPI) call(name string, args []val.Value) (val.Value, error) {
	if u.b.host == nil {
		return nil, fmt.Errorf("uni недоступен: интерпретатор не подключён")
	}
	return u.b.host.UniCall(name, args)
}

func (u *uniAPI) pyEval(code string) val.Value {
	if u.b.py == nil {
		fail("py недоступен: python-мост не подключён")
	}
	r, err := u.b.py.Eval(code)
	if err != nil {
		fail("%s", err.Error())
	}
	return r
}

func (u *uniAPI) PyEvalStr(code string) string { return pyops.PyStr(u.pyEval(code)) }
func (u *uniAPI) PyEvalBool(code string) bool  { return pyops.Truthy(u.pyEval(code)) }

func (u *uniAPI) PyEvalInt(code string) int64 {
	n, err := pyops.IntOf(u.pyEval(code))
	if err != nil {
		fail("%s", err.Error())
	}
	return n.Int64()
}

func (u *uniAPI) PyEvalFloat(code string) float64 {
	f := u.pyEval(code)
	switch x := f.(type) {
	case val.Bool:
		if x {
			return 1
		}
		return 0
	case val.Float:
		return float64(x)
	case *val.Int:
		r, _ := new(big.Float).SetInt(x.N).Float64()
		return r
	case val.Str:
		p, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			fail("can't convert '%s' to float", string(x))
		}
		return p
	}
	fail("can't convert %s to float", pyops.TypeName(f))
	return 0
}

func (u *uniAPI) PyEvalBig(code string) string {
	v := u.pyEval(code)
	n := val.IntVal(v)
	if n == nil {
		fail("'%s' is not an integer", pyops.TypeName(v))
	}
	return n.String()
}

func (u *uniAPI) PyExec(code string) {
	if u.b.py == nil {
		fail("py недоступен: python-мост не подключён")
	}
	if err := u.b.py.Exec(code); err != nil {
		fail("%s", err.Error())
	}
}
