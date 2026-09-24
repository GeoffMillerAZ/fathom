package inventory

import (
	"sort"
	"testing"
)

// TestExtract_EveryKeyKind pins CV-15 (ADR-044 §2.1): every key kind the
// design table names is produced by Extract from raw file content alone —
// api, json, route, env, rpc, cli, and a repo-configured pattern.
func TestExtract_EveryKeyKind(t *testing.T) {
	goSrc := []byte(`package widgets

import (
	"net/http"
	"os"
)

// Widget is exported.
type Widget struct {
	ID   string ` + "`json:\"id\"`" + `
	name string
}

func NewWidget(id string) *Widget { return &Widget{ID: id} }

func (w *Widget) Rename(n string) error { w.name = n; return nil }

func unexported() {}

func init() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /widgets", handleWidgets)
	_ = os.Getenv("WIDGET_TOKEN")
}

func handleWidgets(w http.ResponseWriter, r *http.Request) {}
`)

	protoSrc := []byte(`syntax = "proto3";
package widgets.v1;

service WidgetService {
  rpc GetWidget(GetWidgetRequest) returns (Widget) {}
}

message GetWidgetRequest {}
message Widget {}
`)

	cliSrc := []byte(`package cli

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	return &cobra.Command{Use: "widget-cli [flags]"}
}
`)

	mcpSrc := []byte(`package mcp

func register() {
	s.registerTool(toolDescriptor{
		Name:        "find_widget",
		Description: "finds a widget",
	})
}
`)

	files := []File{
		{Path: "widgets/widget.go", Content: goSrc},
		{Path: "proto/widget.proto", Content: protoSrc},
		{Path: "cli/root.go", Content: cliSrc},
		{Path: "internal/mcp/tools.go", Content: mcpSrc},
	}
	patterns := []Pattern{
		{Kind: "tool", Glob: "internal/mcp/*.go", Regex: `Name:\s*"([a-zA-Z_][a-zA-Z0-9_]*)"`},
	}

	items := Extract(files, patterns)
	byKey := map[string]Item{}
	for _, it := range items {
		byKey[it.Key] = it
	}

	wantKinds := map[string]string{
		"api:widgets.NewWidget":                  "api",
		"api:widgets.Widget":                     "api",
		"api:widgets.Widget.Rename":              "api",
		"json:widgets.Widget.id":                 "json",
		"route:GET /widgets":                     "route",
		"env:WIDGET_TOKEN":                       "env",
		"rpc:widgets.v1.WidgetService/GetWidget": "rpc",
		"cli:widget-cli":                         "cli",
		"tool:find_widget":                       "tool",
	}
	for key, kind := range wantKinds {
		it, ok := byKey[key]
		if !ok {
			t.Errorf("missing item %q; got keys %v", key, keysOf(items))
			continue
		}
		if it.Kind != kind {
			t.Errorf("item %q kind = %q, want %q", key, it.Kind, kind)
		}
	}

	// Unexported func, unexported field, and a test-file-only declaration
	// must never appear.
	for _, bad := range []string{"api:widgets.unexported", "json:widgets.Widget.name"} {
		if _, ok := byKey[bad]; ok {
			t.Errorf("unwanted item %q present", bad)
		}
	}

	// A func-value handler reference ("handleWidgets") must never itself
	// become an api: item accidentally through route extraction only —
	// it's unexported so extractGoAPI already excludes it; route extraction
	// still runs, and the route Detail carries the handler name.
	if it := byKey["route:GET /widgets"]; it.Detail != "handleWidgets" {
		t.Errorf("route detail = %q, want handler name %q", it.Detail, "handleWidgets")
	}

	// Deterministic: two calls over identical input produce byte-identical
	// output (order included).
	again := Extract(files, patterns)
	if len(again) != len(items) {
		t.Fatalf("non-deterministic length: %d vs %d", len(again), len(items))
	}
	for i := range items {
		if items[i] != again[i] {
			t.Fatalf("non-deterministic output at [%d]: %+v vs %+v", i, items[i], again[i])
		}
	}
	if !sort.SliceIsSorted(items, func(i, j int) bool { return items[i].Key < items[j].Key }) {
		t.Error("Extract output not sorted by Key")
	}
}

