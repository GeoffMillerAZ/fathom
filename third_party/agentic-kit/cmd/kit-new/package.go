package main

// Package mode: `kit-new -package <root>/<category>/<name>` scaffolds a new
// kit package in this repository's six-file shape and prints (or, with
// -apply, appends) the exact ledger lines the junk gate demands. See the
// command doc in main.go for the flag reference; scaffoldPackage is the
// testable core.

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/geoffmilleraz/agentic-kit/cmd/internal/catalog"
)

// packageRoots are the five roots that make up the kit's library surface
// (docs/compat.md §method). cmd/, examples/ and scenarios/ are tooling, not
// packages in this shape, so they are not scaffoldable.
var packageRoots = map[string]bool{
	"foundations": true,
	"kits":        true,
	"runtimes":    true,
	"surfaces":    true,
	"toolchains":  true,
}

var (
	// kebabRe is the directory-name grammar: lower-case kebab, digits allowed
	// after the first character (CLAUDE.md: dirs are kebab-case).
	kebabRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	// exportedIdentRe is the grammar for -interface.
	exportedIdentRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	// readmeCountRe is the README package-count sentence constitution-lint's
	// readme-count check verifies.
	readmeCountRe = regexp.MustCompile(`\*\*(\d+) Go packages`)
	// goModuleRe / goDirectiveRe read the two go.mod lines package mode needs.
	goModuleRe    = regexp.MustCompile(`(?m)^module\s+(\S+)`)
	goDirectiveRe = regexp.MustCompile(`(?m)^go\s+(\S+)`)
	// cueVersionRe reads the language version from cue.mod/module.cue.
	cueVersionRe = regexp.MustCompile(`version:\s*"([^"]+)"`)
)

// packageScaffold holds the resolved parameters for one -package run.
type packageScaffold struct {
	// repo is the kit repository root (holds go.mod, TRACKING.md, docs/).
	repo string
	// pkgPath is the module-relative package directory, slash-separated.
	pkgPath string
	// practice is the docs/practices.md section number, or 0 for none.
	practice int
	// iface is the exported interface name to declare and mock, or "".
	iface string
	// consumer names the consumer or future workload that pulls for the
	// package (the junk gate), or "".
	consumer string
	// apply appends the ledger lines instead of only printing them.
	apply bool
	// verifiedAgainst overrides doc.go's Verified-against text; "" derives
	// it from go.mod and cue.mod.
	verifiedAgainst string
}

// packageData is the value every package-mode template receives.
type packageData struct {
	// Path is the module-relative package directory ("foundations/telemetry/read-manifest").
	Path string
	// Root is the first path segment ("foundations").
	Root string
	// Category is the middle segment for three-segment paths ("telemetry"), else "".
	Category string
	// Name is the leaf directory ("read-manifest").
	Name string
	// Pkg is the Go package name: Name with hyphens collapsed ("readmanifest").
	Pkg string
	// Type is the primary type name: Name in PascalCase ("ReadManifest").
	Type string
	// Iface is the -interface name, or "".
	Iface string
	// Module is the repository's Go module path (from go.mod).
	Module string
	// VerifiedAgainst is the toolchain text for doc.go.
	VerifiedAgainst string
}

// ledgerLine is one ledger entry package mode prints and, with -apply, appends.
type ledgerLine struct {
	// File is the repo-relative ledger file.
	File string
	// Where names the anchor the line belongs under, for the human.
	Where string
	// Line is the exact text to add.
	Line string
	// Status is what -apply did (or, in print mode, what it would do):
	// "print" / "applied" / "already present" / "anchor not found".
	Status string
}

// packageResult is what scaffoldPackage reports back for printing.
type packageResult struct {
	Data packageData
	// Files are the repo-relative paths written, sorted.
	Files []string
	// Ledgers are the ledger lines in print order.
	Ledgers []ledgerLine
	// ReadmeFrom / ReadmeTo are the package-count sentence before and after
	// the correction (empty when README.md or its sentence was not found).
	ReadmeFrom, ReadmeTo string
	// ReadmeStatus mirrors ledgerLine.Status for the README count:
	// "print" / "applied" / "already correct" / "anchor not found" /
	// "count unavailable".
	ReadmeStatus string
}

