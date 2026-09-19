package findings

// telemetry.go — SU-16 (docs/vision/31-substrate.md §3 D8 item 3, ADR-041
// decision 8, plan `docs/plans/substrate.md` §D): "per-rule effectiveness
// telemetry after promotion — served, touched, disputed, last fired — so a
// lifted rule can retire." RuleTelemetry is a pure function over already-
// fetched claims/events/beats, exactly like every other file in this
// package — the caller (cmd/fathom's `findings status`) does the I/O
// (reading the hooks beat log via the SAME readBeatsFor/beatLogPath shared
// resolver cmd/fathom/metrics_channels.go already uses).
//
// No beat carries a claim id today — the edit hook's "answered"/
// "finding_touched" beats are per-hook-call aggregates, keyed by file, not
// by which specific claim rode the payload (module-served is the closest
// thing, and it only fires for module-tagged rules). RuleTelemetry's
// Served/Touched are therefore the same kind of NAMED, honest approximation
// disagreementsByRule already uses for a different join this layer cannot
// make exactly: a beat is attributed to every claim whose own Scope glob(s)
// match the beat's file. Disputed needs no approximation at all — a
// core.ClaimEvent already carries its own ClaimID.

import (
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// Telemetry is one claim's post-promotion firing record.
type Telemetry struct {
	// Served counts "edit-hook answered, emitted=true" beats (any phase —
	// the pre-edit guard preview and the post-edit render both count as
	// "this rule's file was in front of the agent") whose file matches the
	// claim's own scope.
	Served int `json:"served"`
	// Touched counts "edit-hook finding_touched" beats matching scope — the
	// finding-specific channel (edithook.go's renderFindingsBlock).
	Touched int `json:"touched"`
	// Disputed counts "refuted" claim_events recorded against this EXACT
	// claim id — exact, no scope-matching approximation needed.
	Disputed int `json:"disputed"`
	// LastFired is the latest Served-or-Touched beat timestamp — a dispute
	// is a verdict, not a firing, so it never advances LastFired. Zero when
	// neither ever fired.
	LastFired time.Time `json:"last_fired,omitempty"`
}

// RuleTelemetry computes SU-16's per-claim firing record for every claim in
// claims. beats is the hooks beat log the caller already read (best-effort
// — an absent/unreadable log degrades to nil, and every claim's Telemetry
// is then all-zero, honestly, never fabricated).
func RuleTelemetry(claims []core.SemanticClaim, events []core.ClaimEvent, beats []heartbeat.Beat) map[string]Telemetry {
	out := make(map[string]Telemetry, len(claims))
	matchers := make(map[string]*filter.Matcher, len(claims))
	for _, cl := range claims {
		if _, exists := out[cl.ClaimID]; exists {
			continue
		}
		out[cl.ClaimID] = Telemetry{}
		if len(cl.Scope) > 0 {
			matchers[cl.ClaimID] = filter.New(cl.Scope)
		}
	}

	for _, ev := range events {
		if ev.Verdict != "refuted" {
			continue
		}
		t, ok := out[ev.ClaimID]
		if !ok {
			continue // a dispute on a claim the caller didn't ask about
		}
		t.Disputed++
		out[ev.ClaimID] = t
	}

	for _, b := range beats {
		if b.Channel != "edit-hook" {
			continue
		}
		file := b.Meta["file"]
		if file == "" {
			continue
		}
		switch b.Kind {
		case "answered":
			if b.Meta["emitted"] != "true" {
				continue
			}
			bumpFired(out, matchers, file, b.At, fireServed)
		case "finding_touched":
			bumpFired(out, matchers, file, b.At, fireTouched)
		}
	}

	return out
}

func fireServed(t *Telemetry)  { t.Served++ }
func fireTouched(t *Telemetry) { t.Touched++ }

// bumpFired applies inc to every claim whose scope matcher matches file,
// and advances LastFired to at when at is the newest firing seen so far.
func bumpFired(out map[string]Telemetry, matchers map[string]*filter.Matcher, file string, at time.Time, inc func(*Telemetry)) {
	for id, m := range matchers {
		if m == nil || !m.Match(file) {
			continue
		}
		t := out[id]
		inc(&t)
		if t.LastFired.IsZero() || at.After(t.LastFired) {
			t.LastFired = at
		}
		out[id] = t
	}
}

// telemetryFireWindow is the "never fired in 30 days" window `findings
// status`'s rules_never_fired_30d counts against (docs/plans/substrate.md
// SU-16).
const telemetryFireWindow = 30 * 24 * time.Hour

// topFiredCap bounds top_fired — a report line, not an unbounded dump.
const topFiredCap = 10

// FiredRule is one row of `findings status`'s top_fired list.
type FiredRule struct {
	ClaimID   string `json:"claim_id"`
	Served    int    `json:"served"`
	Touched   int    `json:"touched"`
	LastFired string `json:"last_fired,omitempty"` // RFC3339, "" when it never fired
}

// summarizeTelemetry computes rules_never_fired_30d (currently-served claims
// with zero fires, or whose last fire predates now-30d) and top_fired (the
// claims that DID fire, ranked served+touched desc then claim id, capped at
// topFiredCap) — `findings status`'s SU-16 extension. now is an injected
// clock: this package never calls time.Now() itself.
func summarizeTelemetry(claims []core.SemanticClaim, telemetry map[string]Telemetry, now time.Time) (neverFired30d int, topFired []FiredRule) {
	threshold := now.Add(-telemetryFireWindow)
	var fired []FiredRule
	for _, cl := range claims {
		if !cl.Status.Served() {
			continue // a superseded/refuted claim is not a "rule" to watch for silence
		}
		t := telemetry[cl.ClaimID]
		fires := t.Served + t.Touched
		if fires == 0 || t.LastFired.Before(threshold) {
			neverFired30d++
		}
		if fires > 0 {
			lastFired := ""
			if !t.LastFired.IsZero() {
				lastFired = t.LastFired.UTC().Format(time.RFC3339)
			}
			fired = append(fired, FiredRule{ClaimID: cl.ClaimID, Served: t.Served, Touched: t.Touched, LastFired: lastFired})
		}
	}
	sortFiredRules(fired)
	if len(fired) > topFiredCap {
		fired = fired[:topFiredCap]
	}
	return neverFired30d, fired
}

func sortFiredRules(fired []FiredRule) {
	// insertion sort is fine at topFiredCap-ish scale and keeps this file
	// import-free of "sort" beyond what the rest of the package already uses
	// — kept explicit rather than clever.
	for i := 1; i < len(fired); i++ {
		j := i
		for j > 0 && firedLess(fired[j], fired[j-1]) {
			fired[j], fired[j-1] = fired[j-1], fired[j]
			j--
		}
	}
}

func firedLess(a, b FiredRule) bool {
	fa, fb := a.Served+a.Touched, b.Served+b.Touched
	if fa != fb {
		return fa > fb
	}
	return a.ClaimID < b.ClaimID
}

// BuildStatusWithTelemetry is BuildStatus extended with SU-16's telemetry
// fields (RulesNeverFired30d, TopFired) computed from beats. now is an
// injected clock. cmd/fathom's `findings status` calls this instead of
// BuildStatus; BuildStatus itself is untouched (SU-16 is additive).
func BuildStatusWithTelemetry(claims []core.SemanticClaim, events []core.ClaimEvent, beats []heartbeat.Beat, now time.Time) StatusReport {
	r := BuildStatus(claims, events)
	telemetry := RuleTelemetry(claims, events, beats)
	r.RulesNeverFired30d, r.TopFired = summarizeTelemetry(claims, telemetry, now)
	return r
}
