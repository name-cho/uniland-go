package interp

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/name-cho/uniland-go/internal/ast"
	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/pyops"
	"github.com/name-cho/uniland-go/internal/std"
	"github.com/name-cho/uniland-go/internal/unierr"
)

type NativeFunc = std.Native

type UniFunction struct {
	Params  []string
	Body    *ast.Block
	Closure *Environment
	Name    string

	Compiled any
}

func (f *UniFunction) Repr() string {
	return fmt.Sprintf("<func %s(%s)>", f.Name, strings.Join(f.Params, ", "))
}

type Environment struct {
	Parent *Environment
	vars   map[string]val.Value
	consts map[string]bool
}

func NewEnvironment(parent *Environment) *Environment {
	return &Environment{Parent: parent, vars: map[string]val.Value{}, consts: map[string]bool{}}
}

func (e *Environment) Define(name string, value val.Value, isConst bool) {
	e.vars[name] = value
	if isConst {
		e.consts[name] = true
	}
}

func (e *Environment) Get(name string) (val.Value, error) {
	env := e
	for env != nil {
		if v, ok := env.vars[name]; ok {
			return v, nil
		}
		env = env.Parent
	}
	return nil, unierr.New(fmt.Sprintf("Undefined variable: %s", name))
}

func (e *Environment) Set(name string, value val.Value) error {
	env := e
	for env != nil {
		if _, ok := env.vars[name]; ok {
			if env.consts[name] {
				return unierr.New(fmt.Sprintf("Cannot reassign constant: %s", name))
			}
			env.vars[name] = value
			return nil
		}
		env = env.Parent
	}
	return unierr.New(fmt.Sprintf("Undefined variable: %s", name))
}

func (e *Environment) Has(name string) bool {
	env := e
	for env != nil {
		if _, ok := env.vars[name]; ok {
			return true
		}
		env = env.Parent
	}
	return false
}

type returnSignal struct{ Value val.Value }
type breakSignal struct{}
type continueSignal struct{}

func (r *returnSignal) Error() string   { return "return" }
func (b *breakSignal) Error() string    { return "break" }
func (c *continueSignal) Error() string { return "continue" }

type ExitSignal struct{ Code int }

func (s *ExitSignal) Error() string { return fmt.Sprintf("SystemExit(%d)", s.Code) }

var errBreak = &breakSignal{}
var errContinue = &continueSignal{}

type Bridge interface {
	Eval(code string) (val.Value, error)
	Exec(code string) error
}

type Interp struct {
	GlobalEnv  *Environment
	env        *Environment
	Out        io.Writer
	Err        io.Writer
	In         io.Reader
	PyBridge   Bridge
	GoBridge   Bridge
	stdCtx     *std.Ctx
	scriptArgs []string
	runner     FnRunner
}

func New(out, errW io.Writer, reader io.Reader, scriptArgs []string) *Interp {
	in := &Interp{
		GlobalEnv:  NewEnvironment(nil),
		Out:        out,
		Err:        errW,
		In:         reader,
		scriptArgs: scriptArgs,
	}
	in.env = in.GlobalEnv

	ctx := &std.Ctx{
		Out: out,
		In:  reader,
		Call: func(fn val.Value, args []val.Value) (val.Value, error) {
			return in.callValue(fn, args, -1)
		},
		OnExit: func(code int) error {
			return &ExitSignal{Code: code}
		},
		ScriptArgs: scriptArgs,
	}
	for name, fn := range std.All(ctx) {
		in.GlobalEnv.Define(name, fn, false)
	}
	in.GlobalEnv.Define("pyeval", bridgeEval("pyeval", in), false)
	in.GlobalEnv.Define("pyexec", bridgeExec("pyexec", in), false)
	in.GlobalEnv.Define("py", bridgeExec("py", in), false)
	in.GlobalEnv.Define("goeval", bridgeEval("goeval", in), false)
	in.GlobalEnv.Define("goexec", bridgeExec("goexec", in), false)
	in.stdCtx = ctx
	return in
}

func (in *Interp) SetRdIn(r *bufio.Reader) {
	if in.stdCtx != nil {
		in.stdCtx.RdIn = r
	}
}

func bridgeEval(kind string, in *Interp) std.Native {
	return func(args []val.Value) (val.Value, error) {
		if len(args) != 1 {
			return nil, unierr.New(fmt.Sprintf("%s() expects one string argument", kind))
		}
		code, ok := args[0].(val.Str)
		if !ok {
			return nil, unierr.New(fmt.Sprintf("%s() expects one string argument", kind))
		}
		br := in.PyBridge
		if kind == "goeval" {
			br = in.GoBridge
		}
		if br == nil {
			prefix := "Python"
			if kind == "goeval" {
				prefix = "Go"
			}
			return nil, unierr.New(prefix + " worker недоступен: интерпретатор не подключён")
		}
		r, err := br.Eval(string(code))
		if err != nil {
			return nil, wrapBridgeErr(kind, err)
		}
		return r, nil
	}
}

