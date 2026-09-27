package main

// refreshfile.go — POST /refresh-file {service, file} (SU-5/SU-6, docs/
// plans/substrate.md §A, ADR-041 decisions 3 and 4): the daemon route the
// pre-edit hook (edithook_pre.go, SU-6) calls when the edited file's
// on-disk content hash differs from the index, so the hook answers
// obligations from a just-refreshed picture of that one file instead of a
// stale one. Loopback only, like every other route here (docs/mcp-tools.md)
// — no auth beyond what modulesAuthMiddleware already gates elsewhere.
import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// refreshFileRequest is POST /refresh-file's JSON body.
type refreshFileRequest struct {
	Service string `json:"service"`
	File    string `json:"file"` // repo-relative, forward-slash
}

// refreshFileResponse mirrors fusion.ReingestReport for the wire.
type refreshFileResponse struct {
	Refreshed       bool `json:"refreshed"`
	Symbols         int  `json:"symbols"`
	TypedEdgesStale bool `json:"typed_edges_stale"`
	// GraphPatched is true when the daemon's memoized call graph was patched
	// in place for this file's package (SU-4) — the answer a hook can cite
	// for "the next query sees this edit" without a full rebuild.
	GraphPatched bool `json:"graph_patched"`
	// Error carries a non-fatal failure reason (e.g. the file could not be
	// read) alongside a 200 — a refresh that could not happen is still an
	// answer, never a silent empty one; the hook's own staleness line (SU-6)
	// reads this to say "refresh failed: <reason>" instead of pretending
	// the index is current.
	Error string `json:"error,omitempty"`
	// Skipped names why a file was checked but not re-extracted without an
	// error — "oversize (N bytes; over the 2097152-byte cap; not read)" (RT-27, I38).
	Skipped string `json:"skipped,omitempty"`
}

// graphPatcher is the daemon's memoized-graph seam (SU-4): the MCP server
// implements it (internal/mcp/graph_patch.go). nil means "no memo to keep
// fresh" — a test, or a serve path without an MCP server.
type graphPatcher interface {
	HasCachedGraph(service string) bool
	PatchPackageGraph(service, pkg string, nodes []core.SymbolNode, typed []core.Edge)
	DropCachedGraphs(service string)
}

// refreshFileHandler serves POST /refresh-file. q is the daemon's refresh
// queue (RT-97): while a whole-service refresh of the service is queued or
// running, the file is left to it — no per-file write races the ingest —
// and the answer says so. queue (nil, or returning nil): no queue.
func refreshFileHandler(c *client.Client, patcher graphPatcher, queue func() *refresh.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req refreshFileRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.Service == "" || req.File == "" {
			http.Error(w, "service and file are required", http.StatusBadRequest)
			return
		}
		// RT-41 (§2.1): a refresh writes the store; in distress it is
		// refused with the named reason.
		if why, on := tenancy.StoreDistressNow(time.Now()); on {
			w.Header().Set("Retry-After", "60")
			http.Error(w, why+" — /refresh-file writes are refused until it clears", http.StatusServiceUnavailable)
			return
		}
		if q := refreshQueueOf(queue); q != nil {
			if st := q.StateOf(req.Service); st == refresh.StateQueued || st == refresh.StateRunning {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(refreshFileResponse{Skipped: "whole-service refresh " + string(st) + " — it covers this file; the index is stale until it completes"})
				return
			}
		}
		out := buildRefreshFileResponse(r.Context(), c, patcher, req.Service, req.File)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

// buildRefreshFileResponse calls Client.ReingestFile and renders its
// report — factored out so a test can call it directly without standing
// up an httptest server. A ReingestFile error is reported IN the 200
// response body (Error field) rather than as an HTTP error status: the
// request itself was well-formed, and the hook's own degraded-branch
// discipline (SU-6) wants a normal response it can read Error off of, not
// a transport-level failure to distinguish from an actual network problem.
func buildRefreshFileResponse(ctx context.Context, c *client.Client, patcher graphPatcher, service, file string) refreshFileResponse {
	report, err := c.ReingestFile(ctx, service, file)
	if err != nil {
		// RT-41 (§2.1): a write's SQLITE_FULL or SQLITE_IOERR is store
		// distress — later refreshes and ingests are refused, named.
		tenancy.NoteStoreError(err, time.Now())
		return refreshFileResponse{Error: err.Error()}
	}
	out := refreshFileResponse{
		Refreshed:       report.Refreshed,
		Symbols:         report.Symbols,
		TypedEdgesStale: report.TypedEdgesStale,
		Skipped:         report.Skipped,
	}
	// SU-4: a re-ingest replaces the file's rows without moving the store
	// watermark the daemon's graph memo is keyed by, so the memo must be
	// patched (or dropped) here or the next query answers from the
	// pre-refresh graph (O99). Only when something is memoized at all —
	// otherwise the next query builds fresh from current data anyway.
	// An oversize file whose old rows were purged (report.Purged) moves the
	// graph's inputs just as a refresh does: its package lookup now finds
	// no rows, so the memo is dropped rather than left serving them.
	if (report.Refreshed || report.Purged) && patcher != nil && patcher.HasCachedGraph(service) {
		out.GraphPatched = patchGraphForFile(ctx, c, patcher, service, file)
	}
	return out
}

// patchGraphForFile patches the memoized graph for every package the
// refreshed file's symbols belong to — by the node's Package field, which is
// what Graph.ReplacePackage selects on — with that package's complete
// current node set, typed edges left alone (they are the typed pass's to
// replace; TypedEdgesStale says so). A file whose fresh extraction produced
// no symbols names no package to patch, so the memo is dropped instead: the
// next query rebuilds, and a stale graph is never served.
//
// RT-27: the file's packages come from its own rows (source_ref) and each
// package's node set from that package's rows — never the service's whole
// symbol table.
func patchGraphForFile(ctx context.Context, c *client.Client, patcher graphPatcher, service, file string) bool {
	file = filepath.ToSlash(file)
	pkgs, err := c.SourcePackages(ctx, service, file)
	if err != nil || len(pkgs) == 0 {
		patcher.DropCachedGraphs(service)
		return false
	}
	nodes, err := c.PackageSymbols(ctx, service, pkgs)
	if err != nil {
		patcher.DropCachedGraphs(service)
		return false
	}
	byPkg := make(map[string][]core.SymbolNode, len(pkgs))
	for _, n := range nodes {
		byPkg[n.Package] = append(byPkg[n.Package], n)
	}
	for _, pkg := range pkgs {
		patcher.PatchPackageGraph(service, pkg, byPkg[pkg], nil)
	}
	return true
}
