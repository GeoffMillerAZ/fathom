package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/readset"
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

func cmdIntent(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: intent requires a subcommand (audit, export)")
		return 1
	}
	switch args[0] {
	case "audit":
		return cmdIntentAudit(args[1:], stdout, stderr)
	case "export":
		return cmdIntentExport(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "error: unknown intent subcommand %q (audit, export)\n", args[0])
	return 1
}

func cmdIntentAudit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("intent audit", stderr)
	root := fs.String("root", ".", "repository root")
	service := fs.String("service", "", "service id (default: resolved from the repo)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	kind := fs.String("kind", "", "restrict to kinds: conditions|invariants|unproven (comma-separated)")
	top := fs.Int("top", intentTopDefault, "max rows per section before eliding (0 = all)")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	corpus, err := intent.Load(abs)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading the intent corpus: %v\n", err)
		return 1
	}

	svc := *service
	if svc == "" {
		// The daemon-backed deterministic resolver, exactly as every hook
		// uses it. The bare basename is NOT good enough: this repo's
		// directory is "sextant" while its indexed service is "fathom", and
		// resolving by basename is the O1 class that once zeroed obligations
		// on a third of edits.
		hc := &http.Client{Timeout: 5 * time.Second}
		if cand, ok := resolveIndexedCandidate(hc, *url, abs); ok {
			svc = cand.name
		} else if cands := serviceCandidates(abs); len(cands) > 0 {
			svc = cands[0].name
		}
	}

	rows, note := planRowStates(abs)
	guardNotes, guardScopes, guardNote2 := repoGuardNotes(abs)
	contracts, contractNote, contractCoverage := planRowContracts(abs)
	ledger := intent.ReadLedger()
	audit := intent.RunAudit(intent.AuditInput{
		Corpus:   corpus,
		Symbols:  newIndexOracle(*url, svc, abs, gitHeadShort(abs)),
		PlanRows: rows,
		PlanNote: note,

		PlanContracts:    contracts,
		PlanContractNote: contractNote,
		ExtraCoverage:    []readset.Manifest{contractCoverage},

		SurfaceFiles: func(globs []string) (int, bool) {
			return filesOnDiskUnder(abs, globs), true
		},

		GuardNotes:  guardNotes,
		GuardScopes: guardScopes,
		GuardNote:   guardNote2,
		Ledger:      ledger,
		Now:         time.Now().UTC(),
	})

	// Law 6: the clock starts the first time the audit runs with a consumer
	// population present, and never moves after.
	population := intentPopulation(abs)
	if len(population) > 1 {
		_ = intent.StartClock(abs, gitHeadShort(abs), population)
	}
	// K1's counterfactual (EP-24), recorded ONCE: the pre-plane rate of
	// answering a decision. A kill clock with nothing to be faster than is
	// a clock measuring itself (law 12).
	bDays, bMedian, bNote := intentBaseline(abs, corpus.DecisionsHome, time.Now(), 90)
	_ = intent.RecordBaseline(abs, gitHeadShort(abs), time.Now().UTC().Format(time.RFC3339), bDays, bMedian, bNote)
	_ = intent.AppendLedger(intent.LedgerRow{
		Kind: intent.KindDebt, RepoRoot: abs, Commit: gitHeadShort(abs),
		ByKind: audit.ByKind, Annotated: audit.Annotated, MS: audit.Elapsed.Milliseconds(),
	})
	// ADR-027 kill criterion: the discipline is theatre if no manifest is
	// ever incomplete in a real run. Counted on the EXISTING beat — no new
	// channel, no new file (law 4).
	incomplete := "0"
	if !audit.Coverage.Complete() {
		incomplete = itoa(len(audit.Coverage.UnreadableEntries()))
	}
	emitBeat("intent-cli", "audit", map[string]string{
		"conditions":          itoa(len(corpus.Conditions)),
		"unproven":            itoa(len(audit.Unproven)),
		"incomplete_coverage": incomplete,
	})

	if *asJSON {
		return emitIntentJSON(stdout, corpus, audit, ledger)
	}
	renderIntentAudit(stdout, corpus, audit, ledger, strings.Split(*kind, ","), *top)
	return 0
}

func renderIntentAudit(w io.Writer, c intent.Corpus, a intent.Audit, ledger []intent.LedgerRow, kinds []string, top int) {
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
		fmt.Fprintf(w, "\nunproven claims: %s\n", a.Coverage.RenderCount(len(a.Unproven), ""))
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

func emitIntentJSON(w io.Writer, c intent.Corpus, a intent.Audit, ledger []intent.LedgerRow) int {
	clock, hasClock := intent.Clock(ledger, c.Root)
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
			"note":       a.Coverage.Note(),
			"unreadable": a.Coverage.UnreadableEntries(),
			"entries":    len(a.Coverage.Entries),
		},
		"unenforced":   joinInvIDs(a.Unenforced),
		"uncomputable": a.Uncomputable,
		"ms":           a.Elapsed.Milliseconds(),
	}
	if hasClock {
		payload["clock_started_at"] = clock.At
		payload["population"] = clock.Population
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return 1
	}
	return 0
}

// planRowStates reads the newest verify-ledger row per plan at HEAD. It
// never runs a verify: 45 plans is ~70 seconds, which no audit, doctor row
// or fleet column may spend.
func planRowStates(root string) ([]intent.PlanRowState, string) {
	rows := readVerifyLedger()
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
func planRowContracts(root string) ([]intent.PlanRow, string, readset.Manifest) {
	reads := &readset.Set{Root: root}
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
