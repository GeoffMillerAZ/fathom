package decision_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// composeLog collects the reports the Observe hooks of composeRegistry
// receive, so a test reads every node's report, inner nodes included.
type composeLog struct {
	mu       sync.Mutex
	cascade  []decision.CascadeReport
	fallback []decision.FallbackReport
	cached   []decision.CachedReport
}

func (l *composeLog) cascades() []decision.CascadeReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]decision.CascadeReport(nil), l.cascade...)
}

func (l *composeLog) fallbacks() []decision.FallbackReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]decision.FallbackReport(nil), l.fallback...)
}

func (l *composeLog) cacheds() []decision.CachedReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]decision.CachedReport(nil), l.cached...)
}

// composeRegistry returns a Registry naming deciders, one MemoryCacheStore
// as "memory", transcripts, and one Observe hook per report type, each
// appending to log.
func composeRegistry(log *composeLog, deciders map[string]decision.Decider, transcripts map[string]decision.Transcript) decision.Registry {
	return decision.Registry{
		Deciders:    deciders,
		Stores:      map[string]decision.CacheStore{"memory": decision.NewMemoryCacheStore()},
		Transcripts: transcripts,
		CascadeObservers: map[string]func(decision.CascadeReport){"cascade-log": func(r decision.CascadeReport) {
			log.mu.Lock()
			defer log.mu.Unlock()
			log.cascade = append(log.cascade, r)
		}},
		FallbackObservers: map[string]func(decision.FallbackReport){"fallback-log": func(r decision.FallbackReport) {
			log.mu.Lock()
			defer log.mu.Unlock()
			log.fallback = append(log.fallback, r)
		}},
		CachedObservers: map[string]func(decision.CachedReport){"cache-log": func(r decision.CachedReport) {
			log.mu.Lock()
			defer log.mu.Unlock()
			log.cached = append(log.cached, r)
		}},
	}
}

// loadPolicy decodes the JSON policy at path.
func loadPolicy(t *testing.T, path string) decision.Policy {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return decodePolicy(t, string(data))
}

// decodePolicy decodes a JSON policy.
func decodePolicy(t *testing.T, data string) decision.Policy {
	t.Helper()
	var p decision.Policy
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		t.Fatalf("decode policy: %v", err)
	}
	return p
}

// composeRequest asks three predicate questions about one State.
func composeRequest() decision.Request {
	return decision.Request{State: "line 3 stopped after a grinding noise", Questions: []decision.Question{decorQuestion("q1"), decorQuestion("q2"), decorQuestion("q3")}}
}

