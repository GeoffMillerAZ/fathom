package semantic

// selector.go — RS-1 (ADR-077 d2, docs/plans/rule-selection.md): the one
// deterministic step between serving's candidates and the MaxObligations
// cap. Candidates rank by SCOPE EVIDENCE, strongest first, in an order ADR-077
// fixed before any measurement read the tasks:
//
//  1. a doc_links row tying the cited doc unit to the edited path;
//  2. a contract key or log template on the edited code, named by the rule;
//  3. a path glob naming the edited path, the most specific first
//     (exact file, its package, then the nearest named ancestor);
//  4. a repo-wide glob.
//
// The evidence key sits where proximity sat in compareScored: freshness
// (BP-17) and role (BP-18) still decide first, and every tie on evidence
// keeps today's order (the re-rank, lexical relevance, glob specificity,
// rule text, claim id). Rank only: every candidate that matched scope stays
// a candidate, so a heuristic (literal) link never denies an edit (I57). The
// cap and the 320-token ceiling do not rise (I35), and the only store read
// is the edited file's own observation names, an indexed point lookup (I48).
//
// Tier 1 is an empty tier on the serving path: RS-M1 read 23 of 1731
// eligible rules anchored (CUT), so no doc_links lookup is built until a
// later reading raises that coverage; its slot stays first in the order.
// Tier 2's contract keys have no persisted reader yet (ADR-050 §6's tables
// are not built), so the tier reads the log templates and key-shaped names
// the edited file's observations carry.

import (
	"context"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/syslink"
)

// The evidence tiers of ADR-077 d2, strongest first. evidenceUnranked is a
// root-scope call's value: it has no edited path, so every candidate ties.
const (
	evidenceUnranked = iota
	evidenceDocLink
	evidenceContractKey
	evidencePathGlob
	evidenceRepoWide
)

// scopeFacts is what the index holds about the edited code that a rule can
// be tied to.
type scopeFacts struct {
	// docLinked holds the claim ids whose cited doc unit carries a doc_links
	// row to the edited path (tier 1). Nil on the serving path (RS-M1 CUT).
	docLinked map[string]bool
	// keys are the contract keys and log templates on the edited file
	// (tier 2), sorted.
	keys []string
}

// minEvidenceKeyLen keeps a short template ("ok", "%s") from tying every
// rule that happens to contain it.
const minEvidenceKeyLen = 6

// editedFileFacts reads the edited file's tier-2 facts: of the names its
// own observations declare (store.SymbolNamesForSource, the indexed
// (service_id, source_ref) read), the log templates (TL-4 stores each as
// the file's observation named by its text, so a name with whitespace) and
// the names shaped like a contract key ("env:HOME"). A read error is no
// facts, never a failed serve.
func editedFileFacts(ctx context.Context, st store.Store, serviceID, file string) scopeFacts {
	if file == "" {
		return scopeFacts{}
	}
	names, err := st.SymbolNamesForSource(ctx, serviceID, file)
	if err != nil {
		return scopeFacts{}
	}
	var keys []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if len(n) < minEvidenceKeyLen {
			continue
		}
		if strings.ContainsAny(n, " \t") {
			keys = append(keys, n)
			continue
		}
		if i := strings.IndexByte(n, ':'); i > 0 {
			if k, err := syslink.ParseContractKey(n); err == nil {
				keys = append(keys, k.String())
			}
		}
	}
	sort.Strings(keys)
	return scopeFacts{keys: keys}
}

// scopeEvidenceOf classifies one candidate. text is what the rule says
// (its heading words, rule and cited lines).
func scopeEvidenceOf(m scored, text string, outFile string, facts scopeFacts) int {
	if outFile == "" {
		return evidenceUnranked
	}
	if facts.docLinked[m.cl.ClaimID] {
		return evidenceDocLink
	}
	for _, k := range facts.keys {
		if strings.Contains(text, k) {
			return evidenceContractKey
		}
	}
	if m.governing.tier >= 0 && m.governing.tier < 3 {
		return evidencePathGlob
	}
	return evidenceRepoWide
}

// rankByScopeEvidence assigns every candidate its scope evidence and orders
// matched by serving's comparator, whose scope key is that evidence (see
// compareScored). text returns what candidate m's rule says. The result is
// the same candidates, reordered: nothing is added or dropped.
func rankByScopeEvidence(matched []scored, outFile string, facts scopeFacts, text func(scored) string, less func(a, b scored) bool) []scored {
	for i := range matched {
		matched[i].evidence = scopeEvidenceOf(matched[i], text(matched[i]), outFile, facts)
	}
	sort.SliceStable(matched, func(i, j int) bool { return less(matched[i], matched[j]) })
	return matched
}

// compareEvidence is the scope key of compareScored: a stronger evidence
// tier wins outright; inside tier 3 the most specific glob wins (exact
// file, package, the nearer ancestor). ok is false on a tie.
func compareEvidence(a, b scored) (less, ok bool) {
	if a.evidence != b.evidence {
		return a.evidence < b.evidence, true
	}
	if a.evidence == evidencePathGlob && a.governing != b.governing {
		return a.governing.closerThan(b.governing), true
	}
	return false, false
}

