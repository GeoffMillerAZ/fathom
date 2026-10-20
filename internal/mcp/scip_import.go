package mcp

// scip_import.go — SR-65 (surface reckoning F-74, owner decision): the
// ingest tool's additive optional `indexers` parameter. `fathom init` runs
// scip-typescript / scip-python when that toolchain is on PATH (and never
// installs one), leaving each index under the service root's
// .fathom/scip/<indexer>.scip; naming the indexer here asks the daemon to
// import what it left — after the syntax pass (an import maps SCIP
// definitions onto the service's own extracted symbols, O115), under
// tenancy's one ingest slot (RT-160), and under an admission budget: an
// index larger than the budget is declined by name, never decoded into the
// daemon's memory.
//
// The set of producers is closed. The name becomes a file name under the
// root, so an open string would be a path; two names are the two producers
// `init` knows how to run.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/statedir"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// scipIndexers is the closed set of producers the ingest tool's `indexers`
// parameter names. An array, not a map (tenancy lint: no package-level
// maps); it is two entries long.
var scipIndexers = [...]string{"scip-typescript", "scip-python"}

// SCIPIndexers lists the producers the ingest tool's `indexers` parameter
// accepts, for the CLI that runs them (cmd/fathom init).
func SCIPIndexers() []string { return append([]string(nil), scipIndexers[:]...) }

// scipImportBackend is the optional backend capability behind `indexers`:
// *client.Client (and the auto-refresh wrapper that forwards it) imports a
// SCIP index file as typed edges for a service.
type scipImportBackend interface {
	ImportSCIP(ctx context.Context, serviceID, path string) (scip.Import, error)
}

// defaultSCIPImportBudget bounds one index file when no admission gate
// supplies a ceiling (stdio, tests): 64 MiB, the index class's per-call
// ceiling.
const defaultSCIPImportBudget int64 = 64 << 20

// SetSCIPImportBudget overrides the size, in bytes, above which an index is
// declined instead of imported. Tests use it; serving derives the budget
// from the admission gate (scipImportBudget).
func (s *Server) SetSCIPImportBudget(n int64) { s.scipBudget = n }

// scipImportBudget is the largest index file one call imports: the gate's
// per-call ceiling for the tree class (budget/16, the class a whole-corpus
// read is admitted in) when a gate is installed, else the default.
func (s *Server) scipImportBudget() int64 {
	if s.scipBudget > 0 {
		return s.scipBudget
	}
	if s.gate != nil {
		if c := s.gate.Budget().PerCall[tenancy.ClassTree]; c > 0 {
			return c
		}
	}
	return defaultSCIPImportBudget
}

// validIndexers checks the caller's names against the closed set and
// returns them de-duplicated in the order given.
func validIndexers(names []string) ([]string, error) {
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		ok := false
		for _, k := range scipIndexers {
			if n == k {
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("indexers: %q is not a known producer — one of %s", n, strings.Join(scipIndexers[:], ", "))
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}

// importIndexers imports each named producer's index from root and returns
// one row per indexer: the import's counts, or the reason it was not
// imported. Never an error for the call: the syntax index is already
// written, and a caller that named an indexer reads its row.
func (s *Server) importIndexers(ctx context.Context, backend QueryInterface, svc, root string, names []string) []map[string]any {
	imp, canImport := backend.(scipImportBackend)
	budget := s.scipImportBudget()
	out := make([]map[string]any, 0, len(names))
	imported := false
	for _, name := range names {
		rel := filepath.ToSlash(projectdir.SCIP.Path("", name+".scip"))
		row := map[string]any{"indexer": name, "path": rel}
		out = append(out, row)
		if !canImport {
			row["error"] = "this backend does not import SCIP indexes"
			continue
		}
		if root == "" {
			row["error"] = "the service has no recorded root to find " + rel + " under"
			continue
		}
		abs := statedir.ReadPath(root, projectdir.SCIP, name+".scip")
		fi, err := os.Lstat(abs)
		if err != nil || !fi.Mode().IsRegular() {
			row["error"] = "no " + rel + " under the service root — run " + name + " there (fathom init does when the toolchain is installed)"
			continue
		}
		if fi.Size() > budget {
			row["error"] = fmt.Sprintf("the index is %s, over the %s import budget — not imported", tenancy.FormatBytes(fi.Size()), tenancy.FormatBytes(budget))
			continue
		}
		if cerr := tenancy.Charge(ctx, fi.Size()); cerr != nil {
			row["error"] = "not imported: " + cerr.Error()
			continue
		}
		release, serr := tenancy.AcquireIngestSlot(ctx)
		if serr != nil {
			row["error"] = "not imported — the wait for the one ingest slot ended: " + serr.Error()
			continue
		}
		res, ierr := imp.ImportSCIP(ctx, svc, abs)
		release()
		if ierr != nil {
			row["error"] = ierr.Error()
			continue
		}
		imported = true
		row["producer"] = res.Report.Producer.String()
		row["edges"] = res.Report.Edges
		row["direct"] = res.Report.Direct
		row["unresolved"] = res.Report.Unresolved
		// O115: a definition the import could not join to the service's own
		// symbols contributes no edge — disclosed, never folded into a count.
		row["unmapped"] = res.Report.Unmapped
	}
	if imported {
		// Typed edges changed after the ingest's own cache eviction.
		evictServiceCaches(backend, svc)
	}
	return out
}
