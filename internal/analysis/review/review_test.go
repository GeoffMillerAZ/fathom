package review_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// TestClaimsVocabularyMatchesSemanticLayer cross-checks review's mirrored
// findings-layer constants against internal/semantic's own — review cannot
// import internal/semantic in its main code (leaf-package rule), so this
// external test package (which CAN import both without creating a cycle)
// is the drift guard. It pins kind AND tier: the bridge stamps verified
// (deterministic oracle, Model empty — ADR-018 oracle classes, ADR-016 M1
// amendment) while the layer's own compiled corpus stays sme-stated, and
// the layer.cue must record that admission explicitly.
func TestClaimsVocabularyMatchesSemanticLayer(t *testing.T) {
	if review.ClaimsKind != semantic.KindFinding {
		t.Errorf("review.ClaimsKind = %q, want semantic.KindFinding %q", review.ClaimsKind, semantic.KindFinding)
	}
	if review.ClaimsTier != core.TierVerified {
		t.Errorf("review.ClaimsTier = %q, want %q (deterministic-oracle findings)", review.ClaimsTier, core.TierVerified)
	}

	// The findings layer manifest must both keep its compiled-corpus tier
	// (sme-stated) and record the bridge's verified-tier admission — mixed
	// tiers on the layer are deliberate only while the manifest says so.
	def, err := semantic.LoadLayer(review.ClaimsLayer)
	if err != nil {
		t.Fatalf("LoadLayer(%q): %v", review.ClaimsLayer, err)
	}
	if def.Tier != string(core.TierSMEStated) {
		t.Errorf("findings layer manifest tier = %q, want %q (the compiled corpus tier)", def.Tier, core.TierSMEStated)
	}
	raw := findingsLayerCue(t)
	for _, marker := range []string{"verified", "review-verb"} {
		if !strings.Contains(raw, marker) {
			t.Errorf("findings layer.cue is missing the oracle-tier admission marker %q — the bridge's verified-tier claims would be undisclosed mixed tiers", marker)
		}
	}
}

// findingsLayerCue reads the findings layer manifest from the repo tree.
func findingsLayerCue(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "../../../schema/semantic/layers/findings/layer.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// ─── fixtures ─────────────────────────────────────────────────────────────

// fixtureNodes: pkg.A (changed) is called by pkg.Caller (unchanged); pkg.B
// is untouched and unrelated. TestA covers pkg.A.
func fixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Line: 10},
		{Service: "svc", Package: "pkg", Name: "Caller", Kind: core.KindGoFunc, FilePath: "pkg/caller.go", Line: 5, Calls: []string{"svc::pkg.A"}},
		{Service: "svc", Package: "pkg", Name: "TestA", Kind: core.KindGoFunc, FilePath: "pkg/a_test.go", Line: 1, Calls: []string{"svc::pkg.A"}},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "pkg/b.go", Line: 1},
	}
}

// archViolationInput is the minimal Input with EXACTLY one arch violation:
// api.Handler (api/handler.go:7) calls store.Query against a deny rule.
func archViolationInput() review.Input {
	return review.Input{
		Service:      "svc",
		ChangedFiles: []string{"api/handler.go"},
		Nodes: []core.SymbolNode{
			{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
			{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
		},
		ArchChecked: true,
		ArchRules:   []archconformance.Rule{{FromPkg: "api", ToPkg: "store", Allow: false}},
	}
}

var fixedNow = func() time.Time { return time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC) }

// fakeFP is a ScopeFingerprint that resolves every path.
func fakeFP(path string) (string, bool) { return "fp-" + path, true }

// persistIn builds a PersistInput with a fake fingerprint and optional
// content map for cites hashing.
func persistIn(content map[string]string) review.PersistInput {
	return review.PersistInput{
		Now:       fixedNow,
		Watermark: "wm",
		FileContent: func(path string) ([]byte, bool) {
			if content == nil {
				return nil, false
			}
			c, ok := content[path]
			return []byte(c), ok
		},
		ScopeFingerprint: fakeFP,
	}
}

// ─── impact dimension ───────────────────────────────────────────────────────

func TestRun_Impact_ReportsDownstreamCallersNotChangedFilesThemselves(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"pkg/a.go"},
		Nodes:        fixtureNodes(),
	})
	d := dimension(t, v, review.DimImpact)
	if !d.Checked {
		t.Fatalf("impact should be checked, reason=%q", d.Reason)
	}
	if len(d.Tests) == 0 {
		t.Error("expected at least one affected test")
	}
	for _, f := range d.Findings {
		if f.File == "pkg/a.go" {
			t.Errorf("finding for the changed file itself should be excluded, got %+v", f)
		}
	}
	if len(d.Findings) == 0 {
		t.Error("expected downstream-caller findings (pkg.Caller, TestA)")
	}
}

