package main

import (
	"bytes"
	"encoding/json"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineWorkflowAndRefusal(t *testing.T) {
	d := t.TempDir()
	spec := filepath.Join(d, "spec.json")
	input := `{"version":1,"layers":[{"name":"general","nodes":[{"slug":"review","source":"general.cue","stable_key":"review","facets":{"language":["go"]}}]}]}`
	if err := os.WriteFile(spec, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"resolve", "-spec", spec}, &out); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(d, "bundle.json")
	if err := os.WriteFile(bundle, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"explain", "-bundle", bundle, "-id", "review"}, {"query", "-bundle", bundle, "-facet", "language=go"}, {"diff", "-before", bundle, "-after", bundle}} {
		out.Reset()
		if err := run(args, &out); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !json.Valid(out.Bytes()) {
			t.Fatal("invalid output")
		}
	}
	out.Reset()
	if err := run([]string{"resolve", "-spec", spec, "-format", "ndjson"}, &out); err != nil || !strings.Contains(out.String(), `"id":"review"`) {
		t.Fatalf("ndjson: %s %v", out.String(), err)
	}
	for _, args := range [][]string{{"resolve", "-spec", spec, "-format", "unknown"}, {"explain", "-bundle", bundle, "-id", "missing"}, {"query", "-bundle", bundle, "-facet", "bad"}, {"diff"}, {"unknown"}} {
		out.Reset()
		if err := run(args, &out); err == nil {
			t.Fatalf("accepted %v", args)
		}
		if out.Len() != 0 {
			t.Fatalf("partial output on refusal: %s", out.String())
		}
	}
	// Integrity failures must stop before any report bytes are emitted.
	raw, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.Replace(raw, []byte(`"Source": "general.cue"`), []byte(`"Source": "changed.cue"`), 1)
	if err := os.WriteFile(bundle, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run([]string{"query", "-bundle", bundle}, &out); err == nil || out.Len() != 0 {
		t.Fatal("tampered bundle produced a report")
	}
}

func TestScanGoUsesRealInventory(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"scan-go", "-root", "../..", "-revision", "test-revision"}, &out); err != nil {
		t.Fatal(err)
	}
	b, err := catalogkit.DecodeBundle(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	n, ok := b.Catalog.Get("kits/catalog-kit")
	if !ok || n.Provenance.Watermark != "test-revision" {
		t.Fatal("real kit inventory missing")
	}
}
