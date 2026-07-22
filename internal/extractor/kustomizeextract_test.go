package extractor_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const kustomizationFixture = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - deployment.yaml
  - service.yaml

patchesStrategicMerge:
  - patches/replica-count.yaml

patches:
  - path: patches/add-label.yaml
    target:
      kind: Deployment
      name: my-app

images:
  - name: myapp
    newName: registry.example.com/myapp
    newTag: "1.2.3"
  - name: sidecar
    newTag: latest
`

func TestKustomizeExtractor_Resources(t *testing.T) {
	x := extractor.NewKustomizeExtractor()
	nodes, err := x.Extract("svc", "kustomization.yaml", []byte(kustomizationFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	var resources []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindKustomizeResource {
			resources = append(resources, n)
		}
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resource nodes, got %d: %v", len(resources), kustomizeNodeNames(nodes))
	}
	byName := kustomizeByName(resources)
	for _, want := range []string{"deployment.yaml", "service.yaml"} {
		if n, ok := byName[want]; !ok {
			t.Errorf("expected resource %q", want)
		} else if n.Tier != core.TierVerified {
			t.Errorf("resource %q Tier: got %q, want TierVerified", want, n.Tier)
		}
	}
}

func TestKustomizeExtractor_Patches(t *testing.T) {
	x := extractor.NewKustomizeExtractor()
	nodes, err := x.Extract("svc", "kustomization.yaml", []byte(kustomizationFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	var patches []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindKustomizePatch {
			patches = append(patches, n)
		}
	}
	if len(patches) != 2 {
		t.Fatalf("expected 2 patch nodes (patchesStrategicMerge + patches), got %d: %v", len(patches), kustomizeNodeNames(nodes))
	}
	byName := kustomizeByName(patches)
	if _, ok := byName["patches/replica-count.yaml"]; !ok {
		t.Error("expected patchesStrategicMerge entry 'patches/replica-count.yaml'")
	}
	n, ok := byName["patches/add-label.yaml"]
	if !ok {
		t.Fatal("expected modern patches entry 'patches/add-label.yaml'")
	}
	if n.Signature != "Deployment/my-app" {
		t.Errorf("patch target Signature: got %q, want %q", n.Signature, "Deployment/my-app")
	}
}

func TestKustomizeExtractor_Images(t *testing.T) {
	x := extractor.NewKustomizeExtractor()
	nodes, err := x.Extract("svc", "kustomization.yaml", []byte(kustomizationFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	var images []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindKustomizeImage {
			images = append(images, n)
		}
	}
	byName := kustomizeByName(images)
	myapp, ok := byName["myapp"]
	if !ok {
		t.Fatal("expected image 'myapp'")
	}
	if myapp.Signature != "registry.example.com/myapp:1.2.3" {
		t.Errorf("myapp Signature: got %q, want %q", myapp.Signature, "registry.example.com/myapp:1.2.3")
	}
	sidecar, ok := byName["sidecar"]
	if !ok {
		t.Fatal("expected image 'sidecar'")
	}
	if sidecar.Signature != "sidecar:latest" {
		t.Errorf("sidecar Signature: got %q, want %q", sidecar.Signature, "sidecar:latest")
	}
}

func TestKustomizeExtractor_NoMetadataNameStillProducesSymbols(t *testing.T) {
	// The grounded defect (ADR-021 §2): a kustomization.yaml has no
	// metadata.name, so routing it through the generic k8s extractor
	// produced zero symbols. This extractor must produce real output from
	// the very same file.
	x := extractor.NewKustomizeExtractor()
	nodes, err := x.Extract("svc", "kustomization.yaml", []byte(kustomizationFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected non-zero symbols from a valid kustomization.yaml")
	}
}

func TestKustomizeExtractor_Empty(t *testing.T) {
	x := extractor.NewKustomizeExtractor()
	nodes, err := x.Extract("svc", "kustomization.yaml", []byte(""))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty input, got %d", len(nodes))
	}
}

func TestKustomizeExtractor_Determinism(t *testing.T) {
	x := extractor.NewKustomizeExtractor()
	a, _ := x.Extract("svc", "kustomization.yaml", []byte(kustomizationFixture))
	b, _ := x.Extract("svc", "kustomization.yaml", []byte(kustomizationFixture))
	if !reflect.DeepEqual(a, b) {
		t.Error("Extract is not deterministic")
	}
}

func kustomizeByName(nodes []core.SymbolNode) map[string]core.SymbolNode {
	m := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		m[n.Name] = n
	}
	return m
}

func kustomizeNodeNames(nodes []core.SymbolNode) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = string(n.Kind) + ":" + n.Name
	}
	return names
}
