package main

// identity.go — GET /identity (docs/design/responsible-tenant.md §11.3,
// RT-133): which indexed service a directory is, answered from the
// daemon's own registry. Every hook's identity lookup used to list every
// service (list_services over /rpc, the fleet's most-called operation) and
// resolve client-side; /identity takes the candidate roots (the directory
// and its work-tree root) and returns the one resolution, with the same
// deterministic rules (resolveFromServicesCands, then a basename match).
// Hooks reach it through the cached resolver (identcache.go), and
// SubagentStart reuses its parent's SessionStart resolution from the
// session stamp — zero RPCs.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// How an identity was matched.
const (
	identityByRoot     = "root"     // an indexed service records this corpus root
	identityByBasename = "basename" // only a candidate's basename names an indexed service
)

// identityResult is GET /identity's answer.
type identityResult struct {
	// Schema marks a real /identity answer (1): a stand-in that answers
	// every path with some other JSON is a daemon without the route.
	Schema    int    `json:"identity_schema"`
	Service   string `json:"service"` // "" when nothing matched
	Root      string `json:"root,omitempty"`
	MatchedBy string `json:"matched_by,omitempty"`
	Symbols   int    `json:"symbols,omitempty"`
	CountErr  string `json:"count_error,omitempty"`
	Langs     string `json:"languages,omitempty"`
	Path      string `json:"path,omitempty"` // the service's recorded corpus root
}

func (r identityResult) info() svcInfo {
	return svcInfo{symbols: r.Symbols, countErr: r.CountErr, langs: r.Langs, path: r.Path}
}

// resolveIdentity is the resolution /identity serves: a corpus-root match
// first (resolveFromServicesCands' deterministic ranking), then the first
// candidate whose basename is an indexed service.
func resolveIdentity(svcs map[string]svcInfo, cands []serviceCandidate) identityResult {
	res := func(name, root, by string) identityResult {
		i := svcs[name]
		return identityResult{Schema: 1, Service: name, Root: root, MatchedBy: by, Symbols: i.symbols, CountErr: i.countErr, Langs: i.langs, Path: i.path}
	}
	if c, ok := resolveFromServicesCands(svcs, cands); ok {
		return res(strings.ToLower(c.name), c.root, identityByRoot)
	}
	for _, c := range cands {
		if _, ok := svcs[strings.ToLower(c.name)]; ok {
			return res(strings.ToLower(c.name), c.root, identityByBasename)
		}
	}
	return identityResult{Schema: 1}
}

// identityCandidates builds the candidates from the query's paths, in the
// order given (the directory, then its work-tree root).
func identityCandidates(paths []string) ([]serviceCandidate, error) {
	if len(paths) == 0 || len(paths) > 4 {
		return nil, fmt.Errorf("give 1 to 4 path parameters (the directory, then its git work-tree root)")
	}
	var out []serviceCandidate
	for _, p := range paths {
		p = strings.TrimRight(p, "/")
		if !filepath.IsAbs(p) {
			return nil, fmt.Errorf("path %q is not absolute", p)
		}
		out = append(out, serviceCandidate{name: filepath.Base(p), root: p})
	}
	return out, nil
}

// handleIdentity is GET /identity?path=<dir>[&path=<toplevel>]. services
// reads the registry (serve-http's in-process registry: memoized, never a
// whole-table read per call).
func handleIdentity(services func(*http.Request) (map[string]svcInfo, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cands, err := identityCandidates(r.URL.Query()["path"])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		svcs, err := services(r)
		if err != nil {
			w.Header().Set("Retry-After", "5")
			http.Error(w, "service registry unavailable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resolveIdentity(svcs, cands))
	})
}

// daemonIdentityServices is the serving daemon's /identity registry read.
func daemonIdentityServices(r *http.Request) (map[string]svcInfo, error) {
	reg := inProcessReg.Load()
	if reg == nil || reg.services == nil {
		return nil, errors.New("no in-process registry in this process")
	}
	return reg.services(r.Context())
}

// errIdentityRouteMissing marks a daemon that predates GET /identity.
var errIdentityRouteMissing = errors.New("daemon has no /identity route")

// lookupIdentity resolves dir: in process when this process is the daemon,
// else one GET /identity with the directory and its work-tree root.
func lookupIdentity(hc *http.Client, baseURL, dir string) (identityResult, error) {
	dir = strings.TrimRight(dir, "/")
	paths := []string{dir}
	if top := gitToplevel(dir); top != "" && top != dir {
		paths = append(paths, top)
	}
	if reg := inProcessReg.Load(); reg != nil && reg.services != nil && sameDaemonURL(baseURL, reg.self) {
		cands, err := identityCandidates(paths)
		if err != nil {
			return identityResult{}, err
		}
		svcs, err := reg.services(context.Background())
		if err != nil {
			return identityResult{}, err
		}
		return resolveIdentity(svcs, cands), nil
	}
	if hc == nil {
		hc = &http.Client{Timeout: 2 * time.Second}
	}
	q := url.Values{}
	for _, p := range paths {
		q.Add("path", p)
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/identity?"+q.Encode(), nil)
	if err != nil {
		return identityResult{}, err
	}
	if rpcOrigin != "" {
		req.Header.Set(originHeaderName, rpcOrigin)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return identityResult{}, fmt.Errorf("GET %s/identity: %w", baseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return identityResult{}, fmt.Errorf("reading /identity: %w", err)
	}
	switch {
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// Any 4xx is a daemon (or stand-in) that does not serve this route
		// (probeIndexed's rule): the full list answers instead.
		return identityResult{}, errIdentityRouteMissing
	case resp.StatusCode != http.StatusOK:
		return identityResult{}, fmt.Errorf("GET /identity: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out identityResult
	if err := json.Unmarshal(raw, &out); err != nil || out.Schema != 1 {
		// A 200 that is not this shape is a stand-in that does not serve
		// the route (a console fallback page): treat it as missing.
		return identityResult{}, errIdentityRouteMissing
	}
	return out, nil
}
