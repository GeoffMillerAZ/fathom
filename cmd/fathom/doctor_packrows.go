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
//	stanza_present  ← pack claude-code: claude_code_stanza_present,
//	                  pack codex: codex_stanza_present
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
	remedy := "add a `fathom verify` (and optionally `fathom review`) step to lefthook.yml's pre-push — docs/adoption/claude-code.md §7.4"

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
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("no lefthook.yml/lefthook.yaml at %s", root), Remedy: remedy}
	case verify.Status != custodian.StatusHolds:
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("%s has no \"fathom verify\" step in its pre-push commands", filepath.Join(root, verify.Files[0])), Remedy: remedy}
	case review.Status == custodian.StatusHolds:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s has both a fathom verify and a fathom review step", filepath.Join(root, verify.Files[0]))}
	default:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s has a fathom verify step (no fathom review step present — optional, C3-1)", filepath.Join(root, verify.Files[0]))}
	}
}

// checkStanzaPresent is harness-independent: `fathom init` always writes
// the identical stanza to both files, so both are always expected. The
// engine's stanza_present details are file-prefixed sentences; the row is
// their join, exactly as the bespoke check joined its own.
func checkStanzaPresent(root string) doctorCheck {
	const name = "stanza_present"
	remedy := "fathom init --path " + root

	fs, err := doctorPackFindings(root, []packRef{
		{"claude-code", "claude_code_stanza_present"},
		{"codex", "codex_stanza_present"},
	})
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("a shipped stanza pack entry did not load (%v) — a fathom defect, so this row cannot be evaluated", err)}
	}
	var problems []string
	for _, f := range fs {
		if f.Status != custodian.StatusHolds {
			problems = append(problems, f.Detail)
		}
	}
	if len(problems) > 0 {
		return doctorCheck{Name: name, Status: doctorFail, Reason: strings.Join(problems, "; "), Remedy: remedy}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: "fathom stanza well-formed in both CLAUDE.md and AGENTS.md"}
}
