// Command constitution-lint mechanically enforces agentic-kit's constitutional
// promises. Each check is independently reportable; the binary exits 1 if any
// fail, 0 if all pass. A -v flag lists passing checks too.
//
// Checks:
//
//  1. empty-require: the root go.mod has no require block.
//  2. tracking-coverage: every foundations/<cat>/<pkg> and kits/<pkg> dir is
//     named somewhere in TRACKING.md.
//  3. concurrency-contract: every library package has a package doc comment
//     mentioning "concurrent" or "concurrency".
//  4. schema-presence: every foundation sub-package has schema.cue or is waived
//     in cmd/constitution-lint/waivers.txt.
//  5. cassette-version: agent-cli cassette entries all have hash_version=2.
//  6. catalog-fresh: docs/catalog.md regenerates identically to what is on disk.
//  7. compat-coverage: every foundations/kits/runtimes/surfaces/toolchains
//     package dir is classified (stable/experimental/internal-ish) in
//     docs/compat.md.
//  8. doc-coverage: every exported package-level identifier in the same
//     package universe has a doc comment starting with its own name, or the
//     package is waived in cmd/constitution-lint/doc-waivers.txt.
//  9. org-agnostic: the on-disk tree (source + docs; git is not consulted)
//     contains no whole-word matches for org-specific tokens from
//     cmd/constitution-lint/org-denylist.txt (waivable in
//     cmd/constitution-lint/org-agnostic-waivers.txt), and no hardcoded
//     model-ID literals outside testdata/, cassettes, and provider-adapter
//     default packages.
//  10. secret-scan: the on-disk tree (excluding .git/, workspace/, tmp/,
//     and _test.go files; git is not consulted) contains no matches for the
//     well-known secret shapes in cmd/constitution-lint/secret-patterns.txt.
//  11. readme-count: README.md's "**N Go packages" sentence matches the
//     source-counted catalog total, so the README count cannot silently
//     drift the way the old hand-maintained "148" did.
//  12. read-coverage: every directory enumeration and file read the checks
//     above attempted is recorded in a read manifest
//     (foundations/telemetry/read-manifest); a path that exists but could
//     not be read is one FAIL finding each, so an unreadable tree can never
//     pass, and the summary line renders the finding count as a lower
//     bound ("≥ N (k unreadable: …)") whenever the manifest is incomplete.
//     Absent paths and deliberate skips are facts, not blindness.
//
// Usage:
//
//	go run ./cmd/constitution-lint [-v]
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/agentic-kit/cmd/internal/catalog"
	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
)

// Finding is one lint observation.
type Finding struct {
	Check   string
	Pass    bool
	Message string
}

// run is the testable entry point. root is the module root; args are the
// additional CLI flags (without the program name).
func run(root string, out io.Writer, args []string) int {
	fs2 := flag.NewFlagSet("constitution-lint", flag.ContinueOnError)
	fs2.SetOutput(out)
	verbose := fs2.Bool("v", false, "list passing checks too")
	if err := fs2.Parse(args); err != nil {
		return 2
	}
	reads = readmanifest.NewSet()
	reads.RelativeTo(root)

	checks := []struct {
		name string
		fn   func(string) []Finding
	}{
		{"empty-require", checkEmptyRequire},
		{"tracking-coverage", checkTrackingCoverage},
		{"concurrency-contract", checkConcurrencyContract},
		{"schema-presence", checkSchemaPresence},
		{"cassette-version", checkCassetteVersion},
		{"catalog-fresh", checkCatalogFresh},
		{"compat-coverage", checkCompatCoverage},
		{"doc-coverage", checkDocCoverage},
		{"org-agnostic", checkOrgAgnostic},
		{"secret-scan", checkSecretScan},
		{"readme-count", checkReadmeCount},
	}

	var allFindings []Finding
	for _, c := range checks {
		findings := c.fn(root)
		allFindings = append(allFindings, findings...)
	}
	// read-coverage runs last by construction: it reports on the reads the
	// checks above attempted, so it cannot sit in the table with them.
	manifest := reads.Manifest()
	allFindings = append(allFindings, checkReadCoverage(manifest)...)

	failed := 0
	for _, f := range allFindings {
		if !f.Pass {
			failed++
			fmt.Fprintf(out, "FAIL [%s] %s\n", f.Check, f.Message)
		} else if *verbose {
			fmt.Fprintf(out, "ok   [%s] %s\n", f.Check, f.Message)
		}
	}

	// The finding count is a lower bound whenever the manifest is
	// incomplete: a package in an unreadable directory produced no findings
	// at all, so "N findings" would overstate what was checked.
	total := manifest.LowerBound(len(allFindings))
	if failed == 0 {
		fmt.Fprintf(out, "constitution-lint: all checks passed (%s findings checked)\n", total)
		return 0
	}
	fmt.Fprintf(out, "constitution-lint: %d finding(s) failed; findings checked: %s\n", failed, total)
	return 1
}

// ── 12. read-coverage ────────────────────────────────────────────────────────

// reads is the read manifest for the current run. Every directory
// enumeration and file read the checks attempt records its outcome here —
// first outcome wins, so an unreadable path can never be quietly skipped
// by a later successful read. run installs a fresh Set rooted at the
// module root; the check functions (which take only root, and are called
// directly by tests) record into whichever Set is current. A *Set is safe
// for concurrent use; the reassignment in run is not, which is fine
// because run is the binary's single entry point and the tests are
// sequential.
var reads = readmanifest.NewSet()

// recordFailure files a failed directory enumeration or file read for
// path in the run's manifest: a path that genuinely is not there is a fact
// (absent); anything else is blindness (unreadable, with the operating
// system's own wording). Every continue-on-error site in this file calls
// it before continuing, so the skip stays — the existing checks keep their
// semantics — but the manifest knows, and read-coverage fails the run.
func recordFailure(path string, err error) {
	if errors.Is(err, fs.ErrNotExist) {
		reads.Record(path, readmanifest.Absent())
		return
	}
	reads.Record(path, readmanifest.Unreadable(err))
}

// checkReadCoverage turns the run's manifest into findings: one FAIL per
// path that exists but could not be read (an unreadable file or directory
// is an instrument failure, never a finding about the tree, and every
// count the other checks printed is a lower bound), or a single Pass when
// every considered path was read, absent, or deliberately skipped.
func checkReadCoverage(m readmanifest.Manifest) []Finding {
	bad := m.Unreadable()
	if len(bad) == 0 {
		return []Finding{{
			Check:   "read-coverage",
			Pass:    true,
			Message: fmt.Sprintf("every considered path was read, absent, or deliberately skipped (%d recorded)", len(m.Entries)),
		}}
	}
	findings := make([]Finding, 0, len(bad))
	for _, e := range bad {
		findings = append(findings, Finding{
			Check:   "read-coverage",
			Message: fmt.Sprintf("%s: %s — the tree could not be fully read, so every count above is a lower bound", e.Path, e.Outcome.Detail),
		})
	}
	return findings
}

