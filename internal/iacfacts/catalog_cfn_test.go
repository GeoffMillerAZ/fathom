package iacfacts

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cfnFixtureType is a made-up registry schema for a made-up service. Nothing here comes from AWS (IF-P7).
func cfnFixtureType(name string, extra map[string]any) []byte {
	s := map[string]any{
		"typeName":    name,
		"description": "prose that a catalog must not carry",
		"properties": map[string]any{
			"Name":    map[string]any{"type": "string", "description": "more prose"},
			"Size":    map[string]any{"type": "integer"},
			"Arn":     map[string]any{"type": "string"},
			"OldKnob": map[string]any{"type": "boolean"},
			"Secret":  map[string]any{"type": "string"},
			"Tags":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
		},
		"required":                        []string{"Name", "Size"},
		"createOnlyProperties":            []string{"/properties/Name"},
		"conditionalCreateOnlyProperties": []string{"/properties/Size"},
		"readOnlyProperties":              []string{"/properties/Arn"},
		"writeOnlyProperties":             []string{"/properties/Secret"},
		"deprecatedProperties":            []string{"/properties/OldKnob"},
		"primaryIdentifier":               []string{"/properties/Arn"},
		"additionalProperties":            false,
	}
	for k, v := range extra {
		s[k] = v
	}
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return b
}

