package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// rootedSearchBackend registers one service root.
type rootedSearchBackend struct {
	QueryInterface
	root string
}

func (b rootedSearchBackend) Services(context.Context) ([]string, error) { return []string{"svc"}, nil }
func (b rootedSearchBackend) ServiceMeta(context.Context, string) (string, string, bool, error) {
	return b.root, "", true, nil
}
func (b rootedSearchBackend) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	return nil, nil
}

// RT-88 (§4.5): search reads ripgrep's output through a 1 MiB cap that
// kills rg, cuts a long line at 512 columns, bounds context to 5 lines,
// names a stopped search, and serves a bare root only inside a registered
// service root.
func TestSearchRipgrep_ByteCapKillsAndRootRestricted(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not on PATH")
	}
	root := t.TempDir()
	var b strings.Builder
	for i := 0; i < 40000; i++ {
		b.WriteString("needle padding padding padding padding padding\n") // 48 B a line: ~1.9 MB of matches
	}
	writeFlagFixture(t, root, "big.txt", b.String())
	writeFlagFixture(t, root, "long.txt", "needle"+strings.Repeat("x", 5000)+"\n")
	writeFlagFixture(t, root, "ctx.txt", strings.Repeat("before\n", 20)+"needle-ctx\n"+strings.Repeat("after\n", 20))

	hits, capped, err := searchRipgrep(context.Background(), root, searchOpts{pattern: "needle", caseSensitive: true})
	if err != nil {
		t.Fatal(err)
	}
	if capped == "" || len(hits) == 0 || len(hits) >= 40000 {
		t.Fatalf("uncapped rg: %d hits, capped=%q — want the 1 MiB cap to stop rg and be named", len(hits), capped)
	}
	var n int
	for _, h := range hits {
		n += len(h.File) + len(h.Text) + 8
	}
	if n > searchRgOutputCap {
		t.Fatalf("read %d bytes of rg output, want ≤ %d", n, searchRgOutputCap)
	}

	hits, _, err = searchRipgrep(context.Background(), root, searchOpts{pattern: "needle", caseSensitive: true, pathGlob: "long.txt"})
	if err != nil || len(hits) != 1 || len(hits[0].Text) > searchRgMaxColumns+64 {
		t.Fatalf("long line: %d hits, err %v, text %d bytes — want one hit cut near %d columns", len(hits), err, len(hits[0].Text), searchRgMaxColumns)
	}
	hits, _, err = searchRipgrep(context.Background(), root, searchOpts{pattern: "needle-ctx", caseSensitive: true, contextN: 50})
	if err != nil || len(hits) != 1 || len(hits[0].Before) != searchContextMax || len(hits[0].After) != searchContextMax {
		t.Fatalf("context: %+v err %v — want %d lines either side", hits, err, searchContextMax)
	}

	// Root restriction: a root outside every registered service root is
	// refused by name; one inside is served.
	srv := NewServer(rootedSearchBackend{root: root})
	outside := t.TempDir()
	writeFlagFixture(t, outside, "x.txt", "needle\n")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFlagFixture(t, root, "sub/y.txt", "needle\n")
	for dir, wantErr := range map[string]bool{outside: true, "/": true, filepath.Join(root, "sub"): false} {
		args, _ := json.Marshal(map[string]any{"root": dir, "pattern": "needle", "limit": 1})
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "search", "arguments": json.RawMessage(args)}})
		resp, _ := srv.HandleRequest(context.Background(), raw)
		var r struct {
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(resp, &r)
		if wantErr != (r.Error != nil) {
			t.Fatalf("root %s: error=%+v, want refused=%v", dir, r.Error, wantErr)
		}
		if wantErr && (r.Error.Code != ErrCodeParams || !strings.Contains(r.Error.Message, "not inside an indexed service's root")) {
			t.Fatalf("root %s: refusal %+v, want a named -32602", dir, r.Error)
		}
	}
}

// A file with exactly limit+1 matches was read whole, so no per-file cap
// is claimed (a lower bound that is not one); a file with more is capped,
// and both engines agree.
func TestSearch_PerFileCapOnlyWhenMatchesWereCut(t *testing.T) {
	const limit = 3
	for _, tc := range []struct {
		matches int
		capped  bool
	}{{limit + 1, false}, {limit + 2, true}, {limit + 5, true}} {
		root := t.TempDir()
		writeFlagFixture(t, root, "f.txt", strings.Repeat("needle\n", tc.matches))
		o := searchOpts{pattern: "needle", caseSensitive: true, limit: limit}
		engines := map[string]func() ([]searchRawHit, string, error){
			"walk": func() ([]searchRawHit, string, error) { return searchWalk(context.Background(), root, o) },
		}
		if _, err := exec.LookPath("rg"); err == nil {
			engines["ripgrep"] = func() ([]searchRawHit, string, error) { return searchRipgrep(context.Background(), root, o) }
		}
		for name, run := range engines {
			hits, capped, err := run()
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if (capped != "") != tc.capped {
				t.Errorf("%s with %d matches: capped=%q, want capped=%v", name, tc.matches, capped, tc.capped)
			}
			if len(hits) > limit+1 {
				t.Errorf("%s with %d matches: %d hits, want at most %d", name, tc.matches, len(hits), limit+1)
			}
		}
	}
}
