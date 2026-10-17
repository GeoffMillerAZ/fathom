package main

// console_knowledge_test.go — the Knowledge pages (/layers,
// /layers/{layer}, /claims/{id}, /packs, /explain) on the fixture fleet,
// and the verbs they render: the fixture store's governed claims, the
// daemon record the Knowledge verbs resolve their store through, and the
// pages' structure, truth states, links and verbs.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// consoleFixtureClaimID is the fixture's fresh, cited, re-anchored claim.
const consoleFixtureClaimID = "kc-payment-wrap"

// seedConsoleFixtureKnowledge gives the fixture fleet governed knowledge:
// payment-service compiled obligations (a budget-stopped run with rejection
// reasons, one statement minted twice, a stale exact-file rule, a
// superseded one) and adr-rules; checkout-service compiled obligations into
// no current claim; ledger-service compiled nothing.
func seedConsoleFixtureKnowledge(path string) error {
	st, err := store.Open(path)
	if err != nil {
		return err
	}
	defer st.Close()
	ctx := context.Background()
	t0 := time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
	type seed struct {
		svc, id, layer, rule, cite, anchor string
		scope                              []string
		status                             core.ClaimStatus
	}
	for _, s := range []seed{
		{"payment-service", consoleFixtureClaimID, "obligations", "Wrap every charge error with the payment context.", "payment.go#L1-L2", "", []string{"**/*.go"}, core.ClaimFresh},
		{"payment-service", "kc-payment-wrap-2", "obligations", "Wrap every charge error with the `payment` context", "payment.go#L1-L2", "", []string{"**/*.go"}, core.ClaimFresh},
		{"payment-service", "kc-payment-idem", "obligations", "Keep refunds idempotent.", "errors.go#L1-L2", "// refunds are retried\n// never charge twice", []string{"payment.go"}, core.ClaimStale},
		{"payment-service", "kc-payment-old", "obligations", "A rule a recompile replaced.", "payment.go#L6-L6", "", []string{"**/*.go"}, core.ClaimSuperseded},
		{"payment-service", "kc-payment-adr", "adr-rules", "Record a decision for every new dependency.", "payment.go#L4-L4", "", []string{"**"}, core.ClaimFresh},
		{"checkout-service", "kc-checkout-old", "obligations", "Retired checkout rule.", "checkout.go#L1-L1", "", []string{"**/*.go"}, core.ClaimSuperseded},
	} {
		cl := core.SemanticClaim{ClaimID: s.id, ServiceID: s.svc, Layer: s.layer, LayerVersion: "v1", Kind: "obligation",
			Subject: "CLAUDE.md#" + s.id, Rule: s.rule, Scope: s.scope, Tier: core.TierDocClaimed, TierProfile: "engineering",
			Status: s.status, Model: "fixture-model", PromptVersion: s.layer + "/v1", CompiledAtWatermark: "fixture-watermark",
			CompiledAt: t0, ValidFrom: t0}
		links := []core.ClaimLink{{ClaimID: s.id, Rel: core.RelCites, TargetRef: s.cite, TargetHash: "h", AnchorText: s.anchor}}
		for _, g := range s.scope {
			links = append(links, core.ClaimLink{ClaimID: s.id, Rel: core.RelScopes, TargetRef: g})
		}
		if err := st.PutClaims(ctx, []core.SemanticClaim{cl}, links); err != nil {
			return err
		}
	}
	for _, r := range []core.CompileRun{
		{RunID: "kr-payment-ob", ServiceID: "payment-service", Layer: "obligations", Model: "fixture-model", CostUSD: 0.12, TokensIn: 900, TokensOut: 150,
			SectionsTotal: 5, SectionsCompiled: 4, ClaimsEmitted: 4, ClaimsRejected: 1, BudgetStopped: true, RanAt: t0,
			UncompiledScopes: []string{"docs/CLAUDE.md"},
			RejectionReasons: []core.RejectionBreakdown{{Cause: "anchor-missing", Count: 1, Samples: []string{"cited L40 does not exist"}}}},
		{RunID: "kr-payment-adr", ServiceID: "payment-service", Layer: "adr-rules", Model: "fixture-model", CostUSD: 0.05, SectionsTotal: 1, SectionsCompiled: 1, ClaimsEmitted: 1, RanAt: t0.Add(time.Minute)},
		{RunID: "kr-payment-adr-refused", ServiceID: "payment-service", Layer: "adr-rules", Refused: true, CostUSD: 3.5, RanAt: t0.Add(2 * time.Minute), RefusalReason: "token total exceeds layer budget"},
		{RunID: "kr-checkout-ob", ServiceID: "checkout-service", Layer: "obligations", Model: "fixture-model", CostUSD: 0.02, SectionsTotal: 1, SectionsCompiled: 1, RanAt: t0.Add(time.Hour)},
	} {
		if err := st.RecordCompileRun(ctx, r); err != nil {
			return err
		}
	}
	for _, e := range []core.ClaimEvent{
		{ClaimID: consoleFixtureClaimID, Verdict: "re-anchored", Author: "sweep", Note: "payment.go#L2-L3 -> payment.go#L1-L2", At: t0.Add(time.Hour)},
		{ClaimID: consoleFixtureClaimID, Verdict: "corroborated", Author: "reviewer", Note: "still binding", At: t0.Add(2 * time.Hour)},
	} {
		if err := st.AppendClaimEvent(ctx, e); err != nil {
			return err
		}
	}
	return nil
}

