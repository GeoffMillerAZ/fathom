package modulehost_test

// federated_test.go — the federated query layer over N mounted modules
// (docs/plans/gke-module-host.md GKE-MH-3/4/6): scope-lookup results are
// tagged per module (never merged into one unattributed set), every rule
// still carries citation+tier+status (GKE-MH-4, unchanged from the
// single-store `obligations` contract), and staleness is honest per module
// (GKE-MH-6: unknown-detached by default, fresh/drifted only when the
// module's corpus is locally present and re-checkable).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// gitProofTempDir mirrors pkg/client/portability_test.go's helper: pins
// GIT_CEILING_DIRECTORIES to the temp dir's parent so git's upward
// discovery can never escape into this repo's own working tree (the
// Taskfile routes TMPDIR inside the repo) — load-bearing for any test that
// needs "not a git repo" (or "a specific, isolated git repo") to actually
// hold.
func gitProofTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	parent := filepath.Dir(dir)
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		parent = resolved
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", parent)
	return dir
}

// buildModuleStoreWithClaim creates a conforming module store at dir/name
// carrying one served (fresh, Served()==true) claim scoped to every path
// ("**"), citing src#L1-L1, for serviceID — the minimal fixture
// semantic.ObligationsFor needs to return a non-empty result.
func buildModuleStoreWithClaim(t *testing.T, dir, name, serviceID, indexedCommit, path, rule string) string {
	t.Helper()
	storePath := filepath.Join(dir, name)
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: serviceID, Path: path, IndexedCommit: indexedCommit}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	// A real module's corpus walk always populates at least one observation
	// row alongside its claims (TestExpertModule_NonGitCorpus_EndToEnd,
	// pkg/client/expertmodule_test.go) — ListServiceIDs (which
	// FederatedObligations/ModuleStaleness both use to discover a module's
	// services) scans DISTINCT service_id FROM observations, so this seed
	// row is what makes the service discoverable at all, matching a real
	// module's shape rather than an artificial claims-only fixture.
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: serviceID, SourceRef: "TEST.md", SourceKind: core.KindDocChunk,
		EvidenceTier: core.TierDocClaimed, Symbol: "rule",
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	claimID := core.ClaimID("obligations", "obligation", "TEST.md#rule", "v1")
	claim := core.SemanticClaim{
		ClaimID: claimID, ServiceID: serviceID,
		Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "TEST.md#rule", Rule: rule, Scope: []string{"**"},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: core.ClaimFresh,
	}
	link := core.ClaimLink{ClaimID: claimID, Rel: core.RelCites, TargetRef: "TEST.md#L1-L1", TargetHash: "irrelevant-for-this-test"}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, []core.ClaimLink{link}); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return storePath
}

