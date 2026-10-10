package main

// configpropose.go — CC-9 (docs/plans/custodian.md; vision 22 §2, §3b):
// `fathom config propose`, the diff-offering verb. It runs the same audit
// as `fathom config audit` and, for each violated concern, offers a unified
// diff where the mend is mechanical (a dead pointer or script path whose
// file moved, unambiguously) and names the remedy where it is not. It
// WRITES NOTHING — no file in the repo, no beat, no receipt: the session,
// with a human present, applies the diff or does not (fathom is the
// instrument, never the hand). The audit stays the gate; this verb exits 0
// whenever it ran, and 1 only when a concerns file would not load.

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/custodian"
)

// configProposeModel is --json's envelope (law 10: one shape, many
// surfaces).
type configProposeModel struct {
	RepoRoot  string               `json:"repo_root"`
	Violated  int                  `json:"violated"`
	Proposals []custodian.Proposal `json:"proposals"`
	Patch     string               `json:"patch"`
}

func cmdConfigPropose(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("config propose", stderr)
	repo := fs.String("repo", ".", "repository root")
	asJSON := fs.Bool("json", false, "emit the proposals and the combined patch as JSON")
	patchOnly := fs.Bool("patch", false, "print only the combined unified diff (empty when nothing is mechanically mendable) — for 'git apply' from the repo root")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *asJSON && *patchOnly {
		fmt.Fprintln(stderr, "error: --json and --patch are exclusive")
		return 1
	}
	root, err := filepath.Abs(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	concerns, loadErrs := loadConcernsBoth(root)
	for _, e := range loadErrs {
		fmt.Fprintf(stderr, "config: %v\n", e)
	}
	exit := 0
	if len(loadErrs) > 0 {
		exit = 1 // what did load is still proposed over; the run is not a clean bill
	}
	if len(concerns) == 0 {
		if !*patchOnly && !*asJSON {
			fmt.Fprintln(stdout, "no concerns registered — there is nothing to propose against.")
			fmt.Fprintf(stdout, "Write ~/.fathom/concerns.cue (user default) or %s (this repo governs), or enable a pack there: packs: [\"claude-code\", \"lefthook\"].\n", filepath.Join(".fathom", "concerns.cue"))
		}
		if *asJSON {
			_ = writeProposeJSON(stdout, configProposeModel{RepoRoot: root, Proposals: []custodian.Proposal{}})
		}
		return exit
	}

	findings := custodian.Audit(root, concerns, custodian.WithWiringDrift(wiringCurrentEvaluator))
	proposals, err := custodian.Propose(root, findings)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	var edits []custodian.Edit
	for _, p := range proposals {
		edits = append(edits, p.Edits...)
	}
	patch, err := custodian.Patch(root, edits)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	switch {
	case *patchOnly:
		_, _ = io.WriteString(stdout, patch)
	case *asJSON:
		if proposals == nil {
			proposals = []custodian.Proposal{}
		}
		if err := writeProposeJSON(stdout, configProposeModel{RepoRoot: root, Violated: len(proposals), Proposals: proposals, Patch: patch}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	default:
		renderConfigPropose(stdout, root, proposals, patch)
	}
	return exit
}

func writeProposeJSON(w io.Writer, m configProposeModel) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func renderConfigPropose(w io.Writer, root string, proposals []custodian.Proposal, patch string) {
	if len(proposals) == 0 {
		fmt.Fprintf(w, "config propose for %s: no concern is violated — nothing to propose.\n", root)
		return
	}
	withDiff := 0
	for _, p := range proposals {
		if len(p.Edits) > 0 {
			withDiff++
		}
	}
	fmt.Fprintf(w, "config propose for %s (%d violated concern(s): %d with a proposed diff, %d needing a hand)\n",
		root, len(proposals), withDiff, len(proposals)-withDiff)
	for _, p := range proposals {
		fmt.Fprintf(w, "\n  ✗ %s — %s  [%s]\n", p.Concern.ID, p.Detail, p.Concern.Provenance())
		for _, e := range p.Edits {
			fmt.Fprintf(w, "      proposed  %s:%d  %s\n", e.File, e.Line, e.Why)
		}
		for _, u := range p.Unmended {
			fmt.Fprintf(w, "      no diff   %s\n", u)
		}
		if p.Remedy != "" {
			fmt.Fprintf(w, "      remedy    %s\n", p.Remedy)
		}
	}
	if patch == "" {
		fmt.Fprintln(w, "\nNo violation is mechanically mendable; the remedies above are for the session to act on.")
		return
	}
	fmt.Fprintln(w, "\nThe proposed diff — fathom has applied nothing. Review it, then `fathom config propose --patch | git apply` from the repo root, or edit by hand:")
	fmt.Fprintln(w)
	_, _ = io.WriteString(w, patch)
}
