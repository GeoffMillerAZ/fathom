package fusion_test

// toolident_test.go — full-pipeline coverage for the ADR-021 §2 tool-identity
// classifier (docs/plans/toolident.md): a Helm chart golden fixture (the
// whole-chart blind spot) plus misroute regression fixtures (a compose
// override file, a non-k8s YAML file, an OpenAPI doc) proving each file type
// is recognized for what it is instead of silently producing zero symbols.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// writeFile is a small helper: create relPath (with parent dirs) under dir
// containing content.
func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", full, err)
	}
}

// ingestDir ingests dir as a fresh service and returns every stored
// observation.
func ingestDir(t *testing.T, dir, serviceID string) ([]core.Observation, fusion.Report) {
	t.Helper()
	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: serviceID, Path: dir}
	p := fusion.New(conn, st, fusion.Config{Concurrency: 1})

	ctx := context.Background()
	report, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	return obs, report
}

func obsByKind(obs []core.Observation, kind core.SourceKind) []core.Observation {
	var out []core.Observation
	for _, o := range obs {
		if o.SourceKind == kind {
			out = append(out, o)
		}
	}
	return out
}

func obsBySource(obs []core.Observation, source string) []core.Observation {
	var out []core.Observation
	for _, o := range obs {
		if o.SourceRef == source {
			out = append(out, o)
		}
	}
	return out
}

// ─── TI-2: Helm chart golden fixture (the whole-chart blind spot) ─────────

func TestIngest_HelmChartGoldenFixture(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "mychart/Chart.yaml", "name: mychart\nversion: 0.1.0\napiVersion: v2\n")
	writeFile(t, dir, "mychart/values.yaml", "image:\n  repository: myapp\n  tag: latest\n")
	writeFile(t, dir, "mychart/templates/deployment.yaml", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "mychart.fullname" . }}
spec:
  template:
    spec:
      containers:
        - name: app
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
`)
	writeFile(t, dir, "mychart/templates/_helpers.tpl", `{{- define "mychart.fullname" -}}
{{- .Release.Name -}}
{{- end }}
`)

	obs, report := ingestDir(t, dir, "helm-golden-svc")

	if len(obsByKind(obs, core.KindHelmChart)) != 1 {
		t.Errorf("expected 1 HelmChart symbol from Chart.yaml, got %d", len(obsByKind(obs, core.KindHelmChart)))
	}
	if len(obsByKind(obs, core.KindHelmValue)) == 0 {
		t.Error("expected HelmValue symbols from values.yaml")
	}
	if len(obsByKind(obs, core.KindHelmTemplate)) == 0 {
		t.Error("expected HelmTemplate symbols from the .tpl partial")
	}

	// The whole-chart blind spot: templates/deployment.yaml used to decode
	// via yaml.v3 directly (fails on {{ }}) and silently emit nothing.
	deploymentObs := obsBySource(obs, "mychart/templates/deployment.yaml")
	if len(deploymentObs) == 0 {
		t.Fatal("templates/deployment.yaml produced zero observations — the whole-chart blind spot is not fixed")
	}
	var sawWorkload bool
	for _, o := range deploymentObs {
		if o.SourceKind == core.KindK8sWorkload {
			sawWorkload = true
			if o.ToolKind != string(fusion.ToolHelmTemplateYAML) {
				t.Errorf("deployment.yaml workload ToolKind = %q, want %q", o.ToolKind, fusion.ToolHelmTemplateYAML)
			}
			if !strings.Contains(o.Claim, "templated:true") {
				t.Errorf("templated metadata.name should mark Signature/Claim templated:true, got %q", o.Claim)
			}
		}
	}
	if !sawWorkload {
		t.Error("expected a K8sWorkload observation from templates/deployment.yaml")
	}

	var sawValueRef bool
	for _, o := range deploymentObs {
		if o.SourceKind == core.KindHelmValueRef {
			sawValueRef = true
		}
	}
	if !sawValueRef {
		t.Error("expected KindHelmValueRef observations (.Values.image.repository / .tag) from the template")
	}

	if report.Unclassified != 0 {
		t.Errorf("a well-formed chart should classify every file: Unclassified = %d, want 0", report.Unclassified)
	}
}

// ─── TI-3: misroute regressions ────────────────────────────────────────────

func TestIngest_ComposeOverrideFile_AnyFilenameClassifiedByContent(t *testing.T) {
	dir := t.TempDir()
	// Not one of the canonical compose basenames — the grounded defect: only
	// docker-compose.yml/.yaml/compose.yml/.yaml were special-cased, so this
	// file fell through to the generic k8s extractor (zero symbols, no
	// apiVersion/kind).
	writeFile(t, dir, "docker-compose.override.yml", `services:
  web:
    image: myapp/web:1.0
    depends_on:
      - db
  db:
    image: postgres:15
