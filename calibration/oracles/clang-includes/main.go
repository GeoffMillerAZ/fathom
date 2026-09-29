// Command clang-includes is the language-breadth harness's import oracle for C
// and C++ (LB-F40, design §1.2 import_resolution). It writes indexbench
// lang-oracle's neutral oracle file with `imports` (-tool-oracle, design §10):
//
//	go run . -subject <checkout> [-clang <clang>] [-I <subject-relative dir>]... [-files <list>] [-lang c|c++] > imports.json
//
// The tool is the compiler's own preprocessor: `clang -E` on each file, from
// the subject root, with the include roots this run is given. Its line
// markers say which file each `#include` entered (flag 1) and the line the
// includer resumes at (flag 2), so an include written in the measured file
// (depth one) is the line before the return marker, and its target is the file
// the compiler opened. A conditional include the preprocessor skipped is not
// an include; a computed one (`#include FOO_H`) is, resolved as the compiler
// resolves it. Only targets inside the subject are written; the rest (system
// headers) are counted on stderr, as are files the preprocessor stopped on (a
// missing header is fatal: that file's later includes are not seen).
//
// Tier B, not A: the include roots are the operator's (-I), not the build's
// own flags (a compile_commands.json run would be the build's evaluation; it
// is not implemented here). Bench-only; never a product dependency.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type entry struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	ToPath string `json:"to_path"`
}

type oracleFile struct {
	Tool    string  `json:"tool"`
	Version string  `json:"version"`
	Tier    string  `json:"tier"`
	Imports []entry `json:"imports"`
}

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

var sourceExts = map[string]bool{".c": true, ".cc": true, ".cpp": true, ".cxx": true, ".c++": true,
	".h": true, ".hh": true, ".hpp": true, ".hxx": true, ".h++": true, ".ipp": true, ".tpp": true, ".inl": true}

func main() {
	subject := flag.String("subject", "", "checkout to measure (required)")
	clang := flag.String("clang", "clang", "clang binary")
	lang := flag.String("lang", "", "c or c++ for every file (default: .c is c, everything else c++)")
	filesList := flag.String("files", "", "file of subject-relative paths to preprocess, one per line (default: every C/C++ file under the subject)")
	var incs multi
	flag.Var(&incs, "I", "include root, subject-relative (repeatable)")
	flag.Parse()
	if *subject == "" {
		fmt.Fprintln(os.Stderr, "usage: clang-includes -subject <dir> [-clang <clang>] [-I <dir>]... [-files <list>] [-lang c|c++]")
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
	files, err := listFiles(root, *filesList)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	version := clangVersion(*clang)
	var out []entry
	stats := map[string]int{}
	for _, rel := range files {
		x := *lang
		if x == "" {
			x = "c++"
			if strings.EqualFold(filepath.Ext(rel), ".c") {
				x = "c"
			}
		}
		args := []string{"-E", "-w", "-x", x}
		for _, i := range incs {
			args = append(args, "-I", filepath.Join(root, i))
		}
		args = append(args, rel)
		es, ok, err := preprocess(*clang, root, rel, args)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !ok {
			stats["files_stopped"]++
		}
		for _, e := range es {
			if e.ToPath == "" {
				stats["includes_outside"]++
				continue
			}
			stats["includes_in_subject"]++
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.ToPath < b.ToPath
	})
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	summary := []string{fmt.Sprintf("files %d", len(files))}
	for _, k := range keys {
		summary = append(summary, fmt.Sprintf("%s %d", k, stats[k]))
	}
	fmt.Fprintln(os.Stderr, strings.Join(summary, " "))
	if out == nil {
		out = []entry{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(oracleFile{Tool: "clang-includes", Version: version, Tier: "B", Imports: out}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func listFiles(root, list string) ([]string, error) {
	if list != "" {
		b, err := os.ReadFile(list)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, l := range strings.Split(string(b), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				files = append(files, filepath.ToSlash(l))
			}
		}
		sort.Strings(files)
		return files, nil
	}
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (strings.HasPrefix(name, ".") || name == "vendor" || name == "third_party" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if sourceExts[strings.ToLower(filepath.Ext(name))] {
			rel, _ := filepath.Rel(root, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func clangVersion(clang string) string {
	b, err := exec.Command(clang, "--version").Output()
	if err != nil {
		return ""
	}
	first, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(first)
}

// preprocess runs clang -E on rel and returns its depth-one includes; ok is
// false when the preprocessor stopped (a fatal error) or timed out.
func preprocess(clang, root, rel string, args []string) ([]entry, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, clang, args...)
	cmd.Dir = root
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false, err
	}
	if err := cmd.Start(); err != nil {
		return nil, false, err
	}
	es, perr := parseMarkers(stdout, root, rel)
	werr := cmd.Wait()
	if perr != nil {
		return nil, false, perr
	}
	return es, werr == nil, nil
}

var markerRE = regexp.MustCompile(`^# (\d+) "((?:[^"\\]|\\.)*)"((?: \d+)*)$`)

// parseMarkers reads clang -E output and returns each include written in
// main (the measured file, subject-relative): the line of the directive (the
// line before the marker that resumes main) and the included file, relative to
// root, or "" when it lies outside root.
func parseMarkers(r io.Reader, root, main string) ([]entry, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	stack := []string{}
	var out []entry
	norm := func(p string) string {
		if p == "" || strings.HasPrefix(p, "<") {
			return p
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		return filepath.Clean(p)
	}
	mainAbs := norm(main)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "# ") {
			continue
		}
		m := markerRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		file := norm(unescape(m[2]))
		flags := strings.Fields(m[3])
		enter, leave := false, false
		for _, f := range flags {
			enter = enter || f == "1"
			leave = leave || f == "2"
		}
		switch {
		case len(stack) == 0:
			stack = append(stack, file) // the first marker names the measured file
		case enter:
			stack = append(stack, file)
		case leave:
			i := len(stack) - 1
			for i >= 0 && stack[i] != file {
				i--
			}
			if i < 0 || i+1 >= len(stack) {
				continue
			}
			child := stack[i+1]
			stack = stack[:i+1]
			if i == 0 && file == mainAbs && !strings.HasPrefix(child, "<") {
				to := ""
				if rel, err := filepath.Rel(root, child); err == nil && !strings.HasPrefix(rel, "..") {
					if st, err := os.Stat(child); err == nil && !st.IsDir() {
						to = filepath.ToSlash(rel)
					}
				}
				out = append(out, entry{Path: main, Line: n - 1, ToPath: to})
			}
		}
	}
	return out, sc.Err()
}

// unescape undoes the line marker's backslash escapes.
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
