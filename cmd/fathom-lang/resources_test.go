package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

type resourceFile struct {
	Path   string `json:"path"`
	Error  string `json:"error"`
	Result *struct {
		Resources []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Attrs []struct {
				Path  string `json:"path"`
				Class string `json:"class"`
				Value any    `json:"value"`
				Span  struct {
					File string `json:"file"`
				} `json:"span"`
			} `json:"attrs"`
		} `json:"resources"`
		SyntaxErrors int `json:"syntax_errors"`
	} `json:"result"`
}

func runResources(t *testing.T, args ...string) (files []resourceFile, code int, raw []byte, stderr string) {
	t.Helper()
	var out, errw bytes.Buffer
	code = run(context.Background(), append([]string{"resources"}, args...), strings.NewReader(""), &out, &errw)
	var doc struct {
		SchemaVersion int            `json:"schema_version"`
		Files         []resourceFile `json:"files"`
	}
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
		if doc.SchemaVersion != 1 {
			t.Fatalf("schema_version %d", doc.SchemaVersion)
		}
	}
	return doc.Files, code, out.Bytes(), errw.String()
}

const tfBucket = "resource \"aws_s3_bucket\" \"b\" {\n  bucket = \"my-logs\"\n  acl    = var.acl\n}\nvariable \"acl\" {\n  default = \"private\"\n}\n"

// A file whose read faults the child costs that file's entry alone: it has an error that says why and no result, the
// run exits 1, the files around it read as they would have, and the batch goes on in a new child.
func TestResourcesFaultCostsOneFileAndTheBatchContinues(t *testing.T) {
	shortTimings(t)
	root := faultRoot(t)
	write(t, root, "a.tf", tfBucket)
	write(t, root, "b.tf", tfBucket)
	for _, c := range []struct {
		fault string
		args  []string
		want  string
	}{
		{"fault-stack.txt", nil, "fatal error: stack overflow"},
		{"fault-hang.txt", []string{"-deadline", "300ms"}, "passed the 300ms read deadline"},
		{"fault-alloc.txt", []string{"-max-memory", "400MiB"}, "passed its memory limit"},
		{"fault-kill.txt", nil, "signal: killed"},
	} {
		t.Run(c.fault, func(t *testing.T) {
			args := append([]string{"-root", root}, c.args...)
			files, code, _, stderr := runResources(t, append(args, "a.tf", c.fault, "b.tf")...)
			if code != 1 || len(files) != 3 {
				t.Fatalf("exit %d, %d files: %s", code, len(files), stderr)
			}
			if files[0].Path != "a.tf" || files[2].Path != "b.tf" || files[1].Path != c.fault {
				t.Fatalf("results are not in input order: %+v", files)
			}
			for _, i := range []int{0, 2} {
				if r := files[i].Result; r == nil || files[i].Error != "" || len(r.Resources) != 1 || r.Resources[0].Type != "aws_s3_bucket" {
					t.Errorf("%s did not read as it would have: %+v", files[i].Path, files[i])
				}
			}
			if files[1].Result != nil || !strings.HasPrefix(files[1].Error, killedNotePrefix) || !strings.Contains(files[1].Error, c.want) {
				t.Fatalf("the killed file reads %+v, want an error naming %q and no result", files[1], c.want)
			}
		})
	}
}

