package harness

// codex_cost.go — PE-14 (docs/design/progress-and-economics.md §5, "outcome
// economics"): Codex session files read by the same incremental fold the
// Stop hook gives Claude Code transcripts (cmd/fathom foldTranscriptCost,
// PE-13), on the harness port. It yields the fields a session's proven
// outcome is priced against — tokens in and out, cache reads and writes,
// and tool calls — from a Codex "rollout" (~/.codex/sessions/YYYY/MM/DD/
// rollout-*.jsonl), and nothing else: no message, tool argument or output
// is decoded, kept or returned.
//
// The schema below was derived from real rollouts (327 files, codex-cli
// 0.45.0 through 0.154.x, 2025-10 to 2026-09, surveyed read-only
// 2026-09-28), not from documentation; the sanitized fixtures under
// testdata/rollouts/codex pin it. One JSON object per line:
//
//	{"timestamp":…,"ordinal":N,"type":T,"payload":{"type":P,…}}
//
//   - T "event_msg", P "token_count": payload.info is null before the first
//     response, then {total_token_usage, last_token_usage,
//     model_context_window}. total_token_usage is CUMULATIVE for the session
//     (input_tokens, cached_input_tokens, cache_write_input_tokens,
//     output_tokens, reasoning_output_tokens, total_tokens). Every token_count
//     event is repeated once (the same numbers, a few hundred ms later), so a
//     sum over events double counts; the fold takes the cumulative maximum.
//     token_count is the ONE usage carrier common to every vintage: 35 of the
//     327 rollouts predate the token_usage_record line and have only it.
//   - T "response_item", P "function_call" | "custom_tool_call": one client
//     side tool invocation each. Their *_output partners, "reasoning",
//     "message", provider-side "web_search_call" and "tool_search_call" are
//     not tool calls.
//   - T "turn_context": payload.model is the model the turn ran on.
//
// Three facts the mapping rests on, each checked over all 327 files:
// cached_input_tokens is never greater than input_tokens (Codex's input
// INCLUDES the cached part, unlike Claude Code's, whose input_tokens
// excludes cache reads — the fold subtracts it so the two harnesses' rows
// price the same way); total_tokens always equals input_tokens plus
// output_tokens (output includes reasoning tokens, which bill as output);
// and cache_write_input_tokens is 0 in every rollout — so its relation to
// input_tokens is unobserved, and it is treated as a subset of input, like
// cached, until a real nonzero sample says otherwise.
//
// One file broke the "cumulative" reading: a codex-cli 0.98.0 session
// (2026-02) whose totals go backwards between events (snapshots written out
// of order). The fold keeps the event with the greatest input+output, whose
// split stays internally consistent, and says so in SessionCost.Note. In
// all 327 files that event is also the file's last, so the reading is the
// session total.
//
// Rollouts reach 1 GB and a single line reaches 11 MB, so the read is
// incremental and bounded exactly like the Claude fold (RT-132): the caller
// keeps the returned CodexCostFold (it is JSON-shaped for a stamp file) and
// passes it back, so each call folds only what the file gained, capped at
// codexReadMaxBytes; Truncated says the cap cut the read and the next call
// continues. A line longer than the cap is skipped and counted, never a
// permanent stall.

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"syscall"
)

// codexReadMaxBytes bounds one fold call's read, the same 16 MiB the Claude
// transcript fold (transcriptReadMaxBytes) allows: far under a hook's
// deadline, and a 1 GB rollout costs many bounded calls, never one
// unbounded one.
const codexReadMaxBytes = 16 << 20

// codexSniffBytes is how much of a line's head the cheap prefilter looks at
// before deciding the line is worth a JSON parse. The envelope puts type
// and payload.type within the first ~120 bytes of every real line; the
// prefilter only ever SKIPS work (a parsed line is classified by exact
// field match), and most of a rollout's bytes are tool output and
// reasoning that need no parse at all.
const codexSniffBytes = 512

// Fixed-string notes, in the style of the Claude fold's TokensNote.
const (
	codexNoteUnreadable  = "unavailable: rollout path unreadable or not a regular file"
	codexNoteNoUsage     = "unavailable: no token_count event carried usage"
	codexNoteNonMonotone = "token_count totals were not monotone; the largest total is used"
	codexNoteSkipped     = "lower bound: a line too long to read, or unparseable, was skipped"
)

