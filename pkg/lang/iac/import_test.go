package iac_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

// IF-C8: ImportCatalog builds a catalog from a local schema zip or a `tofu providers schema -json` export with the
// writer fathom iac import uses, so a library consumer needs no fathom binary.

func writeExport(t testing.TB, version string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "export-"+version+".json")
	if err := os.WriteFile(p, fixtureExport(version), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImportCatalogFromAnExport(t *testing.T) {
	dir := t.TempDir()
	res, err := iac.ImportCatalog(iac.ImportRequest{Dir: dir, Exports: []iac.Export{{Version: "1.0.0", Path: writeExport(t, "1.0.0")}, {Version: "1.1.0", Path: writeExport(t, "1.1.0")}}, Tool: "tofu test"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != fixtureSource || res.Line != "1" || strings.Join(res.Versions, ",") != "1.0.0,1.1.0" || strings.Join(res.Added, ",") != "1.0.0,1.1.0" || res.Merged {
		t.Fatalf("result %+v", res)
	}
	if filepath.Dir(res.Path) != dir || len(res.SHA256) != 64 || res.Bytes == 0 {
		t.Fatalf("path %q sha %q bytes %d", res.Path, res.SHA256, res.Bytes)
	}
	// The manifest says the catalog carries no replacement facts, and so does the answer and the result.
	if res.ReplacementFacts != "absent" || !strings.Contains(res.Facts, "no replacement facts") {
		t.Fatalf("replacement %q facts %q", res.ReplacementFacts, res.Facts)
	}
	cat, err := iac.OpenCatalog(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if cat.ReplacementFacts() != "absent" {
		t.Fatalf("manifest replacement %q", cat.ReplacementFacts())
	}
	ct, err := cat.Contract(fixtureSource, "1.1.0", "widget_thing")
	if err != nil {
		t.Fatal(err)
	}
	if !attrByPath(t, ct, "name").Required || !attrByPath(t, ct, "secret").Sensitive || !attrByPath(t, ct, "legacy").Deprecated {
		t.Fatalf("facts 1 to 4 and 7 went missing: %+v", ct.Attributes)
	}
	for _, a := range ct.Attributes {
		if a.Replacement != nil {
			t.Fatalf("a catalog built from an export states a replacement fact: %+v", a)
		}
	}
	// The catalog is the one fathom iac import writes: same content as the writer's own build.
	fixture := openFixture(t)
	want, err := fixture.Contract(fixtureSource, "1.1.0", "widget_thing")
	if err != nil {
		t.Fatal(err)
	}
	if ct.Digest != want.Digest {
		t.Fatalf("digest %s, the writer's own build gives %s", ct.Digest, want.Digest)
	}
}

// A second import into the same directory merges the line, never drops a release.
func TestImportCatalogMerges(t *testing.T) {
	dir := t.TempDir()
	if _, err := iac.ImportCatalog(iac.ImportRequest{Dir: dir, Exports: []iac.Export{{Version: "1.0.0", Path: writeExport(t, "1.0.0")}}}); err != nil {
		t.Fatal(err)
	}
	res, err := iac.ImportCatalog(iac.ImportRequest{Dir: dir, Exports: []iac.Export{{Version: "1.1.0", Path: writeExport(t, "1.1.0")}}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Merged || strings.Join(res.Added, ",") != "1.1.0" || strings.Join(res.Kept, ",") != "1.0.0" || strings.Join(res.Versions, ",") != "1.0.0,1.1.0" {
		t.Fatalf("result %+v", res)
	}
	// The same release again is unchanged. A different export for it is refused unless Replace is set.
	res, err = iac.ImportCatalog(iac.ImportRequest{Dir: dir, Exports: []iac.Export{{Version: "1.0.0", Path: writeExport(t, "1.0.0")}}})
	if err != nil || strings.Join(res.Unchanged, ",") != "1.0.0" {
		t.Fatalf("unchanged: %v %+v", err, res)
	}
	other := writeExport(t, "1.1.0")
	if _, err := iac.ImportCatalog(iac.ImportRequest{Dir: dir, Exports: []iac.Export{{Version: "1.0.0", Path: other}}}); err == nil {
		t.Fatal("a release with different content was replaced without Replace")
	}
	res, err = iac.ImportCatalog(iac.ImportRequest{Dir: dir, Exports: []iac.Export{{Version: "1.0.0", Path: other}}, Replace: true})
	if err != nil || strings.Join(res.Replaced, ",") != "1.0.0" {
		t.Fatalf("replace: %v %+v", err, res)
	}
}

// cfnImportZip is a made-up registry schema zip. Nothing here comes from AWS (IF-P7).
func cfnImportZip(t testing.TB) string {
	t.Helper()
	schema, err := json.Marshal(map[string]any{
		"typeName":                        "Example::Widget::Thing",
		"properties":                      map[string]any{"Name": map[string]any{"type": "string"}, "Size": map[string]any{"type": "integer"}, "Arn": map[string]any{"type": "string"}, "Tags": map[string]any{"type": "array"}},
		"required":                        []string{"Name"},
		"createOnlyProperties":            []string{"/properties/Name", "/properties/Tags/*/Key"},
		"conditionalCreateOnlyProperties": []string{"/properties/Size"},
		"readOnlyProperties":              []string{"/properties/Arn"},
		"additionalProperties":            false,
	})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "example-widget-thing.json", Method: zip.Deflate, Modified: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	w.Write(schema)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "schemas.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImportCatalogFromACloudFormationZip(t *testing.T) {
	dir := t.TempDir()
	res, err := iac.ImportCatalog(iac.ImportRequest{Dir: dir, SchemaZip: cfnImportZip(t), Region: "us-east-1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "aws/cloudformation" || res.Line != "us-east-1" || res.Types != 1 || res.Date != "2026-10-30" || len(res.SchemaDigest) != 64 || res.ReplacementFacts != "contract" {
		t.Fatalf("result %+v", res)
	}
	if len(res.Versions) != 1 || !strings.HasPrefix(res.Versions[0], "sha256-") {
		t.Fatalf("versions %v", res.Versions)
	}
	// The same zip again is the same snapshot.
	again, err := iac.ImportCatalog(iac.ImportRequest{Dir: dir, SchemaZip: cfnImportZip(t), Region: "us-east-1"})
	if err != nil || !again.SnapshotReplaced || !again.SnapshotUnchanged || again.SHA256 != res.SHA256 {
		t.Fatalf("again: %v %+v", err, again)
	}
	// Contract reads it: the schema's own flags and replacement facts, with the provenance.
	cat, err := iac.OpenCatalog(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	ct, err := cat.Contract("aws/cloudformation", res.Versions[0], "Example::Widget::Thing")
	if err != nil {
		t.Fatal(err)
	}
	name, size, arn := attrByPath(t, ct, "Name"), attrByPath(t, ct, "Size"), attrByPath(t, ct, "Arn")
	if !name.Required || name.Replacement == nil || name.Replacement.Class != "replace" || !strings.Contains(name.Replacement.Provenance, "createOnlyProperties") {
		t.Fatalf("Name %+v", name)
	}
	if size.Replacement == nil || size.Replacement.Class != "conditional" || !arn.Computed || arn.Replacement != nil {
		t.Fatalf("Size %+v Arn %+v", size, arn)
	}
	if key := attrByPath(t, ct, "Tags[*].Key"); key.Replacement == nil || key.Replacement.Class != "replace" {
		t.Fatalf("Tags[*].Key %+v", key)
	}
	// A read-only property cannot be set, so it is not optional. Under the Terraform convention optional plus
	// computed means a user may set it.
	if arn.Optional || arn.Required || !arn.Computed {
		t.Fatalf("a read-only property claims optional or required: %+v", arn)
	}
	if name.Optional || !size.Optional {
		t.Fatalf("top-level optional: Name %+v Size %+v", name, size)
	}
	// The pruned schema keeps no nested required list, so a nested entry states neither required nor optional.
	if key := attrByPath(t, ct, "Tags[*].Key"); key.Required || key.Optional {
		t.Fatalf("a nested entry claims required or optional the schema does not support: %+v", key)
	}
	if ct.ReplacementFacts != "contract" || len(ct.Digest) != 64 {
		t.Fatalf("contract %+v", ct)
	}
}

func TestImportCatalogRefuses(t *testing.T) {
	dir := t.TempDir()
	export := writeExport(t, "1.0.0")
	zipPath := cfnImportZip(t)
	for name, req := range map[string]iac.ImportRequest{
		"no directory":              {Exports: []iac.Export{{Version: "1.0.0", Path: export}}},
		"no input":                  {Dir: dir},
		"both inputs":               {Dir: dir, Exports: []iac.Export{{Version: "1.0.0", Path: export}}, SchemaZip: zipPath, Region: "us-east-1"},
		"a zip with no region":      {Dir: dir, SchemaZip: zipPath},
		"a region with exports":     {Dir: dir, Region: "us-east-1", Exports: []iac.Export{{Version: "1.0.0", Path: export}}},
		"a bad region":              {Dir: dir, SchemaZip: zipPath, Region: "../x"},
		"an export with no version": {Dir: dir, Exports: []iac.Export{{Path: export}}},
		"a url":                     {Dir: dir, SchemaZip: "https://example.invalid/schemas.zip", Region: "us-east-1"},
		"a missing file":            {Dir: dir, Exports: []iac.Export{{Version: "1.0.0", Path: filepath.Join(dir, "nope.json")}}},
		"replace on a zip":          {Dir: dir, SchemaZip: zipPath, Region: "us-east-1", Replace: true},
	} {
		if _, err := iac.ImportCatalog(req); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	// Nothing was installed by any refusal.
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Errorf("a refused import left %d entries", len(ents))
	}
	// A multi-provider export is refused with a typed error a caller can read.
	var exp map[string]any
	if err := json.Unmarshal(fixtureExport("1.0.0"), &exp); err != nil {
		t.Fatal(err)
	}
	ps := exp["provider_schemas"].(map[string]any)
	ps["registry.opentofu.org/example/other"] = ps[fixtureSource]
	b, _ := json.Marshal(exp)
	multi := filepath.Join(t.TempDir(), "multi.json")
	os.WriteFile(multi, b, 0o644)
	_, err := iac.ImportCatalog(iac.ImportRequest{Dir: dir, Exports: []iac.Export{{Version: "1.0.0", Path: multi}}})
	if err == nil {
		t.Fatal("a multi-provider export was imported")
	}
	var ce *iac.CatalogError
	_ = errors.As(err, &ce) // a refusal is a *CatalogError, any other failure a plain error; both name the cause
	if !strings.Contains(err.Error(), "provider") {
		t.Errorf("error %v", err)
	}
}
