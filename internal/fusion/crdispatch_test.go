package fusion

// crdispatch_test.go — LB-F149: Argo, Flux and Crossplane objects are claimed
// by their dialect's content sniff only when EVERY document of the file is
// that dialect's; a mixed file, a Helm template, a CRD and every other
// Kubernetes object keep the verdict they had; ingest reads a claimed file
// through its dialect and stores the cross-file edges; ReingestFile makes the
// decision ingest made.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	crFluxKustomization = "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: apps\n  namespace: flux-system\nspec:\n  interval: 10m\n  sourceRef:\n    kind: GitRepository\n    name: fleet\n  path: ./apps\n  dependsOn:\n    - name: infra\n"
	crFluxSource        = "apiVersion: source.toolkit.fluxcd.io/v1\nkind: GitRepository\nmetadata:\n  name: fleet\n  namespace: flux-system\nspec:\n  url: https://git.example.com/org/fleet.git\n"
	crFluxInfra         = "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: infra\n  namespace: flux-system\nspec:\n  sourceRef:\n    kind: GitRepository\n    name: fleet\n  path: ./infra\n"
	crNamespace         = "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: apps\n"
	crArgoApp           = "apiVersion: argoproj.io/v1alpha1\nkind: Application\nmetadata:\n  name: guestbook\n  namespace: argocd\nspec:\n  source:\n    repoURL: https://git.example.com/org/apps.git\n    path: apps/guestbook\n"
	crArgoWorkflow      = "apiVersion: argoproj.io/v1alpha1\nkind: Workflow\nmetadata:\n  name: w\nspec:\n  entrypoint: main\n  templates:\n  - name: main\n    steps:\n    - - name: hello\n        templateRef:\n          name: shared\n          template: say\n"
	crArgoTemplate      = "apiVersion: argoproj.io/v1alpha1\nkind: WorkflowTemplate\nmetadata:\n  name: shared\nspec:\n  templates:\n  - name: say\n    container:\n      image: busybox\n"
	crArgoRollout       = "apiVersion: argoproj.io/v1alpha1\nkind: Rollout\nmetadata:\n  name: r\nspec:\n  replicas: 2\n"
	crCrossplaneComp    = "apiVersion: apiextensions.crossplane.io/v1\nkind: Composition\nmetadata:\n  name: db\nspec:\n  compositeTypeRef:\n    apiVersion: platform.example.org/v1alpha1\n    kind: XDatabase\n  pipeline:\n  - step: render\n    functionRef:\n      name: function-render\n"
	crCrossplaneXRD     = "apiVersion: apiextensions.crossplane.io/v1\nkind: CompositeResourceDefinition\nmetadata:\n  name: xdatabases.platform.example.org\nspec:\n  group: platform.example.org\n  names:\n    kind: XDatabase\n    plural: xdatabases\n"
	crCrossplaneXR      = "apiVersion: platform.example.org/v1alpha1\nkind: XDatabase\nmetadata:\n  name: prod\nspec:\n  size: large\n"
	crDeployment        = "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec:\n  template:\n    spec:\n      containers:\n      - name: web\n        image: nginx:1.27\n"
	crCRD               = "apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: applications.argoproj.io\nspec:\n  group: argoproj.io\n"
	crKustomizeFile     = "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n- app.yaml\n"
)

