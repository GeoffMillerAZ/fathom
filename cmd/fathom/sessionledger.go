package main

// sessionledger.go — instrument D, the session token ledger
// (docs/design/production-tokenomics.md §6): the Stop hook already runs
// once per session; this extends it to append one line per session to a
// local ledger, read-only with respect to everything else the Stop hook
// does (plan verification, §5.3's fail-open advisory channel) — a ledger
// write failure must never affect that behavior, mirroring every other
// hook-instrumentation write in this program (emitBeat, writeGuardStamp).
//
// The ledger's session_id is the join key that ties this instrument to
// instrument A (payload-cost beats) and instrument B (discovery beats) —
// both tag their beats with the SAME "session" meta key when a session id
// is available (payloadcost.go, discovery.go) — so a downstream analysis
// can group all three by session_id into one dataset instead of three
// disconnected tallies (design §6's own framing).
//
// Transcript parsing (turn count / harness-reported tokens / duration) is
// best-effort against Claude Code's own session transcript JSONL
// (transcript_path, a standard Stop-hook field): absent, unreadable, or
// unrecognized content degrades to an honest "unavailable" note — NEVER a
// fabricated zero (law 12's discipline extended to this instrument).

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// sessionLedgerPath resolves the local ledger file: $FATHOM_SESSION_LEDGER,
// defaulting to ~/.fathom/session-ledger.jsonl — per-user central, mirroring
// beatLogPath's own convention (hooks fire in every fleet repo, so the
// ledger is not repo-scoped).
func sessionLedgerPath() string {
	if p := os.Getenv("FATHOM_SESSION_LEDGER"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "session-ledger.jsonl")
}

// sessionLedgerEntry is one session's row (docs/design/production-tokenomics.md
// §6). Every "IF available" field is a pointer; its sibling *Note field is
// non-empty exactly when the pointer is nil, and empty exactly when the
// pointer carries a real value — so a reader never has to guess whether a
// zero is real.
// sessionLedgerSchemaVersion: 2 adds the telemetry stamps (vision 14 §3,
// IO-13); rows without the field are the v1 legacy shape.
const sessionLedgerSchemaVersion = 2

