package scope_test

// surface_test.go — SP-2: the read chokepoint covers EVERY surface.
//
// Every fathom surface — MCP tool, CLI verb, HTTP route, the observatory's
// counts, the cross-repo platform graph — reaches indexed content through
// store.Store, whose implementations filter inside their own read methods.
// The surface enumeration is therefore the interface's own method set: the
// disposition map below must classify every method as filtered or
// exempt-with-a-written-reason, and the behavioral half proves each
// filtered method actually drops a suppressed ref on BOTH backends. A
// method added to store.Store without a classification fails here rather
// than quietly bypassing the filter. (The constructor half of the audit —
// no constructor can return an unfiltered store — is
// store.TestEveryStoreConstructorIsScoped, V3.)

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	svc        = "svc"
	hiddenRef  = "secrets/token.go"
	visibleRef = "internal/ok.go"
)

var fixedTime = time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

// storeMethodDisposition classifies EVERY method of store.Store for the
// read chokepoint (SP-2, §2.2). The point of the map is that it is
// exhaustive: TestSuppressionCoversEverySurface fails when the interface
// grows a method nobody classified, so a new content-bearing read cannot be
// added without either filtering it or writing down why.
//
// "" (empty string) means "this read carries path-anchored content and MUST
// be filtered"; any other value is the recorded reason it is exempt.
var storeMethodDisposition = map[string]string{
	// --- filtered: path-anchored content leaves the store here ---
	"QueryObservations": "",
	"DocSummaries":      "",
	"KnowledgeCards":    "",
	"TypedEdges":        "",
	"ListSourceRefs":    "",
	"ClaimLinks":        "",

	// --- exempt, with the reason ---
	"PutObservations":                "write path; suppression is a READ decision and must never change what is indexed (§3.5)",
	"PutKnowledgeCards":              "write path",
	"PutDocSummaries":                "write path",
	"PutClaims":                      "write path",
	"PutTypedEdges":                  "write path",
	"SetFingerprint":                 "write path",
	"SetServiceMeta":                 "write path",
	"SetCommits":                     "write path",
	"SetClaimStatus":                 "write path",
	"SetClaimValidTo":                "write path",
	"AppendClaimLink":                "write path",
	"SetTypedEdgePackageFingerprint": "write path",
	"UpdateClaimLink":                "write path",
	"AppendClaimEvent":               "write path",
	"RecordCompileRun":               "write path",
	"PurgeSource":                    "delete path",
	"PurgeService":                   "delete path",
	"RemoveService":                  "delete path",
	"RenameService":                  "delete/rename path",
	"DeleteFingerprint":              "delete path",
	"DeleteTypedEdgePackage":         "delete path",
	"Fingerprint":                    "ingest hash-gate read: filtering it would make the pipeline re-extract a suppressed file on every run, and suppression must not change ingest (§3.5)",
	"CountAbsoluteSourceRefs":        "a count of anchor STYLE for the self-heal, carries no content",
	"ListServiceIDs":                 "service identity, no path-anchored content",
	"ClaimServiceIDs":                "service identity, no path-anchored content",
	"GetServiceMeta":                 "repo path + commit, service identity rather than indexed content",
	"ListCommits":                    "git history, not indexed source content; suppressing a path never rewrites the repository's own log",
	"ClaimsForService":               "claims are anchored through claim_links, which IS filtered; the claim row itself carries no path",
	"ClaimsForServe":                 "SU-2: ClaimsForService's prefiltered sibling — same reasoning, same claim rows, only the WHERE narrowing differs; claims are anchored through claim_links, which IS filtered",
	"ClaimEvents":                    "append-only verdict log keyed by claim id, no path and no content",
	"ClaimEventsSince":               "append-only verdict log keyed by claim id, no path and no content",
	"CompileRuns":                    "compile metadata (tokens, cost, watermark), no content",
	"TypedEdgePackageFingerprint":    "per-package content hash for the enrichment cache, no content",
	"ListTypedEdgePackages":          "package identifiers for the enrichment reconcile, no file paths",
	"PutExternalFinding":             "write path",
	"ExternalFinding":                "cursor/provenance row keyed by claim_id (FL-7); the claim it belongs to is anchored through claim_links, which IS filtered -- this is supplementary source-linked metadata for an already-filtered claim, not the primary path-anchored read",
	"ExternalFindingsForService":     "cursor/provenance rows keyed by claim_id (FL-7); same reasoning as ExternalFinding",
	"RecordReviewRun":                "write path",
	"ReviewRun":                      "idempotency cache row keyed by service+key, no path-anchored content",
	"Close":                          "lifecycle",
}