// recordConsoleFixtureDaemon writes the daemon record serve-http writes
// once its listener is bound, naming the fixture's store and call log.
func recordConsoleFixtureDaemon(t *testing.T, f *consoleFixture) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	addr := strings.TrimPrefix(f.ts.URL, "http://")
	p, err := console.DaemonRecordPath(home, addr)
	if err != nil {
		t.Fatal(err)
	}
	if err := console.WriteDaemonRecord(p, console.NewDaemonRecord(addr, os.Getpid(), f.callLog, f.store, "fixture", f.now)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(p) })
}

// runVerb runs a fathom command against the fixture daemon's store.
func runVerb(t *testing.T, f *consoleFixture, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(append(args, "--url", f.ts.URL), &out, &errb)
	return out.String(), errb.String(), code
}

// consoleRoutes matches a console href against the registered routes.
func routeRegistered(href string) bool {
	path := href
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	for _, p := range defaultConsoleRegistry().pages {
		pat := "^" + regexp.MustCompile(`\\\{[a-z]+\\\}`).ReplaceAllString(regexp.QuoteMeta(p.Route), `[^/]+`) + "$"
		if p.Route == "/{$}" {
			pat = "^/$"
		}
		if regexp.MustCompile(pat).MatchString(path) {
			return true
		}
	}
	return false
}

