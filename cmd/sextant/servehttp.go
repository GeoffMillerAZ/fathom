package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/sextant/internal/mcp"
	"github.com/geoffmilleraz/sextant/pkg/client"
)

// runServeHTTP is the testable entry point for the "serve-http" subcommand.
func runServeHTTP(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("serve-http", stderr)
	storePath := fs.String("store", "", "DuckDB store path (required)")
	addr := fs.String("addr", ":7575", "listen address (host:port)")

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

	srv := mcp.NewServerWithFleet(c, &fleetAdapter{c: c, stderr: stderr})
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

// newHTTPHandler returns an http.Handler for the serve-http subcommand.
// It is factored out so tests can drive it via httptest without binding a port.
//
// Routes:
//
//	POST /rpc     — single JSON-RPC 2.0 request/response
//	GET  /healthz — liveness probe, returns "ok"
//	GET  /events  — SSE stream stub (hello event, keep-alive)
//	GET  /metrics — simple text exposition: uptime and request count
//	POST /reload  — 200 OK stub; would reload a PlatformStore in production
func newHTTPHandler(srv *mcp.Server) http.Handler {
	state := &httpServerState{startTime: time.Now()}
	mux := http.NewServeMux()

	// POST /rpc — JSON-RPC 2.0 single-message endpoint.
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
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

		resp, hasResp := srv.HandleRequest(context.Background(), raw)
		if !hasResp {
			// Notification — no response body.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(resp) //nolint:errcheck
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
		fmt.Fprint(w, "event: hello\ndata: {\"server\":\"sextant\"}\n\n")
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
		fmt.Fprintf(w, "sextant_uptime_seconds %.3f\n", uptime)
		fmt.Fprintf(w, "sextant_requests_total %d\n", reqCount)
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
