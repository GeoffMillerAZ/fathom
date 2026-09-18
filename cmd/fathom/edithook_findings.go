package main

// edithook_findings.go — FL-20 (docs/plans/findings-loop.md §D, docs/
// vision/30-findings-loop.md §3.4, ADR-040 decision 5): the "open findings
// on this file" advisory block. Findings ride the SAME obligations call
// governing rules already make (layer=all — see fetchObligations in
// edithook.go); this file only renders the finding-kind (Kind=="finding")
// rows that call returns, never fetches anything of its own.
//
// Findings are advisory, never enforcement: a served finding never blocks
// a tool call (that stays the enforce:true guard/layer-rule path,
// evaluated and short-circuited BEFORE renderPreEdit is ever reached — see
// cmdEditHookPreFile). This file only renders text.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// findingsBlockMaxCount / findingsBlockMaxBytes are §3.4's own caps for the
// "open findings on this file" block: at most 3 findings, ≤1,200 bytes
// total (header + bullets + the trailing omitted line) — this budget is
// separate from, and does not compete with, the governing-rules payload
// that always precedes it (§3.4: "the block never displaces rules").
const (
	findingsBlockMaxCount = 3
	findingsBlockMaxBytes = 1200
)

// lineRange is an inclusive, 1-based line range — either the region a
// PostToolUse edit touched (editedRangeFromFile) or a finding's own cited
// range (parsed from its Src "path#Lstart-Lend" via core.ParseCiteRef).
type lineRange struct {
	Start, End int
}

// overlaps reports whether r and o share at least one line.
func (r lineRange) overlaps(o lineRange) bool {
	return r.Start <= o.End && o.Start <= r.End
}

// findingsMeta is what renderFindingsBlock learned while rendering — the
// counts the caller's own beat/answered-meta needs.
type findingsMeta struct {
	Total   int    // live finding-kind rows matching this file/phase, before the count/byte cap
	Shown   int    // rows actually rendered
	Omitted int    // Total - Shown
	Reason  string // set only when the post-edit path could not determine an edited range at all
}

// splitFindingRows partitions rules (a raw obligations "all"-layer answer)
// into governs-role rows (Kind != "finding" — obligations, adr-rules, …,
// exactly what every pre-existing renderer already expects) and
// finding-kind rows (Kind == "finding"). Order is preserved within each
// slice. Called BEFORE anything else touches the raw rules, so every
// existing governing-rules code path (guard-stamp dedup, writeRuleBullet,
// reportModuleBeats) sees byte-identical input to what it saw before this
// wave, whether or not any findings exist.
func splitFindingRows(rules []semantic.Obligation) (govRules, findingRows []semantic.Obligation) {
	for _, r := range rules {
		if r.Kind == "finding" {
			findingRows = append(findingRows, r)
		} else {
			govRules = append(govRules, r)
		}
	}
	return govRules, findingRows
}

// renderFindingsBlock renders the §3.4 "open findings on this file" block
// from rules — expected already filtered to Kind=="finding" (splitFindingRows),
// though this function re-checks defensively. phase is "pre" or "post":
//
//   - "pre": every live finding renders unconditionally (there is no
//     proposed edit yet to overlap against) — editedRange is ignored.
//   - "post": renders ONLY rows whose cited Src range overlaps
//     editedRange. editedRange == nil (the caller could not determine what
//     changed — e.g. no NewContent for the tool call) means "skip",
//     surfaced as meta.Reason so the caller can tell "checked, no overlap"
//     from "could not check" without a rendered block either way.
func renderFindingsBlock(rules []semantic.Obligation, editedRange *lineRange, phase string) (string, findingsMeta) {
	if phase == "post" && editedRange == nil {
		return "", findingsMeta{Reason: "no edited range"}
	}

	live := liveFindingRows(rules, phase, editedRange)
	if len(live) == 0 {
		return "", findingsMeta{}
	}
	sortFindingRows(live)
	total := len(live)

	candidates := live
	if len(candidates) > findingsBlockMaxCount {
		candidates = candidates[:findingsBlockMaxCount]
	}
	bullets := make([]string, len(candidates))
	for i, r := range candidates {
		bullets[i] = findingBullet(r)
	}
	// Byte cap: drop from the tail (the lowest-ranked of the already-capped
	// candidates) until the WHOLE block — header, every remaining bullet,
	// and the omitted line the shrink itself grows — fits. omitted is
	// always rendered whenever total > shown (never silent), so the loop
	// re-measures the full text on every iteration rather than the bullets
	// alone.
	for len(bullets) > 0 && len(renderFindingsText(total, bullets)) > findingsBlockMaxBytes {
		bullets = bullets[:len(bullets)-1]
	}
	text := renderFindingsText(total, bullets)
	shown := len(bullets)
	return text, findingsMeta{Total: total, Shown: shown, Omitted: total - shown}
}

