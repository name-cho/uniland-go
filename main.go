package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/name-cho/uniland-go/internal/core"
	"github.com/name-cho/uniland-go/internal/lsp"
	"github.com/name-cho/uniland-go/internal/unifmt"
)

const version = "2.2.0"

const versionLine = "UniLand " + version

const usageLine = "usage: uniland [-h] [-v] [-e CODE] [-i] [-c FILE] {run,eval,repl,check} ..."

func main() {
	os.Exit(dispatch(os.Args[1:]))
}

type cliArgs struct {
	version     bool
	help        bool
	checkFile   string
	evalCode    string
	interactive bool
	command     string
	runFile     string
	runDebug    bool
	checkFile2  string
	evalCode2   string
	scriptArgs  []string
}

func dispatch(args []string) int {
	if len(args) > 0 && args[0] == "fmt" {
		return cmdFmt(args[1:])
	}

	if len(args) > 0 && args[0] == "lsp" {
		return cmdLSP()
	}
	a := parseArgs(args)

	if a.help {
		printHelp(os.Stdout)
		return 0
	}
	if a.version {
		fmt.Println(versionLine)
		return 0
	}

	if a.checkFile != "" {
		return cmdCheckFile(a.checkFile)
	}
	if a.evalCode != "" {
		return cmdEval(a.evalCode)
	}
	if a.interactive {
		return cmdRepl()
	}
	switch a.command {
	case "run":
		return cmdRun(a.runFile, a.runDebug, a.scriptArgs)
	case "eval":
		if a.evalCode2 == "" {
			return usageErr("the following arguments are required: code")
		}
		return cmdEval(a.evalCode2)
	case "check":
		if a.checkFile2 == "" {
			return usageErr("the following arguments are required: file")
		}
		return cmdCheckFile(a.checkFile2)
	case "repl":
		return cmdRepl()
	default:

		return cmdRepl()
	}
}

func parseArgs(args []string) cliArgs {
	a := cliArgs{}
	for i := 0; i < len(args); i++ {
		s := args[i]
		switch s {
		case "-v", "--version":
			a.version = true
		case "-h", "--help":
			a.help = true
		case "-e", "--eval":
			if i+1 >= len(args) {
				usageErr("argument -e/--eval: expected one argument")
			}
			i++
			a.evalCode = args[i]
		case "-i", "--interactive":
			a.interactive = true
		case "-c", "--check":
			if i+1 >= len(args) {
				usageErr("argument -c/--check: expected one argument")
			}
			i++
			a.checkFile = args[i]
		default:
			if a.command == "" {
				a.command = s

				if a.command == "run" {
					i++
					for ; i < len(args); i++ {
						x := args[i]
						if len(a.runFile) == 0 && (x == "-d" || x == "--debug") {
							a.runDebug = true
							continue
						}
						if a.runFile == "" && strings.HasPrefix(x, "-") {
							if len(a.scriptArgs) > 0 || a.runDebug {
								a.scriptArgs = append(a.scriptArgs, x)
								continue
							}
							a.scriptArgs = append(a.scriptArgs, args[i+1:]...)
							a.scriptArgs = append([]string{x}, a.scriptArgs...)
							break
						}
						if a.runFile == "" {
							a.runFile = x
							continue
						}
						a.scriptArgs = append(a.scriptArgs, x)
					}
				} else if a.command == "eval" {
					if i+1 < len(args) {
						a.evalCode2 = args[i+1]
					}
					i++
				} else if a.command == "check" {
					if i+1 < len(args) {
						a.checkFile2 = args[i+1]
					}
					i++
				}
			} else {
				usageErr(fmt.Sprintf("unrecognized arguments: %s", args[i]))
			}
		}
	}
	return a
}

func readSource(path string) (string, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: file '%s' not found.\n", path)
		return "", 1
	}
	return core.NormalizeNewlines(string(data)), 0
}

