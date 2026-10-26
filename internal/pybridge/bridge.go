package pybridge

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/name-cho/uniland-go/internal/interp/val"
)

//go:embed pyworker.py
var workerFS embed.FS

type Host interface {
	UniGet(name string) (val.Value, error)
	UniSet(name string, v val.Value) error
	UniCall(name string, args []val.Value) (val.Value, error)
}

type Bridge struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdoutR    *bufio.Reader
	nextID     int
	out        io.Writer
	errW       io.Writer
	host       Host
	workerPath string
	started    bool
}

func New(out, errW io.Writer) *Bridge {
	return &Bridge{out: out, errW: errW, nextID: 1}
}

func (b *Bridge) SetHost(h Host) { b.host = h }

func FindPython() (string, error) {
	var candidates []string
	if custom := os.Getenv("UNILAND_PYTHON"); custom != "" {
		if _, err := exec.LookPath(custom); err == nil {
			candidates = append(candidates, custom)
		}
	}
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, p)
		}
	}
	for _, py := range candidates {
		if pythonWorks(py) {
			return py, nil
		}
	}
	return "", fmt.Errorf("python не найден (нужен python3 или python в PATH; можно задать UNILAND_PYTHON)")
}

func pythonWorks(py string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	probe := exec.CommandContext(ctx, py, "-c", "print(1)")
	probe.Stdout = io.Discard
	probe.Stderr = io.Discard
	if err := probe.Run(); err == nil {
		return true
	}
	return false
}

func (b *Bridge) startLocked() error {
	if b.started {
		return nil
	}
	data, err := workerFS.ReadFile("pyworker.py")
	if err != nil {
		return fmt.Errorf("worker script missing: %w", err)
	}
	dir, err := os.MkdirTemp("", "uniland-pyworker-*")
	if err != nil {
		return err
	}
	b.workerPath = filepath.Join(dir, "pyworker.py")
	if err := os.WriteFile(b.workerPath, data, 0644); err != nil {
		return err
	}
	py, err := FindPython()
	if err != nil {
		return err
	}
	cmd := exec.Command(py, "-u", b.workerPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Writer(os.Stderr)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить %s: %w", py, err)
	}
	b.cmd = cmd
	b.stdin = stdin
	b.stdoutR = bufio.NewReader(stdout)
	b.started = true
	return nil
}

func (b *Bridge) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.started {
		return
	}
	fmt.Fprintln(b.stdin, `{"op":"shutdown"}`)
	b.stdin.Close()
	_ = b.cmd.Wait()
	os.RemoveAll(filepath.Dir(b.workerPath))
	b.started = false
}

type workerMsg struct {
	ID   int               `json:"id"`
	Kind string            `json:"kind"`
	OK   bool              `json:"ok"`
	Err  string            `json:"err"`
	Val  json.RawMessage   `json:"value"`
	Text string            `json:"text"`
	Op   string            `json:"op"`
	Key  string            `json:"k"`
	V    json.RawMessage   `json:"v"`
	Args []json.RawMessage `json:"args"`
}

func (b *Bridge) call(payload map[string]interface{}) (json.RawMessage, error) {
	b.mu.Lock()
	if err := b.startLocked(); err != nil {
		b.mu.Unlock()
		return nil, err
	}
	id := b.nextID
	b.nextID++
	payload["id"] = id
	line, err := json.Marshal(payload)
	if err != nil {
		b.mu.Unlock()
		return nil, err
	}
	if _, err := b.stdin.Write(append(line, '\n')); err != nil {
		b.mu.Unlock()
		return nil, fmt.Errorf("worker stdin: %w", err)
	}
	for {
		b.mu.Unlock()
		msg, rerr := b.readMsg()
		b.mu.Lock()
		if rerr != nil {
			b.mu.Unlock()
			return nil, rerr
		}
		switch msg.Kind {
		case "out":
			fmt.Fprint(b.out, msg.Text)
			continue
		case "err":
			fmt.Fprint(b.errW, msg.Text)
			continue
		case "req":
			b.replyRequest(msg)
			continue
		}
		if msg.ID == id {
			if !msg.OK {
				b.mu.Unlock()
				return nil, fmt.Errorf("%s", msg.Err)
			}
			b.mu.Unlock()
			return msg.Val, nil
		}
	}
}

func (b *Bridge) reply(resp map[string]interface{}) {
	line, err := json.Marshal(resp)
	if err != nil {
		return
	}
	if b.stdin != nil {
		_, _ = b.stdin.Write(append(line, '\n'))
	}
}

