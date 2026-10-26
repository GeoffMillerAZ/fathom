package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, scope, policy, request) {
	t.Helper()
	root := t.TempDir()
	b := []byte("public one\npublic two\n")
	if e := os.WriteFile(filepath.Join(root, "a.txt"), b, 0600); e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(b)
	return root, scope{Version: "command-guard.scope/1", RequiredLayerIDs: []string{"L"}, RequiredRuleIDs: []string{"R"}}, policy{Version: "command-guard.policy/1", Layers: []layer{{ID: "L", AllowedOperations: []string{"read-range", "search-literal"}, Limits: limits{MaxFileBytes: 1000, MaxTotalBytes: 2000, MaxOutputBytes: 1000, MaxResults: 10}, DeniedPathPrefixes: []string{}}}, Rules: []rule{{ID: "R", Pattern: "SECRET"}}}, request{Version: "command-guard.request/1", Kind: "read-range", Files: []fileSpec{{Path: "a.txt", SHA256: hex.EncodeToString(h[:])}}, StartLine: 1, EndLine: 1}
}
func TestReadSearchAndLateRule(t *testing.T) {
	root, s, p, q := fixture(t)
	r := run(root, s, p, q, "json")
	if r.Status != "no-findings" || len(r.Rows) != 1 || r.Rows[0].Text != "public one" {
		t.Fatalf("%+v", r)
	}
	q.Kind = "search-literal"
	q.StartLine = 0
	q.EndLine = 0
	q.Literal = "two"
	r = run(root, s, p, q, "json")
	if len(r.Rows) != 1 || r.Rows[0].Line != 2 {
		t.Fatalf("%+v", r)
	}
	b := []byte("public one\nSECRET late\n")
	os.WriteFile(filepath.Join(root, "a.txt"), b, 0600)
	h := sha256.Sum256(b)
	q.Files[0].SHA256 = hex.EncodeToString(h[:])
	r = run(root, s, p, q, "json")
	if r.Status != "blocked" || len(r.Rows) != 0 || r.FilesRead != 1 {
		t.Fatalf("%+v", r)
	}
}
func TestFailClosedPathsDigestAndCaps(t *testing.T) {
	root, s, p, q := fixture(t)
	q.Files[0].SHA256 = strings.Repeat("0", 64)
	if r := run(root, s, p, q, "json"); r.Status != "incomplete" || len(r.Rows) != 0 {
		t.Fatal(r)
	}
	_, s, p, q = fixture(t)
	q.Files[0].Path = "../a.txt"
	if r := run(root, s, p, q, "json"); r.Status != "incomplete" {
		t.Fatal(r)
	}
	q.Files[0].Path = "a.txt"
	p.Layers[0].Limits.MaxFileBytes = 1
	if r := run(root, s, p, q, "json"); r.Status != "incomplete" {
		t.Fatal(r)
	}
	p.Layers[0].Limits.MaxFileBytes = 1000
	p.Layers[0].DeniedPathPrefixes = []string{"a.txt"}
	if r := run(root, s, p, q, "json"); r.Status != "blocked" {
		t.Fatal(r)
	}
}
func TestSymlinkAndFIFO(t *testing.T) {
	root, s, p, q := fixture(t)
	if e := os.Symlink("a.txt", filepath.Join(root, "link")); e != nil {
		t.Skip(e)
	}
	q.Files[0].Path = "link"
	if r := run(root, s, p, q, "json"); r.Status != "incomplete" {
		t.Fatal(r)
	}
}
func TestTruncationAndPrivacy(t *testing.T) {
	root, s, p, q := fixture(t)
	p.Layers[0].Limits.MaxResults = 1
	q.EndLine = 2
	r := run(root, s, p, q, "json")
	if r.Returned != 1 || r.Omitted != 1 || !r.Truncated {
		t.Fatal(r)
	}
	q.Files[0].SHA256 = strings.Repeat("0", 64)
	r = run(root, s, p, q, "json")
	b := encode(r, "json")
	if bytes.Contains(b, []byte("a.txt")) || bytes.Contains(b, []byte(q.Files[0].SHA256)) {
		t.Fatal(string(b))
	}
}
func TestStrictCodec(t *testing.T) {
	dir := t.TempDir()
	for _, raw := range []string{`{"version":"x","version":"y"}`, `{"Version":"x"}`, `{"version":null}`, `{"version":"x","unknown":1}`, `{"version":"x"} true`, `{"version":"\ud800"}`} {
		f := filepath.Join(dir, "v.json")
		os.WriteFile(f, []byte(raw), 0600)
		var s scope
		if strictFile(f, &s) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
func TestPortablePathsAndUnsafeText(t *testing.T) {
	for _, p := range []string{"../x", "a\\b", "C:/x", "CON.txt", "dir/NUL", "LPT1", "a/./b", "a\nb"} {
		if validPath(p) {
			t.Fatalf("accepted path %q", p)
		}
	}
	for _, s := range []string{"bad\x00", "bad\x1b[31m", "bad\u202e", "bad\rline"} {
		if validText(s) {
			t.Fatalf("accepted text %q", s)
		}
	}
}
func TestWriteError(t *testing.T) {
	if got := emit(failWriter{}, base(), "json", 2); got != 2 {
		t.Fatal(got)
	}
	for _, w := range []partialWriter{{false}, {true}} {
		if got := emit(w, base(), "json", 0); got != 2 {
			t.Fatal(got)
		}
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

type partialWriter struct{ withError bool }

func (w partialWriter) Write(b []byte) (int, error) {
	if w.withError {
		return len(b) / 2, os.ErrClosed
	}
	return len(b) / 2, nil
}
func BenchmarkRead(b *testing.B) {
	root, s, p, q := fixtureB(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run(root, s, p, q, "json")
	}
}
func fixtureB(b *testing.B) (string, scope, policy, request) {
	b.Helper()
	root := b.TempDir()
	data := []byte("public\n")
	os.WriteFile(filepath.Join(root, "a.txt"), data, 0600)
	h := sha256.Sum256(data)
	return root, scope{Version: "command-guard.scope/1", RequiredLayerIDs: []string{"L"}, RequiredRuleIDs: []string{"R"}}, policy{Version: "command-guard.policy/1", Layers: []layer{{ID: "L", AllowedOperations: []string{"read-range"}, Limits: limits{1000, 1000, 1000, 10}}}, Rules: []rule{{ID: "R", Pattern: "SECRET"}}}, request{Version: "command-guard.request/1", Kind: "read-range", Files: []fileSpec{{"a.txt", hex.EncodeToString(h[:])}}, StartLine: 1, EndLine: 1}
}
