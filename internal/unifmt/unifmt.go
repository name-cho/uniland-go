package unifmt

import (
	"strings"

	"github.com/name-cho/uniland-go/internal/lexer"
)

const indentUnit = "    "

func Format(src string) (string, error) {
	src = normalize(src)
	tokens, err := lexer.Tokenize(src)
	if err != nil {
		return "", err
	}
	return formatWithTokens(src, tokens), nil
}

func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func formatWithTokens(src string, tokens []lexer.Token) string {
	lines := strings.Split(src, "\n")
	verbatim := verbatimLines(tokens)

	firstOnLine := make(map[int]lexer.Token)
	tokensOnLine := make(map[int][]lexer.Token)
	for _, t := range tokens {
		if t.Type == lexer.TokenEOF {
			continue
		}
		if _, ok := firstOnLine[t.Line]; !ok {
			firstOnLine[t.Line] = t
		}
		tokensOnLine[t.Line] = append(tokensOnLine[t.Line], t)
	}

	var out []string
	brace, paren := 0, 0

	for i, raw := range lines {
		lineNo := i + 1
		content := strings.TrimRight(raw, " \t")

		if verbatim[lineNo] {
			out = append(out, content)
			depthStep(tokensOnLine[lineNo], &brace, &paren)
			continue
		}

		if strings.TrimSpace(content) == "" {

			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}

		body := strings.TrimLeft(content, " \t")
		depth := brace
		if paren > 0 {
			depth++
		}
		if t, ok := firstOnLine[lineNo]; ok && t.Type == lexer.TokenPunct &&
			(t.Value == "}" || t.Value == ")" || t.Value == "]") {
			if depth > 0 {
				depth--
			}
		}
		out = append(out, strings.Repeat(indentUnit, depth)+body)
		depthStep(tokensOnLine[lineNo], &brace, &paren)
	}

	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n") + "\n"
}

func depthStep(lineTokens []lexer.Token, brace, paren *int) {
	for _, t := range lineTokens {
		if t.Type != lexer.TokenPunct {
			continue
		}
		switch t.Value {
		case "{":
			*brace++
		case "}":
			if *brace > 0 {
				*brace--
			}
		case "(", "[":
			*paren++
		case ")", "]":
			if *paren > 0 {
				*paren--
			}
		}
	}
}

func verbatimLines(tokens []lexer.Token) map[int]bool {
	verbatim := make(map[int]bool)
	for _, t := range tokens {
		if t.Type != lexer.TokenPyBlock && t.Type != lexer.TokenGoBlock &&
			t.Type != lexer.TokenTemplate && t.Type != lexer.TokenString {
			continue
		}
		span := strings.Count(t.Value, "\n")
		if span == 0 {
			continue
		}

		for l := t.Line + 1; l <= t.Line+span; l++ {
			verbatim[l] = true
		}
	}
	return verbatim
}
