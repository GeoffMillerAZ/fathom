package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestCircleCIClassification (LB-Q-circleci): the manifest's globs claim a
// config.yml under a .circleci directory at any depth and nothing beside it.
func TestCircleCIClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
	}{
		{".circleci/config.yml", "circleci"},
		{".circleci/config.yaml", "circleci"},
		{"svc/api/.circleci/config.yml", "circleci"},
		{"config.yml", "kubernetes"},
		{".circleci/continue.yml", "kubernetes"},
		{"circleci/config.yml", "kubernetes"},
		{".github/workflows/ci.yml", "github-actions"},
	}
	for _, c := range cases {
		if got := connector.ClassifyDialect(rules, c.path, nil).Dialect; got != c.want {
			t.Errorf("ClassifyDialect(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestClassify_CircleCI: a config reaches the dialect by its path, ahead of
// the Compose signal (a legacy config can carry a top-level services key in
// a job's docker list only, never at the top, but the path decides), and a
// file beside it stays what its content says.
func TestClassify_CircleCI(t *testing.T) {
	cfg := []byte("version: 2.1\njobs:\n  build:\n    docker:\n      - image: cimg/base:stable\n    steps: [checkout]\n")
	services := []byte("services:\n  db:\n    image: postgres:16\n")
	cases := []struct {
		path    string
		content []byte
		want    ToolKind
	}{
		{".circleci/config.yml", cfg, ToolCircleCI},
		{"svc/.circleci/config.yaml", cfg, ToolCircleCI},
		{".circleci/config.yml", services, ToolCircleCI},
		{"docs/config.yml", cfg, ToolUnclassified},
		{"docs/stack.yml", services, ToolCompose},
	}
	for _, c := range cases {
		if got := Classify(c.path, c.content, nil); got != c.want {
			t.Errorf("Classify(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func circleCICorpus() map[string]string {
	return map[string]string{
		".circleci/config.yml": `version: 2.1
commands:
  setup:
    steps: [checkout]
jobs:
  build:
    docker:
      - image: cimg/base:stable
    steps: [setup]
  test:
    docker:
      - image: cimg/base:stable
    steps: [setup]
workflows:
  main:
    jobs:
      - build
      - test:
          requires: [build]
`,
		".circleci/continue.yml": "jobs:\n  other:\n    docker: [{image: x}]\n",
	}
}

// TestIngest_CircleCIConfig: an ingest extracts the config's jobs, workflow
// and command through the circleci dialect, tags them "circleci", and leaves
// the neighbouring file in .circleci/ to the generic YAML reader. The edges
// ride on Calls.
func TestIngest_CircleCIConfig(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, circleCICorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cc", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		if o.SourceRef == ".circleci/config.yml" && o.ToolKind != string(ToolCircleCI) {
			t.Errorf("%s|%s: ToolKind %q, want %q", o.SourceRef, o.Symbol, o.ToolKind, ToolCircleCI)
		}
	}
	want := map[string]string{
		".circleci/config.yml|setup": "CircleCICommand",
		".circleci/config.yml|build": "CircleCIJob",
		".circleci/config.yml|test":  "CircleCIJob",
		".circleci/config.yml|main":  "CircleCIWorkflow",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observations\n got: %v\nwant: %v", got, want)
	}
}

// TestReingestFile_CircleCIConfig: the one-file refresh reads .circleci/config.yml
// through the same dialect an ingest does.
func TestReingestFile_CircleCIConfig(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, circleCICorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cc", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := ".circleci/config.yml"
	edited := circleCICorpus()[rel] + "  nightly:\n    jobs: [build]\n"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "cc", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cc", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		if o.SourceRef == rel {
			got[o.Symbol] = string(o.SourceKind)
		}
	}
	want := map[string]string{
		"setup": "CircleCICommand", "build": "CircleCIJob", "test": "CircleCIJob",
		"main": "CircleCIWorkflow", "nightly": "CircleCIWorkflow",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("observations after ReingestFile\n got: %v\nwant: %v", got, want)
	}
}
