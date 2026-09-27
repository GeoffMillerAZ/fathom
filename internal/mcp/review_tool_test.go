package mcp_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ─── reviewFakeBackend ───────────────────────────────────────────────────────
//
// reviewFakeBackend extends extendedFakeBackend with obligationsBackend and
// a minimal in-memory claims store that mirrors the real store's
// refutation-durability upsert (internal/store's PutClaims "status = CASE
// WHEN ... = 'refuted' THEN ... ELSE excluded.status END") — enough to
// prove review_diff's WIRING (attach → read-back → label → reconcile)
// without a real store.Store; the SQL-level durability guarantee itself is
// covered by the store's own contract tests (FT-4).

type reviewFakeBackend struct {
	extendedFakeBackend
	obligations map[string]semantic.Obligations // keyed by file
	claims      map[string]core.SemanticClaim   // keyed by claim ID
	links       map[string][]core.ClaimLink     // keyed by claim ID
	putErr      error
	// sources, when non-nil, overrides extendedFakeBackend's always-nil
	// Sources() — needed so applyAndPersistFindings' fileExists check (IT-7,
	// docs/plans/instrument-truth.md) can see a scoped file as still present
	// (review.ClassifyResolution's code-change branch) rather than every
	// superseded claim defaulting to scope-disappearance.
	sources map[string][]byte // keyed by repo-relative path
}

func (r *reviewFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	if r.sources == nil {
		return nil, nil
	}
	out := make([]core.SourceFile, 0, len(r.sources))
	for path, content := range r.sources {
		out = append(out, core.SourceFile{Path: path, Content: content})
	}
	return out, nil
}

func (r *reviewFakeBackend) Obligations(_ context.Context, _, file string, _ int) (semantic.Obligations, error) {
	return r.obligations[file], nil
}

func (r *reviewFakeBackend) ObligationsIndexStale(_ context.Context, _, _ string) bool { return false }

func (r *reviewFakeBackend) PutClaims(_ context.Context, claims []core.SemanticClaim, links []core.ClaimLink) error {
	if r.putErr != nil {
		return r.putErr
	}
	if r.claims == nil {
		r.claims = map[string]core.SemanticClaim{}
	}
	if r.links == nil {
		r.links = map[string][]core.ClaimLink{}
	}
	for _, c := range claims {
		if existing, ok := r.claims[c.ClaimID]; ok && existing.Status == core.ClaimRefuted {
			c.Status = core.ClaimRefuted // mirror the store's refutation-durability upsert
		}
		r.claims[c.ClaimID] = c
	}
	// Mirror the store's delete-then-insert link replacement per claim ID.
	replaced := map[string]bool{}
	for _, l := range links {
		if !replaced[l.ClaimID] {
			r.links[l.ClaimID] = nil
			replaced[l.ClaimID] = true
		}
		r.links[l.ClaimID] = append(r.links[l.ClaimID], l)
	}
	return nil
}

