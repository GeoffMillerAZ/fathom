package main

// findings_summary.go — GET /findings/summary?service=NAME (FL-21, docs/
// vision/30-findings-loop.md §3.4, ADR-040 decision 5): a plain HTTP route
// — never an MCP tool; the agent-facing surface does not grow (ADR-040
// consequences) — the session hook (sessionhook.go) reads for its "open
// findings: N" brief line.
//
// A root-scope obligations lookup (RootObligationsFor) would badly
// undercount here: a finding's Scope is its own cited file, never a
// wildcard/root-reaching glob, so rootScopeMatch would almost never admit
// one. This route reads the service's finding-kind claims directly
// instead — the same ClaimsForService(service, "findings") call
// cmd/fathom/findings.go's `fathom findings status` already makes.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// findingsSummary is /findings/summary's response shape.
type findingsSummary struct {
	Service string `json:"service"`
	Count   int    `json:"count"`
	// Oldest is the earliest CompiledAt among served finding-kind claims
	// (YYYY-MM-DD) — an "as of" proxy for observed-at: core.SemanticClaim
	// carries no per-claim ObservedAt of its own today; that provenance
	// field lives on the SERVED semantic.Obligation payload (FL-20's
	// Obligation.ObservedAt), populated by the external-import join a
	// concurrent wave (FL-1..FL-19) owns. "" when Count is 0.
	Oldest string `json:"oldest,omitempty"`
}

// findingsSummaryHandler serves GET /findings/summary?service=NAME.
func findingsSummaryHandler(c *client.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		service := r.URL.Query().Get("service")
		if service == "" {
			http.Error(w, "service is required", http.StatusBadRequest)
			return
		}
		out := buildFindingsSummary(r.Context(), c, service)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

// buildFindingsSummary computes the summary from service's served
// finding-kind claims — factored out so a test can call it directly
// without standing up an httptest server.
func buildFindingsSummary(ctx context.Context, c *client.Client, service string) findingsSummary {
	out := findingsSummary{Service: service}
	claims, err := c.ClaimsForService(ctx, service, findingsLayerName)
	if err != nil {
		return out // honest zero on a lookup failure — never a fabricated count
	}
	var oldest time.Time
	for _, cl := range claims {
		if !cl.Status.Served() {
			continue
		}
		out.Count++
		if !cl.CompiledAt.IsZero() && (oldest.IsZero() || cl.CompiledAt.Before(oldest)) {
			oldest = cl.CompiledAt
		}
	}
	if out.Count > 0 && !oldest.IsZero() {
		out.Oldest = oldest.UTC().Format("2006-01-02")
	}
	return out
}

// findingsBriefLine fetches /findings/summary for service and renders
// the session brief's one-line form: "open findings: N (oldest
// YYYY-MM-DD)" when Oldest is known, else bare "open findings: N". ""
// (silent) on any fetch failure or a zero count — this hook's own
// degraded-branch discipline: a slow or unreachable summary is never a
// reason to fail or slow session start.
//
// DEVIATION from docs/vision/30-findings-loop.md §3.4's literal "open
// findings: N (k producers; oldest 2026-09-02)" shape: the producer count
// is dropped. core.SemanticClaim carries no per-claim Producer today — that
// field lives only on the SERVED semantic.Obligation payload (FL-20's
// Producer field), populated by the external-import join a concurrent wave
// (FL-1..FL-19, the provenance substrate) owns. Fabricating a producer
// count from unrelated claim fields would be worse than omitting it.
func findingsBriefLine(hc *http.Client, baseURL, service string) string {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/findings/summary?service="+service, nil)
	if err != nil {
		return ""
	}
	resp, err := hc.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return ""
	}
	var out findingsSummary
	if json.Unmarshal(raw, &out) != nil || out.Count == 0 {
		return ""
	}
	if out.Oldest != "" {
		return fmt.Sprintf("open findings: %d (oldest %s).", out.Count, out.Oldest)
	}
	return fmt.Sprintf("open findings: %d.", out.Count)
}
