package std

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/name-cho/uniland-go/internal/interp/val"
)

func jsonDumps(v val.Value, indent *int) (string, error) {
	var b strings.Builder
	nums := new(errPayload)
	wJSON(&b, v, indent, 0, nums)
	if nums.err != "" {
		return "", fmt.Errorf("%s", nums.err)
	}
	return b.String(), nil
}

type errPayload struct{ err string }

func (e *errPayload) Error() string { return e.err }

func wJSON(b *strings.Builder, v val.Value, indent *int, depth int, ep *errPayload) {
	newlineFor := func() (nl string, pad string) {
		if indent == nil {
			return "", ""
		}
		return "\n", strings.Repeat(" ", *indent*(depth+1))
	}
	oneLine := indent == nil
	switch x := v.(type) {
	case nil, val.Null:
		b.WriteString("null")
	case val.Bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case *val.Int:
		b.WriteString(x.N.String())
	case val.Float:
		f := float64(x)
		if math.IsNaN(f) {
			b.WriteString("NaN")
		} else if math.IsInf(f, 1) {
			b.WriteString("Infinity")
		} else if math.IsInf(f, -1) {
			b.WriteString("-Infinity")
		} else {
			b.WriteString(val.FloatStr(f))
		}
	case val.Str:
		b.WriteString(jsonEscapeStr(string(x)))
	case *val.List:
		if len(x.Items) == 0 {
			b.WriteString("[]")
			return
		}
		nl, pad := newlineFor()
		b.WriteByte('[')
		for i, it := range x.Items {
			if i > 0 {
				if oneLine {
					b.WriteString(", ")
				} else {
					b.WriteString(",\n" + pad)
				}
			} else if !oneLine {
				b.WriteString(nl + pad)
			}
			wJSON(b, it, indent, depth+1, ep)
		}
		if !oneLine {
			b.WriteString("\n" + strings.Repeat(" ", *indent*depth))
		}
		b.WriteByte(']')
	case *val.Map:
		if x.Len() == 0 {
			b.WriteString("{}")
			return
		}
		nl, pad := newlineFor()
		b.WriteByte('{')
		for i, kv := range x.Ordered() {
			if i > 0 {
				if oneLine {
					b.WriteString(", ")
				} else {
					b.WriteString(",\n" + pad)
				}
			} else if !oneLine {
				b.WriteString(nl + pad)
			}
			b.WriteString(jsonEscapeStr(kv.K))
			b.WriteString(": ")
			wJSON(b, kv.V, indent, depth+1, ep)
		}
		if !oneLine {
			b.WriteString("\n" + strings.Repeat(" ", *indent*depth))
		}
		b.WriteByte('}')
	default:

		ep.err = fmt.Sprintf("Object of type %s is not JSON serializable", pyTypeNameJSON(v))
	}
}

func pyTypeNameJSON(v val.Value) string {
	switch v.(type) {
	case nil, val.Null:
		return "NoneType"
	case val.Bool:
		return "bool"
	case *val.Int:
		return "int"
	case val.Float:
		return "float"
	}
	return "object"
}

func jsonEscapeStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

type jsonParser struct {
	src  []rune
	pos  int
	line int
	col  int
}

func jsonParse(s string) (val.Value, error) {
	p := &jsonParser{src: []rune(s), line: 1, col: 1}
	p.skipWS()
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	p.skipWS()
	if p.pos != len(p.src) {
		return nil, fmt.Errorf("Extra data: line %d column %d (char %d)", p.line, p.col, p.pos)
	}
	return v, nil
}

func (p *jsonParser) skipWS() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t' || p.src[p.pos] == '\n' || p.src[p.pos] == '\r') {
		p.step()
	}
}

func (p *jsonParser) step() {
	if p.src[p.pos] == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
	p.pos++
}

func (p *jsonParser) errExpect(what string) error {
	return fmt.Errorf("%s: line %d column %d (char %d)", what, p.line, p.col, p.pos)
}

func (p *jsonParser) value() (val.Value, error) {
	if p.pos >= len(p.src) {
		return nil, p.errExpect("Expecting value")
	}
	switch ch := p.src[p.pos]; {
	case ch == '{':
		return p.object()
	case ch == '[':
		return p.array()
	case ch == '"':
		s, err := p.stringVal()
		if err != nil {
			return nil, err
		}
		return val.Str(s), nil
	case ch == '-' || (ch >= '0' && ch <= '9'):
		return p.number()
	case p.hasWord("true"):
		p.advanceWord(4)
		return val.Bool(true), nil
	case p.hasWord("false"):
		p.advanceWord(5)
		return val.Bool(false), nil
	case p.hasWord("null"):
		p.advanceWord(4)
		return val.Nul, nil
	default:
		return nil, p.errExpect("Expecting value")
	}
}

func (p *jsonParser) hasWord(w string) bool {
	if p.pos+len(w) > len(p.src) {
		return false
	}
	return string(p.src[p.pos:p.pos+len(w)]) == w
}

func (p *jsonParser) advanceWord(n int) {
	for i := 0; i < n; i++ {
		p.step()
	}
}

