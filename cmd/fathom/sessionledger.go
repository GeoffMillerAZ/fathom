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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	// Arm records which measurement arm this session ran under ("on" =
	// channels live, "off" = the counterfactual OFF arm) — the field that
	// makes paired comparison possible (vision 14 §2, IO-14/15).
	Arm string `json:"arm"`
	// ArmCoverage says whether this session's MEASURED WINDOW lies entirely
	// inside the arm window it is stamped with.
	//
	// Without it the Arm field is a lie of omission, and it produced real
	// corrupt data on 2026-08-08: two sessions whose transcripts began TEN
	// DAYS before the arm was set were stamped arm="off", carrying token
	// and discovery totals that were overwhelmingly ASSISTED activity. Fed
	// to a paired comparison, rows like those make the OFF arm look
	// identical to the ON arm — manufacturing the single conclusion
	// ("fathom saves nothing") that would end the program, on evidence
	// that is simply mislabelled.
	//
	//   "full"    — the transcript begins at or after the arm was set;
	//               the whole measured window ran under this arm. The only
	//               value admissible to a paired comparison.
	//   "partial" — the transcript predates the arm. The row is retained
	//               and readable, but it is NOT control data.
	//   "unknown" — no timestamped transcript line, so coverage could not
	//               be determined. Not assumed clean (law 3).
	//   ""        — no arm was active; the question does not arise.
	ArmCoverage string `json:"arm_coverage,omitempty"`
	// ArmCoverageNote states the boundary in words when coverage is not
	// full, so a reader never has to reconstruct it from timestamps.
	ArmCoverageNote string `json:"arm_coverage_note,omitempty"`
	// VerifyCatches is the stop-hook verify pass's proven-failing count for
	// newly-claimed tasks this session — nil (omitted) when verify never
	// ran, which is UNKNOWN, never a fabricated zero (IO-15, law 3).
	VerifyCatches *int   `json:"verify_catches,omitempty"`
	Repo          string `json:"repo,omitempty"`
	At            string `json:"at"` // RFC3339, when this line was appended

	DurationMS   *int64 `json:"duration_ms,omitempty"`
	DurationNote string `json:"duration_note,omitempty"`

	TurnCount *int   `json:"turn_count,omitempty"`
	TurnNote  string `json:"turn_note,omitempty"`

	// FirstEditTurn (IQ-2): the 1-based assistant turn of the first
	// file-changing tool call — the session's warm-up length.
	FirstEditTurn *int   `json:"first_edit_turn,omitempty"`
	FirstEditNote string `json:"first_edit_note,omitempty"`

	// The exploration ledger (EX-2): both discovery modes measured from
	// this session's transcript with the same estimator.
	RawDiscoveryCalls     int  `json:"raw_discovery_calls,omitempty"`
	RawDiscoveryTokensEst *int `json:"raw_discovery_tokens_est,omitempty"`
	FathomCalls           int  `json:"fathom_calls,omitempty"`
	FathomTokensEst       *int `json:"fathom_tokens_est,omitempty"`
	RepeatedReadFiles     int  `json:"repeated_read_files,omitempty"`
	RepeatedReadTokensEst *int `json:"repeated_read_tokens_est,omitempty"`
	SidechainToolResults  int  `json:"sidechain_tool_results,omitempty"`
	// ToolSearchTurns (BP-23, ADR-032 §5) is the deferral tax this session
	// paid: assistant turns spent loading deferred MCP tool schemas instead
	// of doing work. Claude Code 2.1.x defers MCP tools while Bash is core
	// and never deferred, so this is a cost the INDEXED mode pays and the
	// raw-grep mode does not (O24) — it belongs on the exploration ledger
	// beside the two modes it distorts, as the tripwire that says when
	// fathom's own surface has grown past the harness's deferral point.
	ToolSearchTurns int    `json:"toolsearch_turns,omitempty"`
	ExplorationNote string `json:"exploration_note,omitempty"`

	TokensTotal *int   `json:"tokens_total,omitempty"`
	TokensNote  string `json:"tokens_note"` // ALWAYS present — law 12: never a silent absence either

	// The in/out/cache split (impact-page finding, 2026-08-08): TokensTotal
	// sums ALL flow including cache reads, which dominate cached sessions by
	// orders of magnitude (a 596M-token "median session" was cache reads).
	// The spend-shaped, arm-comparable figure is TokensIn+TokensOut; rows
	// without the split (pre-split binaries) render as unknown, never as
	// their inflated total.
	TokensIn          *int `json:"tokens_in,omitempty"`
	TokensOut         *int `json:"tokens_out,omitempty"`
	TokensCacheRead   *int `json:"tokens_cache_read,omitempty"`
	TokensCacheCreate *int `json:"tokens_cache_create,omitempty"`

	// SessionSource / BriefInjected / BriefBytes label the row's COHORT
	// for the vision-21 bet: fresh-with-brief (startup|clear + brief)
	// versus resumed. Joined from the SessionStart stamp at write time; a
	// missing stamp yields "" — unknown, never guessed (law 3). BriefBytes
	// is the feature's own measured cost, so "how many tokens does this
	// take" is a ledger sum, not an estimate.
	SessionSource string `json:"session_source,omitempty"`
	BriefInjected bool   `json:"brief_injected,omitempty"`
	BriefBytes    int    `json:"brief_bytes,omitempty"`

	// IntentLine / IntentActed are K5's pair (EP-24): did this session SEE
	// the attention line, and did it then reach for an intent verb? Acted
	// is derived from beats in this session's window, never from prose in
	// the transcript — law 14, narrative is a lens and never evidence.
	IntentLine  bool `json:"intent_line,omitempty"`
	IntentActed bool `json:"intent_acted,omitempty"`

	// ArmWindow carries the ARM-COVERED span of a session that began
	// before the arm was set (arm_coverage: partial) — the amended unit of
	// comparison (production-tokenomics.md §5, 2026-08-12). Whole-session
	// figures above stay exactly as they were; the window is measured from
	// the same transcript with the same estimators, restricted to lines
	// timestamped at or after the arm's set_at, so the comparison never
	// mixes assisted and unassisted activity.
	ArmWindow *armWindowSummary `json:"arm_window,omitempty"`

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
		Arm:           currentArmLabel(now),
		SessionID:     sessionID,
		Repo:          repo,
		At:            now.UTC().Format(time.RFC3339),
	}

	e.ArmCoverage, e.ArmCoverageNote = armCoverage(ts.FirstTS, now)

	st := readSessionStamp(sessionID)
	e.SessionSource, e.BriefInjected, e.BriefBytes = st.Source, st.BriefInjected, st.BriefBytes
	e.IntentLine = st.IntentLine

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
		e.TokensIn, e.TokensOut = ts.TokensIn, ts.TokensOut
		e.TokensCacheRead, e.TokensCacheCreate = ts.TokensCacheRead, ts.TokensCacheCreate
	} else {
		e.TokensNote = ts.TokensNote
	}
	if ts.FirstEditTurn != nil {
		e.FirstEditTurn = ts.FirstEditTurn
	} else {
		e.FirstEditNote = ts.FirstEditNote
	}
	e.RawDiscoveryCalls = ts.RawDiscoveryCalls
	e.RawDiscoveryTokensEst = ts.RawDiscoveryTokensEst
	e.FathomCalls = ts.FathomCalls
	e.FathomTokensEst = ts.FathomTokensEst
	e.RepeatedReadFiles = ts.RepeatedReadFiles
	e.RepeatedReadTokensEst = ts.RepeatedReadTokensEst
	e.SidechainToolResults = ts.SidechainToolResults
	e.ToolSearchTurns = ts.ToolSearchTurns
	e.ExplorationNote = ts.ExplorationNote
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
	payloadEstimate, discoveryActions, intentActed := sumSessionBeats(beats, sessionID)
	e.PayloadCostEstimateTotal = &payloadEstimate
	e.DiscoveryActionsTotal = &discoveryActions
	e.IntentActed = intentActed
	return e
}

