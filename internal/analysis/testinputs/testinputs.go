// Package testinputs reads the files a repository's Go tests were observed
// to read (TQ-31, ADR-090): Go's own testlog, the record `go test` writes
// through `-test.testlogfile` and keys its test cache on, one log per
// package and, for the packages scripts/test-inputs.sh runs test by test,
// one per top-level test. Selection joins it so a changed plan, changelog,
// reading or source file selects the tests that read it, which the call
// graph cannot see (a test that walks the tree or globs docs/ reads through
// a path computed at run time).
//
// The observation is a named source on disk, never a store table (the
// table freeze until RT-M1), and it is evidence of one run at one commit:
// a test that reads a file only on a branch the run did not take is not in
// it, and neither is a read a subprocess makes, which is why the push gate
// still runs the whole suite.
package testinputs

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Observed is what one testlog recorded: the repository files and
// directories the tests opened or stat'ed, repository-relative with forward
// slashes ("." is the root itself), the environment variables they read,
// and counts of what was left out.
type Observed struct {
	Files   []string
	Env     []string
	Outside int // opens and stats outside the repository (temp dirs, GOROOT, the module cache)
	Unknown int // lines with an op this reader does not know, skipped
}

// Package is one package's observation: the package-wide log, and per
// top-level test when the package was run test by test. Unobserved names
// the tests of a per-test run that left no log of their own (the isolated
// run failed or wrote nothing); the package-wide log stands for them.
//
// Partial is true when the package-wide log cannot be trusted to be whole:
// it is missing or unreadable, or the run ended other than by returning
// from its tests (an exit code other than 0 or 1). Go buffers the testlog
// and flushes it when the tests return, failing ones included (exit 1), so
// only a test that panics (2), calls os.Exit, or is killed leaves it cut
// short. A partial package runs whole for every change.
type Package struct {
	Observed
	Exit       int
	PerTest    bool
	Partial    bool
	Tests      map[string]Observed
	Unobserved []string
}

// Map is one observation run.
type Map struct {
	Commit   string
	At       string
	Packages map[string]Package // keyed by repository-relative package directory
}

// Change is one changed path. Listing is true when the path was added,
// deleted or renamed, so a directory listing changed too: ListingDir's, the
// deepest ancestor of Path that existed before the change (Path's parent
// when empty; a file added in a new directory changes the listing of the
// first ancestor that already existed).
type Change struct {
	Path       string
	Listing    bool
	ListingDir string
}

// Hit is one test or package that read a changed path. Test is empty when
// the whole package stands for it. Via is "open" (the path itself was
// opened or stat'ed), "dir" (the directory whose listing changed was
// opened), "package" (the package-wide log read the path, for a test with no
// log of its own or a read no test's log explains) or "partial" (the
// package's observation is not whole).
type Hit struct {
	Package string
	Test    string
	File    string
	Via     string
}

const header = "# test log"

// ParseLog reads one Go testlog: the `# test log` header, then one
// `<op> <name>` line per os call the tests made. open and stat name a path,
// resolved against pkgDir (the directory `go test` runs the binary in) and
// every chdir before it; getenv names a variable. A path under one of roots
// (the repository root, and its other spellings) becomes
// repository-relative against the longest root that holds it; any other
// path is counted in Outside. An op this reader does not know is counted in
// Unknown, so a newer Go that logs more still parses.
func ParseLog(r io.Reader, pkgDir string, roots ...string) (Observed, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != header {
		if err := sc.Err(); err != nil {
			return Observed{}, err
		}
		return Observed{}, errors.New("testinputs: not a Go testlog (no \"# test log\" header)")
	}
	roots = longestFirst(roots)
	cwd := filepath.Clean(pkgDir)
	files := map[string]bool{}
	env := map[string]bool{}
	var obs Observed
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		op, name, ok := strings.Cut(line, " ")
		if !ok {
			obs.Unknown++
			continue
		}
		switch op {
		case "getenv":
			env[name] = true
		case "chdir":
			cwd = resolve(cwd, name)
		case "open", "stat":
			if rel, ok := relTo(resolve(cwd, name), roots); ok {
				files[rel] = true
			} else {
				obs.Outside++
			}
		default:
			obs.Unknown++
		}
	}
	if err := sc.Err(); err != nil {
		return Observed{}, err
	}
	obs.Files = sortedMapKeys(files)
	obs.Env = sortedMapKeys(env)
	return obs, nil
}