func (p *jsonParser) object() (val.Value, error) {
	p.step()
	m := val.NewMap()
	p.skipWS()
	if p.pos < len(p.src) && p.src[p.pos] == '}' {
		p.step()
		return m, nil
	}
	for {
		p.skipWS()
		if p.pos >= len(p.src) || p.src[p.pos] != '"' {
			if p.pos < len(p.src) && p.src[p.pos] == '}' {
				return nil, p.errExpect("Expecting property name enclosed in double quotes")
			}
			return nil, p.errExpect("Expecting property name enclosed in double quotes")
		}
		key, err := p.stringVal()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		if p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return nil, p.errExpect("Expecting ':' delimiter")
		}
		p.step()
		p.skipWS()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		m.Set(key, v)
		p.skipWS()
		if p.pos >= len(p.src) {
			return nil, p.errExpect("Expecting ',' delimiter")
		}
		if p.src[p.pos] == ',' {
			p.step()
			p.skipWS()

			if p.pos < len(p.src) && p.src[p.pos] == '}' {
				return nil, p.errExpect("Expecting property name enclosed in double quotes")
			}
			continue
		}
		if p.src[p.pos] == '}' {
			p.step()
			return m, nil
		}
		return nil, p.errExpect("Expecting ',' delimiter")
	}
}

func (p *jsonParser) array() (val.Value, error) {
	p.step()
	items := []val.Value{}
	p.skipWS()
	if p.pos < len(p.src) && p.src[p.pos] == ']' {
		p.step()
		return val.NewList(items), nil
	}
	for {
		p.skipWS()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		items = append(items, v)
		p.skipWS()
		if p.pos >= len(p.src) {
			return nil, p.errExpect("Expecting ',' delimiter")
		}
		if p.src[p.pos] == ',' {
			p.step()
			continue
		}
		if p.src[p.pos] == ']' {
			p.step()
			return val.NewList(items), nil
		}
		return nil, p.errExpect("Expecting ',' delimiter")
	}
}

func (p *jsonParser) stringVal() (string, error) {
	p.step()
	var b strings.Builder
	for {
		if p.pos >= len(p.src) {
			return "", p.errExpect("Unterminated string starting at")
		}
		ch := p.src[p.pos]
		if ch == '"' {
			p.step()
			return b.String(), nil
		}
		if ch == '\\' {
			p.step()
			if p.pos >= len(p.src) {
				return "", p.errExpect("Unterminated string starting at")
			}
			switch esc := p.src[p.pos]; esc {
			case '"', '\\', '/':
				b.WriteRune(esc)
				p.step()
			case 'n':
				b.WriteByte('\n')
				p.step()
			case 't':
				b.WriteByte('\t')
				p.step()
			case 'r':
				b.WriteByte('\r')
				p.step()
			case 'b':
				b.WriteByte('\b')
				p.step()
			case 'f':
				b.WriteByte('\f')
				p.step()
			case 'u':
				if p.pos+4 >= len(p.src) {
					return "", p.errExpect("Unterminated string starting at")
				}
				hex := string(p.src[p.pos+1 : p.pos+5])
				code, ok := parseHex(hex)
				if !ok {
					return "", p.errExpect("Invalid \\escape: ")
				}
				for i := 0; i < 5; i++ {
					p.step()
				}
				r := rune(code)
				if utf16.IsSurrogate(r) && p.pos+6 <= len(p.src) && p.src[p.pos] == '\\' &&
					p.pos+1 < len(p.src) && p.src[p.pos+1] == 'u' {
					hex2 := string(p.src[p.pos+2 : p.pos+6])
					if code2, ok2 := parseHex(hex2); ok2 {
						if dec := utf16.DecodeRune(r, rune(code2)); dec != utf8.RuneError {
							for i := 0; i < 6; i++ {
								p.step()
							}
							r = dec
						}
					}
				}
				b.WriteRune(r)
			default:
				return "", p.errExpect("Invalid \\escape")
			}
			continue
		}
		if ch < 0x20 {
			return "", p.errExpect("Invalid control character at")
		}
		b.WriteRune(ch)
		p.step()
	}
}

func parseHex(hex string) (int64, bool) {
	if len(hex) != 4 {
		return 0, false
	}
	n := int64(0)
	for _, c := range hex {
		var d int64
		switch {
		case c >= '0' && c <= '9':
			d = int64(c - '0')
		case c >= 'a' && c <= 'f':
			d = int64(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int64(c-'A') + 10
		default:
			return 0, false
		}
		n = n*16 + d
	}
	return n, true
}

func (p *jsonParser) number() (val.Value, error) {
	start := p.pos
	if p.pos < len(p.src) && p.src[p.pos] == '-' {
		p.step()
	}
	for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
		p.step()
	}
	isFloat := false
	if p.pos < len(p.src) && p.src[p.pos] == '.' {
		isFloat = true
		p.step()
		for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			p.step()
		}
	}
	if p.pos < len(p.src) && (p.src[p.pos] == 'e' || p.src[p.pos] == 'E') {
		isFloat = true
		p.step()
		if p.pos < len(p.src) && (p.src[p.pos] == '+' || p.src[p.pos] == '-') {
			p.step()
		}
		for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
			p.step()
		}
	}
	text := string(p.src[start:p.pos])
	if text == "" || text == "-" {
		return nil, p.errExpect("Expecting value")
	}
	if isFloat {
		f, err := parseFloat64(text)
		if err != nil {
			if isRangeError(err) {
				return val.NewFloat(f), nil
			}
			return nil, p.errExpect("Expecting value")
		}
		return val.NewFloat(f), nil
	}
	n := new(big.Int)
	if _, ok := n.SetString(text, 10); ok {
		return val.NewInt(n), nil
	}
	return nil, p.errExpect("Expecting value")
}