`)

	obs, report := ingestDir(t, dir, "compose-override-svc")
	services := obsByKind(obs, core.KindComposeService)
	if len(services) != 2 {
		t.Fatalf("expected 2 ComposeService symbols from docker-compose.override.yml, got %d: %+v", len(services), services)
	}
	if report.Unclassified != 0 {
		t.Errorf("Unclassified = %d, want 0 (content-classified as compose)", report.Unclassified)
	}
}

func TestIngest_NonK8sYAML_UnclassifiedNotSilentlyRoutedToK8s(t *testing.T) {
	dir := t.TempDir()
	// Plain config YAML: no apiVersion/kind, no services:, no openapi:. The
	// grounded defect: this used to be handed to the k8s extractor and
	// silently produce zero symbols with no visibility at all.
	writeFile(t, dir, "config/app.yaml", `logging:
  level: debug
retries: 3
`)

	obs, report := ingestDir(t, dir, "nonk8s-yaml-svc")
	if len(obsBySource(obs, "config/app.yaml")) != 0 {
		t.Errorf("expected zero observations for non-k8s-shaped YAML, got %d", len(obsBySource(obs, "config/app.yaml")))
	}
	if report.Unclassified != 1 {
		t.Errorf("report.Unclassified = %d, want 1 — the miss must be counted, not silent", report.Unclassified)
	}
}

func TestIngest_OpenAPIDoc_Classified(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "api/openapi.yaml", `openapi: 3.0.0
paths:
  /widgets:
    get:
      operationId: listWidgets
components:
  schemas:
    Widget:
      type: object
`)

	obs, report := ingestDir(t, dir, "openapi-svc")
	if len(obsByKind(obs, core.KindOpenAPIOperation)) != 1 {
		t.Errorf("expected 1 OpenAPIOperation symbol, got %d", len(obsByKind(obs, core.KindOpenAPIOperation)))
	}
	if len(obsByKind(obs, core.KindOpenAPISchema)) != 1 {
		t.Errorf("expected 1 OpenAPISchema symbol, got %d", len(obsByKind(obs, core.KindOpenAPISchema)))
	}
	if report.Unclassified != 0 {
		t.Errorf("Unclassified = %d, want 0", report.Unclassified)
	}
}

func TestIngest_CRD_TaggedDistinctlyFromGenericK8sManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "crds/widget.yaml", `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
`)
	obs, _ := ingestDir(t, dir, "crd-svc")
	workloads := obsByKind(obs, core.KindK8sWorkload)
	if len(workloads) != 1 {
		t.Fatalf("expected 1 K8sWorkload symbol from the CRD, got %d", len(workloads))
	}
	if workloads[0].ToolKind != string(fusion.ToolK8sCRD) {
		t.Errorf("ToolKind = %q, want %q", workloads[0].ToolKind, fusion.ToolK8sCRD)
	}
}

// ─── TI-4: CUE module ancestor tagging ─────────────────────────────────────

func TestIngest_CUEModuleAncestorTagged(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "cue.mod/module.cue", `module: "example.com/schema"
`)
	// A CUE module's root scopes its ENTIRE subtree (real CUE semantics): a
	// nested file is part of the module too, not just files beside cue.mod/.
	writeFile(t, dir, "nested/schema.cue", "package svc\n\n#Config: {\n\tname: string\n}\n")

	obs, _ := ingestDir(t, dir, "cue-module-svc")

	schemaObs := obsBySource(obs, "nested/schema.cue")
	if len(schemaObs) == 0 {
		t.Fatal("expected observations from nested/schema.cue")
	}
	for _, o := range schemaObs {
		if o.ToolKind != string(fusion.ToolCUEModule) {
			t.Errorf("nested/schema.cue (under cue.mod ancestor) ToolKind = %q, want %q", o.ToolKind, fusion.ToolCUEModule)
		}
	}
}

func TestIngest_CUEPlain_NoModuleAncestor(t *testing.T) {
	// A separate corpus with no cue.mod/ anywhere — the plain-CUE case.
	dir := t.TempDir()
	writeFile(t, dir, "standalone/other.cue", "package other\n\n#Other: {\n\tx: int\n}\n")

	obs, _ := ingestDir(t, dir, "cue-plain-svc")
	otherObs := obsBySource(obs, "standalone/other.cue")
	if len(otherObs) == 0 {
		t.Fatal("expected observations from standalone/other.cue")
	}
	for _, o := range otherObs {
		if o.ToolKind != string(fusion.ToolCUEPlain) {
			t.Errorf("standalone/other.cue (no cue.mod anywhere) ToolKind = %q, want %q", o.ToolKind, fusion.ToolCUEPlain)
		}
	}
}

// ─── TI-6: size cap + unclassified counters ────────────────────────────────

func TestIngest_OversizeFileSkippedAndCounted(t *testing.T) {
	dir := t.TempDir()
	huge := strings.Repeat("a", fusion.MaxExtractSize+1)
	writeFile(t, dir, "huge.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\ndata:\n  blob: \""+huge+"\"\n")
	writeFile(t, dir, "small.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: y\n")

	obs, report := ingestDir(t, dir, "oversize-svc")
	if report.Oversize != 1 {
		t.Errorf("report.Oversize = %d, want 1", report.Oversize)
	}
	if len(obsBySource(obs, "huge.yaml")) != 0 {
		t.Errorf("oversize file should produce zero observations, got %d", len(obsBySource(obs, "huge.yaml")))
	}
	if len(obsBySource(obs, "small.yaml")) == 0 {
		t.Error("the small sibling file should still be extracted normally")
	}
}

// ─── TI-7: cross-refs, cheap tier ───────────────────────────────────────────

func TestIngest_CrossRefImageToDockerfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "k8s/deployment.yaml", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
spec:
  template:
    spec:
      containers:
        - name: app
          image: myapp:1.2.3
`)
	writeFile(t, dir, "myapp/Dockerfile", "FROM golang:1.26 AS build\n")

	obs, _ := ingestDir(t, dir, "crossref-svc")
	refs := obsByKind(obs, core.KindImageDockerfileRef)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ImageDockerfileRef, got %d: %+v", len(refs), refs)
	}
	if refs[0].Symbol != "myapp" {
		t.Errorf("cross-ref Symbol = %q, want %q", refs[0].Symbol, "myapp")
	}
	if !strings.Contains(refs[0].Claim, "myapp/Dockerfile") {
		t.Errorf("cross-ref Claim = %q, want it to reference myapp/Dockerfile", refs[0].Claim)
	}
	if refs[0].EvidenceTier != core.TierPreScan {
		t.Errorf("cross-ref Tier = %q, want TierPreScan (same-name match, not a resolved build dependency)", refs[0].EvidenceTier)
	}
}

