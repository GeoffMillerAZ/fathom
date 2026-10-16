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

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"

	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
	"github.com/geoffmilleraz/fathom/internal/readcoverage"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func cmdConfigAudit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("config audit", stderr)
	repo := fs.String("repo", ".", "repository root")
	asJSON := fs.Bool("json", false, "emit JSON instead of a table")
	format := fs.String("format", "", "output format: text (default) | json | sarif — SARIF 2.1.0 of the violated concerns, one rule per concern id, located at the first evidence file:line")
	hook := fs.Bool("hook", false, "run as the harness ConfigChange hook: audit the changed repo, write one beat, exit 0 always (configchangehook.go)")
	hookEvent := fs.String("event", "", "which harness event --hook is answering: config-change (default) | file-changed — only the beat's trigger differs")
	codeFacts := fs.Bool("code-facts", false, "audit code-fact-shaped sentences across every auto-loaded file — CLAUDE.md, AGENTS.md, .cursor/rules, .windsurf/rules, the Cline memory bank, Claude Code auto-memory (vision 29). OFF by default: classify only, never autofix; combine with --strict to fail on drift")
	strict := fs.Bool("strict", false, "with --code-facts: exit 1 if any sentence classifies anchored-drifted (default: advisory, exit 0 always)")
	harnessFlag := addHarnessFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *hook {
		return cmdConfigAuditHook(*harnessFlag, *hookEvent, stderr)
	}
	if *codeFacts {
		switch *format {
		case "", "text", "json":
		default:
			fmt.Fprintf(stderr, "error: --code-facts does not support --format %q (text or json only)\n", *format)
			return 1
		}
		root, err := filepath.Abs(*repo)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		wantJSON := *asJSON || *format == "json"
		return cmdConfigAuditCodeFacts(root, wantJSON, *strict, stdout, stderr)
	}
	if *asJSON && *format == "" {
		*format = "json"
	}
	switch *format {
	case "", "text", "json", "sarif":
	default:
		fmt.Fprintf(stderr, "error: --format %q: want text, json or sarif\n", *format)
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

	findings := custodian.Audit(root, concerns, custodian.WithWiringDrift(wiringCurrentEvaluator))
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

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(configAuditJSON(root, findings)); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "sarif":
		// EVX-3: the run's provenance names the repository's commit.
		_, _ = stdout.Write(cliSARIF(receipt.Run{Dir: root}, "fathom config audit", configAuditSARIF(root, findings)))
	default:
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

// configAuditSARIF renders findings as SARIF results: violated concerns are
// errors located at their first evidence citation (file:line), and
// unable-to-check concerns are notes on the concerns file — a pane that
// showed only violations would read a down daemon as a clean bill (law 3).
func configAuditSARIF(root string, findings []custodian.Finding) []verify.SARIFResult {
	var out []verify.SARIFResult
	for _, f := range findings {
		switch f.Status {
		case custodian.StatusViolated:
			file, line := "", 0
			if len(f.Evidence) > 0 {
				file, line = sarifLocation(root, f.Evidence[0])
			}
			if file == "" {
				file = ".fathom/concerns.cue"
			}
			out = append(out, verify.SARIFResult{
				RuleID: f.Concern.ID, Level: "error",
				Message: f.Concern.Want + " — " + f.Detail,
				File:    file, Line: line,
				Help: configConcernHelp(f.Concern.Want),
			})
		case custodian.StatusUnable:
			out = append(out, verify.SARIFResult{
				RuleID: f.Concern.ID, Level: "note",
				Message: "unable to check — " + f.Detail,
				File:    ".fathom/concerns.cue",
				Help:    configConcernHelp(f.Concern.Want),
			})
		}
	}
	return out
}

// configConcernHelp is a concern's SARIF rule help: what the concern wants,
// and where it is registered.
func configConcernHelp(want string) string {
	return "A registered concern (concerns.cue) about this repository's agent configuration: " + want + "."
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
			fmt.Fprintf(w, "    %-18s want: %s  [%s]\n", "", f.Concern.Want, f.Concern.Provenance())
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

// loadConcernsBoth reads both scopes and resolves them — the packs either
// scope enables at the lowest precedence, then the user file, then the
// project file, which governs (law 13; custodian.Resolve, CC-12). Load
// errors are returned for loud reporting — never a silent partial read: a
// scope that fails to load contributes neither its concerns nor its packs,
// and the error says so.
func loadConcernsBoth(root string) ([]custodian.Concern, []error) {
	var errs []error
	var user, project custodian.ScopeFile
	if userPath := fathomhome.Path(custodian.UserFileName); userPath != "" {
		s, lerr := custodian.LoadScopeFile(userPath, custodian.ScopeUser)
		if lerr != nil {
			errs = append(errs, lerr)
		} else {
			user = s
		}
	}
	s, lerr := custodian.LoadScopeFile(filepath.Join(root, ".fathom", custodian.ProjectFileName), custodian.ScopeProject)
	if lerr != nil {
		errs = append(errs, lerr)
	} else {
		project = s
	}
	resolved, rerr := custodian.Resolve(user, project)
	if rerr != nil {
		// A shipped pack that does not load is fathom's defect, not the
		// repo's; the files' own concerns still get audited.
		errs = append(errs, rerr)
		return custodian.Merge(user.Concerns, project.Concerns), errs
	}
	return resolved, errs
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
	for _, f := range custodian.Audit(projDir, concerns, custodian.WithWiringDrift(wiringCurrentEvaluator)) {
		if f.Status == custodian.StatusViolated {
			violated++
		}
	}
	return violated
}

// concernAttentionLine is the session-brief surface (CC-7), now the composed
// attention line (EP-19): the custodian's count and the intent corpus's
// unproven claims share ONE line. It reports; it never edits (vision 22 §3b).
//
// No session identity is resolved here, so the intent half reads under an
// unread registry (PE-25): nothing is excluded, never guessed.
func concernAttentionLine(projDir string) string {
	return attentionLine(projDir, time.Now(), ledgerroots.Unread("the config audit resolves no service identity"))
}

// cmdConfigAuditCodeFacts is `fathom config audit --code-facts` (VB-28):
// classify every code-fact-shaped sentence across the auto-loaded corpus.
// Mechanical only, same discipline as the rest of this command — reports,
// never edits — and, per the design, advisory: exit 0 unless --strict asks
// for a gate on drift. Every read goes through custodian.AuditCodeFacts'
// read-manifest, so an unreadable file or directory is named and the
// sentence count is rendered as a lower bound (readcoverage.RenderCount)
// rather than a bare total that would read as a clean sweep over a
// directory this audit could not even list (ADR-027's lesson).
func cmdConfigAuditCodeFacts(root string, asJSON, strict bool, stdout, stderr io.Writer) int {
	home, _ := os.UserHomeDir()
	findings, coverage, err := custodian.AuditCodeFacts(root, home)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(codeFactsAuditJSON{
			RepoRoot: root,
			Findings: findings,
			Coverage: codeFactsCoverageJSON{
				Complete:   coverage.Complete(),
				Note:       readcoverage.Note(coverage),
				Unreadable: coverage.Unreadable(),
				Entries:    len(coverage.Entries),
			},
		}); encErr != nil {
			fmt.Fprintf(stderr, "error: %v\n", encErr)
			return 1
		}
	} else {
		renderCodeFactsAudit(stdout, findings, coverage)
	}

	if note := readcoverage.Note(coverage); note != "" {
		fmt.Fprintf(stderr, "config: %s — this run is a lower bound, not a clean bill\n", note)
	}

	if strict {
		for _, f := range findings {
			if f.Class == custodian.ClassAnchoredDrifted {
				return 1
			}
		}
	}
	return 0
}

type codeFactsAuditJSON struct {
	RepoRoot string                      `json:"repo_root"`
	Findings []custodian.CodeFactFinding `json:"findings"`
	Coverage codeFactsCoverageJSON       `json:"coverage"`
}

type codeFactsCoverageJSON struct {
	Complete   bool                 `json:"complete"`
	Note       string               `json:"note,omitempty"`
	Unreadable []readmanifest.Entry `json:"unreadable,omitempty"`
	Entries    int                  `json:"entries"`
}

func renderCodeFactsAudit(w io.Writer, findings []custodian.CodeFactFinding, coverage readmanifest.Manifest) {
	var fresh, drifted, unanchorable, preference int
	for _, f := range findings {
		switch f.Class {
		case custodian.ClassAnchoredFresh:
			fresh++
		case custodian.ClassAnchoredDrifted:
			drifted++
		case custodian.ClassUnanchorable:
			unanchorable++
		case custodian.ClassPreference:
			preference++
		}
	}
	fmt.Fprintf(w, "code-fact sentence audit (%s: %d anchored-fresh, %d anchored-drifted, %d unanchorable, %d preference)\n\n",
		readcoverage.RenderCount(coverage, len(findings), "sentence(s)"), fresh, drifted, unanchorable, preference)

	for _, f := range findings {
		if f.Class == custodian.ClassPreference {
			continue // advisory table highlights code claims only
		}
		mark := "~"
		switch f.Class {
		case custodian.ClassAnchoredFresh:
			mark = "✓"
		case custodian.ClassAnchoredDrifted:
			mark = "✗"
		}
		fmt.Fprintf(w, "  %s %-24s %s:%d  %s\n", mark, f.Class, f.File, f.Line, f.Sentence)
		if f.Receipt != "" {
			fmt.Fprintf(w, "      receipt: %s\n", f.Receipt)
		}
	}

	if note := readcoverage.Note(coverage); note != "" {
		fmt.Fprintf(w, "\n%s — this audit is a LOWER BOUND, not a clean bill.\n", note)
	}
	fmt.Fprintf(w, "\n--code-facts is advisory and OFF by default until precision holds on a larger labelled set (VB-29); pass --strict to fail on anchored-drifted sentences.\n")
}
