package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
	"github.com/geoffmilleraz/fathom/internal/readcoverage"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// intentaudit.go — EP-9, `fathom intent audit`.
//
// One verb over the intent corpus: what it claims, what an instrument can
// show, and what nothing shows. It never edits the corpus, never evaluates
// prose, and never prints a score — a list with remedies is the shape that
// gets acted on here.
//
// It also never re-verifies plans. Verifying this repo's 45 plans takes ~70
// seconds; U1/U2 read the newest verify-ledger row per plan at HEAD and
// carry the measurement's AGE, so a stale reading is visible as stale
// rather than presented as now.

const intentTopDefault = 20

// intentAuditLearningWindow (GL-14, design doc §6) is how far back `fathom
// intent audit` reports a promotion/retirement — wide enough to span a
// promotion's own 30-day evidence window (design doc §6's default), so a
// transition that just happened is still visible on the NEXT audit even
// when it was recorded a few weeks into the reporting gap.
const intentAuditLearningWindow = 30 * 24 * time.Hour

func cmdIntent(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: intent requires a subcommand (audit, adr-rules, doctrine, export, render)")
		return 1
	}
	switch args[0] {
	case "audit":
		return cmdIntentAudit(args[1:], stdout, stderr)
	case "adr-rules":
		return cmdIntentADRRules(args[1:], stdout, stderr) // CX-19: intentadrrules.go
	case "doctrine":
		return cmdIntentDoctrine(args[1:], stdout, stderr) // DP-3/DP-4: intentdoctrine.go
	case "export":
		return cmdIntentExport(args[1:], stdout, stderr)
	case "render":
		return cmdIntentRender(args[1:], stdout, stderr) // HP-35: intent_playbook.go
	}
	fmt.Fprintf(stderr, "error: unknown intent subcommand %q (audit, adr-rules, doctrine, export, render)\n", args[0])
	return 1
}

