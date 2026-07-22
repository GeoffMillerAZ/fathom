package fusion

// classify_test.go — white-box (package fusion, not fusion_test) unit tests
// for the pass-3 tool-identity classifier's building blocks: Classify's
// per-family dispatch, the sibling index, and the image↔Dockerfile cross-ref
// (ADR-021 §2 / TI-1, TI-7). Full-pipeline behavior (dispatch actually wired
// into extractAndStore) is covered by toolident_test.go.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func TestClassify_K8sManifest(t *testing.T) {
	content := []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: x\n")
	if got := Classify("k8s/deployment.yaml", content, nil); got != ToolK8sManifest {
		t.Errorf("Classify = %q, want %q", got, ToolK8sManifest)
	}
}

func TestClassify_CRD(t *testing.T) {
	content := []byte("apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\n")
	if got := Classify("crds/widget.yaml", content, nil); got != ToolK8sCRD {
		t.Errorf("Classify = %q, want %q", got, ToolK8sCRD)
	}
}

func TestClassify_Compose_AnyFilename(t *testing.T) {
	content := []byte("services:\n  web:\n    image: nginx\n")
	if got := Classify("any-name.yml", content, nil); got != ToolCompose {
		t.Errorf("Classify = %q, want %q", got, ToolCompose)
	}
}

func TestClassify_Compose_TrailingCommentAllowed(t *testing.T) {
	// Critic F5: `services: # comment` is valid compose — the end-of-line
	// anchor must tolerate trailing whitespace and comments.
	content := []byte("services: # the app's services\n  web:\n    image: nginx\n")
	if got := Classify("compose-ish.yml", content, nil); got != ToolCompose {
		t.Errorf("Classify with trailing comment = %q, want %q", got, ToolCompose)
	}
	content2 := []byte("services:   \n  web:\n    image: nginx\n")
	if got := Classify("compose-ish2.yml", content2, nil); got != ToolCompose {
		t.Errorf("Classify with trailing whitespace = %q, want %q", got, ToolCompose)
	}
}

func TestClassify_OpenAPI(t *testing.T) {
	content := []byte("openapi: 3.0.0\npaths: {}\n")
	if got := Classify("api.yaml", content, nil); got != ToolOpenAPI {
		t.Errorf("Classify = %q, want %q", got, ToolOpenAPI)
	}
	content2 := []byte("swagger: '2.0'\npaths: {}\n")
	if got := Classify("api2.yaml", content2, nil); got != ToolOpenAPI {
		t.Errorf("Classify = %q, want %q", got, ToolOpenAPI)
	}
}

func TestClassify_Unclassified(t *testing.T) {
	content := []byte("logging:\n  level: debug\nretries: 3\n")
	if got := Classify("config/app.yaml", content, nil); got != ToolUnclassified {
		t.Errorf("Classify = %q, want %q", got, ToolUnclassified)
	}
}

func TestClassify_HelmTemplateYAML_RequiresChartAncestor(t *testing.T) {
	content := []byte("apiVersion: v1\nkind: ConfigMap\n")
	idx := &siblingIndex{chartDirs: map[string]struct{}{"mychart": {}}}
	if got := Classify("mychart/templates/configmap.yaml", content, idx); got != ToolHelmTemplateYAML {
		t.Errorf("Classify = %q, want %q (chart ancestor present)", got, ToolHelmTemplateYAML)
	}
	// Same templates/ path shape, but no chart ancestor recorded — should
	// fall through to content classification instead.
	if got := Classify("orphan/templates/configmap.yaml", content, &siblingIndex{}); got != ToolK8sManifest {
		t.Errorf("Classify = %q, want %q (no chart ancestor, falls back to content)", got, ToolK8sManifest)
	}
}

func TestClassify_CUE_Timoni(t *testing.T) {
	if got := Classify("timoni.cue", []byte("package main\n"), nil); got != ToolTimoni {
		t.Errorf("Classify = %q, want %q (basename)", got, ToolTimoni)
	}
	if got := Classify("bundle.cue", []byte("bundle: {\n\tapiVersion: \"v1alpha1\"\n}\n"), nil); got != ToolTimoni {
		t.Errorf("Classify = %q, want %q (bundle marker)", got, ToolTimoni)
	}
}

