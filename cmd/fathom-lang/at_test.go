package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func runAt(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errw bytes.Buffer
	code = run(context.Background(), append([]string{"at"}, args...), strings.NewReader(""), &out, &errw)
	return out.String(), errw.String(), code
}

type atQuery struct {
	Path      string           `json:"path"`
	Start     int              `json:"start"`
	End       int              `json:"end"`
	Status    string           `json:"status"`
	Enclosing *json.RawMessage `json:"enclosing"`
	Touched   []map[string]any `json:"touched"`
	Complete  bool             `json:"complete"`
	Error     string           `json:"error"`
}

type atDoc struct {
	SchemaVersion int       `json:"schema_version"`
	Queries       []atQuery `json:"queries"`
}

// HLP-1 (CH-D6): the shape of `at` is pinned by a binary golden. The golden includes a range query and
// fixes the envelope, the field order, null for no enclosing symbol, and one read shared by queries of a path.
// Regenerate deliberately: go test ./cmd/fathom-lang -run TestAtJSONGolden -args -update-golden
func TestAtJSONGolden(t *testing.T) {
	src := filepath.Join("testdata", "golden", "src")
	args := []string{"-root", src, "main.tf:2", "main.tf:1-3", "main.tf:4", "a.go:3", "a.go:4", "main.tf:5", "blob.bin:1", "misc.yaml:1"}
	out, errw, code := runAt(t, args...)
	if code != 0 {
		t.Fatalf("at exit %d: %s", code, errw)
	}
	checkGolden(t, "at.json", []byte(out))

	// The supervised child is the default; a batch that faults nowhere prints the same bytes in process.
	inProc, errw, code := runAt(t, append([]string{"-in-process"}, args...)...)
	if code != 0 {
		t.Fatalf("at -in-process exit %d: %s", code, errw)
	}
	if inProc != out {
		t.Errorf("supervised and -in-process output differ\n--- supervised\n%s\n--- in process\n%s", out, inProc)
	}
	again, _, _ := runAt(t, args...)
	if again != out {
		t.Error("equal inputs gave different bytes")
	}
}

// HLP-1: PATH:LINE and PATH:LINE-END. A range reports start and end, Touched over the range and
// completeness; PATH:LINE has start == end.
func TestAtRangeAndLineForms(t *testing.T) {
	src := filepath.Join("testdata", "golden", "src")
	out, errw, code := runAt(t, "-in-process", "-root", src, "main.tf:1-5", "main.tf:3", "a.go:1-2")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errw)
	}
	var doc atDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if doc.SchemaVersion != 1 || len(doc.Queries) != 3 {
		t.Fatalf("%+v", doc)
	}
	keysOf := func(q atQuery) []string {
		var ks []string
		for _, s := range q.Touched {
			ks = append(ks, s["key"].(string))
		}
		return ks
	}
	q := doc.Queries
	if q[0].Path != "main.tf" || q[0].Start != 1 || q[0].End != 5 || !q[0].Complete ||
		!reflect.DeepEqual(keysOf(q[0]), []string{"main.aws_s3_bucket.b", "main.var.name"}) {
		t.Errorf("range query: %+v", q[0])
	}
	if q[1].Start != 3 || q[1].End != 3 || q[1].Status != "found" || !reflect.DeepEqual(keysOf(q[1]), []string{"main.aws_s3_bucket.b"}) {
		t.Errorf("line query: %+v", q[1])
	}
	if q[2].Path != "a.go" || q[2].Start != 1 || q[2].End != 2 || len(q[2].Touched) != 0 || !q[2].Complete {
		t.Errorf("range before any symbol: %+v", q[2])
	}
	for i, x := range q {
		if x.Error != "" {
			t.Errorf("query %d error %q", i, x.Error)
		}
	}
}