func (r *reviewFakeBackend) ClaimsForService(_ context.Context, _, layer string) ([]core.SemanticClaim, error) {
	var out []core.SemanticClaim
	for _, c := range r.claims {
		if layer == "" || c.Layer == layer {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *reviewFakeBackend) SetClaimStatus(_ context.Context, _, claimID string, status core.ClaimStatus) error {
	if c, ok := r.claims[claimID]; ok {
		c.Status = status
		r.claims[claimID] = c
	}
	return nil
}

func (r *reviewFakeBackend) SourceFingerprint(_ context.Context, _, path string) (string, bool, error) {
	return "fp-" + path, true, nil
}

// SetClaimValidTo (SU-19) mirrors the real store's bi-temporal close.
func (r *reviewFakeBackend) SetClaimValidTo(_ context.Context, _, claimID string, t time.Time) error {
	if c, ok := r.claims[claimID]; ok {
		c.ValidTo = t
		r.claims[claimID] = c
	}
	return nil
}

// WriteSupersedes (SU-20) mirrors store.Store.AppendClaimLink's
// non-destructive append — never PutClaims' wholesale link-set replace.
func (r *reviewFakeBackend) WriteSupersedes(_ context.Context, _, successorID, predecessorID, why string) error {
	if successorID == "" || predecessorID == "" {
		return nil
	}
	if r.links == nil {
		r.links = map[string][]core.ClaimLink{}
	}
	r.links[successorID] = append(r.links[successorID], core.ClaimLink{
		ClaimID: successorID, Rel: core.RelSupersedes, TargetRef: predecessorID, AnchorText: why,
	})
	return nil
}

var _ mcp.QueryInterface = (*reviewFakeBackend)(nil)

// ─── tests ────────────────────────────────────────────────────────────────

func TestReviewDiff_RequiredArgs(t *testing.T) {
	srv := mcp.NewServer(&reviewFakeBackend{extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}}})

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"changed_files": []string{"pkg/a.go"}}},
	})
	if resp["error"] == nil {
		t.Fatal("expected an error when service is omitted")
	}

	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{"name": "review_diff", "arguments": map[string]any{"service": "svc"}},
	})
	if resp["error"] == nil {
		t.Fatal("expected an error when changed_files is omitted")
	}
}

func TestReviewDiff_ComposesDimensionsHonestly(t *testing.T) {
	// No svcPath (no ServiceMeta) → arch_conformance falls back to
	// checked:false/none; no obligations map wired → obligations dimension
	// checked:true with zero findings (the fake DOES implement
	// obligationsBackend, just with nothing configured for this file).
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}},
		},
	})
	p := extToolPayload(t, resp)
	if p["service"] != "svc" {
		t.Errorf("service = %v", p["service"])
	}
	dims, ok := p["dimensions"].([]any)
	if !ok || len(dims) != 8 {
		// 8 (ADR-044 CV-12/CV-24): impact, obligations, arch_conformance,
		// spec_coverage, coverage, certifications, integrity, conservation.
		t.Fatalf("expected 8 dimensions, got %v", p["dimensions"])
	}
	byName := map[string]map[string]any{}
	for _, raw := range dims {
		d := raw.(map[string]any)
		byName[d["name"].(string)] = d
	}
	for _, want := range []string{"impact", "obligations", "arch_conformance", "spec_coverage", "coverage", "certifications"} {
		if _, ok := byName[want]; !ok {
			t.Errorf("missing dimension %q", want)
		}
	}
	// No coverage_profile was supplied in this call — the SAME honesty
	// discipline arch_conformance's own unchecked case demonstrates.
	if byName["coverage"]["checked"] != false {
		t.Errorf("coverage should be unchecked (no coverage_profile supplied): %v", byName["coverage"])
	}
	if byName["coverage"]["reason"] != "no coverage profile supplied" {
		t.Errorf("coverage reason = %v", byName["coverage"]["reason"])
	}
	// No certifications-layer claims exist for "svc" in this fixture — the
	// SAME "declared but empty" honesty discipline (TC-10).
	if byName["certifications"]["checked"] != false {
		t.Errorf("certifications should be unchecked (nothing declared): %v", byName["certifications"])
	}
	if byName["certifications"]["reason"] != "no certifications declared" {
		t.Errorf("certifications reason = %v", byName["certifications"]["reason"])
	}
	if byName["impact"]["checked"] != true {
		t.Errorf("impact should be checked: %v", byName["impact"])
	}
	if byName["arch_conformance"]["checked"] != false {
		t.Errorf("arch_conformance should be unchecked (no rules configured): %v", byName["arch_conformance"])
	}
	if byName["arch_conformance"]["reason"] == "" || byName["arch_conformance"]["reason"] == nil {
		t.Error("arch_conformance should carry a reason when unchecked")
	}
	if p["summary"] == "" || p["summary"] == nil {
		t.Error("expected a non-empty summary")
	}
}

