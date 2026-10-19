// Package repofacts is fathom's one test and vendor rule (RF-2, ADR-067
// decision 2): the path convention every product reader of "is this a test
// file?" and "is this vendored?" calls, so no second copy of either rule
// lives in product code (TestNoSecondTestOrVendorRuleInProductCode).
//
// It is a pure leaf. It imports no fathom package (TestRepofactsIsAPureLeaf):
// fusion reaches it through internal/vendored, and every other consumer may
// import it.
//
// The rule is the core table below (universal, K15: no framework and no
// Linguist data) plus the dialect-pack data in packs.json (each ecosystem's
// own test conventions), read from the path alone. RF-1's role classifier
// over a closed vocabulary, and RF-3's attribute, header, dialect and
// Linguist bases, were refused when RF-M8 read CUT
// (docs/bench/readings/repository-facts/roles-r1.md): a path does not carry
// what each runner's configuration collects, and an unwired classifier does
// not stay in the product (the owner-architect's ruling R1).
package repofacts

import (
	_ "embed"
	"encoding/json"
	"path"
	"strings"
)

//go:embed packs.json
var packsJSON []byte

// pack is one ecosystem's framework conventions (packs.json).
type pack struct {
	Dialect         string   `json:"dialect"`
	Exts            []string `json:"exts"`
	TestSuffixes    []string `json:"test_suffixes"`
	TestPrefixes    []string `json:"test_prefixes"`
	TestInfixes     []string `json:"test_infixes"`
	TestDirs        []string `json:"test_dirs"`
	TestDirPrefixes []string `json:"test_dir_prefixes"`
	FixtureNames    []string `json:"fixture_names"`
	SupportDirs     []string `json:"support_dirs"`
	EntryPrefixes   []string `json:"entry_prefixes"`
	EntryAttributes []string `json:"entry_attributes"`
	EntryCalls      []string `json:"entry_calls"`
}

// packByExt maps a lower-case extension to its pack. Built once at init
// from embedded data, read-only after.
var packByExt = loadPacks()

func loadPacks() map[string]*pack {
	var doc struct {
		Packs []*pack `json:"packs"`
	}
	if err := json.Unmarshal(packsJSON, &doc); err != nil {
		panic("repofacts: packs.json: " + err.Error())
	}
	m := map[string]*pack{}
	for _, p := range doc.Packs {
		for _, e := range p.Exts {
			m[strings.ToLower(e)] = p
		}
	}
	return m
}

// The core table's directory segments, matched case-sensitively unless
// noted. A segment names a directory, never the file's own base name.
var (
	vendorDirs    = map[string]bool{"vendor": true, "node_modules": true, "third_party": true}
	generatedDirs = map[string]bool{"generated": true, "__generated__": true}
	fixtureDirs   = map[string]bool{"testdata": true, "fixtures": true, "fixture": true, "__fixtures__": true, "__snapshots__": true}
	// testDirs are matched case-insensitively (test/, tests/, Tests/).
	testDirs = map[string]bool{"test": true, "tests": true}
)

// dirSegments returns rel's directory segments (every segment but the
// last). A path with a trailing slash names a directory: all its segments.
func dirSegments(rel string) []string {
	rel = strings.TrimPrefix(rel, "./")
	if strings.HasSuffix(rel, "/") {
		return strings.Split(strings.TrimSuffix(rel, "/"), "/")
	}
	i := strings.LastIndexByte(rel, '/')
	if i < 0 {
		return nil
	}
	return strings.Split(rel[:i], "/")
}

func anyDir(rel string, set map[string]bool, fold bool) bool {
	for _, s := range dirSegments(rel) {
		if fold {
			s = strings.ToLower(s)
		}
		if set[s] {
			return true
		}
	}
	return false
}

func hasDir(rel string, dirs []string) bool {
	for _, s := range dirSegments(rel) {
		for _, d := range dirs {
			if s == d {
				return true
			}
		}
	}
	return false
}

// IsVendored reports whether rel lies in a vendored tree by the core
// convention: a directory segment vendor, node_modules or third_party, at
// any depth. A bare segment ("vendor") is read as that directory itself, so
// a caller testing one directory name at a time gets the same answer.
func IsVendored(rel string) bool {
	if rel != "" && !strings.Contains(rel, "/") {
		return vendorDirs[rel]
	}
	return anyDir(rel, vendorDirs, false)
}

// IsTest reports whether rel is a test file by convention: its base name
// matches its ecosystem's pack pattern (_test.go, test_*.py, *.test.ts,
// FooTest.java ...) or it is a pack file under a pack test directory
// (__tests__/, a crate's tests/), and it lies in no vendored, generated or
// fixture tree. It is the one test rule product code calls (RF-2).
func IsTest(rel string) bool {
	return classify(rel) == testFile
}

// IsTestMaterial reports whether rel is a test file or test-support
// material: IsTest, or under a fixture directory (testdata, fixtures,
// __snapshots__ ...), a pack's fixture file (conftest.py), or a file under a
// test directory (test/, tests/, a pack's support directory) that is not
// itself a test. Code and data no product path runs, never a risk target, a
// served route or production search noise.
func IsTestMaterial(rel string) bool {
	c := classify(rel)
	return c == testFile || c == testSupport
}