// ── 1. empty-require ────────────────────────────────────────────────────────

// checkEmptyRequire parses the root go.mod textually (no external deps) and
// reports a failure if it finds a non-empty require block.
func checkEmptyRequire(root string) []Finding {
	gomod := filepath.Join(root, "go.mod")
	data, err := os.ReadFile(gomod)
	if err != nil {
		return []Finding{{
			Check:   "empty-require",
			Message: fmt.Sprintf("cannot read go.mod: %v", err),
		}}
	}

	inRequire := false
	hasRequireEntries := false

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// skip comments
		if strings.HasPrefix(line, "//") {
			continue
		}
		if !inRequire {
			if line == "require (" {
				inRequire = true
			} else if strings.HasPrefix(line, "require ") && !strings.HasSuffix(line, "require ()") {
				// inline require: require foo/bar v1.2.3
				trimmed := strings.TrimPrefix(line, "require ")
				trimmed = strings.TrimSpace(trimmed)
				if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
					hasRequireEntries = true
				}
			}
		} else {
			if line == ")" {
				inRequire = false
				continue
			}
			if line != "" && !strings.HasPrefix(line, "//") {
				hasRequireEntries = true
			}
		}
	}

	if hasRequireEntries {
		return []Finding{{
			Check:   "empty-require",
			Message: "go.mod has a require block — zero-deps constitution violated; move dependency-bearing sub-modules to nested go.mod files (see D1)",
		}}
	}
	return []Finding{{
		Check:   "empty-require",
		Pass:    true,
		Message: "go.mod has no require block",
	}}
}

// ── 2. tracking-coverage ────────────────────────────────────────────────────

// checkTrackingCoverage verifies that every package directory under
// foundations/ (two levels) and kits/ is mentioned in TRACKING.md.
func checkTrackingCoverage(root string) []Finding {
	trackingPath := filepath.Join(root, "TRACKING.md")
	trackingData, err := os.ReadFile(trackingPath)
	if err != nil {
		return []Finding{{
			Check:   "tracking-coverage",
			Message: fmt.Sprintf("cannot read TRACKING.md: %v", err),
		}}
	}
	tracking := string(trackingData)

	dirs, err := libraryPkgDirs(root)
	if err != nil {
		return []Finding{{
			Check:   "tracking-coverage",
			Message: fmt.Sprintf("cannot enumerate package dirs: %v", err),
		}}
	}

	var findings []Finding
	for _, rel := range dirs {
		// TRACKING.md uses backtick-quoted names like `agent-harness` or
		// foundations paths like `kits/agent-harness`; check both the full
		// relative path and just the leaf directory name.
		leaf := filepath.Base(rel)
		if strings.Contains(tracking, rel) || strings.Contains(tracking, "`"+leaf+"`") {
			findings = append(findings, Finding{
				Check:   "tracking-coverage",
				Pass:    true,
				Message: fmt.Sprintf("%s is named in TRACKING.md", rel),
			})
		} else {
			findings = append(findings, Finding{
				Check:   "tracking-coverage",
				Message: fmt.Sprintf("%s is not named in TRACKING.md", rel),
			})
		}
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{
			Check:   "tracking-coverage",
			Pass:    true,
			Message: "no trackable package dirs found",
		})
	}
	return findings
}

// ── 3. concurrency-contract ─────────────────────────────────────────────────

// checkConcurrencyContract verifies that every library package doc comment
// contains "concurrent" or "concurrency" (case-insensitive).
func checkConcurrencyContract(root string) []Finding {
	dirs, err := libraryPkgDirs(root)
	if err != nil {
		return []Finding{{
			Check:   "concurrency-contract",
			Message: fmt.Sprintf("cannot enumerate package dirs: %v", err),
		}}
	}

	var findings []Finding
	fset := token.NewFileSet()
	for _, rel := range dirs {
		absDir := filepath.Join(root, rel)
		synopsis, found := packageDocComment(fset, absDir)
		if !found {
			findings = append(findings, Finding{
				Check:   "concurrency-contract",
				Message: fmt.Sprintf("%s: no package doc comment found", rel),
			})
			continue
		}
		lower := strings.ToLower(synopsis)
		if strings.Contains(lower, "concurrent") || strings.Contains(lower, "concurrency") {
			findings = append(findings, Finding{
				Check:   "concurrency-contract",
				Pass:    true,
				Message: fmt.Sprintf("%s: concurrency contract present", rel),
			})
		} else {
			findings = append(findings, Finding{
				Check:   "concurrency-contract",
				Message: fmt.Sprintf("%s: package doc comment does not mention \"concurrent\" or \"concurrency\"", rel),
			})
		}
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{
			Check:   "concurrency-contract",
			Pass:    true,
			Message: "no library package dirs found",
		})
	}
	return findings
}

// packageDocComment returns the full text of the package doc comment for the
// first non-test, non-main package file found in dir (prefers doc.go).
func packageDocComment(fset *token.FileSet, dir string) (text string, found bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		recordFailure(dir, err)
		return "", false
	}

	var goFiles []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		goFiles = append(goFiles, e.Name())
	}
	// prefer doc.go
	sort.Slice(goFiles, func(i, j int) bool {
		if goFiles[i] == "doc.go" {
			return true
		}
		if goFiles[j] == "doc.go" {
			return false
		}
		return goFiles[i] < goFiles[j]
	})

	for _, name := range goFiles {
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.ImportsOnly)
		if err != nil {
			// ParseFile reads the file itself: a *fs.PathError is a failed
			// read (blindness); anything else is a syntax error, which is
			// a fact about the file that vet and fmt-check report.
			var pe *fs.PathError
			if errors.As(err, &pe) {
				recordFailure(path, err)
			} else {
				reads.Record(path, readmanifest.Skipped("parse-error"))
			}
			continue
		}
		if f.Doc == nil {
			continue
		}
		pkgName := f.Name.Name
		if strings.HasSuffix(pkgName, "_test") {
			continue
		}
		return f.Doc.Text(), true
	}
	return "", false
}

// ── 4. schema-presence ──────────────────────────────────────────────────────

