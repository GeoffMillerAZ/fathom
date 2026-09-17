// Package mcp — answer-first envelopes (wave L1, vision 28 §3; H1-T05/
// H1-T07, docs/bench/readings/vs-pilot-6-second-reading.md and vs-pilot-5).
//
// Evidence: fathom's tools handed the agent the right evidence and the
// model reasoned past it. H1-T05 — find_callers(RenderMarkdown, depth=2)
// listed the direct caller first, and the final answer named a depth-2
// caller three hops out instead. H1-T07 — search hit the right line, and
// the answer cited a different document. The tools were correct; the
// presentation made the correct row easy to miss inside a payload built for
// completeness, not for being read top to bottom. The fix here is
// presentation, deterministic: every promoted verb's payload leads with a
// `summary` field the agent can quote verbatim — one sentence, built from
// the SAME sorted/truncated data the rest of the body carries, so the
// summary can never name a row the body does not actually return (never a
// depth-2 caller as "direct", never a count wider than what fitToBudget/
// fitToolBudget actually kept).
package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// summaryMaxChars bounds every verb's summary sentence. The M11b envelope
// bar (docs/design/budgeted-answers.md) was 10% of a tool's median payload;
// 200 chars stays comfortably under that for every verb this wave touches
// while still fitting a direct answer plus two or three supporting facts.
const summaryMaxChars = 200

// capSummary trims s to at most summaryMaxChars runes, cutting on a rune
// boundary (never mid-UTF-8) and marking the cut with an ellipsis.
// Deterministic: the same input always produces the same output, so two
// calls against an unchanged index never disagree.
func capSummary(s string) string {
	if utf8.RuneCountInString(s) <= summaryMaxChars {
		return s
	}
	runes := []rune(s)
	return strings.TrimRight(string(runes[:summaryMaxChars-1]), " ") + "…"
}

// pluralS returns "" for n == 1, "s" otherwise — the one pluralization
// rule every verb's summary sentence needs ("1 direct caller" vs "3 direct
// callers").
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// summaryEnvelope wraps an already-built tool response so `summary`
// marshals as literally the FIRST key in the JSON text the agent reads —
// encoding/json always sorts a map[string]any's keys alphabetically, so
// simply adding a "summary" entry to a handler's response map would not
// reliably put it first (it sorts wherever "s" falls). MarshalJSON splices
// summary in ahead of body's own encoding instead, leaving every other
// field's order (and the elision/budget accounting that produced them)
// completely untouched.
type summaryEnvelope struct {
	summary string
	body    any
}

// withSummary returns v wrapped so "summary" is the first JSON key, or v
// unwrapped when summary is empty — a defensive fallback no verb should
// normally hit (an empty summary is a bug in the caller's construction, not
// a reason to emit a broken envelope).
func withSummary(summary string, v any) any {
	if summary == "" {
		return v
	}
	return summaryEnvelope{summary: capSummary(summary), body: v}
}

// MarshalJSON renders {"summary": ..., <body's own fields>...}. body is
// marshaled exactly as it would be on its own (preserving every existing
// elision/budget/structuredContent guarantee); only summary's position is
// the deliberate exception. Falls back to body's own bytes, unmodified, if
// body does not marshal to a JSON object — every verb withSummary wraps
// always returns a map or struct, so this never fires in practice; it
// exists so a future misuse degrades instead of producing invalid JSON.
func (e summaryEnvelope) MarshalJSON() ([]byte, error) {
	bodyBytes, err := json.Marshal(e.body)
	if err != nil {
		return nil, err
	}
	if len(bodyBytes) < 2 || bodyBytes[0] != '{' {
		return bodyBytes, nil
	}
	summaryBytes, err := json.Marshal(e.summary)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Grow(len(bodyBytes) + len(summaryBytes) + 12)
	buf.WriteString(`{"summary":`)
	buf.Write(summaryBytes)
	if len(bodyBytes) > 2 { // body is not "{}"
		buf.WriteByte(',')
	}
	buf.Write(bodyBytes[1:])
	return buf.Bytes(), nil
}