func seedSurfaceStore(t *testing.T, s store.Store) {
	t.Helper()
	ctx := context.Background()
	now := fixedTime

	obs := []core.Observation{
		{ServiceID: svc, SourceRef: hiddenRef, SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Secret", Package: "secrets", Claim: "func Secret()", IngestedAt: now},
		{ServiceID: svc, SourceRef: visibleRef, SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Ok", Package: "internal", Claim: "func Ok()", IngestedAt: now},
	}
	if err := s.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	for _, ref := range []string{hiddenRef, visibleRef} {
		if err := s.SetFingerprint(ctx, svc, ref, "h-"+ref); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	if err := s.PutDocSummaries(ctx, []store.DocSummary{
		{ServiceID: svc, SourceRef: hiddenRef, Title: "secret", Summary: "the key material"},
		{ServiceID: svc, SourceRef: visibleRef, Title: "ok", Summary: "harmless"},
	}); err != nil {
		t.Fatalf("PutDocSummaries: %v", err)
	}
	if err := s.PutKnowledgeCards(ctx, []store.KnowledgeCard{
		{ServiceID: svc, Symbol: "Secret", File: hiddenRef, Summary: "summarises the key material"},
		{ServiceID: svc, Symbol: "Ok", File: visibleRef, Summary: "harmless"},
	}); err != nil {
		t.Fatalf("PutKnowledgeCards: %v", err)
	}
	if err := s.PutTypedEdges(ctx, svc, "secrets", []core.Edge{
		{FromService: svc, FromFile: hiddenRef, FromSymbol: "secrets.Secret", ToSymbol: "fmt.Println", Kind: core.EdgeDirect},
	}); err != nil {
		t.Fatalf("PutTypedEdges: %v", err)
	}
	if err := s.PutTypedEdges(ctx, svc, "internal", []core.Edge{
		{FromService: svc, FromFile: visibleRef, FromSymbol: "internal.Ok", ToSymbol: "fmt.Println", Kind: core.EdgeDirect},
	}); err != nil {
		t.Fatalf("PutTypedEdges: %v", err)
	}
	claims := []core.SemanticClaim{
		{ServiceID: svc, ClaimID: "c-hidden", Layer: "l", Subject: "s", Rule: "r"},
		{ServiceID: svc, ClaimID: "c-visible", Layer: "l", Subject: "s", Rule: "r"},
	}
	links := []core.ClaimLink{
		{ClaimID: "c-hidden", Rel: core.RelCites, TargetRef: core.CiteRef(hiddenRef, 1, 3), AnchorText: "the key material"},
		{ClaimID: "c-visible", Rel: core.RelCites, TargetRef: core.CiteRef(visibleRef, 1, 3), AnchorText: "harmless"},
	}
	if err := s.PutClaims(ctx, claims, links); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
}

// assertRefs fails when refs contains the hidden ref or misses the visible
// one — the shared assertion for every filtered surface.
func assertRefs(t *testing.T, refs []string) {
	t.Helper()
	var sawVisible bool
	for _, r := range refs {
		if r == "" {
			continue
		}
		if scope.NormalizeRef(r) == hiddenRef {
			t.Fatalf("a suppressed ref is served: %v", refs)
		}
		if scope.NormalizeRef(r) == visibleRef {
			sawVisible = true
		}
	}
	if !sawVisible {
		t.Fatalf("the filter took the visible ref too: %v", refs)
	}
}

func refsOf[T any](items []T, refOf func(T) string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, refOf(it))
	}
	return out
}

// TestSuppressionCoversEverySurface is SP-2. The disposition map must
// classify every store.Store method, and every method classified as
// filtered is proven, behaviorally, to drop a suppressed ref — on the
// SQLite backend and the memory fake both, since every MCP tool, CLI verb
// and HTTP route reads through one of them.
func TestSuppressionCoversEverySurface(t *testing.T) {
	iface := reflect.TypeOf((*store.Store)(nil)).Elem()

	var unclassified []string
	seen := map[string]bool{}
	for i := 0; i < iface.NumMethod(); i++ {
		name := iface.Method(i).Name
		seen[name] = true
		if _, ok := storeMethodDisposition[name]; !ok {
			unclassified = append(unclassified, name)
		}
	}
	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Fatalf("store.Store method(s) %v have no suppression disposition — "+
			"classify each as filtered (\"\") or exempt-with-a-reason in storeMethodDisposition; "+
			"a content-bearing read that nobody classified is a silent bypass of the read chokepoint",
			unclassified)
	}
	for name, reason := range storeMethodDisposition {
		if !seen[name] {
			t.Errorf("storeMethodDisposition names %q, which store.Store no longer has", name)
		}
		if reason == "" {
			continue
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s is exempt with a blank reason", name)
		}
	}

	// The pure chokepoint itself: a drop always yields a disclosure. This
	// is also the direct tie from this surface test to ApplySuppressions —
	// the single function every filtered read above funnels through.
	set, err := scope.NewSet([]scope.Suppression{{
		ID: "s1", Service: svc, Pattern: "secrets/**", Reason: "key material",
		CreatedAt: fixedTime, CreatedBy: scope.OriginCLI,
	}})
	if err != nil {
		t.Fatal(err)
	}
	kept, d := scope.ApplySuppressions(set, svc, fixedTime, []string{hiddenRef, visibleRef}, func(s string) string { return s })
	if len(kept) != 1 || d == nil || d.Count != 1 {
		t.Fatalf("ApplySuppressions kept=%v d=%+v", kept, d)
	}

	// Behavioral pass over both backends, through the environment-armed
	// per-call gate — the exact wiring every shipped surface uses.
	supPath := filepath.Join(t.TempDir(), "suppressions.jsonl")
	t.Setenv(scope.EnvVar, supPath)
	if _, err := scope.AppendSuppression(supPath, scope.Suppression{
		Service: svc, Pattern: "secrets/**", Reason: "key material",
		CreatedAt: time.Now().UTC(), CreatedBy: scope.OriginCLI,
	}); err != nil {
		t.Fatal(err)
	}

	backends := []struct {
		name string
		make func(t *testing.T) store.Store
	}{
		{"sqlite", func(t *testing.T) store.Store {
			s, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		}},
		{"memory", func(t *testing.T) store.Store { return store.NewMemoryStore() }},
	}

	ctx := context.Background()
	for _, b := range backends {
		t.Run(b.name, func(t *testing.T) {
			g := b.make(t)
			seedSurfaceStore(t, g)

			t.Run("QueryObservations", func(t *testing.T) {
				got, err := g.QueryObservations(ctx, svc, "")
				if err != nil {
					t.Fatal(err)
				}
				assertRefs(t, refsOf(got, func(o core.Observation) string { return o.SourceRef }))
			})
			t.Run("DocSummaries", func(t *testing.T) {
				got, err := g.DocSummaries(ctx, svc)
				if err != nil {
					t.Fatal(err)
				}
				assertRefs(t, refsOf(got, func(d store.DocSummary) string { return d.SourceRef }))
			})
			t.Run("KnowledgeCards", func(t *testing.T) {
				got, err := g.KnowledgeCards(ctx, svc)
				if err != nil {
					t.Fatal(err)
				}
				assertRefs(t, refsOf(got, func(c store.KnowledgeCard) string { return c.File }))
			})
			t.Run("TypedEdges", func(t *testing.T) {
				got, err := g.TypedEdges(ctx, svc)
				if err != nil {
					t.Fatal(err)
				}
				assertRefs(t, refsOf(got, func(e core.Edge) string { return e.FromFile }))
			})
			t.Run("ListSourceRefs", func(t *testing.T) {
				got, err := g.ListSourceRefs(ctx, svc)
				if err != nil {
					t.Fatal(err)
				}
				assertRefs(t, got)
			})
			t.Run("ClaimLinks", func(t *testing.T) {
				got, err := g.ClaimLinks(ctx, svc, []string{"c-hidden", "c-visible"})
				if err != nil {
					t.Fatal(err)
				}
				assertRefs(t, refsOf(got, func(l core.ClaimLink) string {
					if p, _, _, ok := core.ParseCiteRef(l.TargetRef); ok {
						return p
					}
					return ""
				}))
			})
			// The exempt hash-gate read still serves the fingerprint: a
			// reversible READ decision must never change what ingest sees.
			t.Run("Fingerprint stays exempt", func(t *testing.T) {
				if _, found, err := g.Fingerprint(ctx, svc, hiddenRef); err != nil || !found {
					t.Fatalf("the ingest hash-gate lost the suppressed file's fingerprint (found=%v err=%v) — suppression must not change what gets indexed", found, err)
				}
			})
		})
	}
}
