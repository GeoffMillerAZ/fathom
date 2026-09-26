// Package mcp_test — BP-42 (docs/plans/bench-pyramid.md): read_source's
// per-call byte budget bounds what goes on the wire, and its description
// says when NOT to reach for it.
//
// The measurement that found the gap: run-20260913T094546Z's H1-T11 t2
// cell read docs/guides/fleet-index.md lines 1-200 at the default
// 8000-byte budget and received 11,189 bytes, 10,849 of them the lines
// array — the budget counted each line's raw text, while the wire carries
// {"line":N,"text":"…"} per line, JSON escaping included. Every other budgeted tool is measured on its
// marshaled rows (budget.go's fitToBudget); read_source now is too.
package mcp_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// linesWireBytes is the byte size of read_source's "lines" array as it goes
// on the wire.
func linesWireBytes(t *testing.T, p map[string]any) int {
	t.Helper()
	b, err := json.Marshal(p["lines"])
	if err != nil {
		t.Fatalf("marshal lines: %v", err)
	}
	return len(b)
}

// docLikeFile renders n short documentation lines of the shape the H1-T11
// cell read: prose with backticks, quotes and a tab, so JSON escaping and
// the per-line {"line","text"} envelope both cost bytes.
func docLikeFile(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "\tline %d: the `fathom ingest` guide says \"refresh via the MCP tool\"\n", i)
	}
	return b.String()
}

// TestReadSource_ByteBudgetBoundsTheWire: at the default budget and at an
// explicit max_bytes, the lines array read_source returns never exceeds the
// budget on the wire, and the cut is disclosed (elided + note).
func TestReadSource_ByteBudgetBoundsTheWire(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "docs/guide.md", docLikeFile(162))
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	for _, tc := range []struct {
		name   string
		args   map[string]any
		budget int
	}{
		{"default budget", map[string]any{"service": "svc", "file": "docs/guide.md", "start_line": 1, "end_line": 162}, 8000},
		{"explicit max_bytes", map[string]any{"service": "svc", "file": "docs/guide.md", "max_bytes": 4000}, 4000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := readSourceCall(t, srv, tc.args)
			if got := linesWireBytes(t, p); got > tc.budget {
				t.Errorf("lines on the wire = %d bytes, want <= the %d-byte budget", got, tc.budget)
			}
			raw, _ := p["lines"].([]any)
			if len(raw) == 0 {
				t.Fatalf("no lines returned: a budget is a cut, never an empty answer")
			}
			elided, ok := p["elided"].(map[string]any)
			if !ok {
				t.Fatalf("elided block missing although lines were dropped: %v", p["note"])
			}
			if int(elided["count"].(float64)) != 162-len(raw) {
				t.Errorf("elided.count = %v, want %d", elided["count"], 162-len(raw))
			}
			if note, _ := p["note"].(string); !strings.Contains(note, "byte cap") {
				t.Errorf("note = %q, want it to name the byte cap", note)
			}
			if end := int(p["end_line"].(float64)); end != len(raw) {
				t.Errorf("end_line = %d, want %d (the last line actually returned)", end, len(raw))
			}
		})
	}
}

// TestReadSource_OverBudgetLineCutOnARuneBoundary: a single line longer than
// the budget comes back cut so its row fits on the wire, and never split
// inside a multi-byte rune.
func TestReadSource_OverBudgetLineCutOnARuneBoundary(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "wide.txt", strings.Repeat("é", 10000)+"\n")
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	p := readSourceCall(t, srv, map[string]any{"service": "svc", "file": "wide.txt"})
	if got := linesWireBytes(t, p); got > 8000 {
		t.Errorf("lines on the wire = %d bytes, want <= 8000", got)
	}
	raw, _ := p["lines"].([]any)
	if len(raw) != 1 {
		t.Fatalf("lines count = %d, want 1", len(raw))
	}
	text, _ := raw[0].(map[string]any)["text"].(string)
	if !utf8.ValidString(text) {
		t.Errorf("cut line is not valid UTF-8 (split inside a rune)")
	}
	if text == "" {
		t.Errorf("cut line is empty: an over-budget line must stay readable")
	}
}