func TestRun_Impact_UncheckedWhenNoSymbolsIndexed(t *testing.T) {
	v := review.Run(review.Input{Service: "svc", ChangedFiles: []string{"pkg/a.go"}})
	d := dimension(t, v, review.DimImpact)
	if d.Checked {
		t.Fatal("impact should be unchecked when the service has no indexed symbols")
	}
	if d.Reason == "" {
		t.Error("expected a non-empty reason")
	}
}

// ─── obligations dimension ─────────────────────────────────────────────────

func TestRun_Obligations_ScopedToChangedFiles(t *testing.T) {
	v := review.Run(review.Input{
		Service:              "svc",
		ChangedFiles:         []string{"pkg/a.go", "pkg/b.go"},
		ObligationsSupported: true,
		ObligationsByFile: map[string][]review.ObligationRule{
			"pkg/a.go": {{File: "pkg/a.go", Rule: "Sort outputs.", Src: "CLAUDE.md#L1-L1", Tier: "doc-claimed"}},
		},
	})
	d := dimension(t, v, review.DimObligations)
	if !d.Checked {
		t.Fatalf("obligations should be checked, reason=%q", d.Reason)
	}
	if len(d.Findings) != 1 {
		t.Fatalf("expected 1 obligation finding (only pkg/a.go has one), got %d: %+v", len(d.Findings), d.Findings)
	}
	if d.Findings[0].Message != "Sort outputs." {
		t.Errorf("Message = %q", d.Findings[0].Message)
	}
}

// TestRun_Obligations_ServedStatusLabelCarriedThrough proves the
// served-with-label discipline: a stale served claim keeps its status label
// in the verdict, never mapped away to severity alone.
func TestRun_Obligations_ServedStatusLabelCarriedThrough(t *testing.T) {
	v := review.Run(review.Input{
		Service:              "svc",
		ChangedFiles:         []string{"pkg/a.go"},
		ObligationsSupported: true,
		ObligationsByFile: map[string][]review.ObligationRule{
			"pkg/a.go": {{File: "pkg/a.go", Rule: "Old rule.", Src: "CLAUDE.md#L1-L1", Status: "stale"}},
		},
	})
	d := dimension(t, v, review.DimObligations)
	if len(d.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(d.Findings))
	}
	if d.Findings[0].Status != "stale" {
		t.Errorf("Status = %q, want %q (served-with-label)", d.Findings[0].Status, "stale")
	}
	if d.Findings[0].Severity != "warn" {
		t.Errorf("Severity = %q, want warn", d.Findings[0].Severity)
	}
	// A stale served obligation is still active.
	if d.Count != 1 {
		t.Errorf("Count = %d, want 1", d.Count)
	}
}

func TestRun_Obligations_UncheckedWhenBackendUnsupported(t *testing.T) {
	v := review.Run(review.Input{Service: "svc", ChangedFiles: []string{"pkg/a.go"}})
	d := dimension(t, v, review.DimObligations)
	if d.Checked {
		t.Fatal("obligations should be unchecked when the backend does not support semantic layers")
	}
	if d.Reason == "" {
		t.Error("expected a non-empty reason")
	}
}

