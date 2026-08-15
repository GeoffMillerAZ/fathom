package main

// derivedoutcome.go — DO-1 (docs/design/continuity-threads.md §4.1): a
// session's outcome is not a feeling; it is on disk. The v1 derivation is
// commits landed in the unit's own beat-window — deliberately narrow and
// stated as such everywhere it renders; proven-row deltas join it as the
// verify ledger (SD-1) accumulates history.

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// derivedCommitsForSession returns the number of commits landed in root
// during the session's beat-window. ok=false when the session has no
// stamped beats for that root (pre-IR-1 rows) or git cannot answer —
// absence, never a fabricated zero.
func derivedCommitsForSession(beats []retroBeat, sid, root string) (int, bool) {
	var start, end time.Time
	for _, b := range beats {
		if b.Meta["session"] != sid || b.Meta["project_dir"] != root {
			continue
		}
		if start.IsZero() || b.At.Before(start) {
			start = b.At
		}
		if b.At.After(end) {
			end = b.At
		}
	}
	if start.IsZero() {
		return 0, false
	}
	out, err := exec.Command("git", "-C", root, "rev-list", "--count",
		"--since="+start.UTC().Format(time.RFC3339),
		"--until="+end.Add(time.Minute).UTC().Format(time.RFC3339),
		"HEAD").Output()
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, false
	}
	return n, true
}
