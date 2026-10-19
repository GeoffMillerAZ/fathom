package main

// certify_sweep_test.go — the post-ingest certificate sweep, driven
// through a plain `fathom ingest`: a repeat ingest at the same commit
// skips the certifications compile, so the sweep is what re-checks every
// existing certificate against the fresh index and run evidence.
import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const sweepFixtureInvariants = "# Invariants\n\n| # | Statement | Enforced by |\n|---|---|---|\n" +
	"| I1 | Sums never overflow | test (`TestMoved`) |\n" +
	"| I2 | Sums are commutative | test (`TestGone`) |\n" +
	"| I3 | Sums are associative | test (`TestRerun`) |\n"

func sweepFixtureTest(name, target string) string {
	return "package cart\n\nimport \"testing\"\n\n// certifies: invariant " + target + " (asserts)\nfunc " + name + "(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"sum\")\n\t}\n}\n"
}

// sweepFixtureEnvelope is one attested go-test run of suite cart/unit,
// bound to commit, with one case per name at status.
func sweepFixtureEnvelope(key, commit, startedAt, status string, names ...string) string {
	verdict := "pass"
	if status != "pass" {
		verdict = "fail"
	}
	cases := ""
	for i, n := range names {
		if i > 0 {
			cases += ",\n"
		}
		cases += `    {"id": "` + n + `", "name": "` + n + `", "status": "` + status + `", "duration": "1ms", "attempts": 1}`
	}
	return `{
  "suite": {"component": "cart", "name": "unit", "class": "unit"},
  "key": "` + key + `",
  "verdict": "` + verdict + `",
  "cases": [
` + cases + `
  ],
  "totals": {"run": 1, "passed": 1, "failed": 0, "skipped": 0, "flaked": 0},
  "timing": {"startedAt": "` + startedAt + `", "wallClock": "1ms"},
  "env": {"toolchainDigest": "x", "runnerClass": "y", "arch": "amd64", "os": "darwin", "commit": "` + commit + `", "dirty": false},
  "artifacts": []
}`
}

// setUpSweepFixture builds a committed Go repository with three marked
// tests (TestMoved, TestGone, TestRerun, each certifying one invariant)
// and a [tests] results_dir holding one passing run of all three, then
// runs one plain ingest, which compiles the certifications layer.
func setUpSweepFixture(t *testing.T) (repo *selfSkipGitRepo, storePath, commit string) {
	t.Helper()
	repo = newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module example.com/cart\n\ngo 1.22\n")
	repo.writePlan("cart.go", "package cart\n\n// Add sums two ints.\nfunc Add(a, b int) int { return a + b }\n")
	repo.writePlan("moved_test.go", sweepFixtureTest("TestMoved", "I1"))
	repo.writePlan("gone_test.go", sweepFixtureTest("TestGone", "I2"))
	repo.writePlan("rerun_test.go", sweepFixtureTest("TestRerun", "I3"))
	repo.writePlan("docs/intent/README.md", "# Router\n\ndocs/intent/invariants.md is the living doc.\n")
	repo.writePlan("docs/intent/invariants.md", sweepFixtureInvariants)
	repo.writePlan(".fathom/config.toml", "[tests]\nresults_dir = \"results\"\n")
	repo.writePlan(".gitignore", "results/\n")
	repo.commitAll("c0")
	commit = headSHAOf(t, repo.root)
	repo.writePlan("results/run-1.json", sweepFixtureEnvelope("k1", commit, "2026-09-23T00:00:00Z", "pass", "TestGone", "TestMoved", "TestRerun"))

	storePath = filepath.Join(t.TempDir(), "sweep.sqlite")
	if code, _, stderr := runInDir(t, repo.root, []string{"ingest", "--service", "cart-svc", "--path", repo.root, "--store", storePath}); code != 0 {
		t.Fatalf("first ingest exit = %d, stderr: %s", code, stderr)
	}
	return repo, storePath, commit
}

type sweepClaimView struct {
	Status core.ClaimStatus
	State  string
}

func sweepClaimsByTest(t *testing.T, storePath string) map[string]sweepClaimView {
	t.Helper()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	claims, err := st.ClaimsForService(context.Background(), "cart-svc", certify.Layer)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]sweepClaimView, len(claims))
	for _, c := range claims {
		var m certify.Meta
		if err := json.Unmarshal(c.Meta, &m); err != nil {
			t.Fatalf("claim %s meta: %v", c.ClaimID, err)
		}
		out[m.Test] = sweepClaimView{Status: c.Status, State: m.State}
	}
	return out
}