func TestFederatedObligations_TaggedPerModuleNeverMerged(t *testing.T) {
	dir := t.TempDir()
	pathA := buildModuleStoreWithClaim(t, dir, "alpha.sqlite", "svc-a", "", "", "module A's rule")
	pathB := buildModuleStoreWithClaim(t, dir, "beta.sqlite", "svc-b", "", "", "module B's rule")

	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(pathA, "alpha"); err != nil {
		t.Fatalf("Mount alpha: %v", err)
	}
	if _, err := h.Mount(pathB, "beta"); err != nil {
		t.Fatalf("Mount beta: %v", err)
	}

	results, err := h.FederatedObligations(context.Background(), "", "any/file.go", 0)
	if err != nil {
		t.Fatalf("FederatedObligations: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("FederatedObligations across 2 modules = %d results, want 2: %+v", len(results), results)
	}
	// Sorted by (module, service): alpha before beta.
	if results[0].Module != "alpha" || results[0].Service != "svc-a" {
		t.Errorf("results[0] = %+v, want module=alpha service=svc-a", results[0])
	}
	if results[1].Module != "beta" || results[1].Service != "svc-b" {
		t.Errorf("results[1] = %+v, want module=beta service=svc-b", results[1])
	}
	if len(results[0].Obligations.Rules) != 1 || results[0].Obligations.Rules[0].Rule != "module A's rule" {
		t.Fatalf("alpha's obligations = %+v, want exactly [\"module A's rule\"]", results[0].Obligations.Rules)
	}
	if len(results[1].Obligations.Rules) != 1 || results[1].Obligations.Rules[0].Rule != "module B's rule" {
		t.Fatalf("beta's obligations = %+v, want exactly [\"module B's rule\"]", results[1].Obligations.Rules)
	}
	// GKE-MH-4: every rule carries its citation and tier (Server
	// conformance, store-format-v0.md §3), unchanged by federation.
	rule := results[0].Obligations.Rules[0]
	if rule.Src == "" {
		t.Error("federated rule missing its citation (Src)")
	}
	if rule.Tier == "" {
		t.Error("federated rule missing its evidence tier")
	}

	// Scoping to exactly one module returns only that module's answer.
	scoped, err := h.FederatedObligations(context.Background(), "alpha", "any/file.go", 0)
	if err != nil {
		t.Fatalf("FederatedObligations scoped to alpha: %v", err)
	}
	if len(scoped) != 1 || scoped[0].Module != "alpha" {
		t.Fatalf("scoped FederatedObligations = %+v, want exactly module=alpha", scoped)
	}

	// An unmounted module name is refused loudly, not silently empty.
	if _, err := h.FederatedObligations(context.Background(), "not-mounted", "any/file.go", 0); err == nil {
		t.Error("FederatedObligations scoped to an unmounted module succeeded; want an error")
	}
}

func TestListModules_EnumeratesEveryMountedCorpus(t *testing.T) {
	dir := t.TempDir()
	pathA := buildModuleStore(t, dir, "alpha.sqlite", "svc-a")

	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(pathA, "alpha"); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	infos, err := h.ListModules(context.Background())
	if err != nil {
		t.Fatalf("ListModules: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("ListModules = %+v, want exactly 1 row", infos)
	}
	if infos[0].Module != "alpha" || infos[0].Service != "svc-a" || infos[0].IndexedCommit != "deadbeef" {
		t.Errorf("ListModules[0] = %+v, want module=alpha service=svc-a indexed_commit=deadbeef", infos[0])
	}
}

// ─── staleness: unknown-detached by default ──────────────────────────────

func TestModuleStaleness_UnknownDetachedWhenPathAbsent(t *testing.T) {
	dir := t.TempDir()
	// service_meta.path points nowhere that exists on this machine — the
	// common "shipped from machine A, served on machine B" case.
	path := buildModuleStoreWithClaim(t, dir, "alpha.sqlite", "svc-a", "deadbeef", "/nonexistent/path/on/this/machine", "rule")

	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(path, "alpha"); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	report, found, err := h.ModuleStaleness(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("ModuleStaleness: %v", err)
	}
	if !found {
		t.Fatal("ModuleStaleness found=false for a mounted module")
	}
	if len(report) != 1 {
		t.Fatalf("ModuleStaleness = %+v, want exactly 1 entry", report)
	}
	if report[0].Status != modulehost.StatusUnknownDetached {
		t.Errorf("Status = %q, want %q (corpus path not locally present)", report[0].Status, modulehost.StatusUnknownDetached)
	}
	if report[0].IndexedCommit != "deadbeef" {
		t.Errorf("IndexedCommit = %q, want the stored compile-time watermark %q", report[0].IndexedCommit, "deadbeef")
	}
}

func TestModuleStaleness_NotMounted(t *testing.T) {
	h := modulehost.NewHost(store.LevelL0)
	_, found, err := h.ModuleStaleness(context.Background(), "nope")
	if err != nil {
		t.Fatalf("ModuleStaleness: %v", err)
	}
	if found {
		t.Error("ModuleStaleness found=true for a module never mounted")
	}
}

// ─── staleness: locally present and re-checkable ─────────────────────────

func TestModuleStaleness_FreshAndDriftedWhenCorpusLocallyPresent(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := gitProofTempDir(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	if err := writeFileAndInit(repo, run); err != nil {
		t.Fatal(err)
	}
	head := gitHeadOf(t, repo)

	dir := t.TempDir()

	// Case 1: stored watermark matches current HEAD -> fresh.
	freshPath := buildModuleStoreWithClaim(t, dir, "fresh.sqlite", "svc", head, repo, "rule")
	// Case 2: stored watermark is stale relative to current HEAD -> drifted.
	driftedPath := buildModuleStoreWithClaim(t, dir, "drifted.sqlite", "svc", "0000000000000000000000000000000000000000", repo, "rule")

	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(freshPath, "fresh"); err != nil {
		t.Fatalf("Mount fresh: %v", err)
	}
	if _, err := h.Mount(driftedPath, "drifted"); err != nil {
		t.Fatalf("Mount drifted: %v", err)
	}

	freshReport, _, err := h.ModuleStaleness(context.Background(), "fresh")
	if err != nil {
		t.Fatalf("ModuleStaleness(fresh): %v", err)
	}
	if len(freshReport) != 1 || freshReport[0].Status != modulehost.StatusFresh {
		t.Fatalf("fresh module staleness = %+v, want status=%q", freshReport, modulehost.StatusFresh)
	}
	if freshReport[0].CurrentCommit != head {
		t.Errorf("CurrentCommit = %q, want %q", freshReport[0].CurrentCommit, head)
	}

	driftedReport, _, err := h.ModuleStaleness(context.Background(), "drifted")
	if err != nil {
		t.Fatalf("ModuleStaleness(drifted): %v", err)
	}
	if len(driftedReport) != 1 || driftedReport[0].Status != modulehost.StatusDrifted {
		t.Fatalf("drifted module staleness = %+v, want status=%q", driftedReport, modulehost.StatusDrifted)
	}
}

func writeFileAndInit(repo string, run func(args ...string)) error {
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		return err
	}
	run("add", ".")
	run("commit", "-q", "-m", "init")
	return nil
}

func gitHeadOf(t *testing.T, repo string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}
