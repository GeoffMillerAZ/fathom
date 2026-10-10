package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func ghaCorpus() map[string]string {
	return map[string]string{
		".github/workflows/ci.yml":          "on: push\njobs:\n  build:\n    runs-on: x\n    steps:\n      - id: co\n        uses: actions/checkout@v4\n      - uses: ./.github/actions/setup\n",
		".github/actions/setup/action.yml":  "name: Setup\ndescription: d\nruns:\n  using: composite\n  steps:\n    - id: s1\n      run: echo\n      shell: bash\n",
		".github/actions/other/action.yaml": "name: Other\ndescription: d\nruns:\n  using: node20\n  main: index.js\n",
		"deploy/action.yml":                 "name: Not a GHA thing\nruns:\n  using: composite\n  steps: []\n",
		"k8s/deploy.yaml":                   "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n",
	}
}

// TestGitHubActionsIsTheDialect (LB-F306): a workflow and an action.yml or
// action.yaml (by basename, anywhere) read through the github-actions dialect;
// ingest and ReingestFile agree.
func TestGitHubActionsIsTheDialect(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	want := p.toolExts[ToolGHA]
	if want == nil {
		t.Fatal("no github-actions dialect registered")
	}
	for rel, src := range ghaCorpus() {
		if rel == "k8s/deploy.yaml" {
			continue
		}
		e, tool, ok, _ := p.resolveExtractor(rel, []byte(src), nil)
		if !ok || e != want || tool != ToolGHA {
			t.Errorf("%s: dispatched to %T (%q, ok=%v), want the github-actions dialect", rel, e, tool, ok)
		}
	}
}

func TestIngest_GitHubActionsDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, ghaCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "gha", Path: dir}); err != nil {
		t.Fatal(err)
	}
	got := crSourceKinds(t, st, "gha")
	for rel, kind := range map[string]string{
		".github/workflows/ci.yml":          "GHAWorkflow",
		".github/actions/setup/action.yml":  "GHAActionDef",
		".github/actions/other/action.yaml": "GHAActionDef",
	} {
		if !crHasKind(got[rel], kind) {
			t.Errorf("%s read as %q, want %s", rel, got[rel], kind)
		}
	}
	if !crHasKind(got[".github/workflows/ci.yml"], "GHAStep") {
		t.Errorf("workflow steps with an id are not entities: %q", got[".github/workflows/ci.yml"])
	}
	if !crHasKind(got[".github/workflows/ci.yml"], "GHAPinRef") {
		t.Errorf("a remote action is no pin node: %q", got[".github/workflows/ci.yml"])
	}
	if crHasKind(got["k8s/deploy.yaml"], "GHAWorkflow") || crHasKind(got["k8s/deploy.yaml"], "GHAActionDef") {
		t.Errorf("a Kubernetes manifest read as GitHub Actions: %q", got["k8s/deploy.yaml"])
	}
}

func TestReingestFile_GitHubActionsParity(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, ghaCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "gha", Path: dir}); err != nil {
		t.Fatal(err)
	}
	before := crSourceKinds(t, st, "gha")
	for rel := range ghaCorpus() {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, []byte("# edited\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "gha", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	after := crSourceKinds(t, st, "gha")
	if !reflect.DeepEqual(before, after) {
		t.Errorf("ReingestFile changed what a file reads as\nbefore: %v\n after: %v", before, after)
	}
}

// TestIngest_GitHubActionsRunSteps (LB-F310, LB-F339): a run step's first
// command reaches a Makefile target, a Taskfile task, a package.json script and
// a script file's function, as typed edges from the step entity: the
// positional GHARunStep of an id-less step, the GHAStep of one with an id.
func TestIngest_GitHubActionsRunSteps(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		".github/workflows/ci.yml": "on: push\njobs:\n  b:\n    runs-on: x\n    steps:\n      - uses: actions/checkout@v4\n      - run: make test\n      - run: |\n          task lint\n          npm run build\n          bash scripts/ci.sh\n      - run: make ghost\n      - id: package\n        run: make dist\n",
		"Makefile":                 "test:\n\t@echo t\ndist:\n\t@echo d\n",
		"Taskfile.yml":             "version: '3'\ntasks:\n  lint:\n    cmds:\n      - echo\n",
		"package.json":             "{\"name\": \"app\", \"scripts\": {\"build\": \"tsc\"}}\n",
		"scripts/ci.sh":            "#!/bin/bash\nsetup() { echo a; }\nmain() { setup; }\nmain\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "run", Path: dir}); err != nil {
		t.Fatal(err)
	}
	edges := crEdges(t, st, "run")
	for _, want := range []string{
		".github/workflows/ci.yml -> run::Makefile.test",
		".github/workflows/ci.yml -> run::Taskfile.lint",
		".github/workflows/ci.yml -> run::package.build",
		".github/workflows/ci.yml -> run::scripts/ci.main",
	} {
		found := false
		for _, e := range edges {
			if strings.HasSuffix(e, strings.TrimPrefix(want, ".github/workflows/ci.yml")) && strings.HasPrefix(e, ".github/workflows/ci.yml") {
				found = true
			}
		}
		if !found {
			t.Errorf("no edge like %q; edges: %q", want, edges)
		}
	}
	for _, e := range edges {
		if strings.Contains(e, "ghost") {
			t.Errorf("an edge to a target nothing defines: %q", e)
		}
	}
	// A run step with an id is the GHAStep its id names (O631, K20): its
	// first command's edge starts there, not at a positional entity.
	all, err := st.TypedEdges(context.Background(), "run")
	if err != nil {
		t.Fatal(err)
	}
	var fromDist []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved && strings.HasSuffix(e.ToSymbol, "Makefile.dist") {
			fromDist = append(fromDist, e.FromSymbol)
		}
	}
	if len(fromDist) != 1 || !strings.HasSuffix(fromDist[0], ".package") {
		t.Errorf("the id'd run step's edge starts at %q, want the GHAStep package", fromDist)
	}
}

// TestIngest_GitHubActionsBuildPush (LB-F311): a docker/build-push-action
// step's literal tags reach the Dockerfile that builds them and the entities
// that name the image; an expression tag reaches nothing.
func TestIngest_GitHubActionsBuildPush(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		".github/workflows/img.yml": "on: push\njobs:\n  b:\n    runs-on: x\n    steps:\n      - uses: docker/build-push-action@v6\n        with:\n          context: ./api\n          tags: |\n            acme/api:1.0\n            ghcr.io/acme/other:${{ github.sha }}\n",
		"api/Dockerfile":            "FROM alpine\n",
		"deploy/compose.yml":        "services:\n  api:\n    image: acme/api:1.0\n  db:\n    image: postgres:16\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "img", Path: dir}); err != nil {
		t.Fatal(err)
	}
	var fromStep, toCompose, toDockerfile, other int
	for _, e := range crEdges(t, st, "img") {
		if !strings.HasPrefix(e, ".github/workflows/img.yml -> ") {
			continue
		}
		fromStep++
		switch {
		case strings.HasSuffix(e, "deploy/compose.api"):
			toCompose++
		case strings.Contains(e, "Dockerfile"):
			toDockerfile++
		default:
			other++
			t.Logf("edge %s", e)
		}
	}
	if toCompose != 1 || toDockerfile != 1 || other != 0 {
		t.Errorf("edges from the build step: %d total, compose %d, dockerfile %d, other %d; edges: %q", fromStep, toCompose, toDockerfile, other, crEdges(t, st, "img"))
	}
}