func resolve(cwd, name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(cwd, name)
}

// longestFirst orders roots so a root nested in another (a worktree under
// the checkout that reads its observation) claims its own paths.
func longestFirst(roots []string) []string {
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		out = append(out, filepath.Clean(r))
	}
	sort.SliceStable(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func relTo(p string, roots []string) (string, bool) {
	for _, root := range roots {
		if p == root {
			return ".", true
		}
		if strings.HasPrefix(p, root+string(filepath.Separator)) {
			return filepath.ToSlash(p[len(root)+1:]), true
		}
	}
	return "", false
}

// meta is meta.json as scripts/test-inputs.sh writes it.
type meta struct {
	Commit   string `json:"commit"`
	At       string `json:"at"`
	Root     string `json:"root"`
	Packages map[string]struct {
		Exit    int            `json:"exit"`
		PerTest bool           `json:"per_test"`
		Tests   map[string]int `json:"tests"`
	} `json:"packages"`
}

// Load reads an observation directory: meta.json, then
// pkg/<dir>/_package.log for every package it lists and
// pkg/<dir>/_test.<Name>.log for every test of a per-test package. root is
// the repository the logs are resolved into; the root the run recorded
// (meta.json "root", a scratch worktree) is accepted as another spelling,
// so an observation made in a clone reads as this checkout's paths. A
// package whose log is missing or unreadable is kept, Partial, and never
// fails the map; only an unreadable meta.json does. Every observed path
// under a symlinked directory of root is also recorded under its target, the
// spelling git reports a change by.
func Load(dir, root string) (Map, error) {
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return Map{}, fmt.Errorf("testinputs: %w", err)
	}
	var mt meta
	if err := json.Unmarshal(b, &mt); err != nil {
		return Map{}, fmt.Errorf("testinputs: meta.json: %w", err)
	}
	roots := rootSpellings(root, mt.Root)
	canon := newCanonicalizer(root)
	m := Map{Commit: mt.Commit, At: mt.At, Packages: make(map[string]Package, len(mt.Packages))}
	for _, pkg := range sortedMapKeys(mt.Packages) {
		pm := mt.Packages[pkg]
		base := filepath.Join(dir, "pkg", filepath.FromSlash(pkg))
		runDir := filepath.Join(firstNonEmpty(mt.Root, root), filepath.FromSlash(pkg))
		p := Package{Exit: pm.Exit, PerTest: pm.PerTest, Partial: pm.Exit != 0 && pm.Exit != 1}
		if obs, err := parseFile(filepath.Join(base, "_package.log"), runDir, roots); err != nil {
			p.Partial = true
		} else {
			obs.Files = canon.expand(obs.Files)
			p.Observed = obs
		}
		if pm.PerTest {
			p.Tests = make(map[string]Observed, len(pm.Tests))
			for _, name := range sortedMapKeys(pm.Tests) {
				if pm.Tests[name] != 0 {
					p.Unobserved = append(p.Unobserved, name)
					continue
				}
				tobs, err := parseFile(filepath.Join(base, "_test."+name+".log"), runDir, roots)
				if err != nil {
					p.Unobserved = append(p.Unobserved, name)
					continue
				}
				tobs.Files = canon.expand(tobs.Files)
				p.Tests[name] = tobs
			}
		}
		m.Packages[pkg] = p
	}
	return m, nil
}

func parseFile(name, runDir string, roots []string) (Observed, error) {
	f, err := os.Open(name)
	if err != nil {
		return Observed{}, err
	}
	defer f.Close()
	return ParseLog(f, runDir, roots...)
}

