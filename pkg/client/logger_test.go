package client_test

// logger_test.go — DG-4 (diagnostics sweep): pkg/client.Client.WithLogger
// threads a *dlog.Logger down into the underlying fusion.Pipeline
// (internal/fusion's own TestPipeline_SetLogger_RoutesTypedEdgeSummaryThroughLogger
// proves the routing itself; this test proves the public Client-level
// entry point wires to it without panicking and returns c for chaining,
// matching WithLLM/WithDecision's own contract).

import (
	"io"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestClient_WithLogger_ReturnsClientForChaining(t *testing.T) {
	c, err := client.NewConstantClient(client.ModeFaux, "")
	if err != nil {
		t.Fatalf("NewConstantClient: %v", err)
	}
	defer c.Close()

	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	got := c.WithLogger(log)
	if got != c {
		t.Error("WithLogger must return the same *Client for chaining, like WithLLM/WithDecision")
	}
}