// IsTestEntry reports whether a symbol is a test entry point, a symbol its
// runner collects, declared in a test file (IsTest): a Test-named function
// or method (Go's convention, which callgraph's test reach also reads, in a
// test file of any language), or one its file's pack names (packs.json,
// TQ-27): a name prefix (pytest's test functions, PHPUnit's test methods),
// an attribute leading the signature (JUnit's @Test, xUnit's [Fact] and
// [Theory], NUnit's [Test], MSTest's [TestMethod], PHPUnit's #[Test],
// Rust's #[test]) or a registering call (a jest, vitest or mocha describe,
// it or test block). A production TestHelper is not one, and neither is a
// test file's helper.
func IsTestEntry(name, file, signature string) bool {
	if !IsTest(file) {
		return false
	}
	if i := strings.LastIndexAny(name, ".:"); i >= 0 {
		name = name[i+1:]
	}
	if strings.HasPrefix(name, "Test") {
		return true
	}
	p := packByExt[strings.ToLower(path.Ext(file))]
	if p == nil {
		return false
	}
	for _, s := range p.EntryPrefixes {
		if strings.HasPrefix(name, s) {
			return true
		}
	}
	for _, c := range p.EntryCalls {
		if rest, ok := strings.CutPrefix(signature, c); ok && rest != "" && (rest[0] == '(' || rest[0] == '.') {
			return true
		}
	}
	if len(p.EntryAttributes) > 0 {
		for _, a := range leadingAttributes(signature) {
			for _, want := range p.EntryAttributes {
				if a == want {
					return true
				}
			}
		}
	}
	return false
}

// leadingAttributes is the names of the attributes and annotations that
// open sig, in order, each by its last path segment: Java's @a.b.Test,
// C#'s [Fact] and [Theory, InlineData(1)], PHP's #[Test] and Rust's
// #[test] or #[tokio::test]. The scan stops at the first token that opens
// none, so a parameter's annotation or an array type is never read.
func leadingAttributes(sig string) []string {
	var names []string
	i := 0
	for {
		for i < len(sig) && sig[i] == ' ' {
			i++
		}
		switch {
		case strings.HasPrefix(sig[i:], "@"):
			name, n := attrPath(sig[i+1:])
			if name == "" {
				return names
			}
			names = append(names, name)
			i += 1 + n
			if i < len(sig) && sig[i] == '(' {
				i += balancedLen(sig[i:], '(', ')')
			}
		case strings.HasPrefix(sig[i:], "#["), strings.HasPrefix(sig[i:], "["):
			if sig[i] == '#' {
				i++
			}
			group := balancedLen(sig[i:], '[', ']')
			if group < 2 {
				return names
			}
			for _, item := range splitTopLevel(sig[i+1 : i+group-1]) {
				if name, _ := attrPath(strings.TrimSpace(item)); name != "" {
					names = append(names, name)
				}
			}
			i += group
		default:
			return names
		}
	}
}

// attrPath reads a dotted, :: or \ separated path at the start of s and
// returns its last segment and the bytes read.
func attrPath(s string) (string, int) {
	i, seg := 0, 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9':
			i++
		case c == '.' || c == '\\':
			i++
			seg = i
		case c == ':' && i+1 < len(s) && s[i+1] == ':':
			i += 2
			seg = i
		default:
			return s[seg:i], i
		}
	}
	return s[seg:i], i
}

// balancedLen is the length of the bracketed group open...close at the
// start of s, nested groups and quoted strings skipped; 0 when s does not
// open one or never closes it.
func balancedLen(s string, open, close byte) int {
	if s == "" || s[0] != open {
		return 0
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'':
			j := strings.IndexByte(s[i+1:], c)
			if j < 0 {
				return 0
			}
			i += 1 + j
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return 0
}

// splitTopLevel splits s at the commas outside parentheses, brackets and
// quoted strings.
func splitTopLevel(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'':
			if j := strings.IndexByte(s[i+1:], c); j >= 0 {
				i += 1 + j
			}
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// testClass is what the convention says of a path for the delegates.
type testClass int

const (
	notTest     testClass = iota
	testFile              // a test file by its pack's patterns
	testSupport           // test-support material: fixture trees, fixture files, the rest of a test tree
)

// classify applies the rule in order, first match wins: a vendored or
// generated tree is never test material, a fixture tree always is, then the
// pack's test patterns and fixture files, then any file under a test
// directory.
func classify(rel string) testClass {
	rel = strings.TrimPrefix(rel, "./")
	base := path.Base(rel)
	if anyDir(rel, vendorDirs, false) || anyDir(rel, generatedDirs, false) {
		return notTest
	}
	if anyDir(rel, fixtureDirs, false) {
		return testSupport
	}
	if p := packByExt[strings.ToLower(path.Ext(base))]; p != nil {
		if packTest(p, rel, base) {
			return testFile
		}
		for _, n := range p.FixtureNames {
			if base == n {
				return testSupport
			}
		}
		if hasDir(rel, p.SupportDirs) {
			return testSupport
		}
	}
	if anyDir(rel, testDirs, true) {
		return testSupport
	}
	return notTest
}

// packTest reports whether base, a file of pack p at rel, is a test by the
// pack's own patterns.
func packTest(p *pack, rel, base string) bool {
	for _, s := range p.TestSuffixes {
		if strings.HasSuffix(base, s) && len(base) > len(s) {
			return true
		}
	}
	for _, s := range p.TestPrefixes {
		if strings.HasPrefix(base, s) {
			return true
		}
	}
	for _, s := range p.TestInfixes {
		if strings.Contains(base, s) {
			return true
		}
	}
	if hasDir(rel, p.TestDirs) {
		return true
	}
	if len(p.TestDirPrefixes) > 0 && anyDir(rel, testDirs, true) {
		for _, s := range p.TestDirPrefixes {
			if strings.HasPrefix(base, s) && len(base) > len(s)+len(path.Ext(base)) {
				return true
			}
		}
	}
	return false
}
