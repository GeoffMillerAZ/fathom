package main

import (
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
func cmdSessionHook(args []string, stdout, stderr io.Writer) int {
	rpcOrigin = "session-hook"
	emitBeat("session-hook", "fired", nil)
	fs := newFlagSet("session-hook", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	dir := fs.String("dir", "", "project dir (default: $CLAUDE_PROJECT_DIR, else cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	projDir := *dir
	if projDir == "" {
		projDir = os.Getenv("CLAUDE_PROJECT_DIR")
	}
	if projDir == "" {
		projDir, _ = os.Getwd()
	}

	ctx := buildSessionContext(*url, projDir)

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

	svcs := fetchServices(hc, baseURL)
	cands := serviceCandidates(projDir)
	if resolved, ok := resolveFromServices(svcs, projDir); ok {
		deduped := []serviceCandidate{resolved}
		for _, c := range cands {
			if !strings.EqualFold(c.name, resolved.name) {
				deduped = append(deduped, c)
			}
		}
		cands = deduped
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
		return fmt.Sprintf("✅ fathom code-intelligence is live — this repo is the indexed service %q (%d symbols%s). Resolve where / what / who-calls via fathom BEFORE any Grep/Read sweep: find_symbol, query_symbols, find_callers, outline, repo_map, references. Read files to *edit* them, not to discover them. The index auto-refreshes on git-HEAD drift; force a refresh with the ingest tool ({\"service\":%q}).", lookup, info.symbols, langs, lookup)
	}

	repo := filepath.Base(strings.TrimRight(projDir, "/"))
	known := make([]string, 0, len(svcs))
	for k := range svcs {
		known = append(known, k)
	}
	sort.Strings(known)
	return fmt.Sprintf("ℹ️ fathom code-intelligence is live, but this repo (%q) is not an indexed service — its symbols aren't in the index, so Grep/Read discovery is expected here. Indexed services: %s. (Add this repo with the ingest tool: {\"service\":\"<name>\",\"path\":\"<absolute repo root>\"}.)", repo, strings.Join(known, ", "))
}

// fetchServices calls the list_services MCP tool over HTTP and returns a map
// keyed by lower-cased service ID. Best-effort: returns an empty map on any error.
func fetchServices(hc *http.Client, baseURL string) map[string]svcInfo {
	out := map[string]svcInfo{}
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}`)
	req, err := originHeader(baseURL, body)
	if err != nil {
		return out
	}
	resp, err := hc.Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return out
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
		return out
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
	return out
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
