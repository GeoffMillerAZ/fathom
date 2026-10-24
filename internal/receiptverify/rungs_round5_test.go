package receiptverify_test

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
)

// round5Absence is an honest absence receipt: Hello has no callers.
func round5Absence(t *testing.T) (string, receipt.ExportedReceipt) {
	t.Helper()
	root, r := rungFixture(t)
	r.Kind, r.Symbol, r.Callers = core.ReceiptKindCallers, "Hello", nil
	r.Facts = []receipt.Fact{{Kind: receipt.FactAbsence, Subject: "@::pkg/pkg.Hello", Count: "0 caller(s)", ReadSet: &receipt.CoverageManifest{
		Files: []string{"pkg/a.go"}, TreeHash: round4Git(t, root, "rev-parse", r.IndexCommit+"^{tree}"), Tool: "fathom/find_callers",
	}}}
	return root, r
}

func TestVerifyExportedAbsenceFieldForgeries(t *testing.T) {
	root, honest := round5Absence(t)
	out, err := receiptverify.VerifyExported(context.Background(), root, honest)
	if err != nil || !out.OK {
		t.Fatalf("honest absence refused: %+v, %v", out, err)
	}
	for _, name := range []string{"count", "lower-bound-no-missing", "missing-no-lower-bound", "anchor", "tree-hash", "wrong-package"} {
		t.Run(name, func(t *testing.T) {
			r := cloneRound4Receipt(t, honest)
			f := &r.Facts[0]
			switch name {
			case "count":
				f.Count = "7 caller(s)"
			case "lower-bound-no-missing":
				f.LowerBound = true
			case "missing-no-lower-bound":
				f.MissingReads = []receipt.MissingRead{{Path: "pkg/a.go", Detail: "unreadable"}}
			case "anchor":
				f.Anchors = honestAnchors(t, root, r.IndexCommit)
			case "tree-hash":
				f.ReadSet.TreeHash = strings.Repeat("0", 40)
			case "wrong-package":
				f.Subject = "@::other/other.Hello"
			}
			out, err := receiptverify.VerifyExported(context.Background(), root, r)
			if err != nil || out.OK {
				t.Fatalf("forged absence accepted: %+v, %v", out, err)
			}
		})
	}
	// An honest lower bound (a missing read, "≥ 0") still verifies.
	r := cloneRound4Receipt(t, honest)
	r.Facts[0].MissingReads = []receipt.MissingRead{{Path: "pkg/b.go", Detail: "unreadable"}}
	r.Facts[0].LowerBound, r.Facts[0].Count = true, "≥ 0 caller(s)"
	if out, err := receiptverify.VerifyExported(context.Background(), root, r); err != nil || !out.OK {
		t.Fatalf("honest lower bound refused: %+v, %v", out, err)
	}
}

func honestAnchors(t *testing.T, root, commit string) []receipt.ExportAnchor {
	t.Helper()
	c, err := receipt.ResolveCommit(context.Background(), root, commit)
	if err != nil {
		t.Fatal(err)
	}
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "pkg/a.go", StartLine: 4, EndLine: 4}, receipt.Source{})
	if err != nil {
		t.Fatal(err)
	}
	return []receipt.ExportAnchor{{Anchor: a, Excerpt: string(excerpt)}}
}

func TestVerifyExportedFingerprintMismatch(t *testing.T) {
	root, original := round4CallerFixture(t)
	r := cloneRound4Receipt(t, original)
	r.Facts[1].Anchors[0].FathomFingerprint = strings.Repeat("f", 64)
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || out.OK || !strings.Contains(strings.Join(out.Rungs, "|")+out.Diff, "fingerprint") {
		t.Fatalf("forged fingerprint accepted: %+v, %v", out, err)
	}
}
