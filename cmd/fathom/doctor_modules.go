package main

// doctor_modules.go — PC-11 (docs/plans/proof-credibility.md §1.2): the
// binding rows used to report "list_modules: RPC error: tool not found" —
// a doctor row naming a tool as missing when the tool exists. It exists in
// the registry only while a module store is mounted (internal/mcp/server.go
// registers registerModuleTools when `modules != nil`), and it is served on
// the full surface (POST /rpc) and the modules surface, never the agent
// surface. So the real reason for "not found" is "nothing is mounted", and
// the row now says that.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// errModulesUnserved is the honest reason list_modules answers "tool not
// found": the daemon mounted no module store at start.
var errModulesUnserved = errors.New("not served by this daemon: no module store was mounted at start (serve-http --module <store>) — the tool registers only when one is, so there is nothing to cross-check")

// moduleListForDoctor calls list_modules on the surface that serves it (the
// full surface at POST /rpc — the same base URL doctor's other daemon rows
// use; the agent surface's 17-tool allowlist does not carry it) and returns
// module → store path. A "tool not found" answer is translated to
// errModulesUnserved; any other failure is returned as-is.
func moduleListForDoctor(hc *http.Client, baseURL string) (map[string]string, error) {
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