func TestCRDispatchClaims(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		content string
		want    ToolKind // "" = not claimed
		mixed   bool
	}{
		{"flux kustomization", "clusters/apps.yaml", crFluxKustomization, ToolFlux, false},
		{"flux source", "clusters/source.yml", crFluxSource, ToolFlux, false},
		{"flux multi-document, all flux", "clusters/all.yaml", "---\n" + crFluxSource + "---\n" + crFluxKustomization, ToolFlux, false},
		{"flux beside a namespace is mixed", "clusters/infra.yaml", crNamespace + "---\n" + crFluxKustomization, "", true},
		{"flux then a namespace is mixed", "clusters/infra2.yaml", crFluxKustomization + "---\n" + crNamespace, "", true},
		{"flux and argo documents are mixed", "clusters/both.yaml", crFluxKustomization + "---\n" + crArgoApp, "", true},
		{"flux with comment and empty documents", "clusters/c.yaml", "# generated\n---\n" + crFluxSource + "---\n# nothing\n---\n", ToolFlux, false},
		{"argo application", "apps/guestbook.yaml", crArgoApp, ToolArgo, false},
		{"argo workflow", "wf/wf.yaml", crArgoWorkflow, ToolArgo, false},
		{"argo templates", "wf/tpl.yaml", crArgoTemplate, ToolArgo, false},
		{"argo application with CRLF line endings", "apps/crlf.yaml", strings.ReplaceAll(crArgoApp, "\n", "\r\n"), ToolArgo, false},
		{"flux documents separated by CRLF markers", "clusters/crlf.yaml", strings.ReplaceAll("---\n"+crFluxSource+"---\n"+crFluxKustomization, "\n", "\r\n"), ToolFlux, false},
		{"argo rollout stays kubernetes", "k8s/rollout.yaml", crArgoRollout, "", false},
		{"argo application beside a deployment is mixed", "apps/mixed.yaml", crArgoApp + "---\n" + crDeployment, "", true},
		{"crossplane composition", "apis/composition.yaml", crCrossplaneComp, ToolCrossplane, false},
		{"crossplane xrd", "apis/xrd.yaml", crCrossplaneXRD, ToolCrossplane, false},
		{"crossplane xrd and composition", "apis/both.yaml", crCrossplaneXRD + "---\n" + crCrossplaneComp, ToolCrossplane, false},
		{"a composite resource stays kubernetes", "examples/xr.yaml", crCrossplaneXR, "", false},
		{"an xrd with its composite resource is mixed", "examples/mixed.yaml", crCrossplaneXRD + "---\n" + crCrossplaneXR, "", true},
		{"plain deployment", "k8s/deploy.yaml", crDeployment, "", false},
		{"crd naming the argoproj group", "crds/app.yaml", crCRD, "", false},
		{"kustomize config", "k8s/kustomization.yaml", crKustomizeFile, "", false},
		{"not yaml", "wf/wf.json", crArgoWorkflow, "", false},
		{"no kind", "x/a.yaml", "apiVersion: argoproj.io/v1alpha1\nmetadata:\n  name: a\n", "", false},
	}
	for _, c := range cases {
		e, tk, ok := crDispatch(c.path, []byte(c.content), nil)
		if c.want == "" {
			if ok || e != nil {
				t.Errorf("%s: crDispatch(%q) claimed it as %q; want unclaimed", c.name, c.path, tk)
			}
		} else if !ok || tk != c.want || e == nil {
			t.Errorf("%s: crDispatch(%q) = (%v, %q, %v), want %q", c.name, c.path, e, tk, ok, c.want)
		}
		if got := CRMixed(c.path, []byte(c.content)); got != c.mixed {
			t.Errorf("%s: CRMixed(%q) = %v, want %v", c.name, c.path, got, c.mixed)
		}
	}
}

// TestCRDispatchKeepsEveryOtherVerdict: a CR file inside a Helm chart's
// templates/ is a Helm template; the dispatch preempts nothing but the
// plain Kubernetes verdict. Envoy, Kong and Traefik files and the plain
// Kubernetes verdicts are the ones they were.
func TestCRDispatchKeepsEveryOtherVerdict(t *testing.T) {
	idx := buildSiblingIndex([]core.SourceRef{{Path: "charts/app/Chart.yaml"}, {Path: "charts/app/templates/app.yaml"}})
	if _, _, ok := crDispatch("charts/app/templates/app.yaml", []byte(crArgoApp), idx); ok {
		t.Errorf("a CR in a chart's templates/ was claimed")
	}
	if got := Classify("charts/app/templates/app.yaml", []byte(crArgoApp), idx); got != ToolHelmTemplateYAML {
		t.Errorf("Classify(chart template) = %q, want %q", got, ToolHelmTemplateYAML)
	}
	if got := Classify("k8s/deploy.yaml", []byte(crDeployment), nil); got != ToolK8sManifest {
		t.Errorf("Classify(deployment) = %q", got)
	}
	if got := Classify("crds/app.yaml", []byte(crCRD), nil); got != ToolK8sCRD {
		t.Errorf("Classify(crd) = %q", got)
	}
	if got := Classify("examples/xr.yaml", []byte(crCrossplaneXR), nil); got != ToolK8sManifest {
		t.Errorf("Classify(composite resource) = %q", got)
	}
	// Envoy, Kong and Traefik stay what they were: the gateway hook runs first.
	for path, content := range gatewayRepoCorpus() {
		ge, gk, gok := gatewayDispatch(path, []byte(content))
		if _, _, cok := crDispatch(path, []byte(content), nil); cok && gok {
			t.Errorf("%s is claimed by both a gateway dialect (%s, %v) and a CR dialect", path, gk, ge != nil)
		}
	}
}

