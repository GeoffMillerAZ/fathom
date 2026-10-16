package gallery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	catalogpkg "github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/qualityplan"
)

func TestQualityJSONMatchesReaderAcrossScopesAndDetails(t *testing.T) {
	h := New()
	plan, err := qualityplan.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"core", "on-demand", "all"} {
		t.Run(scope, func(t *testing.T) {
			want, err := plan.List("", scope)
			if err != nil {
				t.Fatal(err)
			}
			wantJSON, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			res, body := get(t, h, "/api/quality?scope="+url.QueryEscape(scope))
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status %d: %s", res.StatusCode, body)
			}
			if body != string(wantJSON) {
				t.Fatalf("list JSON differs from reader\nwant: %s\n got: %s", wantJSON, body)
			}
		})
	}
	list, err := plan.List("", "all")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list.Items {
		t.Run(item.ID, func(t *testing.T) {
			want, err := plan.Get(item.ID, list.Revision)
			if err != nil {
				t.Fatal(err)
			}
			wantJSON, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			path := "/api/quality/" + item.ID + "?expected_revision=" + list.Revision
			res, body := get(t, h, path)
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status %d: %s", res.StatusCode, body)
			}
			if body != string(wantJSON) {
				t.Fatalf("detail JSON differs from reader")
			}
		})
	}
}

func TestQualityRequestsAndHTMLBoundaries(t *testing.T) {
	h := New()
	for _, tc := range []struct {
		path   string
		status int
		want   string
	}{
		{"/quality?scope=core&theme=dark", 200, `data-theme="dark"`},
		{"/quality?scope=on-demand", 200, "On-demand work"},
		{"/quality?scope=all&capability_id=analytics.comparison", 200, "core.analytics"},
		{"/quality?scope=core&capability_id=reference.pricing-adapter", 200, "No recorded work matches"},
		{"/quality?scope=missing", 400, "needs a reset"},
		{"/quality?scope=core&scope=all", 400, "needs a reset"},
		{"/quality?unknown=x", 400, "needs a reset"},
		{"/quality?theme=purple", 400, "needs a reset"},
		{"/quality?expected_revision=bad", 400, "needs a reset"},
		{"/quality/not-real", 404, "needs a reset"},
		{"/quality/core.analytics?expected_revision=bad", 400, "needs a reset"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res, body := get(t, h, tc.path)
			if res.StatusCode != tc.status {
				t.Fatalf("status %d: %s", res.StatusCode, body)
			}
			if !strings.Contains(strings.ToLower(body), strings.ToLower(tc.want)) {
				t.Fatalf("missing %q in %s", tc.want, body)
			}
		})
	}
	for _, path := range []string{"/api/quality?scope=core&scope=all", "/api/quality?unknown=x", "/api/quality?expected_revision=bad", "/api/quality/core.analytics?expected_revision=bad"} {
		res, _ := get(t, h, path)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
	}
	for _, path := range []string{"/api/quality/not-real", "/api/quality/core.analytics?capability_id=unknown.capability"} {
		res, _ := get(t, h, path)
		if res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
	}

	_, list := get(t, h, "/quality?scope=all&capability_id=analytics.comparison&theme=dark")
	for _, want := range []string{"name=\"scope\"", "name=\"capability_id\"", "name=\"theme\" value=\"dark\"", "min-h-11", "authored-planning-not-certification", "No score or readiness count"} {
		if !strings.Contains(list, want) {
			t.Errorf("list missing %q", want)
		}
	}
	for _, want := range []string{"max-w-full", "min-w-0", "whitespace-normal", "break-all", "focus-visible:outline-2", "focus-visible:outline-offset-2"} {
		if !strings.Contains(list, want) {
			t.Errorf("narrow quality item link regression: missing %q", want)
		}
	}
	plan, err := qualityplan.Default()
	if err != nil {
		t.Fatal(err)
	}
	core, err := plan.List("", "core")
	if err != nil {
		t.Fatal(err)
	}
	path := "/quality/core.analytics?scope=all&theme=dark&expected_revision=" + core.Revision
	_, detail := get(t, h, path)
	for _, want := range []string{"Seven enhancement standards", "Nine recorded gates", "Authoring references", "Recorded consumers", "Reason:", "Next step:", "no source body is served", "authored planning, not certification", "theme=dark", "scope=all", "← Quality work", "focus-visible:outline-2", "focus-visible:outline-offset-2"} {
		if !strings.Contains(strings.ToLower(detail), strings.ToLower(want)) {
			t.Errorf("detail missing %q", want)
		}
	}
	if strings.Contains(strings.ToLower(detail), "ready score") || strings.Contains(strings.ToLower(detail), "certified") {
		t.Error("detail implies an unsupported readiness or certification claim")
	}
	_, catalog := get(t, h, "/catalog?theme=dark")
	if !strings.Contains(catalog, `href="/quality?theme=dark"`) {
		t.Error("catalog must link to quality work while preserving theme")
	}
	_, entry := get(t, h, "/catalog/analytics.comparison?theme=dark")
	current, err := catalogpkg.Default()
	if err != nil {
		t.Fatal(err)
	}
	wantQuality := `/quality?capability_id=analytics.comparison&amp;expected_catalog_revision=` + current.Revision + `&amp;theme=dark`
	if !strings.Contains(entry, `href="`+wantQuality+`"`) {
		t.Error("catalog entry must link to its quality work while preserving theme")
	}
}

