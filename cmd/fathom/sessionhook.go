package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// cmdSessionHook implements the "session-hook" subcommand: a Claude Code
// SessionStart hook that primes each session to use fathom's index before
// falling back to Grep/Read.
//
// Telemetry across real fleet sessions showed fathom is dramatically
// under-used — agents reach for shell grep 10×+ more than fathom even where
// the index exists, because "index first" lives only in passive docs. This hook
// makes it active: at session start it pings the shared serve-http daemon, maps
// the project dir to a service, and emits a one-line reminder (or an honest
// "not available / not indexed" signal) into the session context.
//
// It prints the SessionStart hook JSON to stdout and always exits 0 (a context
// hook must never block session start). HTTP calls use a short timeout so a slow
// or down daemon can't stall startup.
func cmdSessionHook(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if fathomChannelsDisabled() {
		return 0 // FATHOM_CHANNELS=off (channels.go): total silence, before any other work
	}
	rpcOrigin = "session-hook"
	emitBeat("session-hook", "fired", nil)
	fs := newFlagSet("session-hook", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	dir := fs.String("dir", "", "project dir (default: $CLAUDE_PROJECT_DIR, else cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// session_id (docs/design/production-tokenomics.md §6): read
	// best-effort from the SessionStart hook JSON — the same envelope
	// shape edit-hook/stop-hook already read. Absent/unreadable/malformed
	// stdin degrades to sessionID="" (no session-scoped tagging), never a
	// failure of the session brief itself.
	var sessionID string
	if stdin != nil {
		raw, _ := io.ReadAll(io.LimitReader(stdin, 1<<16))
		var hook struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal(raw, &hook) == nil {
			sessionID = hook.SessionID
		}
	}

	projDir := *dir
	if projDir == "" {
		projDir = os.Getenv("CLAUDE_PROJECT_DIR")
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}

	ctx := buildSessionContext(*url, projDir)
	emitPayloadCost("session-brief", ctx, 0, "", sessionID)

	_ = json.NewEncoder(stdout).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "SessionStart",
			"additionalContext": ctx,
		},
	})
	return 0
}

func defaultFathomURL() string {
	if u := os.Getenv("FATHOM_HTTP"); u != "" {
		return u
	}
	return "http://localhost:7575"
}

type svcInfo struct {
	symbols int
	langs   string
	path    string // recorded corpus root; "" when the daemon has none
}

// buildSessionContext returns the additionalContext line for the project at
// projDir. Service identity tries the dir basename first, then the enclosing
// git repo's basename (projects rooted in a subdirectory like <repo>/deploy).
func buildSessionContext(baseURL, projDir string) string {
	hc := &http.Client{Timeout: 2 * time.Second}

	resp, err := hc.Get(baseURL + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return fmt.Sprintf("⚠️ fathom code-intelligence daemon (%s) is not responding — its MCP tools are unavailable this session, so Grep/Read discovery is expected. Restart it with: launchctl kickstart -k gui/$(id -u)/com.geoffmilleraz.fathom-http", baseURL)
	}
	resp.Body.Close()

	if st, active := loadArmState(time.Now()); active {
		// An active OFF arm must be LOUD (the O15 lesson): the session
		// opens by naming it, its bound, and the undo.
		bound := st.Until
		if st.SessionsRemaining != nil {
			bound += fmt.Sprintf(" / %d session(s) left", *st.SessionsRemaining)
		}
		defer decrementArmSessions()
		return fmt.Sprintf("🔬 fathom OFF arm active (measurement, vision 14 §2): channels are silent this session — no obligations, no guards, no verify gate. Bound: %s. This is deliberate counterfactual data collection; end it early with `fathom arm on`.", bound)
	}

	svcs, svcErr := fetchServices(hc, baseURL)
	if svcErr != nil {
		// O2: enumeration failure is NOT "not indexed". The daemon is healthy
		// (healthz just passed) but list_services failed — saying "this repo
		// is not an indexed service" here would be a confident false
		// instruction the agent has no reason to disprove. Degrade honestly.
		return fmt.Sprintf("⚠️ fathom daemon is healthy but service enumeration failed (%v) — index state is unknown this session; the MCP tools may still work, so try them before falling back to Grep/Read. If this repeats, check GET %s/statusz.", svcErr, baseURL)
	}
	cands := serviceCandidates(projDir)
	if resolved, ok := resolveFromServices(svcs, projDir); ok {
		cands = mergeCandidates(resolved, cands)
	}
	for _, cand := range cands {
		lookup := strings.ToLower(cand.name)
		info, ok := svcs[lookup]
		if !ok {
			continue
		}
		langs := ""
		if info.langs != "" {
			langs = ", " + info.langs
		}
		ctxLine := fmt.Sprintf("✅ fathom code-intelligence is live — this repo is the indexed service %q (%d symbols%s). Resolve where / what / who-calls via fathom BEFORE any Grep/Read sweep: find_symbol, query_symbols, find_callers, outline, repo_map, references. Read files to *edit* them, not to discover them. The index auto-refreshes on git-HEAD drift; force a refresh with the ingest tool ({\"service\":%q}).", lookup, info.symbols, langs, lookup)
		// FR-4 (docs/plans/exploration-ledger.md): teach the FIRST query
		// with a LIVE symbol from this repo's own index — the first-query
		// experience decides adoption; one miss loses the session to grep.
		// Degraded-branch discipline holds: any failure omits the line.
		if ex := liveExampleLine(hc, baseURL, lookup); ex != "" {
			ctxLine += " " + ex
		}
		// The wiring tripwire (vision 14 §7 tier-1, IO-19): every drifted
		// session opens by telling its agent the exact remedy — detection
		// is free and universal; nothing mutates here.
		if d := analyzeWiring(projDir); len(d.Missing) > 0 || len(d.StaleCmds) > 0 {
			ctxLine += fmt.Sprintf(" ⚠️ wiring drift: %d missing hook template entr(ies), %d stale binary path(s) — run `fathom fleet reconcile --apply` (or /fathom:rewire) to reconcile in place.", len(d.Missing), len(d.StaleCmds))
		}
		return ctxLine
	}

	repo := filepath.Base(strings.TrimRight(projDir, "/"))
	known := make([]string, 0, len(svcs))
	for k := range svcs {
		known = append(known, k)
	}
	sort.Strings(known)
	indexed := strings.Join(known, ", ")
	if len(known) == 0 {
		indexed = "none yet"
	}
	return fmt.Sprintf("ℹ️ fathom code-intelligence is live, but this repo (%q) is not an indexed service — its symbols aren't in the index, so Grep/Read discovery is expected here. Indexed services: %s. (Add this repo with the ingest tool: {\"service\":\"<name>\",\"path\":\"<absolute repo root>\"}.)", repo, indexed)
}