func TestComposeFromConfigBuildsCascadeFallbackCache(t *testing.T) {
	ctx := context.Background()
	// cheap resolves q1, answers q2 below the policy's 0.8 threshold and
	// abstains on q3; a fails; b answers what reaches it.
	cheap := decision.NewMockDecider("cheap", decision.MockEntry{Response: decision.Response{
		Provider: "cheap", Model: "cheap-1",
		Answers: map[string]decision.Answer{"q1": decorPredicate("q1", 0.95), "q2": decorPredicate("q2", 0.6)},
		Invalid: map[string]string{"q3": "not sure"},
	}})
	a := decision.NewMockDecider("a", decision.MockEntry{Err: errors.New("provider down")})
	b := decision.NewMockDecider("b",
		decision.MockEntry{Response: decision.Response{
			Provider: "b", Model: "b-1",
			Answers: map[string]decision.Answer{"q2": decorPredicate("q2", 0.9), "q3": decorPredicate("q3", 0.1)},
			Invalid: map[string]string{},
		}},
		decision.MockEntry{Response: decision.Response{
			Provider: "b", Model: "b-1",
			Answers: map[string]decision.Answer{"q1": decorPredicate("q1", 0.85), "q2": decorPredicate("q2", 0.9), "q3": decorPredicate("q3", 0.1)},
			Invalid: map[string]string{},
		}},
	)
	log := &composeLog{}
	reg := composeRegistry(log, map[string]decision.Decider{"cheap": cheap, "a": a, "b": b}, nil)

	composed, err := decision.ComposeFromConfig(loadPolicy(t, "testdata/compose_policy.json"), reg)
	if err != nil {
		t.Fatalf("ComposeFromConfig: %v", err)
	}
	if got := composed.Names(); !reflect.DeepEqual(got, []string{"backup", "judgment-cache", "tiers"}) {
		t.Fatalf("Names = %v", got)
	}
	if tiers, ok := composed.Node("tiers"); !ok || !reflect.DeepEqual(tiers.(*decision.Cascade).Stages(), []string{"cheap", "backup"}) {
		t.Fatalf("Node(tiers) = %v, %v; want a cascade over cheap then backup", tiers, ok)
	}
	req := composeRequest()

	resp, err := composed.Decide(ctx, req)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	for id, want := range map[string]float64{"q1": 0.95, "q2": 0.9, "q3": 0.1} {
		if got := resp.Answers[id].Noul; got != want {
			t.Errorf("answer %s Noul = %v, want %v", id, got, want)
		}
	}
	if resp.Provider != "tiers" || resp.Model != "" {
		t.Errorf("merged Provider/Model = %q/%q, want the cascade's name and no model", resp.Provider, resp.Model)
	}

	// Escalation on low confidence (q2) and abstain (q3), reported to the
	// cascade node's Observe hook.
	cascades := log.cascades()
	if len(cascades) != 1 {
		t.Fatalf("cascade reports = %d, want 1", len(cascades))
	}
	cr := cascades[0]
	if src := cr.Sources["q1"]; src.Stage != 0 || src.Provider != "cheap" {
		t.Errorf("q1 source = %+v, want stage 0 cheap", src)
	}
	for _, id := range []string{"q2", "q3"} {
		if src := cr.Sources[id]; src.Stage != 1 || src.Provider != "b" || src.Model != "b-1" {
			t.Errorf("%s source = %+v, want stage 1 b/b-1", id, src)
		}
	}
	if why := cr.Steps[0].Escalated["q2"]; !strings.Contains(why, "confidence 0.6 below 0.8") {
		t.Errorf("q2 escalation = %q, want low confidence against the policy's threshold", why)
	}
	if why := cr.Steps[0].Escalated["q3"]; why != "not sure" {
		t.Errorf("q3 escalation = %q, want the abstention", why)
	}
	if got := decorIDs(cheap.Calls()[0]); !reflect.DeepEqual(got, []string{"q1", "q2", "q3"}) {
		t.Errorf("cheap asked %v", got)
	}

	// Fallback on error: a failed, b answered, and the fallback node's
	// Observe hook was told.
	fallbacks := log.fallbacks()
	if len(fallbacks) != 1 || fallbacks[0].Answered != 1 || len(fallbacks[0].Failures) != 1 || fallbacks[0].Failures[0].Name != "a" {
		t.Fatalf("fallback reports = %+v, want b answering after a failed", fallbacks)
	}
	for name, m := range map[string]*decision.MockDecider{"a": a, "b": b} {
		calls := m.Calls()
		if len(calls) != 1 || !reflect.DeepEqual(decorIDs(calls[0]), []string{"q2", "q3"}) {
			t.Errorf("%s calls = %d (%v), want one with the escalated q2 and q3", name, len(calls), calls)
		}
	}

	// A second identical Request is a cache hit: the root's report, read
	// through Root, names every question a hit, and no provider is asked.
	root, ok := composed.Root().(*decision.Cached)
	if !ok {
		t.Fatalf("Root = %T, want *decision.Cached", composed.Root())
	}
	resp2, report, err := root.DecideReport(ctx, req)
	if err != nil {
		t.Fatalf("second Decide: %v", err)
	}
	if !reflect.DeepEqual(report.Hits, []string{"q1", "q2", "q3"}) || len(report.Misses) != 0 {
		t.Errorf("second call hits/misses = %v/%v, want every question a hit", report.Hits, report.Misses)
	}
	if !reflect.DeepEqual(resp2.Answers, resp.Answers) {
		t.Errorf("cached answers = %+v, want %+v", resp2.Answers, resp.Answers)
	}
	if n := len(cheap.Calls()) + len(a.Calls()) + len(b.Calls()); n != 3 {
		t.Errorf("provider calls after the hit = %d, want 3 (none for the hit)", n)
	}
	if cached := log.cacheds(); len(cached) != 2 || !reflect.DeepEqual(cached[0].Misses, []string{"q1", "q2", "q3"}) {
		t.Errorf("cache-log reports = %+v, want a miss then a hit", cached)
	}

	// The same registry under a reordered policy answers from a different
	// provider, with no code change: the fallback is now the first stage
	// and tries b first, so b resolves q1 and neither cheap nor a is asked.
	reordered, err := decision.ComposeFromConfig(loadPolicy(t, "testdata/compose_policy_reordered.json"), reg)
	if err != nil {
		t.Fatalf("ComposeFromConfig(reordered): %v", err)
	}
	resp3, err := reordered.Decide(ctx, req)
	if err != nil {
		t.Fatalf("reordered Decide: %v", err)
	}
	if resp3.Provider != "b" || resp3.Model != "b-1" {
		t.Errorf("reordered Provider/Model = %q/%q, want b/b-1", resp3.Provider, resp3.Model)
	}
	cascades = log.cascades()
	if first, again := cascades[0].Sources["q1"].Provider, cascades[len(cascades)-1].Sources["q1"].Provider; first != "cheap" || again != "b" {
		t.Errorf("q1 answered by %q then %q, want cheap then b", first, again)
	}
	if got := resp3.Answers["q1"].Noul; got != 0.85 {
		t.Errorf("reordered q1 Noul = %v, want b's 0.85", got)
	}
	if len(cheap.Calls()) != 1 || len(a.Calls()) != 1 || len(b.Calls()) != 2 {
		t.Errorf("calls cheap/a/b = %d/%d/%d, want 1/1/2", len(cheap.Calls()), len(a.Calls()), len(b.Calls()))
	}
}