// cfnZip writes a registry-style zip. Every entry carries the given build time.
func cfnZip(t testing.TB, mod time.Time, order []string, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range order {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Deflate, Modified: mod})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(files[n]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func cfnFixtureZip(t testing.TB, mod time.Time) []byte {
	return cfnZip(t, mod, []string{"example-widget-thing.json", "example-widget-other.json", "notes.txt"}, map[string][]byte{
		"example-widget-thing.json": cfnFixtureType("Example::Widget::Thing", nil),
		"example-widget-other.json": cfnFixtureType("Example::Widget::Other", map[string]any{"required": []string{"Name"}}),
		"notes.txt":                 []byte("not a schema"),
	})
}

func writeFile(t testing.TB, path string, b []byte) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func importCFN(t testing.TB, dir string, zipBytes []byte, region string) CFNImportResult {
	t.Helper()
	src := writeFile(t, filepath.Join(t.TempDir(), "schemas.zip"), zipBytes)
	res, err := ImportCloudFormationSchema(dir, src, region, IaCWriterOptions{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	return res
}

func TestBuildCloudFormationCatalogFacts(t *testing.T) {
	dir := t.TempDir()
	mod := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	res := importCFN(t, dir, cfnFixtureZip(t, mod), "us-east-1")
	m := res.Info.Manifest
	if res.Info.Name != "aws_cloudformation.us-east-1.iaccat.zip" || m.Kind != KindCloudFormation || m.Source != "aws/cloudformation" || m.Line != "us-east-1" {
		t.Fatalf("manifest %+v name %s", m, res.Info.Name)
	}
	if m.Replacement != ReplacementContract || m.Date != "2026-10-30" || len(m.Versions) != 1 || res.Stats.Types != 2 || res.Stats.Other != 1 {
		t.Fatalf("manifest %+v stats %+v", m, res.Stats)
	}
	if m.Inputs[m.Versions[0]] != res.Stats.Digest || len(res.Stats.Digest) != 64 {
		t.Fatalf("digest %q inputs %v", res.Stats.Digest, m.Inputs)
	}
	if _, err := VerifyIaCCatalog(res.Info.Path); err != nil {
		t.Fatalf("verify: %v", err)
	}
	for path, want := range map[string]string{
		"createOnlyProperties":            `["/properties/Name"]`,
		"conditionalCreateOnlyProperties": `["/properties/Size"]`,
		"readOnlyProperties":              `["/properties/Arn"]`,
		"writeOnlyProperties":             `["/properties/Secret"]`,
		"deprecatedProperties":            `["/properties/OldKnob"]`,
		"required":                        `["Name","Size"]`,
	} {
		f, err := ReadIaCCatalog(res.Info.Path, IaCCatalogRequest{Version: m.Versions[0], Kind: SectionResource, Type: "Example::Widget::Thing", Path: []string{path}})
		if err != nil || string(f.Value) != want || f.Path != path || len(f.Digest) != 64 {
			t.Fatalf("%s: %q %+v %v", path, f.Value, f, err)
		}
	}
	whole, err := ReadIaCCatalog(res.Info.Path, IaCCatalogRequest{Version: m.Versions[0], Kind: SectionResource, Type: "Example::Widget::Thing"})
	if err != nil || strings.Contains(string(whole.Value), "prose") || strings.Contains(string(whole.Value), "description") {
		t.Fatalf("a description reached the catalog: %s %v", whole.Value, err)
	}
	if _, err := ReadIaCCatalog(res.Info.Path, IaCCatalogRequest{Version: m.Versions[0], Kind: SectionResource, Type: "Example::Widget::Missing"}); err == nil {
		t.Fatal("a type the zip does not hold answered")
	}
}

func TestCloudFormationDigestIgnoresBuildTime(t *testing.T) {
	a := importCFN(t, t.TempDir(), cfnFixtureZip(t, time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC)), "us-west-2")
	b := importCFN(t, t.TempDir(), cfnFixtureZip(t, time.Date(2026, 9, 9, 9, 9, 8, 0, time.UTC)), "us-west-2")
	if a.Stats.Digest != b.Stats.Digest {
		t.Fatalf("build time moved the digest: %s %s", a.Stats.Digest, b.Stats.Digest)
	}
	// entry order in the zip does not move it either.
	files := map[string][]byte{"b.json": cfnFixtureType("Example::B::B", nil), "a.json": cfnFixtureType("Example::A::A", nil)}
	d1 := importCFN(t, t.TempDir(), cfnZip(t, time.Time{}, []string{"a.json", "b.json"}, files), "eu-west-1")
	d2 := importCFN(t, t.TempDir(), cfnZip(t, time.Time{}, []string{"b.json", "a.json"}, files), "eu-west-1")
	if d1.Stats.Digest != d2.Stats.Digest || d1.Info.SHA256 != d2.Info.SHA256 {
		t.Fatalf("order moved the digest or the bytes: %s %s", d1.Stats.Digest, d2.Stats.Digest)
	}
	if d1.Info.Manifest.Date != "" {
		t.Fatalf("an unset build time read as a date: %q", d1.Info.Manifest.Date)
	}
	files["a.json"] = cfnFixtureType("Example::A::A", map[string]any{"required": []string{"Other"}})
	d3 := importCFN(t, t.TempDir(), cfnZip(t, time.Time{}, []string{"a.json", "b.json"}, files), "eu-west-1")
	if d3.Stats.Digest == d1.Stats.Digest {
		t.Fatal("a changed body kept the digest")
	}
}

func TestCloudFormationImportDeterministicAndReplaces(t *testing.T) {
	zb := cfnFixtureZip(t, time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC))
	a := importCFN(t, t.TempDir(), zb, "us-east-1")
	b := importCFN(t, t.TempDir(), zb, "us-east-1")
	if a.Info.SHA256 != b.Info.SHA256 {
		t.Fatal("two builds of one zip differ")
	}
	dir := t.TempDir()
	first := importCFN(t, dir, zb, "us-east-1")
	if first.Replaced {
		t.Fatal("the first import replaced something")
	}
	same := importCFN(t, dir, zb, "us-east-1")
	if !same.Replaced || !same.Unchanged || same.Previous != first.Stats.Digest {
		t.Fatalf("same zip: %+v", same)
	}
	next := importCFN(t, dir, cfnZip(t, time.Time{}, []string{"a.json"}, map[string][]byte{"a.json": cfnFixtureType("Example::A::A", nil)}), "us-east-1")
	if !next.Replaced || next.Unchanged || next.Previous != first.Stats.Digest {
		t.Fatalf("new zip: %+v", next)
	}
	other := importCFN(t, dir, zb, "eu-west-1")
	if other.Replaced {
		t.Fatal("another region replaced this one")
	}
	cats, bad := ListIaCCatalogs(dir, false)
	if len(cats) != 2 || len(bad) != 0 {
		t.Fatalf("list: %d %v", len(cats), bad)
	}
}

func TestCloudFormationImportRefusals(t *testing.T) {
	good := map[string][]byte{"a.json": cfnFixtureType("Example::A::A", nil)}
	cases := []struct {
		name   string
		region string
		zip    []byte
		lim    IaCLimits
		want   string
	}{
		{"bad region", "../x", cfnZip(t, time.Time{}, []string{"a.json"}, good), IaCLimits{}, "region"},
		{"no types", "us-east-1", cfnZip(t, time.Time{}, []string{"a.json"}, map[string][]byte{"a.json": []byte(`{"description":"x"}`)}), IaCLimits{}, "no resource type"},
		{"duplicate type", "us-east-1", cfnZip(t, time.Time{}, []string{"a.json", "b.json"}, map[string][]byte{"a.json": good["a.json"], "b.json": good["a.json"]}), IaCLimits{}, "twice"},
		{"malformed json", "us-east-1", cfnZip(t, time.Time{}, []string{"a.json"}, map[string][]byte{"a.json": []byte(`{"typeName":`)}), IaCLimits{}, "a.json"},
		{"blob bound", "us-east-1", cfnZip(t, time.Time{}, []string{"a.json"}, good), func() IaCLimits { l := DefaultIaCLimits(); l.BlobBytes = 64; return l }(), "blob bound"},
		{"entry bound", "us-east-1", cfnZip(t, time.Time{}, []string{"a.json", "b.json"}, map[string][]byte{"a.json": good["a.json"], "b.json": good["a.json"]}), func() IaCLimits { l := DefaultIaCLimits(); l.Entries = 1; return l }(), "entry bound"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			src := writeFile(t, filepath.Join(t.TempDir(), "s.zip"), c.zip)
			_, err := ImportCloudFormationSchema(dir, src, c.region, IaCWriterOptions{Limits: c.lim})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
			if ents, _ := os.ReadDir(dir); len(ents) != 0 {
				t.Fatalf("a refused import left %d files", len(ents))
			}
		})
	}
	if _, err := ImportCloudFormationSchema(t.TempDir(), filepath.Join(t.TempDir(), "absent.zip"), "us-east-1", IaCWriterOptions{}); err == nil {
		t.Fatal("a missing file imported")
	}
	if _, err := ImportCloudFormationSchema(t.TempDir(), "https://example.invalid/s.zip", "us-east-1", IaCWriterOptions{}); err == nil || !strings.Contains(err.Error(), "local") {
		t.Fatalf("a URL was not refused as non-local: %v", err)
	}
}

