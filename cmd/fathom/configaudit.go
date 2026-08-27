package main

// configaudit.go — CC-5/CC-6/CC-7 (docs/plans/custodian.md): `fathom
// config audit`, the config custodian's verb (vision 22, "instructed means
// true"). Fathom is the instrument, never the hand (§3b): everything here
// REPORTS — the session with a human present decides what to edit.
// Mechanical only: file reads and regexps, no LLM; the one kind that asks
// the daemon (stanza_tools_served, CC-14) reports unable-to-check when it
// is down, never a violation.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/custodian"
)

func cmdConfigAudit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("config audit", stderr)
	repo := fs.String("repo", ".", "repository root")
	asJSON := fs.Bool("json", false, "emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, err := filepath.Abs(*repo)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	concerns, loadErrs := loadConcernsBoth(root)
	// A broken concerns file is reported loudly and the audit continues
	// over what DID load — but the run cannot then claim a clean bill:
	// partial coverage plus exit 0 would read as "all hold".
	for _, e := range loadErrs {
		fmt.Fprintf(stderr, "config: %v\n", e)
	}

	if len(concerns) == 0 && len(loadErrs) == 0 {
		fmt.Fprintf(stdout, "no concerns registered — the custodian has nothing to hold this repo to.\n")
		fmt.Fprintf(stdout, "Write ~/.fathom/concerns.cue (user default) or %s (this repo governs);\n", filepath.Join(".fathom", "concerns.cue"))
		fmt.Fprintf(stdout, "see docs/design/config-custodian.md for the concern model.\n")
		return 0
	}

	findings := custodian.Audit(root, concerns)
	violatedForBeat := 0
	for _, f := range findings {
		if f.Status == custodian.StatusViolated {
			violatedForBeat++
		}
	}
	// CC-8: the audit records that it LOOKED — the custodian flywheel
	// loop distinguishes "violations being consumed" from "accumulating
	// unread" by exactly this beat.
	emitBeat("custodian", "audit", map[string]string{"violated": fmt.Sprintf("%d", violatedForBeat)})

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(configAuditJSON(root, findings)); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	} else {
		renderConfigAudit(stdout, root, findings)
	}

	violated := 0
	for _, f := range findings {
		if f.Status == custodian.StatusViolated {
			violated++
		}
	}
	if violated > 0 || len(loadErrs) > 0 {
		return 1
	}
	return 0
}

func renderConfigAudit(w io.Writer, root string, findings []custodian.Finding) {
	var holds, violated, unable, off int
	for _, f := range findings {
		switch f.Status {
		case custodian.StatusHolds:
			holds++
		case custodian.StatusViolated:
			violated++
		case custodian.StatusUnable:
			unable++
		case custodian.StatusOff:
			off++
		}
	}
	fmt.Fprintf(w, "config audit for %s (%d concern(s): %d hold, %d violated, %d unable to check, %d off)\n\n",
		root, len(findings), holds, violated, unable, off)

	for _, f := range findings {
		mark := "✓"
		switch f.Status {
		case custodian.StatusViolated:
			mark = "✗"
		case custodian.StatusUnable:
			mark = "~"
		case custodian.StatusOff:
			mark = "–"
		}
		fmt.Fprintf(w, "  %s %-18s %s\n", mark, f.Concern.ID, f.Detail)
		if f.Status == custodian.StatusViolated {
			fmt.Fprintf(w, "    %-18s want: %s  [%s scope]\n", "", f.Concern.Want, f.Concern.Scope)
			if f.Concern.Why != "" {
				fmt.Fprintf(w, "    %-18s why:  %s\n", "", f.Concern.Why)
			}
			for _, ev := range f.Evidence {
				fmt.Fprintf(w, "    %-18s   %s\n", "", ev)
			}
		}
	}

	if violated > 0 {
		fmt.Fprintf(w, "\nA violated concern means an instruction in this repo's config is no longer true.\n")
		fmt.Fprintf(w, "Fathom reports; you (or the session you are in) decide what to edit.\n")
	}
}

// configAuditModel is the shape the observatory will render (law 10: one
// shape, many surfaces).
type configAuditModel struct {
	RepoRoot string              `json:"repo_root"`
	Holds    int                 `json:"holds"`
	Violated int                 `json:"violated"`
	Unable   int                 `json:"unable"`
	Off      int                 `json:"off"`
	Findings []custodian.Finding `json:"findings"`
}

func configAuditJSON(root string, findings []custodian.Finding) configAuditModel {
	m := configAuditModel{RepoRoot: root, Findings: findings}
	for _, f := range findings {
		switch f.Status {
		case custodian.StatusHolds:
			m.Holds++
		case custodian.StatusViolated:
			m.Violated++
		case custodian.StatusUnable:
			m.Unable++
		case custodian.StatusOff:
			m.Off++
		}
	}
	return m
}

// loadConcernsBoth reads both scopes and resolves them (project governs,
// law 13). Load errors are returned for loud reporting — never a silent
// partial read.
func loadConcernsBoth(root string) ([]custodian.Concern, []error) {
	var errs []error
	var user, project []custodian.Concern
	if home, err := os.UserHomeDir(); err == nil {
		s, lerr := custodian.LoadFile(filepath.Join(home, ".fathom", custodian.UserFileName), custodian.ScopeUser)
		if lerr != nil {
			errs = append(errs, lerr)
		} else {
			user = s
		}
	}
	s, lerr := custodian.LoadFile(filepath.Join(root, ".fathom", custodian.ProjectFileName), custodian.ScopeProject)
	if lerr != nil {
		errs = append(errs, lerr)
	} else {
		project = s
	}
	return custodian.Merge(user, project), errs
}

// violatedConcernCount is how many of this repo's registered concerns are
// violated right now — mechanical file reads, no RPC. Returns 0 when the
// repo registers none or the concern file will not load: an unreadable
// custodian config is a doctor finding, not a session-brief alarm.
func violatedConcernCount(projDir string) int {
	concerns, errs := loadConcernsBoth(projDir)
	if len(errs) > 0 || len(concerns) == 0 {
		return 0
	}
	violated := 0
	for _, f := range custodian.Audit(projDir, concerns) {
		if f.Status == custodian.StatusViolated {
			violated++
		}
	}
	return violated
}

// concernAttentionLine is the session-brief surface (CC-7), now the composed
// attention line (EP-19): the custodian's count and the intent corpus's
// unproven claims share ONE line. It reports; it never edits (vision 22 §3b).
func concernAttentionLine(projDir string) string {
	return attentionLine(projDir, time.Now())
}
