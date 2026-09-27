package mcp

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func allocBytes(f func()) uint64 {
	var a, b runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&a)
	f()
	runtime.ReadMemStats(&b)
	return b.TotalAlloc - a.TotalAlloc
}

// RT-90 (§4.5, hazard budget-after-build): the dispatch backstop checks a
// result's size before it copies it — an under-budget answer is passed
// through without a marshaled copy — and an over-budget one is cut without
// the marshal → unmarshal → per-key unmarshal cascade.
func TestBudgetedResultNoRemarshalCascade(t *testing.T) {
	row := strings.Repeat("x", 1000)
	under := map[string]any{"rows": make([]string, 1000)} // ~1 MB
	for i := range under["rows"].([]string) {
		under["rows"].([]string)[i] = row
	}
	budgetedResult(under, 2<<20) // warm the encoder pool
	if got := allocBytes(func() {
		for i := 0; i < 5; i++ {
			budgetedResult(under, 2<<20)
		}
	}) / 5; got > 64<<10 {
		t.Fatalf("an under-budget 1 MB result cost %d bytes per call, want < 64 KiB (no marshaled copy)", got)
	}

	over := map[string]any{}
	var payload int
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		rows := make([]string, 125)
		for i := range rows {
			rows[i] = row
		}
		over[k] = rows
		payload += 125 * 1003
	}
	over["h"] = append(over["h"].([]string), row) // h is the largest
	var out any
	got := allocBytes(func() { out = budgetedResult(over, 16<<10) })
	b, _ := json.Marshal(out)
	if !strings.Contains(string(b), `"elided"`) {
		t.Fatalf("over-budget result carries no elided block")
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)
	var h []json.RawMessage
	if json.Unmarshal(m["h"], &h) != nil || len(h) >= 126 {
		t.Fatalf("the largest field h was not the one trimmed: %d rows", len(h))
	}
	if limit := uint64(payload) * 5 / 2; got > limit {
		t.Fatalf("cutting a %d-byte result allocated %d bytes, want < %d (no per-key unmarshal cascade)", payload, got, limit)
	}
}

// jsonSize agrees with json.Marshal on every shape it measures piecewise.
func TestJSONSizeMatchesMarshal(t *testing.T) {
	type st struct {
		A []string `json:"a"`
		B int      `json:"b,omitempty"`
	}
	for _, v := range []any{
		nil, 3, "x<y>", []any{}, []string(nil), map[string]any{}, map[string]any{"k": []int{1, 2}, "<": "&", "z": nil},
		[]any{1, "two", map[string]any{"a": 1}}, st{A: []string{"q"}}, &st{}, []byte("raw"), [2]int{1, 2},
		json.RawMessage(`{"a": 1}`), map[string]json.RawMessage{"x": json.RawMessage(`[1]`)},
		"q\"\\\b\f\n\r\t\x01\x7f<>&é\u2028\u2029\xff日本", map[string]any{"<k>": true, "n": -42, "u": uint8(7), "f": 1.5, "b": false},
	} {
		b, _ := json.Marshal(v)
		if n, err := jsonSize(v, 1<<30); err != nil || n != len(b) {
			t.Errorf("jsonSize(%#v) = %d, %v; Marshal = %d", v, n, err, len(b))
		}
	}
}
