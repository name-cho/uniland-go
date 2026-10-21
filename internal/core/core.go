package core

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/name-cho/uniland-go/internal/gobridge"
	"github.com/name-cho/uniland-go/internal/interp"
	"github.com/name-cho/uniland-go/internal/lexer"
	"github.com/name-cho/uniland-go/internal/parser"
	"github.com/name-cho/uniland-go/internal/pybridge"
	"github.com/name-cho/uniland-go/internal/unierr"
	"github.com/name-cho/uniland-go/internal/vm"
)

func NormalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

type Runtime struct {
	interp *interp.Interp
	engine *vm.Engine
	py     *pybridge.Bridge
	gob    *gobridge.Bridge
	out    io.Writer
	errW   io.Writer
}

func newRuntime(out, errW io.Writer, stdin io.Reader, scriptArgs []string) *Runtime {
	it := interp.New(out, errW, stdin, scriptArgs)
	pb := pybridge.New(out, errW)
	pb.SetHost(it)
	gb := gobridge.New(out, errW)
	gb.SetHost(it)
	gb.SetPy(pb)
	it.PyBridge = pb
	it.GoBridge = gb
	rt := &Runtime{interp: it, py: pb, gob: gb, out: out, errW: errW}
	rt.setEngine(os.Getenv("UNILAND_TREEWALK") != "1")
	return rt
}

func (rt *Runtime) setEngine(useVM bool) {
	if !useVM {
		rt.interp.SetRunner(nil)
		rt.engine = nil
		return
	}
	if rt.engine == nil {
		rt.engine = vm.New(rt.interp)
	}
	rt.interp.SetRunner(rt.engine)
}

func (rt *Runtime) SetSharedRdIn(r *bufio.Reader) { rt.interp.SetRdIn(r) }

func (rt *Runtime) close() {
	rt.py.Close()
}

func Run(source string, debug bool, scriptArgs []string) (code int, err error) {
	rt := newRuntime(os.Stdout, os.Stderr, os.Stdin, scriptArgs)
	defer rt.close()
	return rt.RunSource(source, debug)
}

func Check(source string) error {
	tokens, err := lexer.Tokenize(NormalizeNewlines(source))
	if err != nil {
		return asUniErr(err)
	}
	_, err = parser.Parse(tokens)
	return asUniErr(err)
}

func (rt *Runtime) RunSource(source string, debug bool) (int, error) {
	src := NormalizeNewlines(source)
	tokens, err := lexer.Tokenize(src)
	if err != nil {
		return 1, asUniErr(err)
	}
	if debug {

		fmt.Fprintln(rt.out, "Tokens:")
		for _, tk := range tokens {
			fmt.Fprintln(rt.out, "  "+tk.Dump())
		}
	}
	program, err := parser.Parse(tokens)
	if err != nil {
		return 1, asUniErr(err)
	}
	var rerr error
	if rt.engine != nil {
		rerr = rt.engine.Interpret(program)
	} else {
		rerr = rt.interp.Interpret(program)
	}
	if rerr != nil {
		return finishErr(rerr)
	}
	return 0, nil
}

func (rt *Runtime) StepSource(source string) (echo string, exit bool, uerr error) {
	tokens, err := lexer.Tokenize(NormalizeNewlines(source))
	if err != nil {
		return "", false, asUniErr(err)
	}
	program, err := parser.Parse(tokens)
	if err != nil {
		return "", false, asUniErr(err)
	}
	if rt.engine != nil {
		echo, err = rt.engine.EvalProgram(program)
	} else {
		echo, err = rt.interp.EvalProgram(program)
	}
	if err != nil {
		switch e := err.(type) {
		case *interp.ExitSignal:
			return "", true, nil
		case *unierr.Error:
			return "", false, e
		}
		return "", false, unierr.New(err.Error())
	}
	return echo, false, nil
}

func finishErr(err error) (int, error) {
	switch e := err.(type) {
	case *interp.ExitSignal:
		return e.Code, nil
	case *unierr.Error:
		return 1, e
	}
	return 1, unierr.New(err.Error())
}

type Repl struct {
	rt   *Runtime
	br   *bufio.Reader
	out  io.Writer
	errW io.Writer
	buf  string
}

func NewRepl(stdin io.Reader, out, errW io.Writer) *Repl {
	br := bufio.NewReader(stdin)
	rt := newRuntime(out, errW, br, nil)
	rt.SetSharedRdIn(br)
	return &Repl{rt: rt, br: br, out: out, errW: errW}
}

func (r *Repl) Loop() int {
	fmt.Fprintln(r.out, "UniLand 2.2.0 — интерактивный режим.")
	fmt.Fprintln(r.out, "Введи код и Enter. Выход: exit, quit или Ctrl+D.")
	for {
		prompt := "uni> "
		if r.buf != "" {
			prompt = "... "
		}
		fmt.Fprint(r.out, prompt)

		line, err := r.br.ReadString('\n')
		if err != nil && line == "" {

			fmt.Fprintln(r.out)
			break
		}
		line = strings.TrimSuffix(NormalizeNewlines(line), "\n")

		if r.buf == "" {
			switch strings.TrimSpace(line) {
			case "exit", "exit()", "quit", "quit()":
				return 0
			}
		}
		if r.buf == "" {
			r.buf = line
		} else {
			r.buf = r.buf + "\n" + line
		}
		if strings.TrimSpace(r.buf) != "" && !balanced(r.buf) {
			continue
		}
		source := r.buf
		r.buf = ""
		if strings.TrimSpace(source) == "" {
			continue
		}

		echo, exit, uerr := r.rt.StepSource(source)
		if uerr != nil {
			fmt.Fprintf(r.errW, "UniLand error: %s\n", uerr)
			continue
		}
		if exit {

			break
		}
		if echo != "" {
			fmt.Fprintln(r.out, echo)
		}
	}
	return 0
}

func asUniErr(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*unierr.Error); ok {
		return e
	}
	return unierr.New(err.Error())
}

func balanced(src string) bool {
	pairs := map[byte]byte{')': '(', ']': '[', '}': '{'}
	var stack []byte
	inStr := byte(0)
	prev := byte(0)
	for i := 0; i < len(src); i++ {
		ch := src[i]
		if inStr != 0 {
			if ch == inStr && prev != '\\' {
				inStr = 0
			}
		} else if ch == '"' || ch == '`' || ch == '\'' {
			inStr = ch
		} else if ch == '(' || ch == '[' || ch == '{' {
			stack = append(stack, ch)
		} else if ch == ')' || ch == ']' || ch == '}' {
			if len(stack) > 0 && stack[len(stack)-1] == pairs[ch] {
				stack = stack[:len(stack)-1]
			}
		}
		prev = ch
	}
	return len(stack) == 0 && inStr == 0
}
