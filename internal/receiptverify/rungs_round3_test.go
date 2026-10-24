package receiptverify_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
)

func TestVerifyExportedUncommittedCallersStillChecksEnvelope(t *testing.T) {
	for _, all := range []bool{false, true} {
		name := "mixed"
		if all {
			name = "all"
		}
		t.Run(name, func(t *testing.T) {
			root, r := rungFixture(t)
			r.Kind = core.ReceiptKindCallers
			r.Callers = nil // Forged: Hello still calls Greet at IndexCommit.
			if all {
				r.Facts[0].Anchors[0].State = receipt.StateUncommitted
			} else {
				r.Facts = append(r.Facts, receipt.Fact{Kind: receipt.FactDefinition, Subject: "@::pkg/pkg.New", Anchors: []receipt.ExportAnchor{{Anchor: receipt.Anchor{State: receipt.StateUncommitted, Path: "pkg/new.go", StartLine: 2}}}})
			}
			out, err := receiptverify.VerifyExported(context.Background(), root, r)
			if err != nil || out.OK || !strings.Contains(out.Diff, "not reproduced (1 unchecked: uncommitted)") || !strings.Contains(out.Diff, "callers of Greet: receipt names 0") || !strings.Contains(out.Diff, "Hello") {
				t.Fatalf("forged callers envelope was not refuted: %+v, %v", out, err)
			}
		})
	}
}

func TestVerifyExportedForgedIndexCommit(t *testing.T) {
	root, r := rungFixture(t)
	r.Kind, r.Callers = core.ReceiptKindCallers, []string{"Hello"}
	if err := os.WriteFile(filepath.Join(root, "pkg/a.go"), []byte("package pkg\n\nfunc Greet() {}\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.IndexCommit = initGitRepo(t, root)
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || out.OK || !strings.Contains(out.Diff, "anchor commit differs from index_commit") {
		t.Fatalf("forged index_commit was not refuted: %+v, %v", out, err)
	}
}

func TestVerifyExportedTestMainAbsence(t *testing.T) {
	root, r := rungFixture(t)
	body := "package pkg\nimport \"testing\"\nfunc TestMain(m *testing.M) { m.Run() }\n"
	if err := os.WriteFile(filepath.Join(root, "pkg/a_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Kind, r.Symbol, r.IndexCommit = core.ReceiptKindCallers, "TestMain", initGitRepo(t, root)
	r.Facts = []receipt.Fact{{Kind: receipt.FactAbsence, Subject: "TestMain", ReadSet: &receipt.CoverageManifest{
		Files: []string{"pkg/a.go", "pkg/a_test.go"}, TreeHash: round4Git(t, root, "rev-parse", "HEAD^{tree}"), Tool: "fathom/find_callers",
	}}}
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || !out.OK {
		t.Fatalf("generated runner refuted an honest absence: %+v, %v", out, err)
	}
}
