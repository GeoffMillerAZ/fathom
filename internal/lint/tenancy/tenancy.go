// Package tenancy is the tenancy lint (design §4.7, plan RT-91/RT-92/RT-137):
// syntactic rules — decidable from one file's AST and its imports, with no
// call-graph or data-flow analysis — that keep request-path cost from
// following repository size and keep tests away from the operator's daemon.
//
// The rules run as a Go test (TestTenancyRules) inside `go test ./...`, so
// `task validate` enforces them with no new gate step. A site may be excused
// with `//fathom:tenancy-allow <rule> <reason>` on the line (or the line
// directly above); every excuse is listed in .fathom/tenancy-allow.txt and the
// count may only fall (allowlistCeiling).
package tenancy

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Rule ids (design §4.7).
const (
	RuleWholeRead   = "whole-read"
	RuleSources     = "sources"
	RuleExecNoCtx   = "exec-noctx"
	RuleBgCtx       = "bg-ctx"
	RuleBareGo      = "bare-go"
	RulePkgMap      = "pkg-map"
	RuleRawServe    = "raw-serve"
	RuleTestLiveURL = "test-live-url"
	RuleTestEnv     = "test-env"
	// RuleAllowNoReason is reported for a tenancy-allow comment that names no
	// reason (or no known rule): an excuse must say why.
	RuleAllowNoReason = "allow-noreason"
)

// Rules lists every rule id, sorted.
func Rules() []string {
	r := []string{RuleWholeRead, RuleSources, RuleExecNoCtx, RuleBgCtx, RuleBareGo, RulePkgMap, RuleRawServe, RuleTestLiveURL, RuleTestEnv}
	sort.Strings(r)
	return r
}

const allowDirective = "//fathom:tenancy-allow"

// Finding is one rule hit. Allowed findings carry the excuse's reason.
type Finding struct {
	Rule    string
	Path    string // slash-separated, relative to the repository root
	Line    int
	Detail  string
	Allowed bool
	Reason  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: [%s] %s", f.Path, f.Line, f.Rule, f.Detail)
}

