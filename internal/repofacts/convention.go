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

// IsTestEntry reports whether a symbol is a test entry point: a Test-named
// function or method (Go's convention, which callgraph's test reach also
// reads) declared in a test file. A production TestHelper is not one.
func IsTestEntry(name, file string) bool {
	if i := strings.LastIndexAny(name, ".:"); i >= 0 {
		name = name[i+1:]
	}
	return strings.HasPrefix(name, "Test") && IsTest(file)
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
