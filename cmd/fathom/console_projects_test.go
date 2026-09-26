package main

// console_projects_test.go — the project gather's own honesty and cost:
// an unreadable intent corpus is a named lower bound all the way to the
// page (never "declares none"), and a model rebuild re-folds the project
// set without re-reading a store, ledger or corpus that did not move.

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// writeIntentCorpus writes a router, two invariants and one incident.
func writeIntentCorpus(t *testing.T, root string) (invariants, incidents string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "intent")
	mustWrite(t, filepath.Join(dir, "README.md"), "# Project intent — router\n")
	invariants = filepath.Join(dir, "invariants.md")
	mustWrite(t, invariants, "# Invariants\n\n| Id | Statement | Enforced by | Source |\n|---|---|---|---|\n| I1 | The store is pure Go. | test `TestStorePurity` | ADR-020 |\n| I2 | Answers are deterministic. | gate `task validate` | ADR-001 |\n")
	incidents = filepath.Join(dir, "incidents.md")
	mustWrite(t, incidents, "# Incidents\n\n### INC-1 — a thing broke\n\nOutcome: pending.\n")
	return invariants, incidents
}

// TestConsoleProjects_UnreadableIntentCorpusIsALowerBound: the survey's
// EPERM case. With the corpus readable a project declares three targets;
// with two of its files unreadable, certify status is a named lower bound
// (partial) — and with nothing readable, unknown (error) — on the tile, in
// the queue (which turns partial), in the fleet cell and in the verb's own
// JSON. Never "no project declares a certification target".
func TestConsoleProjects_UnreadableIntentCorpusIsALowerBound(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	root := t.TempDir()
	inv, inc := writeIntentCorpus(t, root)
	project := func(certs console.Value[console.CertCounts]) console.Projects {
		return console.Projects{
			Ledger: console.SourceRead{Name: console.SourceVerifyLedger, State: console.StatePresent},
			Store:  console.SourceRead{Name: console.SourceStore, State: console.StatePresent},
			Rows: []console.ProjectFacts{{Service: "svc", Path: root, Freshness: "fresh", Certs: certs,
				Claim: console.Present(console.ClaimCounts{}, "s"), Guide: console.Present(console.GuideCoverage{}, "s")}},
		}
	}

	full := certCounts(ctx, st, "svc", root)
	if full.State != console.StatePresent || full.V.Declared != 3 || full.V.Gaps != 3 {
		t.Fatalf("readable corpus = %+v", full)
	}
	if v := console.CertificatesLive(project(full)); v.State != console.StateZero || !strings.Contains(v.Denominator, "3 declared targets") {
		t.Errorf("readable tile = %+v", v)
	}

	for _, p := range []string{inv, inc} {
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	}
	if _, err := os.ReadFile(inv); err == nil {
		t.Skip("this platform still reads a chmod-000 file (running as root?)")
	}
	blind := certCounts(ctx, st, "svc", root)
	if blind.State != console.StateError || !strings.Contains(blind.Reason, "invariants.md") || len(blind.V.Unreadable) != 2 {
		t.Fatalf("nothing declared, corpus unreadable = %+v; want unknown, naming the files", blind)
	}
	p := project(blind)
	if v := console.CertificatesLive(p); v.State != console.StatePartial || strings.Contains(v.Reason, "no project declares") {
		t.Errorf("tile over an unreadable corpus = %+v; want a named lower bound", v)
	}
	a := console.AttentionQueue(p)
	if a.State != console.StatePartial || !strings.Contains(a.Reason, console.SourceCertTargets) || a.ByKind[console.AttentionCertificates] != 1 {
		t.Errorf("queue over an unreadable corpus = %s %q %v", a.State, a.Reason, a.ByKind)
	}
	fleet := console.FleetTable(p, console.ParseFleetQuery(url.Values{}))
	if c := fleet.Rows[0].CertsLive; c.State != console.StateError {
		t.Errorf("fleet cell = %+v; want unreadable, never \"none declared\"", c)
	}
	if cell := fleetRowCells(fleet.Rows[0]).certs; strings.Contains(cell, "none declared") || !strings.Contains(cell, "unreadable") {
		t.Errorf("fleet cell renders %q", cell)
	}
	// The verb's own JSON names what it could not read — never the skip.
	raw, _ := json.Marshal(certifyStatusJSON("svc", certify.GapSummary{Declared: blind.V.Declared, Live: blind.V.Live, Gaps: blind.V.Gaps}, blind.V.Unreadable))
	if strings.Contains(string(raw), "nothing declared") || !strings.Contains(string(raw), "invariants.md") {
		t.Errorf("certify status JSON = %s", raw)
	}

	// A declared target in the store keeps the count known in part.
	if err := st.PutCertifyDeclarations(ctx, "svc", []store.CertifyDeclaration{{ServiceID: "svc", TestSymbolKey: "svc:x_test.go:TestNine", Kind: "invariant", TargetID: "I9", File: "x_test.go", Line: 3}}); err != nil {
		t.Fatal(err)
	}
	part := certCounts(ctx, st, "svc", root)
	if part.State != console.StatePartial || part.V.Declared != 1 || !strings.Contains(part.Reason, "lower bound") {
		t.Errorf("declared in part = %+v", part)
	}
}

