package main

// doctor_packrows.go — CC-13 (docs/plans/custodian.md; O27's structural
// resolution, vision 22 §7): the doctor rows a shipped pack fully expresses
// render what the concern audit knows instead of re-deriving it — law 10,
// one evaluation engine, many surfaces. Two rows qualify:
//
//	lefthook_gates  ← pack lefthook: lefthook_pre_push_verify, plus the
//	                  optional review step as a doctor-side informational
//	                  concern (C3-1: optional, so no pack ships it as a
//	                  want — a pack entry most repos violate would teach
//	                  people to ignore the custodian)
//	stanza_present  ← pack codex: codex_stanza_present (FAIL),
//	                  pack claude-code: claude_code_stanza_present (WARN),
//	                  plus claudeLoadPath (doctor_claudeload.go, ADR-059)
//
// The entries are evaluated as SHIPPED: a repo's same-id override governs
// its own `fathom config audit`, not these wiring rows, which stay the same
// diagnosis on every repo whether or not it enabled a pack. Output is
// unchanged for users — TestDoctorConvergence_RowsUnchanged runs the
// retired bespoke checks beside these over the same fixtures and requires
// identical rows. Two deliberate differences: I38's — a lefthook.yml that
// exists and cannot be read is named as unreadable, where the bespoke
// check fell through to "no lefthook.yml/lefthook.yaml at <root>" — and
// I17's: the verify entry reads only the pre-push hook's non-comment lines,
// so a commented-out verify or a pre-commit-only one fails the row whose
// reason has always said "in its pre-push commands" (the retired raw-line
// check passed both).
//
// Not migrated, because no pack expresses them fully: binary_on_path (a
// PATH fact, not a config file), hooks_installed / skills_installed (their
// evaluators are the harness registry and the embedded skills), and every
// row that asks the daemon.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/custodian"
)

// packRef names one shipped pack entry.
type packRef struct{ pack, id string }

// doctorPackFindings evaluates shipped pack entries — then any doctor-side
// informational concerns — through the custodian engine, one finding per
// concern in order. An entry that does not load is a fathom defect,
// returned as an error for the row to name.
func doctorPackFindings(root string, refs []packRef, extra ...custodian.Concern) ([]custodian.Finding, error) {
	concerns := make([]custodian.Concern, 0, len(refs)+len(extra))
	for _, r := range refs {
		c, err := custodian.PackConcern(r.pack, r.id)
		if err != nil {
			return nil, err
		}
		concerns = append(concerns, c)
	}
	concerns = append(concerns, extra...)
	return custodian.Audit(root, concerns), nil
}

// lefthookReviewConcern is the one doctor-side concern: the optional
// `fathom review` step (C3-1), which only chooses between two passing
// reasons.
func lefthookReviewConcern() custodian.Concern {
	return custodian.Concern{
		ID:   "doctor_lefthook_review",
		Want: "the lefthook config runs `fathom review` (optional, C3-1)",
		Check: custodian.Check{
			Kind:    custodian.KindRequiredPattern,
			Pattern: lefthookInvokesPattern("review"),
			Where:   custodian.CorpusLefthook,
		},
	}
}

func checkLefthookGates(root string) doctorCheck {
	const name = "lefthook_gates"
	if row, ok := checkComposedPushGate(root); ok {
		return row
	}
	remedy := "add a `fathom verify` (and optionally `fathom review`) step to lefthook.yml's pre-push (see docs/adoption/claude-code.md in the fathom repo)"

	fs, err := doctorPackFindings(root, []packRef{{"lefthook", "lefthook_pre_push_verify"}}, lefthookReviewConcern())
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("the shipped lefthook pack did not load (%v) — a fathom defect, so this row cannot be evaluated", err)}
	}
	verify, review := fs[0], fs[1]
	switch {
	case verify.Status == custodian.StatusUnable:
		// I38: a config that exists and cannot be read is never "absent".
		return doctorCheck{Name: name, Status: doctorFail,
			Reason: verify.Detail + " — whether it runs fathom verify cannot be read",
			Remedy: "make the lefthook config readable (an EPERM on macOS is usually a TCC verdict), then re-run doctor"}
	case len(verify.Files) == 0:
		// A repo that never adopted lefthook has no gate to break: SKIP with
		// the reason; a wired repo whose gate lacks the verify step still FAILs.
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("no lefthook.yml/lefthook.yaml at %s — this repo has no lefthook config, so there is no push gate to check", root), Remedy: "optional: add a `fathom verify` step to lefthook.yml's pre-push (see docs/adoption/claude-code.md in the fathom repo)"}
	case verify.Status != custodian.StatusHolds:
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("%s has no \"fathom verify\" step in its pre-push commands", filepath.Join(root, verify.Files[0])), Remedy: remedy}
	case review.Status == custodian.StatusHolds:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s has both a fathom verify and a fathom review step", filepath.Join(root, verify.Files[0]))}
	default:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s has a fathom verify step (no fathom review step present — optional)", filepath.Join(root, verify.Files[0]))}
	}
}

