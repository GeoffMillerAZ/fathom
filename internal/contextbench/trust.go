package contextbench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// trusted_without_recheck (RC-17, F8, vision 28 §2.4 / §3.3): the home
// bench found a fathom answer stood without a Grep/Read/Bash re-check
// 85-90% of the time it was called, while 27% of plain Greps were followed
// by a Read of the same file. This file makes that a per-cell, per-tool
// reading instead of a one-off audit finding: a fathom tool call is
// TRUSTED unless, within the next few turns, a Grep/Read/Bash call targets
// a file path or symbol name that also appeared in the fathom call's own
// input or result — the signal that the agent did not believe the answer
// on its own.

// fathomToolPrefix is how a fathom MCP tool call names itself in a stream
// transcript (the shape scripts/context-bench.sh's allowlists use:
// mcp__fathom__<tool>).
const fathomToolPrefix = "mcp__fathom__"

// recheckTools are the tool calls that count as a re-check of a fathom
// answer: a plain grep, a file read, or an ad hoc shell command — exactly
// the tools arm "full" (SB-6) has alongside the fathom MCP surface.
var recheckTools = map[string]bool{"Grep": true, "Read": true, "Bash": true}

// recheckWindowTurns is "the next 3 turns" (F8): how far past a fathom call
// the reading looks for a Grep/Read/Bash on the same file or symbol before
// calling the answer trusted without re-check.
const recheckWindowTurns = 3

// minTargetLen is the shortest string admitted as a match target — without
// a floor, a 1-2 character value (a bool-ish flag, an index) would "match"
// nearly anything and the reading would be noise.
const minTargetLen = 3

// TrustedToolStats is one tool's slice of TrustedWithoutRecheckStats.
type TrustedToolStats struct {
	Calls   int     `json:"calls"`
	Trusted int     `json:"trusted"`
	Rate    float64 `json:"rate"`
}

// TrustedWithoutRecheckStats is the RC-17 reading, for one cell or
// aggregated over a run: how many fathom tool calls were made, how many of
// them were never re-checked by a Grep/Read/Bash on the same file or
// symbol within the window, the resulting rate, and the same breakdown per
// tool.
type TrustedWithoutRecheckStats struct {
	Calls   int                         `json:"calls"`
	Trusted int                         `json:"trusted"`
	Rate    float64                     `json:"rate"`
	ByTool  map[string]TrustedToolStats `json:"by_tool,omitempty"`
}

// turnCall is one tool_use block, resolved to the string targets a later
// call is compared against: every string value in its own input, plus
// path- and symbol-shaped tokens pulled from its result text.
type turnCall struct {
	name    string
	targets []string
}

// symbolOrPathToken matches a file-path-shaped or dotted-symbol-shaped
// token in free text (e.g. "internal/mcp/tools.go", "core.Edge") — a
// best-effort scan of a tool result, which is prose/JSON, not a schema this
// package owns.
var symbolOrPathToken = regexp.MustCompile(`[A-Za-z0-9_][A-Za-z0-9_./-]*\.[A-Za-z0-9_]+`)

// TrustedWithoutRecheck reads one `claude -p --output-format stream-json
// --verbose` transcript and scores every fathom MCP tool call in it: a call
// is TRUSTED unless a Grep/Read/Bash call in one of the next
// recheckWindowTurns assistant turns targets the same file path or symbol
// name that appeared in the fathom call's own input or result. Pure
// function of its input; non-JSON lines are skipped, exactly like
// TranscriptMetrics and FirstGoldHit.
func TrustedWithoutRecheck(transcript []byte) TrustedWithoutRecheckStats {
	turns := scanTurns(transcript)

	stats := TrustedWithoutRecheckStats{}
	byTool := map[string]TrustedToolStats{}

	for i, turn := range turns {
		for _, call := range turn {
			if !strings.HasPrefix(call.name, fathomToolPrefix) {
				continue
			}
			tool := strings.TrimPrefix(call.name, fathomToolPrefix)
			stats.Calls++
			st := byTool[tool]
			st.Calls++

			trusted := true
		window:
			for j := i + 1; j <= i+recheckWindowTurns && j < len(turns); j++ {
				for _, later := range turns[j] {
					if !recheckTools[later.name] {
						continue
					}
					if overlaps(call.targets, later.targets) {
						trusted = false
						break window
					}
				}
			}
			if trusted {
				stats.Trusted++
				st.Trusted++
			}
			byTool[tool] = st
		}
	}

	if stats.Calls == 0 {
		return stats
	}
	stats.Rate = float64(stats.Trusted) / float64(stats.Calls)
	for tool, st := range byTool {
		st.Rate = float64(st.Trusted) / float64(st.Calls)
		byTool[tool] = st
	}
	stats.ByTool = byTool
	return stats
}

