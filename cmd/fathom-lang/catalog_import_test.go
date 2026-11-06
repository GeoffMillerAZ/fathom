package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// IF-C8: fathom-lang catalog import builds a catalog from a local export or a CloudFormation schema zip, and
// contract reads it back, with no fathom binary.

const importExport = `{"format_version":"1.0","provider_schemas":{"registry.opentofu.org/example/widget":{
 "provider":{"version":0,"block":{"attributes":{}}},
 "resource_schemas":{"widget_thing":{"version":1,"block":{"attributes":{
   "name":{"type":"string","required":true},"old":{"type":"string","optional":true,"deprecated":true}}}}},
 "data_source_schemas":{}}}}`

func runCatalog(t *testing.T, args ...string) (map[string]any, int, string) {
	t.Helper()
	var out, errw bytes.Buffer
	code := run(context.Background(), append([]string{"catalog"}, args...), strings.NewReader(""), &out, &errw)
	var doc map[string]any
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
	}
	return doc, code, errw.String()
}

func TestCatalogImportExportThenContract(t *testing.T) {
	dir := t.TempDir()
	exp := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(exp, []byte(importExport), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, code, stderr := runCatalog(t, "import", "-dir", dir, "-version", "1.2.0", "-tool", "tofu test", exp)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if doc["schema_version"] != float64(1) {
		t.Fatalf("doc %v", doc)
	}
	c := doc["catalog"].(map[string]any)
	if c["source"] != "registry.opentofu.org/example/widget" || c["replacement_facts"] != "absent" || !strings.Contains(c["facts"].(string), "no replacement facts") {
		t.Fatalf("catalog %v", c)
	}
	path := c["path"].(string)
	if filepath.Dir(path) != dir {
		t.Fatalf("path %q", path)
	}
	// The catalog contract reads back is the one just built.
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"contract", "-catalog", path, "widget_thing", "old"}, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("contract exit %d: %s", code, errw.String())
	}
	if !strings.Contains(out.String(), `"deprecated": true`) {
		t.Fatalf("contract %s", out.String())
	}
	// VERSION=export.json, the form fathom iac import takes, names a version per file.
	doc, code, stderr = runCatalog(t, "import", "-dir", dir, "1.3.0="+exp)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	c = doc["catalog"].(map[string]any)
	if c["merged"] != true || len(c["versions"].([]any)) != 2 {
		t.Fatalf("catalog %v", c)
	}
}

func TestCatalogImportCloudFormationZip(t *testing.T) {
	schema := `{"typeName":"Example::Widget::Thing","properties":{"Name":{"type":"string"}},"required":["Name"],"createOnlyProperties":["/properties/Name"],"additionalProperties":false}`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "w.json", Method: zip.Deflate, Modified: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(schema))
	zw.Close()
	zp := filepath.Join(t.TempDir(), "schemas.zip")
	if err := os.WriteFile(zp, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	doc, code, stderr := runCatalog(t, "import", "-dir", dir, "-region", "us-east-1", zp)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	c := doc["catalog"].(map[string]any)
	if c["source"] != "aws/cloudformation" || c["line"] != "us-east-1" || c["types"] != float64(1) || c["replacement_facts"] != "contract" || c["date"] != "2026-10-30" {
		t.Fatalf("catalog %v", c)
	}
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"contract", "-catalog", c["path"].(string), "Example::Widget::Thing"}, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("contract exit %d: %s", code, errw.String())
	}
	if !strings.Contains(out.String(), `"class": "replace"`) {
		t.Fatalf("contract %s", out.String())
	}
}

func TestCatalogImportMisuse(t *testing.T) {
	dir := t.TempDir()
	for name, args := range map[string][]string{
		"no subcommand": {},
		"unknown":       {"build"},
		"no dir":        {"import", "x.json"},
		"no input":      {"import", "-dir", dir},
		"no version":    {"import", "-dir", dir, "x.json"},
		"region + flag": {"import", "-dir", dir, "-region", "us-east-1", "-version", "1", "x.zip"},
		"two zips":      {"import", "-dir", dir, "-region", "us-east-1", "a.zip", "b.zip"},
	} {
		if _, code, _ := runCatalog(t, args...); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
	// A file that is not there is a failure of the run, not misuse.
	if _, code, stderr := runCatalog(t, "import", "-dir", dir, "-version", "1.0.0", filepath.Join(dir, "nope.json")); code != 1 || stderr == "" {
		t.Errorf("a missing file: exit %d stderr %q", code, stderr)
	}
}