// checkSchemaPresence verifies that every foundation sub-package has a
// schema.cue file or is waived in cmd/constitution-lint/waivers.txt.
func checkSchemaPresence(root string) []Finding {
	waivers, err := loadWaivers(filepath.Join(root, "cmd", "constitution-lint", "waivers.txt"))
	if err != nil {
		return []Finding{{
			Check:   "schema-presence",
			Message: fmt.Sprintf("cannot read waivers.txt: %v", err),
		}}
	}

	foundationRoot := filepath.Join(root, "foundations")
	cats, err := os.ReadDir(foundationRoot)
	if err != nil {
		return []Finding{{
			Check:   "schema-presence",
			Message: fmt.Sprintf("cannot read foundations/: %v", err),
		}}
	}

	var findings []Finding
	for _, cat := range cats {
		if !cat.IsDir() || skipDirName(cat.Name()) {
			continue
		}
		catPath := filepath.Join(foundationRoot, cat.Name())
		pkgs, err := os.ReadDir(catPath)
		if err != nil {
			recordFailure(catPath, err)
			continue
		}
		for _, pkg := range pkgs {
			if !pkg.IsDir() || skipDirName(pkg.Name()) {
				continue
			}
			rel := "foundations/" + cat.Name() + "/" + pkg.Name()
			absDir := filepath.Join(root, rel)
			if !hasGoFiles(absDir) {
				continue
			}

			if waivers[rel] {
				findings = append(findings, Finding{
					Check:   "schema-presence",
					Pass:    true,
					Message: fmt.Sprintf("%s: waived (behavioral-only package)", rel),
				})
				continue
			}

			if fileExists(filepath.Join(absDir, "schema.cue")) {
				findings = append(findings, Finding{
					Check:   "schema-presence",
					Pass:    true,
					Message: fmt.Sprintf("%s: schema.cue present", rel),
				})
			} else {
				findings = append(findings, Finding{
					Check:   "schema-presence",
					Message: fmt.Sprintf("%s: missing schema.cue (add it or waive in cmd/constitution-lint/waivers.txt)", rel),
				})
			}
		}
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{
			Check:   "schema-presence",
			Pass:    true,
			Message: "no foundation sub-packages found",
		})
	}
	return findings
}

// loadWaivers reads waivers.txt and returns a set of waived relative paths.
// Lines beginning with # are comments; blank lines are ignored.
func loadWaivers(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	waivers := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		waivers[line] = true
	}
	return waivers, scanner.Err()
}

// ── 5. cassette-version ─────────────────────────────────────────────────────

// checkCassetteVersion finds agent-cli cassette files (testdata/*.jsonl) and
// verifies every entry's hash_version field equals 2. Missing hash_version
// is treated as v1 (fail).
func checkCassetteVersion(root string) []Finding {
	cassetteDir := filepath.Join(root, "examples", "agent-cli", "testdata")
	entries, err := os.ReadDir(cassetteDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				Check:   "cassette-version",
				Message: "examples/agent-cli/testdata/ not found — no cassette to check",
			}}
		}
		return []Finding{{
			Check:   "cassette-version",
			Message: fmt.Sprintf("cannot read cassette dir: %v", err),
		}}
	}

	var cassettes []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			cassettes = append(cassettes, filepath.Join(cassetteDir, e.Name()))
		}
	}
	if len(cassettes) == 0 {
		return []Finding{{
			Check:   "cassette-version",
			Message: "no *.jsonl cassette files found in examples/agent-cli/testdata/",
		}}
	}

	var findings []Finding
	for _, cPath := range cassettes {
		rel := "examples/agent-cli/testdata/" + filepath.Base(cPath)
		f, err := os.Open(cPath)
		if err != nil {
			findings = append(findings, Finding{
				Check:   "cassette-version",
				Message: fmt.Sprintf("%s: cannot open: %v", rel, err),
			})
			continue
		}
		lineFindings := checkCassetteFile(rel, f)
		f.Close()
		findings = append(findings, lineFindings...)
	}
	return findings
}

