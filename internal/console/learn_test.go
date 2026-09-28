package console

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// pipelineFacts is one project read with every instrument answering.
func pipelineFacts() PipelineFacts {
	return PipelineFacts{
		Service: "pay ments", Freshness: "fresh", IndexedCommit: "95ffaf5b680e1234abcd", RepoHead: "95ffaf5b680e1234abcd",
		Files:             Measured(120, "store").WithDenominator("files"),
		Languages:         []LanguageFiles{{Lang: "go", Files: 100, Producer: "native"}, {Lang: "md", Files: 20, Producer: "syntax"}},
		UnclassifiedFiles: 3,
		Symbols:           Measured(2200, "registry"),
		Edges:             Measured(4000, "store"), TypedEdges: Measured(3000, "store"),
		ClaimsCurrent: Measured(40, "store"), ClaimsFlagged: Measured(0, "store"), ClaimsRetired: Measured(9, "store"),
		Layers: []string{"obligations", "adr-rules"}, CompileRuns: Measured(3, "store"),
		RulesServed: Measured(120, "log"), WindowCalls: Measured(900, "log"),
		GateRuns: Measured(50, "ledger"), GatePlans: Measured(4, "ledger"), GateVerified: Measured(30, "ledger"), GateClaimed: Measured(32, "ledger"),
		Sessions: Measured(12, "ledger"), Calls: Measured(2500, "log"),
		Capabilities: []CapabilityState{
			{ID: "intent-corpus", Operating: true},
			{ID: "index", Operating: true, Detail: "2,200 symbols"},
			{ID: "layers", Operating: true},
			{ID: "modules", Operating: false, Reason: "no module bound"},
			{ID: "obligations", Operating: false, Broken: true, Reason: "rules authored, never compiled", Remedy: "fathom compile --service pay"},
			{ID: "agent-surface", Operating: true},
			{ID: "guards", Operating: true},
			{ID: "verify-gate", Operating: true},
			{ID: "session-ledger", Operating: true},
		},
	}
}

