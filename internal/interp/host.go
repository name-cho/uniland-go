package interp

import (
	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/unierr"
)

func (in *Interp) UniGet(name string) (val.Value, error) {
	return in.GlobalEnv.Get(name)
}

func (in *Interp) UniSet(name string, v val.Value) error {
	in.GlobalEnv.Define(name, v, false)
	return nil
}

func (in *Interp) UniCall(name string, args []val.Value) (val.Value, error) {
	v, err := in.GlobalEnv.Get(name)
	if err != nil {
		if ue, ok := err.(*unierr.Error); ok {
			return nil, ue
		}
		return nil, err
	}
	return in.callValue(v, args, -1)
}
