package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// TestDecisionPortWiringSkipsUnderRefuse (RT-65, vision 36 §8): with the
// daemon at its refuse rung, POST /route makes no decider call and says so,
// and the obligations re-rank's decider (gatedDecider) returns the skip as
// its error without calling the provider.
func TestDecisionPortWiringSkipsUnderRefuse(t *testing.T) {
	b, _ := tenancy.ResolveBudget(tenancy.Config{Budget: tenancy.GiB}, tenancy.HostFacts{RAM: 16 * tenancy.GiB, NumCPU: 4, Managed: true})
	srv := mcp.NewServer(nil)
	srv.SetGate(tenancy.NewGate(tenancy.GateConfig{Budget: b, Mode: tenancy.ModeEnforce, InFlight: tenancy.NewInFlight(nil),
		Level: func() tenancy.Level { return tenancy.LevelRefuse }}))
	decisionAdmit.Store(&decisionAdmitter{admit: srv.AdmitDecision})
	defer decisionAdmit.Store(nil)

	var buf bytes.Buffer
	mock := decision.NewMockDecider("mock", decision.MockEntry{Response: decision.Response{Provider: "mock",
		Answers: map[string]decision.Answer{"verb": {Choice: "find_symbol", Confidence: 0.99}}}})
	ports := &decide.Ports{Decider: mock, Cfg: config.Decide{Provider: "llm", Sites: []string{decide.SiteRoute}}, Recorder: decide.NewRecorder(&buf, time.Now, "v")}
	_, out := postRoute(t, routeHandler(ports), "find the symbol Foo")
	if !strings.Contains(out.Error, "decision port skipped") || out.Verb != "" {
		t.Fatalf("/route under refuse: %+v", out)
	}
	if len(mock.Calls()) != 0 {
		t.Fatalf("the provider was called %d times under refuse", len(mock.Calls()))
	}

	gd := gatedDecider{inner: mock, site: "obligations-rerank"}
	if gd.Name() != "mock" {
		t.Fatalf("gatedDecider hides the provider's name: %q", gd.Name())
	}
	if _, err := gd.Decide(context.Background(), decision.Request{}); err == nil || !strings.Contains(err.Error(), "decision port skipped") {
		t.Fatalf("re-rank under refuse: %v", err)
	}
	if len(mock.Calls()) != 0 {
		t.Fatal("the re-rank reached the provider under refuse")
	}
}