func TestQualityEscapesFilterAndReferenceText(t *testing.T) {
	h := New()
	res, body := get(t, h, "/quality?capability_id="+url.QueryEscape(`<script>alert(1)</script>`))
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
	if strings.Contains(body, "<script>") {
		t.Fatal("invalid filter became active markup")
	}

	req := httptest.NewRequest(http.MethodGet, "/quality/core.analytics?scope=all", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `href="docs/`) {
		t.Fatal("reference paths must be plain text, not file-server links")
	}
}

func TestQualityDetailOptionalMetadataUsesNativeDisclosure(t *testing.T) {
	view := qualityDetailView{Result: qualityplan.DetailResult{
		Item: qualityplan.Item{
			ID: "fixture.metadata",
			Consumers: []qualityplan.Consumer{{
				App: "fixture-app", Revision: qualityplan.ConsumerRevision{Kind: "artifact", SHA256: "fixture-artifact"}, IntegrationStatus: "verified-scoped",
				Evidence: []qualityplan.Ref{{Path: "docs/<consumer>", Note: "<scoped evidence>"}},
				Receipt:  &qualityplan.PinnedRef{Path: `receipts/<consumer>`, SHA256: "fixture-receipt"},
				Review:   &qualityplan.Review{By: `<reviewer>`, Ref: `reviews/<review>`},
			}},
			Standards: map[string]qualityplan.StandardStatus{"typed-config": {
				Status: "partial", Evidence: []qualityplan.Ref{{Path: "docs/<evidence>", Note: "<note>"}}, Review: &qualityplan.Review{By: "reviewer", Ref: "review/<ref>"},
			}},
			Gates: map[string]qualityplan.GateStatus{"security": {
				Status: "not-run", Evidence: []qualityplan.Ref{{Path: "docs/gate", Note: "gate note"}}, Receipt: &qualityplan.PinnedRef{Path: "receipts/check", SHA256: "abc123"}, SourceRevision: "source-identity", ArtifactRevision: "artifact-identity", Review: &qualityplan.Review{By: "reviewer", Ref: "review/ref"},
			}},
			SourceRefs: []qualityplan.Ref{{Path: "docs/source", Note: "source note"}},
			Selection: &qualityplan.ConversionSelection{
				Original:           qualityplan.OriginalSelection{Kind: "reference", ID: "original.id", RevisionSHA256: "original-sha"},
				Target:             qualityplan.TargetSelection{ID: "target.id", RevisionSHA256: "target-sha"},
				AcceptanceContract: qualityplan.PinnedRef{Path: "docs/contract", SHA256: "contract-sha"}, Workload: "fixture workload",
			},
		},
		Standards: map[string]qualityplan.StandardDefinition{"typed-config": {Name: "Typed configuration", Requirement: "fixture requirement"}},
	}}
	body, err := htmlf("quality-optional-metadata", qualityDetailHTML, view)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"<details", "<summary class=\"min-h-11", "Evidence and review metadata", "Evidence and gate metadata", "Source references", "Selected IDs and acceptance contract", "Receipt: receipts/check: abc123", "Source revision: source-identity", "Artifact revision: artifact-identity", "Acceptance contract: docs/contract: contract-sha", "docs/&lt;evidence&gt;: &lt;note&gt;", "fixture-app: verified-scoped", "artifact: fixture-artifact", "docs/&lt;consumer&gt;: &lt;scoped evidence&gt;", "Receipt and review metadata", "Receipt: receipts/&lt;consumer&gt;: fixture-receipt", "Review: &lt;reviewer&gt;: reviews/&lt;review&gt;"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(text, `href="docs/`) || strings.Contains(text, `href="receipts/`) || strings.Contains(text, `href="reviews/`) || strings.Contains(text, "<reviewer>") || strings.Contains(text, "<consumer>") {
		t.Error("metadata references must remain escaped plain text")
	}
	view.Result.Item.Consumers = nil
	empty, err := htmlf("quality-empty-consumers", qualityDetailHTML, view)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(empty), "No recorded consumer assertion. This is not a claim that no consumer exists.") || strings.Contains(string(empty), "Receipt and review metadata") {
		t.Fatal("empty consumer state must remain explicit without invented adoption metadata")
	}
}
