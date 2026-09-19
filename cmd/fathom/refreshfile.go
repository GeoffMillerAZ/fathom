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
	// Error carries a non-fatal failure reason (e.g. the file could not be
	// read) alongside a 200 — a refresh that could not happen is still an
	// answer, never a silent empty one; the hook's own staleness line (SU-6)
	// reads this to say "refresh failed: <reason>" instead of pretending
	// the index is current.
	Error string `json:"error,omitempty"`
}

// refreshFileHandler serves POST /refresh-file.
func refreshFileHandler(c *client.Client) http.HandlerFunc {
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
		out := buildRefreshFileResponse(r.Context(), c, req.Service, req.File)
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
func buildRefreshFileResponse(ctx context.Context, c *client.Client, service, file string) refreshFileResponse {
	report, err := c.ReingestFile(ctx, service, file)
	if err != nil {
		return refreshFileResponse{Error: err.Error()}
	}
	return refreshFileResponse{
		Refreshed:       report.Refreshed,
		Symbols:         report.Symbols,
		TypedEdgesStale: report.TypedEdgesStale,
	}
}