func bridgeExec(kind string, in *Interp) std.Native {
	return func(args []val.Value) (val.Value, error) {
		label := kind + "()"
		if kind == "pyexec" || kind == "py" {
			label = "pyexec()/py() "
		}
		if len(args) != 1 {
			return nil, unierr.New(fmt.Sprintf("%sexpects one string argument", label))
		}
		code, ok := args[0].(val.Str)
		if !ok {
			return nil, unierr.New(fmt.Sprintf("%sexpects one string argument", label))
		}
		br := in.PyBridge
		if kind == "goexec" {
			br = in.GoBridge
		}
		if br == nil {
			prefix := "Python"
			if kind == "goexec" {
				prefix = "Go"
			}
			return nil, unierr.New(prefix + " worker недоступен: интерпретатор не подключён")
		}
		if err := br.Exec(string(code)); err != nil {
			return nil, wrapBridgeErr(kind, err)
		}
		return val.Nul, nil
	}
}

func wrapBridgeErr(kind string, err error) error {
	prefix := "Python error: "
	if kind == "goeval" || kind == "goexec" {
		prefix = "Go error: "
	}
	return unierr.New(prefix + err.Error())
}

func (in *Interp) Interpret(program *ast.Program) error {
	for _, stmt := range program.Statements {
		if err := in.execute(stmt); err != nil {
			return in.toTopErr(err)
		}
	}

	if v, ok := in.GlobalEnv.vars["main"]; ok {
		if fn, ok := v.(*UniFunction); ok && len(fn.Params) == 0 {
			if _, err := in.callValue(fn, nil, -1); err != nil {
				return in.toTopErr(err)
			}
		}
	}
	return nil
}

func (in *Interp) toTopErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*ExitSignal); ok {
		return err
	}
	if isControlSignal(err) {
		return unierr.New(controlSignalText(err))
	}
	if e, ok := err.(*unierr.Error); ok {
		return e
	}
	return unierr.New(err.Error())
}

func (in *Interp) EvalProgram(program *ast.Program) (echo string, err error) {
	for _, stmt := range program.Statements {
		if es, ok := stmt.(*ast.ExpressionStatement); ok {
			v, err := in.evaluate(es.Expr)
			if err != nil {
				return "", in.toTopErr(err)
			}
			if v != nil && v != val.Nul {
				s := pyops.Str(v)
				if echo == "" {
					echo = s
				} else {
					echo += "\n" + s
				}
			}
			continue
		}
		if err := in.execute(stmt); err != nil {
			return "", in.toTopErr(err)
		}
	}
	return echo, nil
}

func isControlSignal(err error) bool {
	switch err.(type) {
	case *returnSignal, *breakSignal, *continueSignal:
		return true
	}
	return false
}

func controlSignalText(err error) string {
	switch err.(type) {
	case *returnSignal:
		return "return вне функции"
	case *breakSignal:
		return "break вне цикла"
	case *continueSignal:
		return "continue вне цикла"
	}
	return "неуловленный сигнал"
}

func (in *Interp) callValue(fnVal val.Value, args []val.Value, line int) (val.Value, error) {
	callErr := func(msg string) *unierr.Error {
		if line >= 0 {
			return unierr.AtLine(msg, line)
		}
		return unierr.New(msg)
	}
	switch fn := fnVal.(type) {
	case *UniFunction:
		if in.runner != nil {
			return in.runner.Call(fn, args, line)
		}
		if len(args) != len(fn.Params) {
			return nil, callErr(fmt.Sprintf("%s expects %d argument(s), got %d",
				fn.Name, len(fn.Params), len(args)))
		}
		callEnv := NewEnvironment(fn.Closure)
		for i, param := range fn.Params {
			callEnv.Define(param, args[i], false)
		}
		oldEnv := in.env
		in.env = callEnv
		err := in.execute(fn.Body)
		in.env = oldEnv
		if err != nil {
			if rs, ok := err.(*returnSignal); ok {
				return rs.Value, nil
			}
			return nil, err
		}
		return val.Nul, nil
	case NativeFunc:
		r, err := fn(args)
		if err != nil {
			if e, ok2 := err.(*unierr.Error); ok2 {
				return nil, e
			}
			if _, sig := err.(*ExitSignal); sig {
				return nil, err
			}
			return nil, unierr.New(err.Error())
		}
		return r, nil
	}
	return nil, callErr(fmt.Sprintf("Value of type %s is not callable", pyTypeOf(fnVal)))
}

func pyTypeOf(v val.Value) string { return pyops.TypeOf(v) }