// sumSessionBeats sums instrument A's tokens_estimate (channel
// "payload-cost") and instrument B's action count (channel "discovery")
// over beats tagged with this exact session_id — the join in action.
func sumSessionBeats(beats []heartbeat.Beat, sessionID string) (payloadEstimateTotal, discoveryActions int, intentActed bool) {
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
		case "intent-cli", "doctor":
			// K5 (EP-24): the session reached for an intent instrument.
			// A beat, not a transcript phrase — an agent SAYING it will
			// check the corpus is narrative, and narrative is never
			// evidence (law 14).
			intentActed = true
		}
	}
	return payloadEstimateTotal, discoveryActions, intentActed
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
	// The in/out/cache split — see sessionLedgerEntry's field of the same
	// names. TokensTotal keeps its historical meaning (ALL flow).
	TokensIn          *int
	TokensOut         *int
	TokensCacheRead   *int
	TokensCacheCreate *int
	// FirstEditTurn (IQ-2): the assistant turn index (1-based) of the first
	// file-changing tool_use (Edit/Write/MultiEdit/NotebookEdit) — the
	// session's warm-up length. nil with FirstEditNote when the session
	// never edited (or the transcript carries no content blocks).
	FirstEditTurn *int
	FirstEditNote string
	// The exploration ledger (EX-1, docs/plans/exploration-ledger.md):
	// both discovery modes measured from the SAME transcript with the same
	// estimator (result bytes / 4 ≈ tokens; an estimate, labeled so).
	RawDiscoveryCalls     int    // Grep / Glob / Read tool calls
	RawDiscoveryTokensEst *int   // estimated tokens their results injected
	FathomCalls           int    // mcp__fathom__* tool calls
	FathomTokensEst       *int   // estimated tokens their results injected
	RepeatedReadFiles     int    // files Read 2+ times (the shared index answers once)
	RepeatedReadTokensEst *int   // estimated tokens spent on 2nd+ reads
	SidechainToolResults  int    // tool results inside sidechains (subagents)
	ToolSearchTurns       int    // assistant turns spent loading deferred MCP schemas (BP-23/O24)
	ExplorationNote       string // the estimator disclosure, or the absence reason
	Truncated             bool
	// FirstTS is the earliest timestamped line in the transcript — when
	// this session's measured window actually BEGAN. The duration
	// computation already sorts for it; keeping it is what lets the arm
	// stamp be honest about coverage (see armCoverage).
	FirstTS *time.Time
}

