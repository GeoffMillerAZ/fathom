package main

// doctor_modules.go — PC-11 (docs/plans/proof-credibility.md §1.2): the
// binding rows used to report "list_modules: RPC error: tool not found" —
// a doctor row naming a tool as missing when the tool exists. It exists in
// the registry only while a module store is mounted (internal/mcp/server.go
// registers registerModuleTools when `modules != nil`), and it is served on
// the full surface (POST /rpc) and the modules surface, never the agent
// surface. So the real reason for "not found" is "nothing is mounted", and
// the row now says that. SR-17 (surface reckoning F-68) removed
// list_modules outright, so on a current daemon tools/list never advertises
// it and these rows always skip naming that reason — the design's
// "always-absent row"; an older daemon that still serves it is still read.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// The audit follow-up: the translation alone still sent the doomed call,
// once per doctor run — 338 "tool not found: list_modules" rows in the call
// log. moduleListForDoctor now asks tools/list first and never calls a tool
// the daemon does not advertise.

// errModulesUnserved is the honest reason list_modules answers "tool not
// found": the daemon mounted no module store at start.
var errModulesUnserved = errors.New("not served by this daemon: list_modules was removed from the agent surface, and a daemon predating it registered it only when a --module store was mounted, so there is nothing to cross-check over MCP")

// moduleListForDoctor calls list_modules on the surface that serves it (the
// full surface at POST /rpc — the same base URL doctor's other daemon rows
// use; the agent surface's 17-tool allowlist does not carry it) and returns
// module → store path. A "tool not found" answer is translated to
// errModulesUnserved; any other failure is returned as-is.
func moduleListForDoctor(hc *http.Client, baseURL string) (map[string]string, error) {
	if advertised, known := toolAdvertised(hc, baseURL, "list_modules"); known && !advertised {
		return nil, errModulesUnserved
	}
	inner, err := callMCPTool(hc, baseURL, "list_modules", map[string]any{})
	if err != nil {
		if strings.Contains(err.Error(), "tool not found") {
			return nil, errModulesUnserved
		}
		return nil, err
	}
	var payload struct {
		Modules []struct {
			Module string `json:"module"`
			Path   string `json:"path"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return nil, fmt.Errorf("parse list_modules result: %w", err)
	}
	out := make(map[string]string, len(payload.Modules))
	for _, m := range payload.Modules {
		if _, exists := out[m.Module]; !exists {
			out[m.Module] = m.Path
		}
	}
	return out, nil
}

// toolAdvertised reports whether the full surface's tools/list names tool.
// known is false when tools/list gave no usable answer (unreachable, an
// error, no tools array) — the caller then falls back to calling the tool
// and reading its error, exactly as before.
func toolAdvertised(hc *http.Client, baseURL, tool string) (advertised, known bool) {
	req, err := originHeader(baseURL, bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)))
	if err != nil {
		return false, false
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
	if err != nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
		return false, false
	}
	var env struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &env) != nil || len(env.Result.Tools) == 0 {
		return false, false
	}
	for _, t := range env.Result.Tools {
		if t.Name == tool {
			return true, true
		}
	}
	return false, true
}