func TestReviewDiff_ObligationsScoped(t *testing.T) {
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
		obligations: map[string]semantic.Obligations{
			"pkg/a.go": {
				Service: "svc", File: "pkg/a.go",
				Rules: []semantic.Obligation{{Rule: "Sort outputs.", Src: "CLAUDE.md#L1-L1", Tier: "doc-claimed"}},
			},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}},
		},
	})
	p := extToolPayload(t, resp)
	dims := p["dimensions"].([]any)
	var obligDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "obligations" {
			obligDim = d
		}
	}
	if obligDim == nil {
		t.Fatal("missing obligations dimension")
	}
	findings := obligDim["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected 1 obligation finding, got %d: %v", len(findings), findings)
	}
}

// TestReviewDiff_CoverageProfile proves RC-41's MCP wiring end to end: a
// coverage_profile argument (a repo-relative path under the service's
// corpus root, here workspace/cover.out — the SAME "under the repo or
// workspace/" scoping the CLI mirror documents) produces a checked
// "coverage" dimension with func A's real statement coverage, resolved via
// review.ResolveCoverage exactly as the CLI's --coverage-profile flag does.
func TestReviewDiff_CoverageProfile(t *testing.T) {
	corpusRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(corpusRoot, "workspace"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	profile := "mode: set\npkg/a.go:10.1,12.1 2 1\n"
	if err := os.WriteFile(filepath.Join(corpusRoot, "workspace", "cover.out"), []byte(profile), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}, "coverage_profile": "workspace/cover.out"},
		},
	})
	p := extToolPayload(t, resp)
	dims := p["dimensions"].([]any)
	var covDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "coverage" {
			covDim = d
		}
	}
	if covDim == nil {
		t.Fatal("missing coverage dimension")
	}
	if covDim["checked"] != true {
		t.Fatalf("expected checked:true with coverage_profile supplied, got %v (reason=%v)", covDim["checked"], covDim["reason"])
	}
	items, _ := covDim["coverage"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected exactly one changed-symbol coverage entry (func A), got %+v", covDim["coverage"])
	}
	entry := items[0].(map[string]any)
	if entry["uncovered"] != false {
		t.Errorf("func A's block has Count>0 — expected uncovered:false, got %+v", entry)
	}
}

// TestReviewDiff_CoverageProfiles_TwoLayers proves review_diff's real
// "by layer" attribution: two profiles, each labeled with an explicit
// layer via coverage_profiles, produce a coverage entry naming which
// layer covers the symbol and which does not (never guessed from
// per-test data).
func TestReviewDiff_CoverageProfiles_TwoLayers(t *testing.T) {
	corpusRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(corpusRoot, "workspace"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	unit := "mode: set\npkg/a.go:10.1,12.1 2 0\n"
	if err := os.WriteFile(filepath.Join(corpusRoot, "workspace", "unit.out"), []byte(unit), 0o644); err != nil {
		t.Fatalf("write unit profile: %v", err)
	}
	integration := "mode: set\npkg/a.go:10.1,12.1 2 1\n"
	if err := os.WriteFile(filepath.Join(corpusRoot, "workspace", "integration.out"), []byte(integration), 0o644); err != nil {
		t.Fatalf("write integration profile: %v", err)
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/call",
		Params: map[string]any{
			"name": "review_diff",
			"arguments": map[string]any{
				"service":       "svc",
				"changed_files": []string{"pkg/a.go"},
				"coverage_profiles": []map[string]any{
					{"layer": "unit", "path": "workspace/unit.out"},
					{"layer": "integration", "path": "workspace/integration.out"},
				},
			},
		},
	})
	p := extToolPayload(t, resp)
	dims := p["dimensions"].([]any)
	var covDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "coverage" {
			covDim = d
		}
	}
	if covDim == nil {
		t.Fatal("missing coverage dimension")
	}
	items, _ := covDim["coverage"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected exactly one changed-symbol coverage entry (func A), got %+v", covDim["coverage"])
	}
	entry := items[0].(map[string]any)
	layers, _ := entry["layers"].([]any)
	if len(layers) != 1 || layers[0] != "integration" {
		t.Errorf("layers = %v, want [integration] (only that layer covers it)", entry["layers"])
	}
	uncoveredAt, _ := entry["uncovered_at"].([]any)
	if len(uncoveredAt) != 1 || uncoveredAt[0] != "unit" {
		t.Errorf("uncovered_at = %v, want [unit]", entry["uncovered_at"])
	}
	if entry["uncovered"] != false {
		t.Errorf("uncovered should be false — the union is covered by integration: %+v", entry)
	}
}

