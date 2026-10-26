package core

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/name-cho/uniland-go/internal/pybridge"
)

func TestExamples(t *testing.T) {
	dir := filepath.Join(testRootDir, "examples")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("нет каталога examples: %v", err)
	}
	_, pyErr := pybridge.FindPython()

	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".uni") {
			continue
		}
		base := strings.TrimSuffix(name, ".uni")
		t.Run(base, func(t *testing.T) {
			if (base == "07-python" || base == "09-mixed") && pyErr != nil {
				t.Skipf("python недоступен: %v", pyErr)
			}
			src := filepath.Join(dir, name)
			wantPath := filepath.Join(dir, base+".out")
			want, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatalf("нет ожидаемого вывода %s: %v", wantPath, err)
			}

			cmd := exec.Command(testExe, "run", src)
			cmd.Dir = dir
			cmd.Stdin = bytes.NewReader(nil)
			var ob, eb bytes.Buffer
			cmd.Stdout, cmd.Stderr = &ob, &eb
			runErr := cmd.Run()

			gotExit := 0
			if runErr != nil {
				ee, ok := runErr.(*exec.ExitError)
				if !ok {
					t.Fatalf("запуск: %v", runErr)
				}
				gotExit = ee.ExitCode()
			}

			if eb.Len() > 0 {
				t.Errorf("пример писал в stderr:\n%s", eb.String())
			}
			if gotExit != 0 {
				t.Errorf("код возврата %d, ожидался 0", gotExit)
			}

			got := strings.ReplaceAll(ob.String(), "\r\n", "\n")
			wantText := strings.ReplaceAll(string(want), "\r\n", "\n")
			if got != wantText {
				t.Errorf("вывод расходится с %s\n--- ожидалось ---\n%s\n--- получено ---\n%s",
					base+".out", wantText, got)
			}
		})
	}
}
