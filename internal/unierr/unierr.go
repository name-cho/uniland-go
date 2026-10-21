package unierr

import "fmt"

type Error struct {
	Msg     string
	Line    int
	HasLine bool
}

func (e *Error) Error() string {
	if e.HasLine {
		return fmt.Sprintf("Error at line %d: %s", e.Line, e.Msg)
	}
	return e.Msg
}

func New(msg string) *Error { return &Error{Msg: msg} }

func AtLine(msg string, line int) *Error {
	return &Error{Msg: msg, Line: line, HasLine: true}
}
