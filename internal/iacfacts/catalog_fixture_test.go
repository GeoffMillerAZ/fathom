package iacfacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The fixtures here are generated, never a committed provider export. fxSource is a made-up provider.
const fxSource = "registry.opentofu.org/example/widget"

func fxAttr(extra map[string]any) map[string]any {
	a := map[string]any{"type": "string", "optional": true, "description": "drop me", "description_kind": "plain"}
	for k, v := range extra {
		a[k] = v
	}
	return a
}

// fxResource is a {version, block} schema with the named attributes and one nested block.
func fxResource(attrs ...string) map[string]any {
	as := map[string]any{"description": fxAttr(nil)} // an attribute named "description" must survive pruning
	for _, n := range attrs {
		as[n] = fxAttr(nil)
	}
	as["secret"] = fxAttr(map[string]any{"sensitive": true})
	return map[string]any{"version": 1, "block": map[string]any{
		"attributes":       as,
		"description":      "drop me too",
		"description_kind": "plain",
		"block_types": map[string]any{
			"rule": map[string]any{"nesting_mode": "list", "block": map[string]any{
				"attributes": map[string]any{"port": fxAttr(map[string]any{"type": "number", "required": true})},
			}},
		},
	}}
}

// fxBig is a schema whose pruned form is at least want bytes, built from many small attributes.
func fxBig(t testing.TB, want int) map[string]any {
	t.Helper()
	as := map[string]any{}
	size := 0
	for i := 0; size < want; i++ {
		name := fmt.Sprintf("attribute_%06d", i)
		a := map[string]any{"type": []any{"list", []any{"object", map[string]any{"name": "string", "value": "string"}}}, "optional": true, "computed": true}
		as[name] = a
		size += len(name) + 100
	}
	return map[string]any{"version": 0, "block": map[string]any{"attributes": as}}
}

func fxExport(t testing.TB, source string, provider map[string]any, resources, data map[string]any) []byte {
	t.Helper()
	ps := map[string]any{"provider": provider, "resource_schemas": resources, "data_source_schemas": data}
	b, err := json.Marshal(map[string]any{"format_version": "1.0", "provider_schemas": map[string]any{source: ps}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fxProvider() map[string]any {
	return map[string]any{"version": 0, "block": map[string]any{"attributes": map[string]any{"region": fxAttr(nil)}}}
}

// fxCatalog writes a catalog of the given versions, each built from the export its function returns, and returns
// its path.
func fxCatalog(t testing.TB, dir string, lim IaCLimits, exports map[string][]byte) string {
	t.Helper()
	w, err := NewIaCCatalogWriter(IaCWriterOptions{WorkDir: filepath.Join(dir, "work"), Limits: lim, Tool: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for v, b := range exports {
		f := filepath.Join(dir, "export-"+v+".json")
		if err := os.WriteFile(f, b, 0o644); err != nil {
			t.Fatal(err)
		}
		r, _ := os.Open(f)
		_, err := w.AddExport(v, r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, "cat.iaccat.zip")
	if _, err := w.WriteFile(out); err != nil {
		t.Fatal(err)
	}
	return out
}
