package main

// edithook_blast.go — SR-60 (surface reckoning F-12, design doc §6): the
// pre-edit hook's blast-radius trailer. review_diff already carries the
// blast radius summary as its own DimImpact dimension (internal/analysis/
// review/engine.go); this file is the OTHER half of the row — a one-line
// "N callers, M tests reach this" appended to the guard payload itself, so
// the agent sees the blast radius BEFORE the edit, not only after review.
//
// Additive and non-blocking by construction: a slow or failing
// blast_radius call degrades to silence, exactly like fetchMemories
// (edithook.go) degrades on its own daemon call — never blocking or
// erroring the rest of the guard payload the way a failed obligations
// fetch does.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// blastRadiusTimeout bounds the pre-edit hook's blast-radius lookup —
// additive to preHookBudget (renderPreEdit's own <=2s target for the
// guard-match + obligations round trip), the same "extra, bounded, never
// blocking" shape refreshTimeout already gives the SU-6 refresh call.
const blastRadiusTimeout = 500 * time.Millisecond

// editHookBlastLine renders the pre-edit hook's blast-radius trailer:
// "N callers, M tests reach this" — "" when the edited file has no
// callers at all (SR-60's own gate: "when the edited symbol has
// callers"). Never a leading-zero claim; silence over "0 callers reach
// this". Pure formatting — fetchBlastRadius resolves the counts.
func editHookBlastLine(callers, tests int) string {
	if callers <= 0 {
		return ""
	}
	line := plural(callers, "caller")
	if tests > 0 {
		line += ", " + plural(tests, "test")
	}
	return line + " reach this"
}

// fetchBlastRadius calls blast_radius (kind:"change", the default) scoped
// to file's own directory-relative path, returning the transitive symbol
// count (total_symbols) and the test count it reaches (len(tests)).
// ok=false on any error, non-200 or malformed response — the pre-edit
// hook degrades to silence on a failed or slow lookup rather than block
// or error the guard payload (the fetchMemories precedent, edithook.go).
func fetchBlastRadius(baseURL, service, file string, timeout time.Duration) (callers, tests int, ok bool) {
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "blast_radius",
			"arguments": map[string]any{
				"service":       service,
				"changed_files": []string{file},
			},
		},
	})
	if err != nil {
		return 0, 0, false
	}

	hc := &http.Client{Timeout: timeout}
	req, err := originHeader(baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return 0, 0, false
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, false
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, 0, false
	}

	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return 0, 0, false
	}

	var payload struct {
		TotalSymbols int      `json:"total_symbols"`
		Tests        []string `json:"tests"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return 0, 0, false
	}
	return payload.TotalSymbols, len(payload.Tests), true
}

// blastLineFor is renderPreEdit's own call: fetch, then format. Isolated
// from renderPreEdit's own control flow so the call is trivially
// unit-testable end to end without threading extra params through
// renderPreEdit's signature, and so a future caller (the post-edit hook,
// say) can reuse it without duplicating the fetch-then-format shape.
func blastLineFor(baseURL, service, file string) string {
	callers, tests, ok := fetchBlastRadius(baseURL, service, file, blastRadiusTimeout)
	if !ok {
		return ""
	}
	return editHookBlastLine(callers, tests)
}
