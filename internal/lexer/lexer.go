package lexer

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/name-cho/uniland-go/internal/unierr"
)

type Lexer struct {
	source   []rune
	tokens   []Token
	i, n     int
	line     int
	col      int
	brackets []rune
}

func Tokenize(source string) ([]Token, error) {
	rs := []rune(source)
	l := &Lexer{source: rs, line: 1, col: 1, n: len(rs)}
	return l.tokenize()
}

func (l *Lexer) advance(count int) {
	for k := 0; k < count; k++ {
		if l.i < l.n {
			if l.source[l.i] == '\n' {
				l.line++
				l.col = 1
			} else {
				l.col++
			}
			l.i++
		}
	}
}

func (l *Lexer) add(t, value string, line, col int) {
	l.tokens = append(l.tokens, Token{Type: t, Value: value, Line: line, Column: col})
}

func (l *Lexer) startsWith(s string) bool {
	rs := []rune(s)
	if l.i+len(rs) > l.n {
		return false
	}
	for k, r := range rs {
		if l.source[l.i+k] != r {
			return false
		}
	}
	return true
}

func (l *Lexer) startsWithRune(rs []rune) bool {
	if l.i+len(rs) > l.n {
		return false
	}
	for k, r := range rs {
		if l.source[l.i+k] != r {
			return false
		}
	}
	return true
}

func (l *Lexer) tokenize() ([]Token, error) {
	src := l.source
	for l.i < l.n {
		c := src[l.i]

		if c == ' ' || c == '\t' || c == '\r' {
			l.advance(1)
			continue
		}

		if c == '\n' {
			l.maybeNewline()
			l.advance(1)
			continue
		}

		if l.startsWith("//") || c == '#' {
			for l.i < l.n && src[l.i] != '\n' {
				l.advance(1)
			}
			continue
		}
		if l.startsWith("/*") {
			for l.i < l.n && !l.startsWith("*/") {
				l.advance(1)
			}
			l.advance(2)
			continue
		}

		line, col := l.line, l.col

		if c == '"' {
			if err := l.readString(line, col); err != nil {
				return nil, err
			}
			continue
		}

		if c == '`' {
			if err := l.readTemplate(line, col); err != nil {
				return nil, err
			}
			continue
		}

		if unicode.IsDigit(c) || (c == '.' && l.i+1 < l.n && unicode.IsDigit(src[l.i+1])) {
			l.readNumber(line, col)
			continue
		}

		if unicode.IsLetter(c) || c == '_' {
			l.readWord(line, col)
			continue
		}

		if l.i+1 < l.n {
			two := string(src[l.i : l.i+2])
			if MultiOpsSet[two] {
				l.add(TokenOperator, two, line, col)
				l.advance(2)
				continue
			}
		}

		if strings.ContainsRune(SingleOps, c) {
			l.add(TokenOperator, string(c), line, col)
			l.advance(1)
			continue
		}

		if strings.ContainsRune(Punct, c) {
			if c == '(' || c == '[' || c == '{' {
				l.brackets = append(l.brackets, c)
			} else if c == ')' || c == ']' || c == '}' {
				if len(l.brackets) > 0 {
					l.brackets = l.brackets[:len(l.brackets)-1]
				}
			}
			l.add(TokenPunct, string(c), line, col)
			l.advance(1)
			continue
		}

		return nil, unierr.AtLine(fmt.Sprintf("Unexpected character %s", PyRepr(string(c))), line)
	}

	l.stripTrailingNewline()
	l.add(TokenEOF, "", l.line, l.col)
	return l.tokens, nil
}

func (l *Lexer) last() *Token {
	if len(l.tokens) == 0 {
		return nil
	}
	return &l.tokens[len(l.tokens)-1]
}

func (l *Lexer) maybeNewline() {
	last := l.last()
	if last == nil || last.Type == TokenNewline {
		return
	}
	if len(l.brackets) > 0 && (l.brackets[len(l.brackets)-1] == '(' || l.brackets[len(l.brackets)-1] == '[') {
		return
	}
	if last.Type == TokenOperator && contOps[last.Value] {
		return
	}
	if last.Type == TokenPunct && contPunct[last.Value] {
		return
	}
	l.add(TokenNewline, "\n", l.line, l.col)
}

func (l *Lexer) stripTrailingNewline() {
	for len(l.tokens) > 0 && l.tokens[len(l.tokens)-1].Type == TokenNewline {
		l.tokens = l.tokens[:len(l.tokens)-1]
	}
}

