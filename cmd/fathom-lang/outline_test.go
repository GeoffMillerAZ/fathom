package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

func runOutline(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errw bytes.Buffer
	code = run(context.Background(), append([]string{"outline"}, args...), strings.NewReader(""), &out, &errw)
	return out.String(), errw.String(), code
}

// A level is what a reading measured, so the goldens pin the field and not the state of a reading.
var (
	levelText = regexp.MustCompile(`level=(L[1-4]|none)`)
	levelJSON = regexp.MustCompile(`"level": "(L[1-4]|none)"`)
)

// normalize replaces what moves between runs and machines: a measured level and the absolute root that an
// operating system error names.
func normalize(t *testing.T, out string, level *regexp.Regexp, with string) []byte {
	t.Helper()
	abs, err := filepath.Abs(goldenSrc)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(strings.ReplaceAll(level.ReplaceAllString(out, with), filepath.ToSlash(abs), "<root>"))
}

// HLP-2 (CH-D6): the text and JSON forms of outline are pinned by binary goldens.
// Regenerate deliberately: go test ./cmd/fathom-lang -run TestOutline -args -update-golden
func TestOutlineTextGolden(t *testing.T) {
	args := append([]string{"-root", goldenSrc}, append([]string{"main.tf", "a.go", "blob.bin", "misc.yaml"}, "missing.go")...)
	out, _, code := runOutline(t, args...)
	if code != 1 {
		t.Fatalf("exit %d, want 1 for the file that cannot be read", code)
	}
	checkGolden(t, "outline.txt", normalize(t, out, levelText, "level=<level>"))
	inProc, _, code := runOutline(t, append([]string{"-in-process"}, args...)...)
	if code != 1 || inProc != out {
		t.Errorf("supervised and -in-process text differ (exit %d)\n--- supervised\n%s\n--- in process\n%s", code, out, inProc)
	}
	if again, _, _ := runOutline(t, args...); again != out {
		t.Error("equal inputs gave different bytes")
	}
}