// checkCassetteFile scans a single JSONL cassette file and returns findings.
func checkCassetteFile(rel string, r io.Reader) []Finding {
	var findings []Finding
	scanner := bufio.NewScanner(r)
	// Cassette entries can be large (full request/response JSON).
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var entry struct {
			HashVersion *int `json:"hash_version"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			findings = append(findings, Finding{
				Check:   "cassette-version",
				Message: fmt.Sprintf("%s line %d: cannot parse JSON: %v", rel, lineNum, err),
			})
			continue
		}

		if entry.HashVersion == nil {
			findings = append(findings, Finding{
				Check:   "cassette-version",
				Message: fmt.Sprintf("%s line %d: cassette is hash v1 (hash_version absent) — re-record or migrate to v2", rel, lineNum),
			})
		} else if *entry.HashVersion != 2 {
			findings = append(findings, Finding{
				Check:   "cassette-version",
				Message: fmt.Sprintf("%s line %d: hash_version=%d, want 2 — re-record or migrate", rel, lineNum, *entry.HashVersion),
			})
		} else {
			findings = append(findings, Finding{
				Check:   "cassette-version",
				Pass:    true,
				Message: fmt.Sprintf("%s line %d: hash_version=2 ok", rel, lineNum),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		findings = append(findings, Finding{
			Check:   "cassette-version",
			Message: fmt.Sprintf("%s: read error: %v", rel, err),
		})
	}
	if lineNum == 0 {
		findings = append(findings, Finding{
			Check:   "cassette-version",
			Message: fmt.Sprintf("%s: empty cassette file", rel),
		})
	}
	return findings
}

// ── 6. catalog-fresh ────────────────────────────────────────────────────────

// checkCatalogFresh regenerates docs/catalog.md in memory and diffs it
// against the committed file. Uses cmd/internal/catalog so the logic is
// shared with cmd/kit-catalog and cannot drift.
func checkCatalogFresh(root string) []Finding {
	catalogPath := filepath.Join(root, "docs", "catalog.md")
	committed, err := os.ReadFile(catalogPath)
	if err != nil {
		return []Finding{{
			Check:   "catalog-fresh",
			Message: fmt.Sprintf("cannot read docs/catalog.md: %v", err),
		}}
	}

	entries, err := catalog.Collect(root)
	if err != nil {
		return []Finding{{
			Check:   "catalog-fresh",
			Message: fmt.Sprintf("catalog generation error: %v", err),
		}}
	}
	generated := catalog.Render(entries)

	if generated == string(committed) {
		return []Finding{{
			Check:   "catalog-fresh",
			Pass:    true,
			Message: fmt.Sprintf("docs/catalog.md is up to date (%d packages)", len(entries)),
		}}
	}
	return []Finding{{
		Check:   "catalog-fresh",
		Message: "docs/catalog.md is stale — regenerate with: go run ./cmd/kit-catalog",
	}}
}

// ── 7. compat-coverage ──────────────────────────────────────────────────────

// validCompatClassifications are the only classification tokens
// docs/compat.md rows may use.
var validCompatClassifications = map[string]bool{
	"stable":       true,
	"experimental": true,
	"internal-ish": true,
}

// checkCompatCoverage verifies that every package directory under
// foundations/, kits/, runtimes/, surfaces/, and toolchains/ (the same
// universe as doc-coverage, via scopedPkgDirs) is classified as a row in
// docs/compat.md: `| `<module-relative-path>` | <classification> | <rationale> |`
// where classification is exactly one of stable, experimental, internal-ish.
//
// Coverage and row validity are checked separately: a row present for a
// package but carrying an invalid (e.g. misspelled) classification token
// both fails that package's coverage finding AND produces its own
// typo-protection finding, so a bad token can never quietly pass as
// "classified."
func checkCompatCoverage(root string) []Finding {
	compatPath := filepath.Join(root, "docs", "compat.md")
	data, err := os.ReadFile(compatPath)
	if err != nil {
		return []Finding{{
			Check:   "compat-coverage",
			Message: fmt.Sprintf("cannot read docs/compat.md: %v", err),
		}}
	}

	rows := parseCompatRows(string(data))
	validByPath := map[string]string{}
	var findings []Finding
	for _, r := range rows {
		if !validCompatClassifications[r.class] {
			findings = append(findings, Finding{
				Check:   "compat-coverage",
				Message: fmt.Sprintf("docs/compat.md: `%s` has invalid classification %q (want stable, experimental, or internal-ish)", r.path, r.class),
			})
			continue
		}
		validByPath[r.path] = r.class
	}

	dirs, err := scopedPkgDirs(root)
	if err != nil {
		findings = append(findings, Finding{
			Check:   "compat-coverage",
			Message: fmt.Sprintf("cannot enumerate package dirs: %v", err),
		})
		return findings
	}

	for _, rel := range dirs {
		if class, ok := validByPath[rel]; ok {
			findings = append(findings, Finding{
				Check:   "compat-coverage",
				Pass:    true,
				Message: fmt.Sprintf("%s: classified as %s in docs/compat.md", rel, class),
			})
		} else {
			findings = append(findings, Finding{
				Check:   "compat-coverage",
				Message: fmt.Sprintf("%s: not classified in docs/compat.md (add a row with stable/experimental/internal-ish)", rel),
			})
		}
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{
			Check:   "compat-coverage",
			Pass:    true,
			Message: "no package dirs found",
		})
	}
	return findings
}

// compatRow is one classified package row parsed from docs/compat.md.
type compatRow struct {
	path  string
	class string
}

// parseCompatRows extracts classification rows from docs/compat.md with a
// simple line-based scan: a line whose trimmed text starts with "| `"
// contributes a row whose path is the backtick-quoted first cell and whose
// classification is the next `|`-delimited cell — UNLESS that backtick-quoted
// cell contains no "/", in which case the row is skipped as not a package
// path. This matters in practice: docs/compat.md also carries a "Classification
// counts" sign-off summary table shaped `| `stable` | 121 |` — same row
// shape, bare classification WORD instead of a path — which would otherwise
// be misread as a package row with an invalid classification token ("121").
// Every real module-relative package path contains at least one "/" (every
// entry is "<one of the five root prefixes>/<something>"), so requiring one
// cleanly distinguishes the two tables without needing to track table/section
// boundaries.
func parseCompatRows(md string) []compatRow {
	var rows []compatRow
	scanner := bufio.NewScanner(strings.NewReader(md))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		rest := line[len("| `"):]
		idx := strings.Index(rest, "`")
		if idx < 0 {
			continue
		}
		path := rest[:idx]
		if !strings.Contains(path, "/") {
			continue
		}
		cells := strings.Split(rest[idx+1:], "|")
		if len(cells) < 2 {
			continue
		}
		class := strings.TrimSpace(cells[1])
		rows = append(rows, compatRow{path: path, class: class})
	}
	return rows
}

// ── 8. doc-coverage ─────────────────────────────────────────────────────────

// checkDocCoverage verifies that every exported package-level identifier
// (func, type, const, var, and methods on exported types) in the
// foundations/kits/runtimes/surfaces/toolchains universe (via
// scopedPkgDirs) has a doc comment whose text starts with the identifier's
// own name — the standard Go doc convention ("// Foo does X.").
//
// Nested-module rule: any package directory that has its own go.mod (i.e.
// distinct from the root module — currently kits/docextract/pdf and
// kits/docextract/xlsx, detected generically via os.Stat, never hardcoded
// by name) is SKIPPED from doc-coverage scanning. Nested modules carry
// independent versioning (their own vX.Y.Z tags, D1) and are out of scope
// for v1 doc-coverage — a documented gap, not a silent hole: a Pass finding
// notes the skip so it stays visible in -v output.
//
// Waivable per package in cmd/constitution-lint/doc-waivers.txt (same
// shape as waivers.txt): every finding for a waived package's directory is
// suppressed and replaced with a single Pass finding noting the waiver.
func checkDocCoverage(root string) []Finding {
	waivers, err := loadWaivers(filepath.Join(root, "cmd", "constitution-lint", "doc-waivers.txt"))
	if err != nil {
		return []Finding{{
			Check:   "doc-coverage",
			Message: fmt.Sprintf("cannot read doc-waivers.txt: %v", err),
		}}
	}

	dirs, err := scopedPkgDirs(root)
	if err != nil {
		return []Finding{{
			Check:   "doc-coverage",
			Message: fmt.Sprintf("cannot enumerate package dirs: %v", err),
		}}
	}

	var findings []Finding
	fset := token.NewFileSet()
	for _, rel := range dirs {
		absDir := filepath.Join(root, rel)

		if waivers[rel] {
			findings = append(findings, Finding{
				Check:   "doc-coverage",
				Pass:    true,
				Message: fmt.Sprintf("%s: waived (see cmd/constitution-lint/doc-waivers.txt)", rel),
			})
			continue
		}

		if fileExists(filepath.Join(absDir, "go.mod")) {
			findings = append(findings, Finding{
				Check:   "doc-coverage",
				Pass:    true,
				Message: fmt.Sprintf("%s: nested module, doc-coverage skipped (independent versioning)", rel),
			})
			continue
		}

		total, missing, issues := scanDocCoverage(fset, absDir, rel)
		if total == 0 {
			continue
		}
		findings = append(findings, issues...)
		if missing == 0 {
			findings = append(findings, Finding{
				Check:   "doc-coverage",
				Pass:    true,
				Message: fmt.Sprintf("%s: %d/%d exported identifiers documented", rel, total, total),
			})
		}
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{
			Check:   "doc-coverage",
			Pass:    true,
			Message: "no library package dirs found",
		})
	}
	return findings
}

