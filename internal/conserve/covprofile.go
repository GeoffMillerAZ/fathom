package conserve

// coverage.go — CV-67/CV-68 (docs/design/refactor-conservation.md §3.5,
// CV-M13): loads the Go coverage profile `task tests:attest` now writes
// alongside its envelope, and the `coverage.meta.json` that binds it to a
// commit, tree state, race mode and pass/fail (I44's own "unbound, dirty,
// or from a failing run" bar). Residue (residue.go) is the sole consumer:
// this file does no classification of its own, only parsing plus the
// import-path-to-repo-path mapping CV-68 names.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CoverageSchemaV1 is coverage.meta.json's own schema tag (design §3.5:
// `schema: "fathom.coverage/v1"`).
const CoverageSchemaV1 = "fathom.coverage/v1"

// CoverageMeta mirrors coverage.meta.json 1:1 (design §3.5's "The
// profile"): the commit/tree/race facts `task tests:attest` binds its
// coverage profile to, plus whether that same run passed. The zero value
// (Commit == "") means "no meta was ever read" — Residue treats it
// exactly like any other reason a profile is unusable (CV-70).
type CoverageMeta struct {
	Schema  string `json:"schema"`
	Commit  string `json:"commit"`
	Dirty   bool   `json:"dirty"`
	Race    bool   `json:"race"`
	Profile string `json:"profile"`
	Passed  bool   `json:"passed"`
}

// LoadCoverageMeta reads and decodes a coverage.meta.json file at path.
func LoadCoverageMeta(path string) (CoverageMeta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return CoverageMeta{}, fmt.Errorf("reading %s: %w", path, err)
	}
	var m CoverageMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return CoverageMeta{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return m, nil
}

// CoverageBlock is one basic block from a Go coverage profile (`go tool
// cover`'s text format), already mapped onto a repo-relative file:
// StartLine/StartCol..EndLine/EndCol is the block's own source span,
// NumStmt the statement count `go test -cover` recorded for it, and Count
// how many times ANY test binary in the run executed it — already
// merged, by maximum, across every per-package section a `-coverpkg=./...`
// profile duplicates the same block into (LoadCoverage's own doc
// comment).
type CoverageBlock struct {
	StartLine, StartCol int
	EndLine, EndCol     int
	NumStmt, Count      int
}

// CoverageProfile is a parsed, repo-path-mapped Go coverage profile
// (design §3.5, CV-68). Blocks is keyed by repo-relative file path; each
// file's own blocks are sorted by (StartLine, StartCol).
type CoverageProfile struct {
	Mode   string
	Blocks map[string][]CoverageBlock
}

// coverProfileLineRe matches one `go tool cover` profile data line:
// "<file>:<startLine>.<startCol>,<endLine>.<endCol> <numStmt> <count>".
var coverProfileLineRe = regexp.MustCompile(`^(.+):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$`)

// LoadCoverage reads a Go coverage profile at profilePath (`go tool
// cover`'s text format: a leading "mode: <mode>" line, then one data line
// per block) and maps each block's import-path file key onto a
// repo-relative path under root, via the `module` line of the NEAREST
// go.mod above that file among every go.mod LoadCoverage finds under root
// (a repo may hold several — CV-68's own instruction; a nested module's
// own files are matched against ITS module path, never the root module's,
// by picking the longest matching module-path prefix). A block whose file
// key matches no known module's prefix is skipped — never guessed —
// and named in the returned skipped slice, ChangedFunctions' own
// missing/unanalyzed convention (residue.go) for "fathom could not place
// this"). Duplicate blocks (the identical file:line.col span reported by
// more than one per-package section, the ordinary shape of a
// `-coverpkg=./...` profile run across many test binaries) are merged by
// MAXIMUM count: the block ran if ANY test binary's own run of it shows a
// nonzero count.
func LoadCoverage(ctx context.Context, root, profilePath string) (CoverageProfile, []string, error) {
	f, err := os.Open(profilePath)
	if err != nil {
		return CoverageProfile{}, nil, fmt.Errorf("opening %s: %w", profilePath, err)
	}
	defer f.Close()

	modules, err := findModules(ctx, root)
	if err != nil {
		return CoverageProfile{}, nil, fmt.Errorf("scanning %s for go.mod files: %w", root, err)
	}

	prof := CoverageProfile{Blocks: map[string][]CoverageBlock{}}
	// blockIndex maps "repoPath:startLine.startCol,endLine.endCol" to its
	// slot in prof.Blocks[repoPath], for the max-merge across duplicate
	// sections.
	blockIndex := map[string]int{}
	skippedSet := map[string]bool{}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	first := true
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return CoverageProfile{}, nil, err
		}
		line := scanner.Text()
		if first {
			first = false
			if rest, ok := strings.CutPrefix(line, "mode: "); ok {
				prof.Mode = strings.TrimSpace(rest)
				continue
			}
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := coverProfileLineRe.FindStringSubmatch(line)
		if m == nil {
			continue // not a data line this parser understands; never fatal
		}
		fileKey := m[1]
		repoPath, ok := mapImportPathFile(fileKey, modules)
		if !ok {
			skippedSet[fileKey] = true
			continue
		}
		startLine, _ := strconv.Atoi(m[2])
		startCol, _ := strconv.Atoi(m[3])
		endLine, _ := strconv.Atoi(m[4])
		endCol, _ := strconv.Atoi(m[5])
		numStmt, _ := strconv.Atoi(m[6])
		count, _ := strconv.Atoi(m[7])

		idxKey := fmt.Sprintf("%s:%d.%d,%d.%d", repoPath, startLine, startCol, endLine, endCol)
		if i, dup := blockIndex[idxKey]; dup {
			if count > prof.Blocks[repoPath][i].Count {
				prof.Blocks[repoPath][i].Count = count
			}
			continue
		}
		blockIndex[idxKey] = len(prof.Blocks[repoPath])
		prof.Blocks[repoPath] = append(prof.Blocks[repoPath], CoverageBlock{
			StartLine: startLine, StartCol: startCol,
			EndLine: endLine, EndCol: endCol,
			NumStmt: numStmt, Count: count,
		})
	}
	if err := scanner.Err(); err != nil {
		return CoverageProfile{}, nil, fmt.Errorf("reading %s: %w", profilePath, err)
	}

	for file := range prof.Blocks {
		sort.Slice(prof.Blocks[file], func(i, j int) bool {
			bi, bj := prof.Blocks[file][i], prof.Blocks[file][j]
			if bi.StartLine != bj.StartLine {
				return bi.StartLine < bj.StartLine
			}
			return bi.StartCol < bj.StartCol
		})
	}

	skipped := make([]string, 0, len(skippedSet))
	for s := range skippedSet {
		skipped = append(skipped, s)
	}
	sort.Strings(skipped)
	return prof, skipped, nil
}