// runPackage is the -package entry point: it validates, scaffolds, and
// prints. Exit codes match the consumer mode (0 ok, 1 generation error, 2
// usage error).
func runPackage(sc packageScaffold, stdout, stderr io.Writer) int {
	if _, err := parsePackagePath(sc.pkgPath); err != nil {
		fmt.Fprintf(stderr, "kit-new: %v\n", err)
		return 2
	}
	if sc.iface != "" && !exportedIdentRe.MatchString(sc.iface) {
		fmt.Fprintf(stderr, "kit-new: -interface %q must be an exported Go identifier\n", sc.iface)
		return 2
	}
	if sc.practice < 0 {
		fmt.Fprintf(stderr, "kit-new: -practice %d must be a positive section number (0 = none)\n", sc.practice)
		return 2
	}
	res, err := scaffoldPackage(sc)
	if err != nil {
		fmt.Fprintf(stderr, "kit-new: %v\n", err)
		return 1
	}
	printPackageResult(stdout, sc, res)
	return 0
}

// parsePackagePath validates and splits a -package value.
func parsePackagePath(p string) (packageData, error) {
	clean := path.Clean(strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/"))
	parts := strings.Split(clean, "/")
	if len(parts) < 2 || len(parts) > 3 || clean != p {
		return packageData{}, fmt.Errorf("-package %q must be <root>/<category>/<name> (foundations) or <root>/<name> (kits, runtimes, surfaces, toolchains), slash-separated and clean", p)
	}
	root := parts[0]
	if !packageRoots[root] {
		return packageData{}, fmt.Errorf("-package root %q must be one of foundations, kits, runtimes, surfaces, toolchains (cmd/, examples/ and scenarios/ are not library packages)", root)
	}
	if root == "foundations" && len(parts) != 3 {
		return packageData{}, fmt.Errorf("-package %q: foundations packages are foundations/<category>/<name>", p)
	}
	for _, seg := range parts[1:] {
		if !kebabRe.MatchString(seg) {
			return packageData{}, fmt.Errorf("-package segment %q must be lower-case kebab-case (CLAUDE.md: dirs are kebab-case)", seg)
		}
	}
	name := parts[len(parts)-1]
	d := packageData{
		Path: clean,
		Root: root,
		Name: name,
		Pkg:  strings.ReplaceAll(name, "-", ""),
		Type: pascalCase(name),
	}
	if len(parts) == 3 {
		d.Category = parts[1]
	}
	return d, nil
}

// pascalCase turns "read-manifest" into "ReadManifest".
func pascalCase(kebab string) string {
	var b strings.Builder
	for _, seg := range strings.Split(kebab, "-") {
		if seg == "" {
			continue
		}
		b.WriteString(strings.ToUpper(seg[:1]))
		b.WriteString(seg[1:])
	}
	return b.String()
}

// packageFile is one rendered scaffold file.
type packageFile struct {
	// rel is the path relative to the package directory.
	rel string
	// tmpl is the embedded template name under templates/package/.
	tmpl string
	// gofmt marks Go sources, which are passed through go/format so the
	// scaffold is gofmt-clean by construction.
	gofmt bool
}

// packageFiles lists the scaffold in write order; mock.go is added when an
// interface is requested.
func packageFiles(d packageData) []packageFile {
	files := []packageFile{
		{rel: "schema.cue", tmpl: "schema.cue.tmpl"},
		{rel: "types.go", tmpl: "types.go.tmpl", gofmt: true},
		{rel: "doc.go", tmpl: "doc.go.tmpl", gofmt: true},
		{rel: d.Name + ".go", tmpl: "impl.go.tmpl", gofmt: true},
		{rel: d.Name + "_test.go", tmpl: "test.go.tmpl", gofmt: true},
		{rel: "testdata/valid.cue", tmpl: "valid.cue.tmpl"},
		{rel: "testdata/invalid.cue", tmpl: "invalid.cue.tmpl"},
	}
	if d.Iface != "" {
		files = append(files, packageFile{rel: "mock.go", tmpl: "mock.go.tmpl", gofmt: true})
	}
	return files
}

// scaffoldPackage renders the six-file package shape into
// sc.repo/sc.pkgPath, computes the ledger lines, and — with sc.apply —
// appends them. It is all-or-nothing: every file is rendered and every
// ledger anchor resolved BEFORE anything touches disk, so a refusal (an
// existing file, a missing anchor) leaves the tree exactly as it was.
// Re-running on an existing scaffold is refused, never merged: the tool is
// idempotent by refusing, not by overwriting.
func scaffoldPackage(sc packageScaffold) (*packageResult, error) {
	d, err := parsePackagePath(sc.pkgPath)
	if err != nil {
		return nil, err
	}
	d.Iface = sc.iface

	goMod, err := os.ReadFile(filepath.Join(sc.repo, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("-repo %q must be a Go module root: %w", sc.repo, err)
	}
	m := goModuleRe.FindSubmatch(goMod)
	if m == nil {
		return nil, fmt.Errorf("-repo %q: go.mod has no module line", sc.repo)
	}
	d.Module = string(m[1])
	d.VerifiedAgainst = sc.verifiedAgainst
	if d.VerifiedAgainst == "" {
		d.VerifiedAgainst = deriveVerifiedAgainst(sc.repo, goMod)
	}

	// Render everything in memory first.
	pkgDir := filepath.Join(sc.repo, filepath.FromSlash(d.Path))
	type rendered struct {
		abs  string
		rel  string
		data []byte
	}
	var out []rendered
	var existing []string
	for _, f := range packageFiles(d) {
		data, err := renderPackageTemplate(f.tmpl, d)
		if err != nil {
			return nil, err
		}
		if f.gofmt {
			data, err = format.Source(data)
			if err != nil {
				return nil, fmt.Errorf("gofmt %s: %w", f.rel, err)
			}
		}
		abs := filepath.Join(pkgDir, filepath.FromSlash(f.rel))
		if _, err := os.Stat(abs); err == nil {
			existing = append(existing, d.Path+"/"+f.rel)
		}
		out = append(out, rendered{abs: abs, rel: d.Path + "/" + f.rel, data: data})
	}
	if len(existing) > 0 {
		return nil, fmt.Errorf("refusing to overwrite existing files (remove them or choose another -package):\n  %s", strings.Join(existing, "\n  "))
	}

	// Resolve the ledgers before writing anything.
	res := &packageResult{Data: d}
	edits, err := planLedgers(sc, d, res)
	if err != nil {
		return nil, err
	}

	// Write the package.
	if err := os.MkdirAll(filepath.Join(pkgDir, "testdata"), 0o755); err != nil {
		return nil, fmt.Errorf("create package dir: %w", err)
	}
	for _, r := range out {
		if err := os.WriteFile(r.abs, r.data, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", r.rel, err)
		}
		res.Files = append(res.Files, r.rel)
	}
	sort.Strings(res.Files)

	// Then the ledgers.
	if sc.apply {
		for _, e := range edits {
			if err := os.WriteFile(e.abs, []byte(e.content), 0o644); err != nil {
				return nil, fmt.Errorf("write %s: %w", e.rel, err)
			}
		}
	}

	// The README count last: it is the source-counted total (the same
	// cmd/internal/catalog walk constitution-lint's readme-count verifies),
	// taken after the package exists, so it is right by construction and a
	// re-run finds it already correct.
	if err := readmeCount(sc, res); err != nil {
		return nil, err
	}
	return res, nil
}

// readmeCount fills res.Readme* from README.md's "**N Go packages" sentence
// and the live source count; with sc.apply it rewrites the sentence when the
// two differ. A tree the catalog cannot parse reports "count unavailable"
// rather than failing a scaffold that is already on disk.
func readmeCount(sc packageScaffold, res *packageResult) error {
	readmeAbs := filepath.Join(sc.repo, "README.md")
	readme, err := os.ReadFile(readmeAbs)
	if err != nil {
		res.ReadmeStatus = "anchor not found"
		return nil
	}
	m := readmeCountRe.FindSubmatch(readme)
	if m == nil {
		res.ReadmeStatus = "anchor not found"
		return nil
	}
	entries, err := catalog.Collect(sc.repo)
	if err != nil {
		res.ReadmeStatus = "count unavailable: " + err.Error()
		return nil
	}
	stated, _ := strconv.Atoi(string(m[1]))
	res.ReadmeFrom = string(m[0])
	res.ReadmeTo = fmt.Sprintf("**%d Go packages", len(entries))
	switch {
	case stated == len(entries):
		res.ReadmeStatus = "already correct"
	case sc.apply:
		res.ReadmeStatus = "applied"
		if err := os.WriteFile(readmeAbs, bytes.Replace(readme, m[0], []byte(res.ReadmeTo), 1), 0o644); err != nil {
			return fmt.Errorf("write README.md: %w", err)
		}
	default:
		res.ReadmeStatus = "print"
	}
	return nil
}

// deriveVerifiedAgainst builds the Verified-against text from the go.mod go
// directive and, when present, cue.mod/module.cue's language version.
func deriveVerifiedAgainst(repo string, goMod []byte) string {
	parts := []string{}
	if m := goDirectiveRe.FindSubmatch(goMod); m != nil {
		parts = append(parts, "go "+string(m[1]))
	}
	if cueMod, err := os.ReadFile(filepath.Join(repo, "cue.mod", "module.cue")); err == nil {
		if m := cueVersionRe.FindSubmatch(cueMod); m != nil {
			parts = append(parts, "cue "+string(m[1]))
		}
	}
	if len(parts) == 0 {
		return "not yet verified — run task validate and record the toolchain here"
	}
	return strings.Join(parts, ", ") + " (at scaffold time; re-run task validate and update this line on every toolchain bump)"
}

// renderPackageTemplate renders one embedded templates/package/ template.
func renderPackageTemplate(name string, d packageData) ([]byte, error) {
	src, err := templateFS.ReadFile("templates/package/" + name)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", name, err)
	}
	tmpl, err := template.New(name).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// ── ledgers ──────────────────────────────────────────────────────────────────

// ledgerEdit is one ledger file rewritten by -apply.
type ledgerEdit struct {
	abs, rel, content string
}

// ledgerLines composes the exact lines for one package. Deterministic: no
// dates (the maintainer adds the landing date when the row flips to [x]).
func ledgerLines(sc packageScaffold, d packageData) (tracking, compat, changelog, practices string) {
	consumer := sc.consumer
	if consumer == "" {
		consumer = "TODO — name the consumer or future workload that pulls for it (VISION junk gate)"
	}
	tests := d.Name + "_test.go"
	if d.Iface != "" {
		tests += " · mock.go"
	}
	tracking = fmt.Sprintf("- [ ] `%s` (`%s`) — scaffolded by kit-new: replace with what it does, what it refuses to do, and its bounds — schema.cue · %s. Consumer: %s.", d.Name, d.Path, tests, consumer)
	if sc.practice > 0 {
		tracking += fmt.Sprintf(" Practice: §%d.", sc.practice)
	}
	compat = fmt.Sprintf("| `%s` | experimental | Scaffolded by kit-new; zero consumers proven yet — consumer: %s. Promote to stable only through a real consumer in `task downstream-check` (`docs/intent/repo.md` §Promote). |", d.Path, consumer)
	changelog = fmt.Sprintf("- **`%s`** (experimental): scaffolded package — replace this line with the observable behavior it ships, or drop it if nothing observable lands with it.", d.Path)
	if sc.practice > 0 {
		practices = fmt.Sprintf("| `%s` | replace with the role this package plays in §%d — what it enforces, in one line |", d.Path, sc.practice)
	}
	return tracking, compat, changelog, practices
}

// planLedgers resolves every ledger anchor, fills res.Ledgers with the lines
// and their statuses, and returns the rewritten files for -apply. In print
// mode a missing anchor is reported, not fatal; in -apply mode it is an
// error, raised before any file is written.
func planLedgers(sc packageScaffold, d packageData, res *packageResult) ([]ledgerEdit, error) {
	trackingLine, compatLine, changelogLine, practicesLine := ledgerLines(sc, d)
	marker := "`" + d.Path + "`"

	type plan struct {
		file  string
		where string
		line  string
		// insert computes the rewritten lines, or ok=false when the anchor
		// is missing. present reports the line already there.
		insert func(lines []string) (out []string, where string, present, ok bool)
	}
	plans := []plan{
		{
			file: "TRACKING.md", line: trackingLine,
			insert: func(lines []string) ([]string, string, bool, bool) {
				start, end, header, ok := trackingSection(lines, d.Root, d.Category)
				if !ok {
					return nil, "", false, false
				}
				if containsMarker(lines[start:end], marker) {
					return lines, header, true, true
				}
				at := start + 1
				for i := start + 1; i < end; i++ {
					if strings.HasPrefix(lines[i], "- [") {
						at = i + 1
					}
				}
				if at == start+1 && at < len(lines) && lines[at] == "" {
					at++
				}
				return insertLines(lines, at, trackingLine, true), header, false, true
			},
		},
		{
			file: "docs/compat.md", line: compatLine,
			insert: func(lines []string) ([]string, string, bool, bool) {
				start, end, ok := markdownSection(lines, "## Classification table", "## ")
				if !ok {
					return nil, "", false, false
				}
				at := -1
				lastRow := -1
				for i := start + 1; i < end; i++ {
					p, isRow := compatRowPath(lines[i])
					if !isRow {
						continue
					}
					if p == d.Path {
						return lines, "Classification table", true, true
					}
					lastRow = i
					if at < 0 && p > d.Path {
						at = i
					}
				}
				if at < 0 {
					if lastRow < 0 {
						return nil, "", false, false
					}
					at = lastRow + 1
				}
				return insertLines(lines, at, compatLine, false), "Classification table, sorted by path", false, true
			},
		},
		{
			file: "CHANGELOG.md", line: changelogLine,
			insert: func(lines []string) ([]string, string, bool, bool) {
				start, end, ok := markdownSection(lines, "## [Unreleased]", "## ")
				if !ok {
					return nil, "", false, false
				}
				if containsMarker(lines[start:end], marker) {
					return lines, "[Unreleased] › Added", true, true
				}
				for i := start + 1; i < end; i++ {
					if strings.TrimSpace(lines[i]) != "### Added" {
						continue
					}
					at := i + 1
					if at < end && lines[at] == "" {
						at++
					}
					return insertLines(lines, at, changelogLine, true), "[Unreleased] › Added", false, true
				}
				return nil, "", false, false
			},
		},
	}
	if sc.practice > 0 {
		header := fmt.Sprintf("## %d. ", sc.practice)
		plans = append(plans, plan{
			file: "docs/practices.md", line: practicesLine,
			insert: func(lines []string) ([]string, string, bool, bool) {
				start, end, ok := markdownSection(lines, header, "## ")
				if !ok {
					return nil, "", false, false
				}
				where := fmt.Sprintf("§%d table", sc.practice)
				if containsMarker(lines[start:end], marker) {
					return lines, where, true, true
				}
				lastRow := -1
				for i := start + 1; i < end; i++ {
					if strings.HasPrefix(lines[i], "|") {
						lastRow = i
					} else if lastRow >= 0 {
						break
					}
				}
				if lastRow < 0 {
					return nil, "", false, false
				}
				return insertLines(lines, lastRow+1, practicesLine, false), where, false, true
			},
		})
	}

	var edits []ledgerEdit
	for _, p := range plans {
		abs := filepath.Join(sc.repo, filepath.FromSlash(p.file))
		entry := ledgerLine{File: p.file, Line: p.line, Status: "print"}
		data, err := os.ReadFile(abs)
		if err != nil {
			entry.Status = "anchor not found"
			entry.Where = p.file + " not found under -repo"
		} else {
			lines := splitLines(string(data))
			out, where, present, ok := p.insert(lines)
			switch {
			case !ok:
				entry.Status = "anchor not found"
				entry.Where = "no section for this package in " + p.file + " — add by hand"
			case present:
				entry.Status = "already present"
				entry.Where = where
			default:
				entry.Where = where
				if sc.apply {
					entry.Status = "applied"
					edits = append(edits, ledgerEdit{abs: abs, rel: p.file, content: joinLines(out, strings.HasSuffix(string(data), "\n"))})
				}
			}
		}
		if sc.apply && entry.Status == "anchor not found" {
			return nil, fmt.Errorf("-apply: %s (%s); nothing written", entry.Where, p.file)
		}
		res.Ledgers = append(res.Ledgers, entry)
	}

	return edits, nil
}

// trackingSection locates the TRACKING.md section a package belongs under:
// for foundations the "### <n>.<m> <category>" header whose title names the
// category (titles like "cost · cache · persistence · prompt" and
// "provenance + reasoning" carry several); for the other roots the
// "## <n>. <Root>" header. It returns the header index, the exclusive end
// (the next header or rule), and the header text.
func trackingSection(lines []string, root, category string) (start, end int, header string, ok bool) {
	match := func(line string) bool {
		if category != "" {
			if !strings.HasPrefix(line, "### ") {
				return false
			}
			for _, tok := range headerTitleTokens(strings.TrimPrefix(line, "### ")) {
				if tok == category {
					return true
				}
			}
			return false
		}
		if !strings.HasPrefix(line, "## ") {
			return false
		}
		toks := headerTitleTokens(strings.TrimPrefix(line, "## "))
		return len(toks) > 0 && strings.EqualFold(toks[0], root)
	}
	for i, line := range lines {
		if !match(line) {
			continue
		}
		end = len(lines)
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "## ") || strings.HasPrefix(lines[j], "### ") || strings.HasPrefix(lines[j], "---") {
				end = j
				break
			}
		}
		return i, end, strings.TrimSpace(line), true
	}
	return 0, 0, "", false
}

