package main

// indexed.go — GET /indexed?service=<id> (RT-80, docs/design/
// responsible-tenant.md §4.4): the meta-only "is service X indexed" probe.
// Before it, verify, the stop and edit hooks, plans and the console asked by
// calling list_services — every service's symbol count and languages — to
// answer one yes/no question.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// indexedResponse is GET /indexed's body.
type indexedResponse struct {
	Service string `json:"service"`
	Indexed bool   `json:"indexed"`
}

// indexedHandler serves GET /indexed?service=<id> from client.IsIndexed.
func indexedHandler(c *client.Client) http.HandlerFunc {
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
		indexed, resolved, err := c.IsIndexed(r.Context(), service)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if resolved == "" {
			resolved = service
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(indexedResponse{Service: resolved, Indexed: indexed})
	}
}

// errIndexedRouteMissing marks a daemon that predates GET /indexed.
var errIndexedRouteMissing = fmt.Errorf("daemon has no /indexed route")

// probeIndexed asks the daemon's GET /indexed.
func probeIndexed(hc *http.Client, baseURL, service string) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/indexed?service="+url.QueryEscape(service), nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	if rpcOrigin != "" {
		req.Header.Set(originHeaderName, rpcOrigin)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false, fmt.Errorf("GET %s/indexed: %w", baseURL, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return false, fmt.Errorf("reading /indexed response: %w", err)
	}
	// Any 4xx is a daemon (or stand-in) that does not serve this route —
	// the console's catch-all 404, a 405, a stub's 400 — so the caller asks
	// the probe it has always asked.
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return false, errIndexedRouteMissing
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("GET %s/indexed: %s: %s", baseURL, resp.Status, strings.TrimSpace(string(raw)))
	}
	// A body without an "indexed" field is not this route's answer (a
	// proxy or a catch-all handler that predates it): treat as missing.
	var out struct {
		Indexed *bool `json:"indexed"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Indexed == nil {
		return false, errIndexedRouteMissing
	}
	return *out.Indexed, nil
}
