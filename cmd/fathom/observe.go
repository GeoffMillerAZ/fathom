package main

// observe.go — FW-2: `fathom observe`, the flywheel's write path.
// Structured friction capture — symptom (required), plus
// tool/expected/actual/repro — written as one entry per file,
// .fathom/observations/O<n>.md, in the dogfood-observations entry shape
// (observations.go). Agents are the primary users: the fathom-feedback
// skill teaches an agent to file one the moment it hits friction, so this
// command must be safe to call repeatedly and concurrently.
//
// Concurrency contract (EW-32, superseding the flock-on-one-log design).
// Multiple agent sessions filing friction at once is the flywheel's
// HEADLINE scenario, and parallel waves add entries to the same repo, so
// each entry is its own file created with O_CREATE|O_EXCL: exactly one
// writer wins a given id, a loser moves to the next id, and no existing
// file is ever opened for writing — entries are never lost or clobbered,
// ids never collide, and the legacy .fathom/observations.md log is read
// but never written.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// cmdObserve implements the "observe" subcommand.
func cmdObserve(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("observe", stderr)
	symptom := fs.String("symptom", "", "required: what went wrong or surprised you, using a fathom tool/command")
	tool := fs.String("tool", "", "the tool or command involved")
	expected := fs.String("expected", "", "what you expected to happen")
	actual := fs.String("actual", "", "what actually happened")
	repro := fs.String("repro", "", "how to reproduce it (a verbatim command/call is ideal)")
	root := fs.String("root", ".", "project root — .fathom/observations/ lives under here")
	if err := fs.Parse(args); err != nil {
		return 0
	}
	if strings.TrimSpace(*symptom) == "" {
		fmt.Fprintln(stderr, "error: --symptom is required")
		fs.Usage()
		return 1
	}

	id, path, mErr := mintObservationEntry(*root, *symptom, *tool, *expected, *actual, *repro)
	if mErr != nil {
		fmt.Fprintf(stderr, "error: recording observation: %v\n", mErr)
		return 1
	}
	fmt.Fprintf(stdout, "recorded O%d in %s\n", id, path)
	return 0
}

// mintObservationEntry mints the next O<N> id over the legacy log and the
// per-entry directory together and writes the entry to its own file with
// an exclusive create (EW-32). An id another observe took between our read
// and our create (EEXIST) moves to the next id; no existing file is ever
// opened for writing, so no entry can be lost or clobbered and the legacy
// log is never touched.
func mintObservationEntry(root, symptom, tool, expected, actual, repro string) (int, string, error) {
	dir := observationsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, "", fmt.Errorf("creating %s: %w", dir, err)
	}
	// The README carries the header; losing its create race is fine.
	if f, err := os.OpenFile(filepath.Join(dir, "README.md"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644); err == nil {
		_, werr := f.WriteString(observationsHeader)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return 0, "", fmt.Errorf("writing %s: %w", f.Name(), werr)
		}
	}
	content, _, err := readObservations(root)
	if err != nil {
		return 0, "", fmt.Errorf("reading observations: %w", err)
	}
	id := nextObservationID(content)
	for {
		path := filepath.Join(dir, fmt.Sprintf("O%d.md", id))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if os.IsExist(err) {
			id++
			continue
		}
		if err != nil {
			return 0, "", fmt.Errorf("creating %s: %w", path, err)
		}
		_, werr := f.WriteString(formatObservationEntry(id, symptom, tool, expected, actual, repro))
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return 0, "", fmt.Errorf("writing %s: %w", path, werr)
		}
		return id, path, nil
	}
}

// formatObservationEntry renders one entry in the dogfood-observations
// shape (docs/dogfood-observations.md): symptom → evidence → root cause →
// fix → status → impact. Root cause/fix/impact are always TODO — an agent
// filing an observation at the moment of friction rarely knows them yet;
// status defaults "open". Optional fields (tool/expected/actual/repro)
// that weren't given are marked TODO too, rather than left blank, so a
// human or agent scanning the file later sees "not yet supplied" instead
// of an ambiguous empty line.
func formatObservationEntry(id int, symptom, tool, expected, actual, repro string) string {
	td := func(s string) string {
		if strings.TrimSpace(s) == "" {
			return "TODO"
		}
		return s
	}
	var b strings.Builder
	fmt.Fprintf(&b, "### O%d — %s\n\n", id, symptom)
	fmt.Fprintf(&b, "- **Symptom:** %s\n", symptom)
	fmt.Fprintln(&b, "- **Evidence:**")
	fmt.Fprintf(&b, "  - **Tool:** %s\n", td(tool))
	fmt.Fprintf(&b, "  - **Expected:** %s\n", td(expected))
	fmt.Fprintf(&b, "  - **Actual:** %s\n", td(actual))
	fmt.Fprintf(&b, "  - **Repro:** %s\n", td(repro))
	fmt.Fprintln(&b, "- **Root cause:** TODO")
	fmt.Fprintln(&b, "- **Fix:** TODO")
	fmt.Fprintln(&b, "- **Status:** open")
	fmt.Fprintln(&b, "- **Impact:** TODO")
	return b.String()
}
