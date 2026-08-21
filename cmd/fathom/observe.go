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
//   - Entry IDs are minted under a cross-process flock (IT-10 — the upgrade
//     path this file's own prior comment pre-approved): the O<N> id scan
//     and its append both happen inside one LOCK_EX/UNLOCK critical
//     section, re-reading the file's true current content under the lock
//     rather than trusting whatever was read before it was acquired — so
//     two concurrent observes cannot mint the same O<N> (leases.go took
//     the same upgrade for .fathom/leases.jsonl, for the identical
//     reason: an id/row that must never collide is exactly what O_APPEND
//     alone does not guarantee).

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

	id, mErr := mintAndAppendObservation(path, *symptom, *tool, *expected, *actual, *repro)
	if mErr != nil {
		fmt.Fprintf(stderr, "error: appending to %s: %v\n", path, mErr)
		return 1
	}

	fmt.Fprintf(stdout, "recorded O%d in %s\n", id, path)
	return 0
}

// mintAndAppendObservation mints the next O<N> id and appends its entry
// inside a single cross-process flock critical section (IT-10): the id
// scan re-reads the file's CURRENT content under the lock — never the
// possibly-stale content the caller read before acquiring it — so two
// concurrent observe processes racing this call cannot both mint the same
// id. The lock is held on the observations file itself, matching
// leases.go's own flock-per-target-file idiom; O_RDWR|O_CREATE (never
// O_TRUNC) leaves existing bytes untouched, only ever adding to them.
func mintAndAppendObservation(path, symptom, tool, expected, actual, repro string) (int, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return 0, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return 0, fmt.Errorf("locking %s: %w", path, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	raw, err := io.ReadAll(f)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}
	id := nextObservationID(string(raw))
	entry := formatObservationEntry(id, symptom, tool, expected, actual, repro)

	// ReadAll above already left the fd positioned at EOF; Seek is
	// belt-and-suspenders explicitness, not load-bearing.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return 0, fmt.Errorf("seeking %s: %w", path, err)
	}
	if _, err := f.WriteString("\n" + entry); err != nil {
		return 0, fmt.Errorf("writing %s: %w", path, err)
	}
	return id, nil
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