// TestReviewDiff_CoverageProfile_TraversalRefused proves the safe-path
// discipline applies to coverage_profile too: a path escaping the corpus
// root is refused, never read.
func TestReviewDiff_CoverageProfile_TraversalRefused(t *testing.T) {
	corpusRoot := t.TempDir()
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 7, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}, "coverage_profile": "../../etc/passwd"},
		},
	})
	if resp["error"] == nil {
		t.Fatal("expected an error for a coverage_profile path escaping the corpus root")
	}
}

// TestReviewDiff_CoverageProfile_ProjectFathomignoreCarveOutStillWorks is
// the REGRESSION guard for RC-41's original rationale: a coverage profile
// that is excluded ONLY by the project's own .fathomignore (the ordinary
// "workspace/ is build output" case) must still resolve normally — the
// CRITICAL fix (finding 3) narrows the carve-out, it must not remove it.
func TestReviewDiff_CoverageProfile_ProjectFathomignoreCarveOutStillWorks(t *testing.T) {
	corpusRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(corpusRoot, "workspace"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeCorpusFile(t, corpusRoot, ".fathomignore", "workspace/\n")
	profile := "mode: set\npkg/a.go:10.1,12.1 2 1\n"
	if err := os.WriteFile(filepath.Join(corpusRoot, "workspace", "cover.out"), []byte(profile), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 9, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}, "coverage_profile": "workspace/cover.out"},
		},
	})
	p := extToolPayload(t, resp)
	dims := p["dimensions"].([]any)
	var covDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "coverage" {
			covDim = d
		}
	}
	if covDim == nil || covDim["checked"] != true {
		t.Fatalf("expected checked:true — a project-.fathomignore-only exclusion must still be readable (RC-41's build-artifact carve-out), got %v", covDim)
	}
}

// TestReviewDiff_CoverageProfile_EnvFileRefused is the MAJOR fix's first
// kept refusal: a coverage_profiles path shaped like a .env file must
// still refuse — the carve-out this comment justifies is about the
// project's OWN .fathomignore, never the unconditional secrets-shaped
// defense-in-depth check.
func TestReviewDiff_CoverageProfile_EnvFileRefused(t *testing.T) {
	corpusRoot := t.TempDir()
	writeCorpusFile(t, corpusRoot, "workspace/.env", "API_KEY=super-secret-canary\n")
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 10, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}, "coverage_profile": "workspace/.env"},
		},
	})
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("expected an error for a .env-shaped coverage_profile path")
	}
	msg, _ := errObj["message"].(string)
	if strings.Contains(msg, "super-secret-canary") {
		t.Fatalf("the refusal itself leaked the file's content: %q", msg)
	}
	if !strings.Contains(msg, ".env-pattern") {
		t.Errorf("message = %q, want it to mention .env-pattern", msg)
	}
}

