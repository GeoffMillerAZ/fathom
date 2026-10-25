// Command clang-calls is the C and C++ compiler's own call-site oracle (tier A,
// LB-F116). scip-clang indexes a template's pattern, where a call whose callee
// depends on a template parameter (`buf.size()` on a `Buffer&` of type
// parameter, `swap(a, b)` found by argument-dependent lookup) has no callee:
// the compiler resolves it only per instantiation, and the index references
// nothing there. A lexical extractor sees `size(` and records a call. Which of
// the two is right is a fact of the compiler, not of either tool, so this
// command asks the compiler: it runs every compilation database entry with its
// own flags as
//
//	<compiler> <flags> -fsyntax-only -Xclang -ast-dump=json
//
// streams the JSON AST (one translation unit's dump runs to gigabytes; it is
// never held in memory), and writes each call-shaped site written in a file of
// the subject — a name followed by `(`, or a type followed by `(` or `{` — with
// the class the AST gives it:
//
//   - dependent: CXXDependentScopeMemberExpr, UnresolvedMemberExpr,
//     UnresolvedLookupExpr, DependentScopeDeclRefExpr or
//     CXXUnresolvedConstructExpr — no callee exists before instantiation;
//   - resolved: a DeclRefExpr naming a function, or a MemberExpr naming a
//     method (its type is a bound member function);
//   - construct: CXXTemporaryObjectExpr, CXXConstructExpr, or a
//     CXXFunctionalCastExpr through a constructor, written `T(` or `T{`;
//   - object: a call through a variable, parameter or field (a lambda, a
//     function pointer, a callable object);
//   - cast: a CXXFunctionalCastExpr that converts without a constructor
//     (`size_t(n)`, `char(c)`): no call at all;
//   - declaration: a function's name where it is declared (`int close(int
//     fd);` is written like a call, and an index references it there).
//
// The dump holds no expression inside a type (an array bound, a decltype in a
// declarator): a call written there is in no class, never "no call".
//
// A site in a template pattern is dumped again inside each instantiation,
// resolved there; the pattern's class wins (dependent before any other),
// because the pattern is what the index and a lexical extractor read.
//
//	go run . -subject <checkout> -compdb <compile_commands.json> [-clang <clang>] [-jobs N] > clang-calls.json
//
// `indexbench lang-oracle` reads the file from -tool-oracle beside the SCIP
// index (internal/langbench SCIPOptions.ClangSites). Bench-only; never a
// product dependency.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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

