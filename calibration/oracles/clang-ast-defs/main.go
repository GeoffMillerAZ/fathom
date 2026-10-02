// clang-ast-defs — tier-A definition ranges for a C or C++ calibration subject
// (LB-Q-c-rest step 5, language-breadth §1.2 range_agreement).
//
// scip-clang writes no end line for a definition, so L1 against its index has
// no range to compare and range_agreement is not measured (L1 is capped at
// good). The compiler's own AST has the range: this tool runs
//
//	clang -fsyntax-only -Xclang -ast-dump=json
//
// once per translation unit of a compilation database, with the entry's own
// flags, and writes every definition that lies inside the subject with its
// first and last line: a function with a body, a struct or union with a body, an
// enum with constants, a typedef. A macro has no AST node, so none is written
// (the reading says so). It is its own Go module, never imported by fathom;
// the harness reads its output as a companion of the build-flags preprocessor's
// oracle file (cmd/indexbench/langoracle_imports.go, scoreRangeCompanion).
//
//	go run . -subject <subject> -compdb <scratch-build-dir>/compile_commands.json > ranges.json
//
// clang's JSON writes a location's file and line only where they differ from
// the previous location in print order, so the walk keeps that state across the
// whole tree, subtrees it does not want included.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type loc struct {
	File      *string `json:"file"`
	Line      *int    `json:"line"`
	Spelling  *loc    `json:"spellingLoc"`
	Expansion *loc    `json:"expansionLoc"`
}

type node struct {
	Kind               string `json:"kind"`
	Name               string `json:"name"`
	IsImplicit         bool   `json:"isImplicit"`
	TagUsed            string `json:"tagUsed"`
	CompleteDefinition bool   `json:"completeDefinition"`
	Loc                *loc   `json:"loc"`
	Range              *struct {
		Begin *loc `json:"begin"`
		End   *loc `json:"end"`
	} `json:"range"`
	Inner []node `json:"inner"`
}

// Def is one definition: its subject-relative file, name, canonical kind and
// first and last line.
type Def struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
	End  int    `json:"end"`
}

// cursor is the location state clang's JSON compression keeps.
type cursor struct {
	file string
	line int
}

func (c *cursor) take(l *loc) {
	if l == nil {
		return
	}
	c.take(l.Spelling)
	c.take(l.Expansion)
	if l.File != nil {
		c.file = *l.File
	}
	if l.Line != nil {
		c.line = *l.Line
	}
}

// Extract is the subject definitions in one translation unit's AST dump.
func Extract(r io.Reader, subject string) ([]Def, error) {
	var tu node
	if err := json.NewDecoder(r).Decode(&tu); err != nil {
		return nil, err
	}
	var out []Def
	cur := &cursor{}
	cur.take(tu.Loc)
	var walk func(n *node, top bool)
	walk = func(n *node, top bool) {
		cur.take(n.Loc)
		var bf, ef string
		var bl, el int
		if n.Range != nil {
			cur.take(n.Range.Begin)
			bf, bl = cur.file, cur.line
			cur.take(n.Range.End)
			ef, el = cur.file, cur.line
		}
		if top && !n.IsImplicit && n.Name != "" && bf == ef && bl > 0 && el >= bl {
			if kind := defKind(n); kind != "" {
				if rel, ok := within(subject, bf); ok {
					out = append(out, Def{Path: rel, Name: n.Name, Kind: kind, Line: bl, End: el})
				}
			}
		}
		for i := range n.Inner {
			walk(&n.Inner[i], false)
		}
	}
	for i := range tu.Inner {
		walk(&tu.Inner[i], true)
	}
	return out, nil
}

// defKind is the canonical kind of a definition node, "" for anything else.
func defKind(n *node) string {
	has := func(kind string) bool {
		for i := range n.Inner {
			if n.Inner[i].Kind == kind {
				return true
			}
		}
		return false
	}
	switch n.Kind {
	case "FunctionDecl":
		if has("CompoundStmt") {
			return "function"
		}
	case "RecordDecl":
		if n.CompleteDefinition && (n.TagUsed == "struct" || n.TagUsed == "union") {
			return n.TagUsed
		}
	case "EnumDecl":
		if has("EnumConstantDecl") {
			return "enum"
		}
	case "TypedefDecl":
		return "type_alias"
	}
	return ""
}

