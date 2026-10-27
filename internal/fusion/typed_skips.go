package fusion

// typed_skips.go — EW-81 (docs/plans/efficiency-wave.md, O1506): every
// package the typed pass's load returns and does not resolve is skipped for
// one reason from a closed set, counted and named, never silent (I38).
//
// Before this a package with no connector-allowed compiled file went to
// PackagesSkipped, which no surface read: "21/31 pkg(s) processed (0
// failed)" named none of the other ten. Most of them are harmless (the
// synthesized test mains, the plain package of a test-only directory), but
// a build-constrained package and a directory whose only Go file has a name
// Go ignores lose their typed edges, and a package made only of cgo files
// lost them for no reason at all: its compiled files are all generated
// copies in the build cache, so it read as connector-ignored. It is now
// resolved from its source (GoFiles), like a cgo file in a mixed package.
//
// The reasons are classified from go/packages' file lists, never from error
// text, and they never reach the store: TypedEdgeStatus.Reason is persisted,
// so a reason there carries counts only.

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// TypedSkip is one reason the typed pass did not resolve some of its input
// (O1506): how many packages, directories or go.mod files, and the first
// five of them in sorted order.
type TypedSkip struct {
	Reason string
	Count  int
	Names  []string
}

// Why a package the load returned is not resolved (typedSkipReason), or why
// a discovered Go directory is not loaded at all (unloadedGoDirs).
const (
	typedSkipTestMain         = "test main"
	typedSkipTestOnly         = "test-only"
	typedSkipConnectorScope   = "connector scope"
	typedSkipBuildConstraints = "build constraints"
	typedSkipOutside          = "outside ./..."
	typedSkipNoGoMod          = "no go.mod"
)

// typedSkipNamed is how many entries each reason names; its count is always
// exact.
const typedSkipNamed = 5

// TypedSkipLosesEvidence reports whether a reason's input carries source the
// syntax pass indexed and the typed pass never reads: those are named in the
// pass's warning. The others are harmless (a test main, a test-only plain
// package whose files resolve under its test variant) or declared (the
// connector's scope); a nested module not loaded keeps its own line.
func TypedSkipLosesEvidence(reason string) bool {
	switch reason {
	case typedSkipBuildConstraints, typedSkipOutside, typedSkipNoGoMod:
		return true
	}
	return false
}

// typedSkipReason is why pkg is not resolved, from a closed set; "" for a
// package the pass resolves. The rules apply in order and the first match
// wins:
//  1. test main: a synthesized "<pkg>.test" main, not a test variant, with
//     files, none of them inside the repository (a real package in a
//     directory named "x.test" with no file to resolve falls through);
//  2. "": an allowed file among GoFiles (a cgo file's source) or
//     CompiledGoFiles;
//  3. connector scope: it has files, none of them allowed;
//  4. build constraints: no files, but Go files the build excluded (a
//     non-Go file IgnoredFiles also lists is not one);
//  5. test-only: no files of any kind and no load error — the plain package
//     of a directory whose only Go files are _test.go, resolved under its
//     test variant;
//  6. outside ./...: no files of any kind and a load error — the directory's
//     only Go files have names Go ignores (a leading "_" or ".").
//
// A nil allowed set (whole-module scope) skips nothing.
func typedSkipReason(pkg *packages.Package, repoPath string, allowed map[string]bool) string {
	if allowed == nil {
		return ""
	}
	if strings.HasSuffix(pkg.ID, ".test") && !isTestVariant(pkg.ID) && len(pkg.GoFiles) > 0 && !anyFileInRepo(repoPath, pkg.GoFiles) {
		return typedSkipTestMain
	}
	if anyFileAllowed(repoPath, allowed, pkg.GoFiles) || anyFileAllowed(repoPath, allowed, pkg.CompiledGoFiles) {
		return ""
	}
	switch {
	case len(pkg.GoFiles) > 0 || len(pkg.CompiledGoFiles) > 0:
		return typedSkipConnectorScope
	case anyGoFile(pkg.IgnoredFiles):
		return typedSkipBuildConstraints
	case len(pkg.Errors) == 0:
		return typedSkipTestOnly
	}
	return typedSkipOutside
}