// scanDocCoverage parses all non-test .go files directly in absDir (no
// recursion — scopedPkgDirs/catalog.Collect already yields one entry per
// directory) and returns the total number of exported package-level
// identifiers found, how many are missing/malformed doc comments, and a
// Fail Finding per such identifier (referencing file:line and name). rel is
// the module-relative directory, used only for display.
func scanDocCoverage(fset *token.FileSet, absDir, rel string) (total, missing int, issues []Finding) {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return 0, 0, []Finding{{Check: "doc-coverage", Message: fmt.Sprintf("%s: cannot read dir: %v", rel, err)}}
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(absDir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			issues = append(issues, Finding{Check: "doc-coverage", Message: fmt.Sprintf("%s/%s: parse error: %v", rel, name, err)})
			continue
		}
		if strings.HasSuffix(f.Name.Name, "_test") {
			continue
		}

		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				recvName := receiverTypeName(d.Recv)
				if d.Recv != nil && !ast.IsExported(recvName) {
					continue // method on an unexported type: out of scope
				}
				if !ast.IsExported(d.Name.Name) {
					continue
				}
				total++
				kind := "func"
				if d.Recv != nil {
					kind = "method"
				}
				if ok, why := hasValidDoc(d.Doc, d.Name.Name); !ok {
					missing++
					pos := fset.Position(d.Name.Pos())
					issues = append(issues, Finding{Check: "doc-coverage", Message: fmt.Sprintf("%s:%d: %s %q %s", relFile(rel, name), pos.Line, kind, d.Name.Name, why)})
				}

			case *ast.GenDecl:
				if d.Tok == token.IMPORT {
					continue
				}
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if !ast.IsExported(s.Name.Name) {
							continue
						}
						total++
						doc := s.Doc
						if doc == nil && len(d.Specs) == 1 {
							doc = d.Doc
						}
						if ok, why := hasValidDoc(doc, s.Name.Name); !ok {
							missing++
							pos := fset.Position(s.Name.Pos())
							issues = append(issues, Finding{Check: "doc-coverage", Message: fmt.Sprintf("%s:%d: type %q %s", relFile(rel, name), pos.Line, s.Name.Name, why)})
						}

					case *ast.ValueSpec:
						kind := "const"
						if d.Tok == token.VAR {
							kind = "var"
						}
						doc := s.Doc
						if doc == nil && len(d.Specs) == 1 {
							doc = d.Doc
						}
						for _, ident := range s.Names {
							if ident.Name == "_" || !ast.IsExported(ident.Name) {
								continue
							}
							total++
							if ok, why := hasValidDoc(doc, ident.Name); !ok {
								missing++
								pos := fset.Position(ident.Pos())
								issues = append(issues, Finding{Check: "doc-coverage", Message: fmt.Sprintf("%s:%d: %s %q %s", relFile(rel, name), pos.Line, kind, ident.Name, why)})
							}
						}
					}
				}
			}
		}
	}
	return total, missing, issues
}

// relFile joins a module-relative dir and a filename for display in
// doc-coverage messages.
func relFile(rel, name string) string {
	return rel + "/" + name
}

// receiverTypeName returns the base identifier of a method receiver's type
// (unwrapping pointer/generic-instantiation syntax), or "" if it cannot be
// determined.
func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	for {
		switch t := expr.(type) {
		case *ast.StarExpr:
			expr = t.X
		case *ast.IndexExpr:
			expr = t.X
		case *ast.IndexListExpr:
			expr = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
		}
	}
}

// hasValidDoc reports whether doc is a non-empty comment whose text starts
// with name (the stdlib doc convention: "// Foo does X." or "// Foo is a
// ..."). HasPrefix(name) alone is the check — no additional separator is
// required.
func hasValidDoc(doc *ast.CommentGroup, name string) (ok bool, why string) {
	if doc == nil {
		return false, "has no doc comment"
	}
	text := strings.TrimSpace(doc.Text())
	if text == "" {
		return false, "has no doc comment"
	}
	if !strings.HasPrefix(text, name) {
		return false, fmt.Sprintf("doc comment does not start with %q", name)
	}
	return true, ""
}

// ── 9. org-agnostic ──────────────────────────────────────────────────────────

// modelIDPattern pairs a recognizable model-ID literal shape with whether a
// match must additionally contain a digit to count as a hit.
type modelIDPattern struct {
	re           *regexp.Regexp
	requireDigit bool
}

// modelIDPatterns are recognizable model-ID literal shapes. These are Go
// literals in the SCANNER's own pattern list (matching-pattern config, not
// org data — the design constraint only requires org-name tokens to live in
// the .txt denylist). A standalone bare "-\d{8}" date-suffix shape is
// deliberately omitted: alone it matches any hyphen followed by 8 digits
// (issue numbers, unrelated dates, ...) and would be far too noisy; the
// date suffix is instead absorbed into the trailing [a-z0-9.-]+ of the
// vendor-prefixed patterns below (e.g. "claude-3-5-sonnet-20241022").
//
// requireDigit on the bare "claude-" pattern is load-bearing, not
// decorative: without it, this pattern matched runtimes/credresolve's
// `SourceClaudeCLI Source = "claude-cli"` — a credential-source enum value
// naming the Claude Code CLI tool, not a model ID — during the real sweep
// this check was built against. Every genuine Claude model ID carries a
// version component with a digit (claude-opus-4-8, claude-3-5-sonnet-
// -20241022, ...), so requiring one filters exactly that false positive
// without narrowing real hits.
var modelIDPatterns = []modelIDPattern{
	{re: regexp.MustCompile(`claude-[a-z0-9.-]+`), requireDigit: true},
	{re: regexp.MustCompile(`gpt-[0-9][a-z0-9.-]*`)},
	{re: regexp.MustCompile(`anthropic\.claude-[a-z0-9.-]*`)},
	{re: regexp.MustCompile(`us\.anthropic\.[a-z0-9.-]*`)},
}

// containsDigit reports whether s contains at least one ASCII digit.
func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

// isModelIDCarveOut reports whether rel is under a provider's own directory
// — any third-level dir under foundations/llmclient/ (e.g.
// foundations/llmclient/bedrock, .../anthropic, .../openai) or under
// foundations/decision/ (the decision port's provider adapters, e.g.
// foundations/decision/openai, .../typesafe) — whose entire job is
// choosing/exposing a default model ID. Generalizes the bedrock-specific
// carve-out named in the spec to any provider adapter. The decision port's
// core (a second-level file) and its conformance suite are not adapters and
// stay scanned.
func isModelIDCarveOut(rel string) bool {
	parts := strings.Split(rel, "/")
	if len(parts) < 4 || parts[0] != "foundations" {
		return false
	}
	switch parts[1] {
	case "llmclient":
		return true
	case "decision":
		return parts[2] != "decisiontest"
	}
	return false
}