// headerNumberRe strips a leading "1.19 " / "2. " numbering from a header.
var headerNumberRe = regexp.MustCompile(`^\d+(\.\d+)*\.?\s+`)

// headerTitleTokens reduces a TRACKING.md header to its title tokens:
// numbering stripped, trailing status/description cut at " —", " ✅", " ⭐",
// " (", then split on " · " and " + ".
func headerTitleTokens(title string) []string {
	title = headerNumberRe.ReplaceAllString(title, "")
	for _, cut := range []string{" —", " ✅", " ⭐", " (", "—"} {
		if i := strings.Index(title, cut); i >= 0 {
			title = title[:i]
		}
	}
	title = strings.ReplaceAll(title, " + ", " · ")
	var toks []string
	for _, t := range strings.Split(title, " · ") {
		if t = strings.TrimSpace(t); t != "" {
			toks = append(toks, t)
		}
	}
	return toks
}

// markdownSection returns the index of the first line starting with header
// and the exclusive end: the next line starting with next (a "## "
// sibling), or the end of the file.
func markdownSection(lines []string, header, next string) (start, end int, ok bool) {
	for i, line := range lines {
		if !strings.HasPrefix(line, header) {
			continue
		}
		end = len(lines)
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], next) {
				end = j
				break
			}
		}
		return i, end, true
	}
	return 0, 0, false
}