func cmdIntentAudit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("intent audit", stderr)
	root := fs.String("root", ".", "repository root")
	service := fs.String("service", "", "service id (default: resolved from the repo)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	kind := fs.String("kind", "", "restrict to kinds: conditions|triggers|invariants|unproven (comma-separated)")
	storePath := fs.String("store", defaultStorePath(), "store the arch_violations trigger instrument reads (CX-21); opened only when a trigger names it")
	top := fs.Int("top", intentTopDefault, "max rows per section before eliding (0 = all)")
	asJSON := fs.Bool("json", false, "emit JSON")
	readOnly := fs.Bool("read-only", false, "render the audit and write NOTHING: no intent-ledger rows (clock, baseline, debt, the enforcement-probe cache) and no beat — the Observatory's intent tab runs it this way (OC-38)")
	ledgerFlag := fs.String("ledger", "", "the intent ledger this run appends to (default: $FATHOM_INTENT_LEDGER, else intent-ledger.jsonl under $FATHOM_HOME or ~/.fathom); a scratch or test run names its own file so the operator's ledger is never touched")
	format := fs.String("format", "", "output format: text (default) | json | sarif — SARIF 2.1.0 of the unproven claims, one rule per kind (U1, U2, …), located at the claim's file:line when the evidence names one (PC-21)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if err := useLedgerFlag("FATHOM_INTENT_LEDGER", *ledgerFlag); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
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

	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	run, err := runIntentAudit(intentAuditOptions{Root: abs, Service: *service, URL: *url, ReadOnly: *readOnly, StorePath: *storePath})
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	corpus, audit, ledger := run.Corpus, run.Audit, run.Ledger

	switch *format {
	case "json":
		return emitIntentJSON(stdout, corpus, audit, ledger, run.LedgerIgnored)
	case "sarif":
		_, _ = stdout.Write(verify.RenderSARIF("fathom intent audit", intentAuditSARIF(abs, corpus, audit)))
		return 0
	}
	renderIntentAudit(stdout, corpus, audit, ledger, strings.Split(*kind, ","), *top, time.Now().UTC())
	if p := run.LedgerIgnored.Phrase(intentLedgerNoun); p != "" {
		fmt.Fprintf(stdout, "\nintent ledger: %s\n", p) // PE-25
	}
	return 0
}

// intentAuditOptions parameterizes one audit run.
type intentAuditOptions struct {
	Root, Service, URL string
	// ReadOnly runs the audit and writes nothing: no clock, baseline or
	// debt row, no enforcement-probe cache row, no beat (OC-38). An audit
	// that appends ledger rows by being LOOKED AT (the 2026-09-25 survey)
	// cannot back a page a person reloads.
	ReadOnly bool
	// HTTP, when set, carries the index oracle's requests (the console
	// passes its origin-stamped client).
	HTTP *http.Client
	// Store is the open store a computable trigger's arch_violations
	// instrument reads (the console passes the daemon's); when nil,
	// StorePath is opened on first read (CX-21, intenttriggers.go).
	Store     store.Store
	StorePath string
}

// intentAuditRun is one run's inputs and findings.
type intentAuditRun struct {
	Service string
	Corpus  intent.Corpus
	Audit   intent.Audit
	// Ledger is the intent ledger's rows under the registered service
	// roots; LedgerIgnored is what that left out (PE-25).
	Ledger        []intent.LedgerRow
	LedgerIgnored ledgerroots.Ignored
}

// runIntentAudit is `fathom intent audit` minus its rendering: the verb
// and the Observatory's intent tab both call it, so the page renders
// exactly what the verb prints.
func runIntentAudit(o intentAuditOptions) (intentAuditRun, error) {
	abs := o.Root
	corpus, err := intent.Load(abs)
	if err != nil {
		return intentAuditRun{}, fmt.Errorf("reading the intent corpus: %w", err)
	}

	svc := o.Service
	if svc == "" {
		// The daemon-backed deterministic resolver, exactly as every hook
		// uses it. The bare basename is NOT good enough: this repo's
		// directory is "sextant" while its indexed service is "fathom", and
		// resolving by basename is the O1 class that once zeroed obligations
		// on a third of edits.
		hc := o.HTTP
		if hc == nil {
			hc = &http.Client{Timeout: 5 * time.Second}
		}
		if cand, ok := resolveIndexedCandidate(hc, o.URL, abs); ok {
			svc = cand.name
		} else if cands := serviceCandidates(abs); len(cands) > 0 {
			svc = cands[0].name
		}
	}

	// PE-24: the ledgers are read under the registered service roots,
	// asked once for the whole run (in process inside the daemon).
	rootsClient := o.HTTP
	if rootsClient == nil {
		rootsClient = &http.Client{Timeout: 5 * time.Second}
	}
	roots := registeredLedgerRoots(rootsClient, o.URL)
	rows, note := planRowStates(abs, roots)
	guardNotes, guardScopes, guardNote2 := repoGuardNotes(abs)
	contracts, contractNote, contractCoverage := planRowContracts(abs)
	ledger, ledgerIgnored := intent.ReadRegisteredLedger(roots) // PE-25
	head := gitHeadShort(abs)
	audit := intent.RunAudit(intent.AuditInput{
		Corpus:   corpus,
		Symbols:  newIndexOracle(o.URL, svc, abs, head, o.ReadOnly, o.HTTP),
		PlanRows: rows,
		PlanNote: note,

		PlanContracts:    contracts,
		PlanContractNote: contractNote,
		ExtraCoverage:    []readmanifest.Manifest{contractCoverage},

		SurfaceFiles: func(globs []string) (int, bool) {
			return filesOnDiskUnder(abs, globs), true
		},

		GuardNotes:  guardNotes,
		GuardScopes: guardScopes,
		GuardNote:   guardNote2,
		// CX-21: computable revisit/promote triggers read their instruments
		// here, when the operator runs the audit — never on a schedule.
		Instruments: intentInstruments(context.Background(), o.Store, o.StorePath, svc, rows),
		Ledger:      ledger,
		Now:         time.Now().UTC(),
	})
	run := intentAuditRun{Service: svc, Corpus: corpus, Audit: audit, Ledger: ledger, LedgerIgnored: ledgerIgnored}
	if o.ReadOnly {
		return run, nil
	}

	// Law 6: the clock starts the first time the audit runs with a consumer
	// population present, and never moves after.
	population := intentPopulation(abs)
	if len(population) > 1 {
		_ = intent.StartClock(abs, head, population)
	}
	// K1's counterfactual (EP-24), recorded ONCE: the pre-plane rate of
	// answering a decision. A kill clock with nothing to be faster than is
	// a clock measuring itself (law 12).
	bDays, bMedian, bNote := intentBaseline(abs, corpus.DecisionsHome, time.Now(), 90)
	_ = intent.RecordBaseline(abs, head, time.Now().UTC().Format(time.RFC3339), bDays, bMedian, bNote)
	_ = intent.AppendLedger(intent.LedgerRow{
		Kind: intent.KindDebt, RepoRoot: abs, Commit: head,
		ByKind: audit.ByKind, Annotated: audit.Annotated, MS: audit.Elapsed.Milliseconds(),
	})
	// ADR-027 kill criterion: the discipline is theatre if no manifest is
	// ever incomplete in a real run. Counted on the EXISTING beat — no new
	// channel, no new file (law 4).
	incomplete := "0"
	if !audit.Coverage.Complete() {
		incomplete = itoa(len(audit.Coverage.Unreadable()))
	}
	emitBeat("intent-cli", "audit", map[string]string{
		"conditions":          itoa(len(corpus.Conditions)),
		"unproven":            itoa(len(audit.Unproven)),
		"incomplete_coverage": incomplete,
	})
	return run, nil
}

// fileLineRe matches a "path:line" citation (with an optional trailing
// " → target" or other suffix ignored) as the audit and the custodian
// write them.
var fileLineRe = regexp.MustCompile(`^([^\s:]+):(\d+)\b`)

// sarifLocation turns an evidence/ref citation into a repo-relative file
// and line. A citation without a line locates the file at line 0 (SARIF
// omits the region); a plan ref ("docs/plans/x.md ROW") locates the plan.
func sarifLocation(root string, candidates ...string) (string, int) {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if m := fileLineRe.FindStringSubmatch(c); m != nil {
			n, _ := strconv.Atoi(m[2])
			return relToRoot(root, m[1]), n
		}
		if i := strings.Index(c, ".md"); i > 0 && !strings.ContainsAny(c[:i], " \t") {
			return relToRoot(root, c[:i+3]), 0
		}
	}
	return "", 0
}

