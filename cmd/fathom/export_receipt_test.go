package main

// export_receipt_test.go — EVX-7's product path (docs/plans/
// portable-receipts.md): `fathom export receipt` builds through
// receipt.BuildReceipt, so an exported receipt is the core.Receipt JSON
// `fathom receipt verify` reads, with anchored facts beside it.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/receipt"
)

const exportReceiptFixtureSrc = `package pkg

// Greet says hello.
func Greet() string { return "hi" }

// Hello calls Greet.
func Hello() string {
	return Greet()
}

// Lonely has no callers.
func Lonely() {}
`

func exportReceiptFixture(t *testing.T) (root, storePath string) {
	t.Helper()
	t.Setenv("FATHOM_HOME", t.TempDir()) // no user-layer suppression reaches the run
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module example.invalid/rcpt\n\ngo 1.22\n")
	repo.writePlan("pkg/a.go", exportReceiptFixtureSrc)
	repo.commitAll("a")
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	var out, errBuf bytes.Buffer
	if code := run([]string{"ingest", "--service", "rcpt", "--path", repo.root, "--store", storePath}, &out, &errBuf); code != 0 {
		t.Fatalf("ingest exit %d: %s", code, errBuf.String())
	}
	return repo.root, storePath
}

func exportReceipt(t *testing.T, storePath string, args ...string) (receipt.ExportedReceipt, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	if code := run(append([]string{"export", "receipt", "--service", "rcpt", "--store", storePath}, args...), &out, &errBuf); code != 0 {
		t.Fatalf("export receipt %v exit %d: %s", args, code, errBuf.String())
	}
	var r receipt.ExportedReceipt
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("export receipt %v wrote no receipt JSON: %v\n%s", args, err, out.String())
	}
	return r, out.String()
}

// TestCmdExportReceipt: a definition, a caller and an absence, each built
// through BuildReceipt; --no-excerpts keeps digests only; the exported file
// verifies with `fathom receipt verify`.
func TestCmdExportReceipt(t *testing.T) {
	root, storePath := exportReceiptFixture(t)
	head := gitHead(t, root)

	def, raw := exportReceipt(t, storePath, "--symbol", "Greet")
	if def.Kind != "symbol_location" || def.File != "pkg/a.go" || def.Line != 4 || def.IndexCommit != head {
		t.Fatalf("symbol_location receipt = %+v, want pkg/a.go:4 at %s", def.Receipt, head)
	}
	if len(def.Facts) != 1 || def.Facts[0].Kind != receipt.FactDefinition || len(def.Facts[0].Anchors) != 1 {
		t.Fatalf("facts = %+v, want one anchored definition", def.Facts)
	}
	a := def.Facts[0].Anchors[0]
	if a.State != receipt.StateCommitted || a.BlobID == "" || a.ExcerptSHA256 == "" || a.FathomFingerprint == "" {
		t.Errorf("definition anchor = %+v, want a committed anchor with blob id, digest and fingerprint", a)
	}
	if !strings.Contains(a.Excerpt, "func Greet()") {
		t.Errorf("excerpt = %q, want the definition's line", a.Excerpt)
	}
	if strings.Contains(raw, root) {
		t.Errorf("the receipt names the absolute work tree %s:\n%s", root, raw)
	}

	_, bare := exportReceipt(t, storePath, "--symbol", "Greet", "--no-excerpts")
	if strings.Contains(bare, `"excerpt"`) || !strings.Contains(bare, `"excerpt_sha256": "`+a.ExcerptSHA256+`"`) {
		t.Errorf("--no-excerpts receipt carries an excerpt or lost the digest:\n%s", bare)
	}

	callers, _ := exportReceipt(t, storePath, "--symbol", "Greet", "--kind", "callers")
	if len(callers.Callers) != 1 || !strings.HasSuffix(callers.Callers[0], "Hello") {
		t.Fatalf("callers = %v, want Hello", callers.Callers)
	}
	if len(callers.Facts) != 1 || callers.Facts[0].Kind != receipt.FactCaller || !strings.Contains(callers.Facts[0].Anchors[0].Excerpt, "return Greet()") {
		t.Errorf("caller facts = %+v, want Hello's anchored definition", callers.Facts)
	}

	none, _ := exportReceipt(t, storePath, "--symbol", "Lonely", "--kind", "callers")
	if len(none.Callers) != 0 || len(none.Facts) != 1 || none.Facts[0].Kind != receipt.FactAbsence {
		t.Fatalf("no-caller receipt = %+v, want one absence fact", none)
	}
	f := none.Facts[0]
	if f.ReadSet == nil || f.ReadSet.TreeHash == "" || f.ReadSet.Tool == "" || !slices.Contains(f.ReadSet.Files, "pkg/a.go") {
		t.Errorf("absence read set = %+v, want the commit's tree, the tool and pkg/a.go read", f.ReadSet)
	}
	if f.LowerBound || f.Count != "0 caller(s)" {
		t.Errorf("a complete read set counts %q (lower_bound %v), want 0 caller(s)", f.Count, f.LowerBound)
	}

	// The exported file is the receipt `fathom receipt verify` reads.
	path := filepath.Join(t.TempDir(), "greet.json")
	var out, errBuf bytes.Buffer
	if code := run([]string{"export", "receipt", "--service", "rcpt", "--store", storePath, "--symbol", "Greet", "-o", path}, &out, &errBuf); code != 0 {
		t.Fatalf("export receipt -o exit %d: %s", code, errBuf.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("-o wrote no file: %v", err)
	}
	out.Reset()
	errBuf.Reset()
	if code := run([]string{"receipt", "verify", "--repo", root, path}, &out, &errBuf); code != 0 {
		t.Errorf("receipt verify of an exported receipt exit %d: %s%s", code, out.String(), errBuf.String())
	}
}

// TestCmdExportReceiptRefusesUnknownKind: a kind the export does not build
// is a usage error, never an empty receipt.
func TestCmdExportReceiptRefusesUnknownKind(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := run([]string{"export", "receipt", "--service", "rcpt", "--symbol", "Greet", "--kind", "judgment"}, &out, &errBuf); code == 0 {
		t.Fatalf("an unknown kind exported: %s", out.String())
	}
	if out.Len() != 0 {
		t.Errorf("a refused export wrote %q", out.String())
	}
}
