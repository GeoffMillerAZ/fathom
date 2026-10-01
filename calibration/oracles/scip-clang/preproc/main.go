// Command preproc is the C and C++ build-flags oracle (LB-F80). scip-clang
// indexes one build configuration: the translation units a compilation
// database lists, preprocessed with their own flags. Two things follow that no
// lexical extractor knows and the index does not say, so this tool reads them
// off the compiler's own preprocessor, run the way the build runs it:
//
//   - which source lines each file keeps (`active_lines`): a definition written
//     in a branch that configuration discards (libuv's src/win/ on a Unix
//     build, a `#ifdef __linux__` typedef on macOS) has no occurrence in the
//     index and is no miss of the dialect's;
//
//   - which file each `#include` written in a file of the subject entered
//     (`imports`): scip-clang records no reference on an include line.
//
//     go run . -subject <checkout> -compdb <compile_commands.json> [-clang <clang>] > preproc.json
//
// Each compilation database entry is preprocessed with its own flags (`-o`,
// `-c` and dependency flags dropped, `-E -dD -w` added) from its directory.
// The output keeps line numbers: a line marker (`# N "file"`) resets the
// source line of the next output line, and each output line that is not blank
// is a line the preprocessor kept (an active `#define` is printed by -dD, an
// inactive one is not). The marker that resumes an includer (flag 2) is the
// line after its include directive, so the include is the line before it. A
// file compiled in several translation units, or included by several, keeps
// the union. Only files inside the subject are written, in a neutral oracle
// file at tier A (the build's own flags); `indexbench lang-oracle` reads it
// from -tool-oracle beside the SCIP index. An include the preprocessor skips
// (a guarded re-include, an `#ifdef`-skipped one) writes no marker and is no
// import here, as for ../../clang-includes.
//
// The compiler is the one that preprocesses (Apple clang here), not the one
// scip-clang embeds: builtin macros agree on the platform they share.
// Bench-only; never a product dependency.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type importEntry struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	ToPath string `json:"to_path"`
}

type oracleFile struct {
	Tool        string              `json:"tool"`
	Version     string              `json:"version"`
	Tier        string              `json:"tier"`
	Imports     []importEntry       `json:"imports"`
	ActiveLines map[string][][2]int `json:"active_lines"`
}

type compileCommand struct {
	Directory string   `json:"directory"`
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
	File      string   `json:"file"`
}

