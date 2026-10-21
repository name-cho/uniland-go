package gui

import (
	"errors"

	"github.com/name-cho/uniland-go/internal/interp/val"
)

const ErrUnavailableText = "GUI недоступен в этой сборке: пересобери с -tags fyne"

var ErrUnavailable = errors.New(ErrUnavailableText)

type CallFn func(fn val.Value, args []val.Value) (val.Value, error)

type Widget struct {
	Type    string
	Text    string
	Key     string
	Checked bool
	OnClick val.Value
}

type WindowSpec struct {
	Title   string
	Width   int
	Height  int
	Widgets []Widget
}
