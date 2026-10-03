package fusion

// timonidispatch_test.go — LB-F216: the native Timoni dialect is reached by
// ingest (a timoni.cue, a bundle or runtime by content, a module's own files
// under a timoni.cue), ReingestFile decides as ingest does, the module's
// cue.mod/ dependencies and plain CUE stay plain, and an ingest stores the
// dialect's cross-file references (LB-F218) as doc-xref edges: a bundle
// instance to its local module's timoni.cue, #Instance to the object templates
// it renders.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func timoniCorpus() map[string]string {
	return map[string]string{
		"mods/app/cue.mod/module.cue":                       "module: \"timoni.sh/app\"\nlanguage: version: \"v0.17.1\"\n",
		"mods/app/cue.mod/gen/k8s.io/api/apps/v1/types.cue": "package v1\n\n#Deployment: {\n\tkind: \"Deployment\"\n}\n",
		"mods/app/timoni.cue": `package main

import (
	templates "timoni.sh/app/templates"
)

values: templates.#Config

timoni: {
	apiVersion: "v1alpha1"
	instance: templates.#Instance & {
		config: values
	}
	apply: app: [for obj in instance.objects {obj}]
}
`,
		"mods/app/values.cue": "@if(!debug)\n\npackage main\n\nvalues: {\n\tmessage: \"hi\"\n}\n",
		"mods/app/images.cue": "package main\n\nvalues: {\n\timage: tag: *\"1\" | string\n}\n",
		"mods/app/templates/config.cue": `package templates

#Config: {
	message!: string
	image: tag: string
}

#Instance: {
	config: #Config
	objects: {
		deploy: #Deployment & {#config: config}
	}
}
`,
		"mods/app/templates/deployment.cue": `package templates

import (
	appsv1 "k8s.io/api/apps/v1"
)

#Deployment: appsv1.#Deployment & {
	#config:    #Config
	apiVersion: "apps/v1"
	kind:       "Deployment"
}
`,
		"bundles/bundle.cue": `bundle: {
	apiVersion: "v1alpha1"
	name:       "demo"
	instances: {
		web: {
			module: url: "file://../mods/app"
			namespace: "demo"
		}
		cache: {
			module: url: "oci://registry.example.test/modules/cache"
			namespace: "demo"
		}
	}
}
`,
		"schema/schema.cue":     "package schema\n\n#Thing: {a: int}\n",
		"schema/other.cue":      "package schema\n\n#Other: {b: string}\n",
		"other/values.cue":      "package other\n\nvalues: {\n\ty: 2\n}\n",
		"other/templates/t.cue": "package templates\n\n#T: {kind: \"Pod\"}\n",
	}
}

func TestTimoniDispatchClaims(t *testing.T) {
	refs := []string{"mods/app/timoni.cue", "mods/app/cue.mod/module.cue"}
	idx := &siblingIndex{timoniDirs: map[string]struct{}{"mods/app": {}}}
	_ = refs
	src := timoniCorpus()
	for rel, want := range map[string]bool{
		"mods/app/timoni.cue":                               true,
		"mods/app/values.cue":                               true, // a module's own file, by the timoni.cue above it
		"mods/app/images.cue":                               true,
		"mods/app/templates/config.cue":                     true,
		"mods/app/templates/deployment.cue":                 true,
		"mods/app/cue.mod/gen/k8s.io/api/apps/v1/types.cue": false, // a dependency, not the module's file
		"mods/app/cue.mod/module.cue":                       false,
		"bundles/bundle.cue":                                true, // by content
		"schema/schema.cue":                                 false,
		"other/values.cue":                                  false, // outside any module
		"other/templates/t.cue":                             false, // templates/ alone is no module
	} {
		_, tk, ok := timoniDispatch(rel, []byte(src[rel]), idx)
		if ok != want || (ok && tk != ToolTimoni) {
			t.Errorf("timoniDispatch(%q) = %q, %v; want claimed=%v", rel, tk, ok, want)
		}
	}
	// A timoni.cue claims itself without an index, and a runtime by content.
	if _, _, ok := timoniDispatch("x/timoni.cue", []byte("package main\n"), nil); !ok {
		t.Error("timoni.cue without an index is not claimed")
	}
	if _, _, ok := timoniDispatch("x/rt.cue", []byte("runtime: {\n\tapiVersion: \"v1alpha1\"\n}\n"), nil); !ok {
		t.Error("a runtime by content is not claimed")
	}
	if _, _, ok := timoniDispatch("x/rt.cue", []byte("runtime: {\n\tmode: \"fast\"\n}\n"), nil); ok {
		t.Error("a runtime: field with no v1alpha1 apiVersion is not a Timoni runtime")
	}
	if _, _, ok := timoniDispatch("a/b.yaml", []byte("bundle: {\n"), nil); ok {
		t.Error("only .cue files are claimed")
	}
}

