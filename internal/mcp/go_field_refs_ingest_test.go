package mcp_test

// go_field_refs_ingest_test.go — K17 (wave lf2, LB-F343): "who reads or
// writes Config.Timeout" and a field rename's blast radius, answered through
// the product read path after a real ingest: goextract's member pass names
// the field, the typed pass's field references (core.EdgeFieldRead /
// EdgeFieldWrite) reach it, symbol_context serves them as the field's
// callers, and blast_radius serves them for a rename and for a change to the
// field's file.

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestGoFieldReadersAndWritersThroughTheProductPath(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":       "module example.com/fieldsvc\n\ngo 1.21\n",
		"conf/conf.go": "package conf\n\nimport \"time\"\n\n// Config is read and written from app.\ntype Config struct {\n\tTimeout time.Duration\n\tRetries int\n}\n",
		"app/app.go": "package app\n\nimport (\n\t\"time\"\n\n\t\"example.com/fieldsvc/conf\"\n)\n\n" +
			"func Read(c *conf.Config) time.Duration { return c.Timeout }\n\n" +
			"func Write(c *conf.Config) { c.Timeout = time.Second }\n\n" +
			"func New() conf.Config { return conf.Config{Timeout: time.Minute} }\n\n" +
			"func Retry(c *conf.Config) int { return c.Retries }\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sq, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "fields.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	c := client.New(connector.NewLocalConnector(), sq)
	t.Cleanup(func() { _ = c.Close() })
	if _, err := c.Ingest(t.Context(), client.ServiceRef{ID: "fieldsvc", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	srv := mcp.NewServer(c)
	call := func(id int, tool string, args map[string]any) map[string]any {
		t.Helper()
		resp := roundtrip(t, srv, request{JSONRPC: "2.0", ID: id, Method: "tools/call", Params: map[string]any{"name": tool, "arguments": args}})
		return xtraPayload(t, resp)
	}
	strs := func(v any) []string {
		var out []string
		for _, x := range v.([]any) {
			out = append(out, x.(string))
		}
		sort.Strings(out)
		return out
	}
	readersAndWriters := []string{"fieldsvc::app/app.New", "fieldsvc::app/app.Read", "fieldsvc::app/app.Write"}

	// symbol_context: the field is a symbol, its callers its readers and
	// writers, each labelled with the kind of reference.
	sc := call(9101, "symbol_context", map[string]any{"service": "fieldsvc", "symbol": "Config.Timeout", "snippet": false})
	def, _ := sc["def"].(map[string]any)
	if def == nil || def["kind"] != "GoField" || def["name"] != "Config.Timeout" {
		t.Fatalf("symbol_context def = %v, want the GoField Config.Timeout", sc["def"])
	}
	if got := strs(sc["callers"]); !reflect.DeepEqual(got, readersAndWriters) {
		t.Fatalf("symbol_context callers of Config.Timeout = %v, want %v", got, readersAndWriters)
	}
	typed, _ := sc["typed_edges"].(map[string]any)
	kinds := map[string]string{}
	for _, raw := range typed["callers"].([]any) {
		e := raw.(map[string]any)
		kinds[e["key"].(string)] = e["kind"].(string)
	}
	wantKinds := map[string]string{"fieldsvc::app/app.Read": "field-read", "fieldsvc::app/app.Write": "field-write", "fieldsvc::app/app.New": "field-write"}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Fatalf("typed callers of Config.Timeout = %v, want %v", kinds, wantKinds)
	}
	// A function's callees stay calls: Read calls nothing.
	if got := call(9102, "symbol_context", map[string]any{"service": "fieldsvc", "symbol": "app.Read", "snippet": false})["callees"].([]any); len(got) != 0 {
		t.Fatalf("app.Read's callees = %v: a field read is not a call", got)
	}

	// blast_radius kind:"rename": every site a rename of the field touches.
	rn := call(9103, "blast_radius", map[string]any{"service": "fieldsvc", "kind": "rename", "symbol": "Config.Timeout"})
	if got := strs(rn["callers"]); !reflect.DeepEqual(got, readersAndWriters) {
		t.Fatalf("blast_radius rename Config.Timeout callers = %v, want %v", got, readersAndWriters)
	}

	// blast_radius kind:"change" on the field's file: its readers and
	// writers are depth 1, resolved.
	br := call(9104, "blast_radius", map[string]any{"service": "fieldsvc", "changed_files": []any{"conf/conf.go"}})
	syms := map[string]bool{}
	for _, s := range strs(br["symbols"]) {
		syms[s] = true
	}
	for _, k := range append(readersAndWriters, "fieldsvc::app/app.Retry") {
		if !syms[k] {
			t.Fatalf("blast_radius on conf/conf.go lacks %s: %v", k, br["symbols"])
		}
	}
}
