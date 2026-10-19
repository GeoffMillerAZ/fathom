package main

// assess.go — the priced judgment lanes (docs/design/assess-lanes.md,
// ADR-046): DO-3's benchmark lane (`fathom assess`) judges one unit's
// on-disk work products; IR-3's intervention lane (`fathom assess
// --interventions`) judges the retro rows the mechanics cannot decide.
//
// Both are explicitly invoked CLI verbs. Neither reads a transcript: the
// state is built from git and the local ledgers only, through
// internal/assess's builders, which take work products and nothing else.
// Every model call goes through the decision port as typed noul/choice
// questions (I14 — the port is the sanctioned egress, never a new one).
//
// A live run clears four gates, in order, before anything is sent:
//  1. the `assess` capability (default off; fails CLOSED on a malformed
//     config — an unreadable file is never consent to spend);
//  2. the pre-flight token cap (before any client is constructed);
//  3. a decision provider ([decide] provider != none);
//  4. the `assess` site in [decide] sites.
// Between 2 and 3 the isolation environment is set (IR-4): a CLI
// provider's judge child inherits FATHOM_CHANNELS=off and the spend label.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/assess"
	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/cost"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
	"github.com/geoffmilleraz/fathom/internal/llmsec"
	"github.com/geoffmilleraz/fathom/internal/metrics"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// assessDefaultMaxTokens is the pre-flight cap on estimated input tokens.
const assessDefaultMaxTokens = 40000

// assessCallTimeout bounds the one provider call a run makes.
const assessCallTimeout = 10 * time.Minute

// Seams (package vars so tests inject fakes; production never reassigns).
var (
	// assessOpenPorts opens the decision port for a live run against the
	// ASSESSED repository's config — the same fail-open glue every other
	// port site uses (openDecisionPorts).
	assessOpenPorts = defaultAssessOpenPorts
	// assessGuidance re-derives the guidance a firing served for a file,
	// from on-disk sources, at assess time.
	assessGuidance = defaultAssessGuidance
	// assessSetenv applies the IR-4 isolation environment.
	assessSetenv = os.Setenv
)

// assessLedgerPath resolves the assess ledger: $FATHOM_ASSESS_LEDGER, else
// ~/.fathom/assess-ledger.jsonl — per-user central like the session and
// verify ledgers it sits beside, and never the session ledger (IR-4).
func assessLedgerPath() string {
	return fathomhome.PerFile("FATHOM_ASSESS_LEDGER", "assess-ledger.jsonl")
}

// countAssessRuns is the capability probe's evidence: assess runs the
// ledger holds for root.
func countAssessRuns(root string) int {
	rows, err := assess.ReadRows(assessLedgerPath())
	if err != nil {
		return 0
	}
	n := 0
	for _, r := range rows {
		if sameRepoPath(r.RepoRoot, root) {
			n++
		}
	}
	return n
}

type assessOpts struct {
	dryRun    bool
	asJSON    bool
	maxTokens int
}

func cmdAssess(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "spend" {
		return cmdAssessSpend(args[1:], stdout, stderr)
	}
	fs := newFlagSet("assess", stderr)
	session := fs.String("session", "", "session id (default: the most recent stamped session for --project)")
	project := fs.String("project", ".", "project root")
	interventions := fs.Bool("interventions", false, "judge the retro rows the mechanics cannot decide, instead of the whole unit")
	dryRun := fs.Bool("dry-run", false, "print exactly what would be sent and its estimated tokens; send nothing, spend nothing")
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	maxTokens := fs.Int("max-tokens", assessDefaultMaxTokens, "pre-flight cap on estimated input tokens; above it the run refuses before any call")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, err := filepath.Abs(*project)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	beats, err := readRetroBeats(beatLogPath())
	if err != nil {
		fmt.Fprintf(stderr, "error: reading beat log: %v\n", err)
		return 1
	}
	var projBeats []retroBeat
	for _, b := range beats {
		if b.Meta["project_dir"] == root {
			projBeats = append(projBeats, b)
		}
	}
	sid := *session
	if sid == "" {
		for i := len(projBeats) - 1; i >= 0; i-- {
			if s := projBeats[i].Meta["session"]; s != "" {
				sid = s
				break
			}
		}
	}
	start, end, ok := assessWindow(projBeats, sid)
	if !ok {
		fmt.Fprintf(stdout, "no unit to assess: no beats stamped with session %q for %s — a session without stamped beats has no window, and assess never guesses one.\n", sid, root)
		return 0
	}
	opts := assessOpts{dryRun: *dryRun, asJSON: *asJSON, maxTokens: *maxTokens}
	if *interventions {
		return assessInterventions(root, sid, start, end, projBeats, opts, stdout, stderr)
	}
	return assessUnit(root, sid, start, end, opts, stdout, stderr)
}

