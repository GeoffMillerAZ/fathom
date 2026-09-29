package main

// refactor_status.go — cmdRefactorStatus (CV-34, ADR-044 §4): "fathom
// refactor status --plan FILE" reports each ratchet row's (a `gone:
// pattern` row with `max:` set) current count against its max and against
// the count at the plan's own "Baseline: <sha>" header, plus drift —
// references that landed on the current branch since the baseline (a
// positive current-minus-baseline delta: the ratchet only ever tightens on
// purpose, so more references now than at baseline means something new
// landed, not that the refactor progressed).
//
// Purely git+file based (P19: base facts come from `git show`, never a
// second full ingest) — no store, no daemon, no --service: the SAME
// paths:-scoped lexical scan `fathom refactor plan` seeded max: from
// (countPatternInDir, refactor_plan.go), run twice — once over the CURRENT
// working tree, once over `git show <baseline>:<path>` content for the
// same file set at that commit (git ls-tree, never a re-ingest).
import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// refactorStatusRow is one ratchet row's live reading.
type refactorStatusRow struct {
	ID       string   `json:"id"`
	Pattern  string   `json:"pattern"`
	Paths    []string `json:"paths,omitempty"`
	Max      int      `json:"max"`
	Current  int      `json:"current"`
	Baseline int      `json:"baseline"`
	// Drift is max(0, Current-Baseline): references that landed on the
	// branch since the baseline commit — never negative (progress toward
	// zero is not "negative drift," it is simply not drift).
	Drift int `json:"drift"`
	// OverMax is true when Current exceeds Max — the ratchet itself is
	// violated right now, independent of drift (a max lowered faster than
	// the corpus caught up reads this way too, honestly).
	OverMax bool `json:"over_max"`
}

func cmdRefactorStatus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("refactor status", stderr)
	planPath := fs.String("plan", "", "plan file to report on (required, unless --migration is given)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a text table")
	// CX-13 (design §4, U9): the fleet-wide migration view — a separate
	// mode from the ratchet/drift report above (CV-34/ADR-044 §4's own
	// purely git+file contract is unchanged when --migration is absent).
	migrationText := fs.String("migration", "", "a migration declaration (\"<symbol|import|route|config|pkg> <from> -> <to>\"): report the fleet-wide migration view instead of ratchet status")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db) — --migration mode only")
	servicesFlag := fs.String("services", "", "comma-separated service IDs — --migration mode only (default: every service in the store)")
	conserveSince := fs.String("conserve-since", "", "git ref — --migration mode only (CX-12w): run `fathom conserve --since <ref>` in each repository for the completion verdict; without it no repository is complete (conservation was never checked)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *migrationText != "" {
		return cmdRefactorStatusMigration(*migrationText, *storePath, splitCSV(*servicesFlag), *conserveSince, *jsonOut, stdout, stderr)
	}
	if *planPath == "" {
		fmt.Fprintln(stderr, "error: refactor status requires --plan FILE (or --migration for the fleet-wide migration view)")
		return 2
	}

	body, err := os.ReadFile(*planPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: read %q: %v\n", *planPath, err)
		return 2
	}
	plan := verify.ParseMarkdownPlan("refactor-status", body)
	if plan.Baseline == "" {
		fmt.Fprintf(stderr, "error: %q has no \"Baseline: <sha>\" header — run `fathom refactor plan` first, or add one\n", *planPath)
		return 2
	}

	root := verifyRepoRoot()
	rows := make([]refactorStatusRow, 0)
	for _, t := range plan.Tasks {
		pattern, ok := goneRatchetPattern(t.Gone)
		if !ok || t.Max == nil {
			continue // not a ratchet row (design §4: max: on a gone: row makes it one)
		}
		re, rerr := regexp.Compile("(?m)" + pattern)
		if rerr != nil {
			fmt.Fprintf(stderr, "warning: row %s: pattern %q does not compile: %v\n", t.ID, pattern, rerr)
			continue
		}
		current := countPatternInWorktree(root, t.GonePaths, re)
		baseline, berr := countPatternAtRef(root, plan.Baseline, t.GonePaths, re)
		if berr != nil {
			fmt.Fprintf(stderr, "warning: row %s: baseline count at %s: %v\n", t.ID, plan.Baseline, berr)
			continue
		}
		drift := current - baseline
		if drift < 0 {
			drift = 0
		}
		rows = append(rows, refactorStatusRow{
			ID: t.ID, Pattern: pattern, Paths: t.GonePaths, Max: *t.Max,
			Current: current, Baseline: baseline, Drift: drift, OverMax: current > *t.Max,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

	if *jsonOut {
		b, jerr := json.MarshalIndent(map[string]any{
			"schema":   "fathom.refactor.status/v1",
			"plan":     *planPath,
			"baseline": plan.Baseline,
			"ratchets": rows,
		}, "", "  ")
		if jerr != nil {
			fmt.Fprintf(stderr, "error: encode JSON: %v\n", jerr)
			return 2
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	}

	fmt.Fprintf(stdout, "refactor status: %s (baseline %s)\n", *planPath, plan.Baseline)
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "  no ratchet rows found (a gone: pattern row with max: set)")
	}
	for _, r := range rows {
		mark := "[x]"
		if r.OverMax {
			mark = "[ ]"
		}
		fmt.Fprintf(stdout, "%s %s: %d/%d (baseline %d, drift %d) — %s\n",
			mark, r.ID, r.Current, r.Max, r.Baseline, r.Drift, r.Pattern)
	}
	return 0
}

// goneRatchetPattern extracts a `gone: pattern "<regex>"` row's regex —
// the same shape internal/verify's own (unexported) parseGoneForm reads,
// duplicated here at this small size rather than exporting a one-line
// parser for a single caller outside that package. ok is false for any
// other gone: form (import/module/callers), or an empty/malformed value.
func goneRatchetPattern(raw string) (pattern string, ok bool) {
	raw = strings.TrimSpace(raw)
	rest, cut := strings.CutPrefix(raw, "pattern ")
	if !cut {
		return "", false
	}
	rest = strings.TrimSpace(rest)
	if len(rest) >= 2 && rest[0] == '"' && rest[len(rest)-1] == '"' {
		rest = rest[1 : len(rest)-1]
	}
	if rest == "" {
		return "", false
	}
	return rest, true
}

// countPatternInWorktree sums re's matches across the CURRENT working tree
// content of every path paths names (an exact file, or a "dir/" prefix
// scanned recursively) — empty paths scans the whole root. Matches
// cmd/fathom/refactor_plan.go's countPatternInDir scope exactly, but reads
// live files instead of an already-fetched []core.SourceFile, since this
// command carries no store/client at all.
func countPatternInWorktree(root string, paths []string, re *regexp.Regexp) int {
	scopes := paths
	if len(scopes) == 0 {
		scopes = []string{""}
	}
	total := 0
	seen := map[string]bool{}
	for _, p := range scopes {
		for _, rel := range worktreeFilesUnder(root, p) {
			if seen[rel] {
				continue // two paths: entries covering the same file count it once
			}
			seen[rel] = true
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				continue
			}
			total += len(re.FindAll(b, -1))
		}
	}
	return total
}