// ─── arch_conformance dimension ─────────────────────────────────────────────

func TestRun_Arch_HonestyPassthrough(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"pkg/a.go"},
		ArchChecked:  false,
		ArchReason:   "no rules configured: no inline rules and no project rule pack",
	})
	d := dimension(t, v, review.DimArch)
	if d.Checked {
		t.Fatal("arch_conformance should be unchecked when ArchChecked is false")
	}
	if d.Reason == "" {
		t.Error("expected a non-empty reason")
	}
	if len(d.Findings) != 0 {
		t.Errorf("unchecked dimension must report zero findings, got %d", len(d.Findings))
	}
}

func TestRun_Arch_ReportsViolationsWithLineCitation(t *testing.T) {
	v := review.Run(archViolationInput())
	d := dimension(t, v, review.DimArch)
	if !d.Checked {
		t.Fatalf("expected checked=true, reason=%q", d.Reason)
	}
	if len(d.Findings) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(d.Findings), d.Findings)
	}
	f := d.Findings[0]
	if f.File != "api/handler.go" {
		t.Errorf("File = %q, want api/handler.go", f.File)
	}
	// The citation must be a PARSEABLE line citation (never a bare path):
	// a bare-path citation persisted onto a claim reads as anchor-dead to
	// semantic.Sweep and would flip the claim contradicted on the next
	// daemon refresh.
	if f.Citation != "api/handler.go#L7-L7" {
		t.Errorf("Citation = %q, want %q", f.Citation, "api/handler.go#L7-L7")
	}
	if _, _, _, ok := core.ParseCiteRef(f.Citation); !ok {
		t.Errorf("Citation %q is not parseable by core.ParseCiteRef", f.Citation)
	}
	if f.Severity != "error" {
		t.Errorf("Severity = %q, want error", f.Severity)
	}
	// Findings-as-evidence (docs/design/findings-as-evidence.md §3.2): a
	// broken layering rule is always core.FindingClassConformance,
	// deterministically — never the reviewer's guess.
	if f.Class != core.FindingClassConformance {
		t.Errorf("Class = %q, want %q", f.Class, core.FindingClassConformance)
	}
}

// ─── spec_coverage dimension ────────────────────────────────────────────────

func TestRun_SpecCoverage_FlagsUncoveredRefWithLineCitation(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"pkg/a.go"},
		ChangedSources: []core.SourceFile{
			{Path: "pkg/a.go", Content: []byte("// implements [US-001]\nfunc A() {}")},
		},
	})
	d := dimension(t, v, review.DimSpecCoverage)
	if !d.Checked {
		t.Fatalf("spec_coverage should always be checked, reason=%q", d.Reason)
	}
	if len(d.Findings) != 1 {
		t.Fatalf("expected 1 uncovered spec finding, got %d: %+v", len(d.Findings), d.Findings)
	}
	// The ref appears on line 1: the citation must resolve to it, parseable.
	if d.Findings[0].Citation != "pkg/a.go#L1-L1" {
		t.Errorf("Citation = %q, want %q", d.Findings[0].Citation, "pkg/a.go#L1-L1")
	}
	// A referenced-but-uncovered spec is a verification gap —
	// core.FindingClassTestGap, deterministically (docs/design/findings-
	// as-evidence.md §3.2).
	if d.Findings[0].Class != core.FindingClassTestGap {
		t.Errorf("Class = %q, want %q", d.Findings[0].Class, core.FindingClassTestGap)
	}
}

