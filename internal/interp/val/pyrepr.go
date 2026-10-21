package val

import (
	"fmt"
	"strings"
	"unicode"
)

func PyRepr(s string) string {
	rs := []rune(s)
	quote := "'"
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = "\""
	}
	var b strings.Builder
	b.WriteString(quote)
	for _, ch := range rs {
		switch {
		case string(ch) == quote:
			b.WriteByte('\\')
			b.WriteString(quote)
		case ch == '\\':
			b.WriteString("\\\\")
		case ch == '\n':
			b.WriteString("\\n")
		case ch == '\r':
			b.WriteString("\\r")
		case ch == '\t':
			b.WriteString("\\t")
		case unicode.IsPrint(ch):
			b.WriteRune(ch)
		default:
			r := uint32(ch)
			switch {
			case r < 0x100:
				fmt.Fprintf(&b, "\\x%02x", r)
			case r < 0x10000:
				fmt.Fprintf(&b, "\\u%04x", r)
			default:
				fmt.Fprintf(&b, "\\U%08x", r)
			}
		}
	}
	b.WriteString(quote)
	return b.String()
}
