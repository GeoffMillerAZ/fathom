package main

// discovery.go — instrument B, the discovery-share denominator
// (docs/design/production-tokenomics.md §4): the program has counted
// fathom's own index-tool calls since the beginning but never the greps
// they were supposed to replace. This extends the phase-O PreToolUse
// observer to a COUNTING-ONLY path: it injects nothing, never blocks, and
// makes NO daemon call — it emits one beat per discovery action with a
// coarse kind (grep/glob/read/shell-search) and returns immediately.
//
// Honesty constraints (design §4): Read stays its own bucket, never
// folded into "grep" — reading a file you already located is dual-use,
// not necessarily discovery, and research_report.go's rendering says so
// explicitly. A rise in fathom's index-tool share is correlational only
// until the pre-registered A/B (§5) runs — the report labels the whole
// section that way.

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// cmdEditHookPreDiscovery is the Grep/Glob/Read branch of cmdEditHookPre:
// a single beat, zero daemon calls, zero stdout (Claude Code hooks treat
// empty stdout as "nothing to inject"). sessionID, when non-empty, tags
// the beat — the same join key instrument A's payload-cost beats and
// instrument D's session ledger use (docs/design/production-tokenomics.md §6).
//
// A Read whose target is in sessionID's recent edit set (editset.go,
// design §4: "Read when the target is outside the current edit set") is
// NOT counted — critic finding F8b (phase-P fix round): a verification
// read of a file this session just edited is not discovery, and counting
// it inflates the denominator. filePath/cwd resolution is local-only
// (serviceCandidates + relativeToRoot, mirroring cmdEditHookPreFile) —
// zero daemon calls either way.
// armOff (IT-3) never gates this function — it is ALREADY counting-only
// (zero daemon calls, zero stdout, always) and is exactly the baseline an
// active OFF arm most needs to keep: silencing it too would delete the one
// signal that proves the arm's sessions are doing anything at all. armOff
// only decides whether the beat is stamped "arm":"off".
//
// RC-56 (vision 28 §4, "every discovery action counts"): alongside the
// beat, the observer also records what the call FOUND — recordObservedDiscoveryIfAny
// appends a pointer row to logs/observed-discoveries.jsonl under the repo's
// state directory (statedir.Resolve — outside the repo tree by default,
// O63 item 5) so a planner that only grepped or read still leaves its
// workers a brief (gatherDiscoveryPointers, discoveries.go). Same
// local-only, zero-daemon discipline; armOff never gates it either, for
// the same reason.
func cmdEditHookPreDiscovery(in harness.HookInput, armOff bool) int {
	if in.Tool == harness.ToolDiscoveryRead && fileInEditSet(in.FilePath, in.Cwd, in.SessionID) {
		return 0 // verification read of a just-edited file — excluded, not discovery
	}
	meta := firingMetaArm(map[string]string{"phase": "pre"}, in.SessionID, "", armOff)
	emitBeat("discovery", discoveryBeatName(in.Tool), meta)
	recordObservedDiscoveryIfAny(in)
	return 0
}

// discoveryBeatName is the beat vocabulary the observer has always used
// (the lower-cased Claude Code tool name: read | grep | glob), keyed by
// tool CLASS so a harness with different tool names counts into the same
// series.
func discoveryBeatName(tool harness.ToolClass) string {
	switch tool {
	case harness.ToolDiscoveryRead:
		return "read"
	case harness.ToolDiscoveryGrep:
		return "grep"
	case harness.ToolDiscoveryGlob:
		return "glob"
	}
	return strings.ToLower(string(tool))
}

// shellSearchBinaries are the argv[0] values that count as a "shell
// search" discovery action (design §4's literal list).
var shellSearchBinaries = map[string]bool{
	"grep": true,
	"rg":   true,
	"find": true,
	"ag":   true,
	"ack":  true,
}

// emitDiscoveryBeatIfShellSearch inspects a Bash tool call's command
// string and emits a "shell-search" discovery beat when its first token
// (argv[0], allowing a leading simple `VAR=value` env-assignment prefix
// and/or a `command`/`env` wrapper) names one of shellSearchBinaries.
// Best-effort and deliberately conservative: it is a coarse classifier for
// a denominator, not a shell parser — commands it can't confidently
// classify are simply not counted, never mis-counted.
func emitDiscoveryBeatIfShellSearch(command, sessionID string, armOff bool) {
	if bin := firstCommandToken(command); bin != "" && shellSearchBinaries[bin] {
		emitBeat("discovery", "shell-search", firingMetaArm(map[string]string{"phase": "pre"}, sessionID, "", armOff))
	}
}

// firstCommandToken returns the basename of a shell command's first real
// argv token — skipping leading `VAR=value` environment assignments and a
// leading `command`/`env` wrapper (common in tool-generated shell calls) —
// or "" when the command is empty or its first token can't be read as a
// simple identifier.
func firstCommandToken(command string) string {
	fields := strings.Fields(command)
	for i := 0; i < len(fields); i++ {
		tok := fields[i]
		if strings.Contains(tok, "=") && !strings.HasPrefix(tok, "-") {
			// looks like a leading env assignment (FOO=bar cmd ...); skip it
			eq := strings.Index(tok, "=")
			if isSimpleIdent(tok[:eq]) {
				continue
			}
		}
		if tok == "command" || tok == "env" {
			continue
		}
		// Strip a path prefix (e.g. /usr/bin/grep) — only the binary name
		// matters for classification.
		if slash := strings.LastIndex(tok, "/"); slash >= 0 {
			tok = tok[slash+1:]
		}
		return tok
	}
	return ""
}

func isSimpleIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}
