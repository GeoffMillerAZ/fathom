package main

// observe.go — FW-2 (docs/plans/pack-taxonomy-adoption.md): `fathom
// observe`, the flywheel's write path. Structured friction capture —
// symptom (required), plus tool/expected/actual/repro — appended to
// .fathom/observations.md in the dogfood-observations entry shape
// (observations.go). Agents are the primary users: the fathom-feedback
// skill (AK-track) teaches an agent to file one the moment it hits
// friction, so this command must be safe to call repeatedly and must never
// corrupt or silently reinterpret a file it didn't create.
//
// Concurrency contract (critic B1/H1 on the FW slice). Multiple agent
// sessions filing friction at once is the flywheel's HEADLINE scenario, so:
//
//   - Entries are never lost. First-time file creation uses
//     O_CREATE|O_EXCL, so exactly one concurrent observe wins creation;
//     the losers detect EEXIST and fall through to the normal
//     marker-check + append path (O_APPEND writes are atomic per entry).
//     The pre-fix os.WriteFile (O_TRUNC) here let a concurrent second
//     observe clobber the first's entry with both exiting 0 — silent data
//     loss, proven 13/20 trials.
//   - Entry IDs are ADVISORY under concurrency, by documented choice
//     (H1's accepted alternative to a lock): the O<N> id is scanned from
//     the file at write time without a lock, so two concurrent observes
//     can both mint the same O<N>. Nothing is lost — both entries land in
//     full — and because the file is append-only, FILE ORDER is
//     chronological: disambiguate colliding ids by their order in the
//     file (later entry = later filing). A human renumbers at triage time
//     if it matters; a cross-process flock is the upgrade path if
//     advisory ids ever prove insufficient.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	//
	// BUT a markerless read can also be a concurrent winner's create→write
	// window (B1): O_CREATE|O_EXCL makes the path visible before the single
	// header write lands, so this initial read races it exactly like a lost
	// O_EXCL create does. Wait that window out before refusing — only a file
	// whose marker never appears is genuinely foreign.
	if existed && !strings.Contains(content, observationsMarker) {
		reread, rerr := reReadAfterLostCreateRace(*root)
		if rerr != nil {
			fmt.Fprintf(stderr,
				"error: %s already exists but is not a fathom-observations file (missing the %q marker) — refusing to append; move it aside first if it should become one\n",
				path, observationsMarker)
			return 1
		}
		content = reread
	}

	if !existed {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(stderr, "error: creating %s: %v\n", filepath.Dir(path), err)
			return 1
		}
		// B1: O_CREATE|O_EXCL — exactly one concurrent observe wins
		// first-time creation; every other one gets EEXIST and falls
		// through to the append path below instead of clobbering the
		// winner's file with os.WriteFile's O_TRUNC.
		f, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		switch {
		case createErr == nil:
			entry := formatObservationEntry(1, *symptom, *tool, *expected, *actual, *repro)
			if _, werr := f.WriteString(observationsHeader + "\n" + entry); werr != nil {
				f.Close()
				fmt.Fprintf(stderr, "error: writing %s: %v\n", path, werr)
				return 1
			}
			if cerr := f.Close(); cerr != nil {
				fmt.Fprintf(stderr, "error: writing %s: %v\n", path, cerr)
				return 1
			}
			fmt.Fprintf(stdout, "recorded O1 in %s\n", path)
			return 0
		case os.IsExist(createErr):
			// Lost the creation race: another observe created the file
			// between our read and our create. Re-read (waiting out the
			// winner's create→write window) and fall through to the
			// normal marker-checked append path.
			content, err = reReadAfterLostCreateRace(*root)
			if err != nil {
				fmt.Fprintf(stderr, "error: %s appeared concurrently but %v — refusing to append; move it aside first if it should become a fathom-observations file\n", path, err)
				return 1
			}
		default:
			fmt.Fprintf(stderr, "error: creating %s: %v\n", path, createErr)
			return 1
		}
	}

	id := nextObservationID(content)
	entry := formatObservationEntry(id, *symptom, *tool, *expected, *actual, *repro)

	// O_APPEND, never O_TRUNC/O_RDWR-with-seek: existing bytes are never
	// touched, only ever added to — the append-only contract.
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

	fmt.Fprintf(stdout, "recorded O%d in %s\n", id, path)
	return 0
}

// reReadAfterLostCreateRace re-reads the observations file after this
// process lost the O_EXCL creation race. The winner's O_CREATE|O_EXCL makes
// the path visible BEFORE its single header+entry write lands, so a loser
// that re-reads immediately can see an empty (or partial) file whose marker
// hasn't been written yet — indistinguishable, at that instant, from a
// foreign file. Retry briefly until the marker appears; if it never does,
// the file genuinely isn't ours and the caller refuses, same as the
// pre-existing-malformed-file case.
func reReadAfterLostCreateRace(root string) (string, error) {
	deadline := 40 // × 5ms = 200ms, far beyond a single small write's latency
	for i := 0; i < deadline; i++ {
		content, existed, err := readObservations(root)
		if err != nil {
			return "", err
		}
		if existed && strings.Contains(content, observationsMarker) {
			return content, nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return "", fmt.Errorf("its %q marker never appeared", observationsMarker)
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