// TestConsoleProjects_GatherReusesReadsWhileWatermarksHold pins design §1's
// cache: a model rebuild (a new model, a new generated-at) re-folds the
// project set from the cached store facts, re-reading a project only when
// the store's write stamp, its HEAD or its intent corpus moved.
func TestConsoleProjects_GatherReusesReadsWhileWatermarksHold(t *testing.T) {
	ctx := context.Background()
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "verify-ledger.jsonl"))
	st := &countingStore{Store: store.NewMemoryStore()}
	storePath := filepath.Join(t.TempDir(), "fleet.sqlite")
	mustWrite(t, storePath, "stand-in for the store file: only its stamp is read")
	root := t.TempDir()
	writeIntentCorpus(t, root)
	model := func(at string) observatoryModel {
		return observatoryModel{GeneratedAt: at, Services: []obsServiceRow{{fleetServiceRow: fleetServiceRow{Service: "svc", Path: root, Freshness: "fresh", RepoHead: "3d58a0db490552d76bd15876f6b12cbbacd492c2"}}}}
	}
	caches := &projectsCaches{}
	src := projectSources{model: model("t0"), st: st, storeRead: console.SourceRead{Name: console.SourceStore, State: console.StatePresent}, storePath: storePath, now: time.Now(), cache: caches}

	first := gatherProjects(ctx, src)
	reads := st.claims.Load()
	if reads == 0 || first.projects.Rows[0].Certs.V.Declared != 3 {
		t.Fatalf("first gather read the store %d times, certs %+v", reads, first.projects.Rows[0].Certs)
	}
	src.model = model("t1") // the 30 s rebuild: nothing moved
	again := gatherProjects(ctx, src)
	if st.claims.Load() != reads || again.projects.Rows[0].Certs.V.Declared != 3 {
		t.Errorf("a model rebuild re-read the store (%d → %d reads)", reads, st.claims.Load())
	}
	// The store moves (an ingest, a compile): the next gather re-reads it.
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(storePath, later, later); err != nil {
		t.Fatal(err)
	}
	gatherProjects(ctx, src)
	if st.claims.Load() == reads {
		t.Error("the store moved, and the gather did not re-read it")
	}
	// The intent corpus moves: re-read too.
	reads = st.claims.Load()
	mustWrite(t, filepath.Join(root, "docs", "intent", "invariants.md"), "# Invariants\n\n| Id | Statement | Enforced by | Source |\n|---|---|---|---|\n| I1 | One. | test `TestOne` | x |\n")
	if got := gatherProjects(ctx, src); st.claims.Load() == reads || got.projects.Rows[0].Certs.V.Declared != 2 {
		t.Errorf("an edited corpus was not re-read: %d reads, certs %+v", st.claims.Load()-reads, got.projects.Rows[0].Certs)
	}
}

// countingStore counts the gather's store reads.
type countingStore struct {
	store.Store
	claims atomic.Int64
}

func (c *countingStore) ClaimsForService(ctx context.Context, service, layer string) ([]core.SemanticClaim, error) {
	c.claims.Add(1)
	return c.Store.ClaimsForService(ctx, service, layer)
}