func main() {
	subject := flag.String("subject", "", "checkout the compilation database was written for (required)")
	compdb := flag.String("compdb", "", "compile_commands.json (required)")
	clang := flag.String("clang", "", "preprocessor (default: each entry's own compiler)")
	flag.Parse()
	if *subject == "" || *compdb == "" {
		fmt.Fprintln(os.Stderr, "usage: active-lines -subject <dir> -compdb <compile_commands.json> [-clang <clang>]")
		os.Exit(2)
	}
	root, err := filepath.EvalSymlinks(*subject)
	if err == nil {
		root, err = filepath.Abs(root)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, err := os.ReadFile(*compdb)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var cmds []compileCommand
	if err := json.Unmarshal(raw, &cmds); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	active := map[string]map[int]bool{}
	imports := map[importEntry]bool{}
	seen := map[string]bool{}
	var stopped, units int
	version := ""
	for _, c := range cmds {
		args := c.Arguments
		if len(args) == 0 {
			args = splitCommand(c.Command)
		}
		args = preprocessArgs(args)
		if len(args) == 0 {
			continue
		}
		if *clang != "" {
			args[0] = *clang
		}
		key := c.Directory + "\x00" + strings.Join(args, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		if version == "" {
			version = compilerVersion(args[0])
		}
		units++
		ok, err := preprocess(args, c.Directory, root, active, imports)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !ok {
			stopped++
		}
	}
	out := oracleFile{Tool: "clang-preprocessor", Version: version, Tier: "A", Imports: []importEntry{}, ActiveLines: map[string][][2]int{}}
	for e := range imports {
		out.Imports = append(out.Imports, e)
	}
	sort.Slice(out.Imports, func(i, j int) bool {
		a, b := out.Imports[i], out.Imports[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.ToPath < b.ToPath
	})
	for path, lines := range active {
		out.ActiveLines[path] = ranges(lines)
	}
	fmt.Fprintf(os.Stderr, "translation_units %d stopped %d files %d includes_in_subject %d\n", units, stopped, len(out.ActiveLines), len(out.Imports))
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// splitCommand splits a compilation database `command` into arguments:
// whitespace-separated, with double and single quotes and backslash escapes.
func splitCommand(s string) []string {
	var args []string
	var cur strings.Builder
	inArg := false
	var quote rune
	esc := false
	for _, r := range s {
		switch {
		case esc:
			cur.WriteRune(r)
			esc = false
		case r == '\\' && quote != '\'':
			esc, inArg = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inArg = r, true
		case r == ' ' || r == '\t' || r == '\n':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args
}

// preprocessArgs turns a compile command into a preprocess-only one: the
// output and dependency-file flags go, `-E -dD -w` come in.
func preprocessArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	out := []string{args[0]}
	for i := 1; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-o", a == "-MF", a == "-MT", a == "-MQ":
			i++
		case a == "-c", a == "-MD", a == "-MMD", a == "-MP", a == "-M", a == "-MM":
		case strings.HasPrefix(a, "-o") && len(a) > 2 && !strings.HasPrefix(a, "-objc"):
		default:
			out = append(out, a)
		}
	}
	return append(out, "-E", "-dD", "-w")
}

func compilerVersion(bin string) string {
	b, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return ""
	}
	first, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(first)
}

// preprocess runs one translation unit and adds the lines it keeps to active
// and the includes it enters between subject files to imports; ok is false when the preprocessor stopped (a fatal error) or timed out.
func preprocess(args []string, dir, root string, active map[string]map[int]bool, imports map[importEntry]bool) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	if err := cmd.Start(); err != nil {
		return false, err
	}
	perr := keptLines(stdout, dir, root, active, imports)
	werr := cmd.Wait()
	if perr != nil {
		return false, perr
	}
	return werr == nil, nil
}

var markerRE = regexp.MustCompile(`^# (\d+) "((?:[^"\\]|\\.)*)"((?: \d+)*)$`)

// keptLines reads clang -E -dD output into active and imports. A line marker
// resets the source line of the next output line; each non-blank output line
// is a kept source line of the file on top of the include stack. A marker that
// enters a file (flag 1) pushes it; one that resumes an includer (flag 2) pops
// the file just left, an include written at the line before it.
func keptLines(r io.Reader, dir, root string, active map[string]map[int]bool, imports map[importEntry]bool) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	type frame struct{ abs, rel string }
	var stack []frame
	line := 0
	cur := func() string {
		if len(stack) == 0 {
			return ""
		}
		return stack[len(stack)-1].rel
	}
	for sc.Scan() {
		text := sc.Text()
		if m := markerRE.FindStringSubmatch(text); m != nil {
			n, _ := strconv.Atoi(m[1])
			abs := absPath(dir, unescape(m[2]))
			enter, leave := false, false
			for _, f := range strings.Fields(m[3]) {
				enter = enter || f == "1"
				leave = leave || f == "2"
			}
			fr := frame{abs, relIn(root, abs)}
			if fr.rel != "" && active[fr.rel] == nil {
				// a file the preprocessor entered whose every line is skipped
				// (a whole-file #ifdef) is listed with no range: inactive, not unknown
				active[fr.rel] = map[int]bool{}
			}
			switch {
			case len(stack) == 0 || enter:
				stack = append(stack, fr)
			case leave:
				i := len(stack) - 1
				for i >= 0 && stack[i].abs != abs {
					i--
				}
				if i >= 0 {
					if i+1 < len(stack) && fr.rel != "" && stack[i+1].rel != "" {
						imports[importEntry{Path: fr.rel, Line: n - 1, ToPath: stack[i+1].rel}] = true
					}
					stack = stack[:i+1]
				} else {
					stack = append(stack, fr)
				}
			default:
				stack[len(stack)-1] = fr
			}
			line = n
			continue
		}
		if f := cur(); strings.TrimSpace(text) != "" && f != "" {
			set := active[f]
			if set == nil {
				set = map[int]bool{}
				active[f] = set
			}
			set[line] = true
		}
		line++
	}
	return sc.Err()
}

// absPath is p as an absolute, cleaned path ("<built-in>" and the like kept).
func absPath(dir, p string) string {
	if p == "" || strings.HasPrefix(p, "<") {
		return p
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	return filepath.Clean(p)
}

// relIn is the absolute path p relative to root, or "" when p lies outside it
// (a system header, a built-in) or is no file.
func relIn(root, p string) string {
	if p == "" || strings.HasPrefix(p, "<") {
		return ""
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel)
}

func ranges(lines map[int]bool) [][2]int {
	out := [][2]int{}
	ns := make([]int, 0, len(lines))
	for n := range lines {
		ns = append(ns, n)
	}
	sort.Ints(ns)
	for _, n := range ns {
		if k := len(out); k > 0 && out[k-1][1] == n-1 {
			out[k-1][1] = n
			continue
		}
		out = append(out, [2]int{n, n})
	}
	return out
}

func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