func TestRun_SpecCoverage_CleanWhenNoRefsInDiff(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"pkg/a.go"},
		ChangedSources: []core.SourceFile{
			{Path: "pkg/a.go", Content: []byte("func A() {}")},
		},
	})
	d := dimension(t, v, review.DimSpecCoverage)
	if !d.Checked {
		t.Fatal("spec_coverage should be checked even with zero refs")
	}
	if len(d.Findings) != 0 {
		t.Errorf("expected zero findings, got %d", len(d.Findings))
	}
}

// ─── determinism ────────────────────────────────────────────────────────────

func TestRun_Deterministic(t *testing.T) {
	in := review.Input{
		Service:              "svc",
		ChangedFiles:         []string{"pkg/b.go", "pkg/a.go"},
		Nodes:                fixtureNodes(),
		ObligationsSupported: true,
		ObligationsByFile: map[string][]review.ObligationRule{
			"pkg/a.go": {{File: "pkg/a.go", Rule: "Rule one."}},
		},
	}
	v1 := review.Run(in)
	v2 := review.Run(in)
	if v1.Summary != v2.Summary {
		t.Errorf("Summary differs across identical runs:\n%q\n%q", v1.Summary, v2.Summary)
	}
	if len(v1.ChangedFiles) != 2 || v1.ChangedFiles[0] != "pkg/a.go" {
		t.Errorf("ChangedFiles should be sorted: %v", v1.ChangedFiles)
	}
}

// ─── resolution outcomes (docs/design/findings-as-evidence.md §3.4, §6 item 4) ─

func TestClassifyResolution_AllScopedFilesExist_CodeChange(t *testing.T) {
	exists := map[string]bool{"api/handler.go": true, "api/router.go": true}
	got := review.ClassifyResolution([]string{"api/handler.go", "api/router.go"}, func(p string) bool { return exists[p] })
	if got != core.ResolutionCodeChange {
		t.Errorf("got %q, want %q", got, core.ResolutionCodeChange)
	}
}

func TestClassifyResolution_ScopedFileDeleted_ScopeDisappearance(t *testing.T) {
	exists := map[string]bool{"api/handler.go": true} // api/router.go deleted
	got := review.ClassifyResolution([]string{"api/handler.go", "api/router.go"}, func(p string) bool { return exists[p] })
	if got != core.ResolutionScopeDisappearance {
		t.Errorf("got %q, want %q", got, core.ResolutionScopeDisappearance)
	}
}

func TestClassifyResolution_NilFileExists_ConservativelyScopeDisappearance(t *testing.T) {
	got := review.ClassifyResolution([]string{"api/handler.go"}, nil)
	if got != core.ResolutionScopeDisappearance {
		t.Errorf("got %q, want %q (honest default when the caller can't check)", got, core.ResolutionScopeDisappearance)
	}
}

// ─── C2-1: findings bridge (AttachClaims / ApplyClaimStatus) ────────────────

// TestAttachClaims_OnlyOracleDimensionsPersist proves the persistence
// boundary: obligations findings (re-served claims — re-minting them is
// store corruption) and impact findings (info-severity context noise) NEVER
// become claims; only arch_conformance and spec_coverage do.
func TestAttachClaims_OnlyOracleDimensionsPersist(t *testing.T) {
	in := archViolationInput()
	in.ObligationsSupported = true
	in.ObligationsByFile = map[string][]review.ObligationRule{
		"api/handler.go": {{File: "api/handler.go", Rule: "Sort outputs.", Src: "CLAUDE.md#L1-L1"}},
	}
	v := review.Run(in)

	claims, _ := review.AttachClaims(&v, persistIn(nil))
	if len(claims) != 1 {
		t.Fatalf("expected exactly 1 claim (the arch violation), got %d: %+v", len(claims), claims)
	}
	if !strings.HasPrefix(claims[0].Subject, review.DimArch+"#") {
		t.Errorf("claim subject = %q, want %s#... prefix", claims[0].Subject, review.DimArch)
	}
	// Findings-as-evidence (docs/design/findings-as-evidence.md §3.2): the
	// finding's deterministically-assigned Class carries onto the
	// persisted claim, never re-derived or dropped by AttachClaims.
	if claims[0].Class != core.FindingClassConformance {
		t.Errorf("claim Class = %q, want %q", claims[0].Class, core.FindingClassConformance)
	}
	for _, name := range []string{review.DimImpact, review.DimObligations} {
		for _, f := range dimension(t, v, name).Findings {
			if f.ClaimID != "" {
				t.Errorf("%s finding must not carry a ClaimID, got %q", name, f.ClaimID)
			}
		}
	}
}