func TestComposeFromConfigRefusesWithNodePath(t *testing.T) {
	cases := []struct {
		name     string
		policy   string
		wantPath string
		wantMsg  string
	}{
		{"unknown node type", `{"cascade": {"stages": [{"provider": "cheap"}, {"ensemble": {}}]}}`, "policy.cascade.stages[1]", `unknown node type "ensemble"`},
		{"node type key is case-sensitive", `{"Cascade": {"stages": [{"provider": "cheap"}]}}`, "policy", `unknown node type "Cascade"`},
		{"unknown provider", `{"fallback": {"observe": "fallback-log", "deciders": [{"provider": "a"}, {"provider": "gpt"}]}}`, "policy.fallback.deciders[1]", `unknown provider "gpt"`},
		{"nil provider", `{"provider": "broken"}`, "policy", `provider "broken" is nil`},
		{"unknown store", `{"cache": {"store": "redis", "template_version": "v1", "inner": {"provider": "cheap"}}}`, "policy.cache", `unknown store "redis"`},
		{"missing store", `{"cache": {"template_version": "v1", "inner": {"provider": "cheap"}}}`, "policy.cache", "store is required"},
		{"nil store", `{"cache": {"store": "broken", "template_version": "v1", "inner": {"provider": "cheap"}}}`, "policy.cache", `store "broken" is nil`},
		{"unknown transcript", `{"replay": {"transcript": "yesterday"}}`, "policy.replay", `unknown transcript "yesterday"`},
		{"missing transcript", `{"replay": {}}`, "policy.replay", "transcript is required"},
		{"bad transcript", `{"replay": {"transcript": "old"}}`, "policy.replay", "transcript version 0"},
		{"unknown observe hook", `{"cascade": {"observe": "nope", "stages": [{"provider": "cheap"}]}}`, "policy.cascade", `unknown observe hook "nope"`},
		{"observe hook of another type", `{"cache": {"store": "memory", "template_version": "v1", "observe": "cascade-log", "inner": {"provider": "cheap"}}}`, "policy.cache", `unknown observe hook "cascade-log"`},
		{"nil observe hook", `{"cascade": {"observe": "broken", "stages": [{"provider": "cheap"}]}}`, "policy.cascade", `observe hook "broken" is nil`},
		{"empty stage list", `{"cascade": {"stages": []}}`, "policy.cascade", "stages is empty"},
		{"absent stage list", `{"cascade": {}}`, "policy.cascade", "stages is empty"},
		{"empty fallback list", `{"fallback": {"observe": "fallback-log", "deciders": []}}`, "policy.fallback", "deciders is empty"},
		{"fallback without observe", `{"fallback": {"deciders": [{"provider": "a"}]}}`, "policy.fallback", "observe is required"},
		{"threshold above 1", `{"cascade": {"escalate_below": 1.5, "stages": [{"provider": "cheap"}]}}`, "policy.cascade", "escalate_below 1.5 is outside [0,1]"},
		{"threshold below 0", `{"cascade": {"escalate_below": -0.1, "stages": [{"provider": "cheap"}]}}`, "policy.cascade", "escalate_below -0.1 is outside [0,1]"},
		{"cache without template version", `{"cache": {"store": "memory", "inner": {"provider": "cheap"}}}`, "policy.cache", "template_version is required"},
		{"cache without inner", `{"cache": {"store": "memory", "template_version": "v1"}}`, "policy.cache", "inner is required"},
		{"record without name", `{"record": {"inner": {"provider": "cheap"}}}`, "policy.record", "name is required"},
		{"record without inner", `{"record": {"name": "rec"}}`, "policy.record", "inner is required"},
		{"no node type", `{}`, "policy", "sets no node type"},
		{"two node types", `{"provider": "cheap", "cascade": {"stages": [{"provider": "a"}]}}`, "policy", "sets 2 node types (provider, cascade)"},
		{"unknown field", `{"cascade": {"escalate_belw": 0.2, "stages": [{"provider": "cheap"}]}}`, "policy.cascade", `unknown field "escalate_belw"`},
		{"duplicate name", `{"cascade": {"name": "x", "stages": [{"fallback": {"name": "x", "observe": "fallback-log", "deciders": [{"provider": "a"}]}}]}}`, "policy.cascade.stages[0].fallback", `name "x" is already used by policy.cascade`},
		{"nested path", `{"cache": {"store": "memory", "template_version": "v1", "inner": {"cascade": {"stages": [{"provider": "cheap"}, {"fallback": {"observe": "fallback-log", "deciders": []}}]}}}}`, "policy.cache.inner.cascade.stages[1].fallback", "deciders is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &composeLog{}
			deciders := map[string]decision.Decider{
				"cheap":  decision.NewMockDecider("cheap"),
				"a":      decision.NewMockDecider("a"),
				"broken": (*decision.MockDecider)(nil),
			}
			reg := composeRegistry(log, deciders, map[string]decision.Transcript{"old": {Version: 0}})
			reg.Stores["broken"] = (*decision.MemoryCacheStore)(nil)
			reg.CascadeObservers["broken"] = nil

			composed, err := decision.ComposeFromConfig(decodePolicy(t, tc.policy), reg)
			if err == nil {
				t.Fatalf("ComposeFromConfig = %v, nil; want a refusal", composed)
			}
			if composed != nil {
				t.Errorf("ComposeFromConfig returned %v alongside its refusal", composed)
			}
			var ce *decision.ComposeError
			if !errors.As(err, &ce) {
				t.Fatalf("error %v (%T) is not a *ComposeError", err, err)
			}
			if ce.Path != tc.wantPath {
				t.Errorf("Path = %q, want %q (error: %v)", ce.Path, tc.wantPath, err)
			}
			if !strings.Contains(err.Error(), tc.wantPath+": ") || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %q, want the path %q and %q", err, tc.wantPath, tc.wantMsg)
			}
		})
	}
}