// HLP-1: no source text is printed (CH-D4): the output holds no line of the file, only the Result's own symbol fields.
func TestAtPrintsNoSourceText(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.go", "package a\n\n// zzq-sentinel-comment\nfunc F() {\n\tprintln(\"zzq-sentinel-literal\")\n}\n")
	out, errw, code := runAt(t, "-in-process", "-root", root, "a.go:4-6", "a.go:5")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errw)
	}
	if strings.Contains(out, "zzq-sentinel") {
		t.Errorf("at printed source text:\n%s", out)
	}
	if !strings.Contains(out, `"key": "a.F"`) {
		t.Errorf("expected the enclosing symbol F:\n%s", out)
	}
}

// HLP-1: several queries on one path share one read; each path is read once.
func TestAtReadsEachPathOnce(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.go", "package a\n\nfunc F() {}\n\nfunc G() {}\n")
	write(t, root, "b.go", "package b\n\nfunc H() {}\n")
	counts := map[string]int{}
	beforeRead = func(p string) { counts[p]++ }
	defer func() { beforeRead = nil }()
	out, errw, code := runAt(t, "-in-process", "-root", root, "a.go:3", "b.go:3", "a.go:5", "a.go:1-5", "b.go:1", "a.go:4")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errw)
	}
	if counts["a.go"] != 1 || counts["b.go"] != 1 || len(counts) != 2 {
		t.Errorf("reads per path = %v, want a.go and b.go once each", counts)
	}
	var doc atDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, q := range doc.Queries {
		order = append(order, q.Path)
	}
	if !reflect.DeepEqual(order, []string{"a.go", "b.go", "a.go", "a.go", "b.go", "a.go"}) {
		t.Errorf("queries are answered in argument order, got %v", order)
	}
}

// HLP-1: exit codes as extract. An unreadable file is an entry with an error and the run exits 1, the
// batch goes on; misuse exits 2.
func TestAtExitCodes(t *testing.T) {
	src := filepath.Join("testdata", "golden", "src")
	out, _, code := runAt(t, "-in-process", "-root", src, "nope.go:1", "main.tf:2")
	if code != 1 {
		t.Errorf("unreadable file: exit %d, want 1", code)
	}
	var doc atDoc
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("an unreadable file still prints JSON: %v\n%s", err, out)
	}
	if len(doc.Queries) != 2 || doc.Queries[0].Error == "" || doc.Queries[0].Enclosing != nil || doc.Queries[0].Complete ||
		doc.Queries[0].Status != "unknown" || doc.Queries[1].Error != "" || doc.Queries[1].Status != "found" {
		t.Errorf("%+v", doc.Queries)
	}
	for _, c := range []struct {
		name string
		args []string
	}{
		{"no query", []string{"-root", src}},
		{"no colon", []string{"-root", src, "main.tf"}},
		{"line is not a number", []string{"-root", src, "main.tf:x"}},
		{"line is negative", []string{"-root", src, "main.tf:-1"}},
		{"line is zero", []string{"-root", src, "main.tf:0"}},
		{"end is not a number", []string{"-root", src, "main.tf:2-y"}},
		{"end is before start", []string{"-root", src, "main.tf:5-2"}},
		{"unknown flag", []string{"-nope", "main.tf:1"}},
	} {
		if o, e, code := runAt(t, c.args...); code != 2 {
			t.Errorf("%s: exit %d, want 2 (stdout %q stderr %q)", c.name, code, o, e)
		}
	}
	if _, _, code := runAt(t, "-deadline", "0s", "-root", src, "main.tf:1"); code != 2 {
		t.Errorf("-deadline 0s: exit %d, want 2 as extract", code)
	}
}

// CH-D6: the new surface is named in the usage text and marked provisional.
func TestAtUsageMarksProvisional(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run(context.Background(), []string{"help"}, strings.NewReader(""), &out, &errw); code != 0 {
		t.Fatalf("help exit %d", code)
	}
	u := out.String()
	if !strings.Contains(u, "fathom-lang at") || !strings.Contains(u, "PATH:LINE") {
		t.Errorf("usage does not describe the at verb:\n%s", u)
	}
	i := strings.Index(u, "fathom-lang at")
	if i >= 0 && !strings.Contains(strings.ToLower(u[i:]), "provisional") {
		t.Error("usage does not mark at as provisional")
	}
}