// fetchServices calls the list_services MCP tool over HTTP and returns a map
// keyed by lower-cased service ID, plus an error when ENUMERATION failed —
// callers must distinguish "failed to enumerate" from "zero services" (O2:
// collapsing them once rendered a confident false not-indexed instruction
// for an indexed repo). One retry with its own timeout, decoupled from the
// caller's healthz-sized client.
func fetchServices(hc *http.Client, baseURL string) (map[string]svcInfo, error) {
	m, err := fetchServicesOnce(hc, baseURL)
	if err == nil {
		return m, nil
	}
	retryClient := &http.Client{Timeout: 4 * time.Second}
	if hc != nil && hc.Transport != nil {
		retryClient.Transport = hc.Transport
	}
	return fetchServicesOnce(retryClient, baseURL)
}

func fetchServicesOnce(hc *http.Client, baseURL string) (map[string]svcInfo, error) {
	out := map[string]svcInfo{}
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}`)
	req, err := originHeader(baseURL, body)
	if err != nil {
		return out, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return out, fmt.Errorf("list_services: HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return out, err
	}
	// Unwrap: {"result":{"content":[{"text":"<json>"}]}} or {"result":{services:...}}
	var env struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Services []rawSvc `json:"services"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return out, fmt.Errorf("list_services: %w", err)
	}
	svcs := env.Result.Services
	if len(svcs) == 0 && len(env.Result.Content) > 0 {
		var inner struct {
			Services []rawSvc `json:"services"`
		}
		if err := json.Unmarshal([]byte(env.Result.Content[0].Text), &inner); err == nil {
			svcs = inner.Services
		}
	}
	for _, s := range svcs {
		if err := core.ValidServiceID(s.Service); err != nil {
			continue // skip malformed/path-like service IDs (e.g. "../app-kit")
		}
		out[strings.ToLower(s.Service)] = svcInfo{symbols: s.SymbolCount, langs: joinLangs(s.Languages), path: s.Path}
	}
	return out, nil
}

// liveExampleLine returns the "first query to try" teaching line (FR-4,
// docs/plans/exploration-ledger.md): one REAL symbol fetched from the
// service's own index over the agent surface, so the session's first
// fathom call cannot miss. "" on ANY failure — the hook must never break
// or slow session start, so this degrades silently (the same discipline
// as every other branch in this file), with its own 2s ceiling on top of
// whatever the caller's client allows.
func liveExampleLine(hc *http.Client, baseURL, service string) string {
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_symbols","arguments":{"service":%q,"concern":"main","top":1}}}`, service)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/rpc/agent", strings.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	if rpcOrigin != "" {
		req.Header.Set(originHeaderName, rpcOrigin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := hc.Do(req.WithContext(ctx))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}
	var env struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &env) != nil || len(env.Result.Content) == 0 {
		return ""
	}
	var inner struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	}
	if json.Unmarshal([]byte(env.Result.Content[0].Text), &inner) != nil || len(inner.Nodes) == 0 || inner.Nodes[0].Name == "" {
		return ""
	}
	return fmt.Sprintf("first query to try: find_symbol {\"service\": %q, \"name\": %q} — the index answers where/what/who-calls without reading files.", service, inner.Nodes[0].Name)
}

type rawSvc struct {
	Service     string   `json:"service"`
	SymbolCount int      `json:"symbol_count"`
	Languages   []string `json:"languages"`
	Path        string   `json:"path"` // recorded corpus root (authoritative repo↔service identity)
}

func joinLangs(l []string) string {
	if len(l) == 0 {
		return ""
	}
	return strings.Join(l, "/")
}