// assessWindow is the unit: the span of the session's own stamped beats in
// this project, plus one minute — the window DO-1 and IR-2 already use.
func assessWindow(projBeats []retroBeat, sid string) (time.Time, time.Time, bool) {
	var start, end time.Time
	if sid == "" {
		return start, end, false
	}
	for _, b := range projBeats {
		if b.Meta["session"] != sid {
			continue
		}
		if start.IsZero() || b.At.Before(start) {
			start = b.At
		}
		if b.At.After(end) {
			end = b.At
		}
	}
	return start, end.Add(time.Minute), !start.IsZero()
}

// ---- DO-3: the unit lane --------------------------------------------------

func assessUnit(root, sid string, start, end time.Time, opts assessOpts, stdout, stderr io.Writer) int {
	ev, ignored := gatherUnitEvidence(root, sid, start, end, cliLedgerRoots(defaultFathomURL()))
	if p := ignored.Phrase(verifyLedgerNoun); p != "" {
		fmt.Fprintf(stderr, "gate results: %s\n", p) // PE-24
	}
	if len(ev.Commits) == 0 {
		fmt.Fprintf(stdout, "nothing on disk to judge: session %s landed no commit in %s between %s and %s — no call made, nothing spent.\n",
			sid, root, ev.WindowStart, ev.WindowEnd)
		return 0
	}
	built := assess.BuildUnit(ev, assess.DefaultCaps(), assessRedactor())
	return runAssess(assessRun{
		lane: assess.LaneUnit, label: assess.LabelUnit, root: root, session: sid,
		windowStart: ev.WindowStart, windowEnd: ev.WindowEnd, built: built,
	}, opts, stdout, stderr)
}

// gatherUnitEvidence reads a unit's work products from git and the local
// ledgers — commits whose commit time falls in the window (committed work
// only: the working tree is not what shipped), the plan rows those commits
// changed, and the gate results recorded in the window. Gate results are
// the rows under roots' registered service roots (PE-24); ignored is what
// that left out, for the caller to name.
func gatherUnitEvidence(root, sid string, start, end time.Time, roots ledgerroots.Roots) (assess.UnitEvidence, ledgerroots.Ignored) {
	ev := assess.UnitEvidence{
		WindowStart: start.UTC().Format(time.RFC3339),
		WindowEnd:   end.UTC().Format(time.RFC3339),
	}
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	out, err := exec.Command("git", "-C", root, "log", "--reverse",
		"--since="+ev.WindowStart, "--until="+ev.WindowEnd, "--format=%H%x09%s", "HEAD").Output()
	if err == nil {
		for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			sha, subject, ok := strings.Cut(ln, "\t")
			if !ok || sha == "" {
				continue
			}
			//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
			patch, _ := exec.Command("git", "-C", root, "show", "--format=", "--patch", "--no-color", "--no-ext-diff", sha).Output()
			ev.Commits = append(ev.Commits, assess.Commit{SHA: sha, Subject: subject, Patch: string(patch)})
			//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
			plan, _ := exec.Command("git", "-C", root, "show", "--format=", "--unified=0", "--no-color", "--no-ext-diff", sha, "--", "docs/plans").Output()
			ev.PlanDeltas = append(ev.PlanDeltas, parsePlanDeltas(string(plan))...)
		}
	}
	keep := roots.NewFilter()
	if data, rerr := readFileMaybe(verifyLedgerPath()); rerr == nil {
		for _, ln := range strings.Split(string(data), "\n") {
			var r verifyLedgerRow
			if strings.TrimSpace(ln) == "" || json.Unmarshal([]byte(ln), &r) != nil {
				continue
			}
			at, perr := time.Parse(time.RFC3339, r.At)
			if perr != nil || at.Before(start) || at.After(end) || !sameRepoPath(r.RepoRoot, root) || !keep.Keep(r.RepoRoot) {
				continue
			}
			ev.Gates = append(ev.Gates, assess.GateResult{Plan: r.Plan, Origin: r.Origin, Claimed: r.Claimed, Verified: r.Verified, Failing: r.Failing})
		}
	}
	for _, e := range dedupedLedgerRows() {
		if e.SessionID == sid {
			ev.VerifyCatches = e.VerifyCatches
		}
	}
	return ev, keep.Ignored()
}

var planCheckboxLine = regexp.MustCompile(`^([+-])\s*- \[([ xX-])\]\s+(.*)$`)
var planRowID = regexp.MustCompile(`\{id:\s*([A-Za-z0-9_.-]+)`)