// ─── TI-5: generated-file marker ────────────────────────────────────────────

func TestIngest_GeneratedFileMarker(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "generated.go", "// Code generated by protoc-gen-go. DO NOT EDIT.\npackage foo\n\nfunc GeneratedFunc() {}\n")
	writeFile(t, dir, "handwritten.go", "package foo\n\nfunc HandwrittenFunc() {}\n")

	obs, _ := ingestDir(t, dir, "generated-svc")

	var sawGenerated, sawHandwritten bool
	for _, o := range obs {
		switch o.Symbol {
		case "GeneratedFunc":
			sawGenerated = true
			if !o.Generated {
				t.Error("GeneratedFunc observation should have Generated=true")
			}
		case "HandwrittenFunc":
			sawHandwritten = true
			if o.Generated {
				t.Error("HandwrittenFunc observation should have Generated=false")
			}
		}
	}
	if !sawGenerated {
		t.Fatal("expected an observation for GeneratedFunc")
	}
	if !sawHandwritten {
		t.Fatal("expected an observation for HandwrittenFunc")
	}
}

// ─── Kustomize end-to-end (complements the epoch test's regression proof) ──

func TestIngest_KustomizationEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
images:
  - name: myapp
    newTag: "2.0.0"
`)

	obs, report := ingestDir(t, dir, "kustomize-e2e-svc")
	if len(obsByKind(obs, core.KindKustomizeResource)) != 1 {
		t.Errorf("expected 1 KustomizeResource symbol, got %d", len(obsByKind(obs, core.KindKustomizeResource)))
	}
	if len(obsByKind(obs, core.KindKustomizeImage)) != 1 {
		t.Errorf("expected 1 KustomizeImage symbol, got %d", len(obsByKind(obs, core.KindKustomizeImage)))
	}
	if report.Unclassified != 0 {
		t.Errorf("Unclassified = %d, want 0", report.Unclassified)
	}
}

// ─── Stats() cumulative counters ────────────────────────────────────────────

func TestPipeline_StatsAccumulatesAcrossIngests(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config/app.yaml", "logging:\n  level: debug\n")

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "stats-svc", Path: dir}
	p := fusion.New(conn, st, fusion.Config{Concurrency: 1})
	ctx := context.Background()

	before, _ := p.Stats()
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	after, _ := p.Stats()
	if after-before != 1 {
		t.Errorf("Stats() unclassified delta = %d, want 1", after-before)
	}
}
