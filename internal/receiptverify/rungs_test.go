package receiptverify_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func rungFixture(t *testing.T) (string, receipt.ExportedReceipt) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"go.mod":   "module example.invalid/rungs\n\ngo 1.22\n",
		"pkg/a.go": "package pkg\n\nfunc Greet() {}\nfunc Hello() { Greet() }\n",
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
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "pkg/a.go", StartLine: 3, EndLine: 3}, receipt.Source{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := receipt.BuildReceipt(receipt.ExportInput{
		Receipt: core.Receipt{Kind: core.ReceiptKindSymbolLocation, Symbol: "Greet", IndexCommit: commit, File: "pkg/a.go", Line: 3},
		Facts:   []receipt.FactInput{{Kind: receipt.FactDefinition, Subject: "@::pkg/pkg.Greet", Anchors: []receipt.CitedAnchor{{Anchor: a, Excerpt: excerpt}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, r
}

func TestVerifyExportedRungs(t *testing.T) {
	root, original := rungFixture(t)
	cases := []struct {
		name           string
		change         func(*receipt.ExportedReceipt)
		ok             bool
		git, toolchain string
	}{
		{"confirmed", func(*receipt.ExportedReceipt) {}, true, "git rung: confirmed", "toolchain rung: confirmed"},
		{"edited blob", func(r *receipt.ExportedReceipt) { r.Facts[0].Anchors[0].BlobID = strings.Repeat("0", 40) }, false, "git rung: refuted", "toolchain rung: confirmed"},
		{"edited excerpt", func(r *receipt.ExportedReceipt) { r.Facts[0].Anchors[0].Excerpt = "func Forged() {}\n" }, false, "git rung: refuted", "toolchain rung: confirmed"},
		{"edited digest", func(r *receipt.ExportedReceipt) { r.Facts[0].Anchors[0].ExcerptSHA256 = strings.Repeat("0", 64) }, false, "git rung: refuted", "toolchain rung: confirmed"},
		{"wrong line", func(r *receipt.ExportedReceipt) {
			r.Facts[0].Anchors[0].StartLine = 4
			r.Facts[0].Anchors[0].EndLine = 4
		}, false, "git rung: refuted", "toolchain rung: refuted"},

		{"wrong subject", func(r *receipt.ExportedReceipt) { r.Facts[0].Subject = "@::pkg/pkg.Missing" }, false, "git rung: confirmed", "toolchain rung: refuted"},
		{"withheld", func(r *receipt.ExportedReceipt) { r.Facts[0].Anchors[0].State = receipt.StateWithheld }, false, "git rung: refuted", "toolchain rung: confirmed"},
		{"filtered", func(r *receipt.ExportedReceipt) { r.Facts[0].Anchors[0].State = receipt.StateFiltered }, true, "git rung: confirmed", "toolchain rung: confirmed"},
		{"uncommitted", func(r *receipt.ExportedReceipt) { r.Facts[0].Anchors[0].State = receipt.StateUncommitted }, false, "no git rung (uncommitted)", "no toolchain rung (uncommitted)"},
		{"digest only", func(r *receipt.ExportedReceipt) { r.Facts[0].Anchors[0].Excerpt = "" }, true, "git rung: confirmed", "toolchain rung: confirmed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(original)
			var r receipt.ExportedReceipt
			if err := json.Unmarshal(data, &r); err != nil {
				t.Fatal(err)
			}
			tc.change(&r)
			out, err := receiptverify.VerifyExported(context.Background(), root, r)
			if err != nil {
				t.Fatal(err)
			}
			if out.OK != tc.ok {
				t.Errorf("OK=%v, want %v: %+v", out.OK, tc.ok, out)
			}
			text := fmt.Sprint(out.Rungs)
			for _, want := range []string{"definition @::pkg/pkg.", tc.git, tc.toolchain} {
				if !strings.Contains(text, want) {
					t.Errorf("rungs %s missing %q", text, want)
				}
			}
			if strings.Index(text, tc.git) > strings.Index(text, tc.toolchain) {
				t.Errorf("git must precede toolchain: %s", text)
			}
		})
	}
	// The work tree is irrelevant: anchors name the committed blob.
	if err := os.WriteFile(filepath.Join(root, "pkg/a.go"), []byte("edited work tree"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := receiptverify.VerifyExported(context.Background(), root, original)
	if err != nil || !out.OK {
		t.Fatalf("committed anchor depended on work tree: %+v, %v", out, err)
	}
}

func TestVerifyExportedCallersRoundTrip(t *testing.T) {
	root, r := rungFixture(t)
	c, err := receipt.ResolveCommit(context.Background(), root, r.IndexCommit)
	if err != nil {
		t.Fatal(err)
	}
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "pkg/a.go", StartLine: 4, EndLine: 4}, receipt.Source{})
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range []string{"@", "generated"} {
		t.Run(service, func(t *testing.T) {
			caller := service + "::pkg/pkg.Hello"
			built, err := receipt.BuildReceipt(receipt.ExportInput{
				Receipt: core.Receipt{Kind: core.ReceiptKindCallers, Symbol: "Greet", IndexCommit: r.IndexCommit, Callers: []string{caller}},
				Facts:   []receipt.FactInput{{Kind: receipt.FactCaller, Subject: caller, Anchors: []receipt.CitedAnchor{{Anchor: a, Excerpt: excerpt}}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(built)
			if err != nil {
				t.Fatal(err)
			}
			var saved receipt.ExportedReceipt
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			out, err := receiptverify.VerifyExported(context.Background(), root, saved)
			if err != nil || !out.OK {
				t.Fatalf("round trip: %+v, %v", out, err)
			}
			if !strings.Contains(fmt.Sprint(out.Rungs), "caller "+caller+": toolchain rung: confirmed") {
				t.Fatalf("caller not confirmed: %+v", out)
			}
			saved.Callers = []string{service + "::other/pkg.Hello"}
			out, err = receiptverify.VerifyExported(context.Background(), root, saved)
			if err != nil || out.OK {
				t.Fatalf("wrong package accepted: %+v, %v", out, err)
			}
			saved.Callers = []string{caller}
			saved.Facts[0].Subject = service + "::pkg/pkg.Greet"
			out, err = receiptverify.VerifyExported(context.Background(), root, saved)
			if err != nil || out.OK || !strings.Contains(fmt.Sprint(out.Rungs), "toolchain rung: refuted") {
				t.Fatalf("non-caller fact accepted: %+v, %v", out, err)
			}
		})
	}
}

func TestVerifyExportedBoundedAndWholeFileAnchors(t *testing.T) {
	root, r := rungFixture(t)
	body, err := os.ReadFile(filepath.Join(root, "pkg/a.go"))
	if err != nil {
		t.Fatal(err)
	}
	a := &r.Facts[0].Anchors[0]
	a.EndLine = 4
	a.ExcerptEndLine = 3
	a.ExcerptSHA256, _ = verify.ExcerptSHA256(body, 3, 4)
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || !out.OK {
		t.Fatalf("bounded excerpt: %+v, %v", out, err)
	}
	a.StartLine, a.EndLine, a.ExcerptEndLine = 0, 0, 0
	a.Excerpt = string(body)
	a.ExcerptSHA256, _ = verify.ExcerptSHA256(body, 0, 0)
	out, err = receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || !out.OK {
		t.Fatalf("whole file: %+v, %v", out, err)
	}
}

func TestVerifyExportedRecordedBlobEdited(t *testing.T) {
	root, r := rungFixture(t)
	if err := os.WriteFile(filepath.Join(root, "pkg/a.go"), []byte("package pkg\n\nfunc Greet() { /* changed */ }\nfunc Hello() { Greet() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Facts[0].Anchors[0].Commit = initGitRepo(t, root)
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || out.OK || !strings.Contains(fmt.Sprint(out.Rungs), "git rung: refuted") {
		t.Fatalf("edited recorded blob: %+v, %v", out, err)
	}
}
