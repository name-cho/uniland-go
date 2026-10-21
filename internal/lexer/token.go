package lexer

import (
	"fmt"

	"github.com/name-cho/uniland-go/internal/interp/val"
)

const (
	TokenKeyword    = "KEYWORD"
	TokenIdentifier = "IDENTIFIER"
	TokenOperator   = "OPERATOR"
	TokenNumber     = "NUMBER"
	TokenNewline    = "NEWLINE"
	TokenPunct      = "PUNCTUATION"
	TokenTemplate   = "TEMPLATE"
	TokenString     = "STRING"
	TokenPyBlock    = "PYBLOCK"
	TokenGoBlock    = "GOBLOCK"
	TokenEOF        = "EOF"
)

var Keywords = map[string]bool{
	"let": true, "const": true, "func": true, "fn": true, "return": true,
	"if": true, "else": true, "for": true, "while": true,
	"break": true, "continue": true, "try": true, "catch": true,
	"finally": true, "throw": true,
	"true": true, "false": true, "null": true, "in": true, "python": true,
}

var MultiOpsSet = map[string]bool{
	"==": true, "!=": true, ">=": true, "<=": true, "&&": true, "||": true,
	"++": true, "--": true, "+=": true, "-=": true, "*=": true, "/=": true,
	"%=": true, "=>": true,
}

const SingleOps = "+-*/%^=<>!&|"
const Punct = "(){}[],.;:"

var contOps = map[string]bool{}

func init() {
	for _, c := range SingleOps {
		contOps[string(c)] = true
	}
	for op := range MultiOpsSet {
		if op != "++" && op != "--" {
			contOps[op] = true
		}
	}
}

var contPunct = map[string]bool{"(": true, "[": true, "{": true, ",": true, ".": true, ":": true}

type Token struct {
	Type   string
	Value  string
	Line   int
	Column int
}

func (t Token) Dump() string {
	return fmt.Sprintf("Token(%s, %s, line=%d)", t.Type, PyRepr(t.Value), t.Line)
}

func PyRepr(s string) string { return val.PyRepr(s) }