// liveFindingRows filters rules to the finding-kind rows this render
// actually admits: Kind=="finding", never a status Served() already
// excludes (superseded/refuted — see semantic.Obligation.Status's own doc
// comment: a claim never reaches ANY obligations payload with one of those
// two statuses at all, so this is a defensive assertion, not a correction
// callers should ever observe firing), and — phase=="post" only — whose
// parsed Src cites a line range overlapping editedRange.
func liveFindingRows(rules []semantic.Obligation, phase string, editedRange *lineRange) []semantic.Obligation {
	var out []semantic.Obligation
	for _, r := range rules {
		if r.Kind != "finding" {
			continue
		}
		if r.Status == "superseded" || r.Status == "refuted" {
			continue // defensive: Served() already excludes these upstream
		}
		if phase == "post" {
			path, start, end, ok := core.ParseCiteRef(r.Src)
			if !ok || path == "" {
				continue // unanchored, or an unparsable citation — never overlaps a concrete edit
			}
			if !editedRange.overlaps(lineRange{Start: start, End: end}) {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// sortFindingRows sorts in place per §3.4's own stated order: anchored
// before unanchored, then a current anchor before a drifted one (nil —
// "no drift check recorded" — sorts as current, since AnchorCurrent is
// only ever set false by an explicit drift detection), then ObservedAt
// descending (most recent first), then RuleID ascending for a
// deterministic tie-break.
func sortFindingRows(rows []semantic.Obligation) {
	current := func(r semantic.Obligation) bool {
		return r.AnchorCurrent == nil || *r.AnchorCurrent
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Unanchored != b.Unanchored {
			return !a.Unanchored // anchored first
		}
		if ca, cb := current(a), current(b); ca != cb {
			return ca // current anchor first
		}
		if a.ObservedAt != b.ObservedAt {
			return a.ObservedAt > b.ObservedAt // desc
		}
		return a.RuleID < b.RuleID
	})
}

// findingBullet renders one finding's bullet line, per §3.4's template:
//
//   - [finding · <class> · <producer> · <rule_id>] <rule> — remedy: <remedy> — <src> — observed <observed_at> at <source_revision>
//
// with " (code moved since observed)" appended when AnchorCurrent is
// explicitly false, and " (unanchored process finding)" appended when
// Unanchored — never mutually exclusive of each other, both apply when
// both are true. An empty optional field renders as an honest placeholder
// ("unclassified" / "unknown-producer" / …) rather than an empty token —
// never fabricated data, just a labeled absence.
func findingBullet(r semantic.Obligation) string {
	class := r.Class
	if class == "" {
		class = "unclassified"
	}
	producer := r.Producer
	if producer == "" {
		producer = "unknown-producer"
	}
	ruleID := r.RuleID
	if ruleID == "" {
		ruleID = "unknown-rule"
	}
	remedy := r.Remedy
	if remedy == "" {
		remedy = "(none recorded)"
	}
	observed := r.ObservedAt
	if observed == "" {
		observed = "unknown"
	}
	rev := r.SourceRevision
	if rev == "" {
		rev = "unknown"
	}
	var suffix strings.Builder
	if r.AnchorCurrent != nil && !*r.AnchorCurrent {
		suffix.WriteString(" (code moved since observed)")
	}
	if r.Unanchored {
		suffix.WriteString(" (unanchored process finding)")
	}
	// FT-6 (docs/vision/03-findings-and-experts.md §2): CodeDrifted is a
	// DIFFERENT drift signal from AnchorCurrent above — the finding's own
	// citation still resolves, but the SCOPED CODE it is about changed
	// since compile (a serving annotation, never a status flip: this can
	// mean the finding was already fixed). This block is the only place a
	// finding-kind row renders at all (FL-20 supersedes the old inline
	// obligations-bullet labeling of Kind=="finding"/CodeDrifted), so the
	// signal must not be dropped here.
	if r.CodeDrifted {
		suffix.WriteString(" (scoped code changed since compile — may already be fixed)")
	}
	return fmt.Sprintf("\n- [finding · %s · %s · %s] %s — remedy: %s — %s — observed %s at %s%s",
		class, producer, ruleID, r.Rule, remedy, r.Src, observed, rev, suffix.String())
}

// renderFindingsText builds the full block text for total live findings
// with the given (already count/byte-capped) bullets: the header always
// names both the total and the shown count, and the omitted line always
// renders whenever total > len(bullets) — never silent.
func renderFindingsText(total int, bullets []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "📋 fathom findings — open findings on this file (%d, showing %d):", total, len(bullets))
	for _, b := range bullets {
		sb.WriteString(b)
	}
	if omitted := total - len(bullets); omitted > 0 {
		fmt.Fprintf(&sb, "\n⚠️ omitted: %d", omitted)
	}
	return sb.String()
}

// editedRangeFromFile best-effort locates the 1-based line range a
// PostToolUse edit touched: it searches fileText (the file as it now
// stands — PostToolUse fires after the write) for newContent (the tool
// call's own proposed new_content — HookInput.NewContent) and reports the
// range it occupies. Returns nil — "no edited range" — when newContent is
// empty (a pure deletion, or a tool class the codec could not read
// proposed content for) or when it cannot be found verbatim in the current
// file (should not happen for a just-applied edit; defensive).
func editedRangeFromFile(fileText, newContent string) *lineRange {
	newContent = strings.TrimRight(newContent, "\n")
	if newContent == "" {
		return nil
	}
	idx := strings.Index(fileText, newContent)
	if idx < 0 {
		return nil
	}
	start := strings.Count(fileText[:idx], "\n") + 1
	end := start + strings.Count(newContent, "\n")
	return &lineRange{Start: start, End: end}
}
