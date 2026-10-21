package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/name-cho/uniland-go/internal/core"
	"github.com/name-cho/uniland-go/internal/unierr"
	"github.com/name-cho/uniland-go/internal/unifmt"
)

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type rng struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type diagnostic struct {
	Range    rng    `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

type textEdit struct {
	Range   rng    `json:"range"`
	NewText string `json:"newText"`
}

type Server struct {
	out    io.Writer
	docs   map[string]string
	closed bool
}

func Serve(in io.Reader, out io.Writer) error {
	s := &Server{out: out, docs: make(map[string]string)}
	r := bufio.NewReader(in)
	for {
		body, err := readMessage(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var msg message
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}
		if done := s.handle(msg); done {
			return nil
		}
	}
}

func (s *Server) handle(msg message) (done bool) {
	switch msg.Method {
	case "initialize":
		s.reply(msg.ID, map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync":                1,
				"documentFormattingProvider":      true,
				"documentRangeFormattingProvider": false,
			},
			"serverInfo": map[string]any{"name": "uniland-lsp", "version": "2.2.0"},
		})
	case "initialized":

	case "shutdown":
		s.closed = true
		s.reply(msg.ID, nil)
	case "exit":
		return true
	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(msg.Params, &p) == nil {
			s.docs[p.TextDocument.URI] = p.TextDocument.Text
			s.publishDiagnostics(p.TextDocument.URI, p.TextDocument.Text)
		}
	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if json.Unmarshal(msg.Params, &p) == nil && len(p.ContentChanges) > 0 {
			text := p.ContentChanges[len(p.ContentChanges)-1].Text
			s.docs[p.TextDocument.URI] = text
			s.publishDiagnostics(p.TextDocument.URI, text)
		}
	case "textDocument/formatting":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(msg.Params, &p) != nil {
			s.reply(msg.ID, nil)
			return false
		}
		text, ok := s.docs[p.TextDocument.URI]
		if !ok {
			s.reply(msg.ID, nil)
			return false
		}
		formatted, err := unifmt.Format(text)
		if err != nil {
			s.reply(msg.ID, nil)
			return false
		}
		s.reply(msg.ID, []textEdit{{
			Range:   rng{Start: position{0, 0}, End: position{lineCount(text), 0}},
			NewText: formatted,
		}})
	default:
		if len(msg.ID) > 0 {
			s.write(response{
				JSONRPC: "2.0", ID: msg.ID,
				Error: &rpcError{Code: -32601, Message: "method not found: " + msg.Method},
			})
		}
	}
	return false
}

func (s *Server) publishDiagnostics(uri, text string) {
	diags := []diagnostic{}
	if err := core.Check(text); err != nil {
		line := 0
		msg := err.Error()
		if ue, ok := err.(*unierr.Error); ok {
			msg = ue.Msg
			if ue.HasLine {
				line = ue.Line - 1
			}
		}
		if line < 0 {
			line = 0
		}
		diags = append(diags, diagnostic{
			Range:    rng{Start: position{line, 0}, End: position{line, lineLength(text, line)}},
			Severity: 1,
			Source:   "uniland",
			Message:  msg,
		})
	}
	s.write(map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params":  map[string]any{"uri": uri, "diagnostics": diags},
	})
}

func (s *Server) reply(id json.RawMessage, result any) {
	s.write(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) write(v any) {
	body, err := json.Marshal(v)
	if err != nil {
		return
	}
	fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if line == "" {
				return nil, err
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok &&
			strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				length = n
			}
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("lsp: нет Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func lineCount(text string) int { return strings.Count(text, "\n") }

func lineLength(text string, line int) int {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if line < 0 || line >= len(lines) {
		return 0
	}
	return len([]rune(lines[line]))
}
