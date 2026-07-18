// Package extractiongaplog is the flywheel store for Tier-2 → Tier-3 escalation
// events. Every time the evidence ladder escalates to a human (Tier-3), a
// GapRecord is appended here. Projections over accumulated records drive
// Tier-1 warm-up and improvementflywheel.Signal emission.
//
// # Honest scope claim
//
// Warmed entries (from WarmList) are Tier-1 SIGNAL ONLY. They are keyed to
// content hashes and carry no verdict guarantee. They NEVER satisfy
// PreconditionSatisfied in evidence-signature. Absence-shaped cache entries
// are invalidated by any change under the concern's declared constraints.
//
// # Flywheel integration
//
// Signals(recs) converts gap accumulation into improvementflywheel.Signal
// values so the gap log rides the existing proposal loop without reinventing
// a new one.
//
// # Persistence
//
// GapRecord values are appended via jsonl-log with an injected clock. The log
// is append-only; corrections are out of scope.
//
// # Concurrency
//
// GapLog is safe for concurrent use via sync.Mutex on the underlying Writer.
// Projection functions (HitRate, WarmList, Signals) are pure functions over
// slices and safe to call concurrently.
package extractiongaplog

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	improvementflywheel "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/improvement-flywheel"
	jsonllog "github.com/geoffmilleraz/agentic-kit/foundations/persistence/jsonl-log"
)

// Outcome mirrors evidence-ladder Verdict for the GapRecord.
// Re-declared here so extractiongaplog has no import cycle with evidence-ladder.
type Outcome string

const (
	// OutcomeFound means the concern's evidence was located.
	OutcomeFound Outcome = "found"
	// OutcomeAbsentProven means the concern's evidence was affirmatively
	// proven absent.
	OutcomeAbsentProven Outcome = "absent_proven"
	// OutcomeAbsentUnverified means the concern's evidence appears absent
	// but that absence was not verified.
	OutcomeAbsentUnverified Outcome = "absent_unverified"
	// OutcomeUnresolved means the concern fell through without a
	// determination either way.
	OutcomeUnresolved Outcome = "unresolved"
)

// GapRecord is one gap-log entry.
//
// CUE mirror: #GapRecord in schema.cue.
type GapRecord struct {
	// ConcernID identifies the concern that fell through to Tier-3.
	ConcernID string `json:"concern_id"`
	// Tier1KeyMissed is the Tier-1 cache key that was absent, if known.
	Tier1KeyMissed string `json:"tier1_key_missed,omitempty"`
	// ScopeSearched describes the scope Tier-2 searched.
	ScopeSearched string `json:"scope_searched"`
	// Outcome is the eventual verdict.
	Outcome Outcome `json:"outcome"`
	// FoundRefs are file:// or source:// references when outcome=="found".
	FoundRefs []string `json:"found_refs,omitempty"`
	// OccurredAt is the RFC3339 timestamp (from an injected clock).
	OccurredAt string `json:"occurred_at"`
}

// Validate returns an error if the GapRecord is malformed.
func (r GapRecord) Validate() error {
	if r.ConcernID == "" {
		return fmt.Errorf("extractiongaplog: GapRecord.ConcernID must not be empty")
	}
	if r.OccurredAt == "" {
		return fmt.Errorf("extractiongaplog: GapRecord.OccurredAt must not be empty")
	}
	switch r.Outcome {
	case OutcomeFound, OutcomeAbsentProven, OutcomeAbsentUnverified, OutcomeUnresolved:
	default:
		return fmt.Errorf("extractiongaplog: GapRecord.Outcome %q is not a valid outcome", string(r.Outcome))
	}
	return nil
}

// GapLog appends GapRecord values to a JSONL log via jsonllog.Writer.
// Safe for concurrent use (mutex in the underlying Writer).
type GapLog struct {
	w   *jsonllog.Writer
	now func() time.Time
}

// NewGapLog creates a GapLog writing to w with the given clock.
// clock must not be nil.
func NewGapLog(w io.Writer, clock func() time.Time) (*GapLog, error) {
	if clock == nil {
		return nil, fmt.Errorf("extractiongaplog: NewGapLog: clock must not be nil")
	}
	return &GapLog{w: jsonllog.NewWriter(w), now: clock}, nil
}

// Append validates and appends a GapRecord to the log.
// OccurredAt is set from the injected clock if empty.
func (g *GapLog) Append(rec GapRecord) error {
	if rec.OccurredAt == "" {
		rec.OccurredAt = g.now().UTC().Format(time.RFC3339)
	}
	if err := rec.Validate(); err != nil {
		return err
	}
	return g.w.Append(rec)
}

// Load reads all GapRecords from r via jsonllog.ReadAll.
// Partial trailing lines (torn writes) are recovered silently if acceptPartial is true.
func Load(r io.Reader, acceptPartial bool) ([]GapRecord, error) {
	var recs []GapRecord
	err := jsonllog.ReadAll(r, func(msg json.RawMessage) error {
		var rec GapRecord
		if err := json.Unmarshal(msg, &rec); err != nil {
			return fmt.Errorf("extractiongaplog: unmarshal: %w", err)
		}
		recs = append(recs, rec)
		return nil
	})
	if err != nil {
		if acceptPartial {
			// Partial trailing line is recoverable; return what was read.
			var partial *jsonllog.PartialLineError
			if isPartial(err, &partial) {
				return recs, nil
			}
		}
		return recs, fmt.Errorf("extractiongaplog: load: %w", err)
	}
	return recs, nil
}