// TestConsoleKnowledgePages_OnShellWithVerbsAndTruthStates renders every
// Knowledge page on the fixture fleet: one h1, the verbs it renders as
// copyable chips, role utilities only, every internal link a registered
// route, and the truth states drawn — a cell nothing compiled is hatched
// "not compiled" and never 0; an unmeasured precision says so.
func TestConsoleKnowledgePages_OnShellWithVerbsAndTruthStates(t *testing.T) {
	f := newConsoleFixture(t)
	hrefRe := regexp.MustCompile(`href="(/[^"]*)"`)
	for _, tc := range []struct {
		path  string
		verbs []string
		want  []string
	}{
		{"/layers", []string{verbLayersStrata, verbLayersCoverage}, []string{
			`data-stratum="dialect"`, `data-stratum="provider"`, `data-stratum="platform"`, `data-stratum="concern"`,
			`data-layer-block="obligations"`, `data-coverage-heatmap`, "not compiled", `stroke-dasharray="2 2"`,
			"The coverage matrix as a table", "The strata as a table", "re-anchored", "refused before starting",
		}},
		{"/layers/obligations", []string{"fathom layers show obligations --json"}, []string{
			`id="project-payment-service"`, "anchor-missing", "cited L40 does not exist", "docs/CLAUDE.md",
			"fathom compile --layer obligations --service payment-service --store " + consoleFixtureStorePath(t) + " --continue --dry-run=false", "budget stopped",
		}},
		// The fixture's faux roots are not on disk: a fresh claim's excerpt
		// is named absent, never an empty viewer.
		{"/claims/" + consoleFixtureClaimID, []string{"fathom claim show " + consoleFixtureClaimID + " --json"}, []string{
			"Wrap every charge error with the payment context.", "not measured — the cited file payment.go no longer exists", "re-anchored", "corroborated", "reviewer",
		}},
		// A stale claim shows its compile-time text through the kit's
		// excerpt viewer, labeled as such (J2).
		{"/claims/kc-payment-idem", []string{"fathom claim show kc-payment-idem --json"}, []string{
			`id="cite-L1"`, `id="cite-L2"`, "Cited", "compile-time anchor text", "never charge twice", "Served flagged stale",
		}},
		{"/packs", []string{verbReportPacks}, []string{`data-table="dialect-matrix"`, "unmeasured", "go/parser + go/types", "Published readings", `data-table="layer-packs"`}},
		{"/explain?svc=payment-service&file=payment.go", []string{"fathom obligations --service payment-service --file payment.go --layer rules --explain --json"}, []string{
			`data-scope-tier="exact_file"`, `data-scope-tier="root"`, `data-outcome="duplicate"`, `data-outcome="served"`,
			// The fixture's rules cite a CLAUDE.md its corpus lacks, so the
			// agent is served none of them and the page says so (IV-7,
			// O1149); the guided store shows them served
			// (TestObligationsVerb_DefaultIsWhatServingServes).
			"What the agent saw at the edit", "Keep refunds idempotent.", "compiled rules withheld",
		}},
	} {
		code, _, body := f.get(t, tc.path)
		if code != http.StatusOK {
			t.Fatalf("%s = %d:\n%.800s", tc.path, code, body)
		}
		if n := strings.Count(body, "<h1"); n != 1 {
			t.Errorf("%s: %d h1 elements", tc.path, n)
		}
		for _, v := range tc.verbs {
			if !strings.Contains(body, `data-console-copy="`+htmlEscapeTest(v)+`"`) {
				t.Errorf("%s: no copyable chip for %q", tc.path, v)
			}
		}
		for _, w := range tc.want {
			if !strings.Contains(body, w) {
				t.Errorf("%s: missing %q", tc.path, w)
			}
		}
		assertRoleUtilitiesOnly(t, body)
		for _, m := range hrefRe.FindAllStringSubmatch(body, -1) {
			href := strings.ReplaceAll(m[1], "&amp;", "&")
			if strings.HasPrefix(href, "/assets/") || href == "/search" {
				continue
			}
			if !routeRegistered(href) {
				t.Errorf("%s links to %s, which no page serves", tc.path, href)
			}
		}
	}

	// Absent is never 0: the coverage section's not-compiled cell carries
	// no number, and the page draws it hatched, never on the ramp.
	env := f.pageJSON(t, "/layers")
	var cov console.CoverageMatrix
	if err := json.Unmarshal(env.Sections[verbLayersCoverage], &cov); err != nil {
		t.Fatal(err)
	}
	states := map[string]console.State{}
	for _, r := range cov.Rows {
		for _, c := range r.Cells {
			states[r.Service+"/"+c.Layer] = c.Claims.State
		}
	}
	if states["payment-service/obligations"] != console.StatePresent || states["checkout-service/obligations"] != console.StateZero || states["ledger-service/obligations"] != console.StateAbsent {
		t.Errorf("coverage states = %v", states)
	}
	if bytes.Contains(env.Sections[verbLayersCoverage], []byte(`"state":"absent","v"`)) {
		t.Error("an absent cell carries a number")
	}
	_, _, layers := f.get(t, "/layers")
	// The kit's label gutter is fixed (docs/app-kit.md, FATHOM-D-KIT-01):
	// long names are shortened there and whole in the table alternative.
	// proof-service's live certificate (console wave C's fixture) adds the
	// certifications column.
	if !strings.Contains(layers, "ledger-ser…, oblig: missing — not compiled") || !strings.Contains(layers, `data-table="coverage"`) ||
		!strings.Contains(layers, "columns: adr = adr-rules · certi = certifications · oblig = obligations") {
		t.Error("the heatmap must name a not-compiled cell as missing in its title, beside the table alternative")
	}

	// A claim that is not there, a layer that is not there, a project that
	// is not there and a filter that is not one are errors on the shell.
	for path, status := range map[string]int{
		"/claims/no-such-claim":                  http.StatusNotFound,
		"/layers/no-such-layer":                  http.StatusNotFound,
		"/explain?svc=no-such-project":           http.StatusNotFound,
		"/explain?svc=payment-service&layer=bad": http.StatusBadRequest,
	} {
		code, _, body := f.get(t, path)
		if code != status || !strings.Contains(body, `<main id="observatory-main"`) {
			t.Errorf("%s = %d, want %d on the shell:\n%.400s", path, code, status, body)
		}
	}
	// With no project chosen the explainer is the picker, and names no verb.
	env = f.pageJSON(t, "/explain")
	if len(env.Verbs) != 0 || len(env.Sections) != 0 {
		t.Errorf("the empty explainer renders no verb: %+v", env)
	}
	_, _, picker := f.get(t, "/explain")
	if !strings.Contains(picker, `data-explain-form`) || !strings.Contains(picker, `<option value="payment-service"`) {
		t.Error("the explainer's picker must list the store's projects")
	}
}