// parsePlanDeltas reads a `git show --unified=0 -- docs/plans` patch and
// classifies every changed checkbox row: flipped to done, reopened, added,
// or removed. Rows pair by their `{id: …}` when present, else by text.
func parsePlanDeltas(patch string) []assess.PlanDelta {
	type side struct {
		state, text string
	}
	var out []assess.PlanDelta
	plan := ""
	removed := map[string]side{}
	var addedOrder []string
	added := map[string]side{}
	flush := func() {
		for _, k := range addedOrder {
			a := added[k]
			r, had := removed[k]
			delete(removed, k)
			switch {
			case a.state == "x" && (!had || r.state != "x"):
				out = append(out, assess.PlanDelta{Plan: plan, Kind: assess.DeltaFlipped, Row: a.text})
			case a.state != "x" && had && r.state == "x":
				out = append(out, assess.PlanDelta{Plan: plan, Kind: assess.DeltaReopened, Row: a.text})
			case !had:
				out = append(out, assess.PlanDelta{Plan: plan, Kind: assess.DeltaAdded, Row: a.text})
			}
		}
		var gone []string
		for k := range removed {
			gone = append(gone, k)
		}
		sort.Strings(gone)
		for _, k := range gone {
			out = append(out, assess.PlanDelta{Plan: plan, Kind: "removed", Row: removed[k].text})
		}
		removed, added, addedOrder = map[string]side{}, map[string]side{}, nil
	}
	for _, ln := range strings.Split(patch, "\n") {
		if strings.HasPrefix(ln, "+++ ") {
			flush()
			plan = strings.TrimPrefix(strings.TrimPrefix(ln, "+++ "), "b/")
			continue
		}
		if strings.HasPrefix(ln, "--- ") || strings.HasPrefix(ln, "diff --git") {
			continue
		}
		m := planCheckboxLine.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		state := strings.ToLower(m[2])
		key := m[3]
		if id := planRowID.FindStringSubmatch(m[3]); id != nil {
			key = "id:" + id[1]
		}
		row := "- [" + m[2] + "] " + m[3]
		if m[1] == "-" {
			removed[key] = side{state: state, text: row}
		} else {
			if _, dup := added[key]; !dup {
				addedOrder = append(addedOrder, key)
			}
			added[key] = side{state: state, text: row}
		}
	}
	flush()
	return out
}

// ---- IR-3: the intervention lane -------------------------------------------

func assessInterventions(root, sid string, start, end time.Time, projBeats []retroBeat, opts assessOpts, stdout, stderr io.Writer) int {
	m, ok := buildRetroModel(root, sid, projBeats)
	if !ok {
		fmt.Fprintf(stdout, "no firings stamped with session %s for %s — nothing to judge.\n", sid, root)
		return 0
	}
	type key struct{ channel, file string }
	seen := map[key]bool{}
	var judged []retroFiringOutcome
	var notes []string
	untouched := map[key]bool{}
	for _, f := range m.Firings {
		k := key{f.Channel, f.File}
		switch {
		case f.Channel == "custodian":
			notes = append(notes, "custodian attention: decided mechanically ("+f.Outcome+") — not sent")
		case f.File == "":
			// counted below through m.Unjoinable
		case f.Outcome == "committed" || f.Outcome == "modified-uncommitted":
			if !seen[k] {
				seen[k] = true
				judged = append(judged, f)
			}
		case f.Outcome == "untouched":
			untouched[k] = true
		}
	}
	var untouchedKeys []key
	for k := range untouched {
		if !seen[k] {
			untouchedKeys = append(untouchedKeys, k)
		}
	}
	sort.Slice(untouchedKeys, func(i, j int) bool {
		if untouchedKeys[i].file != untouchedKeys[j].file {
			return untouchedKeys[i].file < untouchedKeys[j].file
		}
		return untouchedKeys[i].channel < untouchedKeys[j].channel
	})
	for _, k := range untouchedKeys {
		notes = append(notes, fmt.Sprintf("%s %s: not judgeable from disk — nothing changed after the firing, and intent lives only in the transcript, which assess never reads", k.channel, k.file))
	}
	if m.Unjoinable > 0 {
		notes = append(notes, fmt.Sprintf("%d firing(s) unjoinable (no session/file stamp, recorded before IR-1) — counted, never judged", m.Unjoinable))
	}
	sort.Slice(judged, func(i, j int) bool {
		if judged[i].File != judged[j].File {
			return judged[i].File < judged[j].File
		}
		return judged[i].Channel < judged[j].Channel
	})
	startS, endS := start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)
	var items []assess.Item
	for _, f := range judged {
		// The retro's outcome is wider than the evidence assess can show:
		// `committed` counts commits made after the window too. A question
		// with no diff asks the judge about nothing, so it is never asked.
		diff := fileDiffInWindow(root, f.File, startS, endS, f.Outcome == "modified-uncommitted")
		if strings.TrimSpace(diff) == "" {
			notes = append(notes, fmt.Sprintf("%s %s: unjudgeable — no change to the file inside the session window (the retro's %q also counts changes made after it), and assess never asks with an empty diff", f.Channel, f.File, f.Outcome))
			continue
		}
		text, prov, gok := assessGuidance(root, f.File, f.Channel)
		if !gok || strings.TrimSpace(text) == "" {
			notes = append(notes, fmt.Sprintf("%s %s: unjudgeable — the governing guidance is not recoverable from disk now, and assess never asks with empty guidance", f.Channel, f.File))
			continue
		}
		items = append(items, assess.Item{
			Channel: f.Channel, File: f.File, Outcome: f.Outcome, Guidance: text, Provenance: prov, Diff: diff,
		})
	}
	if len(items) == 0 {
		for _, n := range notes {
			fmt.Fprintf(stdout, "  %s\n", n)
		}
		fmt.Fprintf(stdout, "nothing undecidable to judge for session %s — the mechanical retro already decides every row; no call made, nothing spent.\n", sid)
		return 0
	}
	built := assess.BuildInterventions(items, assess.DefaultCaps(), assessRedactor())
	built.Skipped = append(notes, built.Skipped...)
	return runAssess(assessRun{
		lane: assess.LaneInterventions, label: assess.LabelInterventions, root: root, session: sid,
		windowStart: startS, windowEnd: endS, built: built,
	}, opts, stdout, stderr)
}