// isUnderTestdata reports whether any path segment of rel is literally
// "testdata".
func isUnderTestdata(rel string) bool {
	for _, p := range strings.Split(rel, "/") {
		if p == "testdata" {
			return true
		}
	}
	return false
}

// checkOrgAgnostic scans tracked source and docs (see walkSweepFiles for the
// exact skip list) for two kinds of finding:
//
//  1. Org-specific tokens: case-sensitive exact substring matches against
//     the literal tokens in cmd/constitution-lint/org-denylist.txt (one
//     token per line, # comments). Legitimate lowercase sibling-repo names
//     (sextant, throughline, growing-gardens-bi) are never denylisted —
//     they are required architecture (task downstream-check's three real
//     consumers), not org leaks.
//  2. Hardcoded model-ID literals (modelIDPatterns) in Go source, outside
//     testdata/ dirs (any path segment literally "testdata" — this also
//     covers cassette .jsonl fixtures, which always live under a testdata/
//     dir) and outside any foundations/llmclient/<providername>/ package
//     (isModelIDCarveOut). Comment-only lines (trimmed text starting with
//     "//") are skipped for this pattern class only: several exported
//     struct-field doc comments in the real tree illustrate the expected
//     value shape with a literal example (e.g. `// Model is ... (e.g.
//     "claude-opus-4-8").`), which is documentation, not a hardcoded
//     runtime default — the genuine hardcoded-default finding this check
//     exists to catch is a real code line (an assignment or map entry).
//
// Both finding kinds are waivable per path or path:line in
// cmd/constitution-lint/org-agnostic-waivers.txt (path/path:line, one per
// line, # comments; same loadWaivers shape as waivers.txt).
//
// org-denylist.txt and org-agnostic-waivers.txt themselves are excluded from
// token matching (not model-ID matching, which never applies to .txt files
// anyway): both files necessarily state, in plain text, the exact tokens
// they govern (the denylist as data, the waiver file's comments explaining
// why a historical hit is waived) — that is the files doing their
// documented job, not a leak. This is a structural exclusion, not a waiver,
// so it applies uniformly to the real root and to every test fixture that
// carries its own nested copy of either file.
func checkOrgAgnostic(root string) []Finding {
	const (
		denylistRelPath = "cmd/constitution-lint/org-denylist.txt"
		waiversRelPath  = "cmd/constitution-lint/org-agnostic-waivers.txt"
	)
	denylist, err := loadDenylist(filepath.Join(root, filepath.FromSlash(denylistRelPath)))
	if err != nil {
		return []Finding{{Check: "org-agnostic", Message: fmt.Sprintf("cannot read org-denylist.txt: %v", err)}}
	}
	waivers, err := loadWaivers(filepath.Join(root, filepath.FromSlash(waiversRelPath)))
	if err != nil {
		return []Finding{{Check: "org-agnostic", Message: fmt.Sprintf("cannot read org-agnostic-waivers.txt: %v", err)}}
	}

	var findings []Finding
	hits := 0

	report := func(rel string, lineNum int, msg string) {
		if waivedLine(waivers, rel, lineNum) {
			findings = append(findings, Finding{
				Check:   "org-agnostic",
				Pass:    true,
				Message: fmt.Sprintf("%s:%d: waived (%s) — see cmd/constitution-lint/org-agnostic-waivers.txt", rel, lineNum, msg),
			})
			return
		}
		hits++
		findings = append(findings, Finding{Check: "org-agnostic", Message: fmt.Sprintf("%s:%d: %s", rel, lineNum, msg)})
	}

	walkErr := walkSweepFiles(root, func(rel string, data []byte) {
		// The denylist and its own waiver file necessarily document, in
		// plain text, the exact tokens they govern (org-denylist.txt lists
		// them as data; org-agnostic-waivers.txt's comments explain why a
		// historical hit is waived, which requires naming it). That is
		// these two files doing their documented job, not a leak — skip
		// token matching against themselves structurally rather than
		// requiring every fixture/root to carry a self-referential waiver.
		isSelf := rel == denylistRelPath || rel == waiversRelPath

		lines := strings.Split(string(data), "\n")
		isGo := strings.HasSuffix(rel, ".go")
		scanModelID := isGo && !isUnderTestdata(rel) && !isModelIDCarveOut(rel)

		for i, line := range lines {
			lineNum := i + 1

			if !isSelf {
				for _, dt := range denylist {
					if dt.re.MatchString(line) {
						report(rel, lineNum, fmt.Sprintf("contains denylisted org-specific token %q", dt.tok))
					}
				}
			}

			if scanModelID && !strings.HasPrefix(strings.TrimSpace(line), "//") {
				for _, mp := range modelIDPatterns {
					m := mp.re.FindString(line)
					if m == "" {
						continue
					}
					if mp.requireDigit && !containsDigit(m) {
						continue
					}
					report(rel, lineNum, fmt.Sprintf("hardcoded model-ID literal %q outside testdata/cassette/provider-adapter carve-out", m))
				}
			}
		}
	})
	if walkErr != nil {
		return []Finding{{Check: "org-agnostic", Message: fmt.Sprintf("sweep error: %v", walkErr)}}
	}

	if hits == 0 {
		findings = append(findings, Finding{
			Check:   "org-agnostic",
			Pass:    true,
			Message: "no org-specific tokens or hardcoded model-ID literals found",
		})
	}
	return findings
}

// loadDenylist reads one literal token per line from path (# comments,
// blank lines ignored) — same file shape as waivers.txt, but the values are
// substrings to match rather than paths.
// denyToken is one denylist entry: the literal token (for messages) plus
// its compiled whole-word matcher. Matching is word-boundary (`\b tok \b`,
// token regex-quoted), not bare substring: short tokens like a 3-character
// org codename would otherwise match inside unrelated identifiers, hashes,
// and .jsonl cassette payloads, failing the gate spuriously.
type denyToken struct {
	tok string
	re  *regexp.Regexp
}

func loadDenylist(path string) ([]denyToken, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tokens []denyToken
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tokens = append(tokens, denyToken{
			tok: line,
			re:  regexp.MustCompile(`\b` + regexp.QuoteMeta(line) + `\b`),
		})
	}
	return tokens, scanner.Err()
}

// waivedLine reports whether rel (whole file) or rel:line is present in a
// loadWaivers-shaped waiver set.
func waivedLine(waivers map[string]bool, rel string, line int) bool {
	if waivers[rel] {
		return true
	}
	return waivers[fmt.Sprintf("%s:%d", rel, line)]
}

// ── 10. secret-scan ──────────────────────────────────────────────────────────