// exemptPackage reports whether rel (a slash path of a NON-test file) sits in
// a package the request-path rules do not govern: the ingest side, whose job
// is to read the tree (design §4.7).
func exemptPackage(rel string) bool {
	for _, p := range []string{"internal/fusion/", "internal/refresh/", "internal/corpus/"} {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return isIngestVerb(rel)
}

// isIngestVerb: the cmd/fathom ingest verbs (ingest.go, ingest_dryrun.go,
// platformingest.go, …) — every top-level cmd/fathom file whose name names
// ingest.
func isIngestVerb(rel string) bool {
	dir, base := path.Split(rel)
	return dir == "cmd/fathom/" && strings.Contains(base, "ingest")
}

// handlerFile: the code that answers requests inside the daemon — all of
// internal/mcp, and the cmd/fathom files that serve HTTP: console_*.go,
// servehttp*.go, and any other cmd/fathom file that names
// http.ResponseWriter or http.Handler (decided from the file's own AST and
// imports). The rest of cmd/fathom is one-shot CLI verbs and hooks — a
// process that exits after one answer, bounded by the hook deadline — which
// the request-path rule does not govern (design §4.7, amended by RT-91).
func handlerFile(rel string, servesHTTP bool) bool {
	dir, base := path.Split(rel)
	if dir == "internal/mcp/" {
		return true
	}
	if dir != "cmd/fathom/" || isIngestVerb(rel) {
		return false
	}
	return strings.HasPrefix(base, "console_") || strings.HasPrefix(base, "servehttp") || servesHTTP
}

// bgCtxFile: files whose code runs under a request context: internal/mcp,
// console_*.go and every servehttp*.go (servehttp.go and its servehttp_<phase>.go
// split; test files never reach checkSource).
func bgCtxFile(rel string) bool {
	dir, base := path.Split(rel)
	if dir == "internal/mcp/" {
		return true
	}
	return dir == "cmd/fathom/" && (strings.HasPrefix(base, "console_") || strings.HasPrefix(base, "servehttp"))
}

// bareGoAllowed: the packages that own goroutine lifetimes.
func bareGoAllowed(rel string) bool {
	switch path.Dir(rel) {
	case "internal/console", "internal/refresh", "internal/memo", "internal/tenancy":
		return true
	}
	return false
}

// graphstoreGroup: the one place a handler may build a call graph.
func graphstoreGroup(rel string) bool {
	return rel == "internal/memo/graphstore.go" || rel == "cmd/fathom/console_graphstore.go"
}

// CheckFile applies every rule to one file's source. rel is its
// slash-separated repository path; it decides which rules apply.
func CheckFile(rel string, src []byte) ([]Finding, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	imports := map[string]string{} // local name -> import path
	for _, im := range f.Imports {
		p, _ := strconv.Unquote(im.Path.Value)
		name := path.Base(p)
		if im.Name != nil {
			name = im.Name.Name
		}
		imports[name] = p
	}
	isTest := strings.HasSuffix(rel, "_test.go")
	c := &checker{fset: fset, rel: rel, imports: imports}
	if isTest {
		c.checkTest(f)
	} else if !exemptPackage(rel) {
		c.checkSource(f)
	}
	c.applyAllows(f)
	sort.SliceStable(c.out, func(i, j int) bool {
		if c.out[i].Line != c.out[j].Line {
			return c.out[i].Line < c.out[j].Line
		}
		return c.out[i].Rule < c.out[j].Rule
	})
	return c.out, nil
}

type checker struct {
	fset    *token.FileSet
	rel     string
	imports map[string]string
	out     []Finding
}

func (c *checker) add(rule string, pos token.Pos, detail string) {
	c.out = append(c.out, Finding{Rule: rule, Path: c.rel, Line: c.fset.Position(pos).Line, Detail: detail})
}

// pkgCall resolves `x.Sel` to (import path, Sel) when x names an import.
func (c *checker) pkgSel(e ast.Expr) (string, string, bool) {
	if e == nil {
		return "", "", false
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	p, ok := c.imports[id.Name]
	if !ok {
		return "", "", false
	}
	return p, sel.Sel.Name, true
}

var wholeReads = map[string]bool{
	"os.ReadFile": true, "io.ReadAll": true, "path/filepath.WalkDir": true,
	"path/filepath.Walk": true, "os.ReadDir": true,
}

var sourceNames = map[string]bool{"SourcesForIngest": true, "ListAllSymbols": true}

func (c *checker) checkSource(f *ast.File) {
	handler := handlerFile(c.rel, c.servesHTTP(f))
	bg := bgCtxFile(c.rel)
	goOK := bareGoAllowed(c.rel)
	dir := path.Dir(c.rel)
	pkgMapScope := dir == "internal/mcp" || dir == "cmd/fathom"
	if pkgMapScope {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, s := range gd.Specs {
				vs := s.(*ast.ValueSpec)
				if isMapVar(vs) {
					for _, n := range vs.Names {
						if n.Name != "_" {
							c.add(RulePkgMap, n.Pos(), "package-level map var "+n.Name)
						}
					}
				}
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt:
			if !goOK {
				c.add(RuleBareGo, x.Pos(), "go statement outside console.Jobs/refresh/memo/tenancy")
			}
		case *ast.CallExpr:
			p, name, ok := c.pkgSel(x.Fun)
			if !ok {
				break
			}
			full := p + "." + name
			switch {
			case handler && wholeReads[full] && !(full == "io.ReadAll" && c.boundedReader(x)):
				c.add(RuleWholeRead, x.Pos(), full+" in a handler package")
			case full == "os/exec.Command":
				c.add(RuleExecNoCtx, x.Pos(), "exec.Command (use exec.CommandContext)")
			case bg && (full == "context.Background" || full == "context.TODO"):
				c.add(RuleBgCtx, x.Pos(), full+"() in a request-path file")
			case full == "net/http.Serve" || full == "net/http.ListenAndServe":
				c.add(RuleRawServe, x.Pos(), full)
			case handler && p == "github.com/geoffmilleraz/fathom/internal/analysis/callgraph" && name == "Build" && !graphstoreGroup(c.rel):
				c.add(RuleSources, x.Pos(), "callgraph.Build outside the graphstore group")
			}
		case *ast.SelectorExpr:
			if handler && sourceNames[x.Sel.Name] {
				c.add(RuleSources, x.Pos(), x.Sel.Name+" in a handler package")
			}
		}
		return true
	})
}

// servesHTTP: the file names net/http's ResponseWriter or Handler.
func (c *checker) servesHTTP(f *ast.File) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		if p, name, ok := c.pkgSel(asExpr(n)); ok && p == "net/http" && (name == "ResponseWriter" || name == "Handler") {
			found = true
		}
		return true
	})
	return found
}