func TestClassify_CUE_Plain(t *testing.T) {
	if got := Classify("schema.cue", []byte("package foo\n\n#X: {}\n"), nil); got != ToolCUEPlain {
		t.Errorf("Classify = %q, want %q", got, ToolCUEPlain)
	}
}

func TestClassify_NonYAMLCUEExtension_Unclassified(t *testing.T) {
	if got := Classify("main.go", []byte("package main\n"), nil); got != ToolUnclassified {
		t.Errorf("Classify = %q, want %q (Classify is never meaningfully called for non-yaml/cue exts)", got, ToolUnclassified)
	}
}

// ─── siblingIndex ───────────────────────────────────────────────────────────

func TestBuildSiblingIndex_ChartAndDockerfileAndCueMod(t *testing.T) {
	refs := []core.SourceRef{
		{Path: "mychart/Chart.yaml"},
		{Path: "mychart/templates/deployment.yaml"},
		{Path: "myapp/Dockerfile"},
		{Path: "Dockerfile"},
		{Path: "build/otherapp.dockerfile"},
		{Path: "cue.mod/module.cue"},
		{Path: "cue.mod/pkg/schema.cue"},
	}
	idx := buildSiblingIndex(refs)

	if !idx.hasChartAncestor("mychart/templates") {
		t.Error("expected mychart/templates to have a Chart.yaml ancestor")
	}
	if idx.hasChartAncestor("other/templates") {
		t.Error("other/templates should NOT have a Chart.yaml ancestor")
	}
	if !idx.hasCueModAncestor(".") {
		t.Error("expected root '.' to have a cue.mod ancestor (module root)")
	}
	if got, ok := idx.dockerfileByDirName["myapp"]; !ok || got != "myapp/Dockerfile" {
		t.Errorf("dockerfileByDirName[\"myapp\"] = %q, ok=%v; want \"myapp/Dockerfile\", true", got, ok)
	}
	// *.dockerfile keys by STEM, never by directory name (critic F3): the
	// file names the image it builds.
	if got, ok := idx.dockerfileByStem["otherapp"]; !ok || got != "build/otherapp.dockerfile" {
		t.Errorf("dockerfileByStem[\"otherapp\"] = %q, ok=%v; want \"build/otherapp.dockerfile\", true", got, ok)
	}
	if _, ok := idx.dockerfileByDirName["build"]; ok {
		t.Error("build/otherapp.dockerfile must not be keyed under its directory name")
	}
	// The root Dockerfile is recorded separately, never under ".".
	if idx.rootDockerfile != "Dockerfile" {
		t.Errorf("rootDockerfile = %q, want \"Dockerfile\"", idx.rootDockerfile)
	}
	if _, ok := idx.dockerfileByDirName["."]; ok {
		t.Error("root Dockerfile must not be keyed under \".\"")
	}
	if idx.dockerfileCount != 3 {
		t.Errorf("dockerfileCount = %d, want 3", idx.dockerfileCount)
	}
}

func TestSiblingIndex_NilIsSafeDefault(t *testing.T) {
	var idx *siblingIndex
	if idx.hasChartAncestor("anything") {
		t.Error("nil siblingIndex must report no chart ancestor")
	}
	if idx.hasCueModAncestor("anything") {
		t.Error("nil siblingIndex must report no cue.mod ancestor")
	}
	if got := crossRefImageToDockerfile("svc", "f.yaml", nil, idx); got != nil {
		t.Errorf("crossRefImageToDockerfile with nil index should return nil, got %v", got)
	}
}

// ─── cross-refs ─────────────────────────────────────────────────────────────