// TestExtract_SkipsTestFiles pins the "_test.go carries no capability
// surface" rule, applied uniformly across every kind.
func TestExtract_SkipsTestFiles(t *testing.T) {
	src := []byte(`package widgets

func ExportedInTestFile() {}
`)
	items := Extract([]File{{Path: "widgets/widget_test.go", Content: src}}, nil)
	if len(items) != 0 {
		t.Errorf("expected no items from a _test.go file, got %+v", items)
	}
}

// TestExtract_ExcludesInternalAndMain pins the "neither internal/ nor
// main" package exclusion (design doc §2.1).
func TestExtract_ExcludesInternalAndMain(t *testing.T) {
	internalSrc := []byte(`package widgets

func Exported() {}
`)
	items := Extract([]File{{Path: "internal/widgets/widget.go", Content: internalSrc}}, nil)
	if len(items) != 0 {
		t.Errorf("internal/ package leaked into api: surface: %+v", items)
	}

	mainSrc := []byte(`package main

func Exported() {}
`)
	items = Extract([]File{{Path: "cmd/thing/main.go", Content: mainSrc}}, nil)
	if len(items) != 0 {
		t.Errorf("main package leaked into api: surface: %+v", items)
	}
}

// TestExtract_SignatureDetailIgnoresParamNames pins "detail is the
// signature with parameter names removed": renaming a parameter alone
// must not change an api: item's Detail.
func TestExtract_SignatureDetailIgnoresParamNames(t *testing.T) {
	a := []byte(`package widgets

func Do(id string, n int) error { return nil }
`)
	b := []byte(`package widgets

func Do(widgetID string, count int) error { return nil }
`)
	itemsA := Extract([]File{{Path: "widgets/w.go", Content: a}}, nil)
	itemsB := Extract([]File{{Path: "widgets/w.go", Content: b}}, nil)
	var da, db string
	for _, it := range itemsA {
		if it.Key == "api:widgets.Do" {
			da = it.Detail
		}
	}
	for _, it := range itemsB {
		if it.Key == "api:widgets.Do" {
			db = it.Detail
		}
	}
	if da == "" || db == "" {
		t.Fatalf("api:widgets.Do not found in one of the extracts (a=%q b=%q)", da, db)
	}
	if da != db {
		t.Errorf("signature detail changed on a param rename alone: %q vs %q", da, db)
	}
}

func keysOf(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Key
	}
	return out
}

// TestDiff_RemovedChangedAdded pins CV-16: base→head delta over Item.Key,
// with Detail deciding removed/changed/added.
func TestDiff_RemovedChangedAdded(t *testing.T) {
	base := []Item{
		{Key: "api:foo.Old", Detail: "func()"},
		{Key: "api:foo.Stable", Detail: "func() error"},
		{Key: "api:foo.Changing", Detail: "func(int) error"},
	}
	head := []Item{
		{Key: "api:foo.Stable", Detail: "func() error"},
		{Key: "api:foo.Changing", Detail: "func(string) error"},
		{Key: "api:foo.New", Detail: "func()"},
	}

	d := Diff(base, head)

	if len(d.Removed) != 1 || d.Removed[0] != "api:foo.Old" {
		t.Errorf("Removed = %v, want [api:foo.Old]", d.Removed)
	}
	if len(d.Changed) != 1 || d.Changed[0] != "api:foo.Changing" {
		t.Errorf("Changed = %v, want [api:foo.Changing]", d.Changed)
	}
	if len(d.Added) != 1 || d.Added[0] != "api:foo.New" {
		t.Errorf("Added = %v, want [api:foo.New]", d.Added)
	}
}

// TestDiff_EmptyBothIsEmptyDelta pins the degenerate case.
func TestDiff_EmptyBothIsEmptyDelta(t *testing.T) {
	d := Diff(nil, nil)
	if len(d.Removed) != 0 || len(d.Changed) != 0 || len(d.Added) != 0 {
		t.Errorf("Diff(nil, nil) = %+v, want all empty", d)
	}
}

// TestDiff_IdenticalIsEmptyDelta pins "same key, same detail" producing no
// entries at all (never a false "changed").
func TestDiff_IdenticalIsEmptyDelta(t *testing.T) {
	items := []Item{{Key: "route:GET /x", Detail: "handler"}}
	d := Diff(items, items)
	if len(d.Removed) != 0 || len(d.Changed) != 0 || len(d.Added) != 0 {
		t.Errorf("Diff(x, x) = %+v, want all empty", d)
	}
}