// fileDiffInWindow is one file's committed hunks inside the window, plus
// the working-tree diff when the retro says the file is modified and
// uncommitted. An untracked new file (the retro reads `git status`'s `??`
// as modified) never appears in `git diff HEAD`, so it is diffed against
// /dev/null: its whole content is the change. "" means no evidence.
func fileDiffInWindow(root, rel, since, until string, withWorkingTree bool) string {
	var b strings.Builder
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	if out, err := exec.Command("git", "-C", root, "log", "--since="+since, "--until="+until,
		"--format=--- commit %h %s", "--patch", "--no-color", "--no-ext-diff", "--", rel).Output(); err == nil {
		b.Write(out)
	}
	if !withWorkingTree {
		return b.String()
	}
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	if out, err := exec.Command("git", "-C", root, "diff", "--no-color", "--no-ext-diff", "HEAD", "--", rel).Output(); err == nil && len(out) > 0 {
		b.WriteString("--- uncommitted working-tree change\n")
		b.Write(out)
		return b.String()
	}
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	if others, err := exec.Command("git", "-C", root, "ls-files", "--others", "--exclude-standard", "--", rel).Output(); err == nil && strings.TrimSpace(string(others)) != "" {
		// --no-index exits 1 when the two sides differ, which they always do here.
		//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
		out, err := exec.Command("git", "-C", root, "diff", "--no-index", "--no-color", "--no-ext-diff", "--", os.DevNull, rel).Output()
		var ee *exec.ExitError
		if (err == nil || (errors.As(err, &ee) && ee.ExitCode() == 1)) && len(out) > 0 {
			b.WriteString("--- untracked new file (created, never committed)\n")
			b.Write(out)
		}
	}
	return b.String()
}

// defaultAssessGuidance re-derives a firing's guidance from on-disk
// sources at assess time. Guards: the matching note in .fathom/guard.cue.
// Obligations: the rules the loopback daemon serves for the file NOW (the
// store is on disk; IR-1 records that a firing happened, not what it said,
// so the provenance says "as served now"). Unrecoverable → ok=false, and
// the row is named unjudgeable rather than asked with empty guidance.
func defaultAssessGuidance(root, rel, channel string) (string, string, bool) {
	switch channel {
	case "guard":
		gs, ok := loadGuardSet(root)
		if !ok {
			return "", "", false
		}
		g, matched := matchingFileGuard(gs, rel)
		if !matched || strings.TrimSpace(g.Note) == "" {
			return "", "", false
		}
		return g.Note, "guard note in .fathom/guard.cue at assess time (the note as served then is not recorded)", true
	case "obligations":
		url := defaultFathomURL()
		hc := newStopHookClient()
		svc, srel, ok := resolveServiceAndRel(hc, url, root, filepath.Join(root, rel))
		if !ok {
			return "", "", false
		}
		obl, found, _, _, err := fetchObligations(url, svc, srel, 6, 3*time.Second, "")
		if err != nil || !found || len(obl.Rules) == 0 {
			return "", "", false
		}
		var b strings.Builder
		for _, r := range obl.Rules {
			fmt.Fprintf(&b, "- %s [%s]\n", strings.TrimSpace(r.Rule), r.Src)
		}
		return b.String(), "obligations the daemon serves for this file at assess time (the set served at firing time is not recorded)", true
	}
	return "", "", false
}

// ---- the shared pipeline ---------------------------------------------------

type assessRun struct {
	lane, label, root, session string
	windowStart, windowEnd     string
	built                      assess.Built
}

// assessOutput is the --json envelope: the ledger row plus what was
// deliberately not asked and what was cut.
type assessOutput struct {
	assess.Row
	Skipped     []string `json:"skipped,omitempty"`
	Truncations []string `json:"truncations,omitempty"`
	DryRun      bool     `json:"dry_run,omitempty"`
	EstTokensIn int      `json:"est_tokens_in"`
	// Redactions counts the spans the DLP gate replaced in the state sent
	// (or, on a dry run, that would be sent); 0 means it found none.
	Redactions int    `json:"redactions"`
	State      string `json:"state,omitempty"`
}