func rootSpellings(roots ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range roots {
		if r == "" {
			continue
		}
		for _, s := range []string{filepath.Clean(r), evalOr(r)} {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

func evalOr(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// canonicalizer maps a repository-relative path through the symlinked
// directories of the reading checkout (`.agents/skills` -> `.claude/skills`),
// one EvalSymlinks per directory.
type canonicalizer struct {
	root     string // the reading checkout, symlinks resolved
	dirCache map[string]string
}

func newCanonicalizer(root string) *canonicalizer {
	return &canonicalizer{root: evalOr(root), dirCache: map[string]string{}}
}

// canonDir is dir's repository-relative target, or dir itself when it does
// not exist, is not under a symlink, or leaves the repository.
func (c *canonicalizer) canonDir(dir string) string {
	if dir == "." || dir == "" {
		return "."
	}
	if v, ok := c.dirCache[dir]; ok {
		return v
	}
	v := dir
	if r, err := filepath.EvalSymlinks(filepath.Join(c.root, filepath.FromSlash(dir))); err == nil {
		if rel, ok := relTo(r, []string{c.root}); ok {
			v = rel
		}
	}
	c.dirCache[dir] = v
	return v
}

// expand returns files plus, for each one under a symlinked directory (or
// a directory that is itself a symlink), its target's spelling. Sorted.
func (c *canonicalizer) expand(files []string) []string {
	set := make(map[string]bool, len(files))
	dirs := make(map[string]bool, len(files))
	for _, f := range files {
		set[f] = true
		dirs[path.Dir(f)] = true
	}
	for _, f := range files {
		d := path.Dir(f)
		if cd := c.canonDir(d); cd != d {
			set[path.Join(cd, path.Base(f))] = true
		}
		// A path that is itself a directory some other observed path sits
		// in (a ReadDir of a symlinked directory) also resolves whole.
		if dirs[f] {
			if cf := c.canonDir(f); cf != f {
				set[cf] = true
			}
		}
	}
	return sortedMapKeys(set)
}

// Readers returns every test or package that read a changed path: the path
// itself opened or stat'ed, or the directory whose listing changed opened.
// A package run test by test answers per test; its unobserved tests answer
// through the package-wide log, and a read the package-wide log saw that no
// test's own log explains (a TestMain read, an unlisted test) selects the
// package whole. A partial package is a whole-package hit for every change.
// Sorted by package, test, file; one hit per (package, test, file).
func (m Map) Readers(changes []Change) []Hit {
	seen := map[Hit]bool{}
	var out []Hit
	add := func(h Hit) {
		k := Hit{Package: h.Package, Test: h.Test, File: h.File}
		if !seen[k] {
			seen[k] = true
			out = append(out, h)
		}
	}
	for _, pkg := range sortedMapKeys(m.Packages) {
		p := m.Packages[pkg]
		for _, c := range changes {
			if p.Partial {
				add(Hit{Package: pkg, File: c.Path, Via: "partial"})
				continue
			}
			via := match(p.Observed, c)
			if !p.PerTest {
				if via != "" {
					add(Hit{Package: pkg, File: c.Path, Via: via})
				}
				continue
			}
			explained := false
			for _, name := range sortedMapKeys(p.Tests) {
				if v := match(p.Tests[name], c); v != "" {
					add(Hit{Package: pkg, Test: name, File: c.Path, Via: v})
					explained = true
				}
			}
			if via == "" {
				continue
			}
			for _, name := range p.Unobserved {
				add(Hit{Package: pkg, Test: name, File: c.Path, Via: "package"})
				explained = true
			}
			if !explained {
				add(Hit{Package: pkg, File: c.Path, Via: "package"})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.Test != b.Test {
			return a.Test < b.Test
		}
		return a.File < b.File
	})
	return out
}

// PartialPackages lists the packages whose observation is not whole, sorted.
func (m Map) PartialPackages() []string {
	var out []string
	for _, pkg := range sortedMapKeys(m.Packages) {
		if m.Packages[pkg].Partial {
			out = append(out, pkg)
		}
	}
	return out
}

func match(o Observed, c Change) string {
	if contains(o.Files, c.Path) {
		return "open"
	}
	if c.Listing {
		dir := c.ListingDir
		if dir == "" {
			dir = path.Dir(c.Path)
		}
		if contains(o.Files, dir) {
			return "dir"
		}
	}
	return ""
}

func contains(sorted []string, s string) bool {
	i := sort.SearchStrings(sorted, s)
	return i < len(sorted) && sorted[i] == s
}

// sortedMapKeys is m's keys, sorted: no map order reaches a result.
func sortedMapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
