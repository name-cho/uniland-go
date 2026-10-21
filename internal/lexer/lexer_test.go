package lexer

import (
	"testing"
)

func TestHelloTokens(t *testing.T) {
	toks, err := Tokenize("let x = 5\nprint(`x = {x}`)\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []Token{
		{TokenKeyword, "let", 1, 1},
		{TokenIdentifier, "x", 1, 5},
		{TokenOperator, "=", 1, 7},
		{TokenNumber, "5", 1, 9},
		{TokenNewline, "\n", 1, 10},
		{TokenIdentifier, "print", 2, 1},
		{TokenPunct, "(", 2, 6},
		{TokenTemplate, "x = {x}", 2, 7},
		{TokenPunct, ")", 2, 16},
		{TokenEOF, "", 3, 1},
	}
	if len(toks) != len(want) {
		t.Fatalf("получено %d токенов, ждали %d", len(toks), len(want))
	}
	for i := range want {
		if toks[i] != want[i] {
			t.Errorf("токен %d: %+v, ждали %+v", i, toks[i], want[i])
		}
	}
}

func TestLineContinuation(t *testing.T) {
	toks, err := Tokenize("let x = 1 +\n2")
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range toks {
		if tk.Type == TokenNewline {
			t.Fatalf("NEWLINE попал внутрь продолженной строки: %+v", toks)
		}
	}
}

func TestPyBlockCapture(t *testing.T) {
	toks, err := Tokenize("python {\n    x = \"скобка } внутри\"\n    uni.q = x\n}\nprint(1)\n")
	if err != nil {
		t.Fatal(err)
	}
	if toks[0].Type != TokenPyBlock {
		t.Fatalf("первый токен не PYBLOCK: %+v", toks[0])
	}
	want := "\n    x = \"скобка } внутри\"\n    uni.q = x\n"
	if toks[0].Value != want {
		t.Errorf("сырой код блока:\n%q\nждали:\n%q", toks[0].Value, want)
	}
	if toks[1].Type != TokenNewline {
		t.Errorf("после блока ждали NEWLINE, получили %+v", toks[1])
	}
}

func TestPyRepr(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"привет", "'привет'"},
		{"it's", "\"it's\""},
		{"a\nb", "'a\\nb'"},
		{"say \"hi\"", "'say \"hi\"'"},
		{"both'and\"", "'both\\'and\"'"},
		{"tab\tx", "'tab\\tx'"},
		{"\x00", "'\\x00'"},
	}
	for _, c := range cases {
		if got := PyRepr(c.in); got != c.want {
			t.Errorf("PyRepr(%q) = %s, ждали %s", c.in, got, c.want)
		}
	}
}