func TestComposeFromConfigRefusesGoBuiltThresholdNaN(t *testing.T) {
	nan := func() float64 { var zero float64; return zero / zero }()
	reg := decision.Registry{Deciders: map[string]decision.Decider{"cheap": decision.NewMockDecider("cheap")}}
	p := decision.Policy{Cascade: &decision.CascadePolicy{EscalateBelow: &nan, Stages: []decision.Policy{{Provider: "cheap"}}}}
	_, err := decision.ComposeFromConfig(p, reg)
	var ce *decision.ComposeError
	if !errors.As(err, &ce) || ce.Path != "policy.cascade" || !strings.Contains(err.Error(), "outside [0,1]") {
		t.Fatalf("err = %v, want a refusal at policy.cascade", err)
	}
}

func TestComposeFromConfigEscalateBelowZeroNeverEscalatesOnConfidence(t *testing.T) {
	ctx := context.Background()
	req := decision.Request{State: "s", Questions: []decision.Question{decorChoiceQuestion("q1")}}
	// Confidence((3*0.4-1)/2) = 0.1: below the default threshold, not
	// below an explicit 0.
	low := decision.MockEntry{Response: decision.Response{
		Provider: "cheap", Model: "cheap-1",
		Answers: map[string]decision.Answer{"q1": decorChoice("q1", "a", map[string]float64{"a": 0.4, "b": 0.3, "c": 0.3})},
		Invalid: map[string]string{},
	}}
	for _, tc := range []struct {
		name       string
		policy     string
		wantStages int
	}{
		{"explicit zero keeps the low answer", `{"cascade": {"escalate_below": 0, "stages": [{"provider": "cheap"}, {"provider": "b"}]}}`, 1},
		{"absent means DefaultEscalateBelow", `{"cascade": {"stages": [{"provider": "cheap"}, {"provider": "b"}]}}`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cheap := decision.NewMockDecider("cheap", low)
			b := decision.NewMockDecider("b")
			composed, err := decision.ComposeFromConfig(decodePolicy(t, tc.policy), decision.Registry{Deciders: map[string]decision.Decider{"cheap": cheap, "b": b}})
			if err != nil {
				t.Fatal(err)
			}
			_, report, err := composed.Root().(*decision.Cascade).DecideReport(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			if len(report.Steps) != tc.wantStages {
				t.Errorf("stages consulted = %d, want %d (report %+v)", len(report.Steps), tc.wantStages, report)
			}
		})
	}
}