// TestPipelineView_StagesAndCounts pins OC-46's projection: seven stages
// in the pipeline's order, each carrying its live counts with their truth
// states (an unread instrument stays absent with its reason — never 0 — and
// a measured nothing stays a zero), the switches that deliver it (required
// ones decide whether it works here, optional ones never do), and the pages
// that show it in full with the project filled in and escaped.
func TestPipelineView_StagesAndCounts(t *testing.T) {
	p := PipelineView(pipelineFacts())

	var ids []PipelineStageID
	for i, st := range p.Stages {
		ids = append(ids, st.ID)
		if st.Step != i+1 {
			t.Errorf("stage %s is step %d, want %d", st.ID, st.Step, i+1)
		}
		if st.Title == "" || st.Does == "" || st.Question == "" || st.Pieces == "" || len(st.Pages) == 0 || st.Live == "" {
			t.Errorf("stage %s is missing its description, pages or live source: %+v", st.ID, st)
		}
	}
	want := []PipelineStageID{StageSource, StageExtract, StageIndex, StageCompile, StageServe, StageGate, StageEvidence}
	if !reflect.DeepEqual(ids, want) || !reflect.DeepEqual(PipelineStageIDs(), want) {
		t.Fatalf("stages = %v, want %v", ids, want)
	}
	if len(p.Edges) != 6 || p.Edges[0] != (PipelineEdge{StageSource, StageExtract}) || p.Edges[5] != (PipelineEdge{StageGate, StageEvidence}) {
		t.Errorf("edges = %v, want the six hand-offs in order", p.Edges)
	}
	stage := func(id PipelineStageID) PipelineStage {
		for _, s := range p.Stages {
			if s.ID == id {
				return s
			}
		}
		t.Fatalf("no stage %s", id)
		return PipelineStage{}
	}

	// Headlines are the facts, unchanged.
	for id, v := range map[PipelineStageID]int{StageSource: 120, StageExtract: 2200, StageIndex: 4000, StageCompile: 40, StageServe: 120, StageGate: 50, StageEvidence: 12} {
		if h := stage(id).Headline.Value; h.V != v || h.State != StatePresent {
			t.Errorf("%s headline = %+v, want %d present", id, h, v)
		}
	}
	src := stage(StageSource)
	if src.Counts[0].Label != "languages" || src.Counts[0].Value.V != 2 {
		t.Errorf("source languages = %+v, want the 2 languages read", src.Counts)
	}
	if strings.Join(src.Notes, "|") != "indexed at 95ffaf5b680e|HEAD 95ffaf5b680e" || src.Status != StageOperating {
		t.Errorf("source notes/status = %v %s", src.Notes, src.Status)
	}
	ext := stage(StageExtract)
	if ext.Counts[0].Value.V != 1 || !strings.Contains(strings.Join(ext.Notes, "|"), "go · 100 files · typed edges from the native type checker") ||
		!strings.Contains(strings.Join(ext.Notes, "|"), "3 files no dialect pack reads") {
		t.Errorf("extract = %+v", ext)
	}
	if n := stage(StageIndex).Notes; len(n) != 1 || n[0] != "75.0% of edges resolved by a type checker or SCIP index" {
		t.Errorf("index share note = %v", n)
	}
	comp := stage(StageCompile)
	if comp.Counts[0].Value.State != StateZero || comp.Notes[0] != "layers: adr-rules, obligations" {
		t.Errorf("a measured zero stays zero and layers are named sorted: %+v", comp)
	}
	if n := stage(StageGate).Notes; len(n) != 1 || n[0] != "30 of 32 claimed rows verified at each plan's latest gate run" {
		t.Errorf("gate note = %v", n)
	}

	// Status: a required switch that is broken, on a stage whose own
	// counts show it at work, is a working stage with the break named and
	// its remedy — never "off" beside live counts; an optional switch
	// (modules) never decides.
	serve := stage(StageServe)
	if serve.Status != StageWorking || !strings.Contains(serve.StatusReason, "obligations is broken here: rules authored, never compiled") || serve.Remedy != "fathom compile --service pay" {
		t.Errorf("serve with obligations broken = %s %q %q", serve.Status, serve.StatusReason, serve.Remedy)
	}
	if st := stage(StageCompile); st.Status != StageOperating || len(st.Capabilities) != 2 || !st.Capabilities[0].Required || st.Capabilities[1].Required {
		t.Errorf("compile: modules is optional and off, layers required and on: %+v", st)
	}
	if st := stage(StageEvidence); st.Status != StageOperating {
		t.Errorf("evidence stage with session-ledger on must be operating: %s %s", st.Status, st.StatusReason)
	}

	// Pages: the project filled in, escaped for a path and for a query.
	if got := stage(StageSource).Pages[0]; got != "/projects/pay%20ments" {
		t.Errorf("source page = %q", got)
	}
	if got := stage(StageServe).Pages[0]; got != "/explain?svc=pay+ments" {
		t.Errorf("serve page = %q", got)
	}
	if got := stage(StageGate).Live; got != "plans:pay ments" {
		t.Errorf("gate live key = %q", got)
	}

	// Absent is never 0: an unread instrument stays absent with its reason,
	// on the stage and in the JSON (no v).
	f := pipelineFacts()
	f.Edges = Absent[int]("no typed pass has run")
	f.TypedEdges = f.Edges
	f.Files = Errored[int]("EPERM")
	f.Freshness = "stale: 3 commits behind"
	f.FreshnessRemedy = "fathom post-commit-hook --service pay"
	f.Capabilities = nil
	f.CapabilitiesNote = "no corpus root is recorded"
	p = PipelineView(f)
	stage = func(id PipelineStageID) PipelineStage {
		for _, s := range p.Stages {
			if s.ID == id {
				return s
			}
		}
		return PipelineStage{}
	}
	idx := stage(StageIndex)
	if idx.Headline.Value.State != StateAbsent || idx.Headline.Value.Reason != "no typed pass has run" || len(idx.Notes) != 0 {
		t.Errorf("absent edges = %+v", idx)
	}
	raw, err := json.Marshal(idx.Headline)
	if err != nil || strings.Contains(string(raw), `"v"`) || !strings.Contains(string(raw), `"state":"absent"`) {
		t.Errorf("absent headline JSON = %s (%v)", raw, err)
	}
	if s := stage(StageSource); s.Headline.Value.State != StateError || s.Counts[0].Value.State != StateError ||
		s.Status != StageStale || s.StatusReason != "the index is behind HEAD (3 commits behind)" || s.Remedy != "fathom post-commit-hook --service pay" {
		t.Errorf("unreadable files and a stale index = %+v", s)
	}
	if s := stage(StageExtract); s.Counts[0].Value.State != StateError {
		t.Errorf("typed-pack languages over unreadable files must be an error, got %+v", s.Counts[0].Value)
	}
	for _, s := range p.Stages[1:] {
		if s.Status != StageUnknown || s.StatusReason != "no corpus root is recorded" {
			t.Errorf("%s without a capability report = %s %q, want unknown with the reason", s.ID, s.Status, s.StatusReason)
		}
	}
	for _, s := range p.Stages {
		for _, c := range append([]PipelineCount{s.Headline}, s.Counts...) {
			if err := c.Value.Validate(); err != nil {
				t.Errorf("%s %q: %v", s.ID, c.Label, err)
			}
		}
	}
}