// TestAttachClaims_LinkSoundness proves every persisted link satisfies the
// serving substrate's contracts: a parseable citation with available
// content gets a cites link whose TargetHash is the REAL line hash
// (semantic.Sweep re-derives and matches it — zero flips); every claim
// carries scopes and scope-fp links (FT-6's drift anchor).
func TestAttachClaims_LinkSoundness(t *testing.T) {
	content := "l1\nl2\nl3\nl4\nl5\nl6\nl7-cited\n"
	v := review.Run(archViolationInput())
	claims, links := review.AttachClaims(&v, persistIn(map[string]string{"api/handler.go": content}))
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	id := claims[0].ClaimID

	var cites, scopes, scopeFP []core.ClaimLink
	for _, l := range links {
		switch l.Rel {
		case core.RelCites:
			cites = append(cites, l)
		case core.RelScopes:
			scopes = append(scopes, l)
		case core.RelScopeFP:
			scopeFP = append(scopeFP, l)
		}
	}
	if len(cites) != 1 || cites[0].ClaimID != id {
		t.Fatalf("expected 1 cites link for %s, got %+v", id, cites)
	}
	wantHash, ok := core.HashLines([]byte(content), 7, 7)
	if !ok {
		t.Fatal("HashLines failed on fixture content")
	}
	if cites[0].TargetHash != wantHash {
		t.Errorf("cites TargetHash = %q, want the real line hash %q", cites[0].TargetHash, wantHash)
	}
	// T5-1/O13: the findings bridge is the second live writer of cites
	// links (compile.go is the first) — it must also persist AnchorText so
	// a drifted finding citation can be re-anchored, not just staled.
	wantText, ok := core.LinesText([]byte(content), 7, 7)
	if !ok {
		t.Fatal("LinesText failed on fixture content")
	}
	if cites[0].AnchorText != wantText {
		t.Errorf("cites AnchorText = %q, want the real line text %q", cites[0].AnchorText, wantText)
	}
	if _, _, _, ok := core.ParseCiteRef(cites[0].TargetRef); !ok {
		t.Errorf("persisted cites TargetRef %q is not parseable — sweep would flip the claim contradicted", cites[0].TargetRef)
	}
	if len(scopes) != 1 || scopes[0].TargetRef != "api/handler.go" {
		t.Errorf("scopes link = %+v, want one exact-path link", scopes)
	}
	if len(scopeFP) != 1 || scopeFP[0].TargetHash != "fp-api/handler.go" {
		t.Errorf("scope-fp link = %+v, want one link carrying the store fingerprint", scopeFP)
	}
}

// TestAttachClaims_NoContentMeansNoCitesLink: when cited content can't be
// fetched, the claim persists WITHOUT a cites link (sweep leaves zero-cites
// claims fresh) — never with a hashless one (which sweep would flip).
func TestAttachClaims_NoContentMeansNoCitesLink(t *testing.T) {
	v := review.Run(archViolationInput())
	claims, links := review.AttachClaims(&v, persistIn(nil))
	if len(claims) != 1 {
		t.Fatalf("expected 1 claim, got %d", len(claims))
	}
	for _, l := range links {
		if l.Rel == core.RelCites {
			t.Errorf("no cites link should be written without fetchable content, got %+v", l)
		}
	}
}