// checkSecretScan sweeps the working tree on disk — every file under root
// except .git/, .claude/worktrees/, workspace/, tmp/, and *_test.go (see
// walkSweepFiles); it
// never consults git, so "tracked vs untracked" plays no part — for matches
// against the well-known secret shapes in
// cmd/constitution-lint/secret-patterns.txt (one regexp per line, #
// comments). It is repo-local and stdlib-only, and is a required,
// always-on check: it never shells out to a sibling binary, and a missing
// or empty pattern file is itself a Fail (never a silent, vacuous pass).
//
// _test.go files are excluded from this sweep (see walkSweepFiles): this
// repo's own security test suites (foundations/security/pre-llm-dlp-gate,
// foundations/llmclient/{anthropic,replay}) already commit exactly this
// class of obviously-fake, pattern-matching credential as a legitimate
// redaction-test fixture (verified live in the tree at the time this check
// was written) — without the exclusion, this required gate would never be
// green. The consequence, stated plainly: a REAL secret committed inside a
// _test.go file is invisible to this always-on gate. The full-history
// `git log -p` sweep that covered test files too was a one-time audit at
// this check's introduction, not a recurring mechanism — reviewers should
// treat credentials in test files as a manual-review concern. Real secrets
// belong in real (non-test) source or config, which this check still
// sweeps in full, including other packages' testdata/ dirs (a real secret
// accidentally left in a fixture is exactly the kind of leak a scanner
// should catch) — the ONLY testdata/ carve-out is this linter's own
// cmd/constitution-lint/testdata (see walkSweepFiles), so its own
// canary/dirty fixtures don't trip the always-on gate either.
func checkSecretScan(root string) []Finding {
	patterns, err := loadSecretPatterns(secretPatternsPath(root))
	if err != nil {
		return []Finding{{Check: "secret-scan", Message: fmt.Sprintf("cannot load secret-patterns.txt: %v", err)}}
	}
	if len(patterns) == 0 {
		return []Finding{{Check: "secret-scan", Message: "secret-patterns.txt has no patterns — secret-scan would vacuously pass"}}
	}

	var findings []Finding
	hits := 0
	walkErr := walkSweepFiles(root, func(rel string, data []byte) {
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			for _, p := range patterns {
				if m := p.re.FindString(line); m != "" {
					hits++
					findings = append(findings, Finding{
						Check:   "secret-scan",
						Message: fmt.Sprintf("%s:%d: matches secret pattern %q", rel, i+1, p.raw),
					})
				}
			}
		}
	})
	if walkErr != nil {
		return []Finding{{Check: "secret-scan", Message: fmt.Sprintf("sweep error: %v", walkErr)}}
	}

	if hits == 0 {
		findings = append(findings, Finding{
			Check:   "secret-scan",
			Pass:    true,
			Message: "no secret-shaped literals found",
		})
	}
	return findings
}

// secretPattern is one named, compiled regexp loaded from
// secret-patterns.txt.
type secretPattern struct {
	raw string
	re  *regexp.Regexp
}

// loadSecretPatterns reads one RE2 regexp per line from path (# comments,
// blank lines ignored).
func loadSecretPatterns(path string) ([]secretPattern, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var patterns []secretPattern
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		re, err := regexp.Compile(line)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", line, err)
		}
		patterns = append(patterns, secretPattern{raw: line, re: re})
	}
	return patterns, scanner.Err()
}

// secretPatternsPath resolves the location of secret-patterns.txt. It
// prefers root/cmd/constitution-lint/secret-patterns.txt (correct in
// production, where root is the repo root and the file is genuinely
// nested there). If that path doesn't exist — as when a test scopes root
// to a narrow testdata fixture (e.g. TestSecretScanCanary's
// "testdata/secretscan-canary", or any other check-specific fixture) — it
// falls back to the package-relative "secret-patterns.txt", which resolves
// correctly because `go test` always runs with the package directory as
// its working directory. This lets every test exercise the one real,
// committed pattern file rather than requiring a duplicate copy per
// fixture, which matters here specifically (D10): a canary that checks a
// scanner against its own private copy of the patterns proves nothing about
// the patterns actually shipped.
func secretPatternsPath(root string) string {
	candidate := filepath.Join(root, "cmd", "constitution-lint", "secret-patterns.txt")
	if fileExists(candidate) {
		return candidate
	}
	return "secret-patterns.txt"
}

// ── 11. readme-count ─────────────────────────────────────────────────────────

// readmeCountRe extracts the bold package-count sentence README.md leads
// with ("The tree is **N Go packages, ..." or "**N Go packages**").
var readmeCountRe = regexp.MustCompile(`\*\*(\d+) Go packages`)

// checkReadmeCount verifies README.md's stated package count equals the
// source-counted catalog total (the same cmd/internal/catalog.Collect count
// that generates docs/catalog.md), closing the drift class where the README
// froze at a hand-maintained number ("148") while the tree moved on.
func checkReadmeCount(root string) []Finding {
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return []Finding{{
			Check:   "readme-count",
			Message: fmt.Sprintf("cannot read README.md: %v", err),
		}}
	}
	m := readmeCountRe.FindSubmatch(readme)
	if m == nil {
		return []Finding{{
			Check:   "readme-count",
			Message: `README.md has no "**N Go packages" sentence to verify`,
		}}
	}
	stated, err := strconv.Atoi(string(m[1]))
	if err != nil { // unreachable given \d+, but fail loudly rather than panic
		return []Finding{{
			Check:   "readme-count",
			Message: fmt.Sprintf("cannot parse README.md package count: %v", err),
		}}
	}

	entries, err := catalog.Collect(root)
	if err != nil {
		return []Finding{{
			Check:   "readme-count",
			Message: fmt.Sprintf("catalog generation error: %v", err),
		}}
	}
	if stated != len(entries) {
		return []Finding{{
			Check:   "readme-count",
			Message: fmt.Sprintf("README.md says %d Go packages; source count is %d — update the README sentence", stated, len(entries)),
		}}
	}
	return []Finding{{
		Check:   "readme-count",
		Pass:    true,
		Message: fmt.Sprintf("README.md package count matches source count (%d)", stated),
	}}
}

// ── sweep helpers (shared by org-agnostic and secret-scan) ──────────────────

// sweepSkipDirs are the directories always excluded from the repo-wide
// sweeps, at any depth: .git/ (VCS internals), workspace/ and tmp/
// (generated, gitignored scratch — see AGENTS.md's workspace/ entry), and
// .claude/worktrees/ (Claude Code agent worktrees — full stale copies of
// the repo, whose waivers, keyed to real paths, can never match; never
// shipped, never swept). A key without a slash matches a directory's base
// name; a "parent/name" key matches only that name directly under that
// parent. The rest of .claude/ is swept: .claude/skills holds the kit's
// shipped skills (the one real copy; .agents/skills links to it), so a
// secret or an org-specific name there must fail like anywhere else.
var sweepSkipDirs = map[string]bool{
	".git":              true,
	".claude/worktrees": true,
	"workspace":         true,
	"tmp":               true,
}

