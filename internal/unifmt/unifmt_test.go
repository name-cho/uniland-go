package unifmt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/name-cho/uniland-go/internal/lexer"
)

func corpusFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{"../../compat/cases", "../../compat/vmcases"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("нет каталога %s: %v", dir, err)
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".uni") {
				out = append(out, filepath.Join(dir, e.Name()))
			}
		}
	}
	return out
}

func tokenDumps(t *testing.T, src string) []string {
	t.Helper()
	tokens, err := lexer.Tokenize(src)
	if err != nil {
		t.Fatalf("лексер: %v", err)
	}
	var dumps []string
	for _, tk := range tokens {
		dumps = append(dumps, tk.Dump())
	}
	return dumps
}

func TestCorpusTokenStreamUnchanged(t *testing.T) {
	for _, path := range corpusFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			src := normalize(string(raw))
			formatted, err := Format(src)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			before := tokenDumps(t, src)
			after := tokenDumps(t, formatted)
			if len(before) != len(after) {
				t.Fatalf("поток токенов изменился: было %d, стало %d", len(before), len(after))
			}
			for i := range before {
				if before[i] != after[i] {
					t.Fatalf("токен #%d изменился:\nбыло:  %s\nстало: %s", i, before[i], after[i])
				}
			}
		})
	}
}

func TestIdempotent(t *testing.T) {
	for _, path := range corpusFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			once, err := Format(string(raw))
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			twice, err := Format(once)
			if err != nil {
				t.Fatalf("Format#2: %v", err)
			}
			if once != twice {
				t.Fatalf("не идемпотентен:\n--- 1 ---\n%s\n--- 2 ---\n%s", once, twice)
			}
		})
	}
}

func TestBlocksPreserved(t *testing.T) {
	src := "python {\n" +
		"    if True:\n" +
		"        x = 1\n" +
		"}\n" +
		"go {\n" +
		"  fmt.Println(\"x\")\n" +
		"}\n"
	formatted, err := Format(src)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	for _, block := range []string{
		"    if True:\n        x = 1\n",
		"  fmt.Println(\"x\")\n",
	} {
		if !strings.Contains(formatted, block) {
			t.Fatalf("тело блока переформатировано:\n%s", formatted)
		}
	}
}

func TestReindent(t *testing.T) {
	src := "func f(a) {\n" +
		"          if (a > 0) {\n" +
		"     print(a)   \n" +
		"          }\n" +
		"\n\n\n" +
		"  return a\n" +
		"}\n"
	want := "func f(a) {\n" +
		"    if (a > 0) {\n" +
		"        print(a)\n" +
		"    }\n" +
		"\n" +
		"    return a\n" +
		"}\n"
	got, err := Format(src)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if got != want {
		t.Fatalf("неверный отступ:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestCommentsKept(t *testing.T) {
	src := "func f() {\n" +
		"// комментарий\n" +
		"  # и такой\n" +
		"  /* и блочный */\n" +
		"  return 1\n" +
		"}\n"
	got, err := Format(src)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	for _, want := range []string{
		"    // комментарий\n",
		"    # и такой\n",
		"    /* и блочный */\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("комментарий потерян/сдвинут, ждали %q:\n%s", want, got)
		}
	}
}

func TestTrickySources(t *testing.T) {
	cases := map[string]string{
		"multiline_template":   "let s = `первая\nвторая {1 + 1}\nтретья`\nprint(s)\n",
		"python_block":         "let n = 1\npython {\n    x = \"}\"\n    # } в комментарии\n    if True:\n        uni.n = 2\n}\nprint(n)\n",
		"go_block":             "go {\n  s := \"}\"\n  _ = s\n}\n",
		"else_on_brace":        "if (true) {\nprint(1)\n} else {\nprint(2)\n}\n",
		"else_if":              "if (false) {\nprint(1)\n} else if (true) {\nprint(2)\n} else {\nprint(3)\n}\n",
		"continuation":         "let x = [1,\n2,\n3]\nprint(x[0] +\n  1)\n",
		"trailing_comment":     "print(1) // хвост\nprint(2) /* блок */\n",
		"nested_closures":      "func f(a) {\nreturn (b) => {\nreturn () => a + b\n}\n}\nprint(f(1)(2)())\n",
		"semicolons":           "let a = 1; let b = 2; print(a + b)\n",
		"template_with_braces": "let x = 1\nprint(`a{{b}}c{x}d`)\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			formatted, err := Format(src)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			before := tokenDumps(t, src)
			after := tokenDumps(t, formatted)
			if len(before) != len(after) {
				t.Fatalf("поток токенов изменился: было %d, стало %d\n--- formatted ---\n%s", len(before), len(after), formatted)
			}
			for i := range before {
				if before[i] != after[i] {
					t.Fatalf("токен #%d изменился:\nбыло:  %s\nстало: %s\n--- formatted ---\n%s", i, before[i], after[i], formatted)
				}
			}
			twice, err := Format(formatted)
			if err != nil {
				t.Fatalf("Format#2: %v", err)
			}
			if twice != formatted {
				t.Fatalf("не идемпотентен:\n--- 1 ---\n%s\n--- 2 ---\n%s", formatted, twice)
			}
		})
	}
}
