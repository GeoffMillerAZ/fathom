package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// stringSliceFlag is a flag.Value collecting one or more occurrences of a
// repeatable flag (e.g. --module path1 --module path2) into a slice, in
// the order given.
type stringSliceFlag []string

func (f *stringSliceFlag) String() string { return strings.Join(*f, ",") }
func (f *stringSliceFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// runServeHTTP is the testable entry point for the "serve-http" subcommand.
//
// CF-7 (docs/plans/config-experience.md): the same user/project
// config.toml layers compile/providers/smoke read are loaded here too, BEFORE
// flag parsing, and feed flag DEFAULTS ([daemon] table: store, addr,
// call_log, auto_refresh) — a flag on the actual invocation still wins
// (flags > env > project > user > defaults). This is what makes
// compile_layer work on the always-on daemon with zero plist env: a launchd
// plist carries no env at all today, which is exactly the "SILENTLY
// DISABLED" gap the file's grounding names. Restart-to-apply: this load
// happens once, at process start.
func runServeHTTP(args []string, stdout, stderr io.Writer) int {
	loaded, cfgErr := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if cfgErr != nil {
		fmt.Fprintf(stderr, "error: loading config: %v\n", cfgErr)
		return 1
	}

	fs := newFlagSet("serve-http", stderr)
	storePath := fs.String("store", loaded.Config.Daemon.Store, "store path, .sqlite/.db (required; [daemon].store in config.toml, or --store)")
	addr := fs.String("addr", firstNonEmpty(loaded.Config.Daemon.Addr, "127.0.0.1:7575"), "listen address (host:port); loopback by default — surfaces are not a security boundary (ADR-014 amendment)")
	autoRefresh := fs.Bool("auto-refresh", daemonAutoRefreshDefault(loaded.Config.Daemon), "re-ingest a service when its git HEAD drifts from the indexed commit, before answering queries")
	callLogPath := fs.String("call-log", firstNonEmpty(loaded.Config.Daemon.CallLog, "workspace/logs/mcp-calls.jsonl"), "JSONL tool-call log path, resolved against the daemon working directory; empty to disable")
	var modulePaths stringSliceFlag
	fs.Var(&modulePaths, "module", "path to a conforming expert-module store (.sqlite/.db, docs/spec/expert-module-v0.md), mounted READ-ONLY on POST /rpc/modules; repeatable. A non-conforming or unreadable store aborts startup loudly (docs/plans/gke-module-host.md GKE-MH-1) rather than starting up silently degraded.")
	modulesTokenEnv := fs.String("modules-token-env", "", "name of an environment variable holding the bearer token required for POST /rpc/modules (GKE-MH-7); unset = auth disabled (default — matches the loopback-only daemon's existing unauthenticated trust boundary). The token itself is never a flag or config value, only this env var's NAME.")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *storePath == "" {
		fmt.Fprintln(stderr, "error: --store is required")
		fs.Usage()
		return 1
	}

	// GKE-MH-7: resolve the modules bearer token BEFORE opening anything —
	// a --modules-token-env naming an empty/unset environment variable is a
	// misconfiguration that must fail loudly, never silently leave auth
	// disabled on what the operator believes is a protected path.
	modulesToken := ""
	if *modulesTokenEnv != "" {
		modulesToken = os.Getenv(*modulesTokenEnv)
		if modulesToken == "" {
			fmt.Fprintf(stderr, "error: --modules-token-env=%s is set but that environment variable is empty or unset\n", *modulesTokenEnv)
			return 1
		}
	}

	c, err := client.NewConstantClient(client.ModeLocal, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening client: %v\n", err)
		return 1
	}
	defer c.Close()

	// Semantic-layer compilation over HTTP (the compile_layer tool) needs an
	// LLM. Wire one when a provider is configured EITHER the pre-profile way
	// (ANTHROPIC_API_KEY, or FATHOM_LLM_PROVIDER set at all) OR via a
	// config-file profile ([daemon].default_profile, else the top-level
	// default_profile, else no profile at all — matching
	// resolveActiveProfile's own fallback chain). Without any of these,
	// dry-run estimation still works and spending compiles return a clear
	// error. Model: compileModelForProfile (FATHOM_COMPILE_MODEL beats the
	// profile's exact pin, which beats the cheap-tier default — ADR-016 §6).
	daemonProfileName := loaded.Config.Daemon.DefaultProfile
	if daemonProfileName == "" {
		daemonProfileName = loaded.Config.DefaultProfile
	}
	hasProfileConfig := daemonProfileName != "" || len(loaded.Config.Profiles) > 0
	if os.Getenv("ANTHROPIC_API_KEY") != "" || os.Getenv("FATHOM_LLM_PROVIDER") != "" || hasProfileConfig {
		p, _, profErr := config.ResolveProfile(loaded.Config, loaded.Provenance, daemonProfileName)
		if profErr != nil {
			fmt.Fprintf(stderr, "warn: compile_layer disabled: %v\n", profErr)
		} else if llmCl, llmErr := buildLLMClientForProfile(p); llmErr == nil {
			model := compileModelForProfile(p)
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

	// GKE-MH-1: mount every --module store read-only, conformance-checked
	// against LevelL0 (the module host's floor — sufficient for
	// internal/semantic.ObligationsFor and the staleness-report shape; see
	// store.ConformanceLevel's doc for why Server conformance itself has no
	// separate schema footprint to check). A refusal aborts startup loudly
	// — never a partial mount, never a silently-degraded module.
	var moduleHost *modulehost.Host
	if len(modulePaths) > 0 {
		moduleHost = modulehost.NewHost(store.LevelL0)
		for _, p := range modulePaths {
			if _, mErr := moduleHost.Mount(p, ""); mErr != nil {
				fmt.Fprintf(stderr, "error: mounting module %s: %v\n", p, mErr)
				return 1
			}
			fmt.Fprintf(stderr, "serve-http: mounted module %s (%s)\n", p, modulehost.ModuleNameFromPath(p))
		}
	}

	var srv *mcp.Server
	if moduleHost != nil {
		srv = mcp.NewServerWithModules(backend, &fleetAdapter{c: c, stderr: stderr}, logger, moduleHost)
	} else {
		srv = mcp.NewServerWithFleetAndLogger(backend, &fleetAdapter{c: c, stderr: stderr}, logger)
	}
	// BA-9: budget defaults are project-settable. The `budgets` block of
	// <cwd>/.fathom/capabilities.cue (the daemon's working repo) and
	// ~/.fathom/capabilities.cue sizes each tool class in tokens, resolved
	// project → user → built-in inside the server. A broken block is
	// reported loudly and the measured built-in stands for THAT scope —
	// never a silent partial read. Restart-to-apply, like config.toml.
	budgets, budgetErrs := capability.LoadBudgetResolution(capability.DefaultUserPath(), capability.DefaultProjectPath())
	for _, e := range budgetErrs {
		fmt.Fprintf(stderr, "warn: %v — built-in budget defaults stand for that scope\n", e)
	}
	srv.SetBudgets(budgets)
	// GKE-MH-7 (critic finding 2): a configured modules token means this is
	// a hosted, network-reachable deployment — /rpc/modules must serve ONLY
	// its own curated allowlist (never GroupShared's fleet-backend reads or
	// write/spend tools), off-surface calls there must hard-gate (never
	// execute-and-flag), and module tools must be unreachable via any other
	// path (bare /rpc, another surface, a batch sub-call). Set BEFORE
	// ListenAndServe — never toggled while serving.
	if modulesToken != "" {
		srv.SetHostedModules(true)
	}
	handler := newHTTPHandlerWithAuth(srv, c, modulesToken, *addr)

	// Adopt a new binary without a human (selfrestart.go). launchd's
	// KeepAlive is true for this job, so exiting IS a restart — which is
	// what lets an agent complete a deploy end to end, rather than
	// building a fix and then waiting for someone to run `launchctl`.
	if selfRestartDisabled() {
		fmt.Fprintln(stdout, "self-restart: OFF (FATHOM_SELF_RESTART=off) — a deploy will need a manual restart")
	} else if exe, exeErr := os.Executable(); exeErr != nil {
		fmt.Fprintf(stdout, "self-restart: disabled — cannot resolve this process's own path (%v)\n", exeErr)
	} else {
		go watchOwnBinary(exe, stdout, selfRestartPoll, selfRestartCooldown, os.Exit)
	}

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

// firstNonEmpty returns configured if non-empty, else fallback — the small
// helper CF-7 uses to feed a config.toml [daemon] value into a flag's
// default while still falling back to the pre-config hardcoded default when
// no config file (or no such key) is present.
func firstNonEmpty(configured, fallback string) string {
	if configured != "" {
		return configured
	}
	return fallback
}

// daemonAutoRefreshDefault resolves the --auto-refresh flag's default from
// [daemon].auto_refresh: unset (nil, no layer ever set it) keeps the
// pre-config default of true; an explicit false or true in config.toml is
// honored as the new default, still overridable by the flag itself.
func daemonAutoRefreshDefault(d config.Daemon) bool {
	if d.AutoRefresh == nil {
		return true
	}
	return *d.AutoRefresh
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
//	POST /rpc/{surface} — same, scoped to a named tool surface (agent,
//	                      companion, coding, investigation, shared, platform);
//	                      the surface filters tools/list and, by default,
//	                      execute-and-flags off-surface tools/call (ADR-014
//	                      amendment) — hard gating is available behind
//	                      Server.strictSurfaces. agent is the primary
//	                      15-tool surface; companion is a compatible alias.
//	GET  /healthz       — liveness probe, returns "ok"
//	GET  /events        — SSE stream stub (hello event, keep-alive)
//	GET  /metrics       — simple text exposition: uptime and request count
//	GET  /statusz       — daemon health, per-service freshness, channel
//	                      liveness, last compile per layer (Q-25/Q-26,
//	                      docs/design/observability-surfaces.md §8 item 6);
//	                      JSON via Accept: application/json or ?format=json,
//	                      HTML otherwise; gated inside the /rpc/modules
//	                      bearer-token allowlist when hosted (modulesAuthMiddleware)
//	POST /reload        — 200 OK stub; would reload a PlatformStore in production
//
// newHTTPHandlerWithAuth wraps newHTTPHandler with the /rpc/modules (AND,
// Q-26, /statusz) bearer-token gate (GKE-MH-7). modulesToken == "" (the
// default) disables the check entirely — identical behavior to
// newHTTPHandler(srv, c), so every existing call site/test unaffected by
// this row keeps working unchanged.
func newHTTPHandlerWithAuth(srv *mcp.Server, c *client.Client, modulesToken, selfAddr string) http.Handler {
	return modulesAuthMiddleware(newHTTPHandler(srv, c, selfAddr), modulesToken)
}

// modulesAuthMiddleware gates ONLY the /rpc/modules path (and its
// sub-paths) behind a bearer token — every other route (including bare
// /rpc and every other named surface) is untouched: the loopback-only
// daemon's existing unauthenticated trust boundary (docs/mcp-tools.md)
// stays exactly as it is. token == "" disables the check (default OFF —
// auth is a launch requirement for a NETWORK-reachable deployment, which
// does not exist until an operator opts in via --modules-token-env,
// docs/plans/gke-module-host.md's design section).
//
// Comparison is constant-time (crypto/subtle.ConstantTimeCompare) so a
// byte-by-byte mismatch never leaks timing information about how much of
// the token matched. The configured token is never logged or echoed back —
// only a boolean authorized/denied is observable from this middleware, and
// the 401 body never repeats the caller-supplied credential.
func modulesAuthMiddleware(next http.Handler, token string) http.Handler {
	if token == "" {
		return next
	}
	const prefix = "Bearer "
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Q-26 (docs/plans/expert-suite-q.md): /statusz sits INSIDE this
		// same allowlist gate, not outside it — the phase-O critic finding
		// #2 class (GKE-MH-7): a hosted, network-reachable deployment must
		// not leak index/service detail through a route the modules-token
		// gate doesn't cover. Loopback-only default (token == "" above)
		// leaves /statusz fully open, exactly like every other route.
		if r.URL.Path != "/rpc/modules" && !strings.HasPrefix(r.URL.Path, "/rpc/modules/") && r.URL.Path != "/statusz" {
			next.ServeHTTP(w, r)
			return
		}
		got := r.Header.Get("Authorization")
		authorized := strings.HasPrefix(got, prefix) &&
			subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(token)) == 1
		if !authorized {
			w.Header().Set("WWW-Authenticate", `Bearer realm="fathom-modules"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newHTTPHandler(srv *mcp.Server, c *client.Client, selfAddr string) http.Handler {
	state := &httpServerState{startTime: time.Now()}
	mux := http.NewServeMux()

	// GET /statusz (Q-25/Q-26): daemon health, per-service index
	// freshness, channel liveness, last compile per layer. Registered
	// unconditionally here — modulesAuthMiddleware (servehttp.go, wrapped
	// around this handler by newHTTPHandlerWithAuth) is what gates it
	// under hosted mode; this function itself has no notion of a token.
	mux.Handle("/statusz", statuszHandler(c, state.startTime))
	// Observatory (vision 17 / docs/plans/observatory.md OB-4/5/6):
	// loopback value-realization pages over the same primitives the fleet
	// report reads. selfURL lets the model reuse the HTTP-shaped helpers.
	selfURL := "http://" + selfAddr
	mux.Handle("/", landingHandler(selfURL))
	mux.Handle("/assets/app.css", assetsHandler())
	mux.Handle("/observatory", observatoryHandler(selfURL))
	mux.Handle("/observatory/service", observatoryServicePage(selfURL))
	mux.Handle("/observatory/impact", observatoryImpactPage(selfURL))
	mux.Handle("/observatory/session", observatorySessionPage(selfURL))
	mux.Handle("/observatory/story", observatoryStoryPage(selfURL))
	mux.Handle("/observatory/brief", observatoryBriefPage(selfURL))
	mux.Handle("/observatory/wiring/", wiringRoutes(clientWiringRegistry(c), selfURL, time.Now)) // SH-7/SH-8: preview → apply → undo, law-13 gated (observatory_apply.go)

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
		// MCP-Protocol-Version (2025-06-18+ Streamable HTTP): clients send
		// the negotiated revision on every post-initialize request. It is
		// TOLERATED here (RP-12) — never a 400 — and answered with the
		// revision this server would negotiate for it, so a client can see
		// the server's choice on every response, not only on initialize.
		w.Header().Set("MCP-Protocol-Version", mcp.NegotiateProtocolVersion(r.Header.Get("MCP-Protocol-Version")))
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