// checkStanzaPresent is harness-independent (ADR-059, AG-11): the stanza
// lives in AGENTS.md for every harness, so the codex pack's
// codex_stanza_present entry decides FAIL (absent file, no region, a
// corrupted marker pair). Whether Claude Code will load AGENTS.md is a WARN,
// never a FAIL, with the remedy: the claude-code pack's
// claude_code_stanza_present (agents_md_reaches_claude — a root CLAUDE file
// without the import), a CLAUDE file still carrying the old stanza, and
// claudeLoadPath's machine half (an ancestor CLAUDE file, the agents-md
// setting, a Claude Code release before 2.1.277). PASS names the path
// AGENTS.md takes into a session.
func checkStanzaPresent(root string) doctorCheck { return checkStanzaPresentProbe(root, true) }

// checkStanzaPresentRepoOnly is the row `doctor --repo-only` renders: the
// same verdict without asking the claude binary for its version.
func checkStanzaPresentRepoOnly(root string) doctorCheck { return checkStanzaPresentProbe(root, false) }

// checkStanzaPresentProbe is checkStanzaPresent with the version probe
// switchable (claudeLoadPath).
func checkStanzaPresentProbe(root string, probeVersion bool) doctorCheck {
	const name = "stanza_present"
	remedy := "fathom init --path " + root

	fs, err := doctorPackFindings(root, []packRef{
		{"codex", "codex_stanza_present"},
		{"claude-code", "claude_code_stanza_present"},
	})
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("a shipped stanza pack entry did not load (%v) — a fathom defect, so this row cannot be evaluated", err)}
	}
	agents, reach := fs[0], fs[1]
	if agents.Status != custodian.StatusHolds {
		return doctorCheck{Name: name, Status: doctorFail, Reason: agents.Detail, Remedy: remedy}
	}
	// A well-formed pair is not a stanza: a region whose body is the
	// one-line import (a CLAUDE-file region written into AGENTS.md through
	// a link) tells a session nothing about the index.
	if data, err := os.ReadFile(filepath.Join(root, agentsMDFile)); err == nil && isImportBody(ownedBodyIn(string(data))) {
		return doctorCheck{Name: name, Status: doctorFail, Reason: "AGENTS.md: fathom's region is well-formed but holds no stanza, only " + strings.TrimSpace(ownedBodyIn(string(data))), Remedy: remedy + " (rewrites the region to the stanza)"}
	}

	load := claudeLoadPath(root, probeVersion)
	var warns, remedies []string
	if len(load.Corrupted) > 0 {
		warns = append(warns, fmt.Sprintf("%s: corrupted fathom marker pair — fleet reconcile will not rewrite it", strings.Join(load.Corrupted, ", ")))
		remedies = append(remedies, "repair or remove the stray fathom marker in "+strings.Join(load.Corrupted, ", ")+" by hand, then "+remedy)
	}
	if len(load.LegacyStanza) > 0 {
		warns = append(warns, fmt.Sprintf("%s still carries the fathom stanza (the retired two-copy layout)", strings.Join(load.LegacyStanza, ", ")))
		remedies = append(remedies, remedy+" (rewrites that region to an @AGENTS.md import)")
	}
	if reach.Status != custodian.StatusHolds && load.Mode != claudeModeBoth {
		warns = append(warns, reach.Detail)
		remedies = append(remedies, remedy+" (adds a marked @AGENTS.md import region)")
	}
	machine, machineRemedies := load.warnings()
	warns = append(warns, machine...)
	remedies = append(remedies, machineRemedies...)
	if len(warns) > 0 {
		return doctorCheck{Name: name, Status: doctorWarn,
			Reason: "AGENTS.md: fathom stanza well-formed, but Claude Code may not load it: " + strings.Join(warns, "; "),
			Remedy: strings.Join(dedupeStrings(remedies), "; ")}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: "AGENTS.md: fathom stanza well-formed; Claude Code loads it: " + load.passReason()}
}

// dedupeStrings keeps the first of each string, in order.
func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
