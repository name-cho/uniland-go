package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func frame(v any) string {
	body, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

func session(t *testing.T, msgs ...any) []map[string]any {
	t.Helper()
	var in bytes.Buffer
	for _, m := range msgs {
		in.WriteString(frame(m))
	}
	var out bytes.Buffer
	if err := Serve(&in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	var got []map[string]any
	r := bufio.NewReader(bytes.NewReader(out.Bytes()))
	for {
		body, err := readMessage(r)
		if err == io.EOF || err != nil {
			break
		}
		var v map[string]any
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatalf("битый ответ: %v", err)
		}
		got = append(got, v)
	}
	return got
}

func TestInitializeAndShutdown(t *testing.T) {
	got := session(t,
		map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}},
		map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"},
		map[string]any{"jsonrpc": "2.0", "method": "exit"},
	)
	if len(got) != 2 {
		t.Fatalf("ждали 2 ответа, получили %d: %v", len(got), got)
	}
	caps := got[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	if caps["documentFormattingProvider"] != true {
		t.Fatalf("нет documentFormattingProvider: %v", caps)
	}
	if caps["textDocumentSync"] != float64(1) {
		t.Fatalf("textDocumentSync: %v", caps["textDocumentSync"])
	}
	if got[1]["result"] != nil {
		t.Fatalf("shutdown должен вернуть null: %v", got[1])
	}
}

func TestDiagnostics(t *testing.T) {
	got := session(t,
		map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		map[string]any{
			"jsonrpc": "2.0", "method": "textDocument/didOpen",
			"params": map[string]any{"textDocument": map[string]any{
				"uri": "file:///bad.uni", "text": "let a = 1\nprint(2 +\n",
			}},
		},
		map[string]any{
			"jsonrpc": "2.0", "method": "textDocument/didOpen",
			"params": map[string]any{"textDocument": map[string]any{
				"uri": "file:///good.uni", "text": "let a = 1\nprint(a)\n",
			}},
		},
	)
	var bad, good bool
	for _, m := range got {
		if m["method"] != "textDocument/publishDiagnostics" {
			continue
		}
		params := m["params"].(map[string]any)
		diags := params["diagnostics"].([]any)
		switch params["uri"] {
		case "file:///bad.uni":
			bad = true
			if len(diags) != 1 {
				t.Fatalf("ждали 1 диагностику, got %v", diags)
			}
			d := diags[0].(map[string]any)
			if d["severity"] != float64(1) || d["source"] != "uniland" {
				t.Fatalf("диагностика: %v", d)
			}
			msg := d["message"].(string)
			if !strings.Contains(msg, "Expected") && !strings.Contains(msg, "Unexpected") {
				t.Fatalf("текст диагностики: %q", msg)
			}
		case "file:///good.uni":
			good = true
			if len(diags) != 0 {
				t.Fatalf("валидный файл дал диагностики: %v", diags)
			}
		}
	}
	if !bad || !good {
		t.Fatalf("не пришло диагностик: %v", got)
	}
}

func TestFormatting(t *testing.T) {
	src := "func f(a) {\n  if (a > 0) {\n  print(a)\n  }\n}\n"
	got := session(t,
		map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}},
		map[string]any{
			"jsonrpc": "2.0", "method": "textDocument/didOpen",
			"params": map[string]any{"textDocument": map[string]any{
				"uri": "file:///f.uni", "text": src,
			}},
		},
		map[string]any{
			"jsonrpc": "2.0", "id": 2, "method": "textDocument/formatting",
			"params": map[string]any{"textDocument": map[string]any{"uri": "file:///f.uni"}},
		},
	)
	var edits []any
	for _, m := range got {
		if m["id"] == float64(2) {
			edits = m["result"].([]any)
		}
	}
	if len(edits) != 1 {
		t.Fatalf("ждали один TextEdit, got %v", got)
	}
	ed := edits[0].(map[string]any)
	newText := ed["newText"].(string)
	want := "func f(a) {\n    if (a > 0) {\n        print(a)\n    }\n}\n"
	if newText != want {
		t.Fatalf("форматирование:\n--- got ---\n%s\n--- want ---\n%s", newText, want)
	}
}

func TestUnknownMethodAndBrokenFrame(t *testing.T) {
	var in bytes.Buffer
	in.WriteString("Content-Length: 5\r\n\r\n{bad}")
	in.WriteString(frame(map[string]any{"jsonrpc": "2.0", "id": 7, "method": "no/such"}))
	var out bytes.Buffer
	if err := Serve(&in, &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if !strings.Contains(out.String(), "-32601") {
		t.Fatalf("ждали ошибку method not found: %s", out.String())
	}
}