func (b *Bridge) replyRequest(msg *workerMsg) {
	resp := map[string]interface{}{"id": msg.ID}
	switch msg.Op {
	case "uni_get":
		b.mu.Unlock()
		v, err := b.host.UniGet(msg.Key)
		b.mu.Lock()
		if err != nil {
			resp["ok"] = false
			resp["err"] = err.Error()
		} else {
			blob, _ := json.Marshal(Encode(v))
			resp["ok"] = true
			resp["value"] = json.RawMessage(blob)
		}
	case "uni_set":
		b.mu.Unlock()
		v, derr := Decode(msg.V)
		if derr != nil {
			b.mu.Lock()
			resp["ok"] = false
			resp["err"] = derr.Error()
		} else {
			err := b.host.UniSet(msg.Key, v)
			b.mu.Lock()
			if err != nil {
				resp["ok"] = false
				resp["err"] = err.Error()
			} else {
				resp["ok"] = true
			}
		}
	case "uni_call":
		b.mu.Unlock()
		args := make([]val.Value, len(msg.Args))
		derr := error(nil)
		for i, raw := range msg.Args {
			args[i], derr = Decode(raw)
			if derr != nil {
				break
			}
		}
		if derr != nil {
			b.mu.Lock()
			resp["ok"] = false
			resp["err"] = derr.Error()
		} else {
			v, err := b.host.UniCall(msg.Key, args)
			b.mu.Lock()
			if err != nil {
				resp["ok"] = false
				resp["err"] = err.Error()
			} else {
				blob, _ := json.Marshal(Encode(v))
				resp["ok"] = true
				resp["value"] = json.RawMessage(blob)
			}
		}
	default:
		resp["ok"] = false
		resp["err"] = "unknown request: " + msg.Op
	}
	b.reply(resp)
}

func (b *Bridge) Exec(code string) error {
	_, err := b.call(map[string]interface{}{"op": "exec", "code": code})
	return err
}

func (b *Bridge) Eval(code string) (val.Value, error) {
	raw, err := b.call(map[string]interface{}{"op": "eval", "code": code})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return val.Nul, nil
	}
	return Decode(raw)
}

func (b *Bridge) readMsg() (*workerMsg, error) {
	for {
		text, err := b.stdoutR.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("worker stdout закрыт")
		}
		text = strings.TrimRight(text, "\r\n")
		if text == "" {
			continue
		}
		var m workerMsg
		if err := json.Unmarshal([]byte(text), &m); err != nil {
			continue
		}
		return &m, nil
	}
}

func Encode(v val.Value) interface{} {
	switch x := v.(type) {
	case nil, val.Null:
		return nil
	case val.Bool:
		return bool(x)
	case *val.Int:
		return map[string]interface{}{"T": "int", "v": x.N.String()}
	case val.Float:
		return map[string]interface{}{"T": "float", "v": pythonFloatRepr(float64(x))}
	case val.Str:
		return map[string]interface{}{"T": "str", "v": string(x)}
	case *val.List:
		items := make([]interface{}, len(x.Items))
		for i, it := range x.Items {
			items[i] = Encode(it)
		}
		return map[string]interface{}{"T": "list", "v": items}
	case *val.Map:
		obj := make(map[string]interface{}, x.Len())
		for _, kv := range x.Ordered() {
			obj[kv.K] = Encode(kv.V)
		}
		return map[string]interface{}{"T": "obj", "v": obj}
	default:
		return map[string]interface{}{"T": "func"}
	}
}

func pythonFloatRepr(f float64) string {
	return val.FloatStr(f)
}

func Decode(raw json.RawMessage) (val.Value, error) {
	var any interface{}
	if err := json.Unmarshal(raw, &any); err != nil {
		return nil, err
	}
	return decodeAny(any)
}

func decodeAny(v interface{}) (val.Value, error) {
	switch x := v.(type) {
	case nil:
		return val.Nul, nil
	case bool:
		return val.Bool(x), nil
	case float64:

		if x == float64(int64(x)) {
			return val.Int64(int64(x)), nil
		}
		return val.Float(x), nil
	case string:
		return val.Str(x), nil
	case []interface{}:
		items := make([]val.Value, len(x))
		for i, it := range x {
			d, err := decodeAny(it)
			if err != nil {
				return nil, err
			}
			items[i] = d
		}
		return val.NewList(items), nil
	case map[string]interface{}:
		t, _ := x["T"].(string)
		switch t {
		case "int":
			s, _ := x["v"].(string)
			return val.IntFromString(s), nil
		case "float":
			s, _ := x["v"].(string)
			f, err := parseLoginFloat(s)
			if err != nil {
				return nil, err
			}
			return val.Float(f), nil
		case "str":
			s, _ := x["v"].(string)
			return val.Str(s), nil
		case "list":
			rawL, _ := x["v"].([]interface{})
			items := make([]val.Value, len(rawL))
			for i, it := range rawL {
				d, err := decodeAny(it)
				if err != nil {
					return nil, err
				}
				items[i] = d
			}
			return val.NewList(items), nil
		case "obj":
			src, _ := x["v"].(map[string]interface{})
			m := val.NewMap()
			var names []string
			if kn, ok := x["k"].([]interface{}); ok {
				for _, k := range kn {
					name, _ := k.(string)
					names = append(names, name)
				}
			} else {
				for k := range src {
					names = append(names, k)
				}
				sortStrings(names)
			}
			for _, name := range names {
				d, err := decodeAny(src[name])
				if err != nil {
					return nil, err
				}
				m.Set(name, d)
			}
			return m, nil
		case "func", "null":
			return val.Nul, nil
		}
		m := val.NewMap()
		for k, kv := range x {
			d, err := decodeAny(kv)
			if err != nil {
				return nil, err
			}
			m.Set(k, d)
		}
		return m, nil
	}
	return val.Nul, nil
}

func parseLoginFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%g", &f)
	return f, err
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

var _ = big.NewInt
