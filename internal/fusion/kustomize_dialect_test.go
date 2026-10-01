package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestKustomizeClassification (LB-Q-kustomize, design §5): the manifest claims
// a kustomization by its basename, kustomization.yaml or kustomization.yml, at
// any depth. A bare `Kustomization` file (kustomize reads it, the doc engine
// does not decode a file with no extension), a backup and a file named after
// the format are not claimed.
func TestKustomizeClassification(t *testing.T) {
	rules := dialectRules()
	cases := []struct {
		path, want string
		by         string
	}{
		{"kustomization.yaml", "kustomize", connector.ClassifiedByBasename},
		{"kustomization.yml", "kustomize", connector.ClassifiedByBasename},
		{"deploy/overlays/prod/kustomization.yaml", "kustomize", connector.ClassifiedByBasename},
		{"Kustomization", "", connector.ClassifiedNone},
		{"kustomization.yaml.bak", "", connector.ClassifiedNone},
	}
	for _, c := range cases {
		v := connector.ClassifyDialect(rules, c.path, nil)
		if v.Dialect != c.want || v.By != c.by {
			t.Errorf("ClassifyDialect(%q) = {%q %q}, want {%q %q}", c.path, v.Dialect, v.By, c.want, c.by)
		}
	}
	// a YAML file named after the tool is the Kubernetes dialect's, by extension
	if v := connector.ClassifyDialect(rules, "kustomize.yaml", nil); v.Dialect == "kustomize" {
		t.Errorf("ClassifyDialect(kustomize.yaml) = kustomize; a file named after the format is not a kustomization")
	}
	for _, name := range []string{"kustomization.yaml", "kustomization.yml"} {
		if lang := connector.LanguageForPath(name); lang != "kustomize" {
			t.Errorf("LanguageForPath(%q) = %q, want kustomize", name, lang)
		}
	}
	if got := connector.TypedEdgeProducer("kustomize"); got != "syntax" {
		t.Errorf("TypedEdgeProducer(kustomize) = %q, want syntax", got)
	}
}

