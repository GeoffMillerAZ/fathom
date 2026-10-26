package receiptverify_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
)

// TestVerifyExportedReferenceFacts (EVX-9): a reference fact names a file,
// anchored whole; its git rung is the only check, so a file that uses the
// symbol without declaring anything verifies, and a tampered blob does not.
func TestVerifyExportedReferenceFacts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"go.mod":   "module example.invalid/refs\n\ngo 1.22\n",
		"pkg/a.go": "package pkg\n\nfunc Greet() {}\n",
		"pkg/b.go": "package pkg\n\nvar _ = Greet\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := initGitRepo(t, root)
	c, err := receipt.ResolveCommit(context.Background(), root, commit)
	if err != nil {
		t.Fatal(err)
	}
	in := receipt.ExportInput{Receipt: core.Receipt{Kind: core.ReceiptKindReferences, Symbol: "Greet", IndexCommit: commit, Files: []string{"pkg/a.go", "pkg/b.go"}}}
	for _, p := range in.Receipt.Files {
		a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: p}, receipt.Source{})
		if err != nil {
			t.Fatal(err)
		}
		in.Facts = append(in.Facts, receipt.FactInput{Kind: receipt.FactReference, Subject: p, Anchors: []receipt.CitedAnchor{{Anchor: a, Excerpt: excerpt}}})
	}
	r, err := receipt.BuildReceipt(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || !out.OK {
		t.Fatalf("references receipt: %+v, %v", out, err)
	}
	r.Facts[1].Anchors[0].BlobID = r.Facts[0].Anchors[0].BlobID
	out, err = receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || out.OK {
		t.Fatalf("a reference anchor naming another blob verified: %+v, %v", out, err)
	}
}
