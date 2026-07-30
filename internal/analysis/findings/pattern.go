// Package findings derives investigable precedent from persisted review
// findings (docs/design/findings-as-evidence.md) — a leaf internal/analysis
// package over already-parsed inputs ([]core.SemanticClaim,
// []core.ClaimEvent), deterministic and I/O-free, exactly like
// internal/analysis/review and internal/metrics. Nothing here does storage:
// patterns, resolution outcomes, tier promotion, and the neighborhood
// ranking are all COMPUTED ON READ from claims already fetched by the
// caller — never a second source of truth (§4 honesty rule 2).
package findings

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// MemberStatus is a pattern member's serving state, mapped from the
// underlying claim's core.ClaimStatus into the three states a pattern
// cares about (docs/design/findings-as-evidence.md §3.3).
type MemberStatus string

const (
	// MemberLive: the claim is currently served (fresh/stale/contradicted
	// — core.ClaimStatus.Served()).
	MemberLive MemberStatus = "live"
	// MemberSuperseded: a later review run no longer emitted this finding
	// — fixed (or its scope disappeared; see ResolutionKind).
	MemberSuperseded MemberStatus = "superseded"
	// MemberRefuted: a human verdict rejected the finding.
	MemberRefuted MemberStatus = "refuted"
)

// memberStatusFrom maps a claim's core.ClaimStatus to a MemberStatus.
func memberStatusFrom(s core.ClaimStatus) MemberStatus {
	switch s {
	case core.ClaimRefuted:
		return MemberRefuted
	case core.ClaimSuperseded:
		return MemberSuperseded
	default:
		// fresh, stale, contradicted — anything Served() is live.
		return MemberLive
	}
}

// PatternMember is one finding-claim belonging to a Pattern.
type PatternMember struct {
	ClaimID string
	Subject string // the site identifier (dimension#symbol-or-file)
	Scope   []string
	Status  MemberStatus

	// ResolutionKind/ResolutionCommit (docs/design/findings-as-evidence.md
	// §3.4) are carried straight from the member's own "superseded"
	// claim_event, when one exists — "" for a live or refuted member, or
	// a superseded member whose event predates resolution classification
	// (additive migration — see internal/store's conditional column add).
	ResolutionKind   string
	ResolutionCommit string

	CompiledAt          time.Time
	CompiledAtWatermark string
}

// HasIndependentCodeChangeResolution reports whether this member carries a
// recorded, code-change resolution — the unit §3.5's promotion rule counts
// ("≥3 members with independent recorded code-change resolutions").
func (m PatternMember) HasIndependentCodeChangeResolution() bool {
	return m.Status == MemberSuperseded && m.ResolutionKind == core.ResolutionCodeChange
}

// Pattern is a derived, deterministic grouping of findings sharing (class,
// normalized rule shape) across >=2 distinct sites (docs/design/findings-
// as-evidence.md §3.3) — computed fresh from claims on every call, never
// cached or persisted as a second source of truth.
type Pattern struct {
	Class     core.FindingClass
	RuleShape string
	Members   []PatternMember // sorted by ClaimID, deterministic

	// FirstSeenAt/LastSeenAt and their watermarks are the min/max of every
	// member's CompiledAt/CompiledAtWatermark — the pattern's own
	// observed lifetime, not asserted.
	FirstSeenAt        time.Time
	LastSeenAt         time.Time
	FirstSeenWatermark string
	LastSeenWatermark  string
}

// ResolvedCodeChangeCount returns how many members carry an independent,
// recorded code-change resolution — the promotion rule's own numerator
// (§3.5, tier.go).
func (p Pattern) ResolvedCodeChangeCount() int {
	n := 0
	for _, m := range p.Members {
		if m.HasIndependentCodeChangeResolution() {
			n++
		}
	}
	return n
}

// SiteCount returns the number of DISTINCT sites (Subject values) this
// pattern's members occupy — always >=2 for any Pattern DerivePatterns
// returns (the grouping's own admission bar).
func (p Pattern) SiteCount() int {
	sites := map[string]bool{}
	for _, m := range p.Members {
		sites[m.Subject] = true
	}
	return len(sites)
}

// groupKey is the pattern grouping key (§3.3): (class, normalized rule
// shape). Stable under re-derivation — an unchanged claim set always
// regroups into identical patterns.
type groupKey struct {
	Class core.FindingClass
	Shape string
}

