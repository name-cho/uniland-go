package core

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/name-cho/uniland-go/internal/lexer"
	"github.com/name-cho/uniland-go/internal/parser"
	"github.com/name-cho/uniland-go/internal/pybridge"
)

const parityRootDir = "../.."

func runEngine(t *testing.T, source string, args []string, stdin string, useVM bool) (out, errOut string, exitCode int, runErr error) {
	t.Helper()
	ob, eb := &bytes.Buffer{}, &bytes.Buffer{}
	rt := newRuntime(ob, eb, strings.NewReader(stdin), args)
	defer rt.close()
	rt.setEngine(useVM)
	code, rerr := rt.RunSource(source, false)
	return ob.String(), eb.String(), code, rerr
}

func normalize(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

func parityCase(t *testing.T, name, source string, args []string, stdin string) {
	t.Helper()
	tout, terr, texit, terr2 := runEngine(t, source, args, stdin, false)
	vout, verr, vexit, verr2 := runEngine(t, source, args, stdin, true)
	if tout != vout {
		t.Errorf("[%s] stdout расходится:\n--- tree-walk ---\n%q\n--- vm ---\n%q (vm err: %v)", name, tout, vout, verr2)
	}
	if terr != verr {
		t.Errorf("[%s] stderr расходится:\n--- tree-walk ---\n%q\n--- vm ---\n%q", name, terr, verr)
	}
	if texit != vexit {
		t.Errorf("[%s] exit расходится: tree=%d (%v) vm=%d (%v)", name, texit, terr2, vexit, verr2)
	}
}

func TestVMParityGolden(t *testing.T) {
	wantDir := filepath.Join(parityRootDir, "compat", "expected")
	entries, err := os.ReadDir(wantDir)
	if err != nil {
		t.Fatalf("не найден каталог снапшотов: %v", err)
	}
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		var tc goldenCase
		raw, err := os.ReadFile(filepath.Join(wantDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &tc); err != nil {
			t.Fatalf("снапшот %s: %v", name, err)
		}
		t.Run(strings.TrimSuffix(name, ".json")+"_vm", func(t *testing.T) {
			if tc.Case == "t12_gui.uni" {
				t.Skip("кейс с окнами проверяется golden-тестом в сборке без GUI")
			}
			if tc.Case == "t10_python.uni" {
				if _, err := pybridge.FindPython(); err != nil {
					t.Skipf("python недоступен для py-моста: %v", err)
				}
			}
			src, err := os.ReadFile(filepath.Join(parityRootDir, "compat", "cases", tc.Case))
			if err != nil {
				t.Fatal(err)
			}
			parityCase(t, tc.Case, string(src), tc.Args, tc.Stdin)
		})
	}
}

func TestVMParityCorpus(t *testing.T) {
	casesDir := filepath.Join(parityRootDir, "compat", "vmcases")
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		t.Fatalf("не найден корпус vmcases: %v", err)
	}
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".uni") {
			continue
		}
		if name == "v15_python.uni" && !hasPython(t) {
			continue
		}
		src, err := os.ReadFile(filepath.Join(casesDir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) {
			tokens, lerr := lexer.Tokenize(string(src))
			if lerr != nil {
				t.Fatalf("%s: лексическая ошибка: %v", name, lerr)
			}
			if _, perr := parser.Parse(tokens); perr != nil {
				t.Fatalf("%s: кейс корпуса не парсится: %v", name, perr)
			}
			parityCase(t, name, string(src), nil, "")
		})
	}
}

func hasPython(t *testing.T) bool {
	t.Helper()
	_, err := pybridge.FindPython()
	return err == nil
}

func TestVMParityRepl(t *testing.T) {
	scripts := []struct {
		name  string
		lines string
	}{
		{"echo_expr", "2+2\n1.5*4\n\"тип: \" + to_string(7)\nexit\n"},
		{"multiline_func", "func greet(who) {\n    return `привет, {who}!`\n}\nprint(greet(\"мир\"))\ngreet(42)\nexit()\n"},
		{"blocks_and_errors", "let xs = [1,2,3]\nprint(map(xs, x => x*x))\ntry { print(nope_var) } catch (e) { print(\"ловим:\", e) }\nprint(\"живой\")\n1/0\nprint(\"после деления\")\nquit\n"},
	}
	if hasPython(t) {
		scripts = append(scripts, struct {
			name  string
			lines string
		}{"py_step", "let v = 21\npython {\n    uni.SetInt(\"v\", v * 2)\n}\nprint(v)\nexit\n"})
	}
	for _, sc := range scripts {
		t.Run(sc.name, func(t *testing.T) {
			to, te := &bytes.Buffer{}, &bytes.Buffer{}
			tr := NewRepl(strings.NewReader(sc.lines), to, te)
			tcode := tr.Loop()
			vo, ve := &bytes.Buffer{}, &bytes.Buffer{}
			vr := NewRepl(strings.NewReader(sc.lines), vo, ve)
			vr.rt.setEngine(true)
			vcode := vr.Loop()
			if a, b := normalize(to.String()), normalize(vo.String()); a != b {
				t.Errorf("stdout REPL расходится:\n--- tree ---\n%q\n--- vm ---\n%q", a, b)
			}
			if a, b := normalize(te.String()), normalize(ve.String()); a != b {
				t.Errorf("stderr REPL расходится:\n--- tree ---\n%q\n--- vm ---\n%q", a, b)
			}
			if tcode != vcode {
				t.Errorf("код выхода REPL: tree=%d vm=%d", tcode, vcode)
			}
		})
	}
}