// assessRedactor is the DLP gate on this lane's egress. The state is raw
// source (commit patches, working-tree diffs), so it runs the kit's
// extended credential catalog (PEM keys, AWS key IDs, GitHub/Slack/Google
// tokens, JWTs) on top of the generic policy compile uses.
func assessRedactor() func(string) (string, int) {
	return llmsec.NewRedactor(llmsec.ExtendedPosture()).RedactCount
}

func runAssess(run assessRun, opts assessOpts, stdout, stderr io.Writer) int {
	req := run.built.Request
	est := assess.EstimateTokens(req)
	if opts.dryRun {
		return renderAssessDryRun(run, est, opts, stdout)
	}

	// Gate 1: consent. Fails CLOSED.
	if on, why := assessConsent(run.root); !on {
		fmt.Fprintf(stderr, "error: %s\n", why)
		fmt.Fprintln(stderr, "       assess spends model tokens, so it runs only with consent: `fathom capabilities set assess on` (this repository) or `fathom capabilities set assess on --scope user`. `fathom assess --dry-run` shows exactly what would be sent and spends nothing.")
		return 1
	}
	// Gate 2: the pre-flight token cap, before any client is constructed.
	if opts.maxTokens > 0 && est > opts.maxTokens {
		fmt.Fprintf(stderr, "error: estimated ~%s input tokens exceeds --max-tokens %s — refusing before any call. Raise --max-tokens deliberately, or assess a smaller unit.\n",
			fmtThousands(est), fmtThousands(opts.maxTokens))
		return 1
	}
	// IR-4: isolation BEFORE the port opens — a CLI provider's judge child
	// inherits both, so fathom's hooks stay silent inside it and any row
	// written there anyway carries the spend label.
	_ = assessSetenv("FATHOM_CHANNELS", "off")
	_ = assessSetenv(assess.SpendLabelEnv, run.label)

	ports := assessOpenPorts(run.root, stderr)
	// Gate 3: a provider.
	if ports == nil || ports.Decider == nil || ports.Decider.Name() == "none" {
		fmt.Fprintln(stderr, "error: no decision provider is configured ([decide] provider = none) — assess speaks only through the decision port (ADR-046, I14). Set [decide] provider = \"llm\" (it resolves your llm profile) or \"typesafe\" in ~/.fathom/config.toml. Nothing was sent.")
		return 1
	}
	// Gate 4: the site allowlist.
	if !decide.SiteAllowed(ports.Cfg, decide.SiteAssess) {
		fmt.Fprintln(stderr, "error: [decide] sites does not include \"assess\" — this configuration's egress allowlist forbids the lane; add \"assess\" to sites to allow it. Nothing was sent.")
		return 1
	}

	started := time.Now().UTC()
	row := assess.Row{
		// Stamped here too (AppendRow stamps its own copy), so the --json
		// envelope mirrors the ledger row exactly.
		SchemaVersion: assess.RowSchemaVersion,
		FathomVersion: buildinfo.Version(),
		At:            started.Format(time.RFC3339),
		RunID:         assessRunID(run, started),
		SpendLabel:    run.label,
		Lane:          run.lane,
		Session:       run.session,
		RepoRoot:      run.root,
		QuestionSet:   run.built.Digest,
		Provider:      ports.Decider.Name(),
		Questions:     len(req.Questions),
		Partial:       run.built.Partial,
		Isolation:     assess.IsolationRegistration,
	}
	ctx, cancel := context.WithTimeout(context.Background(), assessCallTimeout)
	defer cancel()
	// The verb asks from its own site, so the port's per-site cap and miss
	// kill judge it (KS-10, KS-12).
	resp, derr := decide.AtSite(ports.Decider, decide.SiteAssess).Decide(ctx, req)
	row.LatencyMS = time.Since(started).Milliseconds()
	if derr != nil {
		ports.Recorder.RecordError(decide.SiteAssess, ports.Decider.Name(), "", len(req.Questions), time.Since(started), derr)
		row.Error = derr.Error()
		row.TokensNote = "unknown: the call failed"
		row.CostNote = "unknown: the call failed"
		if err := assess.AppendRow(assessLedgerPath(), row); err != nil {
			fmt.Fprintf(stderr, "warning: could not append the assess ledger: %v\n", err)
		}
		fmt.Fprintf(stderr, "error: the provider call failed: %v — nothing was judged, and no verdict is recorded; the failure is on the assess ledger.\n", derr)
		return 2
	}
	// Recorded, never "applied": an assess verdict changes no served content.
	ports.Recorder.Record(decide.SiteAssess, resp, func(decision.Answer) bool { return false })

	row.Model = resp.Model
	row.Verdicts = assess.Verdicts(run.built, resp)
	row.Proceed, row.Confirm, row.Unsure, row.Invalid = assess.BandCounts(row.Verdicts)
	assessMeter(&row, resp)
	if err := assess.AppendRow(assessLedgerPath(), row); err != nil {
		fmt.Fprintf(stderr, "warning: could not append the assess ledger: %v\n", err)
	}
	if opts.asJSON {
		jsonPrint(stdout, assessOutput{Row: row, Skipped: run.built.Skipped, Truncations: run.built.Truncations, EstTokensIn: est, Redactions: run.built.Redactions})
		return 0
	}
	renderAssess(stdout, run, row)
	return 0
}

