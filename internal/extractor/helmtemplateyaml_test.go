package extractor_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// deploymentTemplateFixture is a realistic minimal Helm template: a
// templated metadata.name (via `include`), a partially-templated image
// (literal repo, templated tag), and several `.Values.x.y` references —
// exactly the whole-chart blind spot ADR-021 §2 fixes (yaml.v3 fails
// outright on the raw {{ }} syntax below).
const deploymentTemplateFixture = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "mychart.fullname" . }}
  labels:
    app: {{ .Chart.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
  template:
    spec:
      containers:
        - name: app
          image: "myapp:{{ .Values.image.tag }}"
`

func TestHelmTemplateYAMLExtractor_Workload(t *testing.T) {
	x := extractor.NewHelmTemplateYAMLExtractor()
	nodes, err := x.Extract("svc", "mychart/templates/deployment.yaml", []byte(deploymentTemplateFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	var workloads []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindK8sWorkload {
			workloads = append(workloads, n)
		}
	}
	if len(workloads) != 1 {
		t.Fatalf("expected 1 workload node, got %d: %v", len(workloads), helmTplYAMLNodeNames(nodes))
	}
	w := workloads[0]
	// metadata.name is fully templated (`{{ include ... }}`) → falls back to
	// the file stem, flagged templated:true.
	if w.Name != "Deployment/deployment" {
		t.Errorf("Name: got %q, want %q (stem fallback)", w.Name, "Deployment/deployment")
	}
	if w.Signature != "apps/v1 templated:true" {
		t.Errorf("Signature: got %q, want %q", w.Signature, "apps/v1 templated:true")
	}
	if w.Tier != core.TierVerified {
		t.Errorf("Tier: got %q, want TierVerified", w.Tier)
	}
}

func TestHelmTemplateYAMLExtractor_PartiallyTemplatedImageKept(t *testing.T) {
	x := extractor.NewHelmTemplateYAMLExtractor()
	nodes, err := x.Extract("svc", "mychart/templates/deployment.yaml", []byte(deploymentTemplateFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var containers []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindK8sContainer {
			containers = append(containers, n)
		}
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container node (partially-templated image kept), got %d: %v", len(containers), helmTplYAMLNodeNames(nodes))
	}
	if containers[0].Name != "myapp:__FATHOM_HELM_TPL__" {
		t.Errorf("container image Name: got %q, want the literal repo name preserved with a placeholder tag", containers[0].Name)
	}
}

func TestHelmTemplateYAMLExtractor_FullyTemplatedImageDropped(t *testing.T) {
	src := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: full-image-test
spec:
  template:
    spec:
      containers:
        - name: app
          image: {{ .Values.image }}
`
	x := extractor.NewHelmTemplateYAMLExtractor()
	nodes, err := x.Extract("svc", "mychart/templates/deployment.yaml", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Kind == core.KindK8sContainer {
			t.Errorf("a fully-templated image should be dropped, got container node: %+v", n)
		}
	}
}

func TestHelmTemplateYAMLExtractor_TwoActionImageJoinedByPunctuationDropped(t *testing.T) {
	// Found via the before/after demo corpus: repo and tag as TWO SEPARATE
	// {{ }} actions joined only by ":" substitute to
	// "PLACEHOLDER:PLACEHOLDER" — not equal to the placeholder alone, but
	// still carrying zero literal content. isFullyTemplated must catch this
	// case too, not just the single-action one.
	src := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: two-action-image-test
spec:
  template:
    spec:
      containers:
        - name: app
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
`
	x := extractor.NewHelmTemplateYAMLExtractor()
	nodes, err := x.Extract("svc", "mychart/templates/deployment.yaml", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Kind == core.KindK8sContainer {
			t.Errorf("a two-action image joined only by punctuation should be dropped (zero literal content), got container node: %+v", n)
		}
	}
}

func TestHelmTemplateYAMLExtractor_ValuesReferences(t *testing.T) {
	x := extractor.NewHelmTemplateYAMLExtractor()
	nodes, err := x.Extract("svc", "mychart/templates/deployment.yaml", []byte(deploymentTemplateFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode)
	for _, n := range nodes {
		if n.Kind == core.KindHelmValueRef {
			byName[n.Name] = n
		}
	}
	for _, want := range []string{"replicaCount", "image.tag"} {
		n, ok := byName[want]
		if !ok {
			t.Errorf("expected values ref %q; got %v", want, helmTplYAMLNodeNames(nodes))
			continue
		}
		if n.Tier != core.TierPreScan {
			t.Errorf("values ref %q Tier: got %q, want TierPreScan", want, n.Tier)
		}
	}
}

func TestHelmTemplateYAMLExtractor_NonTemplatedNameKept(t *testing.T) {
	src := `apiVersion: v1
kind: ConfigMap
metadata:
  name: static-config
data:
  foo: bar
`
	x := extractor.NewHelmTemplateYAMLExtractor()
	nodes, err := x.Extract("svc", "mychart/templates/configmap.yaml", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var workloads []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindK8sWorkload {
			workloads = append(workloads, n)
		}
	}
	if len(workloads) != 1 {
		t.Fatalf("expected 1 workload node, got %d", len(workloads))
	}
	if workloads[0].Name != "ConfigMap/static-config" {
		t.Errorf("Name: got %q, want the literal name (not templated, no fallback)", workloads[0].Name)
	}
	if workloads[0].Signature != "v1" {
		t.Errorf("Signature: got %q, want plain apiVersion (no templated:true marker)", workloads[0].Signature)
	}
}

// TestHelmTemplateYAMLExtractor_GuardWrappedTemplate is critic F2: `helm
// create` wraps half its templates (ingress.yaml, serviceaccount.yaml,
// hpa.yaml) in a whole-file guard — {{- if .Values.x.enabled -}} on the
// first line, {{- end }} on the last, plus whole-line control-flow inside.
// Placeholder-substituting those lines produced a bare scalar above the
// document mapping, which breaks the YAML decode → zero workload symbols
// for half of every standard chart. Whole-line actions must be DROPPED
// (replaced with nothing), not placeholder-substituted.
func TestHelmTemplateYAMLExtractor_GuardWrappedTemplate(t *testing.T) {
	// The `helm create` ingress.yaml shape, abbreviated but structurally
	// faithful: guard first line, range/if/end control-flow lines, inline
	// value actions on kept lines, {{- end }} last line.
	src := `{{- if .Values.ingress.enabled -}}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "mychart.fullname" . }}
  labels:
    app: {{ .Chart.Name }}
spec:
  {{- if .Values.ingress.className }}
  ingressClassName: {{ .Values.ingress.className }}
  {{- end }}
  rules:
    {{- range .Values.ingress.hosts }}
    - host: {{ .host | quote }}
    {{- end }}
{{- end }}
`
	x := extractor.NewHelmTemplateYAMLExtractor()
	nodes, err := x.Extract("svc", "mychart/templates/ingress.yaml", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	var workloads []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindK8sWorkload {
			workloads = append(workloads, n)
		}
	}
	if len(workloads) != 1 {
		t.Fatalf("guard-wrapped template produced %d workload nodes, want 1 (Ingress): %v", len(workloads), helmTplYAMLNodeNames(nodes))
	}
	w := workloads[0]
	// metadata.name is templated → stem fallback + templated:true.
	if w.Name != "Ingress/ingress" {
		t.Errorf("Name: got %q, want %q", w.Name, "Ingress/ingress")
	}
	if !strings.Contains(w.Signature, "templated:true") {
		t.Errorf("Signature: got %q, want a templated:true marker", w.Signature)
	}

	// The .Values refs inside the guard lines are still captured.
	var sawEnabled bool
	for _, n := range nodes {
		if n.Kind == core.KindHelmValueRef && n.Name == "ingress.enabled" {
			sawEnabled = true
		}
	}
	if !sawEnabled {
		t.Error("expected values ref ingress.enabled from the guard line")
	}
}

func TestHelmTemplateYAMLExtractor_Determinism(t *testing.T) {
	x := extractor.NewHelmTemplateYAMLExtractor()
	a, _ := x.Extract("svc", "mychart/templates/deployment.yaml", []byte(deploymentTemplateFixture))
	b, _ := x.Extract("svc", "mychart/templates/deployment.yaml", []byte(deploymentTemplateFixture))
	if !reflect.DeepEqual(a, b) {
		t.Error("Extract is not deterministic")
	}
}

func helmTplYAMLNodeNames(nodes []core.SymbolNode) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = string(n.Kind) + ":" + n.Name
	}
	return names
}