// A normal batch is byte-equal in both modes, whatever fails: a missing file, a file no reader reads, a module.
func TestResourcesSupervisedOutputEqualsInProcess(t *testing.T) {
	root := t.TempDir()
	write(t, root, "infra/main.tf", tfBucket)
	write(t, root, "infra/vars.tf", "variable \"extra\" {\n  default = 1\n}\n")
	write(t, root, "stack.yaml", "AWSTemplateFormatVersion: \"2010-09-09\"\nResources:\n  B:\n    Type: AWS::S3::Bucket\n    Properties:\n      BucketName: x\n")
	write(t, root, "Pulumi.yaml", "name: d\nruntime: yaml\nresources:\n  r:\n    type: aws:s3/bucket:Bucket\n    properties:\n      bucket: x\n")
	write(t, root, "a.go", "package a\n")
	for _, extra := range [][]string{nil, {"-values"}, {"-deadline", "20s"}} {
		base := append([]string{"-root", root, "-module", "infra"}, extra...)
		paths := []string{"infra/main.tf", "stack.yaml", "Pulumi.yaml", "missing.tf", "a.go"}
		_, sc, supervised, serr := runResources(t, append(append([]string{}, base...), paths...)...)
		_, ic, inProc, ierr := runResources(t, append(append([]string{"-in-process"}, base...), paths...)...)
		if sc != ic || sc != 1 {
			t.Fatalf("exit %d and %d, want 1 for the missing file in both (%s%s)", sc, ic, serr, ierr)
		}
		if !bytes.Equal(supervised, inProc) {
			t.Fatalf("the two modes differ (%v)\n--- supervised\n%s\n--- in process\n%s", extra, supervised, inProc)
		}
	}
}

func TestResourcesModuleResolvesDefaultsAndNamesFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, "infra/main.tf", tfBucket)
	files, code, _, stderr := runResources(t, "-root", root, "-module", "infra", "-values")
	if code != 0 || len(files) != 1 || files[0].Path != "infra" || files[0].Result == nil {
		t.Fatalf("exit %d: %s %+v", code, stderr, files)
	}
	var found bool
	for _, a := range files[0].Result.Resources[0].Attrs {
		if a.Path == "acl" {
			found = true
			if a.Class != "defaulted" || a.Value != "private" || a.Span.File != "infra/main.tf" {
				t.Errorf("acl = %+v, want defaulted private in infra/main.tf", a)
			}
		}
	}
	if !found {
		t.Fatal("no acl attribute")
	}
	// An absolute directory outside the root is its own root: its files are named by their name alone.
	other := t.TempDir()
	write(t, other, "main.tf", tfBucket)
	files, code, _, stderr = runResources(t, "-root", root, "-module", other)
	if code != 0 || files[0].Result == nil || files[0].Result.Resources[0].Attrs[0].Span.File != "main.tf" {
		t.Fatalf("outside module: exit %d: %s %+v", code, stderr, files)
	}
	// A directory that is not there, or that climbs out of the root, is that entry's error.
	for _, dir := range []string{"nothere", "../escape"} {
		files, code, _, _ = runResources(t, "-root", root, "-module", dir)
		if code != 1 || len(files) != 1 || files[0].Error == "" || files[0].Result != nil {
			t.Errorf("-module %s: exit %d, %+v", dir, code, files)
		}
	}
}

func TestResourcesValuesAreOptIn(t *testing.T) {
	root := t.TempDir()
	write(t, root, "main.tf", tfBucket+"variable \"pw\" {\n  default   = \"cli-sentinel\"\n  sensitive = true\n}\nresource \"a\" \"b\" {\n  p = var.pw\n}\n")
	_, _, plain, _ := runResources(t, "-root", root, "main.tf")
	if bytes.Contains(plain, []byte(`"value"`)) || bytes.Contains(plain, []byte("my-logs")) {
		t.Errorf("a value is printed without -values:\n%s", plain)
	}
	_, _, with, _ := runResources(t, "-root", root, "-values", "main.tf")
	if !bytes.Contains(with, []byte(`"value": "my-logs"`)) {
		t.Errorf("-values prints no value:\n%s", with)
	}
	if bytes.Contains(with, []byte("cli-sentinel")) {
		t.Errorf("a sensitive default is printed under -values:\n%s", with)
	}
}

func TestResourcesMisuse(t *testing.T) {
	for _, args := range [][]string{
		nil, {"-deadline", "31s", "a.tf"}, {"-deadline", "0s", "a.tf"}, {"-max-memory", "lots", "a.tf"}, {"-nope", "a.tf"},
	} {
		var out, errw bytes.Buffer
		if code := run(context.Background(), append([]string{"resources"}, args...), strings.NewReader(""), &out, &errw); code != 2 {
			t.Errorf("%v: exit %d, want 2: %s", args, code, errw.String())
		}
		if out.Len() != 0 {
			t.Errorf("%v: printed JSON on misuse", args)
		}
	}
}

