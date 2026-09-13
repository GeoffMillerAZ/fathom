package scenariobench

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// readings_falsedone.go is SB-21 (vision 27 §3, M2): FalseDoneRate matches
// every `[x]` the agent claimed against the INDEPENDENT oracle — the final
// tree (does the row's symbol still exist, via go/parser, exactly the way
// internal/contextbench/publicloc derives gold functions from hunks) and
// the coverage profile from the same test run (does any covered block fall
// inside the symbol's span). The fathom arm's own gate refuses a false
// claim at claim time; this reading is what an oracle fathom does not own
// says afterwards, for both arms alike.
//
// funcLoc/findFuncInTree/coverBlock/coveredInSpan below are shared with
// ContractDrift (readings_resume.go): both are "does this symbol still
// exist and is it still test-reached" checks, one over box checks, one
// over plan rows.

// funcLoc is one function's file (tree-relative) and 1-based line span, as
// found in the FINAL tree.
type funcLoc struct {
	File  string
	Start int
	End   int
}

// skipTreeDir names directories a whole-tree walk (Package == "") must not
// descend into: VCS metadata, vendored/generated trees, and fixture data —
// none of these hold the subject's own declarations.
func skipTreeDir(name string) bool {
	switch name {
	case ".git", "vendor", "node_modules", "testdata", "third_party":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// findFuncInTree finds symbol's declaration in the final tree: when pkgDir
// is non-empty it searches only that directory's own .go files (Go package
// scope is non-recursive); when pkgDir is empty it walks the whole tree,
// skipping skipTreeDir. Unreadable or unparsable files are skipped rather
// than failing the search — this is a best-effort span finder for a bench
// reading, not an audit instrument that must account for every byte.
func findFuncInTree(tree, pkgDir, symbol string) (funcLoc, bool, error) {
	var files []string
	if pkgDir != "" {
		dir := filepath.Join(tree, filepath.FromSlash(pkgDir))
		entries, err := os.ReadDir(dir)
		if err != nil {
			return funcLoc{}, false, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			files = append(files, filepath.Join(dir, e.Name()))
		}
	} else {
		err := filepath.WalkDir(tree, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable entry: skip, keep walking
			}
			if d.IsDir() {
				if path != tree && skipTreeDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return funcLoc{}, false, err
		}
	}

	sort.Strings(files) // determinism: first match in a stable file order
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		spans, err := publicloc.FuncSpans(src, path)
		if err != nil {
			continue
		}
		for _, sp := range spans {
			if sp.Name == symbol {
				rel, err := filepath.Rel(tree, path)
				if err != nil {
					rel = path
				}
				return funcLoc{File: filepath.ToSlash(rel), Start: sp.Start, End: sp.End}, true, nil
			}
		}
	}
	return funcLoc{}, false, nil
}

// coverBlock is one line of a `go test -coverprofile` file:
// "path/to/file.go:12.2,15.3 3 1" → File (compiler-qualified, e.g.
// "module/path/file.go"), the 1-based line span and the hit Count.
type coverBlock struct {
	File               string
	StartLine, EndLine int
	Count              int
}

var coverLineRe = regexp.MustCompile(`^(.+\.go):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$`)

// parseCoverProfile reads a go coverprofile (the "mode: …" header line is
// skipped; malformed lines are skipped rather than failing the read).
func parseCoverProfile(path string) ([]coverBlock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var blocks []coverBlock
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		m := coverLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, err1 := strconv.Atoi(m[2])
		end, err2 := strconv.Atoi(m[3])
		count, err3 := strconv.Atoi(m[4])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		blocks = append(blocks, coverBlock{File: m[1], StartLine: start, EndLine: end, Count: count})
	}
	return blocks, nil
}

// coveredInSpan reports whether any coverage block for relFile (a
// tree-relative path) intersects [start, end] with Count > 0. A
// coverprofile names files by the compiler's import-path-qualified form
// (module path + subdir), so matching is by path suffix rather than
// equality.
func coveredInSpan(blocks []coverBlock, relFile string, start, end int) bool {
	for _, b := range blocks {
		if b.File != relFile && !strings.HasSuffix(b.File, "/"+relFile) {
			continue
		}
		if b.Count > 0 && b.StartLine <= end && start <= b.EndLine {
			return true
		}
	}
	return false
}

// checkLabel names a BoxCheck in FalseDoneReading.RefutedRows: its row id
// when the harness recorded one, else its symbol, else file:turn.
func checkLabel(c BoxCheck) string {
	switch {
	case c.RowID != "":
		return c.RowID
	case c.Symbol != "":
		return c.Symbol
	default:
		return fmt.Sprintf("%s:%d", c.File, c.Turn)
	}
}

// FalseDoneRate is SB-21 (vision 27 §3, M2): for every `[x]` box check, does
// the independent oracle — the final tree plus the coverage profile from
// the SAME test run — back up the claim? A check with no symbol, one whose
// symbol no longer exists in the final tree, or one whose span has no
// covered block is refuted. `not_measurable` when no coverprofile was
// captured: refutation by "no covered block" cannot be judged without one,
// so the reading does not pretend to.
func FalseDoneRate(checks []BoxCheck, tree string, coverprofile string) FalseDoneReading {
	reading := FalseDoneReading{Claimed: len(checks)}
	if coverprofile == "" {
		reading.NotMeasurable = "no coverprofile"
		return reading
	}
	blocks, err := parseCoverProfile(coverprofile)
	if err != nil {
		reading.NotMeasurable = fmt.Sprintf("coverprofile unreadable: %s", err)
		return reading
	}

	var refuted []string
	for _, c := range checks {
		label := checkLabel(c)
		if c.Symbol == "" {
			refuted = append(refuted, label+": no symbol")
			continue
		}
		loc, found, err := findFuncInTree(tree, c.Package, c.Symbol)
		if err != nil || !found {
			refuted = append(refuted, label+": symbol not found in final tree")
			continue
		}
		if !coveredInSpan(blocks, loc.File, loc.Start, loc.End) {
			refuted = append(refuted, label+": no covered block in span")
		}
	}
	sort.Strings(refuted)
	reading.Refuted = len(refuted)
	reading.RefutedRows = refuted
	return reading
}
