package fusion

import (
	"context"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAzurePipelinesDispatch(t *testing.T) {
	p := New(nil, nil, Config{})
	t.Cleanup(p.Close)
	for _, rel := range []string{"azure-pipelines.yml", "templates/build.yml"} {
		e, k, ok, _ := p.resolveExtractor(rel, []byte("jobs:\n- job: build\n  steps: []\n"), nil)
		if !ok || k != ToolAzurePipelines {
			t.Fatalf("%s: %v %s", rel, ok, k)
		}
		r, err := e.(extractor.DocResulter).ExtractDoc("svc", rel, []byte("jobs:\n- job: build\n  steps: []\n"))
		if err != nil || len(r.Nodes) != 2 {
			t.Fatalf("%s: %v %+v", rel, err, r.Nodes)
		}
	}
	for _, rel := range []string{".github/workflows/ci.yml", "manifest.yaml"} {
		src := []byte("apiVersion: batch/v1\nkind: Job\nmetadata: {name: x}\nspec: {}\n")
		if _, _, ok := azurePipelineDispatch(rel, src, nil); ok {
			t.Fatalf("stole %s", rel)
		}
	}
}

func TestAzurePipelinesIngestAndReingest(t *testing.T) {
	dir := t.TempDir()
	rel := "azure-pipelines.yml"
	src := []byte("jobs:\n- job: build\n  steps: []\n")
	if err := os.WriteFile(filepath.Join(dir, rel), src, 0600); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	src = append(src, []byte("- job: test\n  dependsOn: build\n  steps: []\n")...)
	if err := os.WriteFile(filepath.Join(dir, rel), src, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "svc", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "svc", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range obs {
		if o.SourceRef == rel {
			got[o.Symbol] = true
		}
	}
	if !got["build"] || !got["test"] || !got[rel] {
		t.Fatalf("stored symbols %v", got)
	}
}

func TestAzurePipelinesContentDiscoveryAndEarlierClaims(t *testing.T) {
	p := New(nil, nil, Config{})
	t.Cleanup(p.Close)
	src := []byte("jobs:\n- job: build\n")
	idx := buildSiblingIndex([]core.SourceRef{{Path: "Chart.yaml"}})
	if _, _, ok := azurePipelineDispatch("templates/build.yml", src, idx); ok {
		t.Fatal("stole actual chart template")
	}
	if got := p.ContentDialect("templates/build.yml", src, nil); got != "azure-pipelines" {
		t.Fatalf("content discovery %q", got)
	}
	for _, rel := range []string{"Taskfile.yml", ".circleci/config.yml", ".gitlab-ci.yml", "Chart.yaml", ".github/workflows/ci.yml"} {
		if _, _, ok := azurePipelineDispatch(rel, src, nil); ok {
			t.Fatalf("stole earlier claim %s", rel)
		}
	}
	if _, _, ok := azurePipelineDispatch("notes.yml", []byte("script: |\n  - job: text only\n"), nil); ok {
		t.Fatal("claimed text in a string")
	}
}

// Existing content claims must win even when examples contain Azure-shaped jobs.
// File refresh is also compared where main already supports that content claim.
func TestAzurePipelinesPreservesYAMLClaims(t *testing.T) {
	cases := []struct{ name, path, source string }{
		{"openapi", "api/openapi.yaml", "openapi: 3.0.0\ninfo: {title: schedules, version: '1'}\npaths: {}\nx-example:\n  jobs:\n  - job: nightly\n  - job: weekly\n"},
		{"cloudformation", "cfn/template.yaml", "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  Queue:\n    Type: AWS::SQS::Queue\n    Metadata:\n      jobs:\n      - job: nightly\n      - job: weekly\n"},
		{"ansible", "playbooks/build.yml", "- name: build\n  hosts: all\n  tasks:\n  - name: schedule\n    debug: {msg: ok}\n    vars:\n      jobs:\n      - job: nightly\n      - job: weekly\n"},
		{"gitlab", ".gitlab/ci/build.yml", "build:\n  script: [make]\n  variables:\n    jobs:\n    - job: nightly\n    - job: weekly\n"},
		{"serverless", "serverless.yml", "service: schedules\nprovider: {name: aws}\nfunctions:\n  scheduled:\n    handler: handler.run\ncustom:\n  jobs:\n  - job: nightly\n  - job: weekly\n"},
		{"compose", "stack/example.yml", "services:\n  web:\n    image: example\nx-example:\n  jobs:\n  - job: nightly\n  - job: weekly\n"},
		{"nested-only", "notes.yml", "example:\n  jobs:\n  - job: nightly\n  - job: weekly\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, tc.path)
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			st := store.NewMemoryStore()
			p := New(connector.NewLocalConnector(), st, Config{})
			t.Cleanup(p.Close)
			if _, _, ok := azurePipelineDispatch(tc.path, []byte(tc.source), nil); ok {
				t.Fatal("Azure stole an earlier claim or a nested example")
			}
			if tc.name == "nested-only" {
				return
			}
			ctx := context.Background()
			if _, err := p.Ingest(ctx, core.ServiceRef{ID: "svc", Path: dir}); err != nil {
				t.Fatal(err)
			}
			before, err := st.QueryObservations(ctx, "svc", "")
			if err != nil || len(before) == 0 {
				t.Fatalf("ingest: %v, observations %d", err, len(before))
			}
			// Main has no content-Compose refresh branch (only named Compose files).
			// Its existing refresh gap is outside this Azure regression fix.
			if tc.name == "compose" {
				return
			}
			if _, err := p.ReingestFile(ctx, st, "svc", dir, tc.path); err != nil {
				t.Fatal(err)
			}
			after, err := st.QueryObservations(ctx, "svc", "")
			if err != nil {
				t.Fatal(err)
			}
			keys := func(obs []core.Observation) map[string]int {
				m := map[string]int{}
				for _, o := range obs {
					if o.SourceRef == tc.path {
						m[string(o.SourceKind)+"\x00"+o.Symbol]++
					}
				}
				return m
			}
			if !reflect.DeepEqual(keys(before), keys(after)) {
				t.Fatalf("ingest/refresh differ: %v / %v", keys(before), keys(after))
			}
		})
	}
}

func TestAzurePipelinesExtendsParameterStages(t *testing.T) {
	p := New(nil, nil, Config{})
	t.Cleanup(p.Close)
	source := []byte("extends:\n  template: pipeline.yml\n  parameters:\n    stages:\n    - stage: Build\n      jobs: []\n")
	_, kind, ok, _ := p.resolveExtractor("templates/derived.yml", source, nil)
	if !ok || kind != ToolAzurePipelines {
		t.Fatalf("extends pipeline: %v %s", ok, kind)
	}
}
