package iac_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

var updateCatalog = flag.Bool("update-catalog", false, "rewrite the fixture catalog under testdata")

// fixtureCatalog is a catalog of a made-up provider, built from the exports below by the one writer fathom itself
// uses. It is committed so tests in other packages (the lang closure and no-dial tests, fathom-lang) read the same
// bytes; TestFixtureCatalogMatchesItsExports holds it to the exports.
const (
	fixtureCatalog = "testdata/registry.opentofu.org_example_widget.v1.iaccat.zip"
	fixtureSource  = "registry.opentofu.org/example/widget"
)

func attrSchema(extra map[string]any) map[string]any {
	a := map[string]any{"type": "string", "optional": true, "description": "dropped by the pruner", "description_kind": "plain"}
	for k, v := range extra {
		a[k] = v
	}
	return a
}

func fixtureExport(version string) []byte {
	attrs := map[string]any{
		"name":   attrSchema(map[string]any{"optional": false, "required": true}),
		"size":   attrSchema(map[string]any{"type": "number"}),
		"id":     attrSchema(map[string]any{"optional": false, "computed": true}),
		"secret": attrSchema(map[string]any{"sensitive": true}),
		"legacy": attrSchema(map[string]any{"deprecated": true}),
		"tags":   attrSchema(map[string]any{"type": []any{"map", "string"}}),
		"token":  attrSchema(map[string]any{"write_only": true}),
		"settings": map[string]any{"optional": true, "nested_type": map[string]any{
			"nesting_mode": "list",
			"attributes":   map[string]any{"mode": attrSchema(map[string]any{"required": true, "optional": false})},
		}},
	}
	if version == "1.1.0" {
		attrs["region"] = attrSchema(nil)
	}
	thing := map[string]any{"version": 1, "block": map[string]any{
		"attributes": attrs,
		"block_types": map[string]any{
			"rule": map[string]any{"nesting_mode": "list", "min_items": 1, "block": map[string]any{
				"attributes": map[string]any{"port": attrSchema(map[string]any{"type": "number", "required": true, "optional": false})},
			}},
		},
	}}
	dataThing := map[string]any{"version": 0, "block": map[string]any{"attributes": map[string]any{
		"name":  attrSchema(map[string]any{"optional": false, "required": true}),
		"found": attrSchema(map[string]any{"optional": false, "computed": true, "type": "bool"}),
	}}}
	onlyData := map[string]any{"version": 0, "block": map[string]any{"attributes": map[string]any{"id": attrSchema(map[string]any{"optional": false, "computed": true})}}}
	b, err := json.Marshal(map[string]any{"format_version": "1.0", "provider_schemas": map[string]any{fixtureSource: map[string]any{
		"provider":            map[string]any{"version": 0, "block": map[string]any{"attributes": map[string]any{"region": attrSchema(nil)}}},
		"resource_schemas":    map[string]any{"widget_thing": thing},
		"data_source_schemas": map[string]any{"widget_thing": dataThing, "widget_only_data": onlyData},
	}}})
	if err != nil {
		panic(err)
	}
	return b
}

