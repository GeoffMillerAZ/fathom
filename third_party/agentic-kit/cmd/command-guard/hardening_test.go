package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func TestTotalReadCapOneByteProbe(t *testing.T) {
	root, s, p, q := fixture(t)
	first := []byte("public one\npublic two\n")
	second := []byte("second file\n")
	if e := os.WriteFile(filepath.Join(root, "b.txt"), second, 0600); e != nil {
		t.Fatal(e)
	}
	q.Files = append(q.Files, fileSpec{Path: "b.txt", SHA256: digest(second)})
	p.Layers[0].Limits.MaxTotalBytes = len(first) + 1
	r := run(root, s, p, q, "json")
	if r.Status != "incomplete" || r.BytesRead != len(first)+2 || r.BytesScanned != len(first) || len(r.Rows) != 0 {
		t.Fatalf("%+v", r)
	}
}
func TestCaseFoldDeniedAndDuplicates(t *testing.T) {
	root, s, p, q := fixture(t)
	p.Layers[0].DeniedPathPrefixes = []string{"A.TXT"}
	r := run(root, s, p, q, "json")
	if r.Status != "blocked" {
		t.Fatal(r)
	}
	p.Layers[0].DeniedPathPrefixes = nil
	q.Files = append(q.Files, fileSpec{Path: "A.txt", SHA256: q.Files[0].SHA256})
	r = run(root, s, p, q, "json")
	if r.Status != "incomplete" {
		t.Fatal(r)
	}
	p.Layers[0].DeniedPathPrefixes = make([]string, 65)
	for i := range p.Layers[0].DeniedPathPrefixes {
		p.Layers[0].DeniedPathPrefixes[i] = "private"
	}
	q.Files = q.Files[:1]
	if r = run(root, s, p, q, "json"); r.Status != "incomplete" {
		t.Fatal(r)
	}
}
func TestLongLineOutputBound(t *testing.T) {
	root, s, p, q := fixture(t)
	long := []byte(strings.Repeat("X", 100_000) + "\n")
	os.WriteFile(filepath.Join(root, "a.txt"), long, 0600)
	q.Files[0].SHA256 = digest(long)
	p.Layers[0].Limits.MaxFileBytes = len(long)
	p.Layers[0].Limits.MaxTotalBytes = len(long)
	p.Layers[0].Limits.MaxOutputBytes = 500
	r := run(root, s, p, q, "json")
	if r.Status != "no-findings" || r.TotalMatches != 1 || r.Returned != 0 || r.Omitted != 1 || len(encode(r, "json"))+1 > 500 {
		t.Fatalf("%+v", r)
	}
}
func TestCLIEntryPoint(t *testing.T) {
	root, e := filepath.Abs("testdata")
	if e != nil {
		t.Fatal(e)
	}
	common := []string{"inspect", "--root", root, "--scope", "testdata/scope.json", "--policy", "testdata/policy.json", "--request", "testdata/request-read.json", "--format", "json"}
	var out bytes.Buffer
	if code := execute(common, &out); code != 0 {
		t.Fatalf("code %d: %s", code, out.String())
	}
	var r report
	if json.Unmarshal(out.Bytes(), &r) != nil || r.Returned != 2 || r.Status != "no-findings" {
		t.Fatal(out.String())
	}
	out.Reset()
	search := append([]string(nil), common...)
	search[8] = "testdata/request-search.json"
	search[10] = "text"
	if code := execute(search, &out); code != 0 || !bytes.Contains(out.Bytes(), []byte(`"returned": 1`)) {
		t.Fatalf("code %d: %s", code, out.String())
	}
	out.Reset()
	bad := append([]string(nil), common...)
	bad[8] = "testdata/missing.json"
	if code := execute(bad, &out); code != 2 || bytes.Contains(out.Bytes(), []byte("missing")) {
		t.Fatalf("code %d: %s", code, out.String())
	}
	var pol policy
	if e := strictFile("testdata/policy.json", &pol); e != nil {
		t.Fatal(e)
	}
	pol.Layers[0].DeniedPathPrefixes = []string{"SAMPLE.TXT"}
	blockedFile := filepath.Join(t.TempDir(), "policy.json")
	control, _ := json.Marshal(pol)
	if e := os.WriteFile(blockedFile, control, 0600); e != nil {
		t.Fatal(e)
	}
	blocked := append([]string(nil), common...)
	blocked[6] = blockedFile
	out.Reset()
	if code := execute(blocked, &out); code != 1 || bytes.Contains(out.Bytes(), []byte("alpha")) {
		t.Fatalf("code %d: %s", code, out.String())
	}
	var req request
	if e := strictFile("testdata/request-search.json", &req); e != nil {
		t.Fatal(e)
	}
	req.Literal = "beta\npublic"
	requestFile := filepath.Join(t.TempDir(), "request.json")
	control, _ = json.Marshal(req)
	if e := os.WriteFile(requestFile, control, 0600); e != nil {
		t.Fatal(e)
	}
	invalidSearch := append([]string(nil), search...)
	invalidSearch[8] = requestFile
	out.Reset()
	if code := execute(invalidSearch, &out); code != 2 || bytes.Contains(out.Bytes(), []byte("beta")) {
		t.Fatalf("code %d: %s", code, out.String())
	}
	for _, tc := range []struct {
		original string
		extra    string
	}{
		{"testdata/request-read.json", `,"literal":""`},
		{"testdata/request-search.json", `,"startLine":0`},
	} {
		raw, e := os.ReadFile(tc.original)
		if e != nil {
			t.Fatal(e)
		}
		mutated := strings.TrimSpace(string(raw))
		mutated = strings.TrimSuffix(mutated, "}") + tc.extra + "}"
		file := filepath.Join(t.TempDir(), "request.json")
		if e := os.WriteFile(file, []byte(mutated), 0600); e != nil {
			t.Fatal(e)
		}
		args := append([]string(nil), common...)
		args[8] = file
		out.Reset()
		if code := execute(args, &out); code != 2 || bytes.Contains(out.Bytes(), []byte("alpha")) {
			t.Fatalf("code %d: %s", code, out.String())
		}
	}
	out.Reset()
	if code := execute([]string{"help"}, &out); code != 0 || !bytes.Contains(out.Bytes(), []byte("inspect")) {
		t.Fatal(code, out.String())
	}
	if code := execute(common, failWriter{}); code != 2 {
		t.Fatal(code)
	}
}
func TestLateFileFailureOrSecretSuppressesAll(t *testing.T) {
	root, s, p, q := fixture(t)
	q.Files = append(q.Files, fileSpec{Path: "missing.txt", SHA256: strings.Repeat("0", 64)})
	r := run(root, s, p, q, "json")
	if r.Status != "incomplete" || len(r.Rows) != 0 {
		t.Fatal(r)
	}
	b := []byte("SECRET in second\n")
	os.WriteFile(filepath.Join(root, "b.txt"), b, 0600)
	q.Files[1] = fileSpec{Path: "b.txt", SHA256: digest(b)}
	r = run(root, s, p, q, "json")
	if r.Status != "blocked" || r.FilesRead != 2 || len(r.Rows) != 0 {
		t.Fatal(r)
	}
}
func TestNestedRootRejectsSymlinkSwap(t *testing.T) {
	root, s, p, q := fixture(t)
	public := filepath.Join(root, "public")
	private := filepath.Join(root, "private")
	os.Mkdir(public, 0700)
	os.Mkdir(private, 0700)
	os.WriteFile(filepath.Join(public, "x.txt"), []byte("PUBLIC\n"), 0600)
	secret := []byte("PRIVATE\n")
	os.WriteFile(filepath.Join(private, "x.txt"), secret, 0600)
	q.Files = []fileSpec{{Path: "public/x.txt", SHA256: digest(secret)}}
	p.Layers[0].DeniedPathPrefixes = []string{"private"}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			os.Rename(public, filepath.Join(root, "parked"))
			os.Symlink("private", public)
			os.Remove(public)
			os.Rename(filepath.Join(root, "parked"), public)
		}
	}()
	for i := 0; i < 200; i++ {
		r := run(root, s, p, q, "json")
		if r.Status == "no-findings" {
			t.Fatal("disclosed denied target")
		}
	}
	wg.Wait()
}
func BenchmarkLongLineProjection(b *testing.B) {
	root, s, p, q := fixtureB(b)
	data := []byte(strings.Repeat("X", 100_000) + "\n")
	os.WriteFile(filepath.Join(root, "a.txt"), data, 0600)
	q.Files[0].SHA256 = digest(data)
	p.Layers[0].Limits.MaxFileBytes = len(data)
	p.Layers[0].Limits.MaxTotalBytes = len(data)
	p.Layers[0].Limits.MaxOutputBytes = 500
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run(root, s, p, q, "json")
	}
}
