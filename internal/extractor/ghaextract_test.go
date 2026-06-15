package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
)

// ghaFixture is a GitHub Actions workflow with:
//   - a workflow name
//   - on: [push, pull_request]
//   - two jobs where "deploy" needs "build"
//   - steps using actions/checkout@v4 and a bare run step
const ghaFixture = `name: CI Pipeline
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      - name: Run tests
        run: go test ./...
  deploy:
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v4
`

func TestGHAExtractor_Basic(t *testing.T) {
	x := extractor.NewGHAExtractor()
	nodes, err := x.Extract("svc", ".github/workflows/ci.yml", []byte(ghaFixture))
	if err != nil {
		t.Fatalf("Extract returned unexpected error: %v", err)
	}

	byKindName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKindName[string(n.Kind)+":"+n.Name] = n
	}

	t.Run("KindGHAWorkflow emitted", func(t *testing.T) {
		n, ok := byKindName["GHAWorkflow:CI Pipeline"]
		if !ok {
			t.Fatalf("expected GHAWorkflow node 'CI Pipeline'; got keys: %v", ghaNodeKeys(nodes))
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
		if n.Service != "svc" {
			t.Errorf("Service: got %q, want svc", n.Service)
		}
		if n.FilePath != ".github/workflows/ci.yml" {
			t.Errorf("FilePath: got %q, want .github/workflows/ci.yml", n.FilePath)
		}
		// Signature should mention push and pull_request.
		if n.Signature == "" {
			t.Error("Signature should not be empty")
		}
		if !containsAll(n.Signature, "push", "pull_request") {
			t.Errorf("Signature %q should mention push and pull_request", n.Signature)
		}
	})

	t.Run("KindGHAJob build emitted", func(t *testing.T) {
		n, ok := byKindName["GHAJob:build"]
		if !ok {
			t.Fatalf("expected GHAJob node 'build'; got keys: %v", ghaNodeKeys(nodes))
		}
		if n.Kind != core.KindGHAJob {
			t.Errorf("Kind: got %q, want KindGHAJob", n.Kind)
		}
		if n.Signature != "ubuntu-latest" {
			t.Errorf("Signature (runs-on): got %q, want ubuntu-latest", n.Signature)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier: got %q, want TierVerified", n.Tier)
		}
		if len(n.Calls) != 0 {
			t.Errorf("build job should have no needs/Calls, got: %v", n.Calls)
		}
	})

	t.Run("KindGHAJob deploy with needs edge", func(t *testing.T) {
		n, ok := byKindName["GHAJob:deploy"]
		if !ok {
			t.Fatalf("expected GHAJob node 'deploy'; got keys: %v", ghaNodeKeys(nodes))
		}
		if n.Kind != core.KindGHAJob {
			t.Errorf("Kind: got %q, want KindGHAJob", n.Kind)
		}
		if len(n.Calls) != 1 || n.Calls[0] != "build" {
			t.Errorf("deploy Calls: got %v, want [build]", n.Calls)
		}
	})

	t.Run("KindGHAAction for uses step", func(t *testing.T) {
		// Find all GHAAction nodes for actions/checkout@v4 (appears in both jobs).
		var checkoutNodes []core.SymbolNode
		for _, n := range nodes {
			if n.Kind == core.KindGHAAction && n.Name == "actions/checkout@v4" {
				checkoutNodes = append(checkoutNodes, n)
			}
		}
		if len(checkoutNodes) == 0 {
			t.Fatalf("expected GHAAction node 'actions/checkout@v4'; got keys: %v", ghaNodeKeys(nodes))
		}
		for _, n := range checkoutNodes {
			if n.Tier != core.TierVerified {
				t.Errorf("Tier: got %q, want TierVerified", n.Tier)
			}
		}
		// At least one occurrence must have the step name "Checkout code".
		found := false
		for _, n := range checkoutNodes {
			if n.Signature == "Checkout code" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected one GHAAction node with Signature 'Checkout code'; got signatures: %v",
				func() []string {
					sigs := make([]string, len(checkoutNodes))
					for i, n := range checkoutNodes {
						sigs[i] = n.Signature
					}
					return sigs
				}())
		}
	})

	t.Run("run step does not produce GHAAction", func(t *testing.T) {
		// "go test ./..." is a run step, not a uses step — no GHAAction for it.
		for _, n := range nodes {
			if n.Kind == core.KindGHAAction && n.Name == "go test ./..." {
				t.Errorf("run step should not produce a GHAAction node")
			}
		}
	})
}