// compatRowPath extracts the backtick-quoted package path from a
// docs/compat.md table row, mirroring constitution-lint's parseCompatRows
// (a row whose first cell has no "/" is a summary row, not a package).
func compatRowPath(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "| `") {
		return "", false
	}
	rest := line[len("| `"):]
	i := strings.Index(rest, "`")
	if i < 0 || !strings.Contains(rest[:i], "/") {
		return "", false
	}
	return rest[:i], true
}

// containsMarker reports whether any line carries the backtick-quoted path.
func containsMarker(lines []string, marker string) bool {
	for _, l := range lines {
		if strings.Contains(l, marker) {
			return true
		}
	}
	return false
}

// insertLines inserts line at index at; with spaced, a blank line follows
// when the displaced line is not already blank (bullet-list ledgers keep
// one blank between entries).
func insertLines(lines []string, at int, line string, spaced bool) []string {
	add := []string{line}
	if spaced && at < len(lines) && lines[at] != "" {
		add = append(add, "")
	}
	out := make([]string, 0, len(lines)+len(add))
	out = append(out, lines[:at]...)
	out = append(out, add...)
	out = append(out, lines[at:]...)
	return out
}

// splitLines splits on "\n" dropping the trailing empty element a final
// newline produces; joinLines is its inverse.
func splitLines(s string) []string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func joinLines(lines []string, trailingNewline bool) string {
	s := strings.Join(lines, "\n")
	if trailingNewline {
		s += "\n"
	}
	return s
}