// TestReviewDiff_CoverageProfile_ServiceScopedPurgeStillRefused is the
// MAJOR fix's second kept refusal: a coverage_profiles path excluded by
// the SERVICE-scoped ignore (~/.fathom/ignore.d/<service>, purge's
// write-side chokepoint) must still refuse even when the project's own
// .fathomignore also happens to exclude the same tree — the carve-out
// applies to the project-.fathomignore reason alone, never to purge.
func TestReviewDiff_CoverageProfile_ServiceScopedPurgeStillRefused(t *testing.T) {
	corpusRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_SUPPRESSIONS", "") // isolate: suppression must not be why this refuses
	dir := filepath.Join(home, ".fathom", "ignore.d")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "svc"), []byte("workspace/**\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(corpusRoot, "workspace"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	profile := "mode: set\npkg/a.go:10.1,12.1 2 1\n"
	if err := os.WriteFile(filepath.Join(corpusRoot, "workspace", "cover.out"), []byte(profile), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 11, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}, "coverage_profile": "workspace/cover.out"},
		},
	})
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("expected an error: a service-scoped (purge) exclusion must still refuse")
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "not in the indexed corpus") {
		t.Errorf("message = %q, want it to name the scoped exclusion refusal", msg)
	}
}

// TestReviewDiff_CoverageProfile_SuppressedRefused is the MAJOR fix's
// third kept refusal: a coverage_profiles path matching an active
// suppression must still refuse — suppression is untouched by the
// .fathomignore carve-out.
func TestReviewDiff_CoverageProfile_SuppressedRefused(t *testing.T) {
	corpusRoot := t.TempDir()
	writeCorpusFile(t, corpusRoot, "workspace/cover.out", "mode: set\npkg/a.go:10.1,12.1 2 1\n")
	suppressionsFile(t, "svc", "workspace/**", "suppressed for this test")
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 12, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}, "coverage_profile": "workspace/cover.out"},
		},
	})
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("expected an error: an active suppression must still refuse")
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "suppressed") {
		t.Errorf("message = %q, want it to name the suppression", msg)
	}
}

func TestReviewDiff_ArchViolationViaProjectRules(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ruleFile := `
rules: [
	{from_pkg: "pkg", to_pkg: "pkg", allow: false},
]
`
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(ruleFile), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}},
		},
	})
	p := extToolPayload(t, resp)
	dims := p["dimensions"].([]any)
	var archDim map[string]any
	for _, raw := range dims {
		d := raw.(map[string]any)
		if d["name"] == "arch_conformance" {
			archDim = d
		}
	}
	if archDim == nil {
		t.Fatal("missing arch_conformance dimension")
	}
	if archDim["checked"] != true {
		t.Fatalf("expected checked:true with a loaded project rule pack, got %v (reason=%v)", archDim["checked"], archDim["reason"])
	}
	count, _ := archDim["count"].(float64)
	if count == 0 {
		t.Fatalf("expected at least one violation, got count=%v findings=%v", archDim["count"], archDim["findings"])
	}
}