// relToRoot renders p relative to root when it is absolute and inside it;
// repo-relative citations pass through unchanged.
func relToRoot(root, p string) string {
	if filepath.IsAbs(p) {
		if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(p)
}

// intentAuditSARIF renders the unproven claims as SARIF results: rule id =
// the claim kind, message = ref, detail and remedy, location from the
// claim's evidence or ref, falling back to the router file (line 0) so a
// claim is never dropped for lacking a citation. Uncomputable sources
// become "note"-level results on the router: a code-scanning pane must
// show blindness, not hide it (ADR-027).
func intentAuditSARIF(root string, c intent.Corpus, a intent.Audit) []verify.SARIFResult {
	router := relToRoot(root, c.RouterPath)
	if router == "" {
		router = "docs/intent/README.md"
	}
	var out []verify.SARIFResult
	for _, cl := range a.Unproven {
		file, line := sarifLocation(root, cl.Evidence, cl.Ref)
		if file == "" {
			file = router
		}
		out = append(out, verify.SARIFResult{
			RuleID: cl.Kind, Level: "warning",
			Message: fmt.Sprintf("%s — %s → %s", cl.Ref, cl.Detail, cl.Remedy),
			File:    file, Line: line,
		})
	}
	for _, u := range a.Uncomputable {
		out = append(out, verify.SARIFResult{RuleID: "not-computed", Level: "note", Message: u, File: router})
	}
	return out
}

func renderIntentAudit(w io.Writer, c intent.Corpus, a intent.Audit, ledger []intent.LedgerRow, kinds []string, top int, now time.Time) {
	want := func(k string) bool {
		if len(kinds) == 0 || (len(kinds) == 1 && kinds[0] == "") {
			return true
		}
		for _, s := range kinds {
			if strings.EqualFold(strings.TrimSpace(s), k) {
				return true
			}
		}
		return false
	}

	fmt.Fprintf(w, "intent corpus for %s — tier %s", c.Root, c.Tier)
	if c.RouterPath != "" {
		fmt.Fprintf(w, " (%s)", c.RouterPath)
	}
	fmt.Fprintln(w)
	if c.DecisionsHome != "" {
		fmt.Fprintf(w, "  decisions: %s\n", c.DecisionsHome)
	} else {
		fmt.Fprintln(w, "  decisions: NO HOME RESOLVED — the router names none and no conventional home exists")
	}
	if row, ok := intent.Clock(ledger, c.Root); ok {
		fmt.Fprintf(w, "  clock: started %s with %d repo(s) in the population\n", row.At, len(row.Population))
	} else {
		fmt.Fprintln(w, "  clock: not started (no sibling consumer repo seen yet — kill clocks are not running)")
	}
	// GL-14 (design doc §6): every promotion/retirement applyLearning made,
	// announced HERE with its signals — the same ledger rows
	// learningAttentionFragment counts, never re-derived.
	if learning := intent.RecentLearning(ledger, c.Root, now.Add(-intentAuditLearningWindow)); len(learning) > 0 {
		fmt.Fprintf(w, "  learning: %d promotion/retirement(s) in the last %d days\n",
			len(learning), int(intentAuditLearningWindow.Hours()/24))
		for _, r := range learning {
			fmt.Fprintf(w, "    %s → %s — %s (%s)\n", r.ID, r.Verdict, r.Evidence, r.At)
		}
	}
	for _, n := range c.NestedRouters {
		fmt.Fprintf(w, "  NOTE: nested router %s found and NOT audited (v1 audits the repo root only)\n", n)
	}
	for _, u := range c.Unparsed {
		fmt.Fprintf(w, "  UNPARSED %s:%d — %s\n", u.File, u.Line, u.Reason)
	}

	if want("conditions") {
		annotated, terminal := 0, 0
		for _, cond := range c.Conditions {
			if cond.Annotated() {
				annotated++
			}
			if cond.Terminal {
				terminal++
			}
		}
		fmt.Fprintf(w, "\nconditions: %d (%d annotated, %d terminal, %d human-judged)\n",
			len(c.Conditions), annotated, terminal, len(c.Conditions)-annotated-terminal)
		if annotated == 0 {
			fmt.Fprintln(w, "  every condition is human-judged — no `{oracle: …}` annotation exists yet, so nothing is evaluated")
		}
	}

	if want("triggers") {
		renderIntentTriggers(w, a.Triggers)
	}

	if want("invariants") {
		byRung := map[intent.Rung]int{}
		for _, inv := range c.Invariants {
			byRung[inv.Rung]++
		}
		fmt.Fprintf(w, "\ninvariants: %d — %s\n", len(c.Invariants), renderRungs(byRung))
		if len(a.Unenforced) > 0 {
			fmt.Fprintf(w, "  unenforced (honest, not debt): %s\n", joinInvIDs(a.Unenforced))
		}
	}

	if want("unproven") {
		fmt.Fprintf(w, "\nunproven claims: %s\n", readcoverage.RenderCount(a.Coverage, len(a.Unproven), ""))
		shown := 0
		lastKind := ""
		for _, cl := range a.Unproven {
			if top > 0 && shown >= top {
				fmt.Fprintf(w, "  … %d more elided (--top 0 for all)\n", len(a.Unproven)-shown)
				break
			}
			if cl.Kind != lastKind {
				fmt.Fprintf(w, "  [%s]\n", cl.Kind)
				lastKind = cl.Kind
			}
			fmt.Fprintf(w, "    %s — %s\n      → %s\n", cl.Ref, cl.Detail, cl.Remedy)
			if cl.Evidence != "" {
				fmt.Fprintf(w, "         evidence: %s\n", cl.Evidence)
			}
			shown++
		}
		if len(a.Unproven) == 0 {
			fmt.Fprintln(w, "  none from the sources that could be computed")
		}
	}

	for _, u := range a.Uncomputable {
		fmt.Fprintf(w, "  NOT COMPUTED — %s\n", u)
	}
}

func renderRungs(by map[intent.Rung]int) string {
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, by[intent.Rung(k)]))
	}
	return strings.Join(parts, ", ")
}