// assessConsent reads the `assess` capability, failing CLOSED: a
// malformed or unreadable capabilities file is never consent to spend —
// the opposite of capabilityOffAt, which fails open so a broken file
// cannot disarm a safety feature.
func assessConsent(root string) (bool, string) {
	reg := capability.Default()
	c, ok := reg.Get("assess")
	if !ok {
		return false, "the assess capability is not registered in this binary"
	}
	res, errs := loadCapabilityConfig(reg, root)
	if len(errs) > 0 {
		return false, fmt.Sprintf("the capabilities config could not be read (%v) — assess fails CLOSED: an unreadable file is never consent to spend", errs[0])
	}
	s := res.For(c)
	if !s.On {
		return false, fmt.Sprintf("assess is switched off (%s scope)", s.Scope)
	}
	return true, ""
}

// assessMeter prices the run through the compile meter's table
// (rateForModel, cost.PriceUSD): provider-reported tokens only, a named
// note — never a zero — when usage or the price is unknown.
func assessMeter(row *assess.Row, resp decision.Response) {
	in, out := resp.Usage.InputTokens, resp.Usage.OutputTokens
	if in == 0 && out == 0 {
		row.TokensNote = "not reported by the provider"
		row.CostNote = "unknown: the provider reported no token usage"
		return
	}
	row.TokensIn, row.TokensOut = &in, &out
	inRate, outRate, ok := rateForModel(resp.Model)
	if !ok {
		model := resp.Model
		if model == "" {
			model = "(unnamed)"
		}
		row.CostNote = "cost unknown: no price for model " + model + " in this binary's meter (modelprice.go)"
		return
	}
	usd := math.Round((cost.PriceUSD(in, inRate)+cost.PriceUSD(out, outRate))*1e9) / 1e9
	row.CostUSD = &usd
}

func assessRunID(run assessRun, at time.Time) string {
	h := sha256.Sum256([]byte(run.lane + "\x00" + run.root + "\x00" + run.session + "\x00" + at.Format(time.RFC3339Nano)))
	return hex.EncodeToString(h[:8])
}

func assessCostLine(row assess.Row) string {
	switch {
	case row.TokensIn == nil:
		return "cost: tokens " + row.TokensNote + " — " + row.CostNote
	case row.CostUSD == nil:
		return fmt.Sprintf("cost: %s in / %s out tokens — %s", fmtThousands(*row.TokensIn), fmtThousands(*row.TokensOut), row.CostNote)
	}
	return fmt.Sprintf("cost: %s in / %s out tokens — $%.4f (model %s, at the compile meter's flat API-equivalent rate)",
		fmtThousands(*row.TokensIn), fmtThousands(*row.TokensOut), *row.CostUSD, row.Model)
}

func renderAssess(w io.Writer, run assessRun, row assess.Row) {
	fmt.Fprintf(w, "assess (%s lane) — session %s in %s (%s → %s)\n", run.lane, run.session, run.root, run.windowStart, run.windowEnd)
	model := row.Model
	if model == "" {
		model = "(unnamed)"
	}
	fmt.Fprintf(w, "  question set %s · provider %s · model %s\n", row.QuestionSet, row.Provider, model)
	for _, v := range row.Verdicts {
		subject := v.QuestionID
		if run.lane == assess.LaneInterventions {
			subject = v.QuestionID + " " + v.Subject
		}
		answer := v.Answer
		if answer == "" {
			answer = "—"
		}
		fmt.Fprintf(w, "  %-28s %-14s %.2f  %s\n", subject, answer, v.Confidence, v.Reading())
	}
	for _, s := range run.built.Skipped {
		fmt.Fprintf(w, "  skipped: %s\n", s)
	}
	for _, s := range run.built.Truncations {
		fmt.Fprintf(w, "  truncated: %s\n", s)
	}
	if row.Partial {
		fmt.Fprintln(w, "  partial state: a cap cut the evidence — this run never enters a calibration sample")
	}
	fmt.Fprintf(w, "%s\n", assessCostLine(row))
	fmt.Fprintf(w, "uncalibrated: no adjudicated sample exists for question set %s — verdicts are a lens, not evidence (law 14), stay on this machine, and never feed an arm, bet or flywheel figure.\n", row.QuestionSet)
	fmt.Fprintf(w, "spend isolated under %s: metered on the assess ledger as %q, never on the session ledger. The arm carries causality (law 12); this judges quality only.\n",
		assess.IsolationRegistration, row.SpendLabel)
}