func isPartial(err error, partial **jsonllog.PartialLineError) bool {
	if err == nil {
		return false
	}
	if p, ok := err.(*jsonllog.PartialLineError); ok {
		if partial != nil {
			*partial = p
		}
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Projections.
// ---------------------------------------------------------------------------

// HitRate returns the per-concern Tier-1 hit rate: the fraction of runs where
// Tier-1 returned a useful signal (i.e. outcome was found or absent_proven
// without escalation). In a gap log, every record represents a Tier-1 MISS.
// Therefore the hit rate is computed as:
//
//	hit_rate = 1 - (gaps_for_concern / total_runs_for_concern)
//
// Since this log only records misses, total_runs_for_concern must be supplied
// by the caller (typically from a run counter in a higher-level system).
// If totalRuns[concernID] is zero, the rate for that concern is 0.0.
//
// Returns a map from concern ID to hit rate in [0, 1].
func HitRate(recs []GapRecord, totalRuns map[string]int) map[string]float64 {
	gapCount := make(map[string]int)
	for _, r := range recs {
		gapCount[r.ConcernID]++
	}
	result := make(map[string]float64, len(gapCount))
	for id, gaps := range gapCount {
		total := totalRuns[id]
		if total <= 0 {
			result[id] = 0.0
			continue
		}
		missed := float64(gaps) / float64(total)
		if missed > 1.0 {
			missed = 1.0
		}
		result[id] = 1.0 - missed
	}
	return result
}

// WarmEntry is a candidate for Tier-1 warm-up: a (concern, pattern) pair that
// Tier-1 repeatedly missed. The Refs are file:// or source:// references from
// resolved gaps (outcome=="found") that Tier-1 could cache.
//
// IMPORTANT: Warmed entries are Tier-1 SIGNAL ONLY. They NEVER satisfy
// PreconditionSatisfied and do not represent verdicts.
type WarmEntry struct {
	// ConcernID is the concern this entry warms.
	ConcernID string
	// Refs are the found references from resolved gaps.
	Refs []string
	// MissCount is how many times Tier-1 missed this concern.
	MissCount int
}

// WarmList returns entries suitable for Tier-1 warm-up, sorted by MissCount
// descending (highest miss rate first), then by ConcernID for stability.
// Only concerns with at least one resolved "found" outcome are included.
//
// Honest caveat: callers must key warm entries to content hashes, not concern
// IDs alone. Absence-shaped entries must be invalidated on any content change
// under the concern's constraints. This function does not enforce that;
// it is the caller's responsibility.
func WarmList(recs []GapRecord) []WarmEntry {
	byID := make(map[string]*WarmEntry)
	for _, r := range recs {
		e, ok := byID[r.ConcernID]
		if !ok {
			e = &WarmEntry{ConcernID: r.ConcernID}
			byID[r.ConcernID] = e
		}
		e.MissCount++
		if r.Outcome == OutcomeFound {
			e.Refs = dedupeStrings(append(e.Refs, r.FoundRefs...))
		}
	}

	var out []WarmEntry
	for _, e := range byID {
		if len(e.Refs) > 0 { // only warm concerns that have found refs
			out = append(out, *e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MissCount != out[j].MissCount {
			return out[i].MissCount > out[j].MissCount
		}
		return out[i].ConcernID < out[j].ConcernID
	})
	return out
}

// Signals converts gap accumulation into improvementflywheel.Signal values
// so gap trends ride the existing proposal loop.
//
// A concern that has ≥2 gap records produces a PatternOperatorFriction signal
// (recurring Tier-3 escalation = a friction pattern). Concerns with only 1
// gap are informational and do not yet warrant a proposal.
func Signals(recs []GapRecord) []improvementflywheel.Signal {
	type entry struct {
		count   int
		details []string
	}
	byID := make(map[string]*entry)
	for _, r := range recs {
		e, ok := byID[r.ConcernID]
		if !ok {
			e = &entry{}
			byID[r.ConcernID] = e
		}
		e.count++
		detail := fmt.Sprintf("outcome=%s scope=%s", r.Outcome, r.ScopeSearched)
		e.details = append(e.details, detail)
	}

	// Collect concern IDs sorted for determinism.
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var signals []improvementflywheel.Signal
	for _, id := range ids {
		e := byID[id]
		if e.count < 2 {
			continue
		}
		signals = append(signals, improvementflywheel.Signal{
			Type:       improvementflywheel.PatternOperatorFriction,
			Key:        id,
			WorkflowID: "extraction-gap-log",
			Detail:     strings.Join(e.details, "; "),
		})
	}
	return signals
}

// dedupeStrings returns a deduplicated, sorted slice.
func dedupeStrings(ss []string) []string {
	seen := make(map[string]struct{}, len(ss))
	var out []string
	for _, s := range ss {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