func joinInvIDs(invs []intent.Invariant) string {
	ids := make([]string, 0, len(invs))
	for _, i := range invs {
		ids = append(ids, i.ID)
	}
	return strings.Join(ids, ", ")
}

func emitIntentJSON(w io.Writer, c intent.Corpus, a intent.Audit, ledger []intent.LedgerRow, ignored ledgerroots.Ignored) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(withLedgerIgnored(intentAuditPayload(c, a, ledger, time.Now().UTC()), ignored)); err != nil {
		return 1
	}
	return 0
}

// intentLedgerNoun names an intent-ledger row in an ignored phrase.
const intentLedgerNoun = "intent-ledger row"

// withLedgerIgnored names, in the audit's JSON, the intent-ledger rows the
// registered-roots rule left out (PE-25) — "" keys nothing.
func withLedgerIgnored(payload map[string]any, ignored ledgerroots.Ignored) map[string]any {
	if p := ignored.Phrase(intentLedgerNoun); p != "" {
		payload["ledger_ignored"] = p
	}
	return payload
}

// intentAuditPayload is `intent audit --json`'s document — the verb prints
// it and the intent tab carries it as the verb's section (P10).
func intentAuditPayload(c intent.Corpus, a intent.Audit, ledger []intent.LedgerRow, now time.Time) map[string]any {
	clock, hasClock := intent.Clock(ledger, c.Root)
	learning := intent.RecentLearning(ledger, c.Root, now.Add(-intentAuditLearningWindow))
	payload := map[string]any{
		"root":           c.Root,
		"tier":           c.Tier,
		"router":         c.RouterPath,
		"decisions_home": c.DecisionsHome,
		"nested_routers": c.NestedRouters,
		"conditions":     len(c.Conditions),
		"annotated":      a.Annotated,
		"invariants":     len(c.Invariants),
		"surfaces":       len(c.Surfaces),
		"unparsed":       c.Unparsed,
		"unproven":       a.Unproven,
		"by_kind":        a.ByKind,
		// ADR-027: a machine reader must be able to tell a real zero from a
		// zero the reader was blind to. `complete: false` means every count
		// in this document is a lower bound.
		"coverage": map[string]any{
			"complete":   a.Coverage.Complete(),
			"note":       readcoverage.Note(a.Coverage),
			"unreadable": a.Coverage.Unreadable(),
			"entries":    len(a.Coverage.Entries),
		},
		"unenforced":   joinInvIDs(a.Unenforced),
		"uncomputable": a.Uncomputable,
		"ms":           a.Elapsed.Milliseconds(),
		// CX-21: every revisit/promote condition's verdict — computable ones
		// read at this run, prose ones unevaluated.
		"triggers": a.Triggers,
	}
	if hasClock {
		payload["clock_started_at"] = clock.At
		payload["population"] = clock.Population
	}
	if len(learning) > 0 {
		// GL-14: the SAME ledger rows the text report and the attention
		// line's count read — never a second source.
		payload["learning"] = learning
	}
	return payload
}