// transcriptLine is the subset of Claude Code's session-transcript JSONL
// shape (one object per line) this instrument reads. Any field this
// process doesn't recognize is ignored (forward-compatible by
// construction) — a line that doesn't even parse as this shape is simply
// skipped, never treated as an error for the whole file.
type transcriptLine struct {
	Type        string `json:"type"`
	Timestamp   string `json:"timestamp"`
	IsSidechain bool   `json:"isSidechain"`
	Message     *struct {
		Usage *struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
		Content []struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			ID        string `json:"id"`
			ToolUseID string `json:"tool_use_id"`
			Input     *struct {
				FilePath string `json:"file_path"`
			} `json:"input"`
			Content json.RawMessage `json:"content"`
		} `json:"content"`
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
	// Take the WHOLE summary, then re-apply the only thing this function
	// knows that the parser does not.
	//
	// This used to copy three field pairs by hand, and every field added to
	// transcriptSummary afterwards was computed by the parser and then
	// silently dropped here. Measured 2026-08-12: FirstTS (so arm_coverage
	// read "unknown" and NO OFF-arm session was admissible), the
	// in/out/cache token split (so every row counted as "pre-split" and no
	// arm could have a spend-shaped median), FirstEditTurn, and the entire
	// exploration ledger. Four instruments reported "rows predate the
	// instrument" when the instrument was wired and its output was being
	// thrown away three lines later.
	//
	// Whole-struct assignment is what makes that class impossible: a new
	// field is carried by construction rather than by remembering.
	truncated := out.Truncated
	out = summarizeTranscriptBytes(data)
	out.Truncated = truncated
	return out
}

