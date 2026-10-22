// discoveries_observed.go — RC-56 (docs/vision/28-receipts.md §4, "every
// discovery action counts"; docs/plans/receipts.md wave 2 addendum).
//
// reuse-pilot-2 found the wired planners reaching for grep and Read, not
// the index: the MCP call log held no pointers and every worker brief was
// empty. cmd/fathom/discovery.go's PreToolUse observer (instrument B)
// already sees every Grep/Glob/Read/shell-search call and counts it into
// a beat — it never looked at WHAT the call named. This file extends that
// SAME counting-only, zero-daemon path to also record the call's target
// as a discovery pointer, so a planner that only grepped still hands its
// workers a brief.
//
// Paths are repo-relative. RangePointers additionally snapshots a read span
// and its hash at observation, so the worker brief can verify it locally.
// The legacy Pointers list serves the discovery counters. RangePointers is
// the handoff evidence: a read's local span and content hash, captured with
// the session and agent identity. Neither capture nor verification needs an
// index lookup. Shell targets must resolve on disk before they are recorded.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/discoveryptr"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// observedDiscoverySchemaVersion identifies observed-discoveries.jsonl's
// row shape.
const observedDiscoverySchemaVersion = 1

// observedDiscoveryRow is one JSONL row of workspace/logs/
// observed-discoveries.jsonl — the observer's own mechanical fact ("this
// call named this location"), never a model's summary of one.
type observedDiscoveryRow struct {
	SchemaVersion int                    `json:"schema_version"`
	TS            string                 `json:"ts"` // RFC3339Nano, matches the MCP call log's own ts format
	SessionID     string                 `json:"session_id,omitempty"`
	Service       string                 `json:"service,omitempty"`
	AgentID       string                 `json:"agent_id,omitempty"`
	RangePointers []discoveryptr.Pointer `json:"range_pointers,omitempty"`
	Cwd           string                 `json:"cwd,omitempty"`
	Tool          string                 `json:"tool"` // "read" | "grep" | "glob" — the verb gatherDiscoveryPointers renders an observed-only pointer with
	Pointers      []string               `json:"pointers"`
}

// observedDiscoveryEntry is observedDiscoveryRow parsed back for merging:
// TS as a time.Time, everything else unchanged.
type observedDiscoveryEntry struct {
	TS            time.Time
	SessionID     string
	Service       string
	AgentID       string
	RangePointers []discoveryptr.Pointer
	Tool          string
	Pointers      []string
}

// ─── extraction (item 1) ─────────────────────────────────────────────────

// observedDiscoveryPointers extracts the discovery pointer(s) instrument
// B's counting-only observer already sees in one PreToolUse call: for a
// Read, in.FilePath, repo-relative; for a Grep/Glob, in.Path (tool_input's
// own "path" argument — never in.Command's search pattern, which names no
// location) when it was actually supplied; for a Bash call, the literal
// trailing path of a recognized single-target read/search command
// (bashObservedReadPath). Every other shape, or a path that does not
// resolve under any known service root, yields nil — never a wrong guess.
//
// Zero daemon calls, zero index lookups: this stays on the observer's own
// local-only, never-blocking path (mirrors fileInEditSet's resolution
// exactly, which this hook already calls unconditionally).
func observedDiscoveryPointers(in harness.HookInput) []string {
	switch in.Tool {
	case harness.ToolDiscoveryRead:
		if p, ok := observedRepoRelPath(in.FilePath, in.Cwd); ok {
			return []string{p}
		}
	case harness.ToolDiscoveryGrep, harness.ToolDiscoveryGlob:
		if in.Path == "" {
			return nil // no path argument named — not "the whole repo", just nothing to point at
		}
		if p, ok := observedRepoRelPath(in.Path, in.Cwd); ok {
			return []string{p}
		}
	case harness.ToolShell:
		if target, _ := bashObservedReadPath(in.Command, in.Cwd); target != "" && discoveryptr.PathLike(target) {
			cwd, _ := hookEffectiveDir(in.Command, "", in.Cwd)
			if p, ok := observedRepoRelPath(resolveBashDir(cwd, target), cwd); ok {
				return []string{p}
			}
		}
	}
	return nil
}

// discoveryToolLabel names the observed-discoveries.jsonl row's "tool"
// field for in — discoveryBeatName's own vocabulary (read/grep/glob) for
// the three discovery-class tools, or bashObservedReadPath's finer
// classification (a Bash "cat"/"sed -n"/"head"/"tail" is a read, an
// "rg"/"grep" is a search) for a shell call. "" when in names no
// recognized discovery action at all.
func discoveryToolLabel(in harness.HookInput) string {
	if in.Tool == harness.ToolShell {
		_, tool := bashObservedReadPath(in.Command, in.Cwd)
		return tool
	}
	return discoveryBeatName(in.Tool)
}