func crRepoCorpus() map[string]string {
	return map[string]string{
		"clusters/apps.yaml":         crFluxKustomization,
		"clusters/infra.yaml":        crFluxInfra,
		"clusters/source.yaml":       crFluxSource,
		"clusters/mixed.yaml":        crNamespace + "---\n" + crFluxKustomization,
		"argocd/guestbook.yaml":      crArgoApp,
		"apps/guestbook/deploy.yaml": crDeployment,
		"wf/wf.yaml":                 crArgoWorkflow,
		"wf/tpl.yaml":                crArgoTemplate,
		"k8s/rollout.yaml":           crArgoRollout,
		"apis/composition.yaml":      crCrossplaneComp,
		"apis/xrd.yaml":              crCrossplaneXRD,
		"examples/xr.yaml":           crCrossplaneXR,
		// a chart's template that is a plain Argo Application stays a Helm template
		"charts/app/Chart.yaml":         "apiVersion: v2\nname: app\nversion: 0.1.0\n",
		"charts/app/templates/app.yaml": crArgoApp,
	}
}

// crSourceKinds is each file's set of source kinds, joined sorted with ",".
func crSourceKinds(t *testing.T, st store.Store, svc string) map[string]string {
	t.Helper()
	obs, err := st.QueryObservations(context.Background(), svc, "")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]map[string]bool{}
	for _, o := range obs {
		if by[o.SourceRef] == nil {
			by[o.SourceRef] = map[string]bool{}
		}
		by[o.SourceRef][string(o.SourceKind)] = true
	}
	out := map[string]string{}
	for ref, ks := range by {
		var l []string
		for k := range ks {
			l = append(l, k)
		}
		sort.Strings(l)
		out[ref] = strings.Join(l, ",")
	}
	return out
}

// crHasKind reports whether the kinds list holds kind.
func crHasKind(kinds, kind string) bool {
	for _, k := range strings.Split(kinds, ",") {
		if k == kind {
			return true
		}
	}
	return false
}

func crEdges(t *testing.T, st store.Store, svc string) []string {
	t.Helper()
	all, err := st.TypedEdges(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			out = append(out, e.FromFile+" -> "+e.ToSymbol)
		}
	}
	sort.Strings(out)
	return out
}

// TestIngest_CRRepo: ingest reads each claimed file through its dialect, a
// mixed file and a Kubernetes object through the Kubernetes reading, and
// stores the cross-file edges (a sourceRef, a templateRef, a path, a
// compositeTypeRef) the dialects declare.
func TestIngest_CRRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, crRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cr", Path: dir}); err != nil {
		t.Fatal(err)
	}
	got := crSourceKinds(t, st, "cr")
	for rel, want := range map[string]string{
		"clusters/apps.yaml":    "FluxResource",
		"clusters/infra.yaml":   "FluxResource",
		"clusters/source.yaml":  "FluxResource",
		"argocd/guestbook.yaml": "ArgoResource",
		"wf/wf.yaml":            "ArgoResource",
		"wf/tpl.yaml":           "ArgoResource",
		"apis/composition.yaml": "CrossplaneResource",
		"apis/xrd.yaml":         "CrossplaneResource",
		"clusters/mixed.yaml":   "K8sWorkload",
		"k8s/rollout.yaml":      "K8sWorkload",
		"examples/xr.yaml":      "K8sWorkload",
	} {
		if !crHasKind(got[rel], want) {
			t.Errorf("%s read as %q, want %q among them", rel, got[rel], want)
		}
		if want == "K8sWorkload" && (crHasKind(got[rel], "FluxResource") || crHasKind(got[rel], "ArgoResource") || crHasKind(got[rel], "CrossplaneResource")) {
			t.Errorf("%s was claimed by a CR dialect: %q", rel, got[rel])
		}
	}
	if k := got["charts/app/templates/app.yaml"]; crHasKind(k, "ArgoResource") {
		t.Errorf("a Helm chart's template was claimed by the Argo dialect: %q", k)
	}
	edges := crEdges(t, st, "cr")
	want := []string{
		"apis/composition.yaml -> cr::apis/xrd.XDatabase",
		"clusters/apps.yaml -> cr::clusters/infra.Kustomization/flux-system/infra",
		"clusters/apps.yaml -> cr::clusters/source.GitRepository/flux-system/fleet",
		"clusters/infra.yaml -> cr::clusters/source.GitRepository/flux-system/fleet",
		"wf/wf.yaml -> cr::wf/tpl.WorkflowTemplate/shared",
	}
	for _, w := range want {
		found := false
		for _, e := range edges {
			if e == w {
				found = true
			}
		}
		if !found {
			t.Errorf("no stored edge %q; edges: %q", w, edges)
		}
	}
}

// TestReingestFile_CRRepo: a one-file re-ingest decides what ingest did.
func TestReingestFile_CRRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, crRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cr", Path: dir}); err != nil {
		t.Fatal(err)
	}
	before := crSourceKinds(t, st, "cr")
	for rel := range crRepoCorpus() {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, []byte("# edited\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "cr", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	after := crSourceKinds(t, st, "cr")
	if !reflect.DeepEqual(before, after) {
		t.Errorf("ReingestFile changed what a file reads as\nbefore: %v\n after: %v", before, after)
	}
}
