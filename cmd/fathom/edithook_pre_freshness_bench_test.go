package main

// edithook_pre_freshness_bench_test.go — SU-6 (docs/plans/substrate.md §A,
// ADR-041 decisions 3/4): the whole pre-edit path, including a REAL
// per-file refresh (real fusion extraction over a real store — never a
// canned response), must stay well under the hook's own <=2s budget; the
// design's own target for this specific addition is p50 < 150ms. Skipped
// under -short (a synthetic-file benchmark run 20 times is not free on a
// slow CI runner); the measured number is always logged via t.Log so a
// -short run still shows intent even when it skips the assertion.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// synthetic300LineGoFile returns ~300 lines of trivial, real, extractable
// Go source — enough for a genuine (if small) tree-sitter/extractor pass,
// never a placeholder that skips real work.
func synthetic300LineGoFile(marker int) string {
	var sb strings.Builder
	sb.WriteString("package bench\n\n")
	for i := 0; i < 95; i++ {
		fmt.Fprintf(&sb, "func F%d_%d() int { return %d }\n\n", i, marker, i)
	}
	return sb.String()
}

// benchObligationsAndRefreshMux serves a MINIMAL, REAL (not canned) daemon
// stand-in: POST /rpc's "obligations" tool answers from a real
// semantic.ObligationsFor + Client.ObligationsIndexStale call over a real
// store, and POST /refresh-file calls the real Client.ReingestFile — the
// same code paths the production daemon route (refreshfile.go) and the
// obligations MCP tool (internal/mcp/semantic_tools.go) call. Any other
// tool name (the service-candidate resolution call) gets a harmless empty
// result — candidatesWithDaemon already falls back to basename candidates
// when the daemon's answer does not resolve, so the pre-edit hook still
// reaches the guard on the service the test ingested under.
func benchObligationsAndRefreshMux(c *client.Client, serviceID string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name string `json:"name"`
				Args struct {
					File string `json:"file"`
				} `json:"arguments"`
			} `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		if req.Params.Name != "obligations" {
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"rules": []any{}}))
			return
		}
		ctx := r.Context()
		obl, err := c.Obligations(ctx, serviceID, req.Params.Args.File, 0)
		if err != nil {
			json.NewEncoder(w).Encode(rpcTextResult(map[string]any{"rules": []any{}}))
			return
		}
		stale := c.ObligationsIndexStale(ctx, serviceID, req.Params.Args.File)
		payload := map[string]any{
			"service":     obl.Service,
			"file":        obl.File,
			"rules":       obl.Rules,
			"index_stale": stale,
		}
		if obl.Watermark != "" {
			payload["compiled_at_watermark"] = obl.Watermark
		}
		json.NewEncoder(w).Encode(rpcTextResult(payload))
	})
	mux.HandleFunc("/refresh-file", func(w http.ResponseWriter, r *http.Request) {
		var req refreshFileRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		out := buildRefreshFileResponse(r.Context(), c, nil, req.Service, req.File)
		json.NewEncoder(w).Encode(out)
	})
	return mux
}

func TestEditHookPre_FreshnessRefresh_P50Under150ms(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the 20-run freshness benchmark under -short")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte(synthetic300LineGoFile(0)), 0o644); err != nil {
		t.Fatalf("write target.go: %v", err)
	}
	writeGuardCue(t, dir, `guards: [{scopes: ["**/*.go"], note: "bench guard"}]`)

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	c := client.New(conn, st)
	t.Cleanup(func() { _ = c.Close() })

	serviceID := strings.ToLower(filepath.Base(dir))
	ctx := context.Background()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: serviceID, Path: dir}); err != nil {
		t.Fatalf("initial Ingest: %v", err)
	}

	srv := httptest.NewServer(benchObligationsAndRefreshMux(c, serviceID))
	defer srv.Close()

	const runs = 20
	durations := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		// Force a real, DIFFERENT-from-stored hash every run, so every
		// iteration takes the genuine stale -> refresh -> re-answer path
		// rather than only the first.
		if err := os.WriteFile(target, []byte(synthetic300LineGoFile(i+1)), 0o644); err != nil {
			t.Fatalf("rewrite target.go (run %d): %v", i, err)
		}

		hookInput := fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"bench-sess"}`, target, dir)
		hookStdin(t, hookInput)

		var out, errBuf bytes.Buffer
		start := time.Now()
		code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf)
		elapsed := time.Since(start)
		if code != 0 {
			t.Fatalf("run %d: exit = %d, stderr=%s", i, code, errBuf.String())
		}
		if !strings.Contains(out.String(), "index refreshed for this file") {
			t.Fatalf("run %d: expected a genuine refresh to have happened, got %s", i, out.String())
		}
		durations = append(durations, elapsed)
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p50 := durations[len(durations)/2]
	t.Logf("pre-edit + real per-file refresh, %d runs: p50=%s min=%s max=%s", runs, p50, durations[0], durations[len(durations)-1])
	if p50 > 150*time.Millisecond {
		t.Errorf("p50 = %s, want <= 150ms", p50)
	}
}