// TestReviewDiff_FindingsBridge_RefutationSticksAcrossReReview is the C2-1
// "prove it with a test" scenario at the MCP-wiring layer: emit finding →
// simulate refute (as fathom claim refute / ClaimFeedback would persist it)
// → re-run review_diff → the finding carries status "refuted" and is
// excluded from its dimension's active count, but remains listed.
func TestReviewDiff_FindingsBridge_RefutationSticksAcrossReReview(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ruleFile := `
rules: [
	{from_pkg: "api", to_pkg: "store", allow: false},
]
`
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(ruleFile), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}
	// A dedicated, minimal fixture with EXACTLY one arch-violating edge
	// (api.Handler → store.Query) so the round trip below can assert on a
	// single finding without incidental violations from other edges.
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: nodes, services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)

	call := func(id int) map[string]any {
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: id, Method: "tools/call",
			Params: map[string]any{
				"name":      "review_diff",
				"arguments": map[string]any{"service": "svc", "changed_files": []string{"api/handler.go"}},
			},
		})
		return extToolPayload(t, resp)
	}
	archDimOf := func(p map[string]any) map[string]any {
		for _, raw := range p["dimensions"].([]any) {
			d := raw.(map[string]any)
			if d["name"] == "arch_conformance" {
				return d
			}
		}
		t.Fatal("missing arch_conformance dimension")
		return nil
	}

	// Run 1: emit + persist the finding.
	p1 := call(10)
	if p1["findings_persisted"] != true {
		t.Fatalf("expected findings_persisted:true, got %v", p1["findings_persisted"])
	}
	d1 := archDimOf(p1)
	f1 := d1["findings"].([]any)[0].(map[string]any)
	claimID, _ := f1["claim_id"].(string)
	if claimID == "" {
		t.Fatal("expected a non-empty claim_id after persistence")
	}
	if _, hasStatus := f1["status"]; hasStatus {
		t.Errorf("a fresh finding should carry no status field, got %v", f1["status"])
	}
	if d1["count"].(float64) != 1 {
		t.Errorf("count = %v, want 1", d1["count"])
	}

	// Simulate an external refute (fathom claim refute / ClaimFeedback
	// persisting a refuted status for this exact ClaimID).
	c := fake.claims[claimID]
	c.Status = core.ClaimRefuted
	fake.claims[claimID] = c

	// Run 2: the identical finding must re-derive the SAME ClaimID and now
	// carry status "refuted", excluded from count but still listed.
	p2 := call(11)
	d2 := archDimOf(p2)
	findings2 := d2["findings"].([]any)
	if len(findings2) != 1 {
		t.Fatalf("refuted finding should remain listed, got %d findings", len(findings2))
	}
	f2 := findings2[0].(map[string]any)
	if f2["claim_id"] != claimID {
		t.Fatalf("ClaimID changed across re-review: %v vs %v", claimID, f2["claim_id"])
	}
	if f2["status"] != "refuted" {
		t.Errorf("status = %v, want %q", f2["status"], "refuted")
	}
	if d2["count"].(float64) != 0 {
		t.Errorf("count = %v, want 0 (the only finding is refuted)", d2["count"])
	}
}

// TestReviewDiff_ReconcileSupersedesFixedFinding proves the MCP wiring of
// the fixed-findings reconcile: a review-minted claim whose finding a later
// run (same changed files, violation now fixed) no longer emits is flipped
// to superseded — absent = fixed, never rot.
// TestReviewDiff_AssertedByIsReviewRun (SU-22, ADR-041 decision 7): a
// review_diff-minted claim carries a "review:" AssertedBy identity.
func TestReviewDiff_AssertedByIsReviewRun(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ruleFile := `
rules: [
	{from_pkg: "api", to_pkg: "store", allow: false},
]
`
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(ruleFile), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}
	violating := []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: violating, services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "review_diff",
			"arguments": map[string]any{"service": "svc", "changed_files": []string{"api/handler.go"}},
		},
	})
	_ = extToolPayload(t, resp)
	if len(fake.claims) == 0 {
		t.Fatal("expected at least 1 persisted claim")
	}
	for _, cl := range fake.claims {
		if !strings.HasPrefix(cl.AssertedBy, "review:") {
			t.Errorf("claim %s AssertedBy = %q, want a review: prefix", cl.ClaimID, cl.AssertedBy)
		}
	}
}

