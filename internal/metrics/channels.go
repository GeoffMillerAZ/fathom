package metrics

// channels.go — Q-11 (docs/plans/expert-suite-q.md): `fathom metrics
// channels`'s pure aggregation core over the hooks beat log — injection
// coverage, guarded pre/post, dedup, module serves. This is the SAME
// beat-shape logic cmd/fathom/research_report.go's buildResearchReport
// already implements for the "edit-hook" channel (phase P) — reimplemented
// HERE as the canonical, reusable form (internal/metrics cannot import
// cmd/fathom's research_report.go — it is package main). The design's own
// "not measurable" wording is carried through VERBATIM (see the exact note
// strings below, pinned directly by channels_test.go).

import (
	"strconv"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// ChannelsReport is AggregateChannels' output — injection coverage,
// guarded pre/post, dedup, module serves.
type ChannelsReport struct {
	Since string `json:"since"`
	Until string `json:"until"`

	InjectionSeen         int      `json:"injection_seen"`
	InjectionAnswered     int      `json:"injection_answered"`
	InjectionCoverageRate *float64 `json:"injection_coverage_rate"`
	InjectionCoverageNote string   `json:"injection_coverage_note,omitempty"`

	ObligationsServed      int    `json:"obligations_served"`
	ObligationsEvictedNote string `json:"obligations_evicted_note"`

	GuardPreSeen         int    `json:"guard_pre_seen"`
	GuardPreServed       int    `json:"guard_pre_served"`
	GuardPostSeen        int    `json:"guard_post_seen"`
	GuardPostServed      int    `json:"guard_post_served"`
	GuardDedupSuppressed int    `json:"guard_dedup_suppressed"`
	GuardCoverageNote    string `json:"guard_coverage_note"`

	ModuleRulesServed map[string]int `json:"module_rules_served,omitempty"`
	ModuleServingNote string         `json:"module_serving_note"`
}

// obligationsEvictedNote is phase P's exact string
// (cmd/fathom/research_report.go's ObligationsEvictedNote) — carried
// through VERBATIM per docs/design/observability-surfaces.md §6 rule 2.
const obligationsEvictedNote = "not measurable: no eviction event is logged anywhere in fathom today " +
	"(the edit-hook beat log records seen/answered/error only)"

// AggregateChannels computes injection coverage, guarded pre/post, dedup,
// and module-served counts from beats (already read from the hooks beat
// log by the caller) in [since, until] — the exact accounting
// buildResearchReport's edit-hook section performs, moved to its canonical
// home.
func AggregateChannels(beats []heartbeat.Beat, since, until time.Time) ChannelsReport {
	r := ChannelsReport{
		Since:                  since.UTC().Format(time.RFC3339),
		Until:                  until.UTC().Format(time.RFC3339),
		ObligationsEvictedNote: obligationsEvictedNote,
	}

	seen, answered, servedEmitted := 0, 0, 0
	preSeen, preServed, postSeen, postServed, dedupSuppressed := 0, 0, 0, 0, 0
	moduleServed := map[string]int{}
	for _, b := range beats {
		if b.Channel != "edit-hook" || b.At.Before(since) || b.At.After(until) {
			continue
		}
		// Phase-O addition: a beat before this feature existed (or one
		// this package itself never tags) carries no phase at all —
		// treated as "post", since every pre-phase-O edit-hook
		// invocation WAS the post/default hook.
		phase := b.Meta["phase"]
		if phase == "" {
			phase = "post"
		}
		switch b.Kind {
		case "seen":
			if phase == "pre" {
				preSeen++
				continue
			}
			seen++
			postSeen++
		case "answered":
			if phase == "pre" {
				if b.Meta["emitted"] == "true" {
					preServed++
				}
				continue
			}
			answered++
			if b.Meta["emitted"] == "true" {
				servedEmitted++
				postServed++
			}
		case "dedup-suppressed":
			n := 1
			if cs := b.Meta["count"]; cs != "" {
				if parsed, err := strconv.Atoi(cs); err == nil && parsed > 0 {
					n = parsed
				}
			}
			dedupSuppressed += n
		case "module-served":
			if m := b.Meta["module"]; m != "" {
				n := 1
				if rs := b.Meta["rules"]; rs != "" {
					if parsed, err := strconv.Atoi(rs); err == nil && parsed > 0 {
						n = parsed
					}
				}
				moduleServed[m] += n
			}
		}
	}

	r.InjectionSeen, r.InjectionAnswered, r.ObligationsServed = seen, answered, servedEmitted
	if pct, note := Ratio(answered, seen); note != "" {
		r.InjectionCoverageNote = note
	} else {
		rate := *pct / 100
		r.InjectionCoverageRate = &rate
	}

	// IT-2: preSeen==0 and an empty moduleServed both have an innocent
	// explanation an OFF measurement arm covering this window supplies —
	// hook channels are silent BY DESIGN under the arm, not because a
	// guard hook or module binding is missing. Check once and reuse: a
	// wrong "guard hook is not installed" note under an active arm is
	// itself a false-defect report, exactly the failure mode this note
	// exists to avoid in the first place.
	armNote := ""
	if st, active := ArmStateForWindow("", since, until); active {
		armNote = st.Note()
	}

	r.GuardPreSeen, r.GuardPreServed, r.GuardPostSeen, r.GuardPostServed, r.GuardDedupSuppressed = preSeen, preServed, postSeen, postServed, dedupSuppressed
	switch {
	case preSeen == 0 && armNote != "":
		r.GuardCoverageNote = armNote
	case preSeen == 0:
		r.GuardCoverageNote = "not measurable: no PreToolUse edit-hook (--pre) invocations observed in window — the guard hook is not installed, or no PreToolUse event fired at all"
	case preServed == 0:
		r.GuardCoverageNote = "guard hook fired but served 0 — no configured guard matched any edited file/command in window"
	default:
		r.GuardCoverageNote = "guard hook fired and served in window"
	}

	if len(moduleServed) > 0 {
		r.ModuleRulesServed = moduleServed
	} else if armNote != "" {
		r.ModuleServingNote = armNote
	} else {
		r.ModuleServingNote = "not measurable: no module-tagged rule was served in window — no .fathom/modules.cue binding is configured anywhere observed, or no bound module is mounted on the daemon"
	}

	return r
}
