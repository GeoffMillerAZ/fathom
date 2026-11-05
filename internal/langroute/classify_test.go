package langroute

// classify_test.go — white-box unit tests for the pass-3 tool-identity
// classifier's building blocks: Classify's per-family dispatch and the sibling
// index (ADR-021 §2 / TI-1). Full-pipeline behavior (dispatch actually wired
// into extractAndStore) is covered by internal/fusion's toolident_test.go.

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

// O324: a top-level `services:` holding a LIST (a GitLab CI pipeline's
// deprecated global services, under a name the gitlab-ci dialect does not
// claim) is not a Compose file: Compose's services is a mapping. The
// docker-compose dialect gives every file routed to it a ComposeFile root
// entity, so a list-valued services: must not route there.
func TestClassify_ServicesList_NotCompose(t *testing.T) {
	for name, content := range map[string]string{
		"ci-config.yml":   "services:\n  - docker:dind\nstages:\n  - build\n",
		"ci-zero.yml":     "services:\n- docker:dind\n",
		"ci-comment.yaml": "services: # images\n\n  # the daemon\n  - name: docker:dind\n",
	} {
		if got := Classify(name, []byte(content), nil); got == ToolCompose {
			t.Errorf("Classify(%s) = %q: a services: list is not a Compose file", name, got)
		}
	}
	// A mapping stays Compose when blank and comment lines precede its
	// first service, and a later document's mapping counts.
	for name, content := range map[string]string{
		"spaced.yml":   "services:\n\n  # the web tier\n  web:\n    image: nginx\n",
		"multidoc.yml": "services:\n  - docker:dind\n---\nservices:\n  web:\n    image: nginx\n",
	} {
		if got := Classify(name, []byte(content), nil); got != ToolCompose {
			t.Errorf("Classify(%s) = %q, want %q", name, got, ToolCompose)
		}
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
	idx := &SiblingIndex{chartDirs: map[string]struct{}{"mychart": {}}}
	if got := Classify("mychart/templates/configmap.yaml", content, idx); got != ToolHelmTemplateYAML {
		t.Errorf("Classify = %q, want %q (chart ancestor present)", got, ToolHelmTemplateYAML)
	}
	// Same templates/ path shape, but no chart ancestor recorded — should
	// fall through to content classification instead.
	if got := Classify("orphan/templates/configmap.yaml", content, &SiblingIndex{}); got != ToolK8sManifest {
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

// ─── SiblingIndex ───────────────────────────────────────────────────────────

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
	idx := BuildSiblingIndex(refs)

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
	var idx *SiblingIndex
	if idx.hasChartAncestor("anything") {
		t.Error("nil SiblingIndex must report no chart ancestor")
	}
	if idx.hasCueModAncestor("anything") {
		t.Error("nil SiblingIndex must report no cue.mod ancestor")
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
