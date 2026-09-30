package main

import (
	"fmt"
	"strings"
	"time"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
	"github.com/geoffmilleraz/fathom/internal/readcoverage"
)

// doctor_intent.go — EP-11, two rows that never FAIL.
//
// A repository that has not adopted Project Intent is not broken, and a
// repository carrying honest debt is not broken either. Both rows therefore
// report, at worst, a warning: doctor's value is that a red line means
// something is actually wrong, and a row that is red everywhere teaches the
// reader to skim past the ones that matter (O23's lesson, and T6's).

// checkIntent reports what the corpus contains and what could not be read.
// Its clock and baseline are read from the intent ledger's rows under
// roots' registered service roots (PE-25).
func checkIntent(root string, roots ledgerroots.Roots) doctorCheck {
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
			Name: name, Status: doctorSkipped,
			Reason: "could not read the intent corpus: " + err.Error(),
			Remedy: "re-run; if the volume is external, reset its TCC verdict — tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>",
		}
	}

	// ADR-027 / never-FAIL-doctor rows: intent.Load never fails the read
	// itself — a router that EXISTS but could not be opened still resolves
	// Tier to TierNone, the same value a genuinely absent docs/intent/
	// produces (c.Coverage is the only place the two are still told
	// apart). Reporting that as "not adopted, which is a choice" is a
	// guessed PASS over an instrument failure; SKIP instead, naming
	// exactly what could not be read. Live evidence, 2026-09-14 (operator
	// repo, volume EPERM wedge): the same tree reported WARN "127 unproven
	// claim(s) [U12 111, U13 15, U3 1]" twice and PASS "no intent corpus"
	// once in between, purely because the router read raced a TCC wedge.
	if (c.Tier == intent.TierNone || c.Tier == intent.TierDecisionsOnly) && !c.Coverage.Complete() {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: "could not read the intent corpus: " + readcoverage.Note(c.Coverage),
			Remedy: "check the named path(s) are readable; on an external volume this is usually a wedged TCC verdict — `tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`",
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

	// ADR-027: every count goes through the coverage manifest, so a corpus
	// file that could not be read renders "≥ N" with the path and errno
	// rather than a confident total that is silently too low.
	cov := c.Coverage
	parts := []string{fmt.Sprintf("%s (%d annotated, %d terminal)", readcoverage.RenderCount(cov, len(c.Conditions), "condition(s)"), annotated, terminal)}
	parts = append(parts, fmt.Sprintf("%s [%s]", readcoverage.RenderCount(cov, len(c.Invariants), "invariant(s)"), renderRungs(byRung)))
	// EP-21: sensitive surfaces are only a channel when a guard entry
	// carries their provenance; counting them without saying how many are
	// WIRED is the "instrumented" illusion law 3 exists to stop.
	surfaceNote := readcoverage.RenderCount(cov, len(c.Surfaces), "sensitive surface(s)")
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
	ledger, ignored := intent.ReadRegisteredLedger(roots)
	if p := doctorIgnoredNote(ignored, intentLedgerNoun); p != "" {
		parts = append(parts, p)
	}
	if clock, ok := intent.Clock(ledger, root); ok {
		parts = append(parts, "clock started "+clock.At)
	} else {
		parts = append(parts, "clock not started")
	}
	// A clock without a baseline measures itself (law 12) — say which it is.
	if b, ok := intent.Baseline(ledger, root); ok {
		if len(b.BaselineDays) > 0 {
			parts = append(parts, fmt.Sprintf("K1 baseline: %d record(s), median %d day(s) to answer", len(b.BaselineDays), b.BaselineMedian))
		} else {
			parts = append(parts, "K1 baseline: none measurable — "+b.BaselineNote)
		}
	} else {
		parts = append(parts, "K1 baseline NOT recorded (run `fathom intent audit`)")
	}

	status, remedy := doctorPass, ""
	if !cov.Complete() {
		// A blind read is an instrument failure, not a finding about the
		// repo — WARN so it is visible, never FAIL (ADR-026's contract).
		status = doctorWarn
		remedy = "check the named path(s) are readable; on an external volume this is usually a wedged TCC verdict — `tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`"
	}
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
// seconds here and would make doctor unusable — under roots' registered
// service roots (PE-24; doctorLedgerRoots decides, and asks nothing inside
// a hook).
func checkUnprovenClaims(root, baseURL, service string, roots ledgerroots.Roots) doctorCheck {
	const name = "unproven_claims"

	if capabilityOffAt(root, "intent-corpus") {
		return doctorCheck{
			Name: name, Status: doctorPass,
			Reason: "switched off by this repository (.fathom/capabilities.cue)",
		}
	}

	c, err := intent.Load(root)
	if err != nil {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: "could not read the intent corpus: " + err.Error(),
			Remedy: "re-run; if the volume is external, reset its TCC verdict — tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>",
		}
	}
	// ADR-027: TierNone means "no router was found", true both for a
	// genuinely absent docs/intent/ and for a router that exists but could
	// not be read (checkIntent's doc comment above has the live evidence
	// and the full explanation) — c.Coverage is what tells them apart.
	// This row and checkIntent must not tell two different stories about
	// the same unreadable tree.
	if c.Tier == intent.TierNone {
		if !c.Coverage.Complete() {
			return doctorCheck{
				Name: name, Status: doctorSkipped,
				Reason: "could not read the intent corpus: " + readcoverage.Note(c.Coverage),
				Remedy: "check the named path(s) are readable; on an external volume this is usually a wedged TCC verdict — `tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`",
			}
		}
		return doctorCheck{
			Name: name, Status: doctorPass,
			Reason: "no intent corpus — the plan gate is still the proof of record here",
		}
	}

	// RC-6: this row and `fathom intent audit` must not report different
	// numbers for the same tree. They did — 12 here against the verb's 88 —
	// because this row built a partial AuditInput and then printed its
	// count as a total. U12 and U13 need no index, so they are wired in;
	// only U4's index probe is skipped, and that is NAMED below rather than
	// dropped.
	rows, note := planRowStates(root, roots)
	ledger, ledgerIgnored := intent.ReadRegisteredLedger(roots) // PE-25
	contracts, contractNote, contractCoverage := planRowContracts(root)
	guardNotes, guardScopes, guardWhy := repoGuardNotes(root)
	a := intent.RunAudit(intent.AuditInput{
		Corpus:           c,
		PlanRows:         rows,
		PlanNote:         note,
		PlanContracts:    contracts,
		PlanContractNote: contractNote,
		GuardNotes:       guardNotes,
		GuardScopes:      guardScopes,
		GuardNote:        guardWhy,
		ExtraCoverage:    []readmanifest.Manifest{contractCoverage},
		Ledger:           ledger,
		Now:              time.Now().UTC(),
		// No SymbolOracle: the doctor row must not pay an index probe. U4
		// is reported as not computed here, and `fathom intent audit`
		// (which caches per HEAD) is the place that resolves it.
	})

	// PC-5: measured-stale rows are named by count on this row too — a
	// reader must see that part of the picture is a coverage gap.
	staleNote := ""
	if n := countMeasuredStale(rows); n > 0 {
		staleNote = fmt.Sprintf("; measured-stale %d (a stale daemon's reading — coverage gap, not failure)", n)
	}
	if p := doctorIgnoredNote(ledgerIgnored, intentLedgerNoun); p != "" {
		staleNote += "; " + p
	}

	if len(a.Unproven) == 0 {
		reason := "no unproven claims from the sources that could be computed"
		if len(a.Uncomputable) > 0 {
			reason += "; NOT COMPUTED: " + strings.Join(a.Uncomputable, "; ")
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason + staleNote}
	}

	kinds := renderClaimKinds(a.ByKind)
	first := a.Unproven[0]
	reason := fmt.Sprintf("%s [%s]; first: %s — %s",
		readcoverage.RenderCount(a.Coverage, len(a.Unproven), "unproven claim(s)"), kinds, first.Ref, first.Detail)
	// DG-15: Claim.Evidence already carries the exact "what was checked,
	// and when" for this claim (internal/intent/audit.go) — it was
	// computed and populated on every claim, just never read here. A
	// reader deciding whether "first" is still current needs that
	// timestamp, not just the claim's own ref/detail.
	if first.Evidence != "" {
		reason += fmt.Sprintf(" (%s)", first.Evidence)
	}
	// The zero path always carried this list; the non-zero path dropped it,
	// which is how a partial count got printed as a total (RC-6).
	if n := len(a.Uncomputable); n > 0 {
		// A doctor row must stay scannable: name the count and the first
		// reason, truncated, and point at the verb for the rest. Dumping
		// all of them is how the row became 500 characters.
		head := a.Uncomputable[0]
		if len(head) > 110 {
			head = head[:110] + "…"
		}
		reason += fmt.Sprintf("; NOT COMPUTED (%d, first: %s)", n, head)
	}
	reason += staleNote
	return doctorCheck{
		Name:   name,
		Status: doctorWarn, // never a FAIL: honest debt is not a broken repo
		Reason: reason,
		Remedy: "fathom intent audit  # every claim with its remedy, and every source it could not compute",
	}
}

// doctorIgnoredNote is a doctor row's short name for what the
// registered-roots rule left out: the rows ignored, or that the roots were
// not read (every row then counts). A row stays scannable, so the unread
// reason itself is left to the daemon rows above.
func doctorIgnoredNote(ign ledgerroots.Ignored, noun string) string {
	switch {
	case ign.Rows > 0:
		return ign.Phrase(noun)
	case ign.RegistryNote != "":
		return "every " + noun + " read (the registered roots were not read)"
	}
	return ""
}

// countMeasuredStale is the number of plan-row states markMeasuredStale
// flagged (PC-5).
func countMeasuredStale(rows []intent.PlanRowState) int {
	n := 0
	for _, r := range rows {
		if r.MeasuredStale {
			n++
		}
	}
	return n
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
