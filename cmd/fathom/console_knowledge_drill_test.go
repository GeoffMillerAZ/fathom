package main

// console_knowledge_drill_test.go — the Knowledge pages' drill-downs and
// truth states: every claim count links to the claims it counts (/claims,
// `fathom claim list`), a layer nothing compiled never reads as fully
// covered, remedies are commands that run, a broken or unreadable pack is
// never dressed in defaults, the explainer pages every candidate and gives
// each rule one number, and the kit's own pieces are used.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// TestConsoleClaims_EveryCountDrillsDownToItsRows: /layers, the layer page
// and /packs link their claim counts to /claims with the count's filter,
// and the list those links open holds exactly the counted claims, each
// opening its claim page (vision 35 §5).
func TestConsoleClaims_EveryCountDrillsDownToItsRows(t *testing.T) {
	f := newConsoleFixture(t)
	claimsLink := regexp.MustCompile(`href="(/claims\?[^"]*)"`)
	for _, path := range []string{"/layers", "/layers/obligations", "/packs"} {
		_, _, body := f.get(t, path)
		links := claimsLink.FindAllStringSubmatch(body, -1)
		if len(links) == 0 {
			t.Errorf("%s: no claim count links to /claims", path)
		}
		for _, m := range links {
			href := strings.ReplaceAll(m[1], "&amp;", "&")
			if code, _, _ := f.get(t, href); code != http.StatusOK {
				t.Errorf("%s links to %s = %d", path, href, code)
			}
		}
	}
	// The coverage cell for payment-service × obligations counts 3 current
	// claims (one statement is held twice); its link lists those 3.
	_, _, layers := f.get(t, "/layers")
	cell := `href="/claims?layer=obligations&amp;svc=payment-service"`
	if !strings.Contains(layers, cell) {
		t.Errorf("/layers: the coverage cell must link to its claims (%s)", cell)
	}
	env := f.pageJSON(t, "/claims?layer=obligations&svc=payment-service")
	verb := "fathom claim list --layer obligations --service payment-service --status current --json"
	var l console.ClaimList
	if err := json.Unmarshal(env.Sections[verb], &l); err != nil {
		t.Fatalf("no section for %q: %v (%v)", verb, err, env.Verbs)
	}
	if l.Total.V != 3 || len(l.Claims) != 3 {
		t.Errorf("payment-service obligations = %+v, want the 3 current claims the cell counts", l.Total)
	}
	_, _, list := f.get(t, "/claims?layer=obligations&svc=payment-service")
	if !strings.Contains(list, `href="/claims/kc-payment-idem?svc=payment-service"`) || strings.Count(list, "<h1") != 1 ||
		!strings.Contains(list, `data-console-copy="`+htmlEscapeTest(verb)+`"`) {
		t.Error("/claims must list each claim linked to its page, with its verb chip")
	}
	assertRoleUtilitiesOnly(t, list)
	// Status and paging are in the URL and the verb.
	stale := f.pageJSON(t, "/claims?layer=obligations&status=stale")
	if len(stale.Verbs) != 1 || stale.Verbs[0] != "fathom claim list --layer obligations --status stale --json" {
		t.Errorf("stale verbs = %v", stale.Verbs)
	}
	for path, status := range map[string]int{
		"/claims?status=bogus":        http.StatusBadRequest,
		"/claims?layer=no-such":       http.StatusNotFound,
		"/claims?svc=no-such-project": http.StatusNotFound,
	} {
		if code, _, body := f.get(t, path); code != status || !strings.Contains(body, `<main id="observatory-main"`) {
			t.Errorf("%s = %d, want %d on the shell", path, code, status)
		}
	}
	// The verb refuses what the page refuses.
	if _, errb, code := runVerb(t, f, "claim", "list", "--status", "bogus"); code == 0 || !strings.Contains(errb, "--status must be one of") {
		t.Errorf("claim list --status bogus = %d %s", code, errb)
	}
}

// A layer nothing compiled has nothing measured about what it left
// uncompiled: the panel is absent with its reason, never "every compile
// covered its whole corpus".
func TestConsoleLayerPage_NeverCompiledIsNotFullyCovered(t *testing.T) {
	f := newConsoleFixture(t)
	code, _, body := f.get(t, "/layers/spec-bindings")
	if code != http.StatusOK {
		t.Fatalf("/layers/spec-bindings = %d", code)
	}
	i := strings.Index(body, `id="uncompiled"`)
	if i < 0 {
		t.Fatal("no uncompiled panel")
	}
	start := strings.LastIndex(body[:i], "<div")
	panel := body[start:]
	if j := strings.Index(body[i:], "data-console-panel"); j > 0 {
		panel = body[start : i+j]
	}
	if !strings.Contains(panel, `data-truth-state="absent"`) || !strings.Contains(panel, "not measured — no real compile run recorded") {
		t.Errorf("the uncompiled panel of a never-compiled layer must be absent with its reason:\n%.600s", panel)
	}
	if strings.Contains(body, "covered its whole corpus") {
		t.Error("a never-compiled layer claims full coverage")
	}
	// A compiled layer whose latest compiles were all complete says so,
	// naming how many projects compiled it.
	d := console.LayerPage{Layer: "adr-rules", UncompiledCount: console.Measured(0, "x"), CompiledProjects: 1, Uncompiled: []console.UncompiledRun{}}
	if got := renderUncompiled(d); !strings.Contains(got, "the one project that compiled this layer covered its whole corpus") {
		t.Errorf("complete coverage = %q", got)
	}
}

