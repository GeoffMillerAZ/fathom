package testrun

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Tier 1 (design section 3): which changed symbols each failing test
// reaches, and where localize says to look. The working-tree diff (`git
// diff HEAD`) stands in for "changed since the last attested pass": this
// machine has no certificate history to diff against.

// LocalizeHops caps localize's walk (design section 3).
const LocalizeHops = 4

// MaxChangedQueried caps how many changed symbols test_impact is asked
// about in one run; the rest are counted in Candidates.Skipped.
const MaxChangedQueried = 12

// maxLocalized caps the localize candidates kept.
const maxLocalized = 3

// ChangedSymbol is one Go function or method the working tree changed.
type ChangedSymbol struct {
	// Ref is the symbol's index key without its "<service>::" prefix: the
	// package directory and the package name for a function
	// ("money/money.Round"), the directory and the receiver twice for a
	// method ("money/Ledger.Ledger.Add") — the key shapes the call graph
	// stores. The Index implementation adds the service.
	Ref string `json:"ref"`
	// Name is the display name: "money.Round", "Ledger.Add".
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// LocalizeSeed is localize's one input: a stack trace or a failing test.
type LocalizeSeed struct {
	Kind   string // "stack_trace" or "failing_test"
	Value  string
	Budget int
}

// Located is one localize candidate. Changed marks one the working tree
// changed.
type Located struct {
	Symbol  string `json:"symbol"`
	File    string `json:"file"`
	Changed bool   `json:"changed,omitempty"`
}

// Index is the port to the daemon's answers tier 1 needs; the hook
// implements it over the daemon's /rpc, a test with fixed answers.
type Index interface {
	// TestImpact returns the index keys of every test reaching ref.
	TestImpact(ref string) ([]string, error)
	// Localize runs localize from one seed.
	Localize(seed LocalizeSeed) ([]Located, error)
}

// TestReach is one failing test and the changed symbols it reaches.
type TestReach struct {
	Test    string          `json:"test"`
	Changed []ChangedSymbol `json:"changed,omitempty"`
}

// Candidates is tier 1's answer.
type Candidates struct {
	// ReachUnknown means no complete index answer establishes an empty
	// reach: the index was absent, no symbol was queried, or a query missed.
	ReachUnknown bool `json:"reach_unknown,omitempty"`
	// Reach has one row per leaf failure, in failure order.
	Reach []TestReach `json:"reach,omitempty"`
	// Localized is localize's top candidates, changed ones first.
	Localized []Located `json:"localized,omitempty"`
	// Skipped counts changed symbols past MaxChangedQueried.
	Skipped int `json:"skipped,omitempty"`
}

// FindCandidates is tier 1's entry point (see testRunCandidates).
func FindCandidates(run Run, changed []ChangedSymbol, idx Index) Candidates {
	return testRunCandidates(run, changed, idx)
}

// testRunCandidates (TR-5) asks test_impact about each changed symbol (at
// most MaxChangedQueried, in the order given) and intersects the tests it
// returns with the run's leaf failures, then runs localize once — from the
// first panic's stack trace, else from the first failing test — with
// LocalizeHops hops. A passing run, or one whose only evidence is a build
// error, asks nothing. An index error on one symbol skips that symbol.
func testRunCandidates(run Run, changed []ChangedSymbol, idx Index) Candidates {
	out := Candidates{ReachUnknown: true}
	leaves := run.LeafFailures()
	if (len(leaves) == 0 && len(run.Panics) == 0) || idx == nil {
		return out
	}
	out.Reach = make([]TestReach, len(leaves))
	for i, f := range leaves {
		out.Reach[i].Test = f.ID()
	}
	queried := changed
	if len(queried) > MaxChangedQueried {
		out.Skipped = len(queried) - MaxChangedQueried
		queried = queried[:MaxChangedQueried]
	}
	out.ReachUnknown = len(queried) == 0 || out.Skipped > 0
	for _, c := range queried {
		tests, err := idx.TestImpact(c.Ref)
		if err != nil {
			out.ReachUnknown = true
			continue
		}
		for i, f := range leaves {
			for _, key := range tests {
				if testKeyMatches(key, f) {
					out.Reach[i].Changed = append(out.Reach[i].Changed, c)
					break
				}
			}
		}
	}
	for i := range out.Reach {
		sortChanged(out.Reach[i].Changed)
	}

	seed := LocalizeSeed{Budget: LocalizeHops}
	if p, ok := firstPanicWithFrames(run.Panics); ok {
		seed.Kind, seed.Value = "stack_trace", stackTraceText(p)
	} else if len(leaves) > 0 {
		seed.Kind, seed.Value = "failing_test", leaves[0].Test
	}
	if seed.Kind == "" {
		return out
	}
	located, err := idx.Localize(seed)
	if err != nil {
		return out
	}
	for i := range located {
		for _, c := range changed {
			if keyNames(located[i].Symbol, c.Ref) {
				located[i].Changed = true
				break
			}
		}
	}
	sort.SliceStable(located, func(i, j int) bool { return located[i].Changed && !located[j].Changed })
	if len(located) > maxLocalized {
		located = located[:maxLocalized]
	}
	out.Localized = located
	return out
}

func sortChanged(cs []ChangedSymbol) {
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].File != cs[j].File {
			return cs[i].File < cs[j].File
		}
		return cs[i].Line < cs[j].Line
	})
}