// observedRepoRelPath resolves a tool's target relative to its cwd, then
// expresses it relative to the same repository root used for range capture.
// A subdirectory's basename must never point at an unrelated root-level file.
func observedRepoRelPath(p, cwd string) (string, bool) {
	if p == "" {
		return "", false
	}
	if !filepath.IsAbs(p) {
		a, err := filepath.Abs(filepath.Join(cwd, p))
		if err != nil {
			return "", false
		}
		p = a
	}
	// Always use the same root as range capture and handoff rendering. A
	// lookup from a subdirectory must not become a pointer into the root's
	// unrelated file with the same basename.
	root := hookMapRoot(resolveProjDir(cwd))
	return relativeToRoot(root, p)
}

// bashObservedReadKind classifies which discovery verb a recognized shell
// command implies — "read" for a single-file content dump (cat/sed -n/
// head/tail), "grep" for a search across a path (rg/grep) — the same
// read-vs-search distinction discoveryBeatName already keeps Read and
// Grep apart for (docs/design/production-tokenomics.md §4's honesty
// constraint: reading what you already located is not the same act as
// finding it).
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var bashObservedReadKind = map[string]string{
	"cat": "read", "head": "read", "tail": "read", "sed": "read",
	"rg": "grep", "grep": "grep",
}

// bashObservedReadPath returns the literal trailing path argument of a
// Bash command recognizable as one of bashObservedReadKind's binaries,
// plus the discovery tool label ("read" or "grep") it implies, read through
// the one command lexer (RO-4). "", "" when command is not recognizable: a
// pipe, a redirect, a substitution, a glob or a variable disqualifies the
// whole command (the result is not that one target's raw content, or the
// target is not written out), as does a trailing flag or too few words. A
// command the lexer cannot read records no read; the beat that counts it as
// unreadable is emitDiscoveryBeatIfShellSearch (RO-29).
func bashObservedReadPath(command string, roots ...string) (target, tool string) {
	cmd := parseShellCommand(command, roots...).Transparent()
	if cmd.Unreadable || len(cmd.Pipelines) != 1 || len(cmd.Pipelines[0].Units) != 1 {
		return "", ""
	}
	u := &cmd.Pipelines[0].Units[0]
	if u.Kind != shellcmd.KindSimple || len(u.Redirects) > 0 || len(u.Subst) > 0 || len(u.Words) < 2 {
		return "", ""
	}
	for _, w := range u.Words {
		if !w.Literal() {
			return "", ""
		}
	}
	bin := u.Name()
	kind, recognized := bashObservedReadKind[bin]
	if !recognized {
		return "", ""
	}
	last := u.Words[len(u.Words)-1].Text
	if last == "" || strings.HasPrefix(last, "-") {
		return "", "" // a trailing flag, not a path
	}
	if bin == "sed" && (len(u.Words) < 4 || u.Words[1].Text != "-n") {
		// sed's read shape is specifically "sed -n <script> file" (the
		// RC-11 4-field floor) — a bare "sed -n file" has no script and
		// sed would read it as one, not as a path.
		return "", ""
	}
	return last, kind
}

// ─── write path (item 2) ──────────────────────────────────────────────────

// observedDiscoveriesLogPathFor is logs/observed-discoveries.jsonl under
// root's state directory (statedir.Resolve — outside the repo tree by
// default, O63 item 5) — the same convention sessionsLogPathFor and
// callLogPathFor already use. FATHOM_OBSERVED_DISCOVERIES_LOG overrides it
// directly (tests) — it wins over even FATHOM_STATE_DIR.
func observedDiscoveriesLogPathFor(root string) string {
	if p := os.Getenv("FATHOM_OBSERVED_DISCOVERIES_LOG"); p != "" {
		return p
	}
	base := root
	// By stat, never a git fork: the Read|Grep|Glob observer resolves this
	// on every discovery call (RT-131); readers resolve it the same way.
	if top := gitToplevelByStat(root); top != "" {
		base = top
	}
	return filepath.Join(statedir.Resolve(base), "logs", "observed-discoveries.jsonl")
}

// observedDiscoveryRotateBytes bounds the observed-discoveries log's
// on-disk size (RC-56: "keep the file under ~2 MB") — a package var, not
// a const, so tests can shrink it rather than writing a real 2MB fixture.
var observedDiscoveryRotateBytes int64 = 2 << 20

// rotateObservedDiscoveriesIfNeeded renames path to path+".1" (overwriting
// any prior backup) when it exceeds observedDiscoveryRotateBytes, mirroring
// beats.go's rotateBeatLogIfNeeded exactly: one bounded prior generation,
// not a full archive. Best-effort: any failure (including "no file yet")
// is silent.
func rotateObservedDiscoveriesIfNeeded(path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() < observedDiscoveryRotateBytes {
		return
	}
	_ = os.Rename(path, path+".1")
}