// isSweepSkipDir reports whether the directory at rel (root-relative,
// slash-separated) is one of sweepSkipDirs.
func isSweepSkipDir(rel string) bool {
	name := path.Base(rel)
	if sweepSkipDirs[name] {
		return true
	}
	parent := path.Base(path.Dir(rel))
	return parent != "." && sweepSkipDirs[parent+"/"+name]
}

// lintOwnTestdataRel is this linter's own fixture directory, excluded from
// the repo-wide sweeps as synthetic self-test content (analogous to
// generated noise, not real repo content) — see checkSecretScan's doc
// comment for why this specific exclusion is load-bearing.
const lintOwnTestdataRel = "cmd/constitution-lint/testdata"

// walkSweepFiles calls fn(rel, data) for every regular file under root,
// skipping sweepSkipDirs, lintOwnTestdataRel, and any *_test.go file
// (Go test files legitimately contain fake-secret/fake-org-token literals
// as test fixtures for this repo's own redaction/security suites — see
// checkSecretScan). rel is root-relative and slash-separated. It is
// walkWithManifest over the run's manifest: the sweep's skip semantics
// are unchanged, but every skip and every failed read is recorded, so a
// file that could not be read fails the run through read-coverage
// instead of vanishing from the sweep.
func walkSweepFiles(root string, fn func(rel string, data []byte)) error {
	return walkWithManifest(root, reads, fn)
}

// walkWithManifest walks the tree under root and calls fn(rel, data) for
// every regular, text, non-test file it can read, recording the outcome of
// every path it considers into reads: read, absent (a broken symlink or a
// file removed mid-walk — a fact, still skipped), unreadable (recorded
// with the operating system's own wording, still skipped, and the reason
// read-coverage fails), or skipped with its category ("sweep-skip-dir"
// for sweepSkipDirs and lintOwnTestdataRel, recorded once for the
// directory rather than per descendant; "test-file"; "dir-symlink", a
// symlink whose target is a directory, which the walk never descends and
// which is read at its real path when that lies under root, so
// os.ReadFile's "is a directory" is not a failed read). Binary-looking
// files (a NUL byte in the first 8000 bytes, the same heuristic classic
// Unix text tools use) were read successfully and are recorded as read;
// they are simply not handed to fn. A directory that cannot be entered is
// recorded as unreadable and then returned as the walk error, exactly as
// before, so the caller's "sweep error" finding is preserved alongside
// the manifest's.
func walkWithManifest(root string, reads *readmanifest.Set, fn func(rel string, data []byte)) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				reads.Record(path, readmanifest.Absent())
			} else {
				reads.Record(path, readmanifest.Unreadable(err))
			}
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if isSweepSkipDir(rel) || rel == lintOwnTestdataRel {
				reads.Skip(path, "sweep-skip-dir")
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, "_test.go") {
			reads.Skip(path, "test-file")
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
				reads.Skip(path, "dir-symlink")
				return nil
			}
		}
		data, ok := reads.ReadFile(path)
		if !ok {
			return nil
		}
		if looksBinary(data) {
			return nil // read successfully (and recorded so); not swept
		}
		fn(rel, data)
		return nil
	})
}

// looksBinary reports whether data appears to be a binary file, using the
// classic heuristic of a NUL byte in the first 8000 bytes.
func looksBinary(data []byte) bool {
	limit := 8000
	if len(data) < limit {
		limit = len(data)
	}
	return bytes.IndexByte(data[:limit], 0) >= 0
}

// ── shared helpers ───────────────────────────────────────────────────────────

// scopedPkgDirs returns sorted, deduplicated module-relative package
// directories under foundations/, kits/, runtimes/, surfaces/, and
// toolchains/ — the shared universe for compat-coverage and doc-coverage.
// Built on cmd/internal/catalog.Collect, which already flattens nested
// modules (e.g. kits/docextract/pdf) into their own entries with no regard
// for go.mod boundaries.
func scopedPkgDirs(root string) ([]string, error) {
	entries, err := catalog.Collect(root)
	if err != nil {
		return nil, err
	}
	prefixes := map[string]bool{
		"foundations": true,
		"kits":        true,
		"runtimes":    true,
		"surfaces":    true,
		"toolchains":  true,
	}
	seen := map[string]bool{}
	var dirs []string
	for _, e := range entries {
		top := strings.SplitN(e.RelPath, "/", 2)[0]
		if !prefixes[top] || seen[e.RelPath] {
			continue
		}
		seen[e.RelPath] = true
		dirs = append(dirs, e.RelPath)
	}
	sort.Strings(dirs)
	return dirs, nil
}

// libraryPkgDirs returns sorted module-relative paths for all package dirs
// under foundations/ (two levels: foundations/<cat>/<pkg>) and kits/ (one
// level: kits/<pkg>). Skips _reference/, testdata/, and dirs with no .go files.
func libraryPkgDirs(root string) ([]string, error) {
	var dirs []string

	// foundations/<cat>/<pkg>
	foundRoot := filepath.Join(root, "foundations")
	cats, err := os.ReadDir(foundRoot)
	if err != nil {
		return nil, fmt.Errorf("read foundations/: %w", err)
	}
	for _, cat := range cats {
		if !cat.IsDir() || skipDirName(cat.Name()) {
			continue
		}
		catPath := filepath.Join(foundRoot, cat.Name())
		pkgs, err := os.ReadDir(catPath)
		if err != nil {
			recordFailure(catPath, err)
			continue
		}
		for _, pkg := range pkgs {
			if !pkg.IsDir() || skipDirName(pkg.Name()) {
				continue
			}
			rel := "foundations/" + cat.Name() + "/" + pkg.Name()
			if hasGoFiles(filepath.Join(root, rel)) {
				dirs = append(dirs, rel)
			}
		}
	}

	// kits/<pkg>
	kitsRoot := filepath.Join(root, "kits")
	kits, err := os.ReadDir(kitsRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read kits/: %w", err)
		}
		kits = nil
	}
	for _, kit := range kits {
		if !kit.IsDir() || skipDirName(kit.Name()) {
			continue
		}
		rel := "kits/" + kit.Name()
		if hasGoFiles(filepath.Join(root, rel)) {
			dirs = append(dirs, rel)
		}
	}

	sort.Strings(dirs)
	return dirs, nil
}

// skipDirName returns true for dirs that should not be traversed.
func skipDirName(name string) bool {
	if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
		return true
	}
	return name == "testdata"
}

// hasGoFiles returns true if dir contains at least one non-test .go file.
func hasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		recordFailure(dir, err)
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			return true
		}
	}
	return false
}

// fileExists returns true if path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func main() {
	os.Exit(run(".", os.Stdout, os.Args[1:]))
}
