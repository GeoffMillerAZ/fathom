package main

// ingest_docindex_test.go — KR-8/KR-9's disclosure on the ingest summary:
// the doc-unit pass's counts, what the byte budget cut (named), what could
// not be indexed (named) and why the pass did not run, in the text summary
// and the --json envelope.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestIngestReportsDocIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "GUIDE.md"), []byte("# Guide\n\n## Steps\n\nstep text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(t.TempDir(), "s.sqlite")

	var stdout, stderr bytes.Buffer
	if code := cmdIngest([]string{"--service", "docprobe", "--path", root, "--store", store}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "docs: 1 file(s) scanned, 1 re-indexed (1 unit(s) written)") {
		t.Errorf("text summary lacks the doc-index line:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cmdIngest([]string{"--service", "docprobe", "--path", root, "--store", store, "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	var env ingestJSON
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout.String())
	}
	if env.Docs == nil || env.Docs.Files != 1 || env.Docs.Unchanged != 1 || env.Docs.Changed != 0 {
		t.Fatalf("docs = %+v, want the one file scanned and unchanged on the second run", env.Docs)
	}
}

func TestPrintDocIndexNamesElisionFailureAndSkip(t *testing.T) {
	var rep client.Report
	rep.DocIndex = docindex.IngestDocUnitsResult{
		FilesScanned: 9, FilesChanged: 2, FilesUnchanged: 7, UnitsWritten: 5,
		UnitsElided: 7, ElidedBytes: 4096,
		Failures: []docindex.DocFileFailure{{Path: "docs/huge.md", Err: "3000000 bytes is over the bound"}},
	}
	for i := 0; i < 7; i++ {
		rep.DocIndex.Elided = append(rep.DocIndex.Elided, docindex.ElidedDocUnit{Path: "docs/p" + string(rune('a'+i)) + ".md", HeadingPath: "H", Bytes: 512})
	}
	var stdout, stderr bytes.Buffer
	printDocIndex(&stdout, &stderr, rep)
	out := stdout.String()
	for _, want := range []string{"docs: 9 file(s) scanned, 2 re-indexed (5 unit(s) written), 7 unchanged, 0 removed", "7 unit(s) (4096 bytes) elided", "docs/pa.md > H", "... and 2 more"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "docs/pf.md") {
		t.Errorf("stdout names more than %d elided units inline:\n%s", docElidedShown, out)
	}
	if !strings.Contains(stderr.String(), "warning: docs: docs/huge.md not indexed: 3000000 bytes is over the bound") {
		t.Errorf("stderr lacks the named failure:\n%s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	printDocIndex(&stdout, &stderr, client.Report{DocIndexSkipped: "the store has no doc index"})
	if !strings.Contains(stdout.String(), "docs: skipped — the store has no doc index") {
		t.Errorf("a skipped pass must be named: %q", stdout.String())
	}
}
