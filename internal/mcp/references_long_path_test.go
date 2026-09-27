// Package mcp_test — BP-41 (docs/plans/bench-pyramid.md): the 100-byte
// bound on one references occurrence is conditional on the location. The
// row trims only its text; file:line (and svc, in a multi-service answer)
// are exact facts and are never cut. So a location that leaves no room
// under 100 bytes keeps the whole path, carries an empty text, and the row
// exceeds 100 bytes by exactly what the location costs past the envelope.
// Deep Java/Kotlin/C# paths reach that; this repo's paths (<= 68
// characters) mostly do not, which is why the 42-character-path contract
// test never saw it.
package mcp_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

const longJavaPath = "src/main/java/com/example/platform/orders/internal/OrderReconciliationService.java"

// referencesOccurrences calls references for query and returns each
// occurrence row decoded and marshaled back as it went on the wire.
func referencesOccurrences(t *testing.T, srv *mcp.Server, query string) []map[string]any {
	t.Helper()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2030, Method: "tools/call",
		Params: map[string]any{
			"name":      "references",
			"arguments": map[string]any{"services": []any{"*"}, "query": query},
		},
	})
	p := xtraPayload(t, resp)
	raw, ok := p["occurrences"].([]any)
	if !ok || len(raw) == 0 {
		t.Fatalf("occurrences missing or empty: %v", p)
	}
	out := make([]map[string]any, len(raw))
	for i, r := range raw {
		out[i] = r.(map[string]any)
	}
	return out
}

// wireLen is the marshaled size of one row.
func wireLen(t *testing.T, row map[string]any) int {
	t.Helper()
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal occurrence: %v", err)
	}
	return len(b)
}

// TestReferencesOccurrenceLongPathKeepsLocationWhole: an occurrence whose
// file:line alone overflows the 100-byte row keeps its location whole and
// empties only its text, so the row's size is exactly its fixed fields; a
// short-path occurrence in the same answer still fits in 100 bytes with
// its text. Single-service and multi-service (svc per row) both hold.
func TestReferencesOccurrenceLongPathKeepsLocationWhole(t *testing.T) {
	long := core.SourceFile{Path: longJavaPath, Content: []byte("class X { void a() { FindMe(); } }\n"), Lang: "java"}
	short := core.SourceFile{Path: "pkg/a.go", Content: []byte("func a() { FindMe() }\n"), Lang: "go"}

	for _, tc := range []struct {
		name     string
		services []string
		sources  map[string][]core.SourceFile
	}{
		{"single service", []string{"svc"}, map[string][]core.SourceFile{"svc": {long, short}}},
		{"multi service", []string{"svc", "svc-two"}, map[string][]core.SourceFile{"svc": {long}, "svc-two": {long, short}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := mcp.NewServer(&referencesFakeBackend{
				extendedFakeBackend: extendedFakeBackend{services: tc.services},
				sourcesByService:    tc.sources,
			})
			var sawLong, sawShort bool
			for _, row := range referencesOccurrences(t, srv, "FindMe") {
				loc, _ := row["loc"].(string)
				text, _ := row["text"].(string)
				size := wireLen(t, row)
				switch {
				case strings.HasPrefix(loc, "src/main/"):
					sawLong = true
					if loc != longJavaPath+":1" {
						t.Errorf("loc = %q, want the whole location %q (a path is never cut)", loc, longJavaPath+":1")
					}
					if text != "" {
						t.Errorf("text = %q, want empty: the location alone overflows the row", text)
					}
					fixed := map[string]any{"loc": row["loc"], "text": "", "c": row["c"]}
					if svc, ok := row["svc"]; ok {
						fixed["svc"] = svc
					}
					if want := wireLen(t, fixed); size != want {
						t.Errorf("long-path row is %d bytes, want %d (its fixed fields only)", size, want)
					}
				default:
					sawShort = true
					if size > 100 {
						t.Errorf("short-path occurrence %v is %d bytes, want <= 100", row, size)
					}
					if text == "" {
						t.Errorf("short-path occurrence %v lost its text", row)
					}
				}
			}
			if !sawLong || !sawShort {
				t.Fatalf("want both a long-path and a short-path occurrence (long %v, short %v)", sawLong, sawShort)
			}
		})
	}
}
