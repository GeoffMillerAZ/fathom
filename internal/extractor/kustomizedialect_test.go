package extractor

// kustomizedialect_test.go — goldens for the kustomize doc-engine dialect
// (LB-Q-kustomize, a port of kustomizeextract.go, design §13.2 steps
// 10/12/14/15). The same golden shape as the msbuild and docker-compose
// dialects: symbols (name/kind/package/line/end_line/signature/calls/tier) and
// the cross-file references the engine records, whether or not
// fusion.ResolveDocXrefs can resolve them (a remote base, a directory that
// holds several manifests). A fixture is read as if it sat in
// k8s/overlays/prod/kustomization.yaml, so a relative reference resolves
// against a directory that has siblings and a parent.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

type kustomizeGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type kustomizeGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type kustomizeGolden struct {
	Symbols   []kustomizeGoldenSymbol `json:"symbols"`
	CrossFile []kustomizeGoldenCross  `json:"cross_file"`
}

// kustomizeFixtureRel is where every fixture is read as sitting.
const kustomizeFixtureRel = "k8s/overlays/prod/kustomization.yaml"

func loadKustomizeExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "kustomize" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(kustomize): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"kustomize\" not found in LoadDialects()")
	return nil
}

// kustomizeFixtures lists the testdata files the dialect owns: a golden sits
// beside each as <file>.golden.json.
func kustomizeFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "kustomize", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

// TestKustomizeDialectGoldens: every testdata fixture's kustomization,
// resources, patches, images and generators, and its cross-file references,
// against a hand-written golden.
func TestKustomizeDialectGoldens(t *testing.T) {
	x := loadKustomizeExtractor(t)
	for _, fx := range kustomizeFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", kustomizeFixtureRel, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got kustomizeGolden
			got.Symbols = []kustomizeGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != kustomizeFixtureRel {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, kustomizeGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []kustomizeGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, kustomizeGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want kustomizeGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []kustomizeGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []kustomizeGoldenCross{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("%s: symbols\n got: %+v\nwant: %+v", fx, got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("%s: cross_file\n got: %+v\nwant: %+v", fx, got.CrossFile, want.CrossFile)
			}
		})
	}
}

// kustomizeIncumbentFixture is the retired kustomizeextract_test.go's own
// fixture: the incumbent's cases, run against the dialect (design §13.2 step 15).
const kustomizeIncumbentFixture = `apiVersion: kustomize.config.k8s.io/v1beta1
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

func kustomizeByKind(nodes []core.SymbolNode, kind core.SourceKind) map[string]core.SymbolNode {
	m := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if n.Kind == kind {
			m[n.Name] = n
		}
	}
	return m
}

// TestKustomizeDialectKeepsIncumbentContracts: each case of the retired
// kustomizeextract_test.go (resources, patches, images, a file with no
// metadata.name, empty input, determinism), run against the dialect. Every SourceKind the
// incumbent emitted is kept (KustomizeResource, KustomizePatch,
// KustomizeImage) with the incumbent's names, tier and count, and a file with
// no metadata.name still yields symbols (the ADR-021 §2 defect).
//
// The one divergence is a Signature the incumbent composed from two fields
// ("Deployment/my-app", "registry.example.com/myapp:1.2.3"): the doc engine
// has no concatenation (§4), so the dialect signs a patch by its target's name
// (else its kind) and an image by its newName (else its newTag, else its
// digest), and the incumbent's spelling always contains the dialect's
// (PROVENANCE.md).
func TestKustomizeDialectKeepsIncumbentContracts(t *testing.T) {
	x := loadKustomizeExtractor(t)
	nodes, err := x.Extract("svc", "kustomization.yaml", []byte(kustomizeIncumbentFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("expected non-zero symbols from a valid kustomization.yaml (the ADR-021 §2 defect)")
	}

	resources := kustomizeByKind(nodes, core.KindKustomizeResource)
	if len(resources) != 2 {
		t.Fatalf("expected 2 resource nodes, got %d: %v", len(resources), resources)
	}
	for _, want := range []string{"deployment.yaml", "service.yaml"} {
		n, ok := resources[want]
		if !ok {
			t.Errorf("expected resource %q", want)
		} else if n.Tier != core.TierVerified {
			t.Errorf("resource %q Tier: got %q, want TierVerified", want, n.Tier)
		}
	}

	patches := kustomizeByKind(nodes, core.KindKustomizePatch)
	if len(patches) != 2 {
		t.Fatalf("expected 2 patch nodes (patchesStrategicMerge + patches), got %d: %v", len(patches), patches)
	}
	if _, ok := patches["patches/replica-count.yaml"]; !ok {
		t.Error("expected patchesStrategicMerge entry 'patches/replica-count.yaml'")
	}
	p, ok := patches["patches/add-label.yaml"]
	if !ok {
		t.Fatal("expected modern patches entry 'patches/add-label.yaml'")
	}
	if !strings.Contains("Deployment/my-app", p.Signature) || p.Signature == "" {
		t.Errorf("patch target Signature %q: want a part of the incumbent's %q", p.Signature, "Deployment/my-app")
	}

	images := kustomizeByKind(nodes, core.KindKustomizeImage)
	myapp, ok := images["myapp"]
	if !ok {
		t.Fatal("expected image 'myapp'")
	}
	if !strings.Contains("registry.example.com/myapp:1.2.3", myapp.Signature) || myapp.Signature == "" {
		t.Errorf("myapp Signature %q: want a part of the incumbent's %q", myapp.Signature, "registry.example.com/myapp:1.2.3")
	}
	sidecar, ok := images["sidecar"]
	if !ok {
		t.Fatal("expected image 'sidecar'")
	}
	if !strings.Contains("sidecar:latest", sidecar.Signature) || sidecar.Signature == "" {
		t.Errorf("sidecar Signature %q: want a part of the incumbent's %q", sidecar.Signature, "sidecar:latest")
	}

	empty, err := x.Extract("svc", "kustomization.yaml", []byte(""))
	if err != nil || len(empty) != 0 {
		t.Errorf("empty input: %d nodes, err %v; want none", len(empty), err)
	}
	a, _ := x.Extract("svc", "kustomization.yaml", []byte(kustomizeIncumbentFixture))
	b, _ := x.Extract("svc", "kustomization.yaml", []byte(kustomizeIncumbentFixture))
	if !reflect.DeepEqual(a, b) {
		t.Error("Extract is not deterministic")
	}
}
