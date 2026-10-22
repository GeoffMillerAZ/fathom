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
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
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
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var shellSearchBinaries = map[string]bool{
	"grep": true,
	"rg":   true,
	"find": true,
	"ag":   true,
	"ack":  true,
}

// emitDiscoveryBeatIfShellSearch inspects a Bash tool call's command
// string through the one command lexer (shellcmd.ParseCommand, RO-4) and
// emits one discovery beat for it: "shell-search" when a pipeline leads with
// one of shellSearchBinaries, "shell-read" when it leads with `cat`, `head`,
// `tail` or `sed -n` of a literal source file (RO-29, F75's gap), and
// "unreadable" when the lexer cannot read the command, so an unreadable call
// is counted as such and never as zero reads. Anything else is not counted.
// Zero daemon calls and zero stdout; best-effort and deliberately
// conservative: a coarse classifier for a denominator, never an evaluator.
func emitDiscoveryBeatIfShellSearch(command, sessionID string, armOff bool, roots ...string) {
	if kind := shellDiscoveryKind(command, roots...); kind != "" {
		emitBeat("discovery", kind, firingMetaArm(map[string]string{"phase": "pre"}, sessionID, "", armOff))
	}
}

// discoveryKindUnreadable is the beat kind of a Bash command the lexer
// cannot read (RO-4). It is counted and named, never a discovery action:
// every count of actions skips it, so no share's denominator reads a
// command fathom could not read as a known action (I38).
const discoveryKindUnreadable = "unreadable"

// shellDiscoveryKind classifies a Bash command for instrument B: the beat
// kind ("shell-search", "shell-read" or "unreadable"), or "" when the
// command is empty or is not a discovery action. Each pipeline is judged by
// its first command (a leading cd, pushd or popd is skipped), the first
// pipeline that classifies wins.
func shellDiscoveryKind(command string, roots ...string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	cmd := parseShellCommand(command, roots...).Transparent()
	if cmd.Unreadable {
		return discoveryKindUnreadable
	}
	for _, p := range cmd.Pipelines {
		for i := range p.Units {
			u := &p.Units[i]
			if u.Kind != shellcmd.KindSimple || len(u.Words) == 0 {
				break
			}
			name := u.Name()
			if name == "cd" || name == "pushd" || name == "popd" {
				break
			}
			if shellSearchBinaries[name] {
				return "shell-search"
			}
			if _, ok := shellSourceReadTarget(u); ok {
				return "shell-read"
			}
			break
		}
	}
	return ""
}

// shellSourceReadTarget returns the literal source-file path a simple
// command reads whole or by range: `cat`, `head` or `tail` of it, or
// `sed -n <script> <file>`. ok is false for any other command, a flag or a
// dynamic or glob word in the path position, or a path that is not a
// source file.
func shellSourceReadTarget(u *shellcmd.Unit) (string, bool) {
	if len(u.Words) < 2 {
		return "", false
	}
	switch u.Name() {
	case "cat", "head", "tail":
	case "sed":
		if len(u.Words) < 4 || u.Words[1].Text != "-n" {
			return "", false
		}
	default:
		return "", false
	}
	last := u.Words[len(u.Words)-1]
	if !last.Literal() || last.Text == "" || strings.HasPrefix(last.Text, "-") {
		return "", false
	}
	if !isSourceFilePath(last.Text) {
		return "", false
	}
	return last.Text, true
}

// isSourceFilePath reports whether p names a source file by extension.
func isSourceFilePath(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".go", ".py", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx", ".rs", ".java", ".kt", ".kts",
		".scala", ".c", ".h", ".cc", ".cpp", ".hpp", ".cs", ".rb", ".php", ".swift", ".lua", ".sh",
		".bash", ".zsh", ".ex", ".exs", ".erl", ".hs", ".ml", ".clj", ".dart", ".r", ".pl", ".sql",
		".proto", ".cue", ".vue", ".svelte":
		return true
	}
	return false
}