func TestComposeFromConfigKeepsCapabilitiesTruthful(t *testing.T) {
	declared := func(name string, cost decision.CostClass, latency decision.LatencyClass) *decision.MockDecider {
		m := decision.NewMockDecider(name)
		m.SetCapabilities(decision.Capabilities{
			Declared: true, Kinds: []decision.Kind{decision.KindPredicate, decision.KindChoice},
			MaxOptions: 8, SingleRoundTrip: true, Latency: latency, Cost: cost,
			ConfidenceSemantics: decision.ConfidenceSelfReported,
		})
		return m
	}
	cheap := declared("cheap", decision.CostLow, decision.LatencySubSecond)
	a := declared("a", decision.CostMedium, decision.LatencySeconds)
	b := declared("b", decision.CostHigh, decision.LatencySeconds)
	reg := composeRegistry(&composeLog{}, map[string]decision.Decider{"cheap": cheap, "a": a, "b": b}, nil)
	composed, err := decision.ComposeFromConfig(loadPolicy(t, "testdata/compose_policy.json"), reg)
	if err != nil {
		t.Fatal(err)
	}

	// The same tree built by hand declares exactly what the composed one does.
	fallback, err := decision.NewFallback(decision.FallbackOptions{Deciders: []decision.Decider{a, b}, Observe: func(decision.FallbackReport) {}})
	if err != nil {
		t.Fatal(err)
	}
	cascade, err := decision.NewCascade(decision.CascadeOptions{Stages: []decision.Decider{cheap, fallback}, EscalateBelow: 0.8})
	if err != nil {
		t.Fatal(err)
	}
	byHand, err := decision.NewCached(cascade, decision.CachedOptions{Store: decision.NewMemoryCacheStore(), TemplateVersion: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	got, want := decision.CapabilitiesOf(composed), decision.CapabilitiesOf(byHand)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("composed Capabilities = %+v, want the hand-built tree's %+v", got, want)
	}
	if got.Cost != decision.CostHigh || got.Latency != decision.LatencySeconds || !reflect.DeepEqual(got, decision.CapabilitiesOf(composed.Root())) {
		t.Errorf("composed Capabilities = %+v, want the worst cost and latency of its leaves, as its root declares", got)
	}

	// One undeclared leaf makes the composed tree undeclared.
	reg.Deciders["b"] = decision.NewMockDecider("b")
	undeclared, err := decision.ComposeFromConfig(loadPolicy(t, "testdata/compose_policy.json"), reg)
	if err != nil {
		t.Fatal(err)
	}
	if decision.CapabilitiesOf(undeclared).Declared {
		t.Errorf("a tree over an undeclared leaf declares %+v", decision.CapabilitiesOf(undeclared))
	}
}

func TestComposeFromConfigRecordsThenReplays(t *testing.T) {
	ctx := context.Background()
	req := composeRequest()
	cheap := decision.NewMockDecider("cheap", decision.MockEntry{Response: decision.Response{
		Provider: "cheap", Model: "cheap-1",
		Answers: map[string]decision.Answer{"q1": decorPredicate("q1", 0.95), "q2": decorPredicate("q2", 0.2), "q3": decorPredicate("q3", 0.7)},
		Invalid: map[string]string{},
	}})
	recording, err := decision.ComposeFromConfig(decodePolicy(t, `{"record": {"name": "rec", "inner": {"provider": "cheap"}}}`),
		decision.Registry{Deciders: map[string]decision.Decider{"cheap": cheap}})
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := recording.Decide(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	node, ok := recording.Node("rec")
	if !ok {
		t.Fatal(`Node("rec") not found`)
	}
	transcript := node.(*decision.Recorder).Transcript()

	replaying, err := decision.ComposeFromConfig(decodePolicy(t, `{"replay": {"name": "offline", "transcript": "run-1"}}`),
		decision.Registry{Transcripts: map[string]decision.Transcript{"run-1": transcript}})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := replaying.Decide(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, recorded) {
		t.Errorf("replayed %+v, want the recorded %+v", replayed, recorded)
	}
	if n := len(cheap.Calls()); n != 1 {
		t.Errorf("cheap calls = %d, want 1 (the replay asks no provider)", n)
	}
	if replaying.Name() != "offline" {
		t.Errorf("Name = %q, want the root's name", replaying.Name())
	}
}

func TestPolicyJSONRoundTrips(t *testing.T) {
	data, err := os.ReadFile("testdata/compose_policy.json")
	if err != nil {
		t.Fatal(err)
	}
	p := loadPolicy(t, "testdata/compose_policy.json")
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %s, want the fixture", out)
	}
}