func TestImageBaseName(t *testing.T) {
	cases := map[string]string{
		"myapp":                             "myapp",
		"myapp:1.2.3":                       "myapp",
		"ghcr.io/org/myapp:1.2.3":           "myapp",
		"myapp@sha256:deadbeef":             "myapp",
		"registry.example.com:5000/x/myapp": "myapp",
		"":                                  "",
	}
	for in, want := range cases {
		if got := imageBaseName(in); got != want {
			t.Errorf("imageBaseName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCrossRefImageToDockerfile_MatchAndNoMatch(t *testing.T) {
	idx := buildSiblingIndex([]core.SourceRef{{Path: "myapp/Dockerfile"}, {Path: "other/Dockerfile"}})
	nodes := []core.SymbolNode{
		{Name: "myapp:1.0", Kind: core.KindK8sContainer, Line: 5},
		{Name: "unrelated:1.0", Kind: core.KindK8sContainer, Line: 6},
	}
	refs := crossRefImageToDockerfile("svc", "k8s/deploy.yaml", nodes, idx)
	if len(refs) != 1 {
		t.Fatalf("expected 1 cross-ref, got %d: %+v", len(refs), refs)
	}
	if refs[0].Name != "myapp" || refs[0].Kind != core.KindImageDockerfileRef {
		t.Errorf("unexpected cross-ref: %+v", refs[0])
	}
	if refs[0].Tier != core.TierPreScan {
		t.Errorf("cross-ref Tier = %q, want TierPreScan", refs[0].Tier)
	}
}

func TestCrossRefImageToDockerfile_StemBeatsDirName(t *testing.T) {
	// A *.dockerfile stem match wins over a bare Dockerfile's dir-name
	// match for the same image name.
	idx := buildSiblingIndex([]core.SourceRef{
		{Path: "build/myapp.dockerfile"},
		{Path: "myapp/Dockerfile"},
	})
	nodes := []core.SymbolNode{{Name: "myapp:1.0", Kind: core.KindK8sContainer, Line: 3}}
	refs := crossRefImageToDockerfile("svc", "k8s/deploy.yaml", nodes, idx)
	if len(refs) != 1 {
		t.Fatalf("expected 1 cross-ref, got %d: %+v", len(refs), refs)
	}
	if refs[0].Signature != "dockerfile: build/myapp.dockerfile" {
		t.Errorf("Signature = %q, want the stem-matched *.dockerfile to win", refs[0].Signature)
	}
}

func TestCrossRefImageToDockerfile_ComposeBuildOnlySkipped(t *testing.T) {
	idx := buildSiblingIndex([]core.SourceRef{{Path: "web/Dockerfile"}})
	nodes := []core.SymbolNode{
		{Name: "web", Kind: core.KindComposeService, Signature: "build: ./web"},
	}
	if refs := crossRefImageToDockerfile("svc", "compose.yml", nodes, idx); len(refs) != 0 {
		t.Errorf("a build-only compose service (no image:) should not cross-ref, got %+v", refs)
	}
}

// TestK8sExtractorToolKindParity pins the string parity between fusion's
// classifier constants and the extractor's per-document ToolKind values
// (extractor cannot import fusion, so the shared values are duplicated
// literals — this test is what keeps them from drifting apart).
func TestK8sExtractorToolKindParity(t *testing.T) {
	if string(ToolK8sManifest) != extractor.ToolKindK8sManifest {
		t.Errorf("ToolK8sManifest %q != extractor.ToolKindK8sManifest %q", ToolK8sManifest, extractor.ToolKindK8sManifest)
	}
	if string(ToolK8sCRD) != extractor.ToolKindK8sCRD {
		t.Errorf("ToolK8sCRD %q != extractor.ToolKindK8sCRD %q", ToolK8sCRD, extractor.ToolKindK8sCRD)
	}
}

func TestToolKindForBasename(t *testing.T) {
	cases := map[string]ToolKind{
		"Chart.yaml":          ToolHelmChart,
		"values.yaml":         ToolHelmValues,
		"kustomization.yaml":  ToolKustomize,
		"kustomization.yml":   ToolKustomize,
		"docker-compose.yml":  ToolCompose,
		"compose.yaml":        ToolCompose,
		"unrelated-name.yaml": "",
	}
	for base, want := range cases {
		if got := toolKindForBasename(base); got != want {
			t.Errorf("toolKindForBasename(%q) = %q, want %q", base, got, want)
		}
	}
}
