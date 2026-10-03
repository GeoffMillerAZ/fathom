package extractor

// timonidialect_test.go — goldens and contracts for the Timoni native dialect
// (LB-Q-timoni, LB-F217). Every fixture under dialects/timoni/testdata is
// extracted as the repository path svc/<relative path>; the golden (a hand-
// checked JSON beside it, rewritten with TIMONI_UPDATE_GOLDEN=1) holds the
// nodes, roots, cross-file references and label sets.

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

type timoniGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
}

type timoniGoldenCross struct {
	From     string            `json:"from"`
	Edge     string            `json:"edge"`
	Raw      string            `json:"raw"`
	Target   string            `json:"target,omitempty"`
	Strategy string            `json:"strategy"`
	Line     int               `json:"line"`
	Scope    string            `json:"scope,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type timoniGoldenLabels struct {
	Entity string            `json:"entity"`
	Scope  string            `json:"scope"`
	Labels map[string]string `json:"labels"`
	Line   int               `json:"line"`
}

type timoniGolden struct {
	Path       string               `json:"path"`
	Roots      []string             `json:"roots"`
	Symbols    []timoniGoldenSymbol `json:"symbols"`
	CrossFile  []timoniGoldenCross  `json:"cross_file"`
	Labels     []timoniGoldenLabels `json:"labels"`
	Unresolved []TimoniUnresolved   `json:"unresolved"`
}

func timoniFixtures(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("dialects", "timoni", "testdata")
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(p) == ".cue" {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	if len(out) < 8 {
		t.Fatalf("%s: %d fixtures, want at least 8", root, len(out))
	}
	return out
}

func timoniRepoPath(fx string) string {
	rel, _ := filepath.Rel(filepath.Join("dialects", "timoni", "testdata"), fx)
	return "svc/" + filepath.ToSlash(rel)
}

func timoniGoldenOf(t *testing.T, fx string) timoniGolden {
	t.Helper()
	src, err := os.ReadFile(fx)
	if err != nil {
		t.Fatal(err)
	}
	rel := timoniRepoPath(fx)
	r, _ := NewTimoniExtractor().ExtractDoc("svc", rel, src)
	g := timoniGolden{Path: rel, Roots: r.Roots, Unresolved: TimoniUnresolvedRefs(rel, src)}
	if g.Roots == nil {
		g.Roots = []string{}
	}
	if g.Unresolved == nil {
		g.Unresolved = []TimoniUnresolved{}
	}
	g.Symbols = []timoniGoldenSymbol{}
	for _, n := range r.Nodes {
		g.Symbols = append(g.Symbols, timoniGoldenSymbol{n.Name, string(n.Kind), n.Package, n.Line, n.EndLine, n.Signature, n.Calls})
	}
	g.CrossFile = []timoniGoldenCross{}
	for _, c := range r.CrossFile {
		g.CrossFile = append(g.CrossFile, timoniGoldenCross{c.From, c.Edge, c.Raw, c.Target, c.Strategy, c.Line, c.Scope, c.Labels})
	}
	g.Labels = []timoniGoldenLabels{}
	for _, l := range r.Labels {
		g.Labels = append(g.Labels, timoniGoldenLabels{l.Entity, l.Scope, l.Labels, l.Line})
	}
	return g
}

// TestTimoniDialectGoldens: every fixture's nodes, roots, references and
// labels against its golden.
func TestTimoniDialectGoldens(t *testing.T) {
	for _, fx := range timoniFixtures(t) {
		t.Run(filepath.ToSlash(fx), func(t *testing.T) {
			got := timoniGoldenOf(t, fx)
			gp := strings.TrimSuffix(fx, ".cue") + ".golden.json"
			if os.Getenv("TIMONI_UPDATE_GOLDEN") != "" {
				b, err := json.MarshalIndent(got, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(gp, append(b, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			b, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("%v (TIMONI_UPDATE_GOLDEN=1 writes it)", err)
			}
			var want timoniGolden
			if err := json.Unmarshal(b, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				gb, _ := json.MarshalIndent(got, "", "  ")
				t.Errorf("%s differs from %s:\n%s", fx, gp, gb)
			}
			again := timoniGoldenOf(t, fx)
			if !reflect.DeepEqual(got, again) {
				t.Errorf("%s: two extractions differ (determinism is a contract)", fx)
			}
		})
	}
}

func timoniNodes(t *testing.T, rel, fx string) []core.SymbolNode {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("dialects", "timoni", "testdata", fx))
	if err != nil {
		t.Fatal(err)
	}
	ns, err := NewTimoniExtractor().Extract("svc", rel, src)
	if err != nil {
		t.Fatal(err)
	}
	return ns
}

func timoniNode(ns []core.SymbolNode, name string) (core.SymbolNode, bool) {
	for _, n := range ns {
		if n.Name == name {
			return n, true
		}
	}
	return core.SymbolNode{}, false
}

// TestTimoniComposesTheCUEExtraction: every node the CUE extractor gives is
// in the Timoni result unchanged (the dialect adds, never replaces), and the
// Timoni entities come on top.
func TestTimoniComposesTheCUEExtraction(t *testing.T) {
	for _, fx := range []string{"app/timoni.cue", "app/values.cue", "app/templates/deployment.cue", "bundles/bundle.cue"} {
		src, err := os.ReadFile(filepath.Join("dialects", "timoni", "testdata", fx))
		if err != nil {
			t.Fatal(err)
		}
		rel := "svc/" + fx
		base, _ := NewCUEExtractor().Extract("svc", rel, src)
		got, _ := NewTimoniExtractor().Extract("svc", rel, src)
		// A CUE field may be declared more than once (timoni.cue's two
		// `config:`), so a node is found by its name, kind and line together.
		type at struct {
			name string
			kind core.SourceKind
			line int
		}
		have := map[at]bool{}
		for _, n := range got {
			have[at{n.Name, n.Kind, n.Line}] = true
		}
		for _, b := range base {
			if !have[at{b.Name, b.Kind, b.Line}] {
				t.Errorf("%s: CUE node %q (%s, line %d) lost or changed", fx, b.Name, b.Kind, b.Line)
			}
		}
		if len(got) <= len(base) {
			t.Errorf("%s: no Timoni entity added to the %d CUE nodes", fx, len(base))
		}
	}
}

// TestTimoniEntities: the entities each file role carries.
func TestTimoniEntities(t *testing.T) {
	cases := []struct {
		fx    string
		names map[string]core.SourceKind
	}{
		{"app/timoni.cue", map[string]core.SourceKind{"timoni/module": kindTimoniModule, "timoni/values": kindTimoniValues}},
		{"app/values.cue", map[string]core.SourceKind{"timoni/values": kindTimoniValues}},
		{"app/templates/deployment.cue", map[string]core.SourceKind{"timoni/object/#Deployment": kindTimoniObject}},
		{"app/templates/service.cue", map[string]core.SourceKind{"timoni/object/#Service": kindTimoniObject, "timoni/object/#TestJob": kindTimoniObject}},
		{"bundles/bundle.cue", map[string]core.SourceKind{
			"timoni/bundle": kindTimoniBundle, "timoni/bundle/instance/web": kindTimoniBundleInstance,
			"timoni/bundle/instance/cache": kindTimoniBundleInstance, "timoni/bundle/instance/shared.db": kindTimoniBundleInstance,
		}},
		{"bundles/runtime.cue", map[string]core.SourceKind{
			"timoni/runtime": kindTimoniRuntime, "timoni/runtime/cluster/staging": kindTimoniCluster,
			"timoni/runtime/cluster/production": kindTimoniCluster,
		}},
	}
	for _, c := range cases {
		ns := timoniNodes(t, "svc/"+c.fx, c.fx)
		for name, kind := range c.names {
			n, ok := timoniNode(ns, name)
			if !ok || n.Kind != kind {
				t.Errorf("%s: want %s %q, got %v (%v)", c.fx, kind, name, n.Kind, ok)
			}
		}
	}
	// A definition with no literal kind is no object.
	ns := timoniNodes(t, "svc/app/templates/service.cue", "app/templates/service.cue")
	if _, ok := timoniNode(ns, "timoni/object/#Selector"); ok {
		t.Error("#Selector has no literal kind and is no object")
	}
	// Plain CUE gets no Timoni entity.
	plain, _ := NewTimoniExtractor().Extract("svc", "svc/schema.cue", []byte("package schema\n\n#Thing: {a: int}\n"))
	for _, n := range plain {
		if strings.HasPrefix(n.Name, "timoni/") {
			t.Errorf("plain CUE got %q", n.Name)
		}
	}
}

// TestTimoniValuesCallsConfigFields: a struct-literal values: calls #Config.<k>
// for each top-level key, and a template that takes #config calls #Config.
func TestTimoniValuesCallsConfigFields(t *testing.T) {
	ns := timoniNodes(t, "svc/app/values.cue", "app/values.cue")
	v, ok := timoniNode(ns, "timoni/values")
	if !ok {
		t.Fatal("no timoni/values")
	}
	want := []string{"#Config.image", "#Config.message", "#Config.replicas"}
	if !reflect.DeepEqual(v.Calls, want) {
		t.Errorf("values calls = %v, want %v", v.Calls, want)
	}
	ns = timoniNodes(t, "svc/app/templates/deployment.cue", "app/templates/deployment.cue")
	d, ok := timoniNode(ns, "#Deployment")
	if !ok {
		t.Fatal("no #Deployment")
	}
	found := false
	for _, c := range d.Calls {
		if c == "#Config" {
			found = true
		}
	}
	if !found {
		t.Errorf("#Deployment calls = %v, want #Config among them", d.Calls)
	}
}

// TestTimoniBundleModuleRefs: a file:// url resolves to the module's
// timoni.cue relative to the bundle's directory; every other url is counted
// unresolved with its reason, never guessed.
func TestTimoniBundleModuleRefs(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("dialects", "timoni", "testdata", "bundles", "bundle.cue"))
	if err != nil {
		t.Fatal(err)
	}
	rel := "svc/bundles/bundle.cue"
	r, _ := NewTimoniExtractor().ExtractDoc("svc", rel, src)
	var targets []string
	for _, c := range r.CrossFile {
		if c.Edge == timoniEdgeModule {
			if c.Strategy != "repo-path" {
				t.Errorf("module ref strategy %q, want repo-path", c.Strategy)
			}
			targets = append(targets, c.From+" -> "+c.Target)
		}
	}
	sort.Strings(targets)
	want := []string{
		"timoni/bundle/instance/shared.db -> elsewhere/timoni.cue",
		"timoni/bundle/instance/web -> svc/app/timoni.cue",
	}
	if !reflect.DeepEqual(targets, want) {
		t.Errorf("module targets = %v, want %v", targets, want)
	}
	if len(r.Roots) != 1 || r.Roots[0] != "timoni/bundle" {
		t.Errorf("roots = %v, want [timoni/bundle]", r.Roots)
	}
	reasons := map[string]string{}
	for _, u := range TimoniUnresolvedRefs(rel, src) {
		reasons[u.Raw] = u.Reason
	}
	if reasons["oci://registry.example.test/modules/cache"] != "external:oci" || reasons["file://../../../outside"] != "outside-repo" || reasons[""] != "expression" {
		t.Errorf("unresolved reasons = %v", reasons)
	}
}

// TestTimoniInstanceRefs: #Instance names the object templates it renders, as
// label-selector references scoped by the module root, and each object
// declares the label set the reference selects.
func TestTimoniInstanceRefs(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("dialects", "timoni", "testdata", "app", "templates", "config.cue"))
	if err != nil {
		t.Fatal(err)
	}
	r, _ := NewTimoniExtractor().ExtractDoc("svc", "svc/app/templates/config.cue", src)
	var defs []string
	for _, c := range r.CrossFile {
		if c.Strategy == CUEImportStrategy || c.Strategy == CUEPackageStrategy {
			continue // the composed CUE extraction's own references, kept unchanged
		}
		if c.Edge != timoniEdgeRenders || c.Strategy != "label-selector" || c.From != "#Instance" || c.Scope != "timoni:svc/app" {
			t.Errorf("unexpected reference %+v", c)
			continue
		}
		defs = append(defs, c.Labels["def"])
	}
	sort.Strings(defs)
	if want := []string{"#Deployment", "#Service", "#TestJob"}; !reflect.DeepEqual(defs, want) {
		t.Errorf("instance renders %v, want %v", defs, want)
	}
	dsrc, _ := os.ReadFile(filepath.Join("dialects", "timoni", "testdata", "app", "templates", "deployment.cue"))
	dr, _ := NewTimoniExtractor().ExtractDoc("svc", "svc/app/templates/deployment.cue", dsrc)
	if len(dr.Labels) != 1 || dr.Labels[0].Entity != "timoni/object/#Deployment" || dr.Labels[0].Scope != "timoni:svc/app" ||
		dr.Labels[0].Labels["pkg"] != "templates" || dr.Labels[0].Labels["def"] != "#Deployment" {
		t.Errorf("deployment labels = %+v", dr.Labels)
	}
}

func TestTimoniModuleRoot(t *testing.T) {
	for in, want := range map[string]string{
		"timoni.cue":                        ".",
		"templates/config.cue":              ".",
		"mod/timoni.cue":                    "mod",
		"mod/templates/config.cue":          "mod",
		"a/mod/templates/master/deploy.cue": "a/mod",
		"a/b/c.cue":                         "a/b",
	} {
		if got := TimoniModuleRoot(in); got != want {
			t.Errorf("TimoniModuleRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTimoniSyntaxErrorKeepsWhatParses: a file with a syntax error still
// yields the entities its parsed declarations carry, and no panic.
func TestTimoniSyntaxErrorKeepsWhatParses(t *testing.T) {
	src, _ := os.ReadFile(filepath.Join("dialects", "timoni", "testdata", "app", "syntax-error.cue"))
	r, _ := NewTimoniExtractor().ExtractDoc("svc", "svc/app/syntax-error.cue", src)
	_ = r
}

func TestTimoniDialectExtractorRegistered(t *testing.T) {
	e, ok := NativeDialectExtractor("timoni")
	if !ok {
		t.Fatal("NativeDialectExtractor(timoni) not registered")
	}
	if _, ok := e.(DocResulter); !ok {
		t.Error("the timoni extractor must be a DocResulter so its cross-file references reach the resolver")
	}
	if _, ok := TimoniDialectExtractor("cue"); ok {
		t.Error("only the timoni id is this file's")
	}
}