// ── output ───────────────────────────────────────────────────────────────────

// printPackageResult writes the human report: files written, then each
// ledger line with its anchor and status, then the README bump and the
// catalog reminder.
func printPackageResult(w io.Writer, sc packageScaffold, res *packageResult) {
	d := res.Data
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	fmt.Fprintf(bw, "kit-new: scaffolded %s (package %s, type %s): %d files\n", d.Path, d.Pkg, d.Type, len(res.Files))
	for _, f := range res.Files {
		fmt.Fprintf(bw, "  %s\n", f)
	}
	fmt.Fprintln(bw)
	if sc.apply {
		fmt.Fprintln(bw, "kit-new: ledger lines (-apply):")
	} else {
		fmt.Fprintln(bw, "kit-new: ledger lines to add (re-run with -apply to append them):")
	}
	for _, l := range res.Ledgers {
		fmt.Fprintln(bw)
		fmt.Fprintf(bw, "%s — %s [%s]\n", l.File, l.Where, l.Status)
		fmt.Fprintln(bw, l.Line)
	}
	if sc.practice == 0 {
		fmt.Fprintln(bw)
		fmt.Fprintln(bw, "docs/practices.md — no -practice given, so no ledger entry is claimed; if the package enforces one, add its row to that section's table and update the STATUS line.")
	}
	fmt.Fprintln(bw)
	switch {
	case res.ReadmeStatus == "anchor not found":
		fmt.Fprintln(bw, "README.md — no \"**N Go packages\" sentence found [anchor not found]")
	case strings.HasPrefix(res.ReadmeStatus, "count unavailable"):
		fmt.Fprintf(bw, "README.md — package count [%s]\n", res.ReadmeStatus)
	default:
		fmt.Fprintf(bw, "README.md — source-counted package total [%s]\n%s → %s\n", res.ReadmeStatus, res.ReadmeFrom, res.ReadmeTo)
	}
	fmt.Fprintln(bw, "docs/catalog.md — regenerate: task docs-index")
	fmt.Fprintln(bw)
	fmt.Fprintf(bw, "kit-new: next: go test -race ./%s/ && task validate\n", d.Path)
}

// errPackageMode is returned by run when -package is combined with a
// consumer-mode flag; kept as a sentinel so the usage test can name it.
var errPackageMode = errors.New("-package is exclusive with -name, -dir, -module and -minimal")