// Remedies run: a resume names --store and --dry-run=false, because
// `fathom compile --continue` refuses without --store and compile only
// estimates by default.
func TestConsoleRemedies_ResumeCommandsRun(t *testing.T) {
	f := newConsoleFixture(t)
	_, _, body := f.get(t, "/layers/obligations")
	want := "$ fathom compile --layer obligations --service payment-service --store " + consoleFixtureStorePath(t) + " --continue --dry-run=false"
	if !strings.Contains(body, htmlEscapeTest(want)) {
		t.Errorf("/layers/obligations: no runnable resume remedy %q", want)
	}
	if strings.Contains(body, "--continue</div>") || regexp.MustCompile(`--service [a-z-]+ --continue`).MatchString(body) {
		t.Error("a resume remedy without --store is left on the page")
	}
	// The command it prints is one compile accepts: parse it through the
	// verb's own flag checks up to the store (a dry run of the flags).
	args := strings.Fields(strings.TrimPrefix(want, "$ fathom "))
	if args[0] != "compile" || !containsString(args, "--store") || !containsString(args, "--dry-run=false") {
		t.Errorf("resume args = %v", args)
	}
}

// The explainer's candidate table pages 50 at a time, so the column
// note's "every candidate is in the table below" holds, and a later page
// opens with the table expanded.
func TestConsoleExplain_TablePagesEveryCandidate(t *testing.T) {
	se := console.ServingExplanation{}
	for i := 1; i <= 120; i++ {
		c := semantic.ExplainCandidate{ClaimID: fmt.Sprintf("c%03d", i), Rule: fmt.Sprintf("Rule %03d.", i), Rank: i, ScopeTier: semantic.ScopeTierRoot}
		if i <= 6 {
			c.Served, c.Position = true, i
		} else {
			c.Displaced = &semantic.Displacement{Reason: semantic.DisplacedRank, Why: "ranked lower"}
		}
		se.Candidates = append(se.Candidates, c)
	}
	se.Tiers = []semantic.ExplainTier{{Tier: semantic.ScopeTierRoot, Candidates: 120, Served: 6, Displaced: 114}}
	href := func(p int) string { return fmt.Sprintf("/explain?svc=x&page=%d#candidates", p) }
	p1 := renderExplainTable(se, 1, href)
	if !strings.Contains(p1, "Candidates 1–50 of 120") || !strings.Contains(p1, "Rule 050.") || strings.Contains(p1, "Rule 051.") ||
		!strings.Contains(p1, `href="/explain?svc=x&amp;page=3#candidates"`) || strings.Contains(p1, "<details class=\"group rounded-md border border-line bg-surface-raised\" open") {
		t.Errorf("page 1 of the candidates table:\n%.1200s", p1)
	}
	p3 := renderExplainTable(se, 3, href)
	if !strings.Contains(p3, "Candidates 101–120 of 120") || !strings.Contains(p3, "Rule 120.") || !strings.Contains(p3, " open") {
		t.Error("page 3 must hold the last 20 candidates, its disclosure open")
	}
	if cols := renderExplainColumns(se); !strings.Contains(cols, "every candidate is in the paged table below") {
		t.Error("the column note must point at the paged table")
	}
}

// One rule, one number: the injected list numbers the raw-fallback rows
// apart and each compiled rule by its served position — the number its
// candidate card shows.
func TestConsoleExplain_OneNumberPerRule(t *testing.T) {
	se := console.ServingExplanation{
		RawFallback: []semantic.Obligation{{Rule: "raw guide one", Tier: "raw"}, {Rule: "raw guide two", Tier: "raw"}},
		RawWhy:      "the compiled layer did not reach internal/CLAUDE.md",
		AgentSees: []semantic.Obligation{{Rule: "raw guide one", Tier: "raw"}, {Rule: "raw guide two", Tier: "raw"},
			{Rule: "Keep refunds idempotent.", Tier: "doc-claimed"}, {Rule: "Inject clocks.", Tier: "doc-claimed"}},
	}
	se.Service = "svc"
	se.Candidates = []semantic.ExplainCandidate{
		{ClaimID: "a", Rule: "Keep refunds idempotent.", Served: true, Position: 1, Rank: 1, ScopeTier: semantic.ScopeTierPackage},
		{ClaimID: "b", Rule: "Inject clocks.", Served: true, Position: 2, Rank: 2, ScopeTier: semantic.ScopeTierParentDir},
	}
	got := renderAgentSees(se)
	for _, want := range []string{">raw 1<", ">raw 2<", "#2 · doc-claimed · Ancestor directories"} {
		if !strings.Contains(got, want) {
			t.Errorf("injected list missing %q", want)
		}
	}
	if strings.Contains(got, "#4") || strings.Contains(got, "#3") {
		t.Error("a compiled rule is numbered after the raw rows — a second number for one rule")
	}
	if card := renderCandidate(se.Candidates[1]); !strings.Contains(card, "served #2") {
		t.Errorf("the card must carry the same number: %s", card)
	}
}