// TestAttachClaims_NoFingerprintMeansNoPersist: a finding whose scoped file
// has no stored fingerprint is not persisted at all — a drift-tracked-layer
// claim without a scope-fp anchor would falsify DriftedFindings' invariant.
func TestAttachClaims_NoFingerprintMeansNoPersist(t *testing.T) {
	v := review.Run(archViolationInput())
	in := persistIn(nil)
	in.ScopeFingerprint = func(string) (string, bool) { return "", false }
	claims, links := review.AttachClaims(&v, in)
	if len(claims) != 0 || len(links) != 0 {
		t.Fatalf("expected nothing persisted without a fingerprint anchor, got %d claims / %d links", len(claims), len(links))
	}
	if got := dimension(t, v, review.DimArch).Findings[0].ClaimID; got != "" {
		t.Errorf("unpersisted finding must not carry a ClaimID, got %q", got)
	}
}

// TestFindingsBridge_RefutationDurability proves the pure-logic half of the
// C2-1 durable-findings guarantee: an unchanged finding re-emitted by a
// later review computes the SAME ClaimID, and once ApplyClaimStatus is told
// that ID is refuted (as it would be after a real `fathom claim refute` /
// ClaimFeedback write persisted via the store — see cmd/fathom's end-to-end
// review_test.go for the full store round trip), the finding carries
// Status "refuted" and is excluded from its dimension's active Count —
// never silently resurrected.
func TestFindingsBridge_RefutationDurability(t *testing.T) {
	// Review #1: emit the finding, attach claims.
	v1 := review.Run(archViolationInput())
	claims1, _ := review.AttachClaims(&v1, persistIn(nil))
	if len(claims1) == 0 {
		t.Fatal("expected at least one persisted claim")
	}
	id := dimension(t, v1, review.DimArch).Findings[0].ClaimID
	if id == "" {
		t.Fatal("expected a non-empty ClaimID after AttachClaims")
	}

	// Simulate an external refute (fathom claim refute / ClaimFeedback):
	// the store now holds this ClaimID with Status refuted.
	existing := []core.SemanticClaim{{ClaimID: id, Status: core.ClaimRefuted}}

	// Review #2: an unchanged finding re-emitted from the same diff must
	// compute the IDENTICAL ClaimID.
	v2 := review.Run(archViolationInput())
	claims2, _ := review.AttachClaims(&v2, persistIn(nil))
	id2 := dimension(t, v2, review.DimArch).Findings[0].ClaimID
	if id2 != id {
		t.Fatalf("ClaimID changed across identical re-review: %q vs %q", id, id2)
	}
	if len(claims2) != len(claims1) {
		t.Fatalf("claim count changed across identical re-review: %d vs %d", len(claims1), len(claims2))
	}

	review.ApplyClaimStatus(&v2, existing)
	d := dimension(t, v2, review.DimArch)
	if d.Findings[0].Status != string(core.ClaimRefuted) {
		t.Errorf("Status = %q, want %q", d.Findings[0].Status, core.ClaimRefuted)
	}
	if d.Count != 0 {
		t.Errorf("Count = %d, want 0 (the only finding is refuted)", d.Count)
	}
	// The finding must still be VISIBLE for audit, not dropped.
	if len(d.Findings) != 1 {
		t.Errorf("refuted finding should remain in Findings, got %d entries", len(d.Findings))
	}
}

func TestFindingsBridge_FreshFindingStaysActive(t *testing.T) {
	v := review.Run(archViolationInput())
	review.AttachClaims(&v, persistIn(nil))
	review.ApplyClaimStatus(&v, nil) // nothing persisted yet
	d := dimension(t, v, review.DimArch)
	if d.Findings[0].Status != "" {
		t.Errorf("Status = %q, want empty (never persisted before)", d.Findings[0].Status)
	}
	if d.Count != 1 {
		t.Errorf("Count = %d, want 1", d.Count)
	}
}