func renderAssessDryRun(run assessRun, est int, opts assessOpts, w io.Writer) int {
	if opts.asJSON {
		jsonPrint(w, assessOutput{
			Row: assess.Row{SchemaVersion: assess.RowSchemaVersion, SpendLabel: run.label, Lane: run.lane, Session: run.session, RepoRoot: run.root,
				QuestionSet: run.built.Digest, Questions: len(run.built.Request.Questions), Partial: run.built.Partial, Isolation: assess.IsolationRegistration},
			Skipped: run.built.Skipped, Truncations: run.built.Truncations, DryRun: true, EstTokensIn: est, State: run.built.Request.State,
			Redactions: run.built.Redactions,
		})
		return 0
	}
	fmt.Fprintf(w, "assess --dry-run (%s lane) — session %s in %s (%s → %s): nothing was sent, nothing was spent\n",
		run.lane, run.session, run.root, run.windowStart, run.windowEnd)
	fmt.Fprintf(w, "  question set %s; %d question(s):\n", run.built.Digest, len(run.built.Request.Questions))
	for _, q := range run.built.Request.Questions {
		fmt.Fprintf(w, "    %s (%s): %s\n", q.ID, q.Kind, q.Instructions)
	}
	for _, s := range run.built.Skipped {
		fmt.Fprintf(w, "  skipped: %s\n", s)
	}
	for _, s := range run.built.Truncations {
		fmt.Fprintf(w, "  truncated: %s\n", s)
	}
	line := fmt.Sprintf("  estimated input: ~%s tokens (bytes/4, the compile heuristic)", fmtThousands(est))
	if inRate, _, ok := rateForModel(defaultCompileModel); ok {
		line += fmt.Sprintf("; at the compile meter's flat rate ~$%.4f before output tokens — a live run reports the provider's real figure", cost.PriceUSD(est, inRate))
	}
	fmt.Fprintln(w, line)
	if run.built.Redactions > 0 {
		fmt.Fprintf(w, "  the DLP gate redacted %d span(s) of the state below (llmsec's extended credential catalog)\n", run.built.Redactions)
	} else {
		fmt.Fprintln(w, "  the DLP gate found nothing to redact (llmsec's extended credential catalog — shape matching, not a guarantee)")
	}
	fmt.Fprintln(w, "--- state (exactly what a live run would send, after DLP redaction) ---")
	fmt.Fprintln(w, strings.TrimRight(run.built.Request.State, "\n"))
	fmt.Fprintln(w, "--- end state ---")
	return 0
}

// defaultAssessOpenPorts is the production port opener: the same
// fail-open glue every decision-port site uses, over the user layer and
// the ASSESSED repository's project layer — never the caller's cwd. The
// egress allowlist ([decide] sites) that governs a run is the config of
// the repository whose diffs are sent, so `--project R` from any directory
// honours R's narrowing (I14; doctor's decide rows read the same layer).
func defaultAssessOpenPorts(root string, stderr io.Writer) *decide.Ports {
	loaded, err := config.Load(config.DefaultUserPath(), filepath.Join(root, ".fathom", "config.toml"))
	if err != nil {
		fmt.Fprintf(stderr, "warn: decision port disabled: loading config: %v\n", err)
		return &decide.Ports{Decider: decision.None{}}
	}
	return openDecisionPorts(loaded, "", resolveCallLogPath(), "fathom assess", stderr)
}

// ---- `fathom assess spend` -------------------------------------------------