func TestGHAExtractor_Determinism(t *testing.T) {
	x := extractor.NewGHAExtractor()
	n1, _ := x.Extract("svc", ".github/workflows/ci.yml", []byte(ghaFixture))
	n2, _ := x.Extract("svc", ".github/workflows/ci.yml", []byte(ghaFixture))
	if len(n1) != len(n2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Kind != n2[i].Kind || n1[i].Name != n2[i].Name {
			t.Errorf("non-deterministic at [%d]: %q/%q vs %q/%q",
				i, n1[i].Kind, n1[i].Name, n2[i].Kind, n2[i].Name)
		}
	}
}

func TestGHAExtractor_WorkflowNameFallback(t *testing.T) {
	// A workflow without a `name:` key should use the file basename.
	const noName = `on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo hello
`
	x := extractor.NewGHAExtractor()
	nodes, err := x.Extract("svc", ".github/workflows/my-workflow.yml", []byte(noName))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var wf *core.SymbolNode
	for i := range nodes {
		if nodes[i].Kind == core.KindGHAWorkflow {
			wf = &nodes[i]
			break
		}
	}
	if wf == nil {
		t.Fatal("expected KindGHAWorkflow node")
	}
	if wf.Name != "my-workflow" {
		t.Errorf("Name fallback: got %q, want my-workflow", wf.Name)
	}
}

func TestGHAExtractor_NeedsAsList(t *testing.T) {
	// `needs` can be a YAML list.
	const withList = `name: Multi
on: push
jobs:
  a:
    runs-on: ubuntu-latest
    steps:
      - run: echo a
  b:
    runs-on: ubuntu-latest
    steps:
      - run: echo b
  c:
    runs-on: ubuntu-latest
    needs: [a, b]
    steps:
      - run: echo c
`
	x := extractor.NewGHAExtractor()
	nodes, _ := x.Extract("svc", ".github/workflows/multi.yml", []byte(withList))
	byName := make(map[string]core.SymbolNode)
	for _, n := range nodes {
		byName[string(n.Kind)+":"+n.Name] = n
	}
	c, ok := byName["GHAJob:c"]
	if !ok {
		t.Fatal("expected GHAJob:c")
	}
	if len(c.Calls) != 2 {
		t.Errorf("expected 2 needs, got %d: %v", len(c.Calls), c.Calls)
	}
}

func TestGHAExtractor_MalformedYAML(t *testing.T) {
	// Completely broken YAML should produce no nodes and no error.
	x := extractor.NewGHAExtractor()
	nodes, err := x.Extract("svc", ".github/workflows/broken.yml", []byte("{{not yaml"))
	if err != nil {
		t.Errorf("Extract should not return error for malformed YAML, got: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for malformed YAML, got %d", len(nodes))
	}
}

// ── YAMLExtractor dispatcher tests ──────────────────────────────────────────

// k8sDispatchFixture is a minimal valid Kubernetes Deployment manifest for
// dispatcher routing verification.
const k8sDispatchFixture = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-server
`

func TestYAMLExtractor_RoutesGHAToGHAExtractor(t *testing.T) {
	k8sX := extractor.NewK8sExtractor()
	ghaX := extractor.NewGHAExtractor()
	y := extractor.NewYAMLExtractor(k8sX, ghaX)

	nodes, err := y.Extract("svc", ".github/workflows/ci.yml", []byte(ghaFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected nodes from GHA fixture")
	}
	// At least one KindGHAWorkflow must be present.
	found := false
	for _, n := range nodes {
		if n.Kind == core.KindGHAWorkflow {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected at least one KindGHAWorkflow node; got: %v", ghaNodeKeys(nodes))
	}
}

func TestYAMLExtractor_RoutesK8sToK8sExtractor(t *testing.T) {
	k8sX := extractor.NewK8sExtractor()
	ghaX := extractor.NewGHAExtractor()
	y := extractor.NewYAMLExtractor(k8sX, ghaX)

	nodes, err := y.Extract("svc", "deploy.yaml", []byte(k8sDispatchFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected nodes from k8s fixture")
	}
	// Should contain a K8sWorkload node, not a GHA node.
	found := false
	for _, n := range nodes {
		if n.Kind == core.KindK8sWorkload {
			found = true
		}
		if n.Kind == core.KindGHAWorkflow || n.Kind == core.KindGHAJob || n.Kind == core.KindGHAAction {
			t.Errorf("unexpected GHA node in k8s dispatch: %v", n)
		}
	}
	if !found {
		t.Errorf("expected at least one KindK8sWorkload node; got: %v", ghaNodeKeys(nodes))
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func ghaNodeKeys(nodes []core.SymbolNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = string(n.Kind) + ":" + n.Name
	}
	return out
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