func cmdRun(path string, debug bool, scriptArgs []string) int {
	if path == "" {
		return usageErr("the following arguments are required: file")
	}
	src, code := readSource(path)
	if code != 0 {
		return code
	}
	exit, err := core.Run(src, debug, scriptArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "UniLand error: %s\n", err)
		return 1
	}
	return exit
}

func cmdEval(code string) int {
	exit, err := core.Run(code, false, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "UniLand error: %s\n", err)
		return 1
	}
	return exit
}

func cmdCheckFile(path string) int {
	src, code := readSource(path)
	if code != 0 {
		return code
	}
	if err := core.Check(src); err != nil {
		fmt.Fprintf(os.Stderr, "UniLand error: %s\n", err)
		return 1
	}
	fmt.Printf("OK: %s — синтаксис в порядке\n", path)
	return 0
}

func cmdRepl() int {
	return core.NewRepl(os.Stdin, os.Stdout, os.Stderr).Loop()
}

func cmdLSP() int {
	if err := lsp.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "UniLand error: %s\n", err)
		return 1
	}
	return 0
}

func cmdFmt(args []string) int {
	write, list := false, false
	var file string
	for _, a := range args {
		switch a {
		case "-w", "--write":
			write = true
		case "-l", "--list":
			list = true
		case "-h", "--help":
			fmt.Fprintln(os.Stdout, "usage: uniland fmt [-w] [-l] FILE")
			fmt.Fprintln(os.Stdout, "  без флагов — вывод в stdout; -w — записать; -l — только проверить")
			return 0
		default:
			if strings.HasPrefix(a, "-") {
				return usageErr(fmt.Sprintf("unrecognized arguments: %s", a))
			}
			if file != "" {
				return usageErr(fmt.Sprintf("unrecognized arguments: %s", a))
			}
			file = a
		}
	}
	if file == "" {
		return usageErr("the following arguments are required: file")
	}
	src, code := readSource(file)
	if code != 0 {
		return code
	}
	formatted, err := unifmt.Format(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "UniLand error: %s\n", err)
		return 1
	}
	changed := formatted != core.NormalizeNewlines(src)
	switch {
	case list:
		if changed {
			fmt.Println(file)
			return 1
		}
		return 0
	case write:
		if !changed {
			return 0
		}
		if err := os.WriteFile(file, []byte(formatted), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "UniLand error: %s\n", err)
			return 1
		}
		return 0
	default:
		fmt.Print(formatted)
		return 0
	}
}

func usageErr(msg string) int {
	fmt.Fprintln(os.Stderr, usageLine)
	fmt.Fprintf(os.Stderr, "uniland: error: %s\n", msg)
	return 2
}

func printHelp(w *os.File) {
	fmt.Fprintln(w, usageLine)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "UniLand — лёгкий дружелюбный язык сценариев")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "positional arguments:")
	fmt.Fprintln(w, "  {run,eval,repl,check}")
	fmt.Fprintln(w, "    run                запустить .uni скрипт")
	fmt.Fprintln(w, "    eval               выполнить код одной строкой")
	fmt.Fprintln(w, "    repl               интерактивный режим (REPL)")
	fmt.Fprintln(w, "    check              проверить синтаксис без запуска")
	fmt.Fprintln(w, "options:")
	fmt.Fprintln(w, "  -h, --help           show this help message and exit")
	fmt.Fprintln(w, "  -v, --version        show program's version number and exit")
	fmt.Fprintln(w, "  -e CODE, --eval CODE")
	fmt.Fprintln(w, "                       выполнить код одной строкой и выйти")
	fmt.Fprintln(w, "  -i, --interactive    запустить интерактивный режим (REPL)")
	fmt.Fprintln(w, "  -c FILE, --check FILE")
	fmt.Fprintln(w, "                       проверить синтаксис файла без запуска")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Примеры:  uniland run script.uni   |   uniland -e 'print(2+2)'   |   uniland -i")
	fmt.Fprintln(w, "Форматтер (расширение uniland-go):  uniland fmt [-w] [-l] script.uni")
}