// A pointer to a property that the object it resolves to does not have is unresolved, so its entry states neither
// required nor optional. The leaf is held to the same test as every segment before it. Nothing here comes from AWS
// (IF-P7).
func TestCFNNestedRequiredLeafMustExist(t *testing.T) {
	str := map[string]any{"type": "string"}
	schema := map[string]any{
		"properties": map[string]any{
			"Config": map[string]any{"$ref": "#/definitions/Config"},
			"Rules":  map[string]any{"type": "array", "items": map[string]any{"$ref": "#/definitions/Rule"}},
		},
		"definitions": map[string]any{
			"Config": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"Type": str, "Mode": str}, "required": []any{"Type"}},
			"Rule":   map[string]any{"type": "object", "properties": map[string]any{"Key": str}, "required": []any{"Key"}},
		},
		"createOnlyProperties": []any{
			"/properties/Config/Type", "/properties/Config/Mode", "/properties/Config/Nope",
			"/properties/Rules/*/Key", "/properties/Rules/*/Nope",
		},
	}
	got := cfnNestedRequired(schema)
	want := map[string]bool{"Config.Type": true, "Config.Mode": false, "Rules[*].Key": true}
	for p, req := range want {
		if r, ok := got[p]; !ok || r != req {
			t.Errorf("%s: resolved %v required %v, want resolved with required %v", p, ok, r, req)
		}
	}
	for _, p := range []string{"Config.Nope", "Rules[*].Nope"} {
		if _, ok := got[p]; ok {
			t.Errorf("%s: a property its object does not have reads resolved: %v", p, got)
		}
	}
}