// goModule is one go.mod LoadCoverage found under root: its declared
// module path, and the repo-relative directory it lives in ("" for
// root's own go.mod at the corpus root).
type goModule struct {
	modulePath string
	dir        string
}

// findModules walks root for every go.mod file and returns each one's
// declared module path plus its own repo-relative directory —
// mapImportPathFile's raw material. Skipped, never descended into:
// .git, vendor/ and node_modules/ (never declare a coverable module of
// their own), plus every directory the `go` tool itself ignores for
// package discovery — any name starting with "." or "_", and any
// directory literally named "testdata" (`go help packages`: "Directory
// and file names that begin with '.' or '_' are ignored ... as are
// directories named 'testdata'"). That excludes fixture/scratch modules
// under testdata/ or an _-prefixed directory that `go test ./...` never
// builds, and — since a dot-prefixed directory is skipped AT ANY DEPTH,
// not just at root — a nested worktree or agent scratch clone living
// under a dot-directory, whose own go.mod may declare the SAME module
// path as the real one (a full clone, never a legitimate nested
// submodule) and would otherwise silently claim every block belonging to
// the real files.
//
// Sorted longest modulePath first, so a longer (more specific) module
// path — a genuine nested submodule extending the outer one — always
// wins the PREFIX match over a shorter one nested inside it. Modules
// tied on module-path length (most often two go.mod files declaring the
// IDENTICAL path — an ordinary nested worktree or full-repo clone
// outside a dot/underscore directory, e.g. under a project's own scratch
// convention) are broken by directory depth: the SHALLOWER one — nearer
// root, design §3.5's own "the module path of the nearest go.mod" — wins
// over a deeper, nested duplicate, and a final lexical tie-break on the
// directory keeps the order fully deterministic (never an unstable
// sort's arbitrary pick).
func findModules(ctx context.Context, root string) ([]goModule, error) {
	var mods []goModule
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			switch {
			case name == ".git" || name == "vendor" || name == "node_modules":
				return filepath.SkipDir
			case p != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata"):
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		relFile, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		b, ok, rerr := readWorktreeFile(ctx, root, relFile)
		if rerr != nil || !ok {
			return nil // an unreadable go.mod names no module; never fatal to the whole scan
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		mp, ok := moduleDeclFrom(b)
		if !ok {
			return nil
		}
		rel, rerr := filepath.Rel(root, filepath.Dir(p))
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		mods = append(mods, goModule{modulePath: mp, dir: rel})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(mods, func(i, j int) bool {
		if len(mods[i].modulePath) != len(mods[j].modulePath) {
			return len(mods[i].modulePath) > len(mods[j].modulePath)
		}
		if len(mods[i].dir) != len(mods[j].dir) {
			return len(mods[i].dir) < len(mods[j].dir)
		}
		return mods[i].dir < mods[j].dir
	})
	return mods, nil
}

// moduleDeclFrom extracts the `module <path>` directive from a go.mod
// file's content — a plain line scan (this package's only need is the
// module path; no reason to take a golang.org/x/mod/modfile dependency
// for one line). Strips a trailing `//` line comment and surrounding
// double quotes, both valid go.mod syntax (`go help modfile`) that `go
// list -m` itself renders as the bare path: without this, "module
// example.com/x // note" or `module "example.com/x"` extracts a path no
// profile key ever matches, so every one of that module's blocks lands
// in LoadCoverage's skipped list instead — a real go.mod silently
// produces zero mapped blocks and, since loadConserveCoverage's caller
// still reports a non-nil (only empty) profile, no coverage_unusable
// reason either.
func moduleDeclFrom(content []byte) (string, bool) {
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "module ")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		if i := strings.Index(rest, "//"); i >= 0 {
			rest = strings.TrimSpace(rest[:i])
		}
		rest = strings.Trim(rest, `"`)
		return rest, true
	}
	return "", false
}

// mapImportPathFile maps a coverage profile's file key (an import path,
// e.g. "github.com/geoffmilleraz/fathom/internal/mcp/mcp.go") to a
// repo-relative path (e.g. "internal/mcp/mcp.go") via the longest
// matching module path among modules (already sorted longest-first by
// findModules): a module whose path is a prefix of fileKey, followed
// immediately by "/", claims it. false when no module's path prefixes
// fileKey at all.
func mapImportPathFile(fileKey string, modules []goModule) (string, bool) {
	for _, m := range modules {
		rest, ok := strings.CutPrefix(fileKey, m.modulePath+"/")
		if !ok {
			continue
		}
		if m.dir == "" {
			return rest, true
		}
		return path.Join(m.dir, rest), true
	}
	return "", false
}