// SessionCost is one session's cost-relevant transcript fields, in the
// split cmd/fathom's transcriptCost (PE-13) and internal/cost's ModelRate
// use: TokensIn is UNCACHED input; cache reads and writes are separate.
// Available is false — and every token count 0 — when the rollout carried
// no usage at all (Note says why), so a reader never mistakes "not
// measured" for "measured as zero" (law 12). ToolCalls is counted
// regardless.
type SessionCost struct {
	TokensIn          int
	TokensOut         int
	TokensCacheRead   int
	TokensCacheCreate int
	ToolCalls         int
	// Model is the model of the last turn_context seen ("" when none). A
	// session that switched models mid-way is priced at its last model;
	// the cumulative totals cannot be split per model.
	Model     string
	Available bool
	// Truncated is set when this call's read hit the byte cap before the
	// end of the file: every figure is a lower bound until a call from the
	// returned fold reports it false.
	Truncated bool
	// SkippedLines counts lines the fold could not read: longer than the
	// read cap, or a relevant line that did not parse.
	SkippedLines int
	Note         string
}

// codexUsage is total_token_usage's fields the fold reads.
type codexUsage struct {
	Input      int `json:"input_tokens"`
	Cached     int `json:"cached_input_tokens"`
	CacheWrite int `json:"cache_write_input_tokens"`
	Output     int `json:"output_tokens"`
}

func (u codexUsage) order() int { return u.Input + u.Output }

// CodexCostFold is the resumable state of CodexSessionCost: what has been
// folded so far and where to continue. Exported fields, JSON tags: a caller
// persists it beside the session (as the Claude fold's stamp does) and
// passes it back. The zero value folds from the start.
type CodexCostFold struct {
	// Offset is the bytes folded, always at a line boundary (or, with
	// Skipping, inside a line being skipped).
	Offset int64  `json:"offset"`
	Dev    uint64 `json:"dev"`
	Ino    uint64 `json:"ino"`
	// Skipping is set while the bytes at Offset continue a line longer
	// than the read cap that is being discarded.
	Skipping     bool       `json:"skipping,omitempty"`
	SkippedLines int        `json:"skipped_lines,omitempty"`
	SawUsage     bool       `json:"saw_usage,omitempty"`
	Best         codexUsage `json:"best"`
	Regressed    bool       `json:"regressed,omitempty"`
	ToolCalls    int        `json:"tool_calls"`
	Model        string     `json:"model,omitempty"`
}

// codexLine is the union of the fields the fold reads from any line; a
// field a line does not carry is its zero value.
type codexLine struct {
	Type    string `json:"type"`
	Payload struct {
		Type  string `json:"type"`
		Model string `json:"model"`
		Info  *struct {
			Total codexUsage `json:"total_token_usage"`
		} `json:"info"`
	} `json:"payload"`
}

var (
	codexMarkTokenCount = []byte(`"token_count"`)
	codexMarkFunction   = []byte(`"function_call"`)
	codexMarkCustom     = []byte(`"custom_tool_call"`)
	codexMarkTurn       = []byte(`"turn_context"`)
)

// add folds one complete rollout line into the state.
func (f *CodexCostFold) add(line []byte) {
	head := line
	if len(head) > codexSniffBytes {
		head = head[:codexSniffBytes]
	}
	if !bytes.Contains(head, codexMarkTokenCount) && !bytes.Contains(head, codexMarkFunction) &&
		!bytes.Contains(head, codexMarkCustom) && !bytes.Contains(head, codexMarkTurn) {
		return
	}
	var l codexLine
	if err := json.Unmarshal(line, &l); err != nil {
		f.SkippedLines++
		return
	}
	switch {
	case l.Type == "event_msg" && l.Payload.Type == "token_count":
		if l.Payload.Info == nil {
			return // before the first response: no usage yet, not a zero
		}
		u := l.Payload.Info.Total
		if f.SawUsage && u.order() < f.Best.order() {
			f.Regressed = true
			return
		}
		f.SawUsage, f.Best = true, u
	case l.Type == "response_item" && (l.Payload.Type == "function_call" || l.Payload.Type == "custom_tool_call"):
		f.ToolCalls++
	case l.Type == "turn_context":
		if l.Payload.Model != "" {
			f.Model = l.Payload.Model
		}
	}
}

