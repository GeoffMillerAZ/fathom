package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// doctor_intent.go — EP-11, two rows that never FAIL.
//
// A repository that has not adopted Project Intent is not broken, and a
// repository carrying honest debt is not broken either. Both rows therefore
// report, at worst, a warning: doctor's value is that a red line means
// something is actually wrong, and a row that is red everywhere teaches the
// reader to skim past the ones that matter (O23's lesson, and T6's).

// checkIntent reports what the corpus contains and what could not be read.
func checkIntent(root string) doctorCheck {
	const name = "intent"

	// The capability is a real switch: off silences the reporting channels
	// (this row and unproven_claims), never the verb — `fathom intent
	// audit` still answers on demand. A switch that switches nothing is the
	// defect capabilityenforce_test.go exists to prevent.
	if capabilityOffAt(root, "intent-corpus") {
		return doctorCheck{
			Name: name, Status: doctorPass,
			Reason: "switched off by this repository (.fathom/capabilities.cue) — `fathom intent audit` still answers on demand",
			Remedy: "",
		}
	}

	c, err := intent.Load(root)
	if err != nil {
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: "could not read the intent corpus: " + err.Error(),
			Remedy: "check docs/intent/ is readable",
		}
	}

	switch c.Tier {
	case intent.TierNone:
		return doctorCheck{
			Name: name, Status: doctorPass,
			Reason: "no docs/intent/ router and no decision records — Project Intent is not adopted here, which is a choice, not a defect",
			Remedy: "",
		}
	case intent.TierDecisionsOnly:
		return doctorCheck{
			Name: name, Status: doctorPass,
			Reason: fmt.Sprintf("decisions-only tier: %s, %d condition(s) from Revisit-when trailers, no router",
				c.DecisionsHome, len(c.Conditions)),
			Remedy: "agentic-kit's intent-init adds the router when you want the rest",
		}
	}

	byRung := map[intent.Rung]int{}
	for _, inv := range c.Invariants {
		byRung[inv.Rung]++
	}
	annotated, terminal := 0, 0
	for _, cond := range c.Conditions {
		if cond.Annotated() {
			annotated++
		}
		if cond.Terminal {
			terminal++
		}
	}

	parts := []string{fmt.Sprintf("%d condition(s) (%d annotated, %d terminal)", len(c.Conditions), annotated, terminal)}
	parts = append(parts, fmt.Sprintf("%d invariant(s) [%s]", len(c.Invariants), renderRungs(byRung)))
	// EP-21: sensitive surfaces are only a channel when a guard entry
	// carries their provenance; counting them without saying how many are
	// WIRED is the "instrumented" illusion law 3 exists to stop.
	surfaceNote := fmt.Sprintf("%d sensitive surface(s)", len(c.Surfaces))
	if notes, scopesOf, why := repoGuardNotes(root); scopesOf == nil {
		surfaceNote += ", guard coverage unknown (" + why + ")"
	} else {
		uncovered, stale := 0, 0
		for _, cl := range intent.GuardEntryStale(c, notes, scopesOf) {
			if strings.HasPrefix(cl.Ref, ".fathom/guard.cue") {
				stale++
				continue
			}
			uncovered++
		}
		surfaceNote += fmt.Sprintf(", %d covered by guard.cue", len(c.Surfaces)-uncovered)
		if stale > 0 {
			surfaceNote += fmt.Sprintf(", %d guard entr(ies) STALE", stale)
		}
	}
	parts = append(parts, surfaceNote)
	if c.DecisionsHome != "" {
		parts = append(parts, "decisions: "+c.DecisionsHome)
	}
	if clock, ok := intent.Clock(intent.ReadLedger(), root); ok {
		parts = append(parts, "clock started "+clock.At)
	} else {
		parts = append(parts, "clock not started")
	}

	status, remedy := doctorPass, ""
	if len(c.Unparsed) > 0 {
		status = doctorWarn
		parts = append(parts, fmt.Sprintf("%d UNPARSED (%s:%d %s)",
			len(c.Unparsed), c.Unparsed[0].File, c.Unparsed[0].Line, c.Unparsed[0].Reason))
		remedy = "fathom intent audit  # names every unparsed line"
	}
	if len(c.NestedRouters) > 0 {
		parts = append(parts, fmt.Sprintf("%d nested router(s) found and NOT audited (v1 audits the repo root only)", len(c.NestedRouters)))
	}
	if c.DecisionsHome == "" {
		status = doctorWarn
		parts = append(parts, "NO decisions home resolved")
		remedy = "name the decisions home in the router's docs map"
	}

	return doctorCheck{Name: name, Status: status, Reason: strings.Join(parts, "; "), Remedy: remedy}
}

// checkUnprovenClaims reports the honest ledger. It reads the newest
// verify-ledger row per plan — never a fresh verify, which costs ~70
// seconds here and would make doctor unusable.
func checkUnprovenClaims(root, baseURL, service string) doctorCheck {
	const name = "unproven_claims"

	if capabilityOffAt(root, "intent-corpus") {
		return doctorCheck{
			Name: name, Status: doctorPass,
			Reason: "switched off by this repository (.fathom/capabilities.cue)",
		}
	}

	c, err := intent.Load(root)
	if err != nil || c.Tier == intent.TierNone {
		return doctorCheck{
			Name: name, Status: doctorPass,
			Reason: "no intent corpus — the plan gate is still the proof of record here",
		}
	}

	rows, note := planRowStates(root)
	a := intent.RunAudit(intent.AuditInput{
		Corpus:   c,
		PlanRows: rows,
		PlanNote: note,
		Ledger:   intent.ReadLedger(),
		Now:      time.Now().UTC(),
		// No SymbolOracle: the doctor row must not pay an index probe. U4
		// is reported as not computed here, and `fathom intent audit`
		// (which caches per HEAD) is the place that resolves it.
	})

	if len(a.Unproven) == 0 {
		reason := "no unproven claims from the sources that could be computed"
		if len(a.Uncomputable) > 0 {
			reason += "; NOT COMPUTED: " + strings.Join(a.Uncomputable, "; ")
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}

	kinds := renderClaimKinds(a.ByKind)
	first := a.Unproven[0]
	return doctorCheck{
		Name:   name,
		Status: doctorWarn, // never a FAIL: honest debt is not a broken repo
		Reason: fmt.Sprintf("%d unproven claim(s) [%s]; first: %s — %s",
			len(a.Unproven), kinds, first.Ref, first.Detail),
		Remedy: "fathom intent audit  # every claim with its remedy",
	}
}

func renderClaimKinds(by map[string]int) string {
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sortStringsLocal(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, by[k]))
	}
	return strings.Join(parts, ", ")
}
