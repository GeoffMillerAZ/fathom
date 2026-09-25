package inventory

import (
	"reflect"
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
		// reflect.DeepEqual, not !=: Item.Type is a *TypeDetail (CV-M4),
		// a fresh pointer on every Extract call even over identical
		// content — pointer identity is not what "deterministic" means
		// here, the pointed-to VALUE is.
		if !reflect.DeepEqual(items[i], again[i]) {
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

// TestExtract_TaggedEmbeddedFieldIsAJSONItem pins design §2.1's json: row
// against encoding/json's own documented rule: a tagged anonymous
// (embedded) field is NOT flattened/promoted — it becomes a normal named
// JSON field under its own tag. extractGoJSON used to skip every field
// with zero Names outright, so a tag change on a genuinely embedded field
// (a real, breaking wire-format change on an exported type) produced no
// json: item on either side of a diff and inventory.Diff saw nothing to
// report.
func TestExtract_TaggedEmbeddedFieldIsAJSONItem(t *testing.T) {
	src := []byte(`package widgets

type Meta struct {
	Version int ` + "`json:\"version\"`" + `
}

type Response struct {
	Meta ` + "`json:\"meta\"`" + `
	Data string ` + "`json:\"data\"`" + `
}
`)
	items := Extract([]File{{Path: "widgets/response.go", Content: src}}, nil)
	keys := keysOf(items)

	wantMeta := "json:widgets.Response.meta"
	found := false
	for _, k := range keys {
		if k == wantMeta {
			found = true
		}
	}
	if !found {
		t.Errorf("keys = %v, want %q among them (tagged embedded field must not be skipped)", keys, wantMeta)
	}
	// Data's own ordinary named field still extracts too — the fix must
	// not disturb the non-embedded path.
	wantData := "json:widgets.Response.data"
	found = false
	for _, k := range keys {
		if k == wantData {
			found = true
		}
	}
	if !found {
		t.Errorf("keys = %v, want %q among them", keys, wantData)
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

// ─── CV-M4 (ADR-044 amendment 2026-09-24): apidiff-agreement fixtures ──────
//
// Each test below reproduces one of docs/bench/readings/
// cv-m4-apidiff-calibration-2026-09-24.md's three measured disagreement
// classes as a minimal fixture, pinned against a real release pair's own
// shape (cli/cli, cobra, prometheus) rather than an invented one.

// TestDiff_AnyInterfaceEmptyIsNotChanged pins class A: cli/cli
// v2.98.0->v2.99.0's `interface{}` -> `any` modernization swept 94 api:
// keys and apidiff reported zero incompatible changes for the pair — `any`
// is Go's own predeclared alias for the empty interface, invisible to
// apidiff's type-identity comparison but not to a printed-signature text
// diff.
func TestDiff_AnyInterfaceEmptyIsNotChanged(t *testing.T) {
	base := []byte(`package api

func (c *Client) GraphQL(host, query string, variables map[string]interface{}, data interface{}) error {
	return nil
}
`)
	head := []byte(`package api

func (c *Client) GraphQL(host, query string, variables map[string]any, data any) error {
	return nil
}
`)
	baseItems := Extract([]File{{Path: "api/client.go", Content: base}}, nil)
	headItems := Extract([]File{{Path: "api/client.go", Content: head}}, nil)
	d := Diff(baseItems, headItems)
	if len(d.Changed) != 0 {
		t.Errorf("Changed = %v, want none (interface{} <-> any is not a loss)", d.Changed)
	}
}

// TestDiff_AdditiveStructFieldIsNotChanged pins class B: cobra
// v1.1.3->v1.2.0 and prometheus v3.11.1->v3.11.2 both add an exported
// struct field (or merely gofmt-reflow one), and apidiff reports both
// compatible — Go's own rule (and apidiff's checkCompatibleObjectSets):
// adding a field to a struct cannot break a keyed struct literal or a
// field selection that already compiled.
func TestDiff_AdditiveStructFieldIsNotChanged(t *testing.T) {
	base := []byte(`package discovery

type SDConfig struct {
	Server string
}
`)
	head := []byte(`package discovery

type SDConfig struct {
	Server    string
	Namespace string
}
`)
	baseItems := Extract([]File{{Path: "discovery/config.go", Content: base}}, nil)
	headItems := Extract([]File{{Path: "discovery/config.go", Content: head}}, nil)
	d := Diff(baseItems, headItems)
	if len(d.Changed) != 0 {
		t.Errorf("Changed = %v, want none (an added exported field alone is compatible)", d.Changed)
	}
}

// TestDiff_RemovedOrRetypedStructFieldIsChanged is class B's other half —
// the additive rule must stay ASYMMETRIC: removing an existing field, or
// changing its type, still is a loss (apidiff: "Anything removed or
// changed from the old set is an incompatible change").
func TestDiff_RemovedOrRetypedStructFieldIsChanged(t *testing.T) {
	base := []byte(`package discovery

type SDConfig struct {
	Server    string
	Namespace string
}
`)
	removed := []byte(`package discovery

type SDConfig struct {
	Server string
}
`)
	retyped := []byte(`package discovery

type SDConfig struct {
	Server    string
	Namespace int
}
`)
	baseItems := Extract([]File{{Path: "discovery/config.go", Content: base}}, nil)

	removedItems := Extract([]File{{Path: "discovery/config.go", Content: removed}}, nil)
	if d := Diff(baseItems, removedItems); len(d.Changed) != 1 || d.Changed[0] != "api:discovery.SDConfig" {
		t.Errorf("field removed: Changed = %v, want [api:discovery.SDConfig]", d.Changed)
	}

	retypedItems := Extract([]File{{Path: "discovery/config.go", Content: retyped}}, nil)
	if d := Diff(baseItems, retypedItems); len(d.Changed) != 1 || d.Changed[0] != "api:discovery.SDConfig" {
		t.Errorf("field retyped: Changed = %v, want [api:discovery.SDConfig]", d.Changed)
	}
}

// TestDiff_TypeAliasIntroductionIsChanged pins class C, CV-M4's one false
// negative: cobra v1.9.0->v1.9.1 turns `type CompletionFunc func(...)`
// into `type CompletionFunc = func(...)` — apidiff's single largest
// incompatible change in the whole calibration sample (a defined type and
// an alias to the identical-looking RHS are still different type KINDS:
// methods on the old named type are lost, interface satisfaction
// changes), invisible to a textual signature diff because the RHS text
// itself never changes.
func TestDiff_TypeAliasIntroductionIsChanged(t *testing.T) {
	base := []byte(`package cobra

type CompletionFunc func(cmd *Command, args []string, toComplete string) ([]string, int)
`)
	head := []byte(`package cobra

type CompletionFunc = func(cmd *Command, args []string, toComplete string) ([]string, int)
`)
	baseItems := Extract([]File{{Path: "cobra/completions.go", Content: base}}, nil)
	headItems := Extract([]File{{Path: "cobra/completions.go", Content: head}}, nil)
	d := Diff(baseItems, headItems)
	if len(d.Changed) != 1 || d.Changed[0] != "api:cobra.CompletionFunc" {
		t.Errorf("Changed = %v, want [api:cobra.CompletionFunc] (an alias introduction is a different type kind)", d.Changed)
	}
}

// TestDiff_AnyInterfaceEmptyInFuncLitValueIsNotChanged extends class A to a
// `var Foo = func(w interface{}) ... {...}` declaration (no explicit
// `Type:`, so extractGoAPI's ValueSpec case renders the VALUE — a whole
// *ast.FuncLit, body included) — cli/cli's own utils.TerminalSize
// (v2.98.0->v2.99.0's real modernization sweep): the closure's PARAMETER
// type must still collapse `interface{}`/`any` even though it is reached
// through a func literal, not a *ast.FuncType used as a type annotation.
func TestDiff_AnyInterfaceEmptyInFuncLitValueIsNotChanged(t *testing.T) {
	base := []byte(`package utils

var TerminalSize = func(w interface{}) (int, int, error) {
	return 0, 0, nil
}
`)
	head := []byte(`package utils

var TerminalSize = func(w any) (int, int, error) {
	return 0, 0, nil
}
`)
	baseItems := Extract([]File{{Path: "utils/utils.go", Content: base}}, nil)
	headItems := Extract([]File{{Path: "utils/utils.go", Content: head}}, nil)
	d := Diff(baseItems, headItems)
	if len(d.Changed) != 0 {
		t.Errorf("Changed = %v, want none (interface{} <-> any inside a func-literal value is not a loss)", d.Changed)
	}
}

// TestDiff_VarLiteralPromotedToNamedConstIsNotChanged pins a fourth gap
// the full 50-pair run found beyond CV-M4's three named classes: cobra
// v1.5.0->v1.6.0 promoted a `var Enable... = true` bool literal to a
// named constant default (`var Enable... = defaultCommandSorting`).
// apidiff's own `checkObjects` never looks at a *types.Var's initializer
// at all, only its TYPE (`bool`, unchanged either way) — so this must not
// be a loss, whatever the initializer's own text says.
func TestDiff_VarLiteralPromotedToNamedConstIsNotChanged(t *testing.T) {
	base := []byte(`package widgets

var EnableSorting = true
`)
	head := []byte(`package widgets

const defaultSorting = true

var EnableSorting = defaultSorting
`)
	baseItems := Extract([]File{{Path: "widgets/w.go", Content: base}}, nil)
	headItems := Extract([]File{{Path: "widgets/w.go", Content: head}}, nil)
	d := Diff(baseItems, headItems)
	if len(d.Changed) != 0 {
		t.Errorf("Changed = %v, want none (a var's TYPE, bool, did not change)", d.Changed)
	}
}

// TestDiff_VarLiteralKindChangeIsStillChanged pins the asymmetric half:
// a var whose inferred kind genuinely differs (bool -> string) must still
// be a loss.
func TestDiff_VarLiteralKindChangeIsStillChanged(t *testing.T) {
	base := []byte(`package widgets

var Mode = true
`)
	head := []byte(`package widgets

var Mode = "on"
`)
	baseItems := Extract([]File{{Path: "widgets/w.go", Content: base}}, nil)
	headItems := Extract([]File{{Path: "widgets/w.go", Content: head}}, nil)
	d := Diff(baseItems, headItems)
	if len(d.Changed) != 1 || d.Changed[0] != "api:widgets.Mode" {
		t.Errorf("Changed = %v, want [api:widgets.Mode] (bool -> string is a real type change)", d.Changed)
	}
}

// TestDiff_ConstValueChangeIsStillChanged pins that this fix is VAR-only:
// a const's actual VALUE is part of apidiff's own compatibility check
// (constChanges), so two same-kind but different-value consts must still
// be a loss — unlike a var, where only the type is ever checked.
func TestDiff_ConstValueChangeIsStillChanged(t *testing.T) {
	base := []byte(`package widgets

const MaxRetries = 3
`)
	head := []byte(`package widgets

const MaxRetries = 5
`)
	baseItems := Extract([]File{{Path: "widgets/w.go", Content: base}}, nil)
	headItems := Extract([]File{{Path: "widgets/w.go", Content: head}}, nil)
	d := Diff(baseItems, headItems)
	if len(d.Changed) != 1 || d.Changed[0] != "api:widgets.MaxRetries" {
		t.Errorf("Changed = %v, want [api:widgets.MaxRetries] (a const's VALUE is part of apidiff's own check)", d.Changed)
	}
}

// TestDiff_InterfaceMethodAdditionIncompatibleUnlessSealed pins the design
// doc's other named rule (apidiff's own asymmetric interface check,
// compatibility.go checkCompatibleInterface) alongside the three measured
// classes: unlike a struct, adding a method to a fully-exported interface
// IS a loss (it breaks an external implementer's method set); the same
// addition to an interface that already carries an unexported method is
// compatible, since nothing outside the package could implement it
// anyway.
func TestDiff_InterfaceMethodAdditionIncompatibleUnlessSealed(t *testing.T) {
	openBase := []byte(`package widgets

type Reader interface {
	Read() error
}
`)
	openHead := []byte(`package widgets

type Reader interface {
	Read() error
	Close() error
}
`)
	baseItems := Extract([]File{{Path: "widgets/reader.go", Content: openBase}}, nil)
	headItems := Extract([]File{{Path: "widgets/reader.go", Content: openHead}}, nil)
	if d := Diff(baseItems, headItems); len(d.Changed) != 1 || d.Changed[0] != "api:widgets.Reader" {
		t.Errorf("open interface: Changed = %v, want [api:widgets.Reader]", d.Changed)
	}

	sealedBase := []byte(`package widgets

type Reader interface {
	Read() error
	sealed()
}
`)
	sealedHead := []byte(`package widgets

type Reader interface {
	Read() error
	Close() error
	sealed()
}
`)
	baseItems = Extract([]File{{Path: "widgets/reader.go", Content: sealedBase}}, nil)
	headItems = Extract([]File{{Path: "widgets/reader.go", Content: sealedHead}}, nil)
	if d := Diff(baseItems, headItems); len(d.Changed) != 0 {
		t.Errorf("sealed interface: Changed = %v, want none", d.Changed)
	}
}