func TestOutlineJSONGolden(t *testing.T) {
	args := []string{"-format", "json", "-root", goldenSrc, "main.tf", "a.go", "blob.bin", "misc.yaml", "missing.go"}
	out, _, code := runOutline(t, args...)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	checkGolden(t, "outline.json", normalize(t, out, levelJSON, `"level": "<level>"`))
	inProc, _, _ := runOutline(t, append([]string{"-in-process"}, args...)...)
	if inProc != out {
		t.Error("supervised and -in-process JSON differ")
	}
	var doc struct {
		SchemaVersion int `json:"schema_version"`
		Files         []struct {
			Path    string           `json:"path"`
			Error   string           `json:"error"`
			Outline *json.RawMessage `json:"outline"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != 1 || len(doc.Files) != 5 {
		t.Fatalf("%+v", doc)
	}
	if last := doc.Files[4]; last.Error == "" || last.Outline != nil {
		t.Errorf("an unreadable file has an error and a null outline, got %+v", last)
	}
	if doc.Files[0].Outline == nil || doc.Files[0].Error != "" {
		t.Errorf("a readable file has an outline and no error, got %+v", doc.Files[0])
	}
}

// -max-nodes bounds the outline and the last line says how many were dropped.
func TestOutlineMaxNodesFlag(t *testing.T) {
	out, errw, code := runOutline(t, "-root", goldenSrc, "-max-nodes", "1", "a.go")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errw)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 3 || !strings.HasSuffix(lines[2], " omitted=1") {
		t.Errorf("-max-nodes 1 output:\n%s", out)
	}
	for _, bad := range []string{"-1", "x"} {
		if _, _, code := runOutline(t, "-root", goldenSrc, "-max-nodes", bad, "a.go"); code != 2 {
			t.Errorf("-max-nodes %s exit %d, want 2", bad, code)
		}
	}
}

// outline is view.Outline of the same read extract makes, in the text and JSON forms.
func TestOutlineEqualsViewOutlineOfExtract(t *testing.T) {
	files := []string{"main.tf", "a.go", "misc.yaml"}
	nested, _, code := runCmd(t, append([]string{"extract", "-root", goldenSrc}, files...)...)
	if code != 0 {
		t.Fatal(code)
	}
	var doc struct {
		Files []struct {
			Path   string      `json:"path"`
			Result lang.Result `json:"result"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(nested), &doc); err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	for _, f := range doc.Files {
		want.WriteString(view.Outline(f.Path, f.Result, view.OutlineOptions{}).Text())
	}
	got, _, code := runOutline(t, append([]string{"-root", goldenSrc}, files...)...)
	if code != 0 || got != want.String() {
		t.Errorf("outline text differs from view.Outline of extract (exit %d)\n--- got\n%s\n--- want\n%s", code, got, want.String())
	}
}

func TestOutlineMisuse(t *testing.T) {
	if _, _, code := runOutline(t); code != 2 {
		t.Errorf("no PATH exit %d, want 2", code)
	}
	if _, _, code := runOutline(t, "-format", "xml", "a.go"); code != 2 {
		t.Errorf("-format xml exit %d, want 2", code)
	}
	if _, _, code := runOutline(t, "-deadline", "0s", "a.go"); code != 2 {
		t.Errorf("-deadline 0s exit %d, want 2", code)
	}
}

// CH-D6: the usage text names the verb and marks it provisional.
func TestOutlineUsageMarksProvisional(t *testing.T) {
	out, _, _ := runCmd(t, "help")
	if !strings.Contains(out, "fathom-lang outline [-root DIR] [-format text|json] [-max-nodes N]") {
		t.Error("usage lacks the outline synopsis")
	}
	i := strings.Index(out, "fathom-lang outline (provisional")
	if i < 0 {
		t.Fatal("usage has no paragraph starting fathom-lang outline (provisional")
	}
}

// A nested file: a class holds its methods and a method its inner function. -max-nodes drops the deepest level
// first and the footer says how many went.
func TestOutlineNestedGolden(t *testing.T) {
	src := filepath.Join("testdata", "outline", "src")
	for _, c := range []struct{ golden, max string }{{"outline-nested.txt", "0"}, {"outline-nested-max4.txt", "4"}, {"outline-nested-max1.txt", "1"}} {
		out, errw, code := runOutline(t, "-root", src, "-max-nodes", c.max, "nest.py")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, errw)
		}
		checkGolden(t, c.golden, levelText.ReplaceAll([]byte(out), []byte("level=<level>")))
	}
	out, _, _ := runOutline(t, "-root", src, "nest.py")
	if !strings.Contains(out, "\n  ") || !strings.Contains(out, "\n    ") {
		t.Errorf("no nesting in:\n%s", out)
	}
}

// A Go Signature carries raw newlines and tabs. The text form stays one line per node (lines equal nodes plus
// the header and the footer), while the JSON form keeps the signature unchanged (CH-D4).
func TestOutlineMultiLineSignatureGolden(t *testing.T) {
	src := filepath.Join("testdata", "outline", "src")
	out, errw, code := runOutline(t, "-root", src, "multi.go")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errw)
	}
	checkGolden(t, "outline-multiline.txt", levelText.ReplaceAll([]byte(out), []byte("level=<level>")))
	js, _, _ := runOutline(t, "-root", src, "-format", "json", "multi.go")
	var doc struct {
		Files []struct {
			Outline view.OutlineDoc `json:"outline"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(js), &doc); err != nil {
		t.Fatal(err)
	}
	nodes, raw := 0, false
	var walk func(ns []view.OutlineNode)
	walk = func(ns []view.OutlineNode) {
		for _, n := range ns {
			nodes++
			raw = raw || strings.Contains(n.Signature, "\n")
			walk(n.Children)
		}
	}
	walk(doc.Files[0].Outline.Nodes)
	if !raw {
		t.Error("no node kept a raw newline in its JSON signature")
	}
	if lines := strings.Count(out, "\n"); lines != nodes+2 {
		t.Errorf("%d text lines for %d nodes, want nodes+2:\n%s", lines, nodes, out)
	}
}