// nearestTierReserveSlots (O57 item 4) is how many of a capped payload's
// slots go to the nearest-scope tier unconditionally, before role or
// freshness competes for the rest. See selectWithinCap and
// reserveNearestTier.
const nearestTierReserveSlots = 2

// reserveNearestTier returns up to n items from matched belonging to the
// single closest governingScope TIER actually present among them — tier
// 0 (exact_file) if any matched claim reaches it, else tier 1 (package),
// else tier 2 (parent_dir), else tier 3 (root) — in matched's existing
// order (already sorted by less() before the caller reaches here), so
// which of that tier's own rules fill the reserved slots still respects
// freshness, role and specificity; only the cross-tier competition
// against FARTHER rules is bypassed for these n slots. Empty when
// outFile is "" (a root-scope call has no path to be proximate to, so
// every candidate carries noGoverningScope and reservation is a no-op)
// or when nothing in matched carries a real governing scope.
func reserveNearestTier(matched []scored, n int, outFile string) []scored {
	if outFile == "" || n <= 0 {
		return nil
	}
	nearestTier := -1
	for _, m := range matched {
		if m.governing.tier < 0 {
			continue
		}
		if nearestTier < 0 || m.governing.tier < nearestTier {
			nearestTier = m.governing.tier
		}
	}
	if nearestTier < 0 {
		return nil
	}
	var reserved []scored
	for _, m := range matched {
		if len(reserved) >= n {
			break
		}
		if m.governing.tier == nearestTier {
			reserved = append(reserved, m)
		}
	}
	return reserved
}

// selectWithinCap cuts matched (in ranking order) to max, recording the
// cut on out (Displaced, DisplacedByTier) and on trace. A no-op when
// matched fits.
func selectWithinCap(matched []scored, max int, outFile string, less func(a, b scored) bool, out *Obligations, trace *explainTrace) []scored {
	if len(matched) <= max {
		return matched
	}
	// T5-2/O14: record the cap's damage BEFORE slicing it away — this is
	// the only point that knows how many served-eligible claims existed
	// versus how many fit the budget. BP-18: the cut is role-partitioned,
	// so a claims rule can never evict a governs rule, but Displaced
	// still counts BOTH — the eviction stays one honest integer.
	out.Displaced = len(matched) - max
	// O57 item 4: the nearest-scope tier gets first claim on the first
	// nearestTierReserveSlots slots, unconditionally — before role or
	// freshness gets a say. What the ordering alone does NOT protect is a
	// nearest rule that is stale, or claims-role — six fresh root rules can
	// fill every slot before either is even considered, which is exactly
	// what guards-pilot-5's second reading caught (a served payload naming
	// no rule closer than "root" while a package guide governed the edited
	// file). Reservation only decides MEMBERSHIP; which of the nearest
	// tier's own rules fill the reserved slots still goes through matched's
	// existing order, and the final presentation order below is unaffected
	// by how a rule reached kept.
	reserveN := nearestTierReserveSlots
	if reserveN > max {
		reserveN = max
	}
	reserved := reserveNearestTier(matched, reserveN, outFile)
	reservedIDs := make(map[string]bool, len(reserved))
	for _, m := range reserved {
		reservedIDs[m.cl.ClaimID] = true
	}
	if trace != nil && outFile != "" {
		// A root-scope call has no path to be near: nothing is reserved.
		trace.reserved = reservedIDs
		trace.reserveN = reserveN
	}
	kept := make([]scored, 0, max)
	kept = append(kept, reserved...)
	for _, m := range matched {
		if len(kept) >= max {
			break
		}
		if !m.claims && !reservedIDs[m.cl.ClaimID] {
			kept = append(kept, m)
		}
	}
	if len(kept) > max {
		kept = kept[:max]
	}
	for _, m := range matched {
		if len(kept) >= max {
			break
		}
		if m.claims && !reservedIDs[m.cl.ClaimID] {
			kept = append(kept, m)
		}
	}
	// VB-10 (ADR-035 decision 2): before the survivors are re-sorted,
	// tally the DROPPED claims by governing-scope tier — a caller can
	// then see WHAT KIND of rule the cap displaced, not just how many.
	// File-scoped only: a root-scope call carries no computable tier.
	if outFile != "" {
		keptIDs := make(map[string]bool, len(kept))
		for _, k := range kept {
			keptIDs[k.cl.ClaimID] = true
		}
		byTier := map[string]int{}
		for _, m := range matched {
			if !keptIDs[m.cl.ClaimID] {
				byTier[m.governing.name()]++
			}
		}
		if len(byTier) > 0 {
			out.DisplacedByTier = byTier
		}
	}
	// Re-order the survivors: the role partition above is a BUDGET
	// decision, not a presentation one, and it can leave a stale governs
	// rule ahead of a fresh claims rule. The served payload is ordered
	// by less() — fresh first (BP-17) — whatever the budget did.
	sort.Slice(kept, func(i, j int) bool { return less(kept[i], kept[j]) })
	return kept
}