func summarizeTranscriptBytes(data []byte) transcriptSummary {
	return summarizeTranscriptWindow(data, nil)
}

// summarizeTranscriptWindow is the same fold restricted to lines whose
// timestamp is at or after `since` (nil = no restriction — the whole
// transcript).
//
// This is what makes an ARM-COVERED WINDOW of a resumed session measurable
// (production-tokenomics.md §5, amended 2026-08-12). The owner's practice
// is one session resumed across days, so a session-level unit produced a
// control arm that could never admit anything; the unit had to bend to the
// work rather than the work to the unit.
//
// Two conservative choices, both in the noisy-not-silent direction:
//   - a line with NO parseable timestamp is EXCLUDED when since is set —
//     unattributable activity must not leak into an arm's figures;
//   - a tool_use before the boundary whose result lands after it is
//     excluded too (the use was never registered), so boundary-straddling
//     pairs count in neither window.
func summarizeTranscriptWindow(data []byte, since *time.Time) transcriptSummary {
	lines := splitLines(data)

	turnCount := 0
	tokensTotal := 0
	tokensIn, tokensOut, tokensCacheRead, tokensCacheCreate := 0, 0, 0, 0
	sawUsage := false
	var timestamps []time.Time

	var firstEditTurn *int
	// EX-1 pairing state: tool_use id → its class, and Read file paths.
	type useInfo struct {
		class    string // "raw" | "fathom" | ""
		filePath string
	}
	uses := map[string]useInfo{}
	resultsSeen := map[string]int{}
	var (
		rawCalls, fathomCalls     int
		rawTokens, fathomTokens   int
		sawResult                 bool
		repeatFiles, repeatTokens int
		sidechainResults          int
		toolSearchCount           int
	)
	for _, l := range lines {
		if len(l) == 0 {
			continue
		}
		var tl transcriptLine
		if err := json.Unmarshal(l, &tl); err != nil {
			continue // not this shape — skip, never abort the whole file
		}
		if since != nil {
			t, terr := time.Parse(time.RFC3339, tl.Timestamp)
			if terr != nil || t.Before(*since) {
				continue
			}
		}
		if tl.Type == "assistant" {
			turnCount++
			if toolSearchTurns(tl) > 0 {
				toolSearchCount++
			}
			if tl.Message != nil {
				for _, c := range tl.Message.Content {
					if c.Type != "tool_use" {
						continue
					}
					if firstEditTurn == nil && isEditTool(c.Name) {
						t := turnCount
						firstEditTurn = &t
					}
					u := useInfo{}
					switch {
					case c.Name == "Read" || c.Name == "Grep" || c.Name == "Glob":
						u.class = "raw"
						rawCalls++
						if c.Name == "Read" && c.Input != nil && c.Input.FilePath != "" {
							u.filePath = c.Input.FilePath
						}
					case strings.HasPrefix(c.Name, "mcp__fathom__"):
						u.class = "fathom"
						fathomCalls++
					}
					if c.ID != "" && u.class != "" {
						uses[c.ID] = u
					}
				}
			}
		}
		if tl.Type == "user" && tl.Message != nil {
			for _, c := range tl.Message.Content {
				if c.Type != "tool_result" || c.ToolUseID == "" {
					continue
				}
				u, ok := uses[c.ToolUseID]
				if !ok {
					continue
				}
				if tl.IsSidechain {
					sidechainResults++
				}
				est := len(c.Content) / 4
				sawResult = true
				switch u.class {
				case "raw":
					rawTokens += est
					if u.filePath != "" {
						resultsSeen[u.filePath]++
						if n := resultsSeen[u.filePath]; n >= 2 {
							repeatTokens += est
							if n == 2 {
								repeatFiles++
							}
						}
					}
				case "fathom":
					fathomTokens += est
				}
			}
		}
		if tl.Message != nil && tl.Message.Usage != nil {
			u := tl.Message.Usage
			tokensTotal += u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
			tokensIn += u.InputTokens
			tokensOut += u.OutputTokens
			tokensCacheRead += u.CacheReadInputTokens
			tokensCacheCreate += u.CacheCreationInputTokens
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
		ti, to, tcr, tcc := tokensIn, tokensOut, tokensCacheRead, tokensCacheCreate
		out.TokensIn, out.TokensOut = &ti, &to
		out.TokensCacheRead, out.TokensCacheCreate = &tcr, &tcc
	} else {
		out.TokensNote = "unavailable: no usage data found in transcript"
	}
	if firstEditTurn != nil {
		out.FirstEditTurn = firstEditTurn
	} else if turnCount > 0 {
		out.FirstEditNote = "no file-changing tool call in this transcript"
	} else {
		out.FirstEditNote = "unavailable: no assistant turns parsed"
	}
	out.RawDiscoveryCalls = rawCalls
	out.FathomCalls = fathomCalls
	out.ToolSearchTurns = toolSearchCount
	out.RepeatedReadFiles = repeatFiles
	out.SidechainToolResults = sidechainResults
	if sawResult {
		rt, ft, rp := rawTokens, fathomTokens, repeatTokens
		out.RawDiscoveryTokensEst = &rt
		out.FathomTokensEst = &ft
		out.RepeatedReadTokensEst = &rp
		out.ExplorationNote = "token figures are an estimate (result bytes / 4), same estimator for both modes"
	} else {
		out.ExplorationNote = "no tool results parsed - exploration not measured"
	}
	if len(timestamps) >= 2 {
		sort.Slice(timestamps, func(i, j int) bool { return timestamps[i].Before(timestamps[j]) })
		dur := timestamps[len(timestamps)-1].Sub(timestamps[0]).Milliseconds()
		out.DurationMS = &dur
		first := timestamps[0]
		out.FirstTS = &first
	} else if len(timestamps) == 1 {
		first := timestamps[0]
		out.FirstTS = &first
		out.DurationNote = "unavailable: fewer than two timestamped transcript lines"
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

// toolSearchTurns reports how many deferred-schema loads one transcript
// line carries — the count is per LINE, and the caller increments a TURN
// counter when it is non-zero, because a turn spent loading two schemas is
// still one turn of latency and one full cache-read.
//
// BP-23 / ADR-032 §5 (O24): Claude Code 2.1.x defers MCP tool schemas, so
// every fathom-using session opens by spending one or two whole turns
// loading them — at roughly 28k cache-read tokens each — while the grep arm
// pays nothing, because Bash is a core tool and is never deferred. July's
// transcripts contain zero of these. That makes this counter two things at
// once: a correction term the exploration ledger's two modes need to be
// comparable, and the tripwire that fires when fathom's own agent surface
// has grown past whatever size the harness starts deferring at.
//
// It counts the harness's own tool, never a fathom tool: a session that
// never loads a deferred schema reads a true zero here, not an absence.
func toolSearchTurns(tl transcriptLine) int {
	if tl.Type != "assistant" || tl.Message == nil {
		return 0
	}
	n := 0
	for _, c := range tl.Message.Content {
		if c.Type == "tool_use" && c.Name == "ToolSearch" {
			n++
		}
	}
	return n
}

// isEditTool reports whether a tool_use name changes files — the boundary
// that ends a session's warm-up phase (IQ-2).
func isEditTool(name string) bool {
	switch name {
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		return true
	}
	return false
}

// armWindowSummary is the arm-covered span's own fold — the amended unit
// of comparison (production-tokenomics.md §5, 2026-08-12). Numerator and
// denominator always come from the same parsed span, so a bounded read
// yields a coherent per-turn RATE even when it cannot yield a total.
type armWindowSummary struct {
	Since string `json:"since"` // the arm's set_at (RFC3339)
	Turns *int   `json:"turns,omitempty"`

	TokensIn        *int `json:"tokens_in,omitempty"`
	TokensOut       *int `json:"tokens_out,omitempty"`
	TokensCacheRead *int `json:"tokens_cache_read,omitempty"`

	RawDiscoveryCalls     int  `json:"raw_discovery_calls,omitempty"`
	RawDiscoveryTokensEst *int `json:"raw_discovery_tokens_est,omitempty"`
	FathomCalls           int  `json:"fathom_calls,omitempty"`
	FathomTokensEst       *int `json:"fathom_tokens_est,omitempty"`

	// Truncated: the tail-bounded read did not reach back to the arm
	// boundary, so the window's OLDEST activity is missing. Such a window
	// is disclosed and EXCLUDED from the paired comparison — a partial
	// window admitted as a unit would be the truncation bug all over
	// again, one level down.
	Truncated bool   `json:"truncated,omitempty"`
	Note      string `json:"note,omitempty"`
}

// windowSummaryFrom computes the arm-covered window: it reads the TAIL of
// the transcript (the window is by definition the newest span — the head
// read the whole-session summary uses would be exactly backwards here),
// aligns to a line boundary, and folds only lines at or after since.
//
// The bool is windowTruncated: the tail did not reach back to the arm
// boundary, so the window is incomplete at its old end.
func windowSummaryFrom(path string, since time.Time, capBytes int64) (transcriptSummary, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return transcriptSummary{}, false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return transcriptSummary{}, false, err
	}

	off := int64(0)
	tailBounded := false
	if st.Size() > capBytes {
		off = st.Size() - capBytes
		tailBounded = true
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return transcriptSummary{}, false, err
	}
	data, err := io.ReadAll(io.LimitReader(f, capBytes))
	if err != nil {
		return transcriptSummary{}, false, err
	}
	if off > 0 {
		// Drop the partial first line the seek landed inside.
		if i := bytesIndexByte(data, '\n'); i >= 0 {
			data = data[i+1:]
		} else {
			return transcriptSummary{}, false, fmt.Errorf("no line boundary in the tail read")
		}
	}

	sum := summarizeTranscriptWindow(data, &since)

	// The window is only COMPLETE if the tail reaches back to (or past)
	// the boundary. The filtered FirstTS is >= since by construction, so
	// probe the tail's own first parseable timestamp instead.
	windowTruncated := false
	if tailBounded {
		windowTruncated = true // pessimistic until proven otherwise
		for _, l := range splitLines(data) {
			if len(l) == 0 {
				continue
			}
			var tl transcriptLine
			if json.Unmarshal(l, &tl) != nil {
				continue
			}
			if t, terr := time.Parse(time.RFC3339, tl.Timestamp); terr == nil {
				windowTruncated = t.After(since)
				break
			}
		}
	}
	return sum, windowTruncated, nil
}

func bytesIndexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

// buildArmWindow converts the window fold into the row's arm_window field.
// A window with zero turns yields nil — nothing was measured, so nothing
// is claimed (law 3).
func buildArmWindow(sum transcriptSummary, since time.Time, truncated bool) *armWindowSummary {
	if sum.TurnCount == nil || *sum.TurnCount == 0 {
		return nil
	}
	w := &armWindowSummary{
		Since:                 since.UTC().Format(time.RFC3339),
		Turns:                 sum.TurnCount,
		TokensIn:              sum.TokensIn,
		TokensOut:             sum.TokensOut,
		TokensCacheRead:       sum.TokensCacheRead,
		RawDiscoveryCalls:     sum.RawDiscoveryCalls,
		RawDiscoveryTokensEst: sum.RawDiscoveryTokensEst,
		FathomCalls:           sum.FathomCalls,
		FathomTokensEst:       sum.FathomTokensEst,
		Truncated:             truncated,
	}
	if truncated {
		w.Note = "the tail-bounded transcript read did not reach back to the arm boundary — the window's oldest activity is missing, so this window is excluded from the paired comparison"
	}
	return w
}