func firstPanicWithFrames(ps []Panic) (Panic, bool) {
	for _, p := range ps {
		if len(p.Frames) > 0 {
			return p, true
		}
	}
	return Panic{}, false
}

// stackTraceText renders a panic as the trace localize parses: the
// message, then one "file:line" frame per line.
func stackTraceText(p Panic) string {
	var b strings.Builder
	b.WriteString("panic: " + p.Message + "\n")
	for _, f := range p.Frames {
		fmt.Fprintf(&b, "\t%s:%d\n", f.File, f.Line)
	}
	return b.String()
}

// keyNames reports whether index key names the symbol ref: the key ends
// with ref at a component boundary ("svc::dir/pkg.Name" names
// "dir/pkg.Name").
func keyNames(key, ref string) bool {
	return key == ref || strings.HasSuffix(key, "::"+ref) || strings.HasSuffix(key, "/"+ref)
}

// testKeyMatches reports whether a test's index key ("svc::dir/pkg.TestX")
// is failure f: the same top-level test name, and — when both sides name a
// package — the key's directory is the tail of f's import path.
func testKeyMatches(key string, f Failure) bool {
	name := strings.SplitN(f.Test, "/", 2)[0]
	dot := strings.LastIndex(key, ".")
	if dot < 0 || key[dot+1:] != name {
		return false
	}
	head := key[:dot]
	if i := strings.Index(head, "::"); i >= 0 {
		head = head[i+2:]
	}
	slash := strings.LastIndex(head, "/")
	if slash < 0 || f.Package == "" {
		return true // a root-package key, or a failure with no package: the name decides
	}
	dir := head[:slash]
	return f.Package == dir || strings.HasSuffix(f.Package, "/"+dir)
}

// ─── the working-tree diff ──────────────────────────────────────────────

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// maxGoFileBytes caps each changed file read.
const maxGoFileBytes = 2 << 20

// ChangedGoSymbols maps a unified diff (`git diff HEAD --unified=0`) to the
// Go functions and methods whose lines it touches, reading each changed
// non-test .go file under root. Deleted files, test files and files that
// no longer parse name nothing. Sorted by file, then line.
func ChangedGoSymbols(root, diff string) []ChangedSymbol {
	type span struct{ from, to int }
	touched := map[string][]span{}
	var files []string
	cur := ""
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			cur = ""
			p := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			if p == "/dev/null" {
				continue
			}
			p = strings.TrimPrefix(p, "b/")
			if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				cur = p
				if _, ok := touched[p]; !ok {
					files = append(files, p)
				}
			}
		case cur != "" && strings.HasPrefix(line, "@@"):
			m := hunkRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			start, _ := strconv.Atoi(m[1])
			n := 1
			if m[2] != "" {
				n, _ = strconv.Atoi(m[2])
			}
			end := start + n - 1
			if n == 0 {
				end = start // a pure deletion after line start
			}
			touched[cur] = append(touched[cur], span{start, end})
		}
	}
	sort.Strings(files)
	var out []ChangedSymbol
	for _, rel := range files {
		src, ok := readCapped(filepath.Join(root, filepath.FromSlash(rel)))
		if !ok {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		dir := path.Dir(rel)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			from, to := fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line
			hit := false
			for _, s := range touched[rel] {
				if s.from <= to && s.to >= from {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			out = append(out, changedSymbolFor(dir, f.Name.Name, fn, rel, from))
		}
	}
	return out
}

func changedSymbolFor(dir, pkg string, fn *ast.FuncDecl, file string, line int) ChangedSymbol {
	prefix := ""
	if dir != "." && dir != "" {
		prefix = dir + "/"
	}
	if recv := receiverType(fn); recv != "" {
		return ChangedSymbol{Ref: prefix + recv + "." + recv + "." + fn.Name.Name, Name: recv + "." + fn.Name.Name, File: file, Line: line}
	}
	return ChangedSymbol{Ref: prefix + pkg + "." + fn.Name.Name, Name: pkg + "." + fn.Name.Name, File: file, Line: line}
}

// receiverType is a method's receiver type name without pointer or type
// parameters; "" for a function.
func receiverType(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	t := fn.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
		case *ast.IndexExpr:
			t = x.X
		case *ast.IndexListExpr:
			t = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}

func readCapped(p string) ([]byte, bool) {
	f, err := os.Open(p)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxGoFileBytes+1))
	if err != nil || len(b) > maxGoFileBytes {
		return nil, false
	}
	return b, true
}