func (l *Lexer) readNumber(line, col int) {
	src := l.source
	start := l.i
	seenDot := false
	for l.i < l.n {
		ch := src[l.i]
		if unicode.IsDigit(ch) {
			l.advance(1)
		} else if ch == '.' && !seenDot {
			seenDot = true
			l.advance(1)
		} else {
			break
		}
	}
	l.add(TokenNumber, string(src[start:l.i]), line, col)
}

func (l *Lexer) readWord(line, col int) {
	src := l.source
	start := l.i
	for l.i < l.n && (unicode.IsLetter(src[l.i]) || unicode.IsDigit(src[l.i]) || src[l.i] == '_') {
		l.advance(1)
	}
	word := string(src[start:l.i])

	if word == "python" {
		j := l.i
		for j < l.n && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r' || src[j] == '\n') {
			j++
		}
		if j < l.n && src[j] == '{' {
			code := l.capturePythonBlock(j)
			l.add(TokenPyBlock, code, line, col)
			return
		}
	}

	if word == "go" {
		j := l.i
		for j < l.n && (src[j] == ' ' || src[j] == '\t' || src[j] == '\r' || src[j] == '\n') {
			j++
		}
		if j < l.n && src[j] == '{' {
			code := l.capturePythonBlock(j)
			l.add(TokenGoBlock, code, line, col)
			return
		}
	}

	if Keywords[word] {
		l.add(TokenKeyword, word, line, col)
	} else {
		l.add(TokenIdentifier, word, line, col)
	}
}

func (l *Lexer) capturePythonBlock(braceIndex int) string {
	for l.i < braceIndex {
		l.advance(1)
	}
	src := l.source
	l.advance(1)
	start := l.i
	depth := 1
	for l.i < l.n && depth > 0 {
		ch := src[l.i]

		if ch == '"' || ch == '\'' {
			if l.startsWithRune([]rune{ch, ch, ch}) {
				l.advance(3)
				for l.i < l.n && !l.startsWithRune([]rune{ch, ch, ch}) {
					l.advance(1)
				}
				l.advance(3)
			} else {
				l.advance(1)
				for l.i < l.n && src[l.i] != ch {
					if src[l.i] == '\\' {
						l.advance(1)
					}
					l.advance(1)
				}
				l.advance(1)
			}
			continue
		}
		if ch == '#' {
			for l.i < l.n && src[l.i] != '\n' {
				l.advance(1)
			}
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				break
			}
		}
		l.advance(1)
	}
	code := string(src[start:l.i])
	l.advance(1)
	return code
}

func (l *Lexer) readString(line, col int) error {
	l.advance(1)
	var out []rune
	for l.i < l.n && l.source[l.i] != '"' {
		ch := l.source[l.i]
		if ch == '\\' {
			l.advance(1)
			if l.i >= l.n {
				break
			}
			esc := l.source[l.i]
			out = append(out, unescape(runeEscapesStr, esc, esc))
			l.advance(1)
		} else {
			out = append(out, ch)
			l.advance(1)
		}
	}
	if l.i >= l.n {
		return unierr.AtLine("Unterminated string", line)
	}
	l.advance(1)
	l.add(TokenString, string(out), line, col)
	return nil
}

func (l *Lexer) readTemplate(line, col int) error {
	l.advance(1)
	var out []rune
	for l.i < l.n && l.source[l.i] != '`' {
		ch := l.source[l.i]
		if ch == '\\' {
			l.advance(1)
			if l.i >= l.n {
				break
			}
			esc := l.source[l.i]
			out = append(out, unescape(runeEscapesTpl, esc, esc))
			l.advance(1)
		} else {
			out = append(out, ch)
			l.advance(1)
		}
	}
	if l.i >= l.n {
		return unierr.AtLine("Unterminated template string", line)
	}
	l.advance(1)
	l.add(TokenTemplate, string(out), line, col)
	return nil
}

var runeEscapesStr = map[rune]rune{'n': '\n', 't': '\t', 'r': '\r', '"': '"', '\\': '\\', '0': 0}
var runeEscapesTpl = map[rune]rune{'n': '\n', 't': '\t', 'r': '\r', '`': '`', '\\': '\\', '{': '{', '}': '}'}

func unescape(m map[rune]rune, esc, defaultRune rune) rune {
	if v, ok := m[esc]; ok {
		return v
	}
	return defaultRune
}
