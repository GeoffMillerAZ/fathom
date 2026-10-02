package extractor

// iacdialect_test.go — goldens for the ansible, argo, flux and crossplane
// doc-engine dialects (LB-Q-ansible, LB-Q-argo, LB-Q-flux, LB-Q-crossplane;
// LB-F151, LB-F152; language-breadth iac wave): entities, the intra-file
// references an entity's Calls carry, and the cross-file references (a source,
// a template, a role, an include path). A fixture is read as if it sat in
// infra/<name>. Goldens are written from reading the fixtures (design §13.2
// step 10); the shape and the extraction helper are the gateway dialects'.

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

var iacDialects = []string{"ansible", "argo", "flux", "crossplane"}

// TestIaCDialectGoldens: every fixture's entities, references and cross-file
// references against its golden. The fixtures are looped in the test's own
// body, not subtests, so a plan row's reachable_from can name this test.
func TestIaCDialectGoldens(t *testing.T) {
	for _, id := range iacDialects {
		x, err := NewDocDialectExtractor(id)
		if err != nil {
			t.Fatalf("NewDocDialectExtractor(%s): %v", id, err)
		}
		for _, fx := range gatewayFixtures(t, id) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			if n := bytes.Count(src, []byte("\n")); n > 200 {
				t.Errorf("%s: %d lines, the recipe's cap is 200", fx, n)
			}
			got := gatewayExtract(t, x, fx)
			raw, err := os.ReadFile(fx + ".golden.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want cfnGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", fx+".golden.json", err)
			}
			if want.Symbols == nil {
				want.Symbols = []cfnGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []cfnGoldenCross{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("%s: symbols\n got: %+v\nwant: %+v", fx, got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("%s: cross_file\n got: %+v\nwant: %+v", fx, got.CrossFile, want.CrossFile)
			}
		}
	}
}

// TestIaCDialectsDetectByContent: argo, flux and crossplane claim no name;
// their detect.content_sniff names the apiVersion API groups (and, for Argo,
// the kinds) of one document. Plain Kubernetes objects, the other tools'
// groups, an Argo kind outside the list (a Rollout) and a Crossplane
// user-defined composite are not theirs.
func TestIaCDialectsDetectByContent(t *testing.T) {
	cases := []struct {
		id   string
		src  string
		want bool
	}{
		{"argo", "apiVersion: argoproj.io/v1alpha1\nkind: Application\nmetadata:\n  name: a\n", true},
		{"argo", "kind: Workflow\napiVersion: argoproj.io/v1alpha1\nmetadata:\n  generateName: w-\n", true},
		{"argo", "apiVersion: \"argoproj.io/v1alpha1\"\nkind: 'CronWorkflow'\n", true},
		{"argo", "apiVersion: argoproj.io/v1alpha1\nkind: ClusterWorkflowTemplate\n", true},
		{"argo", "apiVersion: argoproj.io/v1alpha1\nkind: Rollout\nmetadata:\n  name: r\n", false},
		{"argo", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: d\n", false},
		{"argo", "apiVersion: v1\nkind: ConfigMap\ndata:\n  kind: Application\n  apiVersion: argoproj.io/v1alpha1\n", false},
		{"flux", "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\n", true},
		{"flux", "apiVersion: helm.toolkit.fluxcd.io/v2\nkind: HelmRelease\n", true},
		{"flux", "apiVersion: source.toolkit.fluxcd.io/v1\nkind: GitRepository\n", true},
		{"flux", "apiVersion: notification.toolkit.fluxcd.io/v1beta3\nkind: Alert\n", true},
		{"flux", "apiVersion: image.toolkit.fluxcd.io/v1\nkind: ImagePolicy\n", true},
		{"flux", "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n", false},
		{"flux", "apiVersion: v1\nkind: Namespace\nmetadata:\n  labels:\n    toolkit.fluxcd.io/tenant: a\n", false},
		{"crossplane", "apiVersion: apiextensions.crossplane.io/v1\nkind: Composition\n", true},
		{"crossplane", "apiVersion: apiextensions.crossplane.io/v2\nkind: CompositeResourceDefinition\n", true},
		{"crossplane", "apiVersion: pkg.crossplane.io/v1\nkind: Provider\n", true},
		{"crossplane", "apiVersion: meta.pkg.crossplane.io/v1\nkind: Configuration\n", true},
		{"crossplane", "apiVersion: platform.example.org/v1alpha1\nkind: XDatabase\n", false},
		{"crossplane", "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\n", false},
	}
	exts := map[string]*DocExtractor{}
	for _, c := range cases {
		x := exts[c.id]
		if x == nil {
			var err error
			if x, err = NewDocDialectExtractor(c.id); err != nil {
				t.Fatalf("NewDocDialectExtractor(%s): %v", c.id, err)
			}
			exts[c.id] = x
		}
		if got := x.DetectsContent([]byte(c.src)); got != c.want {
			t.Errorf("%s: DetectsContent(%q) = %v, want %v", c.id, c.src, got, c.want)
		}
	}
}
