package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

// doctor_typed.go — BP-36 (docs/plans/bench-pyramid.md): the `typed_edges`
// row.
//
// On 2026-09-12 the launchd daemon had been re-ingesting this repo with a
// broken Go environment for an unknown length of time: every auto-refresh
// logged `fusion: typed-edge pass "fathom" failed to load: … GOPATH entry
// is relative` to /tmp/fathom-http.err, fell back to syntax-only edges, and
// then reported a perfectly healthy index. Doctor's 35 checks said nothing.
// staleness_report said nothing. The CLI's own post-commit ingest, run in a
// user shell with GOPATH exported, type-checked fine — so the tests measured
// a graph the fleet was not being served.
//
// ADR-027's rule is that an instrument may not degrade toward healthy. A
// stderr line in a log file nobody tails is not an instrument. This row is:
// it asks the daemon, through staleness_report's typed_edges block, whether
// the typed pass ran for this service, and FAILS with the named reason when
// it did not.
//
// Deliberate verdict shape:
//   - FAIL when the daemon's last ingest reported the typed pass failed —
//     the graph being served is weaker than the one the tests measure, which
//     is wrong wiring, not a style preference.
//   - PASS with the package count when it ran.
//   - SKIP, with the reason, for everything doctor cannot honestly judge:
//     a non-Go repo, an unreachable daemon, a daemon that has not ingested
//     this service since it started. `known: false` is NOT a pass.
func checkTypedEdges(hc *http.Client, baseURL, root, explicitSvc string, daemonUp bool) doctorCheck {
	const name = "typed_edges"
	const remedy = "fathom daemon install --force  (rewrites the launchd plist with GOPATH/GOCACHE/GOMODCACHE), then: launchctl kickstart -k gui/$(id -u)/<label>"

	// Non-Go repos have no typed pass to run — enrichTypedEdges gates on a
	// go.mod at the corpus root for exactly this reason.
	if !fileExists(filepath.Join(root, "go.mod")) {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "not a Go module (no go.mod at the corpus root) — the typed Go call-edge pass does not apply here"}
	}
	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon not reachable — the typed-edge pass outcome is the daemon's own fact and cannot be read from here", Remedy: daemonRestartHint("")}
	}

	svc := explicitSvc
	if svc == "" {
		// Resolve like checkObligationsServed does: candidatesWithDaemon
		// prepends the daemon-recorded corpus-path identity ahead of the
		// directory-basename guess, so a repo whose dir basename names no
		// (or the wrong) indexed service still resolves to the service that
		// actually records this root (O30: the sextant-dir/fathom-service
		// split, live on this very repo, was defeating this row's own
		// staleness_report lookup while obligations_served resolved fine).
		cands := candidatesWithDaemon(hc, baseURL, root)
		if len(cands) == 0 {
			return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no service identity resolvable for this repo"}
		}
		svc = strings.ToLower(cands[0].name)
	}

	inner, err := callMCPTool(hc, baseURL, "staleness_report", map[string]any{"service": svc})
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("staleness_report(%s) did not answer: %v", svc, err), Remedy: remedy}
	}
	var payload struct {
		TypedEdges *struct {
			Known    bool   `json:"known"`
			Packages int    `json:"packages"`
			Failed   bool   `json:"failed"`
			Reason   string `json:"reason"`
		} `json:"typed_edges"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("staleness_report(%s) returned an unreadable typed_edges block: %v", svc, err)}
	}
	if payload.TypedEdges == nil {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("the daemon serving %s predates the typed_edges block (BP-35) — upgrade it before trusting this row", baseURL),
			Remedy: "task deploy",
		}
	}
	if !payload.TypedEdges.Known {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("this daemon process has not ingested %q since it started — no typed-edge outcome to report (not the same as a passing one)", svc),
			Remedy: "refresh the index (the `ingest` MCP tool, or `fathom ingest --service " + svc + "`), then re-run doctor",
		}
	}
	if payload.TypedEdges.Failed {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("the daemon's last typed-edge pass for %q FAILED (%s) — the index it serves has syntax-only call edges where typed ones are expected, while every other surface reports healthy",
				svc, payload.TypedEdges.Reason),
			Remedy: remedy,
		}
	}
	return doctorCheck{
		Name: name, Status: doctorPass,
		Reason: fmt.Sprintf("the daemon's last typed-edge pass for %q resolved %d package(s)", svc, payload.TypedEdges.Packages),
	}
}
