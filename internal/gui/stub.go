//go:build !fyne

package gui

import (
	"github.com/name-cho/uniland-go/internal/interp/val"
)

func Available() bool { return false }

func RunWindow(spec WindowSpec, call CallFn) (map[string]val.Value, error) {
	return nil, ErrUnavailable
}

func Alert(title, text string) error { return ErrUnavailable }

func Ask(title, text, initial string) (val.Value, error) { return nil, ErrUnavailable }

func Confirm(title, text string) (bool, error) { return false, ErrUnavailable }
