package main

// reach.go — the one reach the hooks name tests by (docs/design/readouts.md
// section 20, H4): blast_radius (kind change) over changed files, the call
// the pre-edit blast trailer counts (SR-60, edithook_blast.go). The trailer
// counts its tests, the tests-to-run line (RO-43, edithook_teststorun.go)
// names them, and the readout hook's daemon join (RO-30,
// readouthook_daemonjoin.go) names them after a compile error. File-scoped:
// it answers for the changed files, every symbol in them, not for the edit's
// changed lines alone (RO-M13's reading).

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// blastReach is blast_radius' answer for changed files: the transitive
// symbol count (the trailer's callers) and the index keys of the tests that
// reach them ("svc::dir/pkg.TestX"), sorted and unique.
type blastReach struct {
	Callers int
	Tests   []string
}

// fetchBlastReach asks the daemon at baseURL for the reach of files in
// service, inside ctx. Any failure, a refused file included, is an error:
// the caller names the miss, never an empty reach.
func fetchBlastReach(ctx context.Context, baseURL, service string, files []string) (blastReach, error) {
	var p struct {
		TotalSymbols int      `json:"total_symbols"`
		Tests        []string `json:"tests"`
	}
	if err := callTestRunTool(ctx, baseURL, "blast_radius", map[string]any{"service": service, "changed_files": files}, &p); err != nil {
		return blastReach{}, err
	}
	tests := slices.Clone(p.Tests)
	slices.Sort(tests)
	return blastReach{Callers: p.TotalSymbols, Tests: slices.Compact(tests)}, nil
}

// reachService resolves root to its indexed service inside ctx: the cached
// identity lookup the other hook paths use, with a client sized to what is
// left of ctx rather than its own fixed cap. errNoService names a root the
// daemon indexes no service at; any other error is the daemon's.
func reachService(ctx context.Context, baseURL, root string) (string, error) {
	id, err := reachIdentity(ctx, baseURL, root)
	return id.Service, err
}

func reachIdentity(ctx context.Context, baseURL, root string) (identityResult, error) {
	if root == "" {
		return identityResult{}, errNoService
	}
	if deadlinePassed(ctx) {
		return identityResult{}, context.DeadlineExceeded
	}
	hc := &http.Client{Timeout: 2 * time.Second}
	if dl, ok := ctx.Deadline(); ok {
		hc.Timeout = time.Until(dl)
	}
	id, err := lookupIdentityCached(hc, baseURL, root, ctx)
	switch {
	case errors.Is(err, errIdentityRouteMissing):
		// A daemon that predates GET /identity: the service list.
		name, _, ok := indexedServiceFor(hc, baseURL, root)
		if !ok {
			return identityResult{}, errNoService
		}
		return identityResult{Service: name}, nil
	case err != nil:
		if deadlinePassed(ctx) {
			return identityResult{}, context.DeadlineExceeded
		}
		return identityResult{}, err
	case id.Service == "":
		return identityResult{}, errNoService
	}
	return id, nil
}

// errNoService is a root the daemon indexes no service at.
var errNoService = errors.New("no indexed service")

// testKeyParts splits a test's index key ("svc::dir/pkg.TestX") into its
// repository directory ("dir", "." for the root package) and its name
// ("TestX"); ok is false for a key with no name.
func testKeyParts(key string) (dir, name string, ok bool) {
	k := key
	if i := strings.Index(k, "::"); i >= 0 {
		k = k[i+2:]
	}
	dot := strings.LastIndex(k, ".")
	if dot < 0 || dot == len(k)-1 {
		return "", "", false
	}
	head := k[:dot] // "dir/pkg", or "dir/pkg.Suite" for a suite's method
	name = k[dot+1:]
	dir = "."
	if s := strings.LastIndex(head, "/"); s >= 0 {
		dir = head[:s]
	}
	return dir, name, true
}

// goTestRunnable reports whether `go test -run` runs a test by this name:
// a Test, Example or Fuzz function (a benchmark needs -bench).
func goTestRunnable(name string) bool {
	return strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Example") || strings.HasPrefix(name, "Fuzz")
}

// goTestRecipe is the runner's own filter for the named tests, as text
// fathom never runs: `go test -run '^(A|B)$' ./a ./b`.
func goTestRecipe(dirs, names []string) string {
	pkgs := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d == "." {
			pkgs = append(pkgs, ".")
			continue
		}
		pkgs = append(pkgs, "./"+d)
	}
	return "go test -run '^(" + strings.Join(names, "|") + ")$' " + strings.Join(pkgs, " ")
}

// testsToRunMax is how many tests the line names.
const testsToRunMax = 5

// testsToRunLineBudget caps the line's bytes; a name past it moves into
// `and N more`, never a cut recipe.
const testsToRunLineBudget = 600

// testRef is one reaching test: its package directory and name.
type testRef struct{ dir, name string }

// pickTestsToRun orders the reaching tests' keys, the edited file's own
// package (near) first, then by directory and name, keeps the runnable
// ones once each, and returns at most testsToRunMax of them with how many
// more there are.
func pickTestsToRun(keys []string, near string) (shown []testRef, more int) {
	seen := map[testRef]bool{}
	var all []testRef
	for _, k := range keys {
		dir, name, ok := testKeyParts(k)
		t := testRef{dir, name}
		if !ok || !goTestRunnable(name) || seen[t] {
			continue
		}
		seen[t] = true
		all = append(all, t)
	}
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if (a.dir == near) != (b.dir == near) {
			return a.dir == near
		}
		if a.dir != b.dir {
			return a.dir < b.dir
		}
		return a.name < b.name
	})
	n := min(len(all), testsToRunMax)
	for n > 1 && len(testsToRunText(all[:n], len(all)-n, "")) > testsToRunLineBudget {
		n--
	}
	return all[:n], len(all) - n
}

// testsToRunText renders the line: the named tests, `and N more`, the
// recipe, then ran (the tree clause; "" for none).
func testsToRunText(shown []testRef, more int, ran string) string {
	if len(shown) == 0 {
		return ""
	}
	var names, dirs []string
	seenName, seenDir := map[string]bool{}, map[string]bool{}
	for _, t := range shown {
		if !seenName[t.name] {
			seenName[t.name] = true
			names = append(names, t.name)
		}
		if !seenDir[t.dir] {
			seenDir[t.dir] = true
			dirs = append(dirs, t.dir)
		}
	}
	sort.Strings(dirs)
	listed := make([]string, len(shown))
	for i, t := range shown {
		listed[i] = t.name
	}
	line := "tests to run: " + strings.Join(listed, ", ")
	if more > 0 {
		line += " and " + strconv.Itoa(more) + " more"
	}
	line += "; recipe: " + goTestRecipe(dirs, names)
	if ran != "" {
		line += "; " + ran
	}
	return line
}