// TestResolveDocXrefs_Kustomize pins LB-Q-kustomize's L3: a resource, a base, a
// component and a patch that name another kustomization or a directory of one
// resolve entity to entity (the kustomization root), written with or without
// a leading ./ ; a remote reference, one that names nothing the repository
// holds, one that climbs out of it and a reference to its own file resolve to
// nothing.
func TestResolveDocXrefs_Kustomize(t *testing.T) {
	files := docFiles(t, "kustomize", map[string]string{
		"overlays/prod/kustomization.yaml":       "resources:\n  - ../../base\n  - ./local\n  - github.com/example/app//deploy?ref=v1\n  - https://example.com/raw/x.yaml\n  - ../../missing\n  - ../../../outside\n  - .\ncomponents:\n  - ../../components/labels\npatches:\n  - path: ../../base/kustomization.yaml\n",
		"overlays/prod/local/kustomization.yaml": "resources:\n  - ../../../base\n",
		"base/kustomization.yaml":                "resources:\n  - deployment.yaml\n",
		"components/labels/kustomization.yml":    "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\n",
		"legacy/kustomization.yaml":              "bases:\n  - ../base\n",
	})
	got := xrefBrief(ResolveDocXrefs(files))
	want := []string{
		"legacy/kustomization.yaml:kustomization.yaml -base-> base/kustomization.yaml:kustomization.yaml",
		"overlays/prod/kustomization.yaml:kustomization.yaml -patch-> base/kustomization.yaml:kustomization.yaml",
		"overlays/prod/kustomization.yaml:kustomization.yaml -resource-> base/kustomization.yaml:kustomization.yaml",
		"overlays/prod/kustomization.yaml:kustomization.yaml -component-> components/labels/kustomization.yml:kustomization.yml",
		"overlays/prod/kustomization.yaml:kustomization.yaml -resource-> overlays/prod/local/kustomization.yaml:kustomization.yaml",
		"overlays/prod/local/kustomization.yaml:kustomization.yaml -resource-> base/kustomization.yaml:kustomization.yaml",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// TestKustomizeBaseDirectoryNeedsTheOneFileRule pins what fusion.ResolveDocXrefs
// does with a directory that holds a kustomization and another file a doc
// dialect reads: of the doc files in the directory, the one the referring
// dialect owns by its basenames (kustomization.yaml, kustomization.yml) is the
// one it names (O165), so a base resolves whether its Kubernetes manifests are
// doc files of the set (the day the kubernetes dialect is dispatched) or not.
func TestKustomizeBaseDirectoryNeedsTheOneFileRule(t *testing.T) {
	kz := docFiles(t, "kustomize", map[string]string{
		"overlay/kustomization.yaml": "resources:\n  - ../base\n",
		"base/kustomization.yaml":    "resources:\n  - deployment.yaml\n",
	})
	if got := xrefBrief(ResolveDocXrefs(kz)); len(got) != 1 {
		t.Fatalf("a base directory holding one doc file resolves: got %q", got)
	}
	k8s := docFiles(t, "kubernetes", map[string]string{
		"base/deployment.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n",
	})
	want := []string{
		"base/kustomization.yaml:kustomization.yaml -resource-> base/deployment.yaml:Deployment/web",
		"overlay/kustomization.yaml:kustomization.yaml -resource-> base/kustomization.yaml:kustomization.yaml",
	}
	if got := xrefBrief(ResolveDocXrefs(append(kz, k8s...))); !reflect.DeepEqual(got, want) {
		t.Fatalf("a base directory holding a second doc file resolves to its kustomization (O165)\n got: %q\nwant: %q", got, want)
	}
}

func kustomizeRepoCorpus() map[string]string {
	return map[string]string{
		"base/kustomization.yaml": "resources:\n  - deployment.yaml\n  - service.yaml\nimages:\n  - name: web\n    newName: registry.example.com/web\n    newTag: \"1.4\"\n",
		"base/deployment.yaml":    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: web\n",
		"base/service.yaml":       "apiVersion: v1\nkind: Service\nmetadata:\n  name: web\nspec:\n  selector:\n    app: web\n",
		"overlays/prod/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - ../../base\n  - ../../components/labels\n  - https://example.com/raw/extra.yaml\n  - ../../missing\n" +
			"patches:\n  - path: replicas.yaml\n    target:\n      kind: Deployment\n      name: web\nconfigMapGenerator:\n  - name: web-env\n    literals:\n      - LEVEL=info\n",
		"overlays/prod/replicas.yaml":          "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec:\n  replicas: 5\n",
		"overlays/dev/kustomization.yml":       "resources:\n  - ../../base\nnamePrefix: dev-\n",
		"components/labels/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\ncommonLabels:\n  team: web\n",
		"Kustomization":                        "resources:\n  - not-read.yaml\n",
		"kustomization.yaml.bak":               "resources:\n  - not-read.yaml\n",
	}
}

// TestIngest_KustomizeRepo (the product-reach proof): an ingest extracts every
// kustomization.yaml and kustomization.yml through the kustomize dialect, tags
// each ToolKustomize, leaves a bare Kustomization file and a backup alone, and
// stores the references that resolve, a resource or a component naming another
// kustomization's directory, as file-tier typed edges from the referring
// kustomization to the one it names. A base's own manifests reach the
// Kubernetes extractor and a remote or missing base names no file.
func TestIngest_KustomizeRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, kustomizeRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "kz", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "kz", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	tools := map[string]bool{}
	for _, o := range obs {
		switch o.SourceKind {
		case core.KindKustomization, core.KindKustomizeResource, core.KindKustomizePatch, core.KindKustomizeImage, "KustomizeConfigMap":
			got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = true
			tools[o.ToolKind] = true
		}
		if o.SourceRef == "Kustomization" || o.SourceRef == "kustomization.yaml.bak" {
			t.Errorf("%s|%s: a file that is not a kustomization was extracted as %s", o.SourceRef, o.Symbol, o.SourceKind)
		}
	}
	for _, w := range []string{
		"base/kustomization.yaml|Kustomization|kustomization.yaml",
		"base/kustomization.yaml|KustomizeResource|deployment.yaml",
		"base/kustomization.yaml|KustomizeResource|service.yaml",
		"base/kustomization.yaml|KustomizeImage|web",
		"overlays/prod/kustomization.yaml|Kustomization|kustomization.yaml",
		"overlays/prod/kustomization.yaml|KustomizeResource|../../base",
		"overlays/prod/kustomization.yaml|KustomizeResource|../../components/labels",
		"overlays/prod/kustomization.yaml|KustomizePatch|replicas.yaml",
		"overlays/prod/kustomization.yaml|KustomizeConfigMap|web-env",
		"overlays/dev/kustomization.yml|Kustomization|kustomization.yml",
		"components/labels/kustomization.yaml|Kustomization|kustomization.yaml",
	} {
		if !got[w] {
			t.Errorf("missing observation %s\n have: %v", w, sortedBoolKeys(got))
		}
	}
	if len(tools) != 1 || !tools[string(ToolKustomize)] {
		t.Errorf("tool kinds on kustomize observations = %v, want only %q", tools, ToolKustomize)
	}
	all, _ := st.TypedEdges(ctx, "kz")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{
		"overlays/dev/kustomization.yml->kz::base/kustomization.kustomization.yaml",
		"overlays/prod/kustomization.yaml->kz::base/kustomization.kustomization.yaml",
		"overlays/prod/kustomization.yaml->kz::components/labels/kustomization.kustomization.yaml",
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, wantEdges)
	}
	if rep.DocXrefEdges != 3 {
		t.Errorf("DocXrefEdges = %d, want 3", rep.DocXrefEdges)
	}
}