func asExpr(n ast.Node) ast.Expr {
	if e, ok := n.(ast.Expr); ok {
		return e
	}
	return nil
}

// boundedReader: io.ReadAll's argument is itself a bounding wrapper
// (io.LimitReader or http.MaxBytesReader) — a read with a named ceiling,
// not a whole read.
func (c *checker) boundedReader(call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	inner, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	p, name, ok := c.pkgSel(inner.Fun)
	return ok && ((p == "io" && name == "LimitReader") || (p == "net/http" && name == "MaxBytesReader"))
}

func isMapVar(vs *ast.ValueSpec) bool {
	if vs.Type != nil {
		_, ok := vs.Type.(*ast.MapType)
		return ok
	}
	for _, v := range vs.Values {
		switch e := v.(type) {
		case *ast.CompositeLit:
			if _, ok := e.Type.(*ast.MapType); ok {
				return true
			}
		case *ast.CallExpr:
			if id, ok := e.Fun.(*ast.Ident); ok && id.Name == "make" && len(e.Args) > 0 {
				if _, ok := e.Args[0].(*ast.MapType); ok {
					return true
				}
			}
		}
	}
	return false
}

func (c *checker) checkTest(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BasicLit:
			if x.Kind == token.STRING && strings.Contains(x.Value, ":7575") {
				c.add(RuleTestLiveURL, x.Pos(), "a :7575 literal (the operator's live daemon) in a test")
			}
		case *ast.AssignStmt:
			for i, l := range x.Lhs {
				sel, ok := l.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Env" || i >= len(x.Rhs) {
					continue
				}
				if c.envDropsEnviron(x.Rhs[i]) {
					c.add(RuleTestEnv, x.Pos(), "Env built without os.Environ() (drops FATHOM_HTTP/FATHOM_TEST)")
				}
			}
		case *ast.CompositeLit:
			// exec.Cmd{Env: …} — the one literal whose type is decidable.
			if p, name, ok := c.pkgSel(x.Type); !ok || p != "os/exec" || name != "Cmd" {
				break
			}
			for _, el := range x.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Env" && c.envDropsEnviron(kv.Value) {
					c.add(RuleTestEnv, kv.Pos(), "Env built without os.Environ() (drops FATHOM_HTTP/FATHOM_TEST)")
				}
			}
		}
		return true
	})
}

// envDropsEnviron: a literal slice, nil, or an append that does not start
// from os.Environ() (possibly nested). A plain variable is undecidable
// syntactically and passes.
func (c *checker) envDropsEnviron(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.CompositeLit:
		// A []string literal; a map or struct literal is not an environment.
		at, ok := x.Type.(*ast.ArrayType)
		if !ok {
			return false
		}
		id, ok := at.Elt.(*ast.Ident)
		return ok && id.Name == "string"
	case *ast.Ident:
		return x.Name == "nil"
	case *ast.CallExpr:
		if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "append" && len(x.Args) > 0 {
			first := x.Args[0]
			if call, ok := first.(*ast.CallExpr); ok {
				if p, name, ok := c.pkgSel(call.Fun); ok && p == "os" && name == "Environ" {
					return false
				}
			}
			return c.envDropsEnviron(first)
		}
	}
	return false
}