// ─── C2-1: reconcile (StaleReviewClaims) ────────────────────────────────────

func TestStaleReviewClaims_FixedFindingSuperseded(t *testing.T) {
	// Run whose arch dimension is CHECKED and clean (rule no longer
	// violated): a prior review claim on the changed file must be
	// superseded — absent = fixed.
	in := archViolationInput()
	in.Nodes[0].Calls = nil // the violating call was removed
	v := review.Run(in)
	review.AttachClaims(&v, persistIn(nil))

	prior := core.SemanticClaim{
		ClaimID:       "prior-arch-claim",
		Layer:         review.ClaimsLayer,
		PromptVersion: review.ClaimsPromptVersion,
		Subject:       review.DimArch + "#api.Handler",
		Scope:         []string{"api/handler.go"},
		Status:        core.ClaimFresh,
	}
	stale := review.StaleReviewClaims(v, []core.SemanticClaim{prior})
	if len(stale) != 1 || stale[0] != "prior-arch-claim" {
		t.Fatalf("expected the fixed finding's claim superseded, got %v", stale)
	}
}

func TestStaleReviewClaims_Boundaries(t *testing.T) {
	in := archViolationInput()
	in.Nodes[0].Calls = nil
	v := review.Run(in)
	review.AttachClaims(&v, persistIn(nil))

	base := core.SemanticClaim{
		Layer:         review.ClaimsLayer,
		PromptVersion: review.ClaimsPromptVersion,
		Subject:       review.DimArch + "#api.Handler",
		Scope:         []string{"api/handler.go"},
		Status:        core.ClaimFresh,
	}

	cases := []struct {
		name   string
		mutate func(*core.SemanticClaim)
		want   int
	}{
		{"corpus claim (different PromptVersion) untouched", func(c *core.SemanticClaim) { c.PromptVersion = "findings/content-addressed" }, 0},
		{"refuted claim untouched (durability)", func(c *core.SemanticClaim) { c.Status = core.ClaimRefuted }, 0},
		{"already-superseded claim untouched", func(c *core.SemanticClaim) { c.Status = core.ClaimSuperseded }, 0},
		{"file outside changed set untouched", func(c *core.SemanticClaim) { c.Scope = []string{"other/file.go"} }, 0},
		{"eligible claim superseded", func(c *core.SemanticClaim) {}, 1},
	}
	for _, tc := range cases {
		cl := base
		cl.ClaimID = "cl-" + tc.name
		tc.mutate(&cl)
		if got := len(review.StaleReviewClaims(v, []core.SemanticClaim{cl})); got != tc.want {
			t.Errorf("%s: got %d supersedes, want %d", tc.name, got, tc.want)
		}
	}
}

// TestStaleReviewClaims_UncheckedDimensionNeverSupersedes: absent because
// unchecked is not absent because fixed.
func TestStaleReviewClaims_UncheckedDimensionNeverSupersedes(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"api/handler.go"},
		Nodes:        fixtureNodes(),
		ArchChecked:  false,
		ArchReason:   "no rules configured",
	})
	review.AttachClaims(&v, persistIn(nil))
	prior := core.SemanticClaim{
		ClaimID:       "prior-arch-claim",
		Layer:         review.ClaimsLayer,
		PromptVersion: review.ClaimsPromptVersion,
		Subject:       review.DimArch + "#api.Handler",
		Scope:         []string{"api/handler.go"},
		Status:        core.ClaimFresh,
	}
	if stale := review.StaleReviewClaims(v, []core.SemanticClaim{prior}); len(stale) != 0 {
		t.Fatalf("an UNCHECKED dimension must never supersede its claims, got %v", stale)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────

func dimension(t *testing.T, v review.Verdict, name string) review.Dimension {
	t.Helper()
	for _, d := range v.Dimensions {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("dimension %q not found in verdict: %+v", name, v)
	return review.Dimension{}
}