func buildFixture(t testing.TB, out string) {
	t.Helper()
	dir := t.TempDir()
	w, err := iacfacts.NewIaCCatalogWriter(iacfacts.IaCWriterOptions{WorkDir: filepath.Join(dir, "work"), Tool: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, v := range []string{"1.0.0", "1.1.0"} {
		if _, err := w.AddExport(v, bytes.NewReader(fixtureExport(v))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.WriteFile(out); err != nil {
		t.Fatal(err)
	}
}

func openFixture(t testing.TB) *iac.Catalog {
	t.Helper()
	c, err := iac.OpenCatalog(fixtureCatalog)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func attrByPath(t testing.TB, ct *iac.Contract, path string) iac.ContractAttr {
	t.Helper()
	for _, a := range ct.Attributes {
		if a.Path == path {
			return a
		}
	}
	t.Fatalf("no attribute %q in %d attributes", path, len(ct.Attributes))
	return iac.ContractAttr{}
}

// The committed fixture is what the exports above build, compared by content (a zip's bytes can differ between Go
// versions' compressors).
func TestFixtureCatalogMatchesItsExports(t *testing.T) {
	if *updateCatalog {
		buildFixture(t, fixtureCatalog)
	}
	fresh := filepath.Join(t.TempDir(), "fresh.zip")
	buildFixture(t, fresh)
	a, err := iac.OpenCatalog(fixtureCatalog)
	if err != nil {
		t.Fatalf("%v (run with -update-catalog to write the fixture)", err)
	}
	b, err := iac.OpenCatalog(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Versions(), b.Versions()) || a.Source() != b.Source() {
		t.Fatalf("manifest differs: %v %s vs %v %s", a.Versions(), a.Source(), b.Versions(), b.Source())
	}
	for _, v := range a.Versions() {
		for _, typ := range []string{"widget_thing", "widget_only_data"} {
			x, errx := a.Contract(fixtureSource, v, typ)
			y, erry := b.Contract(fixtureSource, v, typ)
			if errx != nil || erry != nil {
				t.Fatalf("%s %s: %v / %v", v, typ, errx, erry)
			}
			x.Catalog, y.Catalog = "", ""
			if !reflect.DeepEqual(x, y) {
				t.Errorf("%s %s differs from a fresh build; run with -update-catalog", v, typ)
			}
		}
	}
	if _, err := iacfacts.VerifyIaCCatalog(fixtureCatalog); err != nil {
		t.Errorf("the fixture does not verify: %v", err)
	}
}

// IF-C5: OpenCatalog validates the manifest; Versions and Contract read it.
func TestOpenCatalogAndVersions(t *testing.T) {
	c := openFixture(t)
	if c.Source() != fixtureSource || c.Line() != "1" || !reflect.DeepEqual(c.Versions(), []string{"1.0.0", "1.1.0"}) {
		t.Errorf("source %q line %q versions %v", c.Source(), c.Line(), c.Versions())
	}
	if c.ReplacementFacts() != "absent" {
		t.Errorf("a catalog built from exports carries no replacement facts, got %q", c.ReplacementFacts())
	}
	vs := c.Versions()
	vs[0] = "mutated"
	if c.Versions()[0] != "1.0.0" {
		t.Error("Versions returned the catalog's own slice")
	}
}

func TestContractFacts(t *testing.T) {
	c := openFixture(t)
	ct, err := c.Contract(fixtureSource, "1.0.0", "widget_thing")
	if err != nil {
		t.Fatal(err)
	}
	if ct.Kind != iac.KindResource || ct.Type != "widget_thing" || ct.Version != "1.0.0" || ct.Source != fixtureSource ||
		ct.ReplacementFacts != "absent" || len(ct.Digest) != 64 || !strings.HasSuffix(ct.Catalog, ".iaccat.zip") {
		t.Errorf("header %+v", ct)
	}
	want := map[string]iac.ContractAttr{
		"name":             {Type: "string", Required: true},
		"size":             {Type: "number", Optional: true},
		"id":               {Type: "string", Computed: true},
		"secret":           {Type: "string", Optional: true, Sensitive: true},
		"legacy":           {Type: "string", Optional: true, Deprecated: true},
		"tags":             {Type: `["map","string"]`, Optional: true},
		"token":            {Type: "string", Optional: true, WriteOnly: true},
		"settings":         {Type: "nested_type", Optional: true},
		"settings[*].mode": {Type: "string", Required: true},
		"rule[*]":          {Block: true, Required: true, MinItems: 1},
		"rule[*].port":     {Type: "number", Required: true},
	}
	if len(ct.Attributes) != len(want) {
		t.Errorf("%d attributes, want %d", len(ct.Attributes), len(want))
	}
	for path, w := range want {
		g := attrByPath(t, ct, path)
		w.Path, w.JSONPath = path, g.JSONPath
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s = %+v, want %+v", path, g, w)
		}
		if !strings.HasPrefix(g.JSONPath, "block.") {
			t.Errorf("%s: JSON path %q", path, g.JSONPath)
		}
	}
	for i := 1; i < len(ct.Attributes); i++ {
		if ct.Attributes[i-1].Path >= ct.Attributes[i].Path {
			t.Fatalf("attributes are not sorted by path at %s", ct.Attributes[i].Path)
		}
	}
	// Facts cite where they came from: the JSON path leads to the entry in the blob.
	if got := attrByPath(t, ct, "rule[*].port").JSONPath; got != "block.block_types.rule.block.attributes.port" {
		t.Errorf("JSON path %q", got)
	}
	if got := attrByPath(t, ct, "settings[*].mode").JSONPath; got != "block.attributes.settings.nested_type.attributes.mode" {
		t.Errorf("JSON path %q", got)
	}
	// The release the catalog also holds differs by one attribute.
	next, err := c.Contract(fixtureSource, "1.1.0", "widget_thing")
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Attributes) != len(ct.Attributes)+1 || attrByPath(t, next, "region").Optional != true {
		t.Errorf("1.1.0 has %d attributes", len(next.Attributes))
	}
	if next.Digest == ct.Digest {
		t.Error("two different schemas share a digest")
	}
}

func TestContractKinds(t *testing.T) {
	c := openFixture(t)
	both, err := c.Contract(fixtureSource, "1.0.0", "widget_thing")
	if err != nil || both.Kind != iac.KindResource {
		t.Fatalf("a type that is a resource and a data source reads as the resource: %+v %v", both, err)
	}
	data, err := c.KindContract(fixtureSource, "1.0.0", iac.KindDataSource, "widget_thing")
	if err != nil || data.Kind != iac.KindDataSource || attrByPath(t, data, "found").Type != "bool" {
		t.Fatalf("data source: %+v %v", data, err)
	}
	only, err := c.Contract(fixtureSource, "1.0.0", "widget_only_data")
	if err != nil || only.Kind != iac.KindDataSource {
		t.Fatalf("a data-source-only type: %+v %v", only, err)
	}
	if _, err := c.KindContract(fixtureSource, "1.0.0", iac.KindEphemeral, "widget_thing"); !errors.Is(err, iac.ErrNotFound) {
		t.Errorf("an ephemeral lookup of a type with none: %v", err)
	}
	if _, err := c.KindContract(fixtureSource, "1.0.0", iac.Kind("nope"), "widget_thing"); err == nil || errors.Is(err, iac.ErrNotFound) {
		t.Errorf("an unknown kind must be a plain error: %v", err)
	}
}

func TestContractNotFound(t *testing.T) {
	c := openFixture(t)
	for name, call := range map[string]func() error{
		"missing type":    func() error { _, err := c.Contract(fixtureSource, "1.0.0", "widget_missing"); return err },
		"missing version": func() error { _, err := c.Contract(fixtureSource, "1.0.1", "widget_thing"); return err },
		"nearest version": func() error { _, err := c.Contract(fixtureSource, "1.0", "widget_thing"); return err },
		"other provider": func() error {
			_, err := c.Contract("registry.opentofu.org/example/gadget", "1.0.0", "widget_thing")
			return err
		},
		"other host": func() error {
			_, err := c.Contract("registry.example.com/example/widget", "1.0.0", "widget_thing")
			return err
		},
		"empty source": func() error { _, err := c.Contract("", "1.0.0", "widget_thing"); return err },
	} {
		err := call()
		var ce *iac.CatalogError
		if !errors.Is(err, iac.ErrNotFound) || !errors.As(err, &ce) || ce.Code != "not-found" || ce.Scope != "answer" {
			t.Errorf("%s: want a not-found *CatalogError, got %v", name, err)
		}
	}
	if _, err := c.Contract(fixtureSource, "1.0.0", ""); err == nil || errors.Is(err, iac.ErrNotFound) {
		t.Errorf("an empty type is misuse, not a missing type: %v", err)
	}
}

// A source with no host is on the default registry, and the two default registries stand in for each other.
func TestContractSourceSpellings(t *testing.T) {
	c := openFixture(t)
	for _, src := range []string{fixtureSource, strings.ToUpper(fixtureSource), "registry.terraform.io/example/widget", "example/widget"} {
		if _, err := c.Contract(src, "1.0.0", "widget_thing"); err != nil {
			t.Errorf("source %q: %v", src, err)
		}
	}
}

func TestOpenCatalogRefuses(t *testing.T) {
	dir := t.TempDir()
	if _, err := iac.OpenCatalog(filepath.Join(dir, "missing.zip")); err == nil {
		t.Error("a missing file must be an error")
	}
	junk := filepath.Join(dir, "junk.zip")
	if err := os.WriteFile(junk, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := iac.OpenCatalog(junk); err == nil {
		t.Error("a file that is no zip must be an error")
	}
	// A zip with a manifest of another format is refused as such.
	bad := rawCatalog(t, dir, "badformat.zip", rawOpts{format: 2})
	var ce *iac.CatalogError
	if _, err := iac.OpenCatalog(bad); !errors.As(err, &ce) || ce.Code != "format" || ce.Scope != "catalog" {
		t.Errorf("a manifest of another format: %v", err)
	}
	// Versions out of order are refused at open.
	unsorted := rawCatalog(t, dir, "unsorted.zip", rawOpts{versions: []string{"1.1.0", "1.0.0"}})
	if _, err := iac.OpenCatalog(unsorted); !errors.As(err, &ce) || ce.Code != "corrupt" {
		t.Errorf("unsorted versions: %v", err)
	}
}

type rawOpts struct {
	format   int
	versions []string
	// blob is the schema blob of widget_thing; "" is a minimal one.
	blob string
}

// rawCatalog hand-rolls a catalog zip, so a test can hold what the writer would prune (a replacement fact).
func rawCatalog(t testing.TB, dir, name string, o rawOpts) string {
	t.Helper()
	if o.format == 0 {
		o.format = 1
	}
	if o.versions == nil {
		o.versions = []string{"1.0.0"}
	}
	if o.blob == "" {
		o.blob = `{"version":1,"block":{"attributes":{"name":{"type":"string","required":true}}}}`
	}
	sum := sha256.Sum256([]byte(o.blob))
	digest := hex.EncodeToString(sum[:])
	files := map[string]string{}
	inputs := map[string]string{}
	for _, v := range o.versions {
		files["versions/"+v+".json"] = fmt.Sprintf(`{"resources":{"widget_thing":%q},"data_sources":{},"ephemeral_resources":{}}`, digest)
		inputs[v] = digest
	}
	files["blobs/"+digest] = o.blob
	m, _ := json.Marshal(map[string]any{"format": o.format, "kind": "terraform-provider", "source": fixtureSource, "line": "1",
		"versions": o.versions, "inputs": inputs, "replacement": "present"})
	files["manifest.json"] = string(m)
	path := filepath.Join(dir, name)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range []string{"manifest.json"} {
		w, _ := zw.Create(n)
		w.Write([]byte(files[n]))
		delete(files, n)
	}
	for n, c := range files {
		w, _ := zw.Create(n)
		w.Write([]byte(c))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A replacement fact in a blob reads with its provenance. No catalog carries one before IF-13, so this holds the
// shape the door expects, from a hand-built blob.
func TestContractReplacementFacts(t *testing.T) {
	blob := `{"version":1,"block":{"attributes":{"name":{"type":"string","required":true,"replacement":{"class":"replace","provenance":"registry.opentofu.org/example/widget@v1.0.0"}},"size":{"type":"number","optional":true}}}}`
	path := rawCatalog(t, t.TempDir(), "r.zip", rawOpts{blob: blob})
	c, err := iac.OpenCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.ReplacementFacts() != "present" {
		t.Errorf("manifest claim %q", c.ReplacementFacts())
	}
	ct, err := c.Contract(fixtureSource, "1.0.0", "widget_thing")
	if err != nil {
		t.Fatal(err)
	}
	if r := attrByPath(t, ct, "name").Replacement; r == nil || r.Class != "replace" || r.Provenance != "registry.opentofu.org/example/widget@v1.0.0" {
		t.Errorf("name replacement %+v", r)
	}
	if r := attrByPath(t, ct, "size").Replacement; r != nil {
		t.Errorf("size states no replacement, got %+v", r)
	}
}

func TestContractRefusesAMalformedBlob(t *testing.T) {
	path := rawCatalog(t, t.TempDir(), "m.zip", rawOpts{blob: `{"version":1,"block":"not an object"}`})
	c, err := iac.OpenCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	var ce *iac.CatalogError
	if _, err := c.Contract(fixtureSource, "1.0.0", "widget_thing"); !errors.As(err, &ce) || ce.Code != "corrupt" {
		t.Errorf("a blob this build cannot read: %v", err)
	}
}

// Lookups hold nothing between calls: a catalog deleted after OpenCatalog answers no more, and concurrent lookups
// agree.
func TestLookupsOpenPerCall(t *testing.T) {
	dir := t.TempDir()
	path := rawCatalog(t, dir, "c.zip", rawOpts{})
	c, err := iac.OpenCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make([]string, 8)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ct, err := c.Contract(fixtureSource, "1.0.0", "widget_thing")
			if err == nil {
				b, _ := json.Marshal(ct)
				results[i] = string(b)
			}
		}()
	}
	wg.Wait()
	for _, r := range results {
		if r == "" || r != results[0] {
			t.Fatalf("concurrent lookups differ: %q", results)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Contract(fixtureSource, "1.0.0", "widget_thing"); err == nil {
		t.Error("a deleted catalog still answered: a lookup keeps a handle or a memo")
	}
	if c.Source() != fixtureSource || len(c.Versions()) != 1 {
		t.Error("the manifest is validated at open and kept")
	}
	_ = context.Background
}