// TestSweepCertifications_MovedGoneAndNewRun: after an ingest compiled
// three live certificates, a second ingest at the same commit, with one
// test moved to another file, one test renamed away and a new failing run
// of the third, leaves the moved one stale, the gone one contradicted and
// the rerun one broken.
func TestSweepCertifications_MovedGoneAndNewRun(t *testing.T) {
	repo, storePath, commit := setUpSweepFixture(t)

	before := sweepClaimsByTest(t, storePath)
	for _, name := range []string{"TestGone", "TestMoved", "TestRerun"} {
		if c := before[name]; c.Status != core.ClaimFresh || c.State != string(certify.StateLive) {
			t.Fatalf("after first ingest %s = %+v, want fresh/live (all: %+v)", name, c, before)
		}
	}

	if err := os.Remove(filepath.Join(repo.root, "moved_test.go")); err != nil {
		t.Fatal(err)
	}
	repo.writePlan("elsewhere_test.go", sweepFixtureTest("TestMoved", "I1"))
	repo.writePlan("gone_test.go", sweepFixtureTest("TestGoneRenamed", "I2"))
	repo.writePlan("results/run-2.json", sweepFixtureEnvelope("k2", commit, "2026-09-24T00:00:00Z", "fail", "TestRerun"))

	if code, _, stderr := runInDir(t, repo.root, []string{"ingest", "--service", "cart-svc", "--path", repo.root, "--store", storePath}); code != 0 {
		t.Fatalf("second ingest exit = %d, stderr: %s", code, stderr)
	}

	after := sweepClaimsByTest(t, storePath)
	if c := after["TestMoved"]; c.Status != core.ClaimStale {
		t.Errorf("TestMoved (file moved) = %+v, want stale", c)
	}
	if c := after["TestGone"]; c.Status != core.ClaimContradicted {
		t.Errorf("TestGone (renamed away) = %+v, want contradicted", c)
	}
	if c := after["TestRerun"]; c.Status != core.ClaimFresh || c.State != string(certify.StateBroken) {
		t.Errorf("TestRerun (new failing run) = %+v, want fresh/broken", c)
	}
}

// sweepCountingStore counts the reads only the compile or the sweep make,
// so a test can tell whether the post-ingest certifications pass went past
// the declaration count.
type sweepCountingStore struct {
	store.Store
	claimReads, edgeReads int
}

func (s *sweepCountingStore) ClaimsForService(ctx context.Context, service, layer string) ([]core.SemanticClaim, error) {
	s.claimReads++
	return s.Store.ClaimsForService(ctx, service, layer)
}

func (s *sweepCountingStore) TypedEdges(ctx context.Context, service string) ([]core.Edge, error) {
	s.edgeReads++
	return s.Store.TypedEdges(ctx, service)
}

// TestRunCertifyCompile_NoDeclarationsNeverSweeps: a service with nothing
// declared reads the declaration count and stops (no claims read, no
// sweep), while a service compiled at this watermark already is swept.
func TestRunCertifyCompile_NoDeclarationsNeverSweeps(t *testing.T) {
	ctx := context.Background()

	t.Run("nothing declared", func(t *testing.T) {
		st, err := store.Open(filepath.Join(t.TempDir(), "empty.sqlite"))
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		cs := &sweepCountingStore{Store: st}
		runCertifyCompile(ctx, cs, "bare-svc", "wm-1", t.TempDir(), io.Discard, io.Discard, true)
		if cs.claimReads != 0 || cs.edgeReads != 0 {
			t.Errorf("claim reads = %d, edge reads = %d, want 0 and 0: nothing declared pays only the count", cs.claimReads, cs.edgeReads)
		}
	})

	t.Run("compiled at this watermark", func(t *testing.T) {
		repo, storePath, commit := setUpSweepFixture(t)
		st, err := store.Open(storePath)
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		cs := &sweepCountingStore{Store: st}
		runCertifyCompile(ctx, cs, "cart-svc", commit, repo.root, io.Discard, io.Discard, true)
		if cs.edgeReads != 1 {
			t.Errorf("edge reads = %d, want 1: the skipped compile must hand over to the sweep", cs.edgeReads)
		}
	})
}