// DerivePatterns groups claims (expected already filtered to whatever kind
// the caller cares about — typically kind=="finding") by (Class,
// NormalizeRuleShape(Rule)) and keeps only groups spanning >=2 DISTINCT
// sites (Subject values) — a single-site "pattern" is just one finding,
// not recurrence. events is the full claim_events population (typically
// ClaimEventsSince over a wide window); DerivePatterns looks up each
// member's own resolution by ClaimID. Deterministic: sorted patterns,
// sorted members within each.
func DerivePatterns(claims []core.SemanticClaim, events []core.ClaimEvent) []Pattern {
	eventsByClaim := make(map[string][]core.ClaimEvent, len(events))
	for _, ev := range events {
		eventsByClaim[ev.ClaimID] = append(eventsByClaim[ev.ClaimID], ev)
	}

	groups := map[groupKey][]core.SemanticClaim{}
	for _, cl := range claims {
		key := groupKey{Class: cl.Class, Shape: NormalizeRuleShape(cl.Rule)}
		groups[key] = append(groups[key], cl)
	}

	var patterns []Pattern
	for key, members := range groups {
		sites := map[string]bool{}
		for _, m := range members {
			sites[m.Subject] = true
		}
		if len(sites) < 2 {
			continue // recurrence requires >=2 distinct sites (§3.3)
		}

		pat := Pattern{Class: key.Class, RuleShape: key.Shape}
		for _, cl := range members {
			pm := PatternMember{
				ClaimID:             cl.ClaimID,
				Subject:             cl.Subject,
				Scope:               append([]string(nil), cl.Scope...),
				Status:              memberStatusFrom(cl.Status),
				CompiledAt:          cl.CompiledAt,
				CompiledAtWatermark: cl.CompiledAtWatermark,
			}
			if kind, commit := latestSupersedeResolution(eventsByClaim[cl.ClaimID]); kind != "" {
				pm.ResolutionKind = kind
				pm.ResolutionCommit = commit
			}
			pat.Members = append(pat.Members, pm)

			if pat.FirstSeenAt.IsZero() || cl.CompiledAt.Before(pat.FirstSeenAt) {
				pat.FirstSeenAt = cl.CompiledAt
				pat.FirstSeenWatermark = cl.CompiledAtWatermark
			}
			if pat.LastSeenAt.IsZero() || cl.CompiledAt.After(pat.LastSeenAt) {
				pat.LastSeenAt = cl.CompiledAt
				pat.LastSeenWatermark = cl.CompiledAtWatermark
			}
		}
		sort.Slice(pat.Members, func(i, j int) bool { return pat.Members[i].ClaimID < pat.Members[j].ClaimID })
		patterns = append(patterns, pat)
	}

	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].Class != patterns[j].Class {
			return patterns[i].Class < patterns[j].Class
		}
		return patterns[i].RuleShape < patterns[j].RuleShape
	})
	return patterns
}

// latestSupersedeResolution returns the most recent "superseded" event's
// ResolutionKind/ResolutionCommit among evs, or ("","") when none carries
// one (no supersede event at all, or a legacy event predating resolution
// classification).
func latestSupersedeResolution(evs []core.ClaimEvent) (kind, commit string) {
	var latest time.Time
	for _, ev := range evs {
		if ev.Verdict != "superseded" || ev.ResolutionKind == "" {
			continue
		}
		if kind == "" || ev.At.After(latest) {
			kind, commit, latest = ev.ResolutionKind, ev.ResolutionCommit, ev.At
		}
	}
	return kind, commit
}

// ─── rule-shape normalization ───────────────────────────────────────────────

var (
	// archArrowReason matches the review-verb arch_conformance bridge's
	// own Rule shape ("<from> → <to>: <reason>", engine.go's runArch) —
	// <from>/<to> are per-site symbol identifiers; <reason> is the
	// underlying rule's stable text. Keeping only the reason clause lets
	// every violation of the SAME architecture rule group into one
	// pattern regardless of which symbols triggered it.
	archArrowReason = regexp.MustCompile(`^.+ → .+?: (.+)$`)
	// bracketedID matches a bracketed identifier like "[US-777]" (a spec
	// ID) — replaced with a placeholder so every spec_uncovered finding
	// (engine.go's runSpecCoverage, one per distinct spec ID) normalizes
	// to the same shape.
	bracketedID = regexp.MustCompile(`\[[A-Za-z0-9_-]+\]`)
	// quotedToken matches a quoted or backtick-quoted token — typically a
	// symbol, file, or literal name an LLM-authored finding names
	// verbatim per site.
	quotedToken = regexp.MustCompile("[\"'`][^\"'`]+[\"'`]")
	// pathToken matches a slash- or dot-separated path-shaped token (repo
	// paths, package-qualified symbols).
	pathToken = regexp.MustCompile(`\b[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+\b|\b[A-Za-z_][A-Za-z0-9_]*\.[A-Za-z_][A-Za-z0-9_.]*\b`)
	// numberToken matches a standalone number (line numbers, counts).
	numberToken    = regexp.MustCompile(`\b[0-9]+\b`)
	whitespaceRuns = regexp.MustCompile(`\s+`)
)

// NormalizeRuleShape reduces a finding's Rule text to a coarse, DETERMINISTIC
// shape for pattern grouping (§3.3): lowercased, per-site identifiers
// (quoted tokens, path-shaped tokens, bracketed spec IDs, bare numbers)
// replaced with placeholders, whitespace collapsed. Deliberately coarse —
// same philosophy as the class vocabulary (§3.2): a normalization nobody
// can reason about is worse than a blunt one. Two Rule strings that differ
// only in which symbol/file/spec-ID they name normalize identically; two
// that differ in actual failure-mode language do not.
func NormalizeRuleShape(rule string) string {
	s := rule
	if m := archArrowReason.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	s = bracketedID.ReplaceAllString(s, "<spec>")
	s = quotedToken.ReplaceAllString(s, "<id>")
	s = pathToken.ReplaceAllString(s, "<path>")
	s = numberToken.ReplaceAllString(s, "<n>")
	s = strings.ToLower(s)
	s = whitespaceRuns.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