// within is file relative to subject, false when it lies outside it.
func within(subject, file string) (string, bool) {
	if !filepath.IsAbs(file) {
		return "", false
	}
	rel, err := filepath.Rel(subject, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || rel == "." {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

type entry struct {
	Directory string   `json:"directory"`
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
	File      string   `json:"file"`
}

// clangArgs is the entry's own flags with the compile and dependency outputs
// removed, as a syntax-only AST dump.
func clangArgs(e entry) []string {
	args := e.Arguments
	if len(args) == 0 {
		args = splitCommand(e.Command)
	}
	var out []string
	for i := 1; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-c" || a == "-MD" || a == "-MMD" || a == "-MP":
		case a == "-o" || a == "-MF" || a == "-MT" || a == "-MQ":
			i++
		case strings.HasPrefix(a, "-o") && len(a) > 2 && !strings.HasPrefix(a, "-objc"):
		default:
			out = append(out, a)
		}
	}
	return append(out, "-fsyntax-only", "-w", "-Xclang", "-ast-dump=json")
}

func splitCommand(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	started := false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else if c == '\\' && quote == '"' && i+1 < len(rs) {
				i++
				cur.WriteRune(rs[i])
			} else {
				cur.WriteRune(c)
			}
		case c == '"' || c == '\'':
			quote, started = c, true
		case c == '\\' && i+1 < len(rs):
			i++
			cur.WriteRune(rs[i])
			started = true
		case c == ' ' || c == '\t' || c == '\n':
			if started {
				out = append(out, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(c)
			started = true
		}
	}
	if started {
		out = append(out, cur.String())
	}
	return out
}

// Output is the oracle file.
type Output struct {
	Tool    string `json:"tool"`
	Tier    string `json:"tier"`
	Version string `json:"version"`
	// Units is the translation units dumped, Errored the ones clang could not
	// (a missing generated header), Skipped the entries of a file already
	// dumped (a file built twice, a static and a shared library).
	Units   int   `json:"units"`
	Errored int   `json:"errored"`
	Skipped int   `json:"skipped"`
	Ranges  []Def `json:"ranges"`
}

func main() {
	subject := flag.String("subject", "", "subject checkout (absolute)")
	compdb := flag.String("compdb", "", "compile_commands.json")
	clang := flag.String("clang", "clang", "clang to run")
	jobs := flag.Int("jobs", 4, "parallel dumps")
	flag.Parse()
	if *subject == "" || *compdb == "" {
		fmt.Fprintln(os.Stderr, "usage: clang-ast-defs -subject <dir> -compdb <compile_commands.json> [-clang clang] [-jobs 4]")
		os.Exit(2)
	}
	abs, err := filepath.Abs(*subject)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	raw, err := os.ReadFile(*compdb)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var entries []entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ver, _ := exec.Command(*clang, "--version").Output()
	out := Output{Tool: "clang-ast", Tier: "A", Version: strings.TrimSpace(strings.SplitN(string(ver), "\n", 2)[0])}

	seen := map[string]bool{}
	var todo []entry
	for _, e := range entries {
		f := e.File
		if !filepath.IsAbs(f) {
			f = filepath.Join(e.Directory, f)
		}
		if seen[f] {
			out.Skipped++
			continue
		}
		seen[f] = true
		if _, err := os.Stat(f); err != nil {
			out.Errored++
			continue
		}
		todo = append(todo, e)
	}
	var mu sync.Mutex
	defs := map[Def]bool{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, *jobs)
	for _, e := range todo {
		wg.Add(1)
		sem <- struct{}{}
		go func(e entry) {
			defer wg.Done()
			defer func() { <-sem }()
			ds, err := dumpRetry(*clang, e, abs)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fmt.Fprintf(os.Stderr, "clang-ast-defs: %s: %v\n", e.File, err)
				out.Errored++
				return
			}
			out.Units++
			for _, d := range ds {
				defs[d] = true
			}
		}(e)
	}
	wg.Wait()
	for d := range defs {
		out.Ranges = append(out.Ranges, d)
	}
	sort.Slice(out.Ranges, func(i, j int) bool {
		a, b := out.Ranges[i], out.Ranges[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Kind < b.Kind
	})
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// dump runs clang over one entry and extracts its definitions.
func dump(clang string, e entry, subject string) ([]Def, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, clang, clangArgs(e)...)
	cmd.Dir = e.Directory
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	ds, perr := Extract(stdout, subject)
	_, _ = io.Copy(io.Discard, stdout)
	werr := cmd.Wait()
	// clang exits non-zero on an error yet still dumps what it parsed; the
	// dump is kept when it decoded.
	if perr != nil {
		return nil, fmt.Errorf("%v (clang: %v %s)", perr, werr, firstLine(stderr.String()))
	}
	return ds, nil
}

// dumpRetry dumps one entry, trying again after a pause when the dump fails:
// a build tree on a network or external volume can refuse a read for a moment,
// and a unit counted as errored is a unit missing from the oracle.
func dumpRetry(clang string, e entry, subject string) ([]Def, error) {
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		var ds []Def
		if ds, err = dump(clang, e, subject); err == nil {
			return ds, nil
		}
		time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
	}
	return nil, err
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