// TestReingestFile_KustomizeRepo: the one-file refresh reads a kustomization
// through the same dialect an ingest does, so an edit that adds an image shows
// as a KustomizeImage symbol (the basename, not the .yaml extension's
// classifier, decides).
func TestReingestFile_KustomizeRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, kustomizeRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "kz", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "overlays/dev/kustomization.yml"
	edited := "resources:\n  - ../../base\nimages:\n  - name: cache\n    newTag: \"7\"\n"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "kz", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "kz", "")
	if err != nil {
		t.Fatal(err)
	}
	var images []string
	for _, o := range obs {
		if o.SourceRef == rel && o.SourceKind == core.KindKustomizeImage {
			images = append(images, o.Symbol)
		}
	}
	if want := []string{"cache"}; !reflect.DeepEqual(images, want) {
		t.Fatalf("images of %s after ReingestFile = %v, want %v", rel, images, want)
	}
}

// TestKustomizeDialectIsWhatFusionDispatches: the extractor fusion.New maps to a
// kustomization basename is the doc-engine dialect (it carries a DocResult),
// not the retired bespoke extractor.
func TestKustomizeDialectIsWhatFusionDispatches(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	for _, name := range []string{"kustomization.yaml", "deploy/overlays/prod/kustomization.yml"} {
		e, tk, ok, _ := p.resolveExtractor(name, []byte("resources:\n  - a.yaml\n"), buildSiblingIndex(nil))
		if !ok {
			t.Fatalf("%s: no extractor", name)
		}
		if _, isDoc := e.(extractor.DocResulter); !isDoc {
			t.Errorf("%s dispatches to %T, want the doc-engine dialect", name, e)
		}
		if tk != ToolKustomize {
			t.Errorf("%s tool kind = %q, want %q", name, tk, ToolKustomize)
		}
	}
}

func sortedBoolKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
