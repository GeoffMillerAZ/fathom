package console

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/inventory"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// knowledgeFixture is a three-project fleet with every truth state the
// Knowledge projections draw: alpha compiled two layers (a budget-stopped
// obligations run with rejection reasons, a dry run, a refused adr-rules
// estimate) and carries a kit pack; beta compiled obligations into zero
// current claims; gamma compiled nothing.
type knowledgeFixture struct {
	st        *store.MemoryStore
	src       KnowledgeSource
	alphaRoot string
	t0        time.Time
}

var errFixtureRead = errors.New("disk I/O error")

// failingStore makes one service's claims, or the claim events, unreadable.
type failingStore struct {
	store.Store
	claimsFail string
	eventsFail bool
}

func (f failingStore) ClaimsForService(ctx context.Context, svc, layer string) ([]core.SemanticClaim, error) {
	if svc == f.claimsFail {
		return nil, errFixtureRead
	}
	return f.Store.ClaimsForService(ctx, svc, layer)
}

func (f failingStore) ClaimEventsSince(ctx context.Context, since, until time.Time) ([]core.ClaimEvent, error) {
	if f.eventsFail {
		return nil, errFixtureRead
	}
	return f.Store.ClaimEventsSince(ctx, since, until)
}

func fixtureLayers(alphaRoot string) func(string) ([]semantic.LayerInfo, error) {
	return func(root string) ([]semantic.LayerInfo, error) {
		emb := func(name, kind, role string) semantic.LayerInfo {
			return semantic.LayerInfo{LayerDef: semantic.LayerDef{Name: name, Kind: kind, Role: role, Stratum: StratumConcern, Evaled: true,
				CorpusInclude: []string{"CLAUDE.md"}, Budgets: semantic.LayerBudgets{MaxTokensPerRun: 200000, MaxUSDPerRun: 1}}, Source: semantic.SourceEmbedded}
		}
		out := []semantic.LayerInfo{
			emb("adr-rules", "adr-rule", semantic.RoleGoverns),
			emb("findings", "finding", semantic.RoleClaims),
			emb("obligations", "obligation", semantic.RoleGoverns),
			emb("spec-bindings", "spec-binding", semantic.RoleClaims),
		}
		if root == alphaRoot {
			out = append(out, semantic.LayerInfo{LayerDef: semantic.LayerDef{Name: "ui-appkit", Kind: "adr-rule", Role: semantic.RoleGoverns, Stratum: StratumPlatform,
				PromptFile: "fathom-pack/layers/ui-appkit/prompt.md"}, Source: semantic.SourceProject})
		}
		return out, nil
	}
}

