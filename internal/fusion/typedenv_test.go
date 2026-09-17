package fusion_test

// BP-35 (docs/plans/bench-pyramid.md): the typed-edge loader must build the
// `go` subprocess's environment instead of inheriting whatever the parent
// process happens to carry.
//
// The defect this pins (2026-09-12, /tmp/fathom-http.err): the launchd
// daemon's plist declares EnvironmentVariables with PATH only, so GOPATH —
// and HOME — reach the daemon UNSET. With GOPATH unset, go1.26 resolves its
// default to the literal, unexpanded "$HOME/go" and refuses:
//
//	go: GOPATH entry is relative; must be absolute path: "$HOME/go"
//
// packages.Load surfaces that as a whole-load error, enrichTypedEdges logs
// one stderr line, and every daemon-side ingest of a Go service silently
// falls back to syntax-only edges — while the same ingest from a user shell
// (GOPATH exported) type-checks fine. An instrument may not degrade toward
// healthy (ADR-027), so: the loader repairs the env, and the failure is
// recorded on the service, not only on stderr.

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// runWithGOPATH runs EnrichTypedEdges against the typed fixture with GOPATH
// forced to the given value for this process (t.Setenv restores it), and
// asserts typed edges came out anyway.
func runWithGOPATH(t *testing.T, gopath string) {
	t.Helper()
	abs := typedFixtureDir(t)
	t.Setenv("GOPATH", gopath)
	ctx := context.Background()
	st := store.NewMemoryStore()

	report, err := fusion.EnrichTypedEdges(ctx, st, "typedfixture", abs, nil)
	if err != nil {
		t.Fatalf("EnrichTypedEdges with GOPATH=%q: %v", gopath, err)
	}
	if report.PackagesLoaded == 0 {
		t.Fatalf("GOPATH=%q: PackagesLoaded = 0 (total=%d failed=%d) — the loader inherited a broken GOPATH instead of repairing it",
			gopath, report.PackagesTotal, report.PackagesFailed)
	}
	if report.PackagesFailed != 0 {
		t.Fatalf("GOPATH=%q: PackagesFailed = %d, want 0", gopath, report.PackagesFailed)
	}
	edges, err := st.TypedEdges(ctx, "typedfixture")
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	if len(edges) == 0 {
		t.Fatalf("GOPATH=%q: no typed edges stored — the pass degraded to syntax-only without saying so", gopath)
	}
}

func TestEnrichTypedEdges_EmptyGOPATH(t *testing.T) {
	runWithGOPATH(t, "")
}

func TestEnrichTypedEdges_RelativeGOPATH(t *testing.T) {
	runWithGOPATH(t, "relative/path")
}

// TestTypedEdgeEnv_RepairsGoPath pins the env-construction contract itself,
// independent of a load: GOPATH is absolute, GOWORK stays off, and the
// caches point somewhere absolute too. GOFLAGS is deliberately NOT set —
// the pre-existing invocation never set it and the repo does not need
// -mod=mod; adding it here would change build semantics on every adopter.
func TestTypedEdgeEnv_RepairsGoPath(t *testing.T) {
	for _, broken := range []string{"", "relative/path", "./x"} {
		t.Setenv("GOPATH", broken)
		env := fusion.TypedEdgeEnv()
		got := map[string]string{}
		for _, kv := range env {
			if k, v, ok := strings.Cut(kv, "="); ok {
				got[k] = v // later entries win, exactly as exec does
			}
		}
		if got["GOWORK"] != "off" {
			t.Fatalf("GOPATH=%q: GOWORK = %q, want off", broken, got["GOWORK"])
		}
		for _, key := range []string{"GOPATH", "GOCACHE", "GOMODCACHE"} {
			v := got[key]
			if v == "" {
				t.Fatalf("GOPATH=%q: %s is empty — the go subprocess would fall back to a default that may be unresolvable", broken, key)
			}
			if !strings.HasPrefix(v, "/") {
				t.Fatalf("GOPATH=%q: %s = %q is not absolute", broken, key, v)
			}
			if strings.Contains(v, "$") {
				t.Fatalf("GOPATH=%q: %s = %q carries an unexpanded variable — this is the daemon's exact failure", broken, key, v)
			}
		}
		// The contract is that the LOADER adds nothing: an ambient GOFLAGS
		// (an operator's -p=2, a CI's -mod=vendor) passes through unchanged,
		// and only a value the environment did not already carry is the
		// loader's own guess.
		ambient, ambientSet := os.LookupEnv("GOFLAGS")
		if v, set := got["GOFLAGS"]; set && !(ambientSet && v == ambient) {
			t.Fatalf("GOPATH=%q: GOFLAGS=%q was set by the loader — not needed by this repo, and a guess that changes build semantics", broken, v)
		}
	}
}

