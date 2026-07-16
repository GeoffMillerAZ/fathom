package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// runServeHTTP is the testable entry point for the "serve-http" subcommand.
func runServeHTTP(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("serve-http", stderr)
	storePath := fs.String("store", "", "DuckDB store path (required)")
	addr := fs.String("addr", ":7575", "listen address (host:port)")
	autoRefresh := fs.Bool("auto-refresh", true, "re-ingest a service when its git HEAD drifts from the indexed commit, before answering queries")
	callLogPath := fs.String("call-log", "workspace/logs/mcp-calls.jsonl", "JSONL tool-call log path, resolved against the daemon working directory; empty to disable")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *storePath == "" {
		fmt.Fprintln(stderr, "error: --store is required")
		fs.Usage()
		return 1
	}

	c, err := client.NewConstantClient(client.ModeLocal, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening client: %v\n", err)
		return 1
	}
	defer c.Close()

	// Semantic-layer compilation over HTTP (the compile_layer tool) needs an
	// LLM. Wire one only when a key is configured; without it, dry-run
	// estimation still works and spending compiles return a clear error.
	// Model override: FATHOM_COMPILE_MODEL (defaults to the cheap tier —
	// obligations compiles are cents-scale by design, ADR-016 §6).
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		if llmCl, llmErr := buildAnthropicClient(); llmErr == nil {
			model := os.Getenv("FATHOM_COMPILE_MODEL")
			if model == "" {
				model = "claude-haiku-4-5-20251001"
			}
			c.WithLLM(llmCl, model, 0.001, 0.005)
			fmt.Fprintf(stderr, "serve-http: compile_layer enabled (model %s)\n", model)
		} else {
			fmt.Fprintf(stderr, "warn: compile_layer disabled: %v\n", llmErr)
		}
	}

	// Default: keep the shared index fresh transparently. A query for a service
	// whose git HEAD has drifted from the indexed commit triggers a hash-gated
	// re-ingest before the answer — so the always-on daemon never serves a stale
	// index without anyone noticing.
	var backend mcp.QueryInterface = c
	if *autoRefresh {
		backend = mcp.NewAutoRefresh(c, stderr)
	}

	// Call logging feeds tool-selection telemetry (which tools each surface
	// actually uses); failure to open the log never blocks serving.
	var logger *mcp.CallLogger
	if *callLogPath != "" {
		f, err := openCallLog(*callLogPath)
		if err != nil {
			fmt.Fprintf(stderr, "warn: call log disabled: %v\n", err)
		} else {
			defer f.Close()
			logger = mcp.NewCallLogger(f)
		}
	}

	srv := mcp.NewServerWithFleetAndLogger(backend, &fleetAdapter{c: c, stderr: stderr}, logger)
	handler := newHTTPHandler(srv)

	fmt.Fprintf(stdout, "serve-http: listening on %s\n", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		fmt.Fprintf(stderr, "error: http serve: %v\n", err)
		return 1
	}
	return 0
}

// httpServerState holds lightweight server-level metrics exposed via GET /metrics.
type httpServerState struct {
	startTime time.Time
	reqCount  atomic.Int64 // incremented on every non-/metrics request
}

// openCallLog opens (creating parent directories as needed) the JSONL
// tool-call log for appending.
func openCallLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// newHTTPHandler returns an http.Handler for the serve-http subcommand.
// It is factored out so tests can drive it via httptest without binding a port.
//
// Routes:
//
//	POST /rpc           — single JSON-RPC 2.0 request/response (full surface)
//	POST /rpc/{surface} — same, scoped to a named tool surface (companion,
//	                      coding, investigation, shared, platform); the
//	                      surface filters tools/list AND gates tools/call
//	GET  /healthz       — liveness probe, returns "ok"
//	GET  /events        — SSE stream stub (hello event, keep-alive)
//	GET  /metrics       — simple text exposition: uptime and request count
//	POST /reload        — 200 OK stub; would reload a PlatformStore in production
func newHTTPHandler(srv *mcp.Server) http.Handler {
	state := &httpServerState{startTime: time.Now()}
	mux := http.NewServeMux()

	// serveRPC handles one JSON-RPC 2.0 message on the named surface.
	serveRPC := func(w http.ResponseWriter, r *http.Request, surface string) {
		state.reqCount.Add(1)
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		const maxBody = 4 * 1024 * 1024 // 4 MB
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		raw := make([]byte, 0, 512)
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				raw = append(raw, buf[:n]...)
			}
			if err != nil {
				break
			}
		}

		// Background, not r.Context(): a client disconnect must not cancel a
		// long-running ingest mid-flight. The scope carries the surface.
		ctx := mcp.WithOrigin(mcp.WithRequestScope(context.Background(), surface, true), r.Header.Get("X-Fathom-Origin"))
		resp, hasResp := srv.HandleRequest(ctx, raw)
		if !hasResp {
			// Notification — no response body.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp) //nolint:errcheck
	}

	// POST /rpc — JSON-RPC 2.0 single-message endpoint, full surface.
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		serveRPC(w, r, "")
	})

	// POST /rpc/{surface} — the same endpoint scoped to a named surface.
	mux.HandleFunc("/rpc/{surface}", func(w http.ResponseWriter, r *http.Request) {
		surface := r.PathValue("surface")
		if !mcp.ValidSurface(surface) {
			state.reqCount.Add(1)
			http.Error(w, fmt.Sprintf("unknown surface %q; valid: %s (POST /rpc serves the full surface)",
				surface, strings.Join(mcp.SurfaceNames(), ", ")), http.StatusNotFound)
			return
		}
		serveRPC(w, r, surface)
	})

	// Anything else under /rpc/ (trailing slash, extra segments) gets the same
	// diagnostic instead of the mux's bare 404.
	mux.HandleFunc("/rpc/", func(w http.ResponseWriter, r *http.Request) {
		state.reqCount.Add(1)
		http.Error(w, fmt.Sprintf("unknown RPC path %q; valid: POST /rpc (full surface) or POST /rpc/{%s}",
			r.URL.Path, strings.Join(mcp.SurfaceNames(), "|")), http.StatusNotFound)
	})

	// GET /healthz — liveness probe.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		state.reqCount.Add(1)
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// GET /events — SSE stream stub: sends a hello event then keeps the
	// connection open (flushing every write).  Clients can listen here for
	// future server-sent notifications.
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		state.reqCount.Add(1)
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Send the initial hello event.
		fmt.Fprint(w, "event: hello\ndata: {\"server\":\"fathom\"}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Keep the stream alive until the client disconnects.
		<-r.Context().Done()
	})

	// GET /metrics — simple text exposition of server-level metrics.
	// Format: one "key value\n" line per metric.
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		uptime := time.Since(state.startTime).Seconds()
		reqCount := state.reqCount.Load()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "fathom_uptime_seconds %.3f\n", uptime)
		fmt.Fprintf(w, "fathom_requests_total %d\n", reqCount)
	})

	// POST /reload — 200 OK stub; in production this would call
	// PlatformStore.Reload() to re-scan the store directory.
	mux.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		state.reqCount.Add(1)
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	return mux
}
