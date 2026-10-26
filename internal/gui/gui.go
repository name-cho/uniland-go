package gui

import "github.com/name-cho/uniland-go/internal/interp/val"

type CallFn func(fn val.Value, args []val.Value) (val.Value, error)
