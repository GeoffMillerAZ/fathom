package main

// derivedoutcome.go — DO-1 (docs/design/continuity-threads.md §4.1): a
// session's outcome is not a feeling; it is on disk. The v1 derivation is
// commits landed in the unit's own beat-window — deliberately narrow and
// stated as such everywhere it renders; proven-row deltas join it as the
// verify ledger (SD-1) accumulates history.

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
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
	return derivedCommitsInSpan(root, start, end)
}

// derivedCommitsInSpan counts the commits reachable from root's HEAD in
// [start, end+1m]. The answer is kept per (root, HEAD, span): a past
// window's count changes only when HEAD moves, and the console's model
// asks for every arm unit on every build — one git process per unit per
// build before (RT-115: no git fork per service on a page's path).
func derivedCommitsInSpan(root string, start, end time.Time) (int, bool) {
	if start.IsZero() {
		return 0, false
	}
	head := repoHeadNoFork(root)
	key := strings.Join([]string{root, head, start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano)}, "\x00")
	now := derivedCommitsNow()
	if head != "" {
		derivedCommitsMemo.Lock()
		v, ok := derivedCommitsMemo.m[key]
		derivedCommitsMemo.Unlock()
		// An answer stands until HEAD moves; a failure only for
		// consolePlansRetry (an EPERM wedge clears — I38).
		if ok && (v.ok || now.Sub(v.at) < consolePlansRetry) {
			return v.n, v.ok
		}
	}
	// The model's rebuild is one flight: a git that does not answer within
	// consoleGitTimeout is stopped, never held (RT-115).
	ctx, cancel := context.WithTimeout(context.Background(), consoleGitTimeout)
	defer cancel()
	out, err := gitro.Output(ctx, root, "rev-list", "--count",
		"--since="+start.UTC().Format(time.RFC3339),
		"--until="+end.Add(time.Minute).UTC().Format(time.RFC3339),
		"HEAD")
	n, ok := 0, false
	if err == nil {
		if v, perr := strconv.Atoi(strings.TrimSpace(string(out))); perr == nil {
			n, ok = v, true
		}
	}
	if head != "" && ctx.Err() == nil && (ok || err != nil && !isExecNotFound(err)) {
		derivedCommitsMemo.Lock()
		if derivedCommitsMemo.m == nil || len(derivedCommitsMemo.m) >= derivedCommitsMemoMax {
			derivedCommitsMemo.m = map[string]derivedCount{}
		}
		derivedCommitsMemo.m[key] = derivedCount{n, ok, now}
		derivedCommitsMemo.Unlock()
	}
	return n, ok
}

type derivedCount struct {
	n  int
	ok bool
	at time.Time
}

// derivedCommitsNow is the memo's clock (tests pin it).
var derivedCommitsNow = time.Now

// derivedCommitsMemoMax bounds the kept counts (one per arm unit window).
const derivedCommitsMemoMax = 4096

var derivedCommitsMemo struct {
	sync.Mutex
	m map[string]derivedCount
}

// isExecNotFound is a git that could not be started (not an answer).
func isExecNotFound(err error) bool {
	var ee *exec.Error
	return errors.As(err, &ee)
}
