package governedprosekit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

func TestProfileAndSessionContracts(t *testing.T) {
	good := testProfile(t)
	base := DocumentTypeProfile{Key: "k", Version: "1", Origin: ProfileOrigin{Kind: "supplied"}, Sections: good.Sections, Goals: good.Goals, Defaults: good.Defaults}
	bad := map[string]func(*DocumentTypeProfile){
		"supplied claims catalog":   func(p *DocumentTypeProfile) { p.Origin.CatalogKey = "x" },
		"catalog without evidence":  func(p *DocumentTypeProfile) { p.Origin = ProfileOrigin{Kind: "catalog", CatalogKey: "k"} },
		"no required goal":          func(p *DocumentTypeProfile) { p.Goals = []Goal{good.Goals[1]} },
		"no required section":       func(p *DocumentTypeProfile) { p.Sections = []SectionSpec{{ID: "a", Title: "A"}} },
		"goal scoped to unknown":    func(p *DocumentTypeProfile) { g := good.Goals[0]; g.Scope = "nope"; p.Goals = []Goal{g} },
		"invalid utf8":              func(p *DocumentTypeProfile) { p.Key = "k\xff" },
		"blank defaults":            func(p *DocumentTypeProfile) { p.Defaults.Voice = "" },
		"profile origin on session": func(p *DocumentTypeProfile) { g := good.Goals[0]; g.Origin = OriginSession; p.Goals = []Goal{g} },
	}
	for name, mutate := range bad {
		p := base
		mutate(&p)
		if _, err := NewDocumentTypeProfile(p); !errors.Is(err, ErrSession) {
			t.Errorf("%s: want ErrSession, got %v", name, err)
		}
	}
	digest := strings.Repeat("a", 64)
	cat := base
	cat.Origin = ProfileOrigin{Kind: "catalog", CatalogKey: "doc/incident-exec", DefinitionDigest: digest, LayerEvidence: []string{"grant:org"}}
	if p, err := NewDocumentTypeProfile(cat); err != nil || p.Origin.Kind != "catalog" {
		t.Fatalf("catalog-bound profile: %v", err)
	}
	other := base
	other.Version = "2"
	p2, _ := NewDocumentTypeProfile(other)
	if p2.Digest == good.Digest {
		t.Fatal("profile version must change the digest")
	}

	s1, err := NewDocumentSession(good, SessionInput{ID: "s", Purpose: "p", Supplied: PresentationDefaults{Voice: "formal"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s1.DefaultsFrom["voice"] != "session" || s1.DefaultsFrom["audience"] != "profile:test-brief" || s1.Presentation.Audience != "executive" {
		t.Fatalf("defaults provenance: %+v", s1.DefaultsFrom)
	}
	// Required-goal omission: the caller expects a goal the authored list lacks.
	if _, err := NewDocumentSession(good, SessionInput{ID: "s", Purpose: "p", ExpectedGoalIDs: []string{"g-missing"}}, nil); !errors.Is(err, ErrSession) {
		t.Fatalf("omitted expected goal must refuse: %v", err)
	}
	// An expected goal that exists only as advisory is not satisfied by listing it.
	if _, err := NewDocumentSession(good, SessionInput{ID: "s", Purpose: "p", ExpectedGoalIDs: []string{"g-voice"}}, nil); !errors.Is(err, ErrSession) {
		t.Fatalf("expected advisory goal must refuse: %v", err)
	}
	// Floors: no redefinition, downgrade or unscoped extension.
	down := Goal{ID: "g-support", Class: GoalAdvisory, Statement: "State only supported facts", Scope: ScopeDocument, Method: "m", Acceptance: "a", Origin: OriginTask}
	if _, err := NewDocumentSession(good, SessionInput{ID: "s", Purpose: "p", Goals: []Goal{down}}, nil); !errors.Is(err, ErrSession) {
		t.Fatalf("floor downgrade must refuse: %v", err)
	}
	tighten := good.Goals[1]
	tighten.Class, tighten.Origin = GoalRequired, OriginSession
	if _, err := NewDocumentSession(good, SessionInput{ID: "s", Purpose: "p", Goals: []Goal{tighten}}, nil); err != nil {
		t.Fatalf("tightening advisory to required must be allowed: %v", err)
	}
	local := Goal{ID: "g-local", Class: GoalRequired, Statement: "Mention rollback", Scope: "findings", Method: "exact-support", Acceptance: "present", Origin: OriginTask}
	s2, err := NewDocumentSession(good, SessionInput{ID: "s", Purpose: "p", Goals: []Goal{local}, ExpectedGoalIDs: []string{"g-local"}}, &s1)
	if err != nil || s2.Revision != 2 || s2.Parent != s1.Digest || s2.Digest == s1.Digest {
		t.Fatalf("revision chain: %+v %v", s2, err)
	}
	if _, err := NewDocumentSession(good, SessionInput{ID: "different", Purpose: "p"}, &s1); !errors.Is(err, ErrSession) {
		t.Fatal("parent with another session ID must refuse")
	}
	if _, err := NewDocumentSession(good, SessionInput{ID: "s", Purpose: "bad\xfe"}, nil); !errors.Is(err, ErrSession) {
		t.Fatal("invalid UTF-8 purpose must refuse")
	}
	forged := good
	forged.Goals = []Goal{good.Goals[1]}
	if _, err := NewDocumentSession(forged, SessionInput{ID: "s", Purpose: "p"}, nil); !errors.Is(err, ErrSession) {
		t.Fatal("tampered profile must refuse")
	}
}

func TestGenerateRenderInspectAndPromptCarriesGoals(t *testing.T) {
	e := newDocEnv(t)
	v := e.generate()
	if got := strings.Join(e.client.called(), ","); got != "h1,p1,p2,hs,sum" && got != "h1,p1,p2,sum,hs" {
		// Dependencies first, otherwise document order.
		t.Fatalf("one call per leaf in dependency order, got %s", got)
	}
	if err := VerifyDocument(v); err != nil {
		t.Fatal(err)
	}
	r, err := e.engine.Render(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	want := "## Findings\n\n12 incidents\n\n30 minutes\n\n## Summary\n\nResolved"
	if r.Markdown != want {
		t.Fatalf("markdown:\n%q\nwant\n%q", r.Markdown, want)
	}
	for _, sp := range r.SpanMap {
		if r.Markdown[sp.Start:sp.End] != nodeByID(v, sp.NodeID).Text {
			t.Fatalf("span for %s does not select its text", sp.NodeID)
		}
	}
	prompt := e.client.prompts["p1"]
	for _, must := range []string{"g-support", "State only supported facts", "g-voice", "Brief the executive team", "executive", `"required_goals"`, `"advisory_goals"`} {
		if !strings.Contains(prompt, must) {
			t.Errorf("prompt lacks %q", must)
		}
	}
	var payload struct {
		Required []Goal `json:"required_goals"`
		Advisory []Goal `json:"advisory_goals"`
	}
	_ = json.Unmarshal([]byte(prompt), &payload)
	if len(payload.Required) != 1 || len(payload.Advisory) != 1 || payload.Required[0].ID != "g-support" {
		t.Fatalf("required/advisory split: %+v", payload)
	}
	if strings.Contains(prompt, "12 incidents") == false || strings.Contains(prompt, "Resolved") {
		t.Fatal("p1 must see only its declared facts")
	}
	if v.Receipt.SessionDigest != e.session.Digest || v.Receipt.ProfileDigest != e.profile.Digest || v.Receipt.ConfigDigest == "" {
		t.Fatalf("receipt bindings: %+v", v.Receipt)
	}
	p1 := nodeByID(v, "p1")
	if p1.Generation == nil || p1.Generation.Model != "fixture-generator" || p1.Generation.InputTokens != 100 || p1.GenManifest.Role != "generation" || p1.ReviewMan.Role != "review" || len(p1.ReviewMan.Facts) != 3 || len(p1.GenManifest.Facts) != 1 || len(p1.Support) == 0 {
		t.Fatalf("p1 metadata: %+v", p1)
	}
	in, err := InspectNode(v, "sum")
	if err != nil || len(in.Dependencies) != 2 || len(in.Goals) == 0 {
		t.Fatalf("inspect: %+v %v", in, err)
	}
	if in2, _ := InspectNode(v, "p1"); len(in2.Dependents) != 1 {
		t.Fatalf("dependents: %+v", in2.Dependents)
	}
	if _, err := InspectNode(v, "nope"); err == nil {
		t.Fatal("unknown node must error")
	}
}

func TestLocalCorrectionClosureAndExactReuse(t *testing.T) {
	e := newDocEnv(t)
	v1 := e.generate()
	next := e.revise("fact-a", "13 incidents")
	plan, err := e.engine.PlanChange(v1, next, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ids(plan.Regenerate) != "p1,sum" {
		t.Fatalf("floor must include the dependent summary: %+v", plan.Regenerate)
	}
	if ids(plan.Rerender) != "doc,findings,summary" {
		t.Fatalf("containers rerender: %+v", plan.Rerender)
	}
	if ids(plan.ReviewAgain) != "h1,p2,hs" || len(plan.Hold) != 0 || len(plan.Rebind) != 0 {
		t.Fatalf("wider review read set re-reviews without regenerating: %+v", plan)
	}
	if !strings.Contains(strings.Join(plan.Regenerate[1].Reasons, ","), "depends:p1") {
		t.Fatalf("summary reason: %v", plan.Regenerate[1].Reasons)
	}
	e.client.reset()
	res, err := e.engine.Rebuild(context.Background(), v1, next, nil, RebuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(e.client.called(), ","); got != "p1,sum" {
		t.Fatalf("only the closure calls the model, got %s", got)
	}
	v2 := res.Version
	if nodeByID(v2, "p1").Text != "13 incidents" || nodeByID(v2, "p2").Text != nodeByID(v1, "p2").Text || !nodeByID(v2, "p2").Reused {
		t.Fatal("changed node updated, unrelated node reused")
	}
	if nodeByID(v2, "p2").ContentDigest != nodeByID(v1, "p2").ContentDigest || nodeByID(v2, "p2").Version == nodeByID(v1, "p2").Version {
		t.Fatal("reused words keep content identity but get a fresh review/version")
	}
	if nodeByID(v2, "p2").ReviewMan.Digest == nodeByID(v1, "p2").ReviewMan.Digest {
		t.Fatal("review read set must change when any fact changes")
	}
	if v2.Number != 2 || v2.Parent != v1.Receipt.DocumentDigest || VerifyDocument(v2) != nil {
		t.Fatalf("version chain: %d %q", v2.Number, v2.Parent)
	}
	d := res.Delta
	// sum's words are unchanged: it is provenance (new context), never words.
	if strings.Join(d.WordsChanged, ",") != "p1" || strings.Join(d.ProvenanceChanged, ",") != "h1,p2,hs,sum" {
		t.Fatalf("delta must separate words from provenance: %+v", d)
	}
	if d.RegeneratedLeaves != 2 || d.LeafTotal != 5 || d.ModelCalls != 2 || d.BytesChanged != len("13 incidents") {
		t.Fatalf("delta fractions: %+v", d)
	}
	// v1 is immutable.
	if nodeByID(v1, "p1").Text != "12 incidents" || VerifyDocument(v1) != nil {
		t.Fatal("parent version was mutated")
	}
}

func TestSourceRevisionNeedsExplicitEquivalence(t *testing.T) {
	e := newDocEnv(t)
	v1 := e.generate()
	next := e.revise("fact-a", "12 incidents") // same bytes, new source revision
	plan, err := e.engine.PlanChange(v1, next, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ids(plan.Hold) != "p1,sum" || len(plan.Regenerate) != 0 {
		t.Fatalf("a bare revision holds and does not freshen: %+v", plan)
	}
	e.client.reset()
	if _, err := e.engine.Rebuild(context.Background(), v1, next, nil, RebuildOptions{}); !errors.Is(err, ErrImpact) || len(e.client.called()) != 0 {
		t.Fatalf("held plan must refuse before any call: %v %v", err, e.client.called())
	}
	old, cur := v1.Target.Input.Facts[0].Source, next.Input.Facts[0].Source
	for name, rb := range map[string]SourceRebind{
		"no actor":       {FactID: "fact-a", From: old, To: cur, Equivalence: "same quote"},
		"no equivalence": {FactID: "fact-a", From: old, To: cur, Actor: "editor"},
		"wrong from":     {FactID: "fact-a", From: cur, To: cur, Equivalence: "same quote", Actor: "editor"},
	} {
		p, err := e.engine.PlanChange(v1, next, []SourceRebind{rb})
		if name == "wrong from" {
			if !errors.Is(err, nil) && !errors.Is(err, ErrImpact) {
				t.Errorf("%s: %v", name, err)
			}
			continue
		}
		if err != nil || ids(p.Hold) != "p1,sum" {
			t.Errorf("%s must still hold: %v %+v", name, err, p.Hold)
		}
	}
	rb := SourceRebind{FactID: "fact-a", From: old, To: cur, Equivalence: "identical quoted bytes in revision 2", Actor: "editor-1"}
	res, err := e.engine.Rebuild(context.Background(), v1, next, []SourceRebind{rb}, RebuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.client.called()) != 0 || ids(res.Plan.Rebind) != "p1" {
		t.Fatalf("rebind keeps words, no model: %v %+v", e.client.called(), res.Plan)
	}
	p1 := nodeByID(res.Version, "p1")
	if p1.Text != "12 incidents" || !p1.Reused || p1.Version == nodeByID(v1, "p1").Version || p1.GenManifest.Facts[0].Digest == nodeByID(v1, "p1").GenManifest.Facts[0].Digest {
		t.Fatal("rebound node must re-bind provenance and review, not inherit it")
	}
	if len(res.Delta.WordsChanged) != 0 || !strings.Contains(strings.Join(res.Delta.ProvenanceChanged, ","), "p1") {
		t.Fatalf("delta: %+v", res.Delta)
	}
	// An equivalence claim cannot hide a real change, nor a stale declaration linger.
	changed := e.revise("fact-b", "45 minutes")
	if _, err := e.engine.PlanChange(v1, changed, []SourceRebind{{FactID: "fact-b", From: v1.Target.Input.Facts[1].Source, To: changed.Input.Facts[1].Source, Equivalence: "claimed", Actor: "x"}}); !errors.Is(err, ErrImpact) {
		t.Fatalf("rebind over changed bytes must refuse: %v", err)
	}
	if _, err := e.engine.PlanChange(v1, e.target, []SourceRebind{rb}); !errors.Is(err, ErrImpact) {
		t.Fatalf("unmatched rebind must refuse: %v", err)
	}
	// A rebind target that is no longer current fails the fresh source check.
	e.sources.current["source-a"] = old
	if _, err := e.engine.Rebuild(context.Background(), v1, next, []SourceRebind{rb}, RebuildOptions{}); err == nil {
		t.Fatal("rebind must re-check current source")
	}
}

func TestWholeInputAndUncitedExposureInvalidate(t *testing.T) {
	e := newDocEnv(t)
	for i := range e.target.Spec.Nodes {
		switch e.target.Spec.Nodes[i].ID {
		case "p2":
			e.target.Spec.Nodes[i].WholeInput, e.target.Spec.Nodes[i].FactIDs = true, nil
		case "p1":
			e.target.Spec.Nodes[i].FactIDs = []string{"fact-a", "fact-b"} // exposed, cites only the first
		}
	}
	v1 := e.generate()
	if len(nodeByID(v1, "p2").GenManifest.Facts) != 3 || !nodeByID(v1, "p2").GenManifest.WholeInput {
		t.Fatal("whole-input manifest must list every fact")
	}
	// fact-c is unrelated to p1 but p2 sees the whole input.
	plan, err := e.engine.PlanChange(v1, e.revise("fact-c", "Mitigated"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ids(plan.Regenerate), "p2") || strings.Contains(ids(plan.Regenerate), "p1") {
		t.Fatalf("whole input regenerates p2 only: %v", ids(plan.Regenerate))
	}
	// fact-b is exposed to p1 but never cited by it.
	plan, err = e.engine.PlanChange(v1, e.revise("fact-b", "45 minutes"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ids(plan.Regenerate), "p1") || !strings.Contains(ids(plan.Regenerate), "p2") {
		t.Fatalf("uncited exposure is still a dependency: %v", ids(plan.Regenerate))
	}
	// Neighbor context is a dependency too, and its change cascades to dependents.
	e2 := newDocEnv(t)
	for i := range e2.target.Spec.Nodes {
		if e2.target.Spec.Nodes[i].ID == "p2" {
			e2.target.Spec.Nodes[i].ContextNodeIDs = []string{"p1"}
		}
	}
	w1 := e2.generate()
	plan, _ = e2.engine.PlanChange(w1, e2.revise("fact-a", "13 incidents"), nil)
	if ids(plan.Regenerate) != "p1,p2,sum" {
		t.Fatalf("context edge cascades: %v", ids(plan.Regenerate))
	}
}

func TestModelCannotShrinkFloorButMayWiden(t *testing.T) {
	e := newDocEnv(t)
	v1 := e.generate()
	next := e.revise("fact-a", "13 incidents")
	e.client.reset()
	if _, err := e.engine.Rebuild(context.Background(), v1, next, nil, RebuildOptions{Requested: []string{"p2"}}); !errors.Is(err, ErrImpact) || len(e.client.called()) != 0 {
		t.Fatalf("shrinking request must refuse before any call: %v", err)
	}
	if _, err := e.engine.Rebuild(context.Background(), v1, next, nil, RebuildOptions{Requested: []string{"p1"}}); !errors.Is(err, ErrImpact) {
		t.Fatal("request omitting the dependent summary must refuse")
	}
	if _, err := e.engine.Rebuild(context.Background(), v1, next, nil, RebuildOptions{SemanticWiden: []string{"findings"}}); !errors.Is(err, ErrImpact) {
		t.Fatal("widening a container must refuse")
	}
	res, err := e.engine.Rebuild(context.Background(), v1, next, nil, RebuildOptions{Requested: []string{"p1", "sum", "p2"}, SemanticWiden: []string{"h1"}})
	if err != nil {
		t.Fatal(err)
	}
	if ids(res.Plan.Regenerate) != "h1,p1,p2,sum" {
		t.Fatalf("widened set: %v", ids(res.Plan.Regenerate))
	}
}

func TestUnknownDependenciesCyclesAndOmittedEdgesHold(t *testing.T) {
	cases := map[string]func(*DocumentSpec){
		"cycle": func(s *DocumentSpec) {
			for i := range s.Nodes {
				switch s.Nodes[i].ID {
				case "p1":
					s.Nodes[i].ContextNodeIDs = []string{"p2"}
				case "p2":
					s.Nodes[i].ContextNodeIDs = []string{"p1"}
				}
			}
		},
		"missing node": func(s *DocumentSpec) { s.Nodes[4].ContextNodeIDs = []string{"ghost"} },
		"omitted expected edge": func(s *DocumentSpec) {
			s.ExpectedEdges = append(s.ExpectedEdges, Edge{From: "p2", To: "p1", Kind: EdgeContext})
		},
		"expected edge to missing": func(s *DocumentSpec) {
			s.ExpectedEdges = append(s.ExpectedEdges, Edge{From: "sum", To: "ghost", Kind: EdgeSummary})
		},
		"summary without inputs": func(s *DocumentSpec) { s.Nodes[7].Summarizes = nil },
	}
	for name, mutate := range cases {
		e := newDocEnv(t)
		mutate(&e.target.Spec)
		_, err := e.engine.Generate(context.Background(), e.target)
		if !errors.Is(err, ErrImpact) && !errors.Is(err, ErrSession) || len(e.client.called()) != 0 {
			t.Errorf("%s: want hold before any call, got %v (calls %v)", name, err, e.client.called())
		}
	}
	// Structure: expected section absent, required goal bound to nothing, unknown extra section.
	e := newDocEnv(t)
	e.target.Spec.Nodes, e.target.Spec.ExpectedEdges = e.target.Spec.Nodes[:5], nil // drops the required summary section
	if _, err := e.engine.Generate(context.Background(), e.target); !errors.Is(err, ErrSession) {
		t.Fatalf("missing expected section must refuse: %v", err)
	}
	e = newDocEnv(t)
	e.target.Spec.Nodes = append(e.target.Spec.Nodes, NodeSpec{ID: "extra", Kind: KindSection, ParentID: "doc", Order: 9})
	if _, err := e.engine.Generate(context.Background(), e.target); !errors.Is(err, ErrSession) {
		t.Fatalf("unknown section without PermitExtra must refuse: %v", err)
	}
	e = newDocEnv(t)
	local := Goal{ID: "g-roll", Class: GoalRequired, Statement: "Mention rollback", Scope: "findings", Method: "exact-support", Acceptance: "present", Origin: OriginTask}
	s2, err := NewDocumentSession(e.profile, SessionInput{ID: "sess-1", Purpose: "p", Goals: []Goal{local}, ExpectedGoalIDs: []string{"g-roll"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	e.target.Session = s2
	spec := e.target.Spec
	spec.Nodes = []NodeSpec{
		{ID: "doc", Kind: KindDocument},
		{ID: "findings", Kind: KindSection, ParentID: "doc", Order: 1},
		{ID: "summary", Kind: KindSection, ParentID: "doc", Order: 2},
		{ID: "hs", Kind: KindHeading, ParentID: "summary", Order: 1, Instruction: "Summary"},
	}
	spec.ExpectedEdges = nil
	e.target.Spec = spec
	if _, err := e.engine.Generate(context.Background(), e.target); !errors.Is(err, ErrSession) || !strings.Contains(err.Error(), "bound to no generated node") && !strings.Contains(err.Error(), "required goal") {
		t.Fatalf("required goal with no leaf must refuse: %v", err)
	}
}

func TestSessionAndProfileChangeInvalidateWithEqualBytes(t *testing.T) {
	e := newDocEnv(t)
	v1 := e.generate()
	local := Goal{ID: "g-local", Class: GoalRequired, Statement: "Be exact", Scope: "findings", Method: "exact-support", Acceptance: "exact", Origin: OriginTask}
	s2, err := NewDocumentSession(e.profile, SessionInput{ID: "sess-1", Purpose: "Brief the executive team", Intent: "tighten", Goals: []Goal{local}, ExpectedGoalIDs: []string{"g-local"}}, &e.session)
	if err != nil {
		t.Fatal(err)
	}
	next := e.target
	next.Session = s2
	plan, err := e.engine.PlanChange(v1, next, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Intent is exposed to every leaf, so every leaf regenerates.
	if len(plan.Regenerate) != 5 {
		t.Fatalf("session text change reaches every exposed leaf: %+v", plan.Regenerate)
	}
	res, err := e.engine.Rebuild(context.Background(), v1, next, nil, RebuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	v2 := res.Version
	if nodeByID(v2, "p1").Text != nodeByID(v1, "p1").Text || v2.Receipt.DocumentDigest == v1.Receipt.DocumentDigest || v2.Receipt.SessionDigest == v1.Receipt.SessionDigest {
		t.Fatal("equal bytes under a new session revision must still produce a new bound identity")
	}
	if len(res.Delta.WordsChanged) != 0 {
		t.Fatalf("words unchanged: %+v", res.Delta)
	}
	// A goal scoped to findings with unchanged session text only reaches that section.
	e3 := newDocEnv(t)
	w1 := e3.generate()
	s3, _ := NewDocumentSession(e3.profile, SessionInput{ID: "sess-1", Purpose: "Brief the executive team", Intent: "initial draft", Goals: []Goal{local}}, &e3.session)
	n3 := e3.target
	n3.Session = s3
	p3, err := e3.engine.PlanChange(w1, n3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ids(p3.Regenerate) != "h1,p1,p2,sum" || !strings.Contains(ids(p3.ReviewAgain), "hs") {
		t.Fatalf("scoped goal: regenerate %s review %s", ids(p3.Regenerate), ids(p3.ReviewAgain))
	}
	// A different profile pin invalidates the session binding outright.
	alt := e3.profile
	alt.Version = "9"
	alt, _ = NewDocumentTypeProfile(alt)
	n4 := e3.target
	n4.Profile = alt
	if _, err := e3.engine.PlanChange(w1, n4, nil); !errors.Is(err, ErrSession) {
		t.Fatalf("session pinned to another profile must refuse: %v", err)
	}
}

func TestImmutabilityAndInvalidUTF8(t *testing.T) {
	e := newDocEnv(t)
	v := e.generate()
	e.target.Input.Facts[0].Span.Value = "tampered"
	e.target.Spec.Nodes[3].Instruction = "tampered"
	if VerifyDocument(v) != nil || v.Target.Input.Facts[0].Span.Value != "12 incidents" {
		t.Fatal("caller input mutation must not reach the version")
	}
	mut := v
	mut.Nodes = append([]DocNode(nil), v.Nodes...)
	for i := range mut.Nodes {
		if mut.Nodes[i].ID == "p1" {
			mut.Nodes[i].Text = "forged"
		}
	}
	if VerifyDocument(mut) == nil {
		t.Fatal("tampered node text must fail verification")
	}
	if _, err := RenderMarkdown(mut); err == nil {
		t.Fatal("tampered version must not render")
	}
	mut2 := v
	mut2.Receipt.SessionDigest = strings.Repeat("0", 64)
	if VerifyDocument(mut2) == nil {
		t.Fatal("receipt rebinding must fail")
	}
	// Invalid UTF-8 anywhere in the target is refused, not normalized.
	e2 := newDocEnv(t)
	e2.target.Spec.Nodes[3].Instruction = "bad\xff"
	if _, err := e2.engine.Generate(context.Background(), e2.target); err == nil || len(e2.client.called()) != 0 {
		t.Fatalf("invalid UTF-8 spec must refuse before any call: %v", err)
	}
	e3 := newDocEnv(t)
	e3.target.Input.Facts[0].Span.Value = "bad\xff"
	if _, err := e3.engine.Generate(context.Background(), e3.target); err == nil {
		t.Fatal("invalid UTF-8 fact must refuse")
	}
	// Inspect returns copies.
	in, _ := InspectNode(v, "p1")
	in.Node.Text = "x"
	if nodeByID(v, "p1").Text != "12 incidents" {
		t.Fatal("inspection must copy")
	}
}

type nilSources struct{ SourceProvider }

func TestTypedNilDependenciesCancellationAndRecovery(t *testing.T) {
	e := newDocEnv(t)
	cfg := e.engine.cfg
	var nilClient *fakeDocClient
	var nilSrc *testSources
	var nilRev *ExactSupportReviewer
	clock := func() time.Time { return sourcedEpoch }
	if _, err := NewDocumentEngine(nilClient, e.sources, ExactSupportReviewer{}, cfg, clock); err == nil {
		t.Fatal("typed-nil client must refuse")
	}
	if _, err := NewDocumentEngine(e.client, nilSrc, ExactSupportReviewer{}, cfg, clock); err == nil {
		t.Fatal("typed-nil sources must refuse")
	}
	if _, err := NewDocumentEngine(e.client, e.sources, nilRev, cfg, clock); err == nil {
		t.Fatal("typed-nil reviewer must refuse")
	}
	bad := cfg
	bad.Limits.MaxNodes = 0
	if _, err := NewDocumentEngine(e.client, e.sources, ExactSupportReviewer{}, bad, clock); err == nil {
		t.Fatal("zero limits must refuse")
	}
	// Canceled before work and mid-run: no version, no partial publication.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if v, err := e.engine.Generate(ctx, e.target); err == nil || v.Receipt.DocumentDigest != "" {
		t.Fatalf("canceled before start: %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	e.client.after = func(n int) {
		if n == 2 {
			cancel()
		}
	}
	e.client.reset()
	v, err := e.engine.Generate(ctx, e.target)
	if !errors.Is(err, context.Canceled) || v.Receipt.DocumentDigest != "" {
		t.Fatalf("mid-run cancel must hold: %v", err)
	}
	if got := len(e.client.called()); got != 2 {
		t.Fatalf("generation must stop at cancellation, calls %d", got)
	}
	// Render refuses stale or revoked sources even for reused bytes.
	e.client.after = nil
	good := e.generate()
	ref := good.Target.Input.Facts[0].Source
	snap := e.sources.items[ref]
	snap.Revoked = true
	e.sources.items[ref] = snap
	if _, err := e.engine.Render(context.Background(), good); err == nil {
		t.Fatal("revoked source must refuse render")
	}
}

func TestModelCannotSupplyAuthorityOrEraseRequirements(t *testing.T) {
	cases := map[string]func(c *fakeDocClient){
		"wrong unit id": func(c *fakeDocClient) {
			c.raw = func(n string) (string, bool) {
				return `{"segments":[{"id":"other","kind":"assertion","text":"x","fact_ids":["fact-a"],"contradictions":[],"reason":""}]}`, n == "p1"
			}
		},
		"unknown status field": func(c *fakeDocClient) {
			c.raw = func(n string) (string, bool) {
				return `{"segments":[{"kind":"label","text":"Findings","fact_ids":[],"contradictions":[],"reason":"label","status":"supported"}]}`, n == "h1"
			}
		},
		"cites unexposed fact": func(c *fakeDocClient) {
			c.only = "p1"
			c.text = func(n string, f []promptFact) (string, []string) { return "30 minutes", []string{"fact-b"} }
		},
		"non-factual paragraph": func(c *fakeDocClient) {
			c.raw = func(n string) (string, bool) {
				return `{"segments":[{"kind":"label","text":"Findings","fact_ids":[],"contradictions":[],"reason":"label"}]}`, n == "p1"
			}
		},
		"misclassified factual heading": func(c *fakeDocClient) {
			c.raw = func(n string) (string, bool) {
				return `{"segments":[{"kind":"label","text":"Revenue grew 40%","fact_ids":[],"contradictions":[],"reason":"label"}]}`, n == "h1"
			}
		},
		"truncated": func(c *fakeDocClient) { c.stop = llmclient.StopMaxTokens },
		"wrong but valid citation": func(c *fakeDocClient) {
			// Cites fact-a but quotes different wording: exact support must refuse.
			c.only = "p1"
			c.text = func(n string, f []promptFact) (string, []string) { return "99 incidents", []string{f[0].ID} }
		},
	}
	for name, setup := range cases {
		e := newDocEnv(t)
		setup(e.client)
		if _, err := e.engine.Generate(context.Background(), e.target); err == nil {
			t.Errorf("%s: model output must not become a version", name)
		}
	}
}

func TestDocumentJSONMatchesCUE(t *testing.T) {
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Skip("CUE tool unavailable; task cue-vet has the same optional prerequisite")
	}
	e := newDocEnv(t)
	v1 := e.generate()
	next := e.revise("fact-a", "13 incidents")
	plan, err := e.engine.PlanChange(v1, next, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.engine.Rebuild(context.Background(), v1, next, nil, RebuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := e.engine.Render(context.Background(), res.Version)
	if err != nil {
		t.Fatal(err)
	}
	check := func(name string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "actual.json")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, cue, "vet", "schema.cue", path, "-d", name).CombinedOutput(); err != nil {
			t.Fatalf("actual Go JSON violates %s: %v\n%s", name, err, out)
		}
	}
	check("#DocumentTypeProfile", e.profile)
	check("#DocumentSession", e.session)
	check("#DocumentTarget", e.target)
	check("#DocumentVersion", v1)
	check("#DocumentVersion", res.Version)
	check("#ImpactPlan", plan)
	check("#DocumentDelta", res.Delta)
	check("#RenderedDocument", rendered)
	cat := e.profile
	cat.Origin = ProfileOrigin{Kind: "catalog", CatalogKey: "doc/x", DefinitionDigest: strings.Repeat("b", 64), LayerEvidence: []string{"grant:org"}}
	cat, _ = NewDocumentTypeProfile(cat)
	check("#DocumentTypeProfile", cat)
}