// TestPipelineView_StatusNeverContradictsTheCounts: a switch that is off
// because its inputs do not exist here (ABSENT: no rules authored, no pack
// installed) says nothing about a stage its own counts show at work — the
// stage is working without it and no remedy is pressed on it; the same
// switch on a stage that shows no work turns it off with the remedy; a
// stale index makes extract and index stale, with the daemon-routed
// refresh, never off.
func TestPipelineView_StatusNeverContradictsTheCounts(t *testing.T) {
	status := func(f PipelineFacts, id PipelineStageID) PipelineStage {
		t.Helper()
		for _, s := range PipelineView(f).Stages {
			if s.ID == id {
				return s
			}
		}
		t.Fatalf("no stage %s", id)
		return PipelineStage{}
	}
	setCap := func(f *PipelineFacts, c CapabilityState) {
		for i := range f.Capabilities {
			if f.Capabilities[i].ID == c.ID {
				f.Capabilities[i] = c
			}
		}
	}
	absentRules := CapabilityState{ID: "obligations", Reason: "no obligation rules are authored for this repository (.fathom/rules/ is empty or absent)", Remedy: "fathom layers install <pack>"}

	// Serving compiled layer claims with no rules authored: working.
	f := pipelineFacts()
	setCap(&f, absentRules)
	if s := status(f, StageServe); s.Status != StageWorking || s.Remedy != "" || !strings.Contains(s.StatusReason, "obligations is off here") {
		t.Errorf("serve with rules absent and 120 served = %s %q %q", s.Status, s.StatusReason, s.Remedy)
	}
	// Nothing served and no rules authored: off, with the remedy.
	f.RulesServed = Measured(0, "log")
	if s := status(f, StageServe); s.Status != StageOff || s.Remedy != "fathom layers install <pack>" || !strings.HasPrefix(s.StatusReason, "obligations: no obligation rules") {
		t.Errorf("serve with rules absent and none served = %s %q %q", s.Status, s.StatusReason, s.Remedy)
	}
	// Not measured is not "no work": an unread log leaves the stage off.
	f.RulesServed = Errored[int]("call log unreadable")
	if s := status(f, StageServe); s.Status != StageOff {
		t.Errorf("serve with rules absent and an unreadable log = %s", s.Status)
	}

	// Compiled claims with no pack installed in the repository: working.
	f = pipelineFacts()
	setCap(&f, CapabilityState{ID: "layers", Reason: "no layer packs are installed in this repository", Remedy: "fathom layers install <pack>"})
	if s := status(f, StageCompile); s.Status != StageWorking || s.Remedy != "" {
		t.Errorf("compile with 40 current claims and no pack = %s %q", s.Status, s.Remedy)
	}
	f.ClaimsCurrent = Absent[int]("no layer has been compiled")
	if s := status(f, StageCompile); s.Status != StageOff || s.Remedy != "fathom layers install <pack>" {
		t.Errorf("compile with nothing compiled and no pack = %s %q", s.Status, s.Remedy)
	}

	// A broken switch on a stage that shows no work: off, with its remedy.
	f = pipelineFacts()
	f.RulesServed = Measured(0, "log")
	if s := status(f, StageServe); s.Status != StageOff || s.Remedy != "fathom compile --service pay" {
		t.Errorf("serve with obligations broken and none served = %s %q", s.Status, s.Remedy)
	}

	// A stale index: extract and index are stale — the stage runs on an
	// old copy — with the daemon-routed refresh, not a shell ingest.
	f = pipelineFacts()
	f.Freshness, f.FreshnessRemedy = "stale: 3 commits behind", "fathom post-commit-hook --service pay"
	setCap(&f, CapabilityState{ID: "index", Broken: true, Reason: "the index is behind the repository's current commit (stale: 3 commits behind)", Remedy: "fathom ingest --service pay"})
	for _, id := range []PipelineStageID{StageExtract, StageIndex} {
		if s := status(f, id); s.Status != StageStale || s.Remedy != "fathom post-commit-hook --service pay" || !strings.Contains(s.StatusReason, "behind HEAD") {
			t.Errorf("%s over a stale index = %s %q %q", id, s.Status, s.StatusReason, s.Remedy)
		}
	}
}
