package fusion

// vuesvelte_ingest_test.go — LB-F536, LB-F537 (wave lf7, item vuesvelte):
// an ingest reads a Vue single-file component and a Svelte component through
// their dialects (dispatched by the manifest loop, no register.go line), each
// file one module symbol keyed by its base name with its extension (F1), its
// <script>'s definitions and imports its own (F2), and its template's
// components calls from that module symbol; the file resolver binds a
// template's component to the imported component file's module symbol.

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func componentCorpus() map[string]string {
	return map[string]string{
		"src/components/Button.vue": "<script setup lang=\"ts\">\nimport { buttonProps } from './button'\nimport Icon from './Icon.vue'\nconst props = defineProps(buttonProps)\n</script>\n\n<template>\n  <button><Icon /><slot /></button>\n</template>\n",
		"src/components/button.ts":  "export const buttonProps = { size: String }\nexport function useButton() {\n  return 1\n}\n",
		"src/components/Icon.vue":   "<template>\n  <i class=\"icon\" />\n</template>\n",
		"src/App.vue":               "<script setup lang=\"ts\">\nimport ElButton from './components/Button.vue'\nimport { track } from './util'\ntrack('app')\n</script>\n\n<template>\n  <ElButton />\n  <el-button />\n  <router-view />\n</template>\n\n<style>\n.app { color: red; }\n</style>\n",
		"src/util.ts":               "export function track(name: string) {\n  return name\n}\n",
		"web/routes/+page.svelte":   "<script lang=\"ts\">\n  import Card from '../lib/Card.svelte';\n  let { data } = $props();\n</script>\n\n<Card title={data.title} />\n",
		"web/routes/+page.ts":       "export function load() {\n  return { title: 'x' }\n}\n",
		"web/lib/Card.svelte":       "<script lang=\"ts\">\n  let { title } = $props();\n</script>\n\n<h1>{title}</h1>\n",
	}
}

func TestIngest_VueAndSvelteComponents(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, componentCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "web", Path: dir}
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	var all []string
	for _, o := range obs {
		k := o.SourceRef + "|" + string(o.SourceKind) + "|" + o.Symbol
		got[k] = true
		all = append(all, k)
	}
	slices.Sort(all)
	for _, w := range []string{
		"src/App.vue|VueComponent|<module>",
		"src/components/Button.vue|VueComponent|<module>",
		"src/components/Button.vue|JSConst|props",
		"src/components/Icon.vue|VueComponent|<module>",
		"src/components/button.ts|JSFunc|useButton",
		"src/App.vue|CSSSelector|.app",
		"web/routes/+page.svelte|SvelteComponent|<module>",
		"web/lib/Card.svelte|SvelteComponent|<module>",
		"web/routes/+page.ts|JSFunc|load",
	} {
		if !got[w] {
			t.Errorf("%s not stored; have %q", w, all)
		}
	}

	// A component and a script beside it with the same stem are two modules:
	// Button.vue's module symbol is keyed by its base name with the extension
	// (F1), never button.ts's, and +page.svelte's never +page.ts's.
	keys := map[string]string{}
	var appCalls []string
	for _, o := range obs {
		if o.Symbol != core.ModuleSymbolName {
			continue
		}
		k := o.Package + "." + o.Symbol
		if prev, dup := keys[k]; dup && prev != o.SourceRef {
			t.Errorf("%s and %s share the module key %s", prev, o.SourceRef, k)
		}
		keys[k] = o.SourceRef
		if o.SourceRef == "src/App.vue" {
			appCalls = o.Calls
		}
	}
	// The template's components (named as Vue resolves them) and the script's
	// top-level call are the component's calls (F2).
	for _, c := range []string{"ElButton", "RouterView", "track"} {
		if !slices.ContainsFunc(appCalls, func(x string) bool { return x == c || strings.HasSuffix(x, "."+c) }) {
			t.Errorf("App.vue's module calls = %q, want %s among them", appCalls, c)
		}
	}

	// The file resolver binds the template's <ElButton /> through the
	// script's default import to Button.vue's module symbol, and the
	// Svelte route's <Card /> to Card.svelte's.
	edges, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	fileEdges := map[string]bool{}
	var have []string
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			k := e.FromSymbol + " -> " + e.ToSymbol
			fileEdges[k] = true
			have = append(have, k)
		}
	}
	for _, w := range [][2]string{
		{"web::src/App.vue.<module>", "web::src/components/Button.vue.<module>"},
		{"web::web/routes/+page.svelte.<module>", "web::web/lib/Card.svelte.<module>"},
	} {
		if !fileEdges[w[0]+" -> "+w[1]] {
			t.Errorf("no file-resolved edge %s -> %s; have %q", w[0], w[1], have)
		}
	}
	for k := range fileEdges {
		if strings.Contains(k, "Icon.vue") && strings.HasPrefix(k, "web::src/App.vue") {
			t.Errorf("App.vue imports no Icon: %s", k)
		}
	}
}