func TestReviewDiff_ReconcileSupersedesFixedFinding(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ruleFile := `
rules: [
	{from_pkg: "api", to_pkg: "store", allow: false},
]
`
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(ruleFile), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}
	violating := []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: violating, services: []string{"svc"}, svcPath: corpusRoot},
	}
	srv := mcp.NewServer(fake)
	call := func(id int) map[string]any {
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: id, Method: "tools/call",
			Params: map[string]any{
				"name":      "review_diff",
				"arguments": map[string]any{"service": "svc", "changed_files": []string{"api/handler.go"}},
			},
		})
		return extToolPayload(t, resp)
	}

	// Run 1: violation emitted + persisted fresh.
	call(20)
	if len(fake.claims) != 1 {
		t.Fatalf("expected 1 persisted claim after run 1, got %d", len(fake.claims))
	}
	var claimID string
	for id := range fake.claims {
		claimID = id
	}
	if fake.claims[claimID].Status != core.ClaimFresh {
		t.Fatalf("claim status after run 1 = %q, want fresh", fake.claims[claimID].Status)
	}

	// Fix the violation: the call edge is removed.
	fake.nodes = []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}

	// Run 2: same changed files, arch checked and clean → the prior claim
	// is superseded, not left served.
	p2 := call(21)
	if fake.claims[claimID].Status != core.ClaimSuperseded {
		t.Errorf("claim status after fixed re-review = %q, want superseded", fake.claims[claimID].Status)
	}
	for _, raw := range p2["dimensions"].([]any) {
		d := raw.(map[string]any)
		if d["name"] == "arch_conformance" && d["count"].(float64) != 0 {
			t.Errorf("arch count after fix = %v, want 0", d["count"])
		}
	}
}

// TestReviewEmitsVerdictOutcome is IT-7's real evidence (docs/plans/
// instrument-truth.md): review_diff's result payload must carry a
// verdict_outcome map (active/fixed/superseded finding counts) mirroring
// verify_plan's own pattern (verify_plan_tool_test.go's
// TestVerifyPlan_VerdictOutcomeReachesCallLog), so the call logger's
// existing extraction (calllog.go's verdictOutcomeFrom) picks review_diff
// calls up with zero logger changes. Run 1 emits+persists an active arch
// violation (active:1, nothing yet reconciled); run 2 fixes it IN PLACE
// (the violating call edge removed, the scoped file still present in the
// fake's Sources()) so the prior claim reconciles as a code-change
// resolution (active:0, fixed:1, superseded:0) rather than the
// scope-disappearance branch.
func TestReviewEmitsVerdictOutcome(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ruleFile := `
rules: [
	{from_pkg: "api", to_pkg: "store", allow: false},
]
`
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte(ruleFile), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}
	violating := []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: violating, services: []string{"svc"}, svcPath: corpusRoot},
		sources: map[string][]byte{
			"api/handler.go": []byte("package api\n"),
			"store/query.go": []byte("package store\n"),
		},
	}
	var buf bytes.Buffer
	srv := mcp.NewServerWithLogger(fake, mcp.NewCallLogger(&buf))
	args := map[string]any{"service": "svc", "changed_files": []string{"api/handler.go"}}
	call := func(id int) map[string]any {
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: id, Method: "tools/call",
			Params: map[string]any{"name": "review_diff", "arguments": args},
		})
		return extToolPayload(t, resp)
	}

	// Run 1: violation emitted + persisted fresh.
	p1 := call(40)
	vo1, ok := p1["verdict_outcome"].(map[string]any)
	if !ok {
		t.Fatalf("result payload missing verdict_outcome (fixture broken?): %v", p1)
	}
	if vo1["active"] != float64(1) || vo1["fixed"] != float64(0) || vo1["superseded"] != float64(0) {
		t.Fatalf("run1 verdict_outcome = %v, want active:1 fixed:0 superseded:0", vo1)
	}
	entry1 := callLogEntryFor(t, &buf, "review_diff", "")
	logged1, ok := entry1["verdict_outcome"].(map[string]any)
	if !ok {
		t.Fatalf("logged entry missing verdict_outcome: %v", entry1)
	}
	if logged1["active"] != float64(1) || logged1["fixed"] != float64(0) || logged1["superseded"] != float64(0) {
		t.Errorf("logged verdict_outcome = %v, want active:1 fixed:0 superseded:0", logged1)
	}

	// Fix the violation in place: the call edge is removed, both scoped
	// files remain present in Sources().
	fake.nodes = []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}

	// Run 2: the prior claim reconciles as fixed (code-change resolution).
	p2 := call(41)
	vo2, ok := p2["verdict_outcome"].(map[string]any)
	if !ok {
		t.Fatalf("run2 result payload missing verdict_outcome: %v", p2)
	}
	if vo2["active"] != float64(0) || vo2["fixed"] != float64(1) || vo2["superseded"] != float64(0) {
		t.Fatalf("run2 verdict_outcome = %v, want active:0 fixed:1 superseded:0", vo2)
	}
}