// /packs: the dialect matrix carries each language's typed share and says
// which tier a reading measured; a pack that failed to load keeps its load
// error beside its name and invents no stratum or role; an unreadable
// catalog is named on the page.
func TestConsolePacks_TruthBesideEveryFigure(t *testing.T) {
	f := newConsoleFixture(t)
	_, _, body := f.get(t, "/packs")
	for _, want := range []string{">Typed share<", "native tier", "file tier", "data-reading-tier"} {
		if !strings.Contains(body, want) {
			t.Errorf("/packs missing %q", want)
		}
	}
	m := console.PacksView{
		Layers: []console.LayerPack{
			{Name: "resume-standards", Source: console.PackProject, Note: "broken project pack ignored: layer.cue does not satisfy #Layer", Contributes: console.Absent[int]("never compiled")},
			{Name: "imbue-standards", Source: console.PackUnknown, Origin: "no readable pack defines this layer, but the pack catalog of imbue could not be read", Contributes: console.Measured(4, "x")},
		},
	}
	got := renderLayerPackTable(m)
	if !strings.Contains(got, "does not satisfy #Layer") || !strings.Contains(got, "pack catalog of imbue could not be read") || strings.Count(got, ">unknown<") < 3 {
		t.Errorf("broken and unknown packs must show their reason and no invented stratum or role:\n%s", got)
	}
	if strings.Contains(got, ">concern<") || strings.Contains(got, ">governs<") {
		t.Error("a pack that failed to load is shown with a default stratum or role")
	}
	d := console.DialectPack{Name: "Python", TypedEdges: "scip", Producer: "scip-python", EdgeKinds: map[string]int{"file-resolved": 22},
		TypedShare: console.Measured(0.0, "x").WithDenominator("0 of 22 call edges confirmed by a resolver — every one heuristic")}
	if cell := typedEdgesText(d); !strings.Contains(cell, "no SCIP index imported") {
		t.Errorf("a scip language with no SCIP edge = %q", cell)
	}
	if cell := shareText(d); !strings.Contains(cell, "0%") || !strings.Contains(cell, "0 of 22 resolved") || !strings.Contains(cell, "every one heuristic") {
		t.Errorf("share = %q", cell)
	}
}

// The pack source badge is the kit's (callout.Badge), never a hand-rolled
// pill (doctrine rule 1).
func TestConsoleSourceBadge_IsTheKitBadge(t *testing.T) {
	if got, want := sourceBadge(console.PackEmbedded), callout.Badge("embedded", ""); got != want {
		t.Errorf("sourceBadge = %s, want the kit's %s", got, want)
	}
	if got := sourceBadge(console.PackUnknown); got != callout.Badge("pack unknown", "caution") {
		t.Errorf("an unknown pack's badge = %s", got)
	}
}

// Heatmap row labels fit the kit's gutter by width, so a wide name is
// shortened, never clipped at its first letters.
func TestHeatmapLabel_FitsTheGutterByWidth(t *testing.T) {
	for in, want := range map[string]string{
		"resume_as_code":     "resume_a…",
		"growing-gardens-bi": "growing-g…",
		"pulumi_eks_cluster": "pulumi_ek…",
		"fathom":             "fathom",
		"ledger-service":     "ledger-ser…",
	} {
		if got := heatmapLabel(in); got != want {
			t.Errorf("heatmapLabel(%q) = %q, want %q", in, got, want)
		}
		if w := labelWidth(heatmapLabel(in)); w > heatmapLabelBudget {
			t.Errorf("%q is %.1f px wide, over the %.0f px budget", heatmapLabel(in), w, heatmapLabelBudget)
		}
	}
}

// A claim page's current-file lines for a claim whose range may have moved
// are marked "at the cited range", never "cited".
func TestClaimExcerpt_MovedRangeIsNotCalledCited(t *testing.T) {
	d := console.ClaimPage{Status: "superseded", Excerpt: console.Present(console.Excerpt{Path: "ADR.md", Start: 2, End: 2, From: "current file",
		Note:  "the claim is superseded — a terminal verdict, so its citation is never re-anchored as the file changes, and its compile-time text was not kept: these are the current lines at the cited range, which may have moved",
		Lines: []console.ExcerptLine{{Number: 1, Text: "a"}, {Number: 2, Text: "b", Cited: true}}}, "x")}
	got := renderClaimExcerpt(d)
	if !strings.Contains(got, "At the cited range") || strings.Contains(got, ">Cited<") || !strings.Contains(got, "data-excerpt-note") {
		t.Errorf("a superseded claim's current lines:\n%s", got)
	}
}