// planRowStates reads the newest verify-ledger row per plan at HEAD. It
// never runs a verify: 45 plans is ~70 seconds, which no audit, doctor row
// or fleet column may spend. Only rows under roots' registered service
// roots count (PE-24); the note names the rows ignored.
func planRowStates(root string, roots ledgerroots.Roots) ([]intent.PlanRowState, string) {
	rows, ignored := readRegisteredVerifyLedger(roots)
	states, note := planRowStatesOf(root, rows)
	return states, withIgnored(note, ignored, verifyLedgerNoun)
}

// planRowStatesOf folds the (registered) rows for root.
func planRowStatesOf(root string, rows []verifyLedgerRow) ([]intent.PlanRowState, string) {
	if len(rows) == 0 {
		return nil, "no verify-gate run recorded (~/.fathom/verify-ledger.jsonl is empty or unreadable)"
	}
	head := gitHeadShort(root)
	newest := map[string]verifyLedgerRow{}
	for _, r := range rows {
		if r.RepoRoot != root {
			continue
		}
		if prev, ok := newest[r.Plan]; !ok || r.At > prev.At {
			newest[r.Plan] = r
		}
	}
	if len(newest) == 0 {
		return nil, "no verify-gate run recorded for this repo root"
	}
	plans := make([]string, 0, len(newest))
	for p := range newest {
		plans = append(plans, p)
	}
	sort.Strings(plans)

	var out []intent.PlanRowState
	stale := 0
	for _, p := range plans {
		r := newest[p]
		if head != "" && r.Commit != "" && r.Commit != head {
			stale++
		}
		for i := 0; i < r.Failing; i++ {
			out = append(out, intent.PlanRowState{
				Plan: p, ID: fmt.Sprintf("row %d of %d failing", i+1, r.Failing),
				Claimed: true, Failing: true, MeasuredAt: r.At,
			})
		}
		if unknown := r.Claimed - r.Verified - r.Failing; unknown > 0 {
			out = append(out, intent.PlanRowState{
				Plan: p, ID: fmt.Sprintf("%d claimed row(s) with no contract", unknown),
				Claimed: true, Unknown: true, MeasuredAt: r.At,
			})
		}
	}
	note := ""
	if stale > 0 {
		note = fmt.Sprintf("%d of %d plan readings predate HEAD", stale, len(plans))
	}
	// PC-5: rows whose reading came from a stale daemon leave U1/U2 and
	// are counted as coverage gaps by the audit (intent.auditPlanRows).
	fathomTree, _ := fathomSourceTree(root)
	if n, stalePlans := markMeasuredStale(out, newest, fathomTree); n > 0 {
		msg := fmt.Sprintf("%d row(s) in %d plan(s) measured-stale", n, len(stalePlans))
		if note != "" {
			note += "; " + msg
		} else {
			note = msg
		}
	}
	if len(out) == 0 && note != "" {
		return nil, note
	}
	return out, note
}