// scanTurns groups a stream transcript into one []turnCall slot per
// assistant turn — including a turn that made no tool call at all, so "the
// next 3 turns" counts real elapsed turns and not just turns that happened
// to call a tool. Each call's targets are resolved from its own input
// plus, for a fathom call only, the matching tool_result text the
// following user event carries (looked up by tool_use_id, the same
// attribution TranscriptMetrics uses for gold_source_tool) — a recheck
// tool's own result (a Read's file body, a Grep's match list) is never
// mined for targets, only its input arguments are.
//
// currentByID maps a tool_use id to its INDEX in current, not a pointer:
// current keeps growing as the same assistant turn's tool_use blocks are
// appended, and a pointer taken mid-loop can be left pointing at a
// backing array append() has since replaced, orphaning every call but the
// last one added in the turn.
func scanTurns(transcript []byte) [][]turnCall {
	var turns [][]turnCall
	var current []turnCall
	currentByID := map[string]int{}
	started := false

	sc := bufio.NewScanner(bytes.NewReader(transcript))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "assistant":
			if started {
				turns = append(turns, current)
			}
			started = true
			current = nil
			currentByID = map[string]int{}
			for _, c := range ev.Message.Content {
				if c.Type != "tool_use" {
					continue
				}
				current = append(current, turnCall{name: c.Name, targets: stringsInJSON(c.Input)})
				if c.ID != "" {
					currentByID[c.ID] = len(current) - 1
				}
			}
		case "user":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				idx, ok := currentByID[c.ToolUseID]
				if !ok || !strings.HasPrefix(current[idx].name, fathomToolPrefix) {
					continue
				}
				current[idx].targets = append(current[idx].targets, pathOrSymbolTokens(rawText(c.Content))...)
			}
		}
	}
	if started {
		turns = append(turns, current)
	}
	return turns
}

// nonIdentifyingKeys are JSON keys whose string value is a fixed
// configuration token, never a file path or symbol name: every fathom
// tool requires "service" (its value is always the repo/service name,
// e.g. "fathom" in this repo, which otherwise reads as a plausible bare
// match target), and a "model" or "flag" argument is the same kind of
// plumbing value. A string under one of these keys never enters targets,
// regardless of its shape.
var nonIdentifyingKeys = map[string]bool{"service": true, "model": true, "flag": true}

// stringsInJSON recursively collects string leaf values out of a JSON
// value (a tool call's input arguments, whatever shape its schema gives
// them — "path", "symbol", "file_path", "pattern", … all fall out the same
// way), deduplicated, floored at minTargetLen, and excluding
// nonIdentifyingKeys.
func stringsInJSON(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	var walk func(key string, v interface{})
	walk = func(key string, v interface{}) {
		switch t := v.(type) {
		case string:
			if nonIdentifyingKeys[key] {
				return
			}
			if len(t) >= minTargetLen && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		case []interface{}:
			for _, e := range t {
				walk(key, e)
			}
		case map[string]interface{}:
			for k, e := range t {
				walk(k, e)
			}
		}
	}
	walk("", v)
	return out
}

// pathOrSymbolTokens pulls file-path- and dotted-symbol-shaped tokens out
// of free text (a tool result), capped so a large index answer cannot make
// this scan unbounded.
func pathOrSymbolTokens(text string) []string {
	matches := symbolOrPathToken.FindAllString(text, -1)
	if len(matches) > 200 {
		matches = matches[:200]
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		if len(m) >= minTargetLen && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// overlaps reports whether any target in a names the same file path or
// symbol as any target in b, via pathOrSymbolMatch.
func overlaps(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if pathOrSymbolMatch(x, y) {
				return true
			}
		}
	}
	return false
}

