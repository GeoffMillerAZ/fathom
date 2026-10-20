package testevidence

// join_nongo.go -- the non-Go half of JoinCasesToSymbols (TQ-28). A symbol
// is a non-Go test entry when repofacts.IsTestEntry says so (pytest
// functions, jest/vitest/mocha blocks, JUnit @Test, xUnit, PHPUnit, Rust).
// A case joins to one by FILE and NAME together: the file comes from the
// case id (a pytest or jest "path::name" or "path > name" id, or a JUnit
// classname "com.acme.FooTest" ahead of the method), the name from the
// case name. Exactly one entry must match; zero or several leave the case
// unjoined, never guessed.

import (
	"path"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/repofacts"
)

type nonGoEntry struct {
	key  string
	file string // repo-relative, forward slashes
	stem string // file without extension
	name string // canonical name
}

var (
	nameSeparators = regexp.MustCompile(`\s*(?:::|>|\.|/)\s*|\s+`)
	trailingParams = regexp.MustCompile(`(?:\[[^\]]*\]|\([^)]*\))$`)
)

// canonName reduces a test name to space-joined tokens: quotes dropped,
// "::", " > ", "." and "/" read as separators, and a trailing "[param]" or
// "()" removed, so jest "calc > adds", pytest "TestCalc::test_zero" and a
// symbol named `"calc" "adds"` all compare on one form.
func canonName(s string) string {
	s = strings.NewReplacer(`"`, "", "'", "", "`", "").Replace(strings.TrimSpace(s))
	s = trailingParams.ReplaceAllString(s, "")
	return strings.Join(nameSeparators.Split(s, -1), " ")
}

func nonGoEntries(symbols []core.SymbolNode) []nonGoEntry {
	var out []nonGoEntry
	for _, s := range symbols {
		if s.Kind == core.KindGoFunc || s.Kind == core.KindGoMethod {
			continue
		}
		if !repofacts.IsTestEntry(s.Name, s.FilePath, s.Signature) {
			continue
		}
		f := strings.ReplaceAll(s.FilePath, "\\", "/")
		out = append(out, nonGoEntry{key: s.Key(), file: f, stem: strings.TrimSuffix(f, path.Ext(f)), name: canonName(s.Name)})
	}
	return out
}

// caseHints returns the candidate file references a case carries and the
// name forms to compare, each name form with a hint's prefix removed.
func caseHints(c Case) (files, names []string) {
	names = append(names, canonName(c.Name))
	for _, s := range []string{c.ID, c.Name} {
		for _, sep := range []string{"::", " > "} {
			if i := strings.Index(s, sep); i > 0 {
				files = append(files, strings.TrimSpace(s[:i]))
				names = append(names, canonName(s[i+len(sep):]))
			}
		}
	}
	if pkg, ok := strings.CutSuffix(c.ID, "."+c.Name); ok && pkg != "" {
		files = append(files, pkg)
	}
	return files, names
}

func fileMatches(e nonGoEntry, hint string) bool {
	h := strings.TrimPrefix(strings.ReplaceAll(hint, "\\", "/"), "./")
	if h == "" {
		return false
	}
	if e.file == h || strings.HasSuffix(e.file, "/"+h) {
		return true
	}
	// A dotted class or module name ("com.acme.FooTest", "tests.test_calc").
	d := strings.ReplaceAll(h, ".", "/")
	return e.stem == d || strings.HasSuffix(e.stem, "/"+d)
}

func nameMatches(entry, c string) bool {
	return c != "" && (entry == c || strings.HasSuffix(entry, " "+c) || strings.HasSuffix(c, " "+entry))
}

// joinNonGo returns the one non-Go test entry c names by file and name, or "".
func joinNonGo(c Case, entries []nonGoEntry) string {
	files, names := caseHints(c)
	if len(files) == 0 {
		return ""
	}
	match, n := "", 0
	for _, e := range entries {
		okFile := false
		for _, f := range files {
			if fileMatches(e, f) {
				okFile = true
				break
			}
		}
		if !okFile {
			continue
		}
		for _, nm := range names {
			if nameMatches(e.name, nm) {
				if e.key != match {
					n++
				}
				match = e.key
				break
			}
		}
	}
	if n == 1 {
		return match
	}
	return ""
}
