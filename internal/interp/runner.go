package interp

import (
	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/unierr"
)

type FnRunner interface {
	Call(fn *UniFunction, args []val.Value, line int) (val.Value, error)
}

func (in *Interp) SetRunner(r FnRunner) { in.runner = r }

func (in *Interp) Runner() FnRunner { return in.runner }

func (in *Interp) ApplyBinary(op string, left, right val.Value, line int) (val.Value, error) {
	return in.applyBinary(op, left, right, line)
}

func (in *Interp) IndexValue(obj, index val.Value, line int) (val.Value, error) {
	return in.indexValue(obj, index, line)
}

func (in *Interp) IterableOf(v val.Value) ([]val.Value, error) {
	return iterableOf(v)
}

func (in *Interp) PyExecBlock(code string) error {
	if in.PyBridge == nil {
		return unierr.New("Python worker недоступен: интерпретатор не подключён")
	}
	if err := in.PyBridge.Exec(dedent(code)); err != nil {
		return wrapBridgeErr("pyexec", err)
	}
	return nil
}

func (in *Interp) GoExecBlock(code string) error {
	if in.GoBridge == nil {
		return unierr.New("Go worker недоступен: интерпретатор не подключён")
	}
	if err := in.GoBridge.Exec(dedent(code)); err != nil {
		return wrapBridgeErr("goexec", err)
	}
	return nil
}

func (in *Interp) CallValue(fnVal val.Value, args []val.Value, line int) (val.Value, error) {
	return in.callValue(fnVal, args, line)
}

func (in *Interp) ToTopErr(err error) error { return in.toTopErr(err) }