func newKnowledgeFixture(t *testing.T) *knowledgeFixture {
	t.Helper()
	ctx := context.Background()
	st := store.NewMemoryStore()
	f := &knowledgeFixture{st: st, alphaRoot: t.TempDir(), t0: time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)}
	if err := os.MkdirAll(filepath.Join(f.alphaRoot, ".fathom", "layers", "ui-appkit"), 0o755); err != nil {
		t.Fatal(err)
	}
	claude := "# alpha\nInject clocks; never call time.Now().\nKeep the store pure.\nOld rule, since moved.\nLast line.\n"
	if err := os.WriteFile(filepath.Join(f.alphaRoot, "CLAUDE.md"), []byte(claude), 0o644); err != nil {
		t.Fatal(err)
	}
	for svc, refs := range map[string][]string{
		"alpha": {"main.go", "store.go", "CLAUDE.md"},
		"beta":  {"app.py", "lib.py", "handlers.go"},
		"gamma": {"index.ts"},
	} {
		root := ""
		if svc == "alpha" {
			root = f.alphaRoot
		}
		if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: svc, Path: root}); err != nil {
			t.Fatal(err)
		}
		var obs []core.Observation
		for _, r := range refs {
			obs = append(obs, core.Observation{ServiceID: svc, SourceRef: r, SourceKind: core.KindGoFunc, Symbol: "S" + r})
		}
		if err := st.PutObservations(ctx, obs); err != nil {
			t.Fatal(err)
		}
	}
	claim := func(svc, id, layer, rule string, status core.ClaimStatus, cite, anchor string) {
		cl := core.SemanticClaim{ClaimID: id, ServiceID: svc, Layer: layer, Kind: "obligation", Subject: "CLAUDE.md#s-" + id,
			Rule: rule, Scope: []string{"**/*.go"}, Tier: core.TierDocClaimed, Status: status, Model: "m",
			PromptVersion: layer + "/v1", CompiledAtWatermark: "abc123", CompiledAt: f.t0, ValidFrom: f.t0}
		links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: cite, TargetHash: "h", AnchorText: anchor},
			{ClaimID: id, Rel: core.RelScopes, TargetRef: "**/*.go"}}
		if err := st.PutClaims(ctx, []core.SemanticClaim{cl}, links); err != nil {
			t.Fatal(err)
		}
	}
	claim("alpha", "c-a1", "obligations", "Inject clocks; never call time.Now().", core.ClaimFresh, "CLAUDE.md#L2-L2", "")
	claim("alpha", "c-a2", "obligations", "Old rule.", core.ClaimStale, "CLAUDE.md#L4-L4", "Old rule, as compiled.")
	claim("alpha", "c-a3", "obligations", "Superseded rule.", core.ClaimSuperseded, "CLAUDE.md#L3-L3", "")
	claim("alpha", "c-a4", "adr-rules", "Record every decision.", core.ClaimFresh, "CLAUDE.md#L3-L3", "")
	claim("beta", "c-b1", "obligations", "Retired rule.", core.ClaimSuperseded, "CLAUDE.md#L1-L1", "")
	run := func(r core.CompileRun) {
		if err := st.RecordCompileRun(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	run(core.CompileRun{RunID: "r-a1", ServiceID: "alpha", Layer: "obligations", Model: "m", CostUSD: 0.40, TokensIn: 1000, TokensOut: 200,
		SectionsTotal: 10, SectionsCompiled: 7, ClaimsEmitted: 3, ClaimsRejected: 2, RanAt: f.t0, BudgetStopped: true,
		UncompiledScopes: []string{"docs/CLAUDE.md"},
		RejectionReasons: []core.RejectionBreakdown{{Cause: "anchor-missing", Count: 2, Samples: []string{"cite L9 not found", "cite L12 not found"}}}})
	run(core.CompileRun{RunID: "r-a0", ServiceID: "alpha", Layer: "obligations", DryRun: true, RanAt: f.t0.Add(-time.Hour)})
	run(core.CompileRun{RunID: "r-a2", ServiceID: "alpha", Layer: "adr-rules", CostUSD: 0.25, ClaimsEmitted: 1, RanAt: f.t0.Add(time.Minute), SectionsTotal: 3, SectionsCompiled: 3})
	run(core.CompileRun{RunID: "r-a3", ServiceID: "alpha", Layer: "adr-rules", Refused: true, CostUSD: 9.99, RanAt: f.t0.Add(2 * time.Minute), RefusalReason: "token total exceeds budget"})
	run(core.CompileRun{RunID: "r-b1", ServiceID: "beta", Layer: "obligations", CostUSD: 0.05, ClaimsEmitted: 0, ClaimsRejected: 4, RanAt: f.t0.Add(time.Hour), SectionsTotal: 2, SectionsCompiled: 2})
	for i, ev := range []core.ClaimEvent{
		{ClaimID: "c-a1", Verdict: "re-anchored", Author: "sweep", Note: "CLAUDE.md#L1-L1 -> CLAUDE.md#L2-L2", At: f.t0.Add(time.Hour)},
		{ClaimID: "c-a1", Verdict: "corroborated", Author: "geoff", Note: "still true", At: f.t0.Add(2 * time.Hour)},
		{ClaimID: "c-a2", Verdict: "annotated", Author: "sweep", Note: "cited lines changed", At: f.t0.Add(3 * time.Hour)},
	} {
		_ = i
		if err := st.AppendClaimEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	f.src = KnowledgeSource{Store: st, Layers: fixtureLayers(f.alphaRoot)}
	return f
}

func bandOf(t *testing.T, s StrataView, stratum string) StrataBand {
	t.Helper()
	for _, b := range s.Bands {
		if b.Stratum == stratum {
			return b
		}
	}
	t.Fatalf("no %s band", stratum)
	return StrataBand{}
}

func blockOf(t *testing.T, b StrataBand, name string) LayerBlock {
	t.Helper()
	for _, l := range b.Layers {
		if l.Name == name {
			return l
		}
	}
	t.Fatalf("band %s has no layer %s: %+v", b.Stratum, name, b.Layers)
	return LayerBlock{}
}

// noV asserts a value's JSON carries no number — absence is never 0.
func noV(t *testing.T, what string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"v":`) {
		t.Errorf("%s must carry no number: %s", what, raw)
	}
}

// TestStrata_BandsAndLifecycle pins OC-27: the four bands in taxonomy
// order with every layer in its own band (a kit's platform pack in
// platform, the embedded catalog in concern), each carrying current
// claims, its lifecycle, real runs and their cost — refusals and dry runs
// never counted as spend — and the fleet's claim-event lifecycle.
func TestStrata_BandsAndLifecycle(t *testing.T) {
	f := newKnowledgeFixture(t)
	s, err := Strata(context.Background(), f.src)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, b := range s.Bands {
		order = append(order, b.Stratum)
	}
	if strings.Join(order, ",") != "dialect,provider,platform,concern" {
		t.Fatalf("bands = %v", order)
	}
	dialect := bandOf(t, s, StratumDialect)
	if len(dialect.Dialects) == 0 || dialect.Dialects[0].Level != "native" || !containsStr(dialect.Dialects[0].Languages, "Go") {
		t.Errorf("dialect band leads with the native tier holding Go: %+v", dialect.Dialects)
	}
	if p := bandOf(t, s, StratumProvider); !p.Parked || !strings.Contains(p.Reason, "TX-3") || len(p.Layers) != 0 {
		t.Errorf("provider is parked with its reason: %+v", p)
	}
	kit := blockOf(t, bandOf(t, s, StratumPlatform), "ui-appkit")
	if kit.Source != PackKit || strings.Join(kit.InstalledIn, ",") != "alpha" || kit.Claims.State != StateAbsent || kit.Runs.State != StateAbsent {
		t.Errorf("the kit pack sits in platform, installed in alpha, never compiled: %+v", kit)
	}
	noV(t, "a never-compiled layer's claims", kit.Claims)
	concern := bandOf(t, s, StratumConcern)
	ob := blockOf(t, concern, "obligations")
	if ob.Claims.State != StatePresent || ob.Claims.V != 2 || ob.Statuses != (StatusCounts{Fresh: 1, Stale: 1, Superseded: 2}) {
		t.Errorf("obligations = %+v %+v, want 2 current (1 fresh, 1 stale) and 2 superseded", ob.Claims, ob.Statuses)
	}
	if ob.Runs.V != 2 || ob.DryRuns != 1 || ob.CostUSD.V != 0.45 || strings.Join(ob.Services, ",") != "alpha" || ob.Role != semantic.RoleGoverns {
		t.Errorf("obligations runs = %+v cost %+v dry %d services %v", ob.Runs, ob.CostUSD, ob.DryRuns, ob.Services)
	}
	adr := blockOf(t, concern, "adr-rules")
	if adr.Refusals != 1 || adr.CostUSD.V != 0.25 || adr.Runs.V != 1 {
		t.Errorf("a refused run's estimate is never spend: %+v", adr)
	}
	find := blockOf(t, concern, "findings")
	if find.Claims.State != StateAbsent || !strings.Contains(find.Runs.Reason, "imported") || find.Role != semantic.RoleClaims {
		t.Errorf("findings = %+v", find)
	}
	tot := s.Totals
	if tot.Claims.V != 3 || tot.Runs.V != 3 || tot.CostUSD.V != 0.70 || tot.Emitted.V != 4 || tot.Rejected.V != 6 || tot.Refused.V != 1 || tot.Services.V != 1 || tot.Projects != 3 || tot.Layers != 5 {
		t.Errorf("totals = %+v", tot)
	}
	lc := s.Lifecycle
	if lc.Events["re-anchored"].V != 1 || lc.Events["corroborated"].V != 1 || lc.Events["annotated"].V != 1 {
		t.Errorf("lifecycle events = %+v", lc.Events)
	}
	if z := lc.Events["refuted"]; z.State != StateZero {
		t.Errorf("no refutation recorded is a measured zero, not an absence: %+v", z)
	}
	if lc.Statuses.Current() != 3 || lc.Statuses.Superseded != 2 {
		t.Errorf("lifecycle statuses = %+v", lc.Statuses)
	}

	// An unreadable source names itself: events → error, a project's
	// claims → a lower bound naming the project.
	broken := f.src
	broken.Store = failingStore{Store: f.st, claimsFail: "beta", eventsFail: true}
	s2, err := Strata(context.Background(), broken)
	if err != nil {
		t.Fatal(err)
	}
	if e := s2.Lifecycle.Events["re-anchored"]; e.State != StateError || !strings.Contains(e.Reason, "disk I/O error") {
		t.Errorf("unreadable events = %+v, want error naming the failure", e)
	}
	if c := s2.Totals.Claims; c.State != StatePartial || !strings.Contains(c.Reason, "beta") {
		t.Errorf("an unreadable project makes totals a named lower bound: %+v", c)
	}
}

// TestLayerCoverage_MissingIsNotZero pins OC-28: a (project, layer) cell
// nothing compiled is absent — "not compiled", no number in its JSON — a
// compile that left no current claim is a measured zero, an unreadable
// project is an error, and a layer nothing ever compiled is listed with
// where it is installed instead of a column of empty cells.
func TestLayerCoverage_MissingIsNotZero(t *testing.T) {
	f := newKnowledgeFixture(t)
	m, err := LayerCoverage(context.Background(), f.src)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(m.Layers, ",") != "adr-rules,obligations" {
		t.Fatalf("columns = %v, want the layers compiled somewhere", m.Layers)
	}
	var never []string
	for _, a := range m.NeverCompiled {
		never = append(never, a.Layer)
	}
	if strings.Join(never, ",") != "findings,spec-bindings,ui-appkit" {
		t.Errorf("never compiled = %v", never)
	}
	cell := func(svc, layer string) CoverageCell {
		for _, r := range m.Rows {
			if r.Service != svc {
				continue
			}
			for _, c := range r.Cells {
				if c.Layer == layer {
					return c
				}
			}
		}
		t.Fatalf("no cell %s/%s", svc, layer)
		return CoverageCell{}
	}
	if c := cell("alpha", "obligations"); c.Claims.State != StatePresent || c.Claims.V != 2 || c.Runs != 1 {
		t.Errorf("alpha/obligations = %+v", c)
	}
	if c := cell("beta", "obligations"); c.Claims.State != StateZero || c.Claims.V != 0 || !strings.Contains(c.Claims.Denominator, "superseded") {
		t.Errorf("beta compiled obligations into no current claim: %+v, want a measured zero", c)
	}
	gamma := cell("gamma", "obligations")
	if gamma.Claims.State != StateAbsent || gamma.Claims.Reason != "not compiled" {
		t.Errorf("gamma/obligations = %+v, want absent: not compiled", gamma)
	}
	noV(t, "a not-compiled cell", gamma.Claims)
	if m.Rows[0].Service != "alpha" || m.Covered.V != 1 {
		t.Errorf("rows lead with the most claims; covered = %+v", m.Covered)
	}
	// Only refused or dry runs: still not compiled, and it says so.
	ctx := context.Background()
	if err := f.st.RecordCompileRun(ctx, core.CompileRun{RunID: "r-g", ServiceID: "gamma", Layer: "adr-rules", Refused: true, RanAt: f.t0}); err != nil {
		t.Fatal(err)
	}
	m, _ = LayerCoverage(ctx, f.src)
	for _, r := range m.Rows {
		if r.Service == "gamma" && (r.Cells[0].Claims.State != StateAbsent || !strings.Contains(r.Cells[0].Claims.Reason, "refused")) {
			t.Errorf("a refused-only cell = %+v", r.Cells[0])
		}
	}
	broken := f.src
	broken.Store = failingStore{Store: f.st, claimsFail: "beta"}
	m, _ = LayerCoverage(ctx, broken)
	for _, r := range m.Rows {
		if r.Service == "beta" && (r.Cells[1].Claims.State != StateError || !strings.Contains(r.Cells[1].Claims.Reason, "disk I/O error")) {
			t.Errorf("an unreadable project's cell = %+v, want an error naming the failure", r.Cells[1])
		}
	}
	if m.Covered.State != StatePartial {
		t.Errorf("covered with an unreadable project = %+v, want partial", m.Covered)
	}
}

// TestLayerDetail_RunsAndRejections pins OC-29: a layer page lists every
// compile run newest first with its kind, cost and accepted/rejected
// counts, the rejection reasons with samples (and says when older runs
// kept only a count), what each project's latest run left uncompiled with
// the resume command, and the dry-run command.
func TestLayerDetail_RunsAndRejections(t *testing.T) {
	f := newKnowledgeFixture(t)
	d, err := LayerDetail(context.Background(), f.src, "obligations")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Known || d.Definition == nil || d.Definition.Stratum != StratumConcern || d.Definition.MaxTokensRun != 200000 {
		t.Fatalf("detail = %+v", d)
	}
	var runs []string
	for _, r := range d.Runs {
		runs = append(runs, r.RunID+":"+r.Kind)
	}
	if strings.Join(runs, ",") != "r-b1:compile,r-a1:compile,r-a0:dry-run" {
		t.Errorf("runs = %v, want newest first with their kind", runs)
	}
	if d.Runs[1].CostUSD != 0.40 || d.Runs[1].Accepted != 3 || d.Runs[1].Rejected != 2 || !d.Runs[1].Stopped || d.Runs[1].Uncompiled != 1 {
		t.Errorf("r-a1 = %+v", d.Runs[1])
	}
	if d.Accepted.V != 3 || d.Rejected.V != 6 {
		t.Errorf("accepted %+v rejected %+v, want 3 and 6 over real runs", d.Accepted, d.Rejected)
	}
	if len(d.Rejections) != 1 || d.Rejections[0].Cause != "anchor-missing" || d.Rejections[0].Count != 2 || len(d.Rejections[0].Samples) != 2 {
		t.Errorf("rejections = %+v", d.Rejections)
	}
	if !strings.Contains(d.RejectionsNote, "1 of 2 runs") {
		t.Errorf("a run that rejected without reasons is named: %q", d.RejectionsNote)
	}
	if len(d.Uncompiled) != 1 || d.Uncompiled[0].Service != "alpha" || strings.Join(d.Uncompiled[0].Scopes, ",") != "docs/CLAUDE.md" ||
		d.Uncompiled[0].Resume != "fathom compile --layer obligations --service alpha --continue" {
		t.Errorf("uncompiled = %+v", d.Uncompiled)
	}
	if d.DryRun != "fathom compile --layer obligations --service <project>" {
		t.Errorf("dry run = %q", d.DryRun)
	}
	var projects []string
	for _, p := range d.Projects {
		projects = append(projects, p.Service+":"+string(p.Claims.State))
	}
	if strings.Join(projects, ",") != "alpha:present,beta:zero" {
		t.Errorf("projects = %v", projects)
	}
	if p := d.Projects[0]; p.DryRun != "fathom compile --layer obligations --service alpha" {
		t.Errorf("project dry run = %q", p.DryRun)
	}
	unknown, err := LayerDetail(context.Background(), f.src, "no-such-layer")
	if err != nil || unknown.Known {
		t.Errorf("an unknown layer = %+v, %v", unknown, err)
	}
	never, _ := LayerDetail(context.Background(), f.src, "spec-bindings")
	if !never.Known || never.Accepted.State != StateAbsent || len(never.Runs) != 0 {
		t.Errorf("a never-compiled layer = %+v", never)
	}
	noV(t, "a never-compiled layer's accepted count", never.Accepted)
}

// TestClaimDetail_EventsAndCitation pins OC-30: the claim page carries the
// statement, scope, tier and status, the parsed citation with its cited
// lines excerpted from the current file (context around, cited lines
// marked), and the verdict history oldest first; a stale claim shows its
// compile-time anchor text, never the current lines as its source.
func TestClaimDetail_EventsAndCitation(t *testing.T) {
	f := newKnowledgeFixture(t)
	ctx := context.Background()
	d, err := ClaimDetail(ctx, f.src, "c-a1", "")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Found || d.Service != "alpha" || d.Statement != "Inject clocks; never call time.Now()." || d.Status != "fresh" || !d.Served ||
		d.Tier != string(core.TierDocClaimed) || strings.Join(d.Scope, ",") != "**/*.go" || d.CompiledAt == "" {
		t.Fatalf("detail = %+v", d)
	}
	if len(d.Citations) != 1 || d.Citations[0].Path != "CLAUDE.md" || d.Citations[0].Start != 2 || d.Citations[0].End != 2 {
		t.Errorf("citations = %+v", d.Citations)
	}
	ex := d.Excerpt
	if ex.State != StatePresent || ex.V.From != "current file" || len(ex.V.Lines) != 4 || ex.V.Lines[0].Number != 1 {
		t.Fatalf("excerpt = %+v", ex)
	}
	for _, l := range ex.V.Lines {
		if l.Cited != (l.Number == 2) {
			t.Errorf("line %d cited = %v", l.Number, l.Cited)
		}
	}
	if ex.V.Lines[1].Text != "Inject clocks; never call time.Now()." {
		t.Errorf("cited line = %q", ex.V.Lines[1].Text)
	}
	var verdicts []string
	for _, e := range d.Events {
		verdicts = append(verdicts, e.Verdict)
	}
	if strings.Join(verdicts, ",") != "re-anchored,corroborated" || d.EventCount.V != 2 || d.Events[1].Author != "geoff" {
		t.Errorf("events = %+v", d.Events)
	}
	var rels []string
	for _, l := range d.Links {
		rels = append(rels, l.Rel)
	}
	if strings.Join(rels, ",") != "scopes" {
		t.Errorf("links = %+v", d.Links)
	}

	stale, _ := ClaimDetail(ctx, f.src, "c-a2", "alpha")
	if stale.Excerpt.V.From != "compile-time anchor text" || stale.Excerpt.V.Lines[0].Text != "Old rule, as compiled." || !strings.Contains(stale.Excerpt.V.Note, "stale") {
		t.Errorf("a stale claim's excerpt = %+v", stale.Excerpt)
	}
	missing, _ := ClaimDetail(ctx, f.src, "nope", "")
	if missing.Found || missing.Excerpt.State != StateAbsent || missing.EventCount.State != StateAbsent {
		t.Errorf("an unknown claim = %+v", missing)
	}
	// The same id in two services is ambiguous until one is named.
	cl := core.SemanticClaim{ClaimID: "c-a1", ServiceID: "gamma", Layer: "obligations", Rule: "x", Status: core.ClaimFresh}
	if err := f.st.PutClaims(ctx, []core.SemanticClaim{cl}, nil); err != nil {
		t.Fatal(err)
	}
	amb, _ := ClaimDetail(ctx, f.src, "c-a1", "")
	if amb.Found || len(amb.Matches) != 2 || amb.Matches[0].Service != "alpha" {
		t.Errorf("an ambiguous id = %+v", amb)
	}
	if named, _ := ClaimDetail(ctx, f.src, "c-a1", "gamma"); !named.Found || named.Excerpt.State != StateAbsent {
		t.Errorf("gamma's copy has no root or citation to read: %+v", named.Excerpt)
	}
	// A cited file that is gone is absent; an unreadable one is an error.
	if err := os.Remove(filepath.Join(f.alphaRoot, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	gone, _ := ClaimDetail(ctx, f.src, "c-a4", "alpha")
	if gone.Excerpt.State != StateAbsent || !strings.Contains(gone.Excerpt.Reason, "no longer exists") {
		t.Errorf("a vanished citation = %+v", gone.Excerpt)
	}
}

// repoRoot is this repository's root, from this file's location.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// TestPackMatrix_MeasuredOrSaidUnmeasured pins OC-31: every registered
// language is a row with its engine, typed-edge producer, fleet use,
// integrity reach and inventory keys; precision and recall are the
// published reading where one exists — citing a document in this
// repository — and "unmeasured", with no number, where none does; layer
// packs name their source and where each is active.
func TestPackMatrix_MeasuredOrSaidUnmeasured(t *testing.T) {
	f := newKnowledgeFixture(t)
	m, err := PackMatrix(context.Background(), f.src)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]DialectPack{}
	for _, d := range m.Dialects {
		rows[d.Language] = d
	}
	for _, r := range DialectReadings {
		if _, err := os.Stat(filepath.Join(repoRoot(t), r.Source)); err != nil {
			t.Errorf("reading %s/%s cites %s, which is not in the repository: %v", r.Language, r.Tier, r.Source, err)
		}
		row, ok := rows[r.Language]
		if !ok {
			t.Errorf("reading for unregistered language %s", r.Language)
			continue
		}
		if row.Precision.State != StatePresent || row.Precision.V != r.Precision || row.Recall.V != r.Recall || !strings.Contains(row.Reading, r.Source) {
			t.Errorf("%s = %+v, want the published reading", r.Language, row)
		}
	}
	rust := rows["rust"]
	if rust.Precision.State != StateAbsent || !strings.HasPrefix(rust.Precision.Reason, "unmeasured") || rust.Recall.State != StateAbsent {
		t.Errorf("rust = %+v, want unmeasured", rust)
	}
	noV(t, "an unmeasured precision", rust.Precision)
	goRow := rows["go"]
	if goRow.Engine != "go/parser + go/types" || goRow.TypedEdges != "native" || goRow.Integrity != "checked" || goRow.Files.V != 3 || goRow.Projects.V != 2 {
		t.Errorf("go = %+v", goRow)
	}
	if py := rows["py"]; py.TypedEdges != "scip" || py.Producer != "scip-python" || py.Files.V != 2 || py.Integrity != "unproven" || py.Engine != "tree-sitter (wasm)" {
		t.Errorf("py = %+v", py)
	}
	if md := rows["md"]; md.Files.V != 1 || md.Integrity != "n/a" || len(md.Inventory) != 0 {
		t.Errorf("md = %+v", md)
	}
	if rs := rows["rust"]; rs.Files.State != StateZero {
		t.Errorf("a registered language nobody indexes is a measured zero: %+v", rs.Files)
	}
	if m.Dialects[0].Language != "go" {
		t.Errorf("most-used language first, got %s", m.Dialects[0].Language)
	}

	// The inventory column is what the inventory extractor actually draws
	// from each language — checked against it, never asserted.
	probes := map[string]inventory.File{
		"go": {Path: "svc/main.go", Content: []byte(`package svc

import (
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

type Config struct {
	Name string ` + "`json:\"name\"`" + `
}

func Serve() {}

var root = cobra.Command{Use: "serve"}

func init() {
	http.HandleFunc("/health", nil)
	_ = os.Getenv("SVC_TOKEN")
}
`)},
		"py":    {Path: "app.py", Content: []byte("import os\n\n" + "@app.route(\"/health\")\ndef health():\n    return os.environ[\"SVC_TOKEN\"]\n")},
		"ts":    {Path: "server.ts", Content: []byte("app.get('/health', handler);\nconst t = process.env.SVC_TOKEN;\n")},
		"proto": {Path: "svc.proto", Content: []byte("syntax = \"proto3\";\nservice Svc {\n  rpc Health(Req) returns (Resp);\n}\n")},
	}
	for tag, probe := range probes {
		kinds := map[string]bool{}
		for _, it := range inventory.Extract([]inventory.File{probe}, nil) {
			kinds[it.Kind] = true
		}
		var got []string
		for k := range kinds {
			got = append(got, k)
		}
		sort.Strings(got)
		if want := inventoryKinds(tag); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: the extractor draws %v, the matrix says %v", tag, got, want)
		}
	}

	packs := map[string]LayerPack{}
	for _, l := range m.Layers {
		packs[l.Name] = l
	}
	if kit := packs["ui-appkit"]; kit.Source != PackKit || kit.Stratum != StratumPlatform || strings.Join(kit.InstalledIn, ",") != "alpha" || len(kit.ActiveIn) != 0 || kit.Contributes.State != StateAbsent {
		t.Errorf("ui-appkit = %+v", kit)
	}
	if ob := packs["obligations"]; ob.Source != PackEmbedded || strings.Join(ob.ActiveIn, ",") != "alpha" || ob.Contributes.V != 2 {
		t.Errorf("obligations = %+v", ob)
	}
}