// worktreeFilesUnder lists repo-relative paths under root scoped by p (a
// "dir/" prefix, walked recursively skipping the same noise directories
// internal/connector.LocalConnector.Discover skips, or an exact file) — ""
// means the whole root.
func worktreeFilesUnder(root, p string) []string {
	if p == "" {
		var out []string
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if isNoiseDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr == nil {
				out = append(out, filepath.ToSlash(rel))
			}
			return nil
		})
		return out
	}
	dir := strings.TrimSuffix(p, "/")
	full := filepath.Join(root, filepath.FromSlash(dir))
	fi, err := os.Stat(full)
	if err != nil {
		return nil
	}
	if !fi.IsDir() {
		return []string{filepath.ToSlash(dir)} // an exact file path entry
	}
	var out []string
	_ = filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if isNoiseDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func isNoiseDir(name string) bool {
	switch name {
	case "vendor", ".git", "node_modules", "testdata", ".idea", ".vscode", ".claude":
		return true
	}
	return false
}

// countPatternAtRef sums re's matches across ref's own content (git show,
// never a working-tree read) of every path paths names, listed at ref via
// `git ls-tree` — a file that did not exist yet at ref contributes nothing,
// never an error (the ordinary case for a brand-new file the refactor
// itself introduced). Empty paths lists the whole tree at ref.
func countPatternAtRef(root, ref string, paths []string, re *regexp.Regexp) (int, error) {
	scopes := paths
	if len(scopes) == 0 {
		scopes = []string{"."}
	}
	total := 0
	seen := map[string]bool{}
	for _, p := range scopes {
		files, err := lsTreeAt(root, ref, p)
		if err != nil {
			return 0, err
		}
		for _, rel := range files {
			if seen[rel] {
				continue
			}
			seen[rel] = true
			b, ok, gerr := gitShowAt(root, ref, rel)
			if gerr != nil {
				return 0, gerr
			}
			if !ok {
				continue
			}
			total += len(re.FindAll(b, -1))
		}
	}
	return total, nil
}

// lsTreeAt lists ref's own repo-relative file paths under p (an exact
// file, or a "dir/" prefix — "." for the whole tree) via `git ls-tree -r
// --name-only`.
func lsTreeAt(root, ref, p string) ([]string, error) {
	out, err := certify.ExecGit(root, "ls-tree", "-r", "--name-only", ref, "--", p)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if l := strings.TrimSpace(line); l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

// gitShowAt reads path's content at ref via `git show <ref>:<path>`. ok is
// false — never an error — when git ran but the path simply did not exist
// at ref.
func gitShowAt(root, ref, path string) (content []byte, ok bool, err error) {
	out, gerr := certify.ExecGit(root, "show", ref+":"+path)
	if gerr != nil {
		return nil, false, nil //nolint:nilerr — git's own "not found at this ref" and a real failure are indistinguishable from CombinedOutput's exit status alone; the caller only needs "nothing to count," and lsTreeAt already proved the path is real at some ref
	}
	return out, true, nil
}
