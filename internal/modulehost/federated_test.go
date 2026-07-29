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

	"github.com/geoffmilleraz/fathom/internal/connector"
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
	// pkg/client/expertmodule_test.go) — this seed row matches that real
	// shape. Discovery no longer STRICTLY depends on it (discoverServiceIDs
	// unions ListServiceIDs with ClaimServiceIDs — critic finding 4;
	// TestListModules_ClaimsOnlyModuleDiscoverable below proves the
	// claims-only, zero-observations case directly), but keeping this
	// fixture realistic costs nothing.
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

// ─── critic finding 3: manifest-watermark modules must never false-drift ──

// TestModuleStaleness_ManifestWatermark_NeverFalselyDrifted is the missing
// test cell the critic named directly: a module compiled with a manifest:
// watermark (non-git corpus) whose corpus directory is deliberately nested
// INSIDE an enclosing git work tree (a docs folder inside a checkout; a
// module expanded into a repo-internal temp dir are the real-world
// shapes). Before the fix, ModuleStaleness called plain localGitHead
// unconditionally, which — because git discovers upward through parent
// directories — resolved to the ENCLOSING repo's HEAD and compared it
// against the manifest: watermark: two values that can never be equal,
// producing a PERMANENT, FALSE "drifted" verdict for a module that never
// drifted at all. recheckWatermark's manifest-prefix branch must recompute
// via the SAME content-addressed algorithm instead and never touch git.
func TestModuleStaleness_ManifestWatermark_NeverFalselyDrifted(t *testing.T) {
	ctx := context.Background()

	// Deliberately the INVERSE of gitProofTempDir: build an ENCLOSING git
	// repo on purpose, with the module's corpus nested inside it.
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", root) // bound discovery at root; still lets it find the enclosing repo we create below
	outerRepo := filepath.Join(root, "outer-checkout")
	if err := os.MkdirAll(outerRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(outerRepo, "unrelated.txt"), []byte("the enclosing checkout's own content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(outerRepo, "init", "-q")
	runGit(outerRepo, "config", "user.email", "t@t")
	runGit(outerRepo, "config", "user.name", "t")
	runGit(outerRepo, "add", ".")
	runGit(outerRepo, "commit", "-q", "-m", "outer checkout init")
	enclosingHead := gitHeadOf(t, outerRepo)

	// The module's own corpus: a plain subdirectory, itself NOT a git repo,
	// nested inside outerRepo — `git -C corpusDir rev-parse HEAD` resolves
	// to enclosingHead via upward discovery, exactly the false-signal the
	// bug used.
	corpusDir := filepath.Join(outerRepo, "module-corpus")
	if err := os.MkdirAll(corpusDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corpusDir, "finding.md"), []byte("# A finding\n\nSome governed rule text.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Compute the module's TRUE compile-time watermark the same way a real
	// non-git ingest would (connector.ManifestWatermark — never git, even
	// though corpusDir sits inside outerRepo).
	compileTimeWatermark, err := connector.ManifestWatermark(ctx, connector.NewLocalConnector(), core.ServiceRef{ID: "svc", Path: corpusDir})
	if err != nil {
		t.Fatalf("ManifestWatermark: %v", err)
	}
	if !strings.HasPrefix(compileTimeWatermark, connector.ManifestWatermarkPrefix) {
		t.Fatalf("precondition: ManifestWatermark = %q, want manifest: prefix", compileTimeWatermark)
	}
	if compileTimeWatermark == enclosingHead {
		t.Fatalf("precondition: manifest watermark accidentally equals the enclosing repo's HEAD — test fixture is not exercising the bug")
	}

	modPath := buildModuleStoreWithClaim(t, t.TempDir(), "nested.sqlite", "svc", compileTimeWatermark, corpusDir, "a governed rule")
	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(modPath, "nested"); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	report, found, err := h.ModuleStaleness(ctx, "nested")
	if err != nil {
		t.Fatalf("ModuleStaleness: %v", err)
	}
	if !found {
		t.Fatal("ModuleStaleness found=false for a mounted module")
	}
	if len(report) != 1 {
		t.Fatalf("ModuleStaleness = %+v, want exactly 1 entry", report)
	}
	// The load-bearing assertion: unchanged content must read FRESH, never
	// "drifted" just because the corpus happens to sit inside someone
	// else's git repo.
	if report[0].Status != modulehost.StatusFresh {
		t.Fatalf("Status = %q, want %q (unchanged manifest-watermarked corpus nested inside an enclosing git repo must never false-drift; enclosing repo HEAD = %q, manifest watermark = %q)",
			report[0].Status, modulehost.StatusFresh, enclosingHead, compileTimeWatermark)
	}
	if report[0].CurrentCommit != compileTimeWatermark {
		t.Errorf("CurrentCommit = %q, want the recomputed manifest watermark %q (never a git SHA)", report[0].CurrentCommit, compileTimeWatermark)
	}
	if report[0].CurrentCommit == enclosingHead {
		t.Error("CurrentCommit equals the enclosing repo's git HEAD — the bug is still live")
	}

	// Now actually change the corpus content and prove real drift IS still
	// detected (the fix recomputes correctly, it doesn't just always say
	// "fresh").
	if err := os.WriteFile(filepath.Join(corpusDir, "finding.md"), []byte("# A finding\n\nThe rule text changed.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	driftedReport, _, err := h.ModuleStaleness(ctx, "nested")
	if err != nil {
		t.Fatalf("ModuleStaleness (after edit): %v", err)
	}
	if len(driftedReport) != 1 || driftedReport[0].Status != modulehost.StatusDrifted {
		t.Fatalf("after editing the corpus, staleness = %+v, want status=%q (real drift must still be detected)", driftedReport, modulehost.StatusDrifted)
	}
}

// ─── critic finding 4: claims-only modules must be discoverable ──────────

// buildClaimsOnlyModuleStore creates a conforming module store carrying ONE
// served claim for serviceID and ZERO observations rows — the exact
// findings-style shape docs/design/expert-modules.md describes and
// ListModules' own (now-fixed) comment names: a claims-only corpus that
// ListServiceIDs (observations-derived) alone would never see.
func buildClaimsOnlyModuleStore(t *testing.T, dir, name, serviceID, rule string) string {
	t.Helper()
	storePath := filepath.Join(dir, name)
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ctx := context.Background()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: serviceID, IndexedCommit: "deadbeef"}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	claimID := core.ClaimID("obligations", "obligation", "FINDING.md#"+serviceID, "v1")
	claim := core.SemanticClaim{
		ClaimID: claimID, ServiceID: serviceID,
		Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
		Subject: "FINDING.md#" + serviceID, Rule: rule, Scope: []string{"**"},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: core.ClaimFresh,
	}
	// AnchorText set (unlike buildModuleStoreWithClaim's fixture above) so
	// this helper doubles as the fixture for QueryClaim.Anchor propagation
	// (the wire contract's anchor field — critic follow-up round).
	link := core.ClaimLink{ClaimID: claimID, Rel: core.RelCites, TargetRef: "FINDING.md#L1-L1", TargetHash: "irrelevant-for-this-test", AnchorText: "the literal cited anchor text"}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, []core.ClaimLink{link}); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	// Deliberately NO PutObservations call — this is the whole point.
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return storePath
}

func TestListModules_ClaimsOnlyModuleDiscoverable(t *testing.T) {
	dir := t.TempDir()
	path := buildClaimsOnlyModuleStore(t, dir, "findings.sqlite", "svc-findings", "a claims-only rule")

	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(path, "findings"); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	infos, err := h.ListModules(context.Background())
	if err != nil {
		t.Fatalf("ListModules: %v", err)
	}
	if len(infos) != 1 || infos[0].Module != "findings" || infos[0].Service != "svc-findings" {
		t.Fatalf("ListModules = %+v, want exactly one row for module=findings service=svc-findings", infos)
	}
	if infos[0].Error != "" {
		t.Errorf("ListModules[0].Error = %q, want empty", infos[0].Error)
	}
}

func TestFederatedObligations_ClaimsOnlyModuleAnswers(t *testing.T) {
	dir := t.TempDir()
	path := buildClaimsOnlyModuleStore(t, dir, "findings.sqlite", "svc-findings", "a claims-only rule")

	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(path, "findings"); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	results, err := h.FederatedObligations(context.Background(), "", "any/file.go", 0)
	if err != nil {
		t.Fatalf("FederatedObligations: %v", err)
	}
	if len(results) != 1 || results[0].Module != "findings" || results[0].Service != "svc-findings" {
		t.Fatalf("FederatedObligations = %+v, want exactly one findings/svc-findings result", results)
	}
	if len(results[0].Obligations.Rules) != 1 || results[0].Obligations.Rules[0].Rule != "a claims-only rule" {
		t.Fatalf("claims-only module's obligations = %+v", results[0].Obligations.Rules)
	}
}

// ─── critic finding 1: module_query (free-text federated query) ──────────

func TestFederatedQuery_SymbolsAndClaimsPerModule(t *testing.T) {
	dir := t.TempDir()

	// Module "code": symbol-bearing (an observation the query engine can
	// rank), no matching claim text.
	codePath := filepath.Join(dir, "code.sqlite")
	codeStore, err := store.Open(codePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	if err := codeStore.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc-code", IndexedCommit: "codebeef"}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	if err := codeStore.PutObservations(context.Background(), []core.Observation{{
		ServiceID: "svc-code", SourceRef: "auth/token.go", SourceKind: core.KindGoFunc,
		EvidenceTier: core.TierVerified, Symbol: "ValidateToken", Claim: "func ValidateToken(...) error",
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	if err := codeStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Module "findings": claims-only, text match on the query.
	findingsPath := buildClaimsOnlyModuleStore(t, dir, "findings.sqlite", "svc-findings", "always validate the auth token before use")

	h := modulehost.NewHost(store.LevelL0)
	if _, err := h.Mount(codePath, "code"); err != nil {
		t.Fatalf("Mount code: %v", err)
	}
	if _, err := h.Mount(findingsPath, "findings"); err != nil {
		t.Fatalf("Mount findings: %v", err)
	}

	results, err := h.FederatedQuery(context.Background(), "", "token", 0)
	if err != nil {
		t.Fatalf("FederatedQuery: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("FederatedQuery across 2 modules = %d results, want 2: %+v", len(results), results)
	}
	// Sorted by (module, service): code before findings.
	if results[0].Module != "code" || results[0].Service != "svc-code" {
		t.Errorf("results[0] = %+v, want module=code service=svc-code", results[0])
	}
	if len(results[0].Nodes) == 0 {
		t.Errorf("code module's query for %q returned no symbol nodes: %+v", "token", results[0])
	}
	if results[1].Module != "findings" || results[1].Service != "svc-findings" {
		t.Errorf("results[1] = %+v, want module=findings service=svc-findings", results[1])
	}
	if len(results[1].Claims) != 1 || results[1].Claims[0].Src == "" {
		t.Fatalf("findings module's claim match = %+v, want exactly one cited claim", results[1].Claims)
	}
	// Durable claim identity + anchor text (the follow-up wire-contract
	// round: expertchat's cite tokens are Module#ClaimID).
	wantClaimID := core.ClaimID("obligations", "obligation", "FINDING.md#svc-findings", "v1")
	gotClaim := results[1].Claims[0]
	if gotClaim.ClaimID != wantClaimID {
		t.Errorf("Claims[0].ClaimID = %q, want %q", gotClaim.ClaimID, wantClaimID)
	}
	if gotClaim.Text != "always validate the auth token before use" {
		t.Errorf("Claims[0].Text = %q", gotClaim.Text)
	}
	if gotClaim.Anchor != "the literal cited anchor text" {
		t.Errorf("Claims[0].Anchor = %q, want the fixture's AnchorText", gotClaim.Anchor)
	}
	// Watermark rides on every entry (ServiceMeta.IndexedCommit), regardless
	// of whether the pair answered via nodes, claims, or both.
	if results[0].Watermark != "codebeef" {
		t.Errorf("results[0].Watermark = %q, want %q", results[0].Watermark, "codebeef")
	}
	if results[1].Watermark != "deadbeef" {
		t.Errorf("results[1].Watermark = %q, want %q", results[1].Watermark, "deadbeef")
	}

	// Scoping to one module returns only that module's entry.
	scoped, err := h.FederatedQuery(context.Background(), "findings", "token", 0)
	if err != nil {
		t.Fatalf("FederatedQuery scoped to findings: %v", err)
	}
	if len(scoped) != 1 || scoped[0].Module != "findings" {
		t.Fatalf("scoped FederatedQuery = %+v, want exactly module=findings", scoped)
	}

	// A query matching nothing anywhere returns an empty (not erroring) result.
	empty, err := h.FederatedQuery(context.Background(), "", "xyzzy-nonexistent-term", 0)
	if err != nil {
		t.Fatalf("FederatedQuery (no matches): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("FederatedQuery with no matches = %+v, want empty", empty)
	}

	// Empty text and an unmounted module name are both refused loudly.
	if _, err := h.FederatedQuery(context.Background(), "", "", 0); err == nil {
		t.Error("FederatedQuery with empty text succeeded; want an error")
	}
	if _, err := h.FederatedQuery(context.Background(), "not-mounted", "token", 0); err == nil {
		t.Error("FederatedQuery scoped to an unmounted module succeeded; want an error")
	}
}
