package main

// observations.go — the observation store shared by `fathom observe` (the
// write path: one exclusive-create .fathom/observations/O<n>.md per entry,
// EW-32) and every reader (`fathom instrument --adoption`, the fleet
// surfaces, the research report and bundle). readObservations merges the
// legacy .fathom/observations.md log, still read and never written, with
// the per-entry directory. Entry shape: symptom → evidence → root cause →
// fix → status → impact, scoped to a single adopting repo.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// observationsRelPath is the project-relative path both FW-1 and FW-2
// resolve against --root.
const observationsRelPath = projectdir.Dir + "/" + string(projectdir.ObservationLog)

// observationsDirRelPath is the per-entry directory: one O<n>.md per
// observation, so parallel writers add files instead of editing one log.
const observationsDirRelPath = projectdir.Dir + "/" + string(projectdir.Observations)

// observationEntryFile matches a per-entry file name.
var observationEntryFile = regexp.MustCompile(`^O(\d+)\.md$`)

// observationsMarker identifies a file `fathom observe` owns: present in
// the header cmdObserve writes when creating a fresh file, and checked
// before appending to a PRE-EXISTING file at the same path. Its absence
// from an existing file is the load-bearing signal that the file is not
// fathom's — `fathom observe` refuses to append into it rather than risk
// corrupting whatever it actually is (the "malformed-existing-file
// refusal" FW-2 requires).
const observationsMarker = "<!-- fathom-observations:v1 -->"

// observationsHeader is written once, the first time `fathom observe` runs
// in a project with no .fathom/observations/README.md yet. It explains the
// flywheel loop this directory feeds: observe → root-cause → fix → close.
// It names no repo-specific path (O291): every adopting repo reads it.
const observationsHeader = observationsMarker + `
# Fathom observations

A project-local friction log, fed by ` + "`fathom observe`" + ` — the flywheel's
intake (agents are the primary users, so agent-filed observations are the
flywheel's fuel). Entries graduate into fixes (and a CHANGELOG entry) or are
closed with a recorded reason; ` + "`fathom instrument --adoption`" + ` reports how
many are still open.

Entry shape: **symptom → evidence → root cause → fix → status → impact**.
One entry per file: ` + "`fathom observe`" + ` creates ` + "`O<n>.md`" + ` here with an
exclusive create and never touches an existing entry — correct an entry by
hand (root cause / fix / status) once it's understood, never by re-running
the command over it. A legacy ` + "`.fathom/observations.md`" + ` log, when a repo
has one, is still read alongside this directory and is never written.

Entry ids (O1, O2, …) are minted over the legacy log and this directory
together; an id already taken is skipped, so two sessions filing at once
never collide and no entry is lost.
`

// observationEntryHeader matches an entry's own "### O<N> — <title>" line.
var observationEntryHeader = regexp.MustCompile(`(?m)^### O(\d+)\b`)

// observationStatusLine matches a "- **Status:** <value>" line anywhere in
// an entry's body (docs/dogfood-observations.md's convention writes it as
// a prefix, e.g. "open; documented v0 trade-off." — matched as a prefix
// below, not an exact string).
var observationStatusLine = regexp.MustCompile(`(?m)^- \*\*Status:\*\*\s*(.+)$`)

// observationsPath resolves the observations file path under root.
func observationsPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(observationsRelPath))
}

// observationsDir resolves the per-entry directory under root.
func observationsDir(root string) string {
	return filepath.Join(root, filepath.FromSlash(observationsDirRelPath))
}

// readObservations is the one merged reader: the legacy log (when present)
// followed by every O<n>.md in the per-entry directory, in id order. ok is
// false only when neither exists — a legitimate "no observations filed
// yet" state, not an error. A genuine read failure (permissions, a TCC
// wedge, a directory at the legacy path, …) is returned as err, and an
// unreadable entry file is NAMED in it — never skipped and folded into a
// smaller total (I38).
func readObservations(root string) (content string, ok bool, err error) {
	var b strings.Builder
	data, err := os.ReadFile(observationsPath(root))
	switch {
	case err == nil:
		ok = true
		b.Write(data)
	case !os.IsNotExist(err):
		return "", false, err
	}
	dir := observationsDir(root)
	des, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return b.String(), ok, nil
		}
		return "", false, err
	}
	type entry struct {
		id   int
		name string
	}
	var entries []entry
	for _, de := range des {
		m := observationEntryFile.FindStringSubmatch(de.Name())
		if m == nil || de.IsDir() {
			continue
		}
		n, aerr := strconv.Atoi(m[1])
		if aerr != nil {
			continue
		}
		entries = append(entries, entry{n, de.Name()})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].id != entries[j].id {
			return entries[i].id < entries[j].id
		}
		return entries[i].name < entries[j].name
	})
	for _, e := range entries {
		ed, rerr := os.ReadFile(filepath.Join(dir, e.name))
		if rerr != nil {
			return "", false, fmt.Errorf("observation entry %s unreadable: %w", filepath.Join(dir, e.name), rerr)
		}
		ok = true
		b.WriteString("\n")
		b.Write(ed)
	}
	return b.String(), ok, nil
}

// serviceOpenObservations reads one service root's open-observation count
// for the fleet surfaces (the fleet report, the observatory model): the
// count when the file was read, nil with no error when there is no file
// (the repo does not report), and nil WITH the read error when the file
// exists and could not be read — a TCC wedge or a permission defect, which
// must be named, never shown as "no observations file" or folded into a
// smaller total (I38).
func serviceOpenObservations(root string) (n *int, errNote string) {
	if root == "" {
		return nil, ""
	}
	content, existed, err := readObservations(root)
	switch {
	case err != nil:
		return nil, err.Error() + tccWedgeHint(observationsPath(root))
	case !existed:
		return nil, ""
	}
	c := countOpenObservations(content)
	return &c, ""
}

// nextObservationID scans content for the highest existing "### O<N>"
// entry header and returns N+1 (1 if content carries no entries yet).
// Entry IDs auto-increment by scanning the file itself, never a stored
// counter, so there is nothing to desync from the file's actual contents.
func nextObservationID(content string) int {
	max := 0
	for _, m := range observationEntryHeader.FindAllStringSubmatch(content, -1) {
		n, err := strconv.Atoi(m[1])
		if err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// countOpenObservations reports how many entries' Status line starts with
// "open" (case-insensitive prefix match — see observationStatusLine's doc
// comment). Splitting on entry headers first means each entry's own status
// is counted from its own body, never from another entry's or the file's
// front-matter prose.
func countOpenObservations(content string) int {
	sections := observationEntryHeader.Split(content, -1)
	open := 0
	for _, sec := range sections {
		matches := observationStatusLine.FindAllStringSubmatch(sec, -1)
		if len(matches) == 0 {
			continue
		}
		last := strings.TrimSpace(matches[len(matches)-1][1])
		if strings.HasPrefix(strings.ToLower(last), "open") {
			open++
		}
	}
	return open
}