// TestResolveExtractor_Timoni: the dispatch is product-reached, through
// resolveExtractor, the function ingest and ExtractForBench both call.
func TestResolveExtractor_Timoni(t *testing.T) {
	p := New(nil, nil, Config{})
	defer p.Close()
	idx := &siblingIndex{timoniDirs: map[string]struct{}{"mods/app": {}}}
	src := timoniCorpus()
	e, tk, ok, _ := p.resolveExtractor("mods/app/templates/deployment.cue", []byte(src["mods/app/templates/deployment.cue"]), idx)
	if !ok || tk != ToolTimoni {
		t.Fatalf("resolveExtractor = %v, %q, %v", e, tk, ok)
	}
	if _, isTimoni := e.(*extractor.TimoniExtractor); !isTimoni {
		t.Errorf("resolveExtractor chose %T, want the Timoni extractor", e)
	}
	e, tk, _, _ = p.resolveExtractor("schema/schema.cue", []byte(src["schema/schema.cue"]), idx)
	if _, isTimoni := e.(*extractor.TimoniExtractor); isTimoni || tk == ToolTimoni {
		t.Errorf("plain CUE resolved to %T, %q", e, tk)
	}
}

// TestIngest_TimoniDialect: ingest extracts a module's files, a bundle and the
// object templates as Timoni entities, leaves the vendored dependency and plain
// CUE plain, and stores the bundle-to-module and instance-to-object edges.
func TestIngest_TimoniDialect(t *testing.T) {
	p, st, dir, rep := ingestStored(t, "tm", timoniCorpus())
	ctx := context.Background()
	if rep.DocXrefEdges == 0 {
		t.Fatal("no doc-xref edge stored")
	}
	obs, err := st.QueryObservations(ctx, "tm", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
	}
	for key, kind := range map[string]string{
		"mods/app/timoni.cue|timoni/module":                           "TimoniModule",
		"mods/app/timoni.cue|timoni":                                  "CUEField",
		"mods/app/values.cue|timoni/values":                           "TimoniValues",
		"mods/app/images.cue|timoni/values":                           "TimoniValues",
		"mods/app/templates/deployment.cue|timoni/object/#Deployment": "TimoniObject",
		"bundles/bundle.cue|timoni/bundle":                            "TimoniBundle",
		"bundles/bundle.cue|timoni/bundle/instance/web":               "TimoniBundleInstance",
		"schema/schema.cue|#Thing":                                    "CUEDefinition",
		"schema/other.cue|#Other":                                     "CUEDefinition", // a .cue no Timoni signal claims is read as before, never gated out
		"other/values.cue|values":                                     "CUEField",
	} {
		if got[key] != kind {
			t.Errorf("%s = %q, want %q", key, got[key], kind)
		}
	}
	for key, kind := range got {
		plain := strings.HasPrefix(key, "mods/app/cue.mod/") || strings.HasPrefix(key, "schema/") || strings.HasPrefix(key, "other/")
		if plain && strings.HasPrefix(kind, "Timoni") {
			t.Errorf("%s: plain CUE read as %s", key, kind)
		}
	}
	edges := storedEdges(t, st, "tm")
	for _, want := range []string{
		"bundles/bundle.cue|tm::bundles/bundle.timoni/bundle/instance/web->tm::mods/app/main.timoni/module",
		"mods/app/templates/config.cue|tm::mods/app/templates/templates.#Instance->tm::mods/app/templates/templates.timoni/object/#Deployment",
	} {
		if !containsString(edges, want) {
			t.Errorf("stored edges lack %q (stored: %v)", want, edges)
		}
	}
	for _, e := range edges {
		if len(e) > 0 && strings.Contains(e, "instance/cache") {
			t.Errorf("an oci:// module resolved to a repository file: %s", e)
		}
	}

	// ReingestFile decides as ingest does: an edited template stays Timoni's.
	rel := "mods/app/templates/deployment.cue"
	body := "package templates\n\n#Deployment: {\n\tkind: \"StatefulSet\"\n}\n"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "tm", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err = st.QueryObservations(ctx, "tm", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, o := range obs {
		if o.SourceRef == rel && o.Symbol == "timoni/object/#Deployment" && string(o.SourceKind) == "TimoniObject" {
			found = true
		}
	}
	if !found {
		t.Error("after ReingestFile: the template is no longer a Timoni object")
	}
}
