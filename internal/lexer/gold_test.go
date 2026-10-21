package lexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenTokens(t *testing.T) {
	casesDir := filepath.Join("..", "..", "compat", "cases")
	snapDir := filepath.Join("..", "..", "compat", "expected-tokens")

	matches, err := filepath.Glob(filepath.Join(casesDir, "*.uni"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("кейсы не найдены: %v (%v)", matches, err)
	}
	run := false
	for _, path := range matches {
		snap := filepath.Join(snapDir, strings.TrimSuffix(filepath.Base(path), ".uni")+".txt")
		want, err := os.ReadFile(snap)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		run = true
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		toks, lerr := Tokenize(string(src))
		if lerr != nil {
			t.Errorf("%s: ошибка лексера: %v", filepath.Base(path), lerr)
			continue
		}
		var got strings.Builder
		for _, tk := range toks {
			got.WriteString(tk.Dump())
			got.WriteString("\n")
		}
		if got.String() != string(want) {
			t.Errorf("%s: дамп токенов расходится со снапшотом эталона", filepath.Base(path))
			gotLines := strings.Split(strings.TrimSuffix(got.String(), "\n"), "\n")
			wantLines := strings.Split(strings.TrimSuffix(string(want), "\n"), "\n")
			for i := 0; i < max(len(gotLines), len(wantLines)); i++ {
				g, w := "", ""
				if i < len(gotLines) {
					g = gotLines[i]
				}
				if i < len(wantLines) {
					w = wantLines[i]
				}
				if g != w {
					t.Errorf("  строка %d:\n    Go:     %s\n    эталон: %s", i+1, g, w)
					break
				}
			}
		}
	}
	if !run {
		t.Skip("снапшотов токенов нет — прогнать compat/tokens.py")
	}
}
