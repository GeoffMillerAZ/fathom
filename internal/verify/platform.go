package verify

// platform.go -- a symbol row whose bound symbol is declared only in Go
// files the indexing build context excludes (go/build MatchFile for the
// index's GOOS/GOARCH, no extra tags) cannot be judged by that index
// either way: a Linux index never holds a darwin-only file. Such a row is
// unreadable evidence, an operational outcome (exit 2 under --strict), not
// a failing claim. A symbol declared in any included file is untouched.

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strings"
)

// PlatformExcludedLabel labels a row held Unknown because its symbol lives
// only in files the indexing build context excludes.
const PlatformExcludedLabel = "platform-excluded"

// PlatformMarker ends the detail of a criterion held Unknown that way.
const PlatformMarker = "[platform-excluded]"

// IsPlatformCriterion reports whether c is a criterion held Unknown by the
// platform exclusion.
func IsPlatformCriterion(c Criterion) bool {
	return c.Status == Unknown && strings.HasSuffix(c.Detail, " "+PlatformMarker)
}

// platformCriteria are the symbol-derived criteria an excluded definition
// makes unreadable.
var platformCriteria = map[string]bool{"symbol_present": true, "tier": true, "test_reaches": true, "edge_wired": true, "wired": true}

// declaresSymbol reports whether src declares symbol: a func F, or for
// T.M a method M on T or *T.
func declaresSymbol(src []byte, symbol string) bool {
	recv, name := "", symbol
	if i := strings.LastIndex(symbol, "."); i >= 0 {
		recv, name = symbol[:i], symbol[i+1:]
	}
	if !strings.Contains(string(src), name) {
		return false
	}
	f, _ := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	if f == nil {
		return false
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != name {
			continue
		}
		if recv == "" {
			if fd.Recv == nil {
				return true
			}
			continue
		}
		if fd.Recv == nil || len(fd.Recv.List) == 0 {
			continue
		}
		t := fd.Recv.List[0].Type
		if st, ok := t.(*ast.StarExpr); ok {
			t = st.X
		}
		if ix, ok := t.(*ast.IndexExpr); ok {
			t = ix.X
		}
		if id, ok := t.(*ast.Ident); ok && id.Name == recv {
			return true
		}
	}
	return false
}

// platformExcludedFiles returns the corpus files declaring symbol when
// every one of them is build-excluded; nil when none declares it or any
// included file does.
func platformExcludedFiles(ev Evidence, symbol, pkg string) []string {
	ge, ok := ev.(GoneEvidence)
	if !ok || symbol == "" {
		return nil
	}
	var excluded []string
	for _, f := range ge.GoFiles() {
		if pkg != "" && path.Dir(f.Path) != strings.TrimSuffix(pkg, "/") && !strings.HasPrefix(f.Path, strings.TrimSuffix(pkg, "/")+"/") {
			continue
		}
		if !declaresSymbol(f.Content, symbol) {
			continue
		}
		if f.Excluded == "" {
			return nil
		}
		excluded = append(excluded, f.Path)
	}
	sort.Strings(excluded)
	return excluded
}

// applyPlatformExclusion rewrites v's symbol-derived failures to
// platform-excluded Unknowns when the symbol is declared only in
// build-excluded files. Any other failing criterion keeps the row Fail.
func applyPlatformExclusion(v *TaskVerdict, t Task, ev Evidence) {
	if v.Status == Pass || t.Expect.Symbol == "" {
		return
	}
	files := platformExcludedFiles(ev, t.Expect.Symbol, t.Expect.Package)
	if len(files) == 0 {
		return
	}
	detail := "platform: defined only in " + strings.Join(files, ", ") + ", excluded on " + build.Default.GOOS + "/" + build.Default.GOARCH + " " + PlatformMarker
	converted, otherFail := false, false
	for i, c := range v.Criteria {
		switch {
		case platformCriteria[c.Name] && (c.Status == Fail || c.Status == Unknown):
			v.Criteria[i] = Criterion{Name: c.Name, Status: Unknown, Detail: detail}
			converted = true
		case c.Status == Fail:
			otherFail = true
		}
	}
	if converted && !otherFail {
		v.Status, v.Label = Unknown, PlatformExcludedLabel
	}
}