// A child that answered by abandoning a read still has it running, so the parent ends it.
func TestResourcesAbandonedAnswersEndTheChild(t *testing.T) {
	for _, e := range []string{"read-deadline: the read passed its deadline and was abandoned, so none of its facts are returned", "read-busy: reads that passed their deadline are still running, so this read was not started"} {
		if !resourcesAbandoned(resourceResult{Error: e}) {
			t.Errorf("%q does not end the child", e)
		}
	}
	for _, e := range []string{"", "no such file", "iac: no reader for a.go"} {
		if resourcesAbandoned(resourceResult{Error: e}) {
			t.Errorf("%q ends a child that abandoned nothing", e)
		}
	}
}

func runContract(t *testing.T, args ...string) (doc map[string]any, code int, stderr string) {
	t.Helper()
	var out, errw bytes.Buffer
	code = run(context.Background(), append([]string{"contract"}, args...), strings.NewReader(""), &out, &errw)
	if out.Len() > 0 {
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
	}
	return doc, code, errw.String()
}

var widgetCatalog = filepath.Join("..", "..", "pkg", "lang", "iac", "testdata", "registry.opentofu.org_example_widget.v1.iaccat.zip")

func TestContractCommand(t *testing.T) {
	attrs := func(doc map[string]any) []any { return doc["contract"].(map[string]any)["attributes"].([]any) }

	doc, code, stderr := runContract(t, "-catalog", widgetCatalog, "-version", "1.0.0", "widget_thing", "secret")
	if code != 0 || len(attrs(doc)) != 1 || attrs(doc)[0].(map[string]any)["sensitive"] != true {
		t.Fatalf("one attribute: exit %d %s %v", code, stderr, doc)
	}
	doc, _, _ = runContract(t, "-catalog", widgetCatalog, "widget_thing")
	if v := doc["contract"].(map[string]any)["version"]; v != "1.1.0" {
		t.Errorf("the default version is the highest, got %v", v)
	}
	doc, _, _ = runContract(t, "-catalog", widgetCatalog, "-kind", "data_source", "widget_thing")
	if k := doc["contract"].(map[string]any)["kind"]; k != "data_source" {
		t.Errorf("-kind data_source answered %v", k)
	}
	doc, _, _ = runContract(t, "-catalog", widgetCatalog, "-version", "1.0.0", "widget_thing", "settings")
	if n := len(attrs(doc)); n != 2 {
		t.Errorf("an attribute with nested attributes prints both, got %d", n)
	}
	doc, _, _ = runContract(t, "-catalog", widgetCatalog, "-source", "example/widget", "widget_only_data")
	if k := doc["contract"].(map[string]any)["kind"]; k != "data_source" {
		t.Errorf("a data-source-only type answered %v", k)
	}
}

func TestContractCommandErrors(t *testing.T) {
	for name, c := range map[string]struct {
		args []string
		code int
	}{
		"no catalog":      {[]string{"widget_thing"}, 2},
		"too many args":   {[]string{"-catalog", widgetCatalog, "a", "b", "c"}, 2},
		"missing catalog": {[]string{"-catalog", "nothere.zip", "widget_thing"}, 1},
		"unknown type":    {[]string{"-catalog", widgetCatalog, "nope"}, 1},
		"unknown version": {[]string{"-catalog", widgetCatalog, "-version", "9.9.9", "widget_thing"}, 1},
		"other provider":  {[]string{"-catalog", widgetCatalog, "-source", "example/gadget", "widget_thing"}, 1},
		"unknown attr":    {[]string{"-catalog", widgetCatalog, "widget_thing", "nope"}, 1},
		"unknown kind":    {[]string{"-catalog", widgetCatalog, "-kind", "nope", "widget_thing"}, 1},
	} {
		doc, code, stderr := runContract(t, c.args...)
		if code != c.code || doc != nil || stderr == "" {
			t.Errorf("%s: exit %d, want %d (printed %v, stderr %q)", name, code, c.code, doc, stderr)
		}
	}
}