// TestReadSourceDescriptionSaysWhenNotToReachForIt: the description an agent
// reads names the case read_source is NOT for — the index already answered,
// because a find_symbol hit carries the file and line.
func TestReadSourceDescriptionSaysWhenNotToReachForIt(t *testing.T) {
	srv, _ := newReferencesSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3100, Method: "tools/list", Params: map[string]any{},
	})
	var desc string
	for _, raw := range resp["result"].(map[string]any)["tools"].([]any) {
		e := raw.(map[string]any)
		if e["name"] == "read_source" {
			desc, _ = e["description"].(string)
		}
	}
	if desc == "" {
		t.Fatal("read_source not found in tools/list")
	}
	for _, want := range []string{"Not when", "find_symbol", "file and line"} {
		if !strings.Contains(desc, want) {
			t.Errorf("read_source description lacks %q: %q", want, desc)
		}
	}
}

// TestReadSource_FirstLineUnderCapRawOverCapOnWire: a first line whose raw
// text fits the budget but whose row does not — the {"line","text"}
// envelope and JSON escaping push it over — comes back cut to fit, never a
// panic. Before the fix the cut sliced text[:maxBytes] ahead of the wire
// branch, so any of these killed a stdio serve-mcp outright ("slice bounds
// out of range [:50] with length 38"). "First line" is whichever line
// start_line lands on.
func TestReadSource_FirstLineUnderCapRawOverCapOnWire(t *testing.T) {
	root := t.TempDir()
	goMod := "module github.com/example/fathom-demo\n\ngo 1.24\n" // 38-byte first line
	writeCorpusFile(t, root, "go.mod", goMod)
	writeCorpusFile(t, root, "near.txt", strings.Repeat("x", 7990)+"\nsecond\n")
	writeCorpusFile(t, root, "quotes.json", strings.Repeat(`"`, 5000)+"\n")
	writeCorpusFile(t, root, "min.svg", strings.Repeat(`<a href="x">&</a>`, 400)+"\n")
	writeCorpusFile(t, root, "tiny.go", "package x\n\nfunc F() {}\n")
	writeCorpusFile(t, root, "mid.go", "package x\n\n"+strings.Repeat("y", 490)+"\n")
	srv := mcp.NewServer(&extendedFakeBackend{services: []string{"svc"}, svcPath: root})

	for _, tc := range []struct {
		name     string
		args     map[string]any
		first    string // the full first line the cut must be a prefix of
		budget   int
		fitsWire bool // the budget has room for at least a one-rune row
	}{
		{"explicit max_bytes between raw and row size", map[string]any{"service": "svc", "file": "go.mod", "max_bytes": 50}, "module github.com/example/fathom-demo", 50, true},
		{"start_line lands on the long line", map[string]any{"service": "svc", "file": "mid.go", "start_line": 3, "max_bytes": 500}, strings.Repeat("y", 490), 500, true},
		{"default budget, first line just under 8000 raw", map[string]any{"service": "svc", "file": "near.txt"}, strings.Repeat("x", 7990), 8000, true},
		{"escape-heavy first line under the cap raw", map[string]any{"service": "svc", "file": "quotes.json"}, strings.Repeat(`"`, 5000), 8000, true},
		{"HTML-escaped first line under the cap raw", map[string]any{"service": "svc", "file": "min.svg"}, strings.Repeat(`<a href="x">&</a>`, 400), 8000, true},
		{"budget smaller than one row", map[string]any{"service": "svc", "file": "tiny.go", "max_bytes": 20}, "package x", 20, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p map[string]any
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("read_source panicked: %v", r)
					}
				}()
				p = readSourceCall(t, srv, tc.args)
			}()
			raw, _ := p["lines"].([]any)
			if len(raw) != 1 {
				t.Fatalf("lines count = %d, want 1 (the first line, cut)", len(raw))
			}
			text, _ := raw[0].(map[string]any)["text"].(string)
			if text == "" {
				t.Fatalf("cut line is empty: an over-budget line must stay readable")
			}
			if !strings.HasPrefix(tc.first, text) || len(text) >= len(tc.first) {
				t.Errorf("cut text (%d bytes) is not a strict prefix of the %d-byte first line", len(text), len(tc.first))
			}
			if got := linesWireBytes(t, p); tc.fitsWire && got > tc.budget {
				t.Errorf("lines on the wire = %d bytes, want <= the %d-byte budget", got, tc.budget)
			}
			if note, _ := p["note"].(string); !strings.Contains(note, "byte cap") {
				t.Errorf("note = %q, want it to name the byte cap", note)
			}
		})
	}
}