// site is one call-shaped site in a subject file: the callee's name token
// (1-based line and byte column) and its class.
type site struct {
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Col   int    `json:"col"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

type oracleFile struct {
	Tool      string `json:"tool"`
	Version   string `json:"version"`
	Tier      string `json:"tier"`
	CallSites []site `json:"call_sites"`
}

type compileCommand struct {
	Directory string   `json:"directory"`
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
	File      string   `json:"file"`
}

// retries is how many times a unit that stopped is run before it is reported.
const retries = 8

// classRank orders the classes one site can be given across translation units
// and instantiations: the lowest wins.
var classRank = map[string]int{"dependent": 0, "resolved": 1, "construct": 2, "object": 3, "cast": 4, "declaration": 5}

func main() {
	subject := flag.String("subject", "", "checkout the compilation database was written for (required)")
	compdb := flag.String("compdb", "", "compile_commands.json (required)")
	clang := flag.String("clang", "", "compiler (default: each entry's own)")
	jobs := flag.Int("jobs", 4, "translation units dumped at once")
	tierArg := flag.String("tier", "A", "the output's oracle tier: A for the build's own compiler, B for a translated database")
	flag.Parse()
	tier, terr := tierFlag(*tierArg)
	if *subject == "" || *compdb == "" || terr != nil {
		fmt.Fprintln(os.Stderr, "usage: clang-calls -subject <dir> -compdb <compile_commands.json> [-clang <clang>] [-jobs N] [-tier A|B]")
		os.Exit(2)
	}
	out, err := run(*subject, *compdb, *clang, *jobs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out.Tier = tier
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(subject, compdb, clang string, jobs int) (oracleFile, error) {
	root, err := filepath.EvalSymlinks(subject)
	if err == nil {
		root, err = filepath.Abs(root)
	}
	if err != nil {
		return oracleFile{}, err
	}
	raw, err := os.ReadFile(compdb)
	if err != nil {
		return oracleFile{}, err
	}
	var cmds []compileCommand
	if err := json.Unmarshal(raw, &cmds); err != nil {
		return oracleFile{}, err
	}
	type unit struct {
		args []string
		dir  string
	}
	var units []unit
	seen := map[string]bool{}
	version := ""
	for _, c := range cmds {
		args := c.Arguments
		if len(args) == 0 {
			args = splitCommand(c.Command)
		}
		args = dumpArgs(args)
		if len(args) == 0 {
			continue
		}
		if clang != "" {
			args[0] = clang
		}
		key := c.Directory + "\x00" + strings.Join(args, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		if version == "" {
			version = compilerVersion(args[0])
		}
		units = append(units, unit{args, c.Directory})
	}
	files := newFileCache(root)
	sites := map[siteKey]string{}
	var mu sync.Mutex
	add := func(s site) {
		mu.Lock()
		defer mu.Unlock()
		k := siteKey{s.Path, s.Line, s.Col, s.Name}
		if prev, ok := sites[k]; !ok || classRank[s.Class] < classRank[prev] {
			sites[k] = s.Class
		}
	}
	if jobs < 1 {
		jobs = 1
	}
	work := make(chan unit)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var stopped int
	var firstErr error
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range work {
				// A unit is committed only when the compiler finished it: a
				// unit that stopped (a header it could not open) parses what
				// remains with errors, and a call to an undeclared name reads
				// as unresolved. A stopped unit is tried again (an unreadable
				// file can be transient), then reported.
				var got []site
				var ok bool
				var err error
				for attempt := 0; attempt < retries && err == nil && !ok; attempt++ {
					if attempt > 0 {
						time.Sleep(2 * time.Second)
					}
					got = got[:0]
					ok, err = dumpUnit(u.args, u.dir, files, func(s site) { got = append(got, s) })
				}
				if ok {
					for _, s := range got {
						add(s)
					}
				}
				errMu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				if !ok {
					stopped++
				}
				errMu.Unlock()
			}
		}()
	}
	for _, u := range units {
		work <- u
	}
	close(work)
	wg.Wait()
	if firstErr == nil {
		firstErr = files.err
	}
	if firstErr == nil && stopped > 0 {
		// A unit the compiler could not finish (an unreadable file, a
		// timeout) leaves its files' sites unknown: no partial oracle.
		firstErr = fmt.Errorf("clang-calls: %d of %d translation units stopped", stopped, len(units))
	}
	if firstErr != nil {
		return oracleFile{}, firstErr
	}
	out := oracleFile{Tool: "clang-ast", Version: version, Tier: "A", CallSites: make([]site, 0, len(sites))}
	for k, c := range sites {
		out.CallSites = append(out.CallSites, site{Path: k.path, Line: k.line, Col: k.col, Name: k.name, Class: c})
	}
	sort.Slice(out.CallSites, func(i, j int) bool {
		a, b := out.CallSites[i], out.CallSites[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Name < b.Name
	})
	fmt.Fprintf(os.Stderr, "translation_units %d stopped %d sites %d\n", len(units), stopped, len(out.CallSites))
	return out, nil
}

type siteKey struct {
	path      string
	line, col int
	name      string
}

// dumpArgs turns a compile command into a syntax-only JSON AST dump: the
// output, dependency-file and optimisation-irrelevant flags go.
func dumpArgs(args []string) []string {
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
	return append(out, "-fsyntax-only", "-w", "-Xclang", "-ast-dump=json")
}

func compilerVersion(bin string) string {
	b, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return ""
	}
	first, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(first)
}

// dumpUnit runs one translation unit's dump and feeds its call-shaped sites in
// subject files to add; ok is false when the compiler failed or timed out
// (its sites up to the failure are kept).
func dumpUnit(args []string, dir string, files *fileCache, add func(site)) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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
	p := &astParser{r: bufio.NewReaderSize(stdout, 1<<20), dir: dir, files: files, add: add}
	perr := p.top()
	_, _ = io.Copy(io.Discard, stdout)
	werr := cmd.Wait()
	if perr != nil && !errors.Is(perr, io.EOF) {
		return false, perr
	}
	if werr != nil {
		fmt.Fprintf(os.Stderr, "clang-calls: %s: %v\n", args[len(args)-5], werr)
	}
	return werr == nil, nil
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

// tierFlag is the -tier value written to the output's "tier": A when the
// database is the build's own compiler's, B when it is a translation (a
// clang CUDA database for an nvcc build, LB-Q-cuda); anything else is refused.
func tierFlag(s string) (string, error) {
	switch s {
	case "A", "B", "C", "D":
		return s, nil
	}
	return "", fmt.Errorf("-tier %q: want A, B, C or D", s)
}