// intentPopulation is the computed consumer set: sibling repos beside this
// one that carry BOTH a router and a .fathom directory. Never a hand-kept
// list — law 8's precondition has to be measured, not asserted.
func intentPopulation(root string) []string {
	parent := filepath.Dir(root)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return []string{filepath.Base(root)}
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cand := filepath.Join(parent, e.Name())
		if _, err := os.Stat(filepath.Join(cand, "docs", "intent", "README.md")); err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(cand, ".fathom")); err != nil {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// planRowContracts reads every gated plan's row annotations statically —
// no index, no daemon, no verify run. U12 (EP-38) joins these against the
// router's sensitive surfaces.
func planRowContracts(root string) ([]intent.PlanRow, string, readmanifest.Manifest) {
	reads := readmanifest.NewSet()
	reads.RelativeTo(root)
	dir := filepath.Join(root, "docs", "plans")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Sprintf("docs/plans is unreadable (%v)", err), reads.Manifest()
	}
	var out []intent.PlanRow
	files := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "README.md" {
			// archive/ is out of gate scope, and so is the grammar doc —
			// a deliberate skip, accounted for rather than invisible.
			reads.Skip(filepath.Join(dir, name), "not a gated plan")
			continue
		}
		md, ok := reads.ReadFile(filepath.Join(dir, name))
		if !ok {
			continue // recorded on the manifest; U12 will render as a bound
		}
		files++
		rel := filepath.Join("docs", "plans", name)
		for _, t := range verify.ParseMarkdownPlan("", md).Tasks {
			if t.Skipped {
				continue // a refused or deferred row changes nothing
			}
			var reach []string
			if t.Expect.ReachableFrom != "" {
				reach = append(reach, t.Expect.ReachableFrom)
			}
			out = append(out, intent.PlanRow{
				Plan: rel, ID: t.ID, Package: t.Expect.Package,
				Symbol: t.Expect.Symbol, ReachableFrom: reach, Claimed: t.Claimed,
				SkipTest: t.Expect.SkipTest, SkipReason: t.Expect.SkipReason,
			})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Sprintf("%d plan file(s) read, no row carried a contract annotation", files), reads.Manifest()
	}
	return out, "", reads.Manifest()
}

// repoGuardNotes reads this repo's .fathom/guard.cue and returns each
// entry's note plus a lookup from note back to scopes. A missing file is a
// NAMED absence, not an error: most repos have no guard.cue, and U13 says so
// rather than reporting every sensitive surface as uncovered.
func repoGuardNotes(root string) ([]string, func(string) []string, string) {
	path := filepath.Join(root, ".fathom", "guard.cue")
	if _, err := os.Stat(path); err != nil {
		return nil, nil, "this repo has no .fathom/guard.cue (`fathom intent export --guard` proposes one)"
	}
	set, err := guardspec.LoadGuards(path)
	if err != nil {
		return nil, nil, fmt.Sprintf(".fathom/guard.cue did not load (%v) — `fathom doctor` carries that finding", err)
	}
	scopes := map[string][]string{}
	notes := make([]string, 0, len(set.Guards))
	for _, g := range set.Guards {
		if g.Note == "" {
			continue
		}
		notes = append(notes, g.Note)
		scopes[g.Note] = g.Scopes
	}
	sort.Strings(notes)
	return notes, func(n string) []string { return scopes[n] }, ""
}

// intentRunner runs `fathom intent` (registered below; EW-45).
type intentRunner struct{}

func (intentRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdIntent(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "intent", Group: "operate", Order: 10, Run: intentRunner{},
		Summary: "The intent corpus (ADR-026): audit — conditions, invariant rungs, unproven claims | doctrine — the vision promotion pipeline and goals as pre-registered bets | export --guard"})
}