// TestObligationsVerb_DefaultIsWhatServingServes pins OC-32's "the default
// output is unchanged": `fathom obligations` without --explain prints the
// rules the MCP obligations tool serves for the same file, in the same
// order — the explanation adds candidates around that list, never a
// different one — and the duplicate statement is served once (OC-33).
func TestObligationsVerb_DefaultIsWhatServingServes(t *testing.T) {
	f := newConsoleFixtureOpts(t, consoleFixtureOpts{Guided: true})
	out, errb, code := runVerb(t, f, "obligations", "--service", "payment-service", "--file", "payment.go", "--json")
	if code != 0 {
		t.Fatalf("obligations: %d %s", code, errb)
	}
	var served struct {
		Rules []struct {
			Rule string `json:"rule"`
		} `json:"rules"`
		Deduped int `json:"deduped"`
	}
	if err := json.Unmarshal([]byte(out), &served); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	raw, err := callMCPTool(&http.Client{Timeout: 10 * time.Second}, f.ts.URL, "obligations", map[string]any{"service": "payment-service", "file": "payment.go"})
	if err != nil {
		t.Fatal(err)
	}
	var tool struct {
		Rules []struct {
			Rule string `json:"rule"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(raw, &tool); err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	var a, b []string
	for _, r := range served.Rules {
		a = append(a, r.Rule)
	}
	for _, r := range tool.Rules {
		b = append(b, r.Rule)
	}
	// The guided fixture's corpus holds the CLAUDE.md the rules cite
	// (consoleFixtureGuide), so both sides serve them: the list is equal,
	// row for row, and not empty.
	if len(a) == 0 || strings.Join(a, "|") != strings.Join(b, "|") {
		t.Errorf("the verb serves %q, the obligations tool %q", a, b)
	}
	// /explain draws the same value: the served stale rule is flagged.
	if _, _, page := f.get(t, "/explain?svc=payment-service&file=payment.go"); !strings.Contains(page, "Keep refunds idempotent.") || !strings.Contains(page, "served flagged stale") {
		t.Error("/explain must show the served stale rule flagged")
	}
	wraps := 0
	for _, r := range a {
		if strings.HasPrefix(r, "Wrap every charge error") {
			wraps++
		}
	}
	if wraps != 1 || served.Deduped != 1 {
		t.Errorf("the statement minted twice is served %d times (deduped %d), want once", wraps, served.Deduped)
	}
	// The text form names the served list and, with --explain, each
	// displaced rule's reason.
	text, _, code := runVerb(t, f, "obligations", "--service", "payment-service", "--file", "payment.go", "--explain")
	if code != 0 || !strings.Contains(text, "EXACT FILE") || !strings.Contains(text, "duplicate: the same statement") {
		t.Errorf("explain text = %d:\n%s", code, text)
	}
	// Unknown service and missing flags are refused.
	if _, errb, code := runVerb(t, f, "obligations", "--service", "nope"); code == 0 || !strings.Contains(errb, "not in the store") {
		t.Errorf("unknown service = %d %s", code, errb)
	}
	if _, errb, code := runVerb(t, f, "obligations"); code == 0 || !strings.Contains(errb, "--service is required") {
		t.Errorf("no service = %d %s", code, errb)
	}
}

// TestObligationsVerb_WithholdsWhatServingWithholds is O1149 (IV-7, I38):
// when the file the rules were compiled from is gone, the obligations tool
// withholds each rule and counts it in layer_coverage.stale_withheld; the
// verb withholds the same rules through the same predicate and counts them
// in its own stale_withheld, never listing a rule the agent is not served.
func TestObligationsVerb_WithholdsWhatServingWithholds(t *testing.T) {
	// The default fixture's faux root resolves nowhere, so the CLAUDE.md
	// every seeded rule cites is not in the corpus: the source is gone.
	f := newConsoleFixture(t)
	out, errb, code := runVerb(t, f, "obligations", "--service", "payment-service", "--file", "payment.go", "--json")
	if code != 0 {
		t.Fatalf("obligations: %d %s", code, errb)
	}
	var verb struct {
		Rules []struct {
			Rule string `json:"rule"`
		} `json:"rules"`
		StaleWithheld int `json:"stale_withheld"`
	}
	if err := json.Unmarshal([]byte(out), &verb); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	raw, err := callMCPTool(&http.Client{Timeout: 10 * time.Second}, f.ts.URL, "obligations", map[string]any{"service": "payment-service", "file": "payment.go"})
	if err != nil {
		t.Fatal(err)
	}
	var tool struct {
		Rules []struct {
			Rule string `json:"rule"`
		} `json:"rules"`
		LayerCoverage struct {
			StaleWithheld int `json:"stale_withheld"`
		} `json:"layer_coverage"`
	}
	if err := json.Unmarshal(raw, &tool); err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	var a, b []string
	for _, r := range verb.Rules {
		a = append(a, r.Rule)
	}
	for _, r := range tool.Rules {
		b = append(b, r.Rule)
	}
	if strings.Join(a, "|") != strings.Join(b, "|") {
		t.Errorf("the verb serves %q, the obligations tool %q", a, b)
	}
	if tool.LayerCoverage.StaleWithheld == 0 || verb.StaleWithheld != tool.LayerCoverage.StaleWithheld {
		t.Errorf("stale_withheld: verb %d, tool %d; want the same nonzero count", verb.StaleWithheld, tool.LayerCoverage.StaleWithheld)
	}
	// The text form names the withhold rather than "nothing compiled".
	text, _, code := runVerb(t, f, "obligations", "--service", "payment-service", "--file", "payment.go")
	if code != 0 || !strings.Contains(text, "withheld") || strings.Contains(text, "nothing compiled governs this path") {
		t.Errorf("text = %d:\n%s", code, text)
	}
}

// TestKnowledgeVerbs_ReadTheDaemonsStoreReadOnly: the Knowledge verbs read
// the store the daemon at --url recorded, name the rule that chose it, take
// --store over it, refuse a missing store instead of creating an empty one,
// and never write.
func TestKnowledgeVerbs_ReadTheDaemonsStoreReadOnly(t *testing.T) {
	f := newConsoleFixture(t)
	path := consoleFixtureStorePath(t)
	ks := resolveKnowledgeStore("", f.ts.URL)
	if ks.Path != path || ks.Rule != "daemon" {
		t.Errorf("resolved = %+v, want the daemon's recorded store", ks)
	}
	if ks := resolveKnowledgeStore("/x.sqlite", f.ts.URL); ks.Rule != "flag" {
		t.Errorf("--store must win: %+v", ks)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"layers", "strata"}, {"layers", "coverage"}, {"layers", "show", "obligations", "--json"},
		{"claim", "show", consoleFixtureClaimID}, {"report", "packs"},
	} {
		out, errb, code := runVerb(t, f, args...)
		if code != 0 {
			t.Errorf("%v: %d %s", args, code, errb)
			continue
		}
		if !strings.Contains(args[len(args)-1], "json") && !strings.Contains(out, "rule daemon") && args[0] != "claim" {
			t.Errorf("%v: text output must name the store and its rule:\n%s", args, out)
		}
	}
	after, _ := os.Stat(path)
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Error("a Knowledge verb wrote to the store")
	}
	missing := filepath.Join(t.TempDir(), "absent.sqlite")
	var out, errb bytes.Buffer
	if code := run([]string{"layers", "strata", "--store", missing}, &out, &errb); code == 0 || !strings.Contains(errb.String(), "no store at") {
		t.Errorf("a missing store = %d %s", code, errb.String())
	}
	if _, err := os.Stat(missing); err == nil {
		t.Error("a missing store must not be created")
	}
	if _, errb, code := runVerb(t, f, "layers", "show", "no-such-layer", "--json"); code == 0 || !strings.Contains(errb, "no layer") {
		t.Errorf("unknown layer = %d %s", code, errb)
	}
}

// verbsFillTemplates reports whether rendered are the page's declared verbs
// filled for one request. A page with no parameters renders exactly its
// registry verbs. Otherwise each rendered verb fills the next matching
// declared template, in order: a "{name}" placeholder matches the value
// (quoted by fillVerb when it holds a space), a bracketed group
// ("[--layer {layer}]") is present or absent, no placeholder is left
// unfilled, and a verb whose input the request lacks (a root not on this
// machine) may be left out, never rendered half-filled.
func verbsFillTemplates(rendered, registry []string) bool {
	templated := false
	for _, r := range registry {
		templated = templated || strings.Contains(r, "{")
	}
	if !templated {
		return strings.Join(rendered, "\n") == strings.Join(registry, "\n")
	}
	matches := func(v, tmpl string) bool {
		for _, t := range expandOptionalGroups(tmpl) {
			if templateRe(t).MatchString(v) {
				return true
			}
		}
		return false
	}
	j := 0
	for _, v := range rendered {
		if unfilledPlaceholderRe.MatchString(v) {
			return false // a placeholder the page left unfilled
		}
		for j < len(registry) && !matches(v, registry[j]) {
			j++
		}
		if j == len(registry) {
			return false
		}
		j++
	}
	return true
}

var unfilledPlaceholderRe = regexp.MustCompile(`\{[a-z]+\}`)

// templateRe matches a filled template: each "{name}" placeholder is one
// or more characters (a quoted value may hold spaces).
func templateRe(t string) *regexp.Regexp {
	q := regexp.QuoteMeta(t)
	q = regexp.MustCompile(`\\\{[a-z]+\\\}`).ReplaceAllString(q, `.+?`)
	return regexp.MustCompile("^" + q + "$")
}

// expandOptionalGroups expands a verb template's bracketed groups into
// every combination of present and absent.
func expandOptionalGroups(tmpl string) []string {
	out := [][]string{{}}
	toks := strings.Fields(tmpl)
	for i := 0; i < len(toks); i++ {
		if !strings.HasPrefix(toks[i], "[") {
			for j := range out {
				out[j] = append(out[j], toks[i])
			}
			continue
		}
		var group []string
		for ; i < len(toks); i++ {
			group = append(group, strings.Trim(toks[i], "[]"))
			if strings.HasSuffix(toks[i], "]") {
				break
			}
		}
		n := len(out)
		for j := 0; j < n; j++ {
			with := append(append([]string{}, out[j]...), group...)
			out = append(out, with)
		}
	}
	res := make([]string, 0, len(out))
	for _, o := range out {
		res = append(res, strings.Join(o, " "))
	}
	return res
}

// assertKnowledgeMetricsAgree extends OC-9 to the Knowledge pages: /layers
// and /packs show the fleet's current claims through one projection, and
// the coverage matrix's rows — the store's own project list — number the
// registry's services (a cross-source check, as statusz's is).
func assertKnowledgeMetricsAgree(t *testing.T, f *consoleFixture, services, claims console.Value[int]) {
	t.Helper()
	// Dedupe is a serving decision: at rest the store holds both copies of
	// the statement minted twice, so 3 obligations and 1 adr-rule — and
	// proof-service's live certificate (console wave C's fixture), 5.
	if claims.State != console.StatePresent || claims.V != 5 {
		t.Errorf("claims.current = %+v, want the fixture's 5 current claims", claims)
	}
	env := f.pageJSON(t, "/layers")
	var cov console.CoverageMatrix
	if err := json.Unmarshal(env.Sections[verbLayersCoverage], &cov); err != nil {
		t.Fatal(err)
	}
	if len(cov.Rows) != services.V {
		t.Errorf("the coverage matrix has %d projects, the registry %d services", len(cov.Rows), services.V)
	}
	var strata console.StrataView
	if err := json.Unmarshal(env.Sections[verbLayersStrata], &strata); err != nil {
		t.Fatal(err)
	}
	if strata.Totals.Claims.V != claims.V || strata.Totals.Projects != services.V {
		t.Errorf("strata totals %+v disagree with claims %d / services %d", strata.Totals, claims.V, services.V)
	}
	// The HTML agrees with the JSON: the stat band's tile shows the metric.
	_, _, page := f.get(t, "/layers")
	if got := tileValue(t, page, "current claims"); got != fmtThousands(claims.V) {
		t.Errorf("/layers tile shows %q current claims, the metric says %d", got, claims.V)
	}
	// A count and its rows agree: the claim list behind the metric holds
	// exactly the claims it counts, fleet-wide and per layer.
	list := f.pageJSON(t, "/claims")
	var l console.ClaimList
	if err := json.Unmarshal(list.Sections["fathom claim list --status current --json"], &l); err != nil {
		t.Fatal(err)
	}
	if l.Total.V != claims.V || l.Total.State != claims.State {
		t.Errorf("/claims lists %+v current claims, the metric counts %+v", l.Total, claims)
	}
	for _, band := range strata.Bands {
		for _, b := range band.Layers {
			if !b.Claims.Known() {
				continue
			}
			env := f.pageJSON(t, claimsHref(b.Name, "", ""))
			var bl console.ClaimList
			if err := json.Unmarshal(env.Sections[env.Verbs[0]], &bl); err != nil {
				t.Fatal(err)
			}
			if bl.Total.V != b.Claims.V {
				t.Errorf("layer %s counts %d current claims, its list holds %d", b.Name, b.Claims.V, bl.Total.V)
			}
		}
	}
}