func (r *reviewFakeBackend) ScanSources(ctx context.Context, svc string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	files, err := r.Sources(ctx, svc)
	if err != nil {
		return corpus.Coverage{}, err
	}
	return corpus.ScanSourceFiles(ctx, files, bud, fn)
}

// partialScanReviewBackend's tree scan stops before any file (the handler
// budget spent), while the files themselves are readable by name.
type partialScanReviewBackend struct {
	*reviewFakeBackend
}

func (b partialScanReviewBackend) ScanSources(context.Context, string, corpus.Budget, func(core.SourceFile) bool) (corpus.Coverage, error) {
	return corpus.Coverage{Scanned: 0, Total: len(b.sources), Stopped: corpus.StopBytes}, nil
}

func (b partialScanReviewBackend) ReadSourceFiles(_ context.Context, _ string, paths []string) ([]core.SourceFile, error) {
	var out []core.SourceFile
	for _, p := range paths {
		if c, ok := b.sources[p]; ok {
			out = append(out, core.SourceFile{Path: p, Content: c})
		}
	}
	return out, nil
}

// RT-71 / I38: a review over a service whose tree scan stopped early still
// hashes the cited file (read by name), so re-running review_diff never
// strips an existing finding's cites link (PutClaims replaces the link set).
func TestReviewDiff_PartialScanKeepsCitationLinks(t *testing.T) {
	corpusRoot := t.TempDir()
	rulesDir := filepath.Join(corpusRoot, ".fathom", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "archconformance.cue"), []byte("rules: [\n\t{from_pkg: \"api\", to_pkg: \"store\", allow: false},\n]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}
	handler := []byte("package api\n\nimport \"svc/store\"\n\n// Handler serves.\n//\nfunc Handler() { store.Query() }\n\n")
	fake := &reviewFakeBackend{
		extendedFakeBackend: extendedFakeBackend{nodes: nodes, services: []string{"svc"}, svcPath: corpusRoot},
		sources:             map[string][]byte{"api/handler.go": handler, "store/query.go": []byte("package store\n\nfunc Query() {}\n")},
	}
	call := func(backend mcp.QueryInterface) string {
		t.Helper()
		resp := roundtrip(t, mcp.NewServer(backend), request{
			JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: map[string]any{
				"name":      "review_diff",
				"arguments": map[string]any{"service": "svc", "changed_files": []string{"api/handler.go"}},
			},
		})
		p := extToolPayload(t, resp)
		for _, raw := range p["dimensions"].([]any) {
			d := raw.(map[string]any)
			if d["name"] == "arch_conformance" {
				f := d["findings"].([]any)[0].(map[string]any)
				return f["claim_id"].(string)
			}
		}
		t.Fatal("missing arch_conformance dimension")
		return ""
	}
	hasCites := func(id string) bool {
		for _, l := range fake.links[id] {
			if l.Rel == core.RelCites {
				return true
			}
		}
		return false
	}
	id := call(fake)
	if !hasCites(id) {
		t.Fatalf("a complete scan persisted no cites link: %+v", fake.links[id])
	}
	if again := call(partialScanReviewBackend{fake}); again != id {
		t.Fatalf("claim id moved: %s vs %s", again, id)
	}
	if !hasCites(id) {
		t.Fatalf("a partial scan stripped the cites link: %+v", fake.links[id])
	}
}
