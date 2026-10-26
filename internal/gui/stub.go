//go:build !cgo || (nogui && !fyne)

package gui

import (
	"errors"

	"github.com/name-cho/uniland-go/internal/interp/val"
)

const ErrUnavailableText = "GUI недоступен в этой сборке: нет cgo или включён тег nogui"

var ErrUnavailable = errors.New(ErrUnavailableText)

func Window(title string, width, height int) (val.Value, error) {
	return nil, ErrUnavailable
}

func Label(win val.Value, text string) (val.Value, error) {
	return nil, ErrUnavailable
}

func Button(win val.Value, text string, fn val.Value, call CallFn) (val.Value, error) {
	return nil, ErrUnavailable
}

func Input(win val.Value, initial string) (val.Value, error) {
	return nil, ErrUnavailable
}

func Get(w val.Value) (val.Value, error) {
	return nil, ErrUnavailable
}

func Set(w val.Value, text string) error {
	return ErrUnavailable
}

func Run(win val.Value) error {
	return ErrUnavailable
}

func Close(win val.Value) error {
	return ErrUnavailable
}

func OnClose(win val.Value, fn val.Value, call CallFn) error {
	return ErrUnavailable
}

func Image(win val.Value, path string, width, height int) (val.Value, error) {
	return nil, ErrUnavailable
}

func ImageButton(win val.Value, path string, fn val.Value, width, height int, call CallFn) (val.Value, error) {
	return nil, ErrUnavailable
}

func Gif(win val.Value, path string, width, height int) (val.Value, error) {
	return nil, ErrUnavailable
}

func Video(win val.Value, path string) (val.Value, error) {
	return nil, ErrUnavailable
}

func PickFile(title string) (val.Value, error) {
	return nil, ErrUnavailable
}

func PickSave(title string) (val.Value, error) {
	return nil, ErrUnavailable
}

func PickFolder(title string) (val.Value, error) {
	return nil, ErrUnavailable
}

func Alert(message string) error {
	return ErrUnavailable
}

func Confirm(message string) (bool, error) {
	return false, ErrUnavailable
}

func Prompt(message, initial string) (val.Value, error) {
	return nil, ErrUnavailable
}