type sessionLedgerEntry struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`

	SessionID string `json:"session_id"`
	Repo      string `json:"repo,omitempty"`
	At        string `json:"at"` // RFC3339, when this line was appended

	DurationMS   *int64 `json:"duration_ms,omitempty"`
	DurationNote string `json:"duration_note,omitempty"`

	TurnCount *int   `json:"turn_count,omitempty"`
	TurnNote  string `json:"turn_note,omitempty"`

	TokensTotal *int   `json:"tokens_total,omitempty"`
	TokensNote  string `json:"tokens_note"` // ALWAYS present — law 12: never a silent absence either

	// TranscriptSchemaNote (critic finding F4, docs/design/production-
	// tokenomics.md §8's amended non-goal) is ALWAYS present when a
	// transcript was read at all (duration/turn_count/tokens_total came
	// from parsing it): states plainly that these figures are coupled to
	// Claude Code's own transcript JSONL shape and may silently degrade
	// to "unavailable" (never a wrong number — schema-defensive parsing,
	// summarizeTranscriptBytes) if that shape changes. TranscriptTruncated
	// is set when the read hit transcriptReadMaxBytes (critic finding F5)
	// — independent of which fields still got a real value.
	TranscriptSchemaNote string `json:"transcript_schema_note,omitempty"`
	TranscriptTruncated  bool   `json:"transcript_truncated,omitempty"`

	// PayloadCostEstimateTotal / DiscoveryActionsTotal are instrument A/B's
	// counts for THIS session_id, summed from the beat log — the join in
	// action, computed once here rather than left for a separate query.
	PayloadCostEstimateTotal *int   `json:"payload_cost_estimate_total,omitempty"`
	DiscoveryActionsTotal    *int   `json:"discovery_actions_total,omitempty"`
	JoinNote                 string `json:"join_note,omitempty"`
}

// appendSessionLedger appends one entry to sessionLedgerPath() as a single
// JSON line. Best-effort: every failure is swallowed, exactly like
// emitBeat — a ledger write must never be the reason the Stop hook breaks
// its own silent/advisory contract (§5.3).
//
// Critic finding F3 (BLOCKING, phase-P fix round): the ledger can carry
// duration/turn/tokens NOTES (never raw transcript content, but see
// transcriptSummaryFrom's fixed-reason-string fix below) and a repo
// label — sensitive enough that it gets the SAME hardening
// guardstamp.go/identcache.go use for less sensitive data: 0700 dir,
// 0600 file. Previously 0755/0644, the weakest permissions of any
// cache-adjacent file this phase added.
func appendSessionLedger(entry sessionLedgerEntry) {
	path := sessionLedgerPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	data = append(data, '\n')
	_, _ = f.Write(data)
}

// buildSessionLedgerEntry is the pure, testable core: given a session id,
// repo label, the already-loaded beat log, and (optionally) a parsed
// transcript summary, it produces the ledger row. now is injected (never
// time.Now() here) per the repo's determinism convention.
func buildSessionLedgerEntry(sessionID, repo string, ts transcriptSummary, beats []heartbeat.Beat, now time.Time) sessionLedgerEntry {
	e := sessionLedgerEntry{
		SchemaVersion: sessionLedgerSchemaVersion,
		FathomVersion: buildinfo.Version(),
		SessionID:     sessionID,
		Repo:          repo,
		At:            now.UTC().Format(time.RFC3339),
	}

	if ts.DurationMS != nil {
		e.DurationMS = ts.DurationMS
	} else {
		e.DurationNote = ts.DurationNote
	}
	if ts.TurnCount != nil {
		e.TurnCount = ts.TurnCount
	} else {
		e.TurnNote = ts.TurnNote
	}
	if ts.TokensTotal != nil {
		e.TokensTotal = ts.TokensTotal
	} else {
		e.TokensNote = ts.TokensNote
	}
	// A transcript was actually read (as opposed to absent/unreadable)
	// whenever ANY of the three fields resolved to a real value — the
	// schema-coupling disclosure applies the moment parsing happened at
	// all, regardless of which individual fields it could fill in.
	if ts.DurationMS != nil || ts.TurnCount != nil || ts.TokensTotal != nil {
		e.TranscriptSchemaNote = "figures above are parsed from Claude Code's own transcript JSONL " +
			"(transcript_path) — schema-coupled and may silently degrade to \"unavailable\" (never a " +
			"wrong number) if that format changes; docs/design/production-tokenomics.md §8"
	}
	e.TranscriptTruncated = ts.Truncated

	if sessionID == "" {
		e.JoinNote = "unavailable: no session_id on this Stop hook invocation — instrument A/B beats cannot be attributed to this session"
		return e
	}
	payloadEstimate, discoveryActions := sumSessionBeats(beats, sessionID)
	e.PayloadCostEstimateTotal = &payloadEstimate
	e.DiscoveryActionsTotal = &discoveryActions
	return e
}

// sumSessionBeats sums instrument A's tokens_estimate (channel
// "payload-cost") and instrument B's action count (channel "discovery")
// over beats tagged with this exact session_id — the join in action.
func sumSessionBeats(beats []heartbeat.Beat, sessionID string) (payloadEstimateTotal, discoveryActions int) {
	for _, b := range beats {
		if b.Meta["session"] != sessionID {
			continue
		}
		switch b.Channel {
		case "payload-cost":
			if v := b.Meta["tokens_estimate"]; v != "" {
				payloadEstimateTotal += atoiSafe(v)
			}
		case "discovery":
			discoveryActions++
		}
	}
	return payloadEstimateTotal, discoveryActions
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// transcriptSummary is transcriptSummaryFrom's honest-or-absent output.
// Truncated is independent of the per-field value/note pattern (it can be
// true alongside a REAL value — a read that hit the byte cap partway
// through can still summarize everything read up to that point).
type transcriptSummary struct {
	DurationMS   *int64
	DurationNote string
	TurnCount    *int
	TurnNote     string
	TokensTotal  *int
	TokensNote   string
	Truncated    bool
}

// transcriptLine is the subset of Claude Code's session-transcript JSONL
// shape (one object per line) this instrument reads. Any field this
// process doesn't recognize is ignored (forward-compatible by
// construction) — a line that doesn't even parse as this shape is simply
// skipped, never treated as an error for the whole file.
type transcriptLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Message   *struct {
		Usage *struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// transcriptReadMaxBytes bounds how much of a transcript file this
// instrument will ever read — critic finding F5 (BLOCKING, phase-P fix
// round): an unbounded os.ReadFile on a stdin-supplied path could hang the
// Stop hook (a FIFO, /dev/zero) to its full timeout, or exhaust memory on
// a pathological file. 16MB comfortably covers a real multi-hour session
// transcript while staying far under the Stop hook's own timeout budget.
const transcriptReadMaxBytes = 16 << 20

// transcriptSummaryFrom reads path (Claude Code's transcript_path Stop
// hook field) and summarizes it: turn count (lines with type=="assistant"),
// harness-reported token totals (summed "message.usage" fields across
// every line that carries one), and duration (last minus first parseable
// timestamp). Any field this process cannot determine is left nil with an
// honest, FIXED-STRING note — an absent/unreadable file degrades every
// field, not just some.
//
// Critic finding F3 (BLOCKING, phase-P fix round): notes here NEVER embed
// err.Error() — a raw os.ReadFile error on a user-supplied path can
// contain the full $HOME, username, and repo layout, which would then sit
// in a 0600 file forever (this ledger, appendSessionLedger). Every note is
// a fixed reason string naming the CLASS of problem, never OS-level detail.
//
// Critic finding F5 (BLOCKING, phase-P fix round): the file is opened
// O_RDONLY|O_NOFOLLOW (a symlinked transcript_path is refused, not
// followed — mirrors guardstamp.go/identcache.go's own read hardening),
// Stat-checked IsRegular BEFORE any read (a FIFO or device node is
// refused outright, closing the "/dev/zero or a FIFO hangs the hook"
// risk), and capped at transcriptReadMaxBytes via io.LimitReader — a
// truncated read still summarizes what was read, with the note saying so.
func transcriptSummaryFrom(path string) transcriptSummary {
	const unavailableNoPath = "unavailable: no transcript_path on this Stop hook invocation"
	const unavailableUnreadable = "unavailable: transcript_path unreadable or not a regular file"
	if path == "" {
		return transcriptSummary{DurationNote: unavailableNoPath, TurnNote: unavailableNoPath, TokensNote: unavailableNoPath}
	}
	// O_NONBLOCK is the load-bearing part of this open: opening a FIFO
	// for O_RDONLY WITHOUT it blocks at the open() syscall itself — before
	// Stat/IsRegular ever runs — until a writer connects, which is
	// exactly the hang critic finding F5 named (a FIFO parked at
	// transcript_path would block the Stop hook to its own timeout, not
	// this function's IsRegular check). O_NONBLOCK makes the open return
	// immediately regardless of file type; the IsRegular check below then
	// refuses the FIFO outright rather than reading from it.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return transcriptSummary{DurationNote: unavailableUnreadable, TurnNote: unavailableUnreadable, TokensNote: unavailableUnreadable}
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return transcriptSummary{DurationNote: unavailableUnreadable, TurnNote: unavailableUnreadable, TokensNote: unavailableUnreadable}
	}
	data, err := io.ReadAll(io.LimitReader(f, transcriptReadMaxBytes+1))
	if err != nil {
		return transcriptSummary{DurationNote: unavailableUnreadable, TurnNote: unavailableUnreadable, TokensNote: unavailableUnreadable}
	}
	out := transcriptSummary{}
	if int64(len(data)) > transcriptReadMaxBytes {
		data = data[:transcriptReadMaxBytes]
		out.Truncated = true
	}
	rest := summarizeTranscriptBytes(data)
	out.DurationMS, out.DurationNote = rest.DurationMS, rest.DurationNote
	out.TurnCount, out.TurnNote = rest.TurnCount, rest.TurnNote
	out.TokensTotal, out.TokensNote = rest.TokensTotal, rest.TokensNote
	return out
}

func summarizeTranscriptBytes(data []byte) transcriptSummary {
	lines := splitLines(data)

	turnCount := 0
	tokensTotal := 0
	sawUsage := false
	var timestamps []time.Time

	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		var tl transcriptLine
		if err := json.Unmarshal(l, &tl); err != nil {
			continue // not this shape — skip, never abort the whole file
		}
		if tl.Type == "assistant" {
			turnCount++
		}
		if tl.Message != nil && tl.Message.Usage != nil {
			u := tl.Message.Usage
			tokensTotal += u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
			sawUsage = true
		}
		if tl.Timestamp != "" {
			if t, terr := time.Parse(time.RFC3339, tl.Timestamp); terr == nil {
				timestamps = append(timestamps, t)
			}
		}
	}

	out := transcriptSummary{}
	if len(lines) == 0 {
		out.TurnNote = "unavailable: empty transcript"
	} else {
		tc := turnCount
		out.TurnCount = &tc
	}
	if sawUsage {
		tt := tokensTotal
		out.TokensTotal = &tt
	} else {
		out.TokensNote = "unavailable: no usage data found in transcript"
	}
	if len(timestamps) >= 2 {
		sort.Slice(timestamps, func(i, j int) bool { return timestamps[i].Before(timestamps[j]) })
		dur := timestamps[len(timestamps)-1].Sub(timestamps[0]).Milliseconds()
		out.DurationMS = &dur
	} else {
		out.DurationNote = "unavailable: fewer than two timestamped transcript lines"
	}
	return out
}

// splitLines splits data on '\n' without allocating via bytes.Split's
// interface overhead — a small local helper since this file already
// avoids importing bufio/bytes for one call site.
func splitLines(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			out = append(out, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out
}
