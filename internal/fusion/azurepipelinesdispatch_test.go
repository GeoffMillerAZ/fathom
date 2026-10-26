package fusion

import (
	"context"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
	"os"
	"path/filepath"
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