// recordObservedDiscovery appends row to root's observed-discoveries log
// under a cross-process flock — recordSessionWindow's own pattern
// (discoveries.go), since an observed-discovery row must never interleave
// with a concurrent hook's write. Best-effort and silent on any failure —
// a hook must never fail a tool call over a missing log directory.
func recordObservedDiscovery(root string, row observedDiscoveryRow) {
	if len(row.Pointers) == 0 {
		return
	}
	logPath := observedDiscoveriesLogPathFor(root)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return
	}
	rotateObservedDiscoveriesIfNeeded(logPath)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	line, err := json.Marshal(row)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
}

// recordObservedDiscoveryIfAny is cmdEditHookPreDiscovery's (and the Bash
// branch's) write-path call: extract, and record only when the call
// actually named a pointer — a Grep with no path, a Bash pipeline, an
// unresolvable target, all leave the log untouched.
func recordObservedDiscoveryIfAny(in harness.HookInput) {
	pointers := observedDiscoveryPointers(in)
	root := hookMapRoot(resolveProjDir(in.Cwd))
	if in.Tool == harness.ToolShell {
		var existing []string
		for _, p := range pointers {
			real, err := filepath.EvalSymlinks(filepath.Join(root, p))
			if err != nil {
				continue
			}
			if _, ok := relativeToRoot(resolveSymlinks(root), real); ok {
				existing = append(existing, p)
			}
		}
		pointers = existing
	}
	if len(pointers) == 0 {
		return
	}
	recordObservedDiscovery(resolveProjDir(in.Cwd), observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion,
		TS:            nowFunc().UTC().Format(time.RFC3339Nano),
		SessionID:     in.SessionID,
		AgentID:       in.AgentID,
		RangePointers: observedRangePointers(in, pointers),
		Cwd:           in.Cwd,
		Tool:          discoveryToolLabel(in),
		Pointers:      pointers,
	})
}

// ─── read path (item 3) ────────────────────────────────────────────────────

// readObservedDiscoveryRows reads root's observed-discoveries log (rows at
// or after since), tolerant of a missing file (no observer has fired yet)
// or a malformed line (best-effort instrument, never a gate) — mirrors
// readCallLogPointerRows exactly. Only the current generation is read; a
// rotated .1 backup is not (the same bounded-growth trade-off
// rotateBeatLogIfNeeded already accepts for the beat log).
func readObservedDiscoveryRows(root string, since time.Time) []observedDiscoveryEntry {
	data, err := os.ReadFile(observedDiscoveriesLogPathFor(root))
	if err != nil {
		return nil
	}
	var out []observedDiscoveryEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row observedDiscoveryRow
		if json.Unmarshal([]byte(line), &row) != nil || len(row.Pointers) == 0 {
			continue
		}
		ts, terr := time.Parse(time.RFC3339Nano, row.TS)
		if terr != nil || ts.Before(since) {
			continue
		}
		out = append(out, observedDiscoveryEntry{TS: ts, SessionID: row.SessionID, Service: row.Service, AgentID: row.AgentID, RangePointers: row.RangePointers, Tool: row.Tool, Pointers: row.Pointers})
	}
	return out
}

// observedDiscoveryVerb names the verb gatherDiscoveryPointers renders an
// OBSERVED-ONLY pointer with (vision 28 §4: "renders with a read/grep verb
// … so the reader knows it was located, not established by the index") —
// "read" for the read tool, "grepped" for grep/glob/a shell search (any
// act of finding a location rather than reading one file's content).
func observedDiscoveryVerb(tool string) string {
	if tool == "read" {
		return "read"
	}
	return "grepped"
}

// observedRangePointers snapshots only the literal range the read named.
// Search directory targets carry no range and cannot enter a worker brief.
func observedRangePointers(in harness.HookInput, paths []string) []discoveryptr.Pointer {
	start, end := 1, 0
	if in.Tool == harness.ToolDiscoveryRead {
		if in.ReadOffset > 0 {
			start = in.ReadOffset
		}
		if in.ReadLimit > 0 {
			end = start + in.ReadLimit - 1
		}
	} else if in.Tool == harness.ToolShell {
		cmd := shellcmd.ParseCommand(in.Command)
		if cmd.Unreadable || len(cmd.Pipelines) != 1 || len(cmd.Pipelines[0].Units) != 1 {
			return nil
		}
		u := cmd.Pipelines[0].Units[0]
		switch u.Name() {
		case "cat":
		case "sed":
			if len(u.Words) != 4 || u.Words[1].Text != "-n" {
				return nil
			}
			var n int
			script := u.Words[2].Text
			if n, _ = fmt.Sscanf(script, "%d,%dp", &start, &end); n != 2 || script != fmt.Sprintf("%d,%dp", start, end) {
				return nil
			}
		default:
			return nil
		}
	} else {
		return nil
	}
	root := hookMapRoot(resolveProjDir(in.Cwd))
	var out []discoveryptr.Pointer
	for _, p := range paths {
		if ptr, _, ok := discoveryptr.Read(root, p, start, end); ok {
			out = append(out, ptr)
		}
	}
	return out
}
