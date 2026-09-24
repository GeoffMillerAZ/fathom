package integrity

// scan.go — the one Go-source scanner GuardedTests, Check and
// NormalizedBodyHash all share: parse a file once (go/parser, comments
// kept), then extract, per top-level Test* function, exactly the facts
// design §1.2/§1.3 need (certifies marker text, assertion-call count, skip/
// build-tag guards, testdata/ paths referenced, and the *ast.FuncDecl
// itself for NormalizedBodyHash). One scan serves every caller so a file is
// never re-parsed per question asked of it.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// scannedTest is everything scan.go's walk recovers about one top-level
// Test* function.
type scannedTest struct {
	Name string
	File string
	Line int
	// Decl is the parsed function — NormalizedBodyHash's own input (via
	// normalizedBodyHashOfDecl), kept here so Check need not re-parse the
	// file a second time to hash a body it already found.
	Decl *ast.FuncDecl
	// CertifiesMarker is the trimmed text following "// certifies:" in the
	// function's doc comment, "" when absent.
	CertifiesMarker string
	// AssertCount is the number of assertion-shaped calls in the body
	// (design §1.3's "t.Error*, t.Fatal*, t.Fail*, and assert.*,
	// require.*, cmp.*, is.* selectors").
	AssertCount int
	// HasSkip is true when the body calls t.Skip*/t.SkipNow or
	// testing.Short() (design §1.3's skipped check; the "//go:build line
	// added to the file" half of that check is file-level, see
	// scanGoFile's buildTagAdded return).
	HasSkip bool
	// GoldenPaths is every distinct "testdata/…" substring found inside a
	// string literal in the body, sorted.
	GoldenPaths []string
}

// scanGoFile parses src (path is used only for scannedTest.File and error
// messages — it need not exist on disk) and returns every top-level Test*
// function it finds, plus whether the file's leading comment carries a
// "//go:build" (or the legacy "// +build") constraint line — design §1.3's
// "a //go:build line was added to the file" half of the skipped check. ok
// is false when src does not parse as Go at all (never a fatal error to a
// caller scanning many files, one of which might be malformed mid-edit).
func scanGoFile(path string, src []byte) (tests []scannedTest, hasBuildTag bool, ok bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, false, false
	}
	for _, cg := range f.Comments {
		// A build-constraint comment must precede the package clause to be
		// legal Go; f.Comments is already only the file's comment groups,
		// so checking "starts before Package" is exactly that test without
		// a second parse pass.
		if cg.Pos() >= f.Package {
			continue
		}
		for _, c := range cg.List {
			t := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if strings.HasPrefix(t, "go:build") || strings.HasPrefix(t, "+build") {
				hasBuildTag = true
			}
		}
	}
	for _, decl := range f.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Recv != nil || fn.Body == nil {
			continue // only top-level, non-method funcs with a body — same convention certify/compile.go's testSymbolsByBareName uses
		}
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		st := scannedTest{
			Name: fn.Name.Name,
			File: path,
			Line: fset.Position(fn.Pos()).Line,
			Decl: fn,
		}
		if fn.Doc != nil {
			st.CertifiesMarker = certifiesMarkerText(fn.Doc)
		}
		st.AssertCount, st.HasSkip, st.GoldenPaths = scanBody(fn.Body)
		tests = append(tests, st)
	}
	return tests, hasBuildTag, true
}

// certifiesMarkerRe-equivalent: a doc-comment line of the form
// "// certifies: <rest>" (design §1.2's own marker convention — the same
// vocabulary internal/certify's declarations use, TC-5). Matched by hand
// (TrimPrefix, not regexp) since the shape is a single fixed literal
// prefix, case-sensitive, exactly as every other marker/annotation
// convention in this repo reads its own fixed keyword prefixes.
func certifiesMarkerText(doc *ast.CommentGroup) string {
	for _, c := range doc.List {
		t := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if rest, ok := strings.CutPrefix(t, "certifies:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// assertSelectors is the design §1.3 "t.Error*, t.Fatal*, t.Fail*"
// family: a CallExpr's SelectorExpr.Sel.Name starting with one of these
// (covers Error/Errorf, Fatal/Fatalf, Fail/FailNow) on ANY receiver name —
// tests routinely call their *testing.T under "t", "tt", "st" (subtests)
// or a helper's own parameter name, so the receiver's spelling is never
// part of the match.
var assertSelectorPrefixes = []string{"Error", "Fatal", "Fail"}

// assertPackages is design §1.3's "assert.*, require.*, cmp.*, is.*"
// family: any call whose receiver/package qualifier is one of these idents
// counts as an assertion call regardless of which method — e.g.
// assert.Equal, require.NoError, cmp.Diff, is.True (github.com/matryer/is).
var assertPackages = map[string]bool{"assert": true, "require": true, "cmp": true, "is": true}

// skipSelectors is design §1.3's "t.Skip*" family.
var skipSelectorPrefixes = []string{"Skip"}

// scanBody walks body once and returns the assertion-call count, whether a
// skip guard (t.Skip*/testing.Short()) is present, and every distinct
// "testdata/…" path substring referenced in a string literal — the three
// per-function body facts design §1.3's kind table needs beyond the
// normalized hash itself.
func scanBody(body *ast.BlockStmt) (assertCount int, hasSkip bool, goldenPaths []string) {
	golden := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
				if x, isIdent := sel.X.(*ast.Ident); isIdent {
					if x.Name == "testing" && sel.Sel.Name == "Short" {
						hasSkip = true
					}
					if assertPackages[x.Name] {
						assertCount++
					}
				}
				if hasPrefixAny(sel.Sel.Name, assertSelectorPrefixes) {
					assertCount++
				}
				if hasPrefixAny(sel.Sel.Name, skipSelectorPrefixes) {
					hasSkip = true
				}
			}
		case *ast.BasicLit:
			if v.Kind == token.STRING {
				if s, err := strconv.Unquote(v.Value); err == nil {
					if p := goldenPathIn(s); p != "" {
						golden[p] = true
					}
				}
			}
		}
		return true
	})
	for p := range golden {
		goldenPaths = append(goldenPaths, p)
	}
	sort.Strings(goldenPaths)
	return assertCount, hasSkip, goldenPaths
}

// goldenPathIn returns the "testdata/…" substring of s starting at its
// first occurrence of "testdata/", trimmed of anything js/format-verb-like
// trailing a %s or similar that would make it an unusable literal path —
// deliberately simple (a whole-token match on the literal as written): a
// dynamically-built path (fmt.Sprintf("testdata/%s.golden", name)) still
// contributes its static "testdata/" prefix, which is enough to detect
// that SOME golden fixture under that directory is referenced; a body that
// changes which file within an unchanged directory it opens is a
// deliberately accepted false negative rather than a heavier per-call
// data-flow analysis this package refuses to build (ADR-044 §8: "a
// transformation engine" and heavier inference are out of scope; the
// finding stays honestly narrower rather than fabricating certainty).
func goldenPathIn(s string) string {
	const marker = "testdata/"
	i := strings.Index(s, marker)
	if i < 0 {
		return ""
	}
	return s[i:]
}

func hasPrefixAny(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
