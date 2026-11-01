package main

// tests_select_inputs.go — TQ-33 and TQ-34 (docs/plans/test-intelligence.md,
// ADR-090): the tests a diff reaches by what they read, and the tests the
// index has not seen yet.
//
// A test that walks the source tree, globs docs/plans or reads CHANGELOG.md
// through a path computed at run time reaches the changed file through no
// call edge and no literal (TQ-24 matches 150 of 1,359 reads), so `tests
// select` named none of the reader tests behind seven failed pre-push gates
// in five days (2026-10-26 to 10-31). The files they read are on record
// anyway: Go's testlog, which scripts/test-inputs.sh asks every test binary for
// (`task test:inputs`, internal/analysis/testinputs). A changed file selects
// every test the observation saw open or stat it, and a file added, deleted
// or renamed also selects every test that opened its directory. A package
// observed only as a whole runs whole. The changed files now include
// untracked files and both sides of a rename, and a Test function in a
// changed test file that the index does not hold yet is selected by name.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/analysis/testinputs"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// testsSelectChanges lists what changed since since: every path git diff
// names, with renames split into the old and the new path (--no-renames),
// plus the untracked files, NUL-separated so a non-ASCII path is not
// quoted. Listing marks the paths added or deleted (an untracked file is
// added), and ListingDir the deepest ancestor that existed at since, whose
// listing that changed: a file added in a new directory changes the listing
// of the first directory that was already there. Sorted by path.
func testsSelectChanges(gitRunner func(args ...string) ([]byte, error), since string) ([]testinputs.Change, error) {
	out, err := gitRunner("diff", "--name-only", "--no-renames", "-z", since)
	if err != nil {
		return nil, err
	}
	listing := map[string]bool{}
	if added, aerr := gitRunner("diff", "--name-only", "--no-renames", "-z", "--diff-filter=AD", since); aerr == nil {
		for _, p := range gitPathLines(added) {
			listing[p] = true
		}
	}
	paths := map[string]bool{}
	for _, p := range gitPathLines(out) {
		paths[p] = true
	}
	// Best effort: an untracked file is a change the working tree holds and
	// git diff never names.
	if untracked, uerr := gitRunner("ls-files", "-z", "--others", "--exclude-standard"); uerr == nil {
		for _, p := range gitPathLines(untracked) {
			paths[p] = true
			listing[p] = true
		}
	}
	existed := map[string]bool{".": true}
	changes := make([]testinputs.Change, 0, len(paths))
	for p := range paths {
		c := testinputs.Change{Path: p, Listing: listing[p]}
		if c.Listing {
			c.ListingDir = deepestExistingDir(gitRunner, since, path.Dir(p), existed)
		}
		changes = append(changes, c)
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, nil
}

// deepestExistingDir walks up from dir to the first directory the tree at
// since holds (git cat-file -e <since>:<dir>), memoized in existed. The
// root always exists.
func deepestExistingDir(gitRunner func(args ...string) ([]byte, error), since, dir string, existed map[string]bool) string {
	for dir != "." && dir != "" {
		ok, seen := existed[dir]
		if !seen {
			_, err := gitRunner("cat-file", "-e", since+":"+dir)
			ok = err == nil
			existed[dir] = ok
		}
		if ok {
			return dir
		}
		dir = path.Dir(dir)
	}
	return "."
}

// gitPathLines splits git's NUL-separated path list (a newline-separated
// one, from an older caller or a test's fake, splits the same way).
func gitPathLines(b []byte) []string {
	var out []string
	for _, line := range strings.FieldsFunc(string(b), func(r rune) bool { return r == 0 || r == '\n' }) {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func changedPaths(changes []testinputs.Change) []string {
	out := make([]string, len(changes))
	for i, c := range changes {
		out[i] = c.Path
	}
	return out
}

// inputSelection is what the observed inputs and the changed test files add
// to a selection.
type inputSelection struct {
	Dir     string
	Missing bool   // no observation at Dir
	Err     string // an observation that did not load
	Commit  string
	At      string
	Behind  int // commits from the observed commit to HEAD; -1 when unknown
	Hits    []testinputs.Hit
	Tests   []string            // index keys of the tests a per-test hit names
	Named   map[string][]string // package -> test names to run that the index does not hold
	Whole   []string            // packages a package-wide observation selects whole
	Partial []string            // packages whose observation is not whole (they run whole)
	New     map[string][]string // package -> new Test functions in changed test files
}

// selectByInputs joins the observation in dir (a missing one is noted, not
// an error: the reach selection still answers) and the changed test files.
func selectByInputs(dir, root string, nodes []core.SymbolNode, changes []testinputs.Change, gitRunner func(args ...string) ([]byte, error)) inputSelection {
	ins := inputSelection{Dir: dir, Behind: -1, Named: map[string][]string{}, New: map[string][]string{}}
	byPkgName := testNodesByPackage(nodes)

	ins.New = newTestsInChangedFiles(root, byPkgName, changedPaths(changes))
	for pkg, names := range ins.New {
		ins.Named[pkg] = append(ins.Named[pkg], names...)
	}

	if _, err := os.Stat(filepath.Join(dir, "meta.json")); err != nil {
		ins.Missing = true
		return finishNamed(ins)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	m, err := testinputs.Load(dir, absRoot)
	if err != nil {
		ins.Err = err.Error()
		return finishNamed(ins)
	}
	ins.Commit, ins.At = m.Commit, m.At
	if isHexCommit(m.Commit) {
		if out, gerr := gitRunner("rev-list", "--count", m.Commit+"..HEAD"); gerr == nil {
			if n, perr := strconv.Atoi(strings.TrimSpace(string(out))); perr == nil {
				ins.Behind = n
			}
		}
	}
	ins.Hits = m.Readers(changes)
	ins.Partial = m.PartialPackages()
	whole := map[string]bool{}
	keys := map[string]bool{}
	for _, h := range ins.Hits {
		if !inRootModule(root, filepath.Join(filepath.FromSlash(h.Package), "x.go")) {
			continue
		}
		if h.Test == "" {
			whole[h.Package] = true
			continue
		}
		if k, ok := byPkgName[h.Package][h.Test]; ok {
			keys[k] = true
		} else {
			ins.Named[h.Package] = append(ins.Named[h.Package], h.Test)
		}
	}
	ins.Whole = sortedSet(whole)
	ins.Tests = sortedSet(keys)
	return finishNamed(ins)
}

func finishNamed(ins inputSelection) inputSelection {
	for pkg, names := range ins.Named {
		ins.Named[pkg] = dedupSorted(sortedCopy(names))
	}
	return ins
}

func sortedCopy(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return out
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// testNodesByPackage indexes the Go test functions by package directory and
// name, to their index keys.
func testNodesByPackage(nodes []core.SymbolNode) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, n := range nodes {
		f := filepath.ToSlash(n.FilePath)
		if n.Kind != core.KindGoFunc || !strings.HasSuffix(f, "_test.go") || !isGoTestName(n.Name) {
			continue
		}
		dir := path.Dir(f)
		if out[dir] == nil {
			out[dir] = map[string]string{}
		}
		out[dir][n.Name] = n.Key()
	}
	return out
}

// newTestsInChangedFiles parses each changed _test.go file in the working
// tree and returns, per package directory, the top-level Test functions the
// index does not hold for that package: a test written since the index was
// built, which no reach can select because no edge to it exists yet.
func newTestsInChangedFiles(root string, known map[string]map[string]string, changed []string) map[string][]string {
	out := map[string][]string{}
	fset := token.NewFileSet()
	for _, rel := range changed {
		if !strings.HasSuffix(rel, "_test.go") || !inRootModule(root, rel) || ignoredByGoList(rel) {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			continue // deleted, or not Go yet: nothing to run by name
		}
		dir := path.Dir(rel)
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !isGoTestName(fd.Name.Name) || fd.Type.Params.NumFields() != 1 {
				continue
			}
			if _, ok := known[dir][fd.Name.Name]; !ok {
				out[dir] = append(out[dir], fd.Name.Name)
			}
		}
	}
	for dir, names := range out {
		out[dir] = dedupSorted(sortedCopy(names))
	}
	return out
}

// ignoredByGoList is go list's rule for directories a package pattern never
// matches: a path segment named testdata, or starting with "_" or ".". A
// test file there is a fixture, never a test the gate runs.
func ignoredByGoList(rel string) bool {
	segs := strings.Split(path.Dir(rel), "/")
	for _, seg := range segs {
		if seg == "testdata" || strings.HasPrefix(seg, "_") || (strings.HasPrefix(seg, ".") && seg != ".") {
			return true
		}
	}
	return false
}

// isGoTestName is go test's rule: Test, then nothing or a character that is
// not a lower-case letter.
func isGoTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	if len(name) == 4 {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[4:])
	return !unicode.IsLower(r)
}

// mergeInputInvocations adds the named tests to their package's invocation
// (or a new one) and makes every whole package one `go test ./<pkg>` with no
// -run, replacing a narrowed line for it. Sorted by package.
func mergeInputInvocations(invs []testInvocation, ins inputSelection) []testInvocation {
	byPkg := map[string]testInvocation{}
	for _, inv := range invs {
		byPkg[inv.Package] = inv
	}
	for pkg, names := range ins.Named {
		inv := byPkg[pkg]
		inv.Package = pkg
		inv.Tests = dedupSorted(sortedCopy(append(inv.Tests, names...)))
		byPkg[pkg] = inv
	}
	for _, pkg := range ins.Whole {
		inv := byPkg[pkg]
		inv.Package = pkg
		inv.Whole = true
		byPkg[pkg] = inv
	}
	out := make([]testInvocation, 0, len(byPkg))
	for _, inv := range byPkg {
		inv.Command = invocationCommand(inv)
		out = append(out, inv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Package < out[j].Package })
	return out
}

func invocationCommand(inv testInvocation) string {
	pkgArg := inv.Package
	if pkgArg == "" {
		pkgArg = "."
	}
	if inv.Whole {
		return "go test ./" + pkgArg
	}
	quoted := make([]string, len(inv.Tests))
	for i, n := range inv.Tests {
		quoted[i] = regexp.QuoteMeta(n)
	}
	return "go test ./" + pkgArg + " -run '^(" + strings.Join(quoted, "|") + ")$'"
}

func inputSelectionJSON(ins inputSelection) map[string]any {
	hits := make([]any, len(ins.Hits))
	for i, h := range ins.Hits {
		m := map[string]any{"file": h.File, "package": h.Package, "via": h.Via}
		if h.Test != "" {
			m["test"] = h.Test
		}
		hits[i] = m
	}
	out := map[string]any{
		"dir":              ins.Dir,
		"missing":          ins.Missing,
		"hits":             hits,
		"whole_packages":   nonNilStrList(ins.Whole),
		"partial_packages": nonNilStrList(ins.Partial),
	}
	if ins.Err != "" {
		out["error"] = ins.Err
	}
	if ins.Commit != "" {
		out["commit"] = ins.Commit
		out["at"] = ins.At
		if ins.Behind >= 0 {
			out["commits_behind_head"] = ins.Behind
		}
	}
	return out
}

func newTestsJSON(ins inputSelection) map[string]any {
	out := map[string]any{}
	for pkg, names := range ins.New {
		out[pkg] = nonNilStrList(names)
	}
	return out
}

// inputSelectionNote is the one stderr line about the observation.
func inputSelectionNote(ins inputSelection) string {
	switch {
	case ins.Missing:
		return "note: tests select: no observed test inputs at " + ins.Dir + ", so a test that reads a changed file through a computed path is not selected; `task test:inputs` observes them"
	case ins.Err != "":
		return "warning: tests select: the observed test inputs did not load (" + ins.Err + "); selecting by reach alone"
	}
	tests := 0
	for _, h := range ins.Hits {
		if h.Test != "" {
			tests++
		}
	}
	behind := "an unknown number of"
	if ins.Behind >= 0 {
		behind = strconv.Itoa(ins.Behind)
	}
	note := "note: tests select: observed inputs (" + shortSHA(ins.Commit) + ", " + behind + " commit(s) behind HEAD): " +
		strconv.Itoa(tests) + " test read(s) and " + strconv.Itoa(len(ins.Whole)) + " whole package(s) match the changed files"
	if len(ins.Partial) > 0 {
		note += "; " + strconv.Itoa(len(ins.Partial)) + " package(s) observed only in part run whole (" + strings.Join(ins.Partial, ", ") + ")"
	}
	return note
}
