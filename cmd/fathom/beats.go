package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/dlog"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// beatLogPath resolves the central hooks beat log (GI-1): FATHOM_HOOKS_LOG,
// defaulting to ~/.fathom/hooks.jsonl. Hooks fire in every fleet repo, so the
// log is per-user central, never per-repo.
func beatLogPath() string {
	if p := os.Getenv("FATHOM_HOOKS_LOG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "hooks.jsonl")
}

// beatLogRotateBytes bounds the beat log's on-disk size — critic finding
// F7 (phase-P fix round): phase P's own discovery observer adds a
// highest-tool-call-frequency beat source (one per Grep/Glob/Read/
// shell-search) with no offsetting rotation, so the pre-existing
// unbounded-growth log now grows materially faster than before this
// phase. rotateBeatLogIfNeeded keeps ONE prior generation
// (hooks.jsonl.1) as a bounded backstop — not a full multi-generation
// archive — so worst-case on-disk cost is ~2x this cap, never unbounded.
// Trade-off, stated honestly: a report or session-ledger read that spans
// beats older than one rotation back will not see them; this is a real
// cost of bounding growth at all, not something rotation avoids for free.
// A package var (not a const) so tests can shrink it rather than writing
// a real 64MB fixture file.
var beatLogRotateBytes int64 = 64 << 20

// emitBeat appends one channel beat to the hooks beat log. It is the
// activity/effect denominator for forced channels (docs/vision/04 Gap 1).
// ALL failures are swallowed: hooks never block, and instrumentation must
// never be the reason a hook breaks its silent-exit contract.
func emitBeat(channel, kind string, meta map[string]string) {
	path := beatLogPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	rotateBeatLogIfNeeded(path)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	l, err := heartbeat.NewLog(f, time.Now)
	if err != nil {
		return
	}
	_ = l.Beat(channel, kind, stampBeatMeta(meta))
}

// firingMeta adds the join keys the intervention retro needs (IR-1,
// docs/design/intervention-outcomes.md): which session a firing belonged
// to and which file it was about. Absent values are omitted — an
// unrecorded fact must not look recorded as an empty string.
func firingMeta(meta map[string]string, sessionID, file string) map[string]string {
	if meta == nil {
		meta = map[string]string{}
	}
	if sessionID != "" {
		meta["session"] = sessionID
	}
	if file != "" {
		meta["file"] = file
	}
	return meta
}

// firingMetaArm is firingMeta plus IT-3's OFF-arm stamp: when armOff is
// true the returned meta carries "arm":"off", so a beat recorded during an
// active kill switch / measurement arm is distinguishable from one
// recorded normally — never silently indistinguishable from "the hook
// never ran at all" (the false-IDLE failure class flywheel.go guards
// elsewhere).
func firingMetaArm(meta map[string]string, sessionID, file string, armOff bool) map[string]string {
	meta = firingMeta(meta, sessionID, file)
	if armOff {
		meta["arm"] = "off"
	}
	return meta
}

// beatHarness is the registry id of the harness whose envelope this
// process is speaking, set by resolveHookHarness the moment a hook verb
// resolves its codec (hookharness.go). Every beat a hook writes is then
// stamped with it, which is what lets doctor's hooks_firing answer
// "have CODEX's hooks ever fired here?" from one user-global log that
// every harness appends to. "" for a non-hook invocation (a CLI run), and
// the key is then omitted rather than recorded as an empty string.
//
// A package var, not a parameter: emitBeat has ~40 call sites across the
// hook verbs and threading a harness id through all of them would be a
// large diff whose only effect is this one meta key.
var beatHarness string

// stampBeatMeta returns a copy of meta carrying the telemetry stamps every
// persistent record now shares (vision 14 §3, plan IO-12): schema_version,
// fathom_version, and project_dir — the per-repo attribution the beat log
// lacked (doctor once cited one repo's beats as evidence about another,
// because ~/.fathom/hooks.jsonl is user-global and rows carried nothing to
// scope them by). The caller's map is never mutated; caller-provided keys
// win on collision.
func stampBeatMeta(meta map[string]string) map[string]string {
	out := make(map[string]string, len(meta)+3)
	out["schema_version"] = beatSchemaVersion
	out["fathom_version"] = buildinfo.Version()
	if cwd, err := os.Getwd(); err == nil {
		out["project_dir"] = cwd
	}
	if beatHarness != "" {
		out["harness"] = beatHarness
	}
	for k, v := range meta {
		out[k] = v
	}
	return out
}

// beatSchemaVersion identifies the beat meta contract: "2" adds the stamps
// above; rows without the key are the v1 legacy shape, read as
// version-unknown, never backfilled.
const beatSchemaVersion = "2"

// rotateBeatLogIfNeeded renames path to path+".1" (overwriting any prior
// backup) when it exceeds beatLogRotateBytes, so the NEXT append starts a
// fresh file. Best-effort: any failure (including "no file yet," the
// overwhelmingly common case) is silent, exactly like every other beat-log
// operation. A thin wrapper over internal/dlog.RotateIfNeeded (DG-6,
// diagnostics sweep) — this file proved the one-prior-generation rotation
// out first; the daemon's own launchd-piped stdout/stderr (DG-6's other
// half, cmd/fathom/daemonstdio.go) reuses the same primitive.
func rotateBeatLogIfNeeded(path string) {
	_ = dlog.RotateIfNeeded(path, beatLogRotateBytes)
}
