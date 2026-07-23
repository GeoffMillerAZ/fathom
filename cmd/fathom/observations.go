package main

// observations.go — the .fathom/observations.md format shared by `fathom
// observe` (FW-2, docs/plans/pack-taxonomy-adoption.md: the write path) and
// `fathom instrument --adoption` (FW-1: the read path, open-observation
// count). One project-local friction log, append-only, in the
// dogfood-observations entry shape (docs/dogfood-observations.md: symptom →
// evidence → root cause → fix → status → impact) scoped to a single
// adopting repo instead of the fleet-wide doc this project keeps for
// itself.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// observationsRelPath is the project-relative path both FW-1 and FW-2
// resolve against --root.
const observationsRelPath = ".fathom/observations.md"

// observationsMarker identifies a file `fathom observe` owns: present in
// the header cmdObserve writes when creating a fresh file, and checked
// before appending to a PRE-EXISTING file at the same path. Its absence
// from an existing file is the load-bearing signal that the file is not
// fathom's — `fathom observe` refuses to append into it rather than risk
// corrupting whatever it actually is (the "malformed-existing-file
// refusal" FW-2 requires).
const observationsMarker = "<!-- fathom-observations:v1 -->"

// observationsHeader is written once, the first time `fathom observe` runs
// in a project with no .fathom/observations.md yet. It explains the
// flywheel loop this file feeds (docs/plans/pack-taxonomy-adoption.md):
// observe → root-cause → fix → close, mirroring this repo's own
// docs/dogfood-observations.md discipline, scoped to one adopting project.
// Ends mid-"## Open" section so the first entry appends directly beneath
// it, exactly like every later entry.
const observationsHeader = observationsMarker + `
# Fathom observations

A project-local friction log, fed by ` + "`fathom observe`" + ` — the flywheel's
intake (agents are the primary users, so agent-filed observations are the
flywheel's fuel; docs/plans/pack-taxonomy-adoption.md). Entries graduate
into fixes (and a CHANGELOG entry) or are closed with a recorded reason;
` + "`fathom instrument --adoption`" + ` reports how many are still open.

Entry shape: **symptom → evidence → root cause → fix → status → impact**
(docs/dogfood-observations.md's own convention, scoped to this repo).
Append-only: ` + "`fathom observe`" + ` only ever adds a new entry at the end —
correct an entry by hand (root cause / fix / status) once it's understood,
never by re-running the command over it.

## Open
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

// readObservations reads the observations file under root. ok is false
// when the file does not exist yet — a legitimate "no observations filed
// yet" state, not an error. A genuine read failure (permissions, a
// directory at that path, …) is returned as err.
func readObservations(root string) (content string, ok bool, err error) {
	data, err := os.ReadFile(observationsPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
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
