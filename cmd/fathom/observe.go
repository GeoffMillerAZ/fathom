package main

// observe.go — FW-2 (docs/plans/pack-taxonomy-adoption.md): `fathom
// observe`, the flywheel's write path. Structured friction capture —
// symptom (required), plus tool/expected/actual/repro — appended to
// .fathom/observations.md in the dogfood-observations entry shape
// (observations.go). Agents are the primary users: the fathom-feedback
// skill (AK-track) teaches an agent to file one the moment it hits
// friction, so this command must be safe to call repeatedly and must never
// corrupt or silently reinterpret a file it didn't create.

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
	root := fs.String("root", ".", "project root — .fathom/observations.md lives under here")
	if err := fs.Parse(args); err != nil {
		return 0
	}
	if strings.TrimSpace(*symptom) == "" {
		fmt.Fprintln(stderr, "error: --symptom is required")
		fs.Usage()
		return 1
	}

	path := observationsPath(*root)
	content, existed, err := readObservations(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading %s: %v\n", path, err)
		return 1
	}
	// Malformed-existing-file refusal: a pre-existing file at this path
	// that doesn't carry fathom's own marker is not ours to append into —
	// refusing here (before any write) is what "never clobber" means in
	// practice, not just in a doc comment.
	if existed && !strings.Contains(content, observationsMarker) {
		fmt.Fprintf(stderr,
			"error: %s already exists but is not a fathom-observations file (missing the %q marker) — refusing to append; move it aside first if it should become one\n",
			path, observationsMarker)
		return 1
	}

	id := nextObservationID(content)
	entry := formatObservationEntry(id, *symptom, *tool, *expected, *actual, *repro)

	if !existed {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(stderr, "error: creating %s: %v\n", filepath.Dir(path), err)
			return 1
		}
		full := observationsHeader + "\n" + entry
		if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
			fmt.Fprintf(stderr, "error: writing %s: %v\n", path, err)
			return 1
		}
	} else {
		// O_APPEND, never O_TRUNC/O_RDWR-with-seek: existing bytes are
		// never touched, only ever added to — the append-only contract.
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "error: opening %s: %v\n", path, err)
			return 1
		}
		defer f.Close()
		if _, err := f.WriteString("\n" + entry); err != nil {
			fmt.Fprintf(stderr, "error: appending to %s: %v\n", path, err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "recorded O%d in %s\n", id, path)
	return 0
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
