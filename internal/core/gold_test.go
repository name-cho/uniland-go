package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/name-cho/uniland-go/internal/pybridge"
)

var (
	testRootDir string
	testExe     string
)

func findPython() string {
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func TestMain(m *testing.M) {
	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(thisFile)
	testRootDir = filepath.Dir(filepath.Dir(pkgDir))

	tmp, err := os.MkdirTemp("", "uniland-go-gold-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	exe := filepath.Join(tmp, "uniland-go.exe")
	if runtime.GOOS != "windows" {
		exe = filepath.Join(tmp, "uniland-go")
	}
	build := exec.Command("go", "build", "-o", exe, ".")
	build.Dir = testRootDir
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("golden: go build: " + err.Error())
	}
	testExe = exe
	os.Exit(m.Run())
}

type goldenCase struct {
	Case   string   `json:"case"`
	Args   []string `json:"args"`
	Stdin  string   `json:"stdin"`
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
	Exit   int      `json:"exit"`
}

func TestGolden(t *testing.T) {
	wantDir := filepath.Join(testRootDir, "compat", "expected")
	entries, err := os.ReadDir(wantDir)
	if err != nil {
		t.Fatalf("не найден каталог снапшотов: %v", err)
	}
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		t.Run(strings.TrimSuffix(name, ".json"), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(wantDir, name))
			if err != nil {
				t.Fatal(err)
			}
			var tc goldenCase
			if err := json.Unmarshal(raw, &tc); err != nil {
				t.Fatalf("снапшот %s: %v", name, err)
			}

			if tc.Case == "t10_python.uni" {
				if _, err := pybridge.FindPython(); err != nil {
					t.Skipf("python недоступен для py-моста: %v", err)
				}
			}

			casePath := filepath.Join(testRootDir, "compat", "cases", tc.Case)
			argv := append([]string{"run", casePath}, tc.Args...)

			cmd := exec.Command(testExe, argv...)

			cmd.Dir = filepath.Dir(casePath)
			var ob, eb bytes.Buffer
			cmd.Stdout = &ob
			cmd.Stderr = &eb
			if tc.Stdin != "" {
				cmd.Stdin = strings.NewReader(tc.Stdin)
			} else {

				cmd.Stdin = bytes.NewReader(nil)
			}
			runErr := cmd.Run()
			if runErr == nil && len(tc.Stdout)+len(tc.Stderr) == 0 {

			}
			gotExit := 0
			if runErr != nil {

				if ee, ok := runErr.(*exec.ExitError); ok {
					gotExit = ee.ExitCode()
				} else {
					t.Fatalf("запуск: %v", runErr)
				}
			}

			gotOut := strings.ReplaceAll(ob.String(), "\r\n", "\n")
			gotErr := strings.ReplaceAll(eb.String(), "\r\n", "\n")
			wantOut := strings.ReplaceAll(tc.Stdout, "\r\n", "\n")
			wantErr := strings.ReplaceAll(tc.Stderr, "\r\n", "\n")

			if gotOut != wantOut {
				t.Errorf("stdout расходится:\n--- ожидалось ---\n%s\n--- получено ---\n%s", wantOut, gotOut)
			}
			if gotErr != wantErr {
				t.Errorf("stderr расходится:\n--- ожидалось ---\n%s\n--- получено ---\n%s", wantErr, gotErr)
			}
			if gotExit != tc.Exit {
				t.Errorf("exit: ожидалось %d, получено %d", tc.Exit, gotExit)
			}
		})
	}
}

func TestCliBasics(t *testing.T) {
	t.Run("version", func(t *testing.T) {
		out, err := exec.Command(testExe, "-v").Output()
		if err != nil {
			t.Fatal(err)
		}
		if got := string(out); got != "UniLand 2.2.0\n" {
			t.Errorf("-v: ожидался баннер, получено %q", got)
		}
	})

	t.Run("check_ok", func(t *testing.T) {

		path := filepath.Join(testRootDir, "compat", "cases", "hello.uni")
		var ob bytes.Buffer
		cmd := exec.Command(testExe, "-c", path)
		cmd.Stdout = &ob
		code := cmd.Run()
		if got := ob.String(); got != fmt.Sprintf("OK: %s — синтаксис в порядке\n", path) {
			t.Errorf("-c success: stdout %q", got)
		}
		if got := exitOf(code); got != 0 {
			t.Errorf("-c success: exit = %d", got)
		}
	})

	t.Run("check_syntax_error", func(t *testing.T) {

		src := "print(2 +\n{(}"
		path := filepath.Join(t.TempDir(), "broken.uni")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		var ob, eb bytes.Buffer
		cmd := exec.Command(testExe, "-c", path)
		cmd.Stdout = &ob
		cmd.Stderr = &eb
		code := cmd.Run()
		if got := eb.String(); !strings.HasPrefix(got, "UniLand error: Error at line 2: ") {
			t.Errorf("-c failure: stderr %q", got)
		}
		if ob.Len() != 0 {
			t.Errorf("-c failure: stdout не пуст: %q", ob.String())
		}
		if got := exitOf(code); got != 1 {
			t.Errorf("-c failure: exit = %d", got)
		}
	})

	t.Run("run_missing_file", func(t *testing.T) {

		var ob, eb bytes.Buffer
		cmd := exec.Command(testExe, "run", "nope-definitely-missing.uni")
		cmd.Stdout = &ob
		cmd.Stderr = &eb
		code := cmd.Run()
		if got := eb.String(); got != "Error: file 'nope-definitely-missing.uni' not found.\n" {
			t.Errorf("run missing: stderr %q", got)
		}
		if got := exitOf(code); got != 1 {
			t.Errorf("run missing: exit = %d", got)
		}
	})

	t.Run("check_missing_file", func(t *testing.T) {

		var ob, eb bytes.Buffer
		cmd := exec.Command(testExe, "-c", "nope-definitely-missing.uni")
		cmd.Stdout = &ob
		cmd.Stderr = &eb
		code := cmd.Run()
		if got := eb.String(); got != "Error: file 'nope-definitely-missing.uni' not found.\n" {
			t.Errorf("-c missing: stderr %q", got)
		}
		if got := exitOf(code); got != 1 {
			t.Errorf("-c missing: exit = %d", got)
		}
	})
}

func runHelp(t *testing.T) (string, int) {
	cmd := exec.Command(testExe, "--help")
	var ob bytes.Buffer
	cmd.Stdout = &ob
	code := cmd.Run()
	return ob.String(), exitOf(code)
}

func exitOf(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}

var danglingIO io.Writer = io.Discard