// applyAllows marks findings excused by a tenancy-allow comment on their
// line or the line directly above, and reports excuses without a reason.
func (c *checker) applyAllows(f *ast.File) {
	type allow struct{ rule, reason string }
	byLine := map[int][]allow{}
	known := map[string]bool{}
	for _, r := range Rules() {
		known[r] = true
	}
	for _, cg := range f.Comments {
		for _, cm := range cg.List {
			if !strings.HasPrefix(cm.Text, allowDirective) {
				continue
			}
			fields := strings.Fields(strings.TrimPrefix(cm.Text, allowDirective))
			line := c.fset.Position(cm.Pos()).Line
			if len(fields) < 2 || !known[fields[0]] {
				c.add(RuleAllowNoReason, cm.Pos(), "tenancy-allow needs a known rule and a reason: "+cm.Text)
				continue
			}
			byLine[line] = append(byLine[line], allow{fields[0], strings.Join(fields[1:], " ")})
		}
	}
	for i := range c.out {
		fd := &c.out[i]
		// The finding's own line, then the run of excuse lines directly
		// above it (one line per rule when a site trips two).
		for ln := fd.Line; ln == fd.Line || len(byLine[ln]) > 0; ln-- {
			for _, a := range byLine[ln] {
				if a.rule == fd.Rule && !fd.Allowed {
					fd.Allowed, fd.Reason = true, a.reason
				}
			}
			if ln == fd.Line && len(byLine[ln-1]) == 0 {
				break
			}
		}
	}
}

// CheckTenancyRules walks cmd/, internal/ and pkg/ under root and applies
// every rule to every .go file (testdata and vendor trees skipped).
func CheckTenancyRules(root string) ([]Finding, error) {
	var out []Finding
	for _, top := range []string{"cmd", "internal", "pkg"} {
		base := filepath.Join(root, top)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "testdata", "vendor", "node_modules":
					return filepath.SkipDir
				}
				if strings.HasPrefix(d.Name(), ".") && p != base {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(p, ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			fs, err := CheckFile(filepath.ToSlash(rel), src)
			if err != nil {
				return err
			}
			out = append(out, fs...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

// Violations returns the findings no excuse covers.
func Violations(fs []Finding) []Finding {
	var v []Finding
	for _, f := range fs {
		if !f.Allowed {
			v = append(v, f)
		}
	}
	return v
}

// RenderAllowlist renders .fathom/tenancy-allow.txt from the excused
// findings: a comment header, then one `rule<TAB>path<TAB>reason` line per
// excuse, sorted. Line numbers are left out so an unrelated edit does not
// churn the file.
func RenderAllowlist(fs []Finding) []byte {
	var lines []string
	for _, f := range fs {
		if f.Allowed {
			lines = append(lines, f.Rule+"\t"+f.Path+"\t"+f.Reason)
		}
	}
	sort.Strings(lines)
	var b bytes.Buffer
	b.WriteString("# The tenancy lint's excuses (design §4.7; plan RT-91, RT-92, RT-137).\n")
	b.WriteString("# Generated from //fathom:tenancy-allow comments by\n")
	b.WriteString("#   FATHOM_TENANCY_ALLOW_UPDATE=1 go test ./internal/lint/tenancy\n")
	b.WriteString("# One line per excused site: rule<TAB>path<TAB>reason. The count may only fall.\n")
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// allowlistCount counts the entries (non-comment, non-blank lines) of a
// rendered allowlist.
func allowlistCount(body []byte) int {
	n := 0
	for _, l := range strings.Split(string(body), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			n++
		}
	}
	return n
}

// allowlistCeiling is the committed number of excuses (RT-92). It may only
// fall: TestTenancyAllowlistOnlyShrinks fails when the allowlist grows past
// it, and when the allowlist shrinks below it until it is lowered to match.
const allowlistCeiling = 164