// ServingView folds what the obligations tool adds around the compiled
// rules: raw-fallback rows first, the coverage note naming the uncompiled
// guide with its resume command, and whether serving re-ranks.
func TestServingView_RawRowsFirstAndCoverageNote(t *testing.T) {
	ex := semantic.ObligationsExplanation{Service: "alpha", File: "internal/x/y.go",
		Served: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "compiled"}}}}
	v := ServingView(ServingInputs{
		Explanation: ex, Raw: []semantic.Obligation{{Rule: "raw", Tier: "raw"}}, RawWhy: "compiled layer incomplete", RawOK: true,
		Coverage: semantic.LayerCoverage{SectionsTotal: 10, SectionsCompiled: 7, UncompiledScopes: []string{"internal/CLAUDE.md"}}, CoverageFound: true,
	})
	if len(v.AgentSees) != 2 || v.AgentSees[0].Rule != "raw" || v.AgentSees[1].Rule != "compiled" {
		t.Errorf("agent sees = %+v, want the raw row first", v.AgentSees)
	}
	if !strings.Contains(v.CoverageNote, "internal/CLAUDE.md governs this path and was not compiled: 7/10 sections compiled") || !strings.Contains(v.CoverageNote, "--service alpha --continue") {
		t.Errorf("coverage note = %q", v.CoverageNote)
	}
	if !strings.HasPrefix(v.Rerank, "off") {
		t.Errorf("rerank = %q", v.Rerank)
	}
	none := ServingView(ServingInputs{Explanation: ex, RerankProvider: "llm"})
	if none.Coverage.State != StateAbsent || len(none.RawFallback) != 0 || none.RawFallback == nil || !strings.Contains(none.Rerank, "llm") {
		t.Errorf("no coverage recorded = %+v", none)
	}
	bad := ServingView(ServingInputs{Explanation: ex, CoverageErr: errFixtureRead})
	if bad.Coverage.State != StateError || !strings.Contains(bad.Coverage.Reason, "disk I/O error") {
		t.Errorf("unreadable coverage = %+v", bad.Coverage)
	}
}
