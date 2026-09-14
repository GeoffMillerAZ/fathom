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
// Pointers here are paths as written (repo-relative), never a symbol id
// — resolving path+line against the index is deliberately skipped on the
// hook path (it would add a daemon round trip to a path that must never
// block); gatherDiscoveryPointers/renderOneDiscoveryLine do that
// resolution, fresh, at render time, exactly as they already do for the
// MCP call log's pointers.
package main

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// observedDiscoverySchemaVersion identifies observed-discoveries.jsonl's
// row shape.
const observedDiscoverySchemaVersion = 1

// observedDiscoveryRow is one JSONL row of workspace/logs/
// observed-discoveries.jsonl — the observer's own mechanical fact ("this
// call named this location"), never a model's summary of one.
type observedDiscoveryRow struct {
	SchemaVersion int      `json:"schema_version"`
	TS            string   `json:"ts"` // RFC3339Nano, matches the MCP call log's own ts format
	SessionID     string   `json:"session_id,omitempty"`
	Cwd           string   `json:"cwd,omitempty"`
	Tool          string   `json:"tool"` // "read" | "grep" | "glob" — the verb gatherDiscoveryPointers renders an observed-only pointer with
	Pointers      []string `json:"pointers"`
}

// observedDiscoveryEntry is observedDiscoveryRow parsed back for merging:
// TS as a time.Time, everything else unchanged.
type observedDiscoveryEntry struct {
	TS        time.Time
	SessionID string
	Tool      string
	Pointers  []string
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
		if target, _ := bashObservedReadPath(in.Command); target != "" {
			if p, ok := observedRepoRelPath(target, in.Cwd); ok {
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
		_, tool := bashObservedReadPath(in.Command)
		return tool
	}
	return discoveryBeatName(in.Tool)
}

// observedRepoRelPath resolves p (as sent on a tool call: usually
// absolute, occasionally relative to the session's own cwd) against the
// local, daemon-free service-candidate roots — the exact resolution
// fileInEditSet already performs for a Read's target. ok is false when p
// is empty or resolves under no known root (out of any indexed repo
// entirely, or a path fathom cannot place).
func observedRepoRelPath(p, cwd string) (string, bool) {
	if p == "" {
		return "", false
	}
	if !filepath.IsAbs(p) {
		a, err := filepath.Abs(p)
		if err != nil {
			return "", false
		}
		p = a
	}
	projDir := resolveProjDir(cwd)
	for _, cand := range serviceCandidates(projDir) {
		if rel, ok := relativeToRoot(cand.root, p); ok {
			return rel, true
		}
	}
	return "", false
}

// bashObservedReadKind classifies which discovery verb a recognized shell
// command implies — "read" for a single-file content dump (cat/sed -n/
// head/tail), "grep" for a search across a path (rg/grep) — the same
// read-vs-search distinction discoveryBeatName already keeps Read and
// Grep apart for (docs/design/production-tokenomics.md §4's honesty
// constraint: reading what you already located is not the same act as
// finding it).
var bashObservedReadKind = map[string]string{
	"cat": "read", "head": "read", "tail": "read", "sed": "read",
	"rg": "grep", "grep": "grep",
}

// bashObservedShellMetachars disqualifies a Bash command from being a
// literal single-target read/search: a pipe or redirection means the
// tool's result is not that one target's raw content, a variable/
// subshell/glob means the target is not written out in the transcript at
// all. Mirrors internal/scenariobench/readings_overlap.go's own
// bashShellMetachars (RC-11) as an IDEA, not an import — the production
// hook path must never depend on a benchmark package.
const bashObservedShellMetachars = "|&;<>`$*?~\n"

// stripBashObservedQuoted drops the content of every single- or
// double-quoted span in s, naively (no backslash-escape handling), so a
// sed/awk script's own quoted metacharacters never trip
// bashObservedShellMetachars. Mirrors readings_overlap.go's stripQuoted.
func stripBashObservedQuoted(s string) string {
	var b strings.Builder
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// bashObservedReadPath returns the literal trailing path argument of a
// Bash command recognizable as one of bashObservedReadKind's binaries,
// plus the discovery tool label ("read" or "grep") it implies — "", ""
// when command is not recognizable (a pipe, redirection, glob, or
// variable substitution disqualifies the whole command; a trailing flag
// or too few fields means no path at all).
func bashObservedReadPath(command string) (target, tool string) {
	command = strings.TrimSpace(command)
	if command == "" || strings.ContainsAny(stripBashObservedQuoted(command), bashObservedShellMetachars) {
		return "", ""
	}
	fields := strings.Fields(command)
	if len(fields) < 2 {
		return "", ""
	}
	bin := path.Base(fields[0])
	kind, recognized := bashObservedReadKind[bin]
	if !recognized {
		return "", ""
	}
	last := fields[len(fields)-1]
	if last == "" || strings.HasPrefix(last, "-") {
		return "", "" // a trailing flag, not a path
	}
	if bin == "sed" && (len(fields) < 4 || fields[1] != "-n") {
		// sed's read shape is specifically "sed -n <script> file" (the
		// RC-11 4-field floor) — a bare "sed -n file" has no script and
		// sed would read it as one, not as a path.
		return "", ""
	}
	return last, kind
}

// ─── write path (item 2) ──────────────────────────────────────────────────

// observedDiscoveriesLogPathFor is workspace/logs/observed-discoveries.jsonl
// under root's git toplevel (or root itself outside a repo) — the same
// workspace/ convention sessionsLogPathFor and callLogPathFor already use.
// FATHOM_OBSERVED_DISCOVERIES_LOG overrides it (tests).
func observedDiscoveriesLogPathFor(root string) string {
	if p := os.Getenv("FATHOM_OBSERVED_DISCOVERIES_LOG"); p != "" {
		return p
	}
	base := root
	if top := gitToplevel(root); top != "" {
		base = top
	}
	return filepath.Join(base, "workspace", "logs", "observed-discoveries.jsonl")
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
	if len(pointers) == 0 {
		return
	}
	recordObservedDiscovery(resolveProjDir(in.Cwd), observedDiscoveryRow{
		SchemaVersion: observedDiscoverySchemaVersion,
		TS:            time.Now().UTC().Format(time.RFC3339Nano),
		SessionID:     in.SessionID,
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
		out = append(out, observedDiscoveryEntry{TS: ts, SessionID: row.SessionID, Tool: row.Tool, Pointers: row.Pointers})
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