// TestTypedEdgeEnv_KeepsAbsoluteGoPath: a healthy environment is left alone.
func TestTypedEdgeEnv_KeepsAbsoluteGoPath(t *testing.T) {
	t.Setenv("GOPATH", "/tmp/some-abs-gopath")
	for _, kv := range fusion.TypedEdgeEnv() {
		if strings.HasPrefix(kv, "GOPATH=") && kv != "GOPATH=/tmp/some-abs-gopath" {
			t.Fatalf("an already-absolute GOPATH was rewritten: %s", kv)
		}
	}
}

// TestTypedEdgeStatus_RecordedOnService: the pass's outcome is readable
// from the pipeline afterwards, not only from a stderr line. A service no
// ingest has touched in this process reports known=false — never a clean
// pass, which is the shape of the degradation this row exists to refuse.
func TestTypedEdgeStatus_RecordedOnService(t *testing.T) {
	abs := typedFixtureDir(t)
	ctx := context.Background()
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})

	if _, known := p.TypedEdgeStatus("typedfixture"); known {
		t.Fatal("TypedEdgeStatus reported a status before any ingest ran")
	}
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "typedfixture", Path: abs}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	got, known := p.TypedEdgeStatus("typedfixture")
	if !known {
		t.Fatal("no typed-edge status recorded after ingesting a Go service")
	}
	if got.Failed {
		t.Fatalf("typed pass failed after ingest: %s", got.Reason)
	}
	if got.Packages == 0 {
		t.Fatal("typed-edge status reports 0 packages after a successful pass")
	}
}

// TestTypedEdgeStatus_RecordsFailureReason: a broken toolchain environment
// leaves a NAMED reason on the service (I24), which is what turns doctor's
// typed_edges row red instead of leaving it quietly green.
func TestTypedEdgeStatus_RecordsFailureReason(t *testing.T) {
	abs := typedFixtureDir(t)
	ctx := context.Background()
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})

	// PATH without a go toolchain: the load cannot even be attempted.
	t.Setenv("PATH", t.TempDir())
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "typedfixture", Path: abs}); err != nil {
		t.Fatalf("Ingest must not fail when the typed pass does: %v", err)
	}
	got, known := p.TypedEdgeStatus("typedfixture")
	if !known {
		t.Fatal("no typed-edge status recorded for a failed pass")
	}
	if !got.Failed {
		t.Fatal("typed pass with no go toolchain on PATH reported success — the exact silent downgrade BP-35 exists to refuse")
	}
	if got.Reason == "" {
		t.Fatal("failed typed pass recorded no reason (I24: a named reason, never a bare count)")
	}
}

// ─── wave P3 / O82: the ingest Report itself carries typed-edge disclosure ───

// TestIngestReport_CarriesTypedEdgeDisclosure: a successful pass populates
// Report.TypedPackagesSeen and leaves TypedFailed false — the happy path
// for the fields ingest's own response (CLI text/--json, the MCP tool) now
// reads directly, instead of requiring a separate staleness_report call.
func TestIngestReport_CarriesTypedEdgeDisclosure(t *testing.T) {
	abs := typedFixtureDir(t)
	ctx := context.Background()
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})

	report, err := p.Ingest(ctx, core.ServiceRef{ID: "typedfixture", Path: abs})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if report.TypedFailed {
		t.Fatalf("report.TypedFailed = true on a clean fixture: %s", report.TypedFailReason)
	}
	if report.TypedPackagesSeen == 0 {
		t.Fatal("report.TypedPackagesSeen = 0 after a successful pass")
	}
}

// TestIngestReport_DisclosesTypedFailure pins the exact gap the 2026-09-16
// incident hit: a Go tree whose typed-edge pass produces nothing to trust
// must say so on the ingest Report ITSELF (never only via a separate
// staleness_report/doctor call) — this is the field the MCP `ingest` tool
// and `fathom ingest`/`--json` read to build their own typed_edges block.
func TestIngestReport_DisclosesTypedFailure(t *testing.T) {
	abs := typedFixtureDir(t)
	ctx := context.Background()
	st := store.NewMemoryStore()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})

	t.Setenv("PATH", t.TempDir()) // no go toolchain: the load cannot even be attempted
	report, err := p.Ingest(ctx, core.ServiceRef{ID: "typedfixture", Path: abs})
	if err != nil {
		t.Fatalf("Ingest must not fail when only the typed pass does: %v", err)
	}
	if !report.TypedFailed {
		t.Fatal("report.TypedFailed = false with no go toolchain on PATH — the exact silent downgrade this disclosure exists to refuse")
	}
	if report.TypedFailReason == "" {
		t.Error("report.TypedFailReason is empty (I24: a named reason, never a bare count)")
	}
}