// anyFileInRepo reports whether one of files (absolute paths) lies inside
// repoPath.
func anyFileInRepo(repoPath string, files []string) bool {
	for _, f := range files {
		if !outsideRepo(repoRelSlash(repoPath, f)) {
			return true
		}
	}
	return false
}

// anyGoFile reports whether one of files is a Go file.
func anyGoFile(files []string) bool {
	for _, f := range files {
		if strings.HasSuffix(f, ".go") {
			return true
		}
	}
	return false
}

// typedSkipName names a skipped package: its go/packages ID, or for the
// directory form go list gives a package with no Go files ("./us"), that
// directory relative to the repository ("." for the root). A nested
// module's import-path IDs carry its directory ("tools: example.com/x/t"):
// two nested modules may declare the same module path.
func typedSkipName(m goModule, pkgID string) string {
	if pkgID == "." || strings.HasPrefix(pkgID, "./") {
		if dir := path.Join(m.rel, pkgID); dir != "." {
			return "./" + dir
		}
		return "."
	}
	if m.nested() {
		return m.rel + ": " + pkgID
	}
	return pkgID
}

// typedSkipTally collects names by reason; skips renders them.
type typedSkipTally struct {
	names map[string][]string
}

func (t *typedSkipTally) add(reason, name string) {
	if t.names == nil {
		t.names = map[string][]string{}
	}
	t.names[reason] = append(t.names[reason], name)
}

// skips returns one TypedSkip per reason, sorted by reason, each naming its
// first typedSkipNamed names in sorted order (sorted before the cap, so the
// names shown never depend on load order).
func (t *typedSkipTally) skips() []TypedSkip {
	if len(t.names) == 0 {
		return nil
	}
	reasons := make([]string, 0, len(t.names))
	for r := range t.names {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	out := make([]TypedSkip, 0, len(reasons))
	for _, r := range reasons {
		names := append([]string(nil), t.names[r]...)
		sort.Strings(names)
		s := TypedSkip{Reason: r, Count: len(names)}
		if len(names) > typedSkipNamed {
			names = names[:typedSkipNamed]
		}
		s.Names = names
		out = append(out, s)
	}
	return out
}

// FormatTypedSkipNames renders a skip's names with "and N more" when its
// count exceeds them.
func FormatTypedSkipNames(s TypedSkip) string {
	named := strings.Join(s.Names, ", ")
	if more := s.Count - len(s.Names); more > 0 {
		named += fmt.Sprintf(" and %d more", more)
	}
	return named
}

// FormatTypedSkipCounts renders skips as "4 test main, 1 test-only", in
// their (reason) order: counts only, never a name, so it may reach the
// persisted TypedEdgeStatus.Reason.
func FormatTypedSkipCounts(skips []TypedSkip) string {
	parts := make([]string, len(skips))
	for i, s := range skips {
		parts[i] = fmt.Sprintf("%d %s", s.Count, s.Reason)
	}
	return strings.Join(parts, ", ")
}

// typedSkipsLine is the typed pass's warning naming the input of each class
// that loses evidence: the packages skipped for one, then the discovered Go
// directories no load covered; "" when there is none. A nested module not
// loaded is foreignModulesLine's, never repeated here.
func typedSkipsLine(serviceID string, skips, notLoaded []TypedSkip) string {
	var parts []string
	for _, s := range skips {
		if TypedSkipLosesEvidence(s.Reason) {
			parts = append(parts, fmt.Sprintf("%s %d package(s): %s", s.Reason, s.Count, FormatTypedSkipNames(s)))
		}
	}
	for _, s := range notLoaded {
		if TypedSkipLosesEvidence(s.Reason) {
			parts = append(parts, fmt.Sprintf("%s %d directory(ies): %s", s.Reason, s.Count, FormatTypedSkipNames(s)))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("fusion: typed-edge pass %q: Go input the typed pass did not resolve, its edges the syntax pass's: %s\n", serviceID, strings.Join(parts, "; "))
}
