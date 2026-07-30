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

import "strings"

// cmdEditHookPreDiscovery is the Grep/Glob/Read branch of cmdEditHookPre:
// a single beat, zero daemon calls, zero stdout (Claude Code hooks treat
// empty stdout as "nothing to inject"). sessionID, when non-empty, tags
// the beat — the same join key instrument A's payload-cost beats and
// instrument C's session ledger use (docs/design/production-tokenomics.md §6).
func cmdEditHookPreDiscovery(toolName, sessionID string) int {
	meta := map[string]string{"phase": "pre"}
	if sessionID != "" {
		meta["session"] = sessionID
	}
	emitBeat("discovery", strings.ToLower(toolName), meta)
	return 0
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
func emitDiscoveryBeatIfShellSearch(command, sessionID string) {
	if bin := firstCommandToken(command); bin != "" && shellSearchBinaries[bin] {
		meta := map[string]string{"phase": "pre"}
		if sessionID != "" {
			meta["session"] = sessionID
		}
		emitBeat("discovery", "shell-search", meta)
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