// pathOrSymbolMatch reports whether x and y name the same file path or
// symbol: exact equality (a "whole-token match" — the same bare symbol
// name, or the same path spelled identically), or, when at least one
// value looks like a path, a shared path suffix at a "/" boundary (a
// fathom answer may cite "internal/mcp/tools.go" while the later Read
// names the absolute path). A bare shared directory segment ("internal",
// "pkg") does not qualify on its own — two unrelated files that merely
// live in the same directory must not count as a match — and neither does
// an arbitrary substring: a short generic token (a service name, a common
// word) must no longer "match" nearly everything, which is what let a
// fathom call's own required arguments and an unrelated Read's full
// result text count as re-checks of each other.
func pathOrSymbolMatch(x, y string) bool {
	if len(x) < minTargetLen || len(y) < minTargetLen {
		return false
	}
	if x == y {
		return true
	}
	if strings.Contains(x, "/") || strings.Contains(y, "/") {
		if strings.HasSuffix(y, "/"+x) || strings.HasSuffix(x, "/"+y) {
			return true
		}
	}
	return false
}

// AggregateTrustedWithoutRecheck merges every cell's TrustedWithoutRecheck
// reading (set on an ArmResult when its transcript recorded at least one
// fathom tool call) into one run-level total, per tool. Returns nil when no
// cell recorded any fathom call — a run scored without --stream, or one
// with no fathom calls at all, carries none, following the same convention
// GoldSourceRate uses for empty input.
func AggregateTrustedWithoutRecheck(results []ArmResult) *TrustedWithoutRecheckStats {
	var out TrustedWithoutRecheckStats
	byTool := map[string]TrustedToolStats{}
	for _, r := range results {
		if r.TrustedWithoutRecheck == nil {
			continue
		}
		out.Calls += r.TrustedWithoutRecheck.Calls
		out.Trusted += r.TrustedWithoutRecheck.Trusted
		for tool, st := range r.TrustedWithoutRecheck.ByTool {
			agg := byTool[tool]
			agg.Calls += st.Calls
			agg.Trusted += st.Trusted
			byTool[tool] = agg
		}
	}
	if out.Calls == 0 {
		return nil
	}
	out.Rate = float64(out.Trusted) / float64(out.Calls)
	for tool, st := range byTool {
		st.Rate = float64(st.Trusted) / float64(st.Calls)
		byTool[tool] = st
	}
	out.ByTool = byTool
	return &out
}

// RenderTrustedWithoutRecheck renders stats as one summary line plus one
// deterministic per-tool line (highest rate first, then tool name) — the
// search-receipt reading (SB-48; F8) rendered beside gold_source_rate. "" for
// nil or zero-call stats.
func RenderTrustedWithoutRecheck(stats *TrustedWithoutRecheckStats) string {
	if stats == nil || stats.Calls == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("  trusted_without_recheck: calls=")
	b.WriteString(strconv.Itoa(stats.Calls))
	b.WriteString(" trusted=")
	b.WriteString(strconv.Itoa(stats.Trusted))
	b.WriteString(" rate=")
	b.WriteString(trimFloat(stats.Rate))
	b.WriteByte('\n')

	tools := make([]string, 0, len(stats.ByTool))
	for t := range stats.ByTool {
		tools = append(tools, t)
	}
	sort.Slice(tools, func(i, j int) bool {
		if stats.ByTool[tools[i]].Rate != stats.ByTool[tools[j]].Rate {
			return stats.ByTool[tools[i]].Rate > stats.ByTool[tools[j]].Rate
		}
		return tools[i] < tools[j]
	})
	for _, t := range tools {
		st := stats.ByTool[t]
		b.WriteString("    ")
		b.WriteString(t)
		b.WriteString(": calls=")
		b.WriteString(strconv.Itoa(st.Calls))
		b.WriteString(" trusted=")
		b.WriteString(strconv.Itoa(st.Trusted))
		b.WriteString(" rate=")
		b.WriteString(trimFloat(st.Rate))
		b.WriteByte('\n')
	}
	return b.String()
}