// cmdAssessSpend totals the assess ledger through the compile spend
// meter's aggregation (metrics.AggregateSpend), grouped by spend label,
// day, or model — assess spend stays on its own meter (IR-4) and is still
// summed by the same arithmetic as compile spend.
func cmdAssessSpend(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assess spend", stderr)
	since := fs.Duration("since", 30*24*time.Hour, "report window, measured back from now")
	by := fs.String("by", "label", "group by: label | day | model")
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	groupBy := metrics.SpendGroupByLayer
	switch *by {
	case "label":
	case "day":
		groupBy = metrics.SpendGroupByDay
	case "model":
		groupBy = metrics.SpendGroupByModel
	default:
		fmt.Fprintf(stderr, "error: --by must be label, day, or model (got %q)\n", *by)
		return 1
	}
	rows, err := assess.ReadRows(assessLedgerPath())
	if err != nil {
		fmt.Fprintf(stderr, "error: reading the assess ledger: %v\n", err)
		return 1
	}
	now := time.Now().UTC()
	opts := metrics.SpendOptions{Since: now.Add(-*since), Until: now, GroupBy: groupBy}
	runs, classes := assess.SpendRuns(rows)
	// Runs whose cost (or tokens) is unknown are aggregated a second time,
	// through the same grouping, so every group and the total can say it is
	// a lower bound or unknown — never print the unknown as a zero.
	var costUnknown, tokensUnknown []core.CompileRun
	byClass := map[string]int{}
	for i, run := range runs {
		if !run.RanAt.IsZero() && (run.RanAt.Before(opts.Since) || run.RanAt.After(opts.Until)) {
			continue // the window AggregateSpend applies
		}
		byClass[classes[i]]++
		if classes[i] != assess.CostPriced {
			costUnknown = append(costUnknown, run)
		}
		if classes[i] == assess.CostFailed || classes[i] == assess.CostNoUsage {
			tokensUnknown = append(tokensUnknown, run)
		}
	}
	report := metrics.AggregateSpend(runs, nil, opts)
	unknownCost, unknownTokens := spendRunsByGroup(costUnknown, opts), spendRunsByGroup(tokensUnknown, opts)
	nUnknownCost := len(costUnknown)
	if *asJSON {
		jsonPrint(stdout, struct {
			metrics.SpendReport
			CostComplete         bool           `json:"cost_complete"`
			TokensComplete       bool           `json:"tokens_complete"`
			UnknownCostRuns      int            `json:"unknown_cost_runs"`
			UnpricedRuns         int            `json:"unpriced_runs"`
			NoUsageRuns          int            `json:"no_usage_runs"`
			FailedRuns           int            `json:"failed_runs"`
			UnknownCostByGroup   map[string]int `json:"unknown_cost_by_group,omitempty"`
			UnknownTokensByGroup map[string]int `json:"unknown_tokens_by_group,omitempty"`
		}{report, nUnknownCost == 0, len(tokensUnknown) == 0, nUnknownCost,
			byClass[assess.CostUnpriced], byClass[assess.CostNoUsage], byClass[assess.CostFailed],
			unknownCost, unknownTokens})
		return 0
	}
	fmt.Fprintf(stdout, "assess spend: %s .. %s (by %s) — the assess ledger, never the session ledger (IR-4)\n", report.Since, report.Until, *by)
	if len(report.Rows) == 0 {
		fmt.Fprintln(stdout, "  no assess runs in window")
		return 0
	}
	for _, r := range report.Rows {
		ut := unknownTokens[r.Group]
		fmt.Fprintf(stdout, "  %-22s runs=%-4d %-18s %-18s %s\n", r.Group, r.Runs,
			spendBound("tokens_in", fmt.Sprintf("%d", r.TokensIn), ut, r.Runs),
			spendBound("tokens_out", fmt.Sprintf("%d", r.TokensOut), ut, r.Runs),
			spendCost(r.CostUSD, unknownCost[r.Group], r.Runs))
	}
	fmt.Fprintf(stdout, "total: runs=%d %s\n", report.TotalRuns, spendCost(report.TotalCostUSD, nUnknownCost, report.TotalRuns))
	if n := byClass[assess.CostFailed]; n > 0 {
		fmt.Fprintf(stdout, "%d failed run(s): the provider call failed, so tokens and cost are unknown — a failed CLI or API call may still have been billed.\n", n)
	}
	if n := byClass[assess.CostNoUsage]; n > 0 {
		fmt.Fprintf(stdout, "%d run(s) whose provider reported no token usage contribute neither tokens nor cost.\n", n)
	}
	if n := byClass[assess.CostUnpriced]; n > 0 {
		fmt.Fprintf(stdout, "%d run(s) with no known price for their model contribute tokens but no cost.\n", n)
	}
	if nUnknownCost > 0 {
		fmt.Fprintln(stdout, "Every figure marked ≥ is a lower bound and every \"unknown\" is unmeasured — never a complete figure, never a zero.")
	}
	return 0
}

// spendRunsByGroup counts runs per spend group through AggregateSpend's own
// grouping and window, so the counts key exactly the report's rows.
func spendRunsByGroup(runs []core.CompileRun, opts metrics.SpendOptions) map[string]int {
	if len(runs) == 0 {
		return nil
	}
	out := map[string]int{}
	for _, r := range metrics.AggregateSpend(runs, nil, opts).Rows {
		out[r.Group] = r.Runs
	}
	return out
}

// spendBound renders key=v when every run's figure is known, key≥v when
// some are unknown (a lower bound), and key=unknown when none is.
func spendBound(key, v string, unknown, runs int) string {
	switch {
	case unknown == 0:
		return key + "=" + v
	case unknown >= runs:
		return key + "=unknown"
	}
	return key + "≥" + v
}

func spendCost(usd float64, unknown, runs int) string {
	s := spendBound("cost", fmt.Sprintf("$%.4f", usd), unknown, runs)
	if unknown > 0 && unknown < runs {
		s += fmt.Sprintf(" (%d of %d run(s) unknown)", unknown, runs)
	}
	return s
}

// assessRunner runs `fathom assess` (registered below; EW-45).
type assessRunner struct{}

func (assessRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdAssess(args, stdout, stderr)
}

func init() {
	// SR-18: frozen (ADR-046/ADR-053)
	declareVerb(verb{Name: "assess", Group: "operate", Order: 180, Hidden: true, Run: assessRunner{},
		Summary: "Priced, explicit, off by default: a model judges a unit's on-disk work (typed questions, never the transcript): [--session S] [--interventions] [--dry-run] [--json] | spend"})
}