// cost reads the state out as a SessionCost.
func (f CodexCostFold) cost() SessionCost {
	c := SessionCost{ToolCalls: f.ToolCalls, Model: f.Model, SkippedLines: f.SkippedLines}
	var notes []string
	if f.SkippedLines > 0 {
		notes = append(notes, codexNoteSkipped)
	}
	if !f.SawUsage {
		notes = append([]string{codexNoteNoUsage}, notes...)
		c.Note = strings.Join(notes, "; ")
		return c
	}
	u := f.Best
	cached := clampInt(u.Cached, 0, u.Input)
	write := clampInt(u.CacheWrite, 0, u.Input-cached)
	c.TokensIn = u.Input - cached - write
	c.TokensCacheRead = cached
	c.TokensCacheCreate = write
	c.TokensOut = u.Output
	c.Available = true
	if f.Regressed {
		notes = append([]string{codexNoteNonMonotone}, notes...)
	}
	c.Note = strings.Join(notes, "; ")
	return c
}

func clampInt(v, lo, hi int) int {
	if v > hi {
		v = hi
	}
	if v < lo {
		v = lo
	}
	return v
}

// CodexSessionCost folds the Codex rollout at path from prev's offset (nil
// folds from the start) and returns the session's cost so far plus the
// state to pass to the next call. It reads at most codexReadMaxBytes per
// call; see the file header for the schema and its provenance.
func CodexSessionCost(path string, prev *CodexCostFold) (SessionCost, CodexCostFold) {
	return codexSessionCost(path, prev, codexReadMaxBytes)
}

// codexSessionCost is CodexSessionCost with the read cap as a parameter, so
// a test can exercise the cap on a small fixture.
func codexSessionCost(path string, prev *CodexCostFold, maxBytes int64) (SessionCost, CodexCostFold) {
	var fold CodexCostFold
	if prev != nil {
		fold = *prev
	}
	// Same hardening as the Claude transcript read: never follow a
	// symlink, never block on a FIFO, only a regular file.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return SessionCost{Note: codexNoteUnreadable}, fold
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return SessionCost{Note: codexNoteUnreadable}, fold
	}
	var dev, ino uint64
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		dev, ino = uint64(st.Dev), uint64(st.Ino)
	}
	// A stored fold for another file, or past this one's end (truncated,
	// replaced), is discarded and the file is folded from 0.
	if fold.Offset > 0 && (fold.Dev != dev || fold.Ino != ino || fold.Offset > fi.Size()) {
		fold = CodexCostFold{}
	}
	fold.Dev, fold.Ino = dev, ino
	if _, err := f.Seek(fold.Offset, io.SeekStart); err != nil {
		return SessionCost{Note: codexNoteUnreadable}, fold
	}
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return SessionCost{Note: codexNoteUnreadable}, fold
	}
	truncated := int64(len(data)) > maxBytes
	if truncated {
		data = data[:maxBytes]
	}

	// Mid-way through an over-long line: discard up to and including its
	// newline.
	resumedMidLine := fold.Skipping
	if fold.Skipping {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			fold.Offset += int64(len(data))
			c := fold.cost()
			c.Truncated = truncated
			return c, fold
		}
		fold.Offset += int64(i + 1)
		data = data[i+1:]
		fold.Skipping = false
	}

	// Fold every complete line into the state; an unterminated tail (a
	// write in progress, or the cap's cut) is folded into a copy for this
	// reading only and read again next time.
	complete, tail := data, []byte(nil)
	if i := bytes.LastIndexByte(data, '\n'); i < len(data)-1 {
		complete, tail = data[:i+1], data[i+1:]
	}
	for len(complete) > 0 {
		i := bytes.IndexByte(complete, '\n')
		fold.add(bytes.TrimSuffix(complete[:i], []byte{'\r'}))
		complete = complete[i+1:]
	}
	consumed := int64(len(data) - len(tail))
	fold.Offset += consumed
	if consumed == 0 && truncated && len(tail) > 0 && !resumedMidLine {
		// One line fills the whole cap (this read began at a line
		// boundary): skip it, and count it, rather than stall at this
		// offset forever. After a skipped prefix the fragment is just a
		// line the cap cut; the next call re-reads it from its start.
		fold.SkippedLines++
		fold.Skipping = true
		fold.Offset += int64(len(tail))
		tail = nil
	}
	view := fold
	if len(tail) > 0 && !truncated {
		view.add(bytes.TrimSuffix(tail, []byte{'\r'}))
	}
	c := view.cost()
	c.Truncated = truncated
	return c, fold
}
