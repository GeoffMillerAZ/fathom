package receiptverify_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
)

func TestBareSymbolNamePreservesMethods(t *testing.T) {
	for _, name := range []string{"T.M", "T.P", "S.G"} {
		for _, prefix := range []string{"", "@::pkg/T.", "generated::pkg/T."} {
			if got := receiptverify.BareSymbolName(prefix + name); got != name {
				t.Errorf("BareSymbolName(%q) = %q, want %q", prefix+name, got, name)
			}
		}
	}
}

func TestVerifyExportedUncommittedAnchors(t *testing.T) {
	for _, kind := range []core.ReceiptKind{core.ReceiptKindSymbolLocation, core.ReceiptKindCallers} {
		t.Run(string(kind), func(t *testing.T) {
			root, r := rungFixture(t)
			r.Kind, r.Symbol, r.File, r.Line = kind, "New", "pkg/new.go", 2
			r.Facts[0].Subject = "@::pkg/pkg.New"
			a := &r.Facts[0].Anchors[0]
			a.State, a.Path, a.StartLine, a.EndLine = receipt.StateUncommitted, "pkg/new.go", 2, 2
			if kind == core.ReceiptKindCallers {
				r.Symbol = "Greet"
				r.Callers = []string{r.Facts[0].Subject}
				r.Facts[0].Kind = receipt.FactCaller
			}
			out, err := receiptverify.VerifyExported(context.Background(), root, r)
			text := fmt.Sprint(out.Rungs)
			if err != nil || out.OK || !strings.Contains(text, "no toolchain rung (uncommitted)") || strings.Contains(text, "toolchain rung: confirmed") {
				t.Fatalf("uncommitted fact read committed bytes: %+v, %v", out, err)
			}
		})
	}
}

func TestVerifyExportedMixedUncommittedAnchors(t *testing.T) {
	root, r := rungFixture(t)
	r.Facts = append(r.Facts, receipt.Fact{Kind: receipt.FactDefinition, Subject: "@::pkg/pkg.New", Anchors: []receipt.ExportAnchor{{Anchor: receipt.Anchor{State: receipt.StateUncommitted, Path: "pkg/new.go", StartLine: 2}}}})
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	text := fmt.Sprint(out.Rungs)
	if err != nil || out.OK || !strings.Contains(out.Diff, "not reproduced (1 unchecked: uncommitted)") || !strings.Contains(text, "definition @::pkg/pkg.New: no toolchain rung (uncommitted)") || !strings.Contains(text, "definition @::pkg/pkg.Greet: toolchain rung: confirmed") {
		t.Fatalf("mixed anchors: %+v, %v", out, err)
	}
}

func TestVerifyExportedAnchorCommitMismatch(t *testing.T) {
	root, r := rungFixture(t)
	if err := os.WriteFile(filepath.Join(root, "extra.txt"), []byte("next commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next := initGitRepo(t, root)
	c, err := receipt.ResolveCommit(context.Background(), root, next)
	if err != nil {
		t.Fatal(err)
	}
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "pkg/a.go", StartLine: 3, EndLine: 3}, receipt.Source{})
	if err != nil {
		t.Fatal(err)
	}
	r.Facts[0].Anchors[0] = receipt.ExportAnchor{Anchor: a, Excerpt: string(excerpt)}
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	text := fmt.Sprint(out.Rungs)
	if err != nil || out.OK || !strings.Contains(text, "git rung: confirmed") || !strings.Contains(out.Diff, "anchor commit differs from index_commit") {
		t.Fatalf("mismatched commits: %+v, %v", out, err)
	}
}

func TestVerifyExportedMethodLocations(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for file, body := range map[string]string{
		"go.mod":   "module example.invalid/methods\n\ngo 1.22\n",
		"pkg/a.go": "package pkg\ntype T struct{}\ntype S[X any] struct{}\nfunc (T) M() {}\nfunc (*T) P() {}\nfunc (S[X]) G() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := initGitRepo(t, root)
	c, err := receipt.ResolveCommit(context.Background(), root, commit)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"T.M", "T.P", "S.G"} {
		t.Run(name, func(t *testing.T) {
			a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "pkg/a.go", StartLine: i + 4, EndLine: i + 4}, receipt.Source{})
			if err != nil {
				t.Fatal(err)
			}
			subject := "@::pkg/" + strings.Split(name, ".")[0] + "." + name
			r, err := receipt.BuildReceipt(receipt.ExportInput{Receipt: core.Receipt{Kind: core.ReceiptKindSymbolLocation, Symbol: name, File: "pkg/a.go", Line: i + 4, IndexCommit: commit}, Facts: []receipt.FactInput{{Kind: receipt.FactDefinition, Subject: subject, Anchors: []receipt.CitedAnchor{{Anchor: a, Excerpt: excerpt}}}}})
			if err != nil {
				t.Fatal(err)
			}
			out, err := receiptverify.VerifyExported(context.Background(), root, r)
			if err != nil || !out.OK || !strings.Contains(fmt.Sprint(out.Rungs), "toolchain rung: confirmed") {
				t.Fatalf("method: %+v, %v", out, err)
			}
			r.Facts[0].Subject = "@::other/T." + name
			out, err = receiptverify.VerifyExported(context.Background(), root, r)
			if err != nil || out.OK {
				t.Fatalf("wrong method package accepted: %+v, %v", out, err)
			}
		})
	}
}

func TestVerifyExportedTestMainOutsideRepo(t *testing.T) {
	root, r := rungFixture(t)
	body := "package pkg\nimport \"testing\"\nfunc TestMain(m *testing.M) { m.Run() }\nfunc Runner(m *testing.M) { TestMain(m) }\n"
	if err := os.WriteFile(filepath.Join(root, "pkg/a_test.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := initGitRepo(t, root)
	c, err := receipt.ResolveCommit(context.Background(), root, commit)
	if err != nil {
		t.Fatal(err)
	}
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "pkg/a_test.go", StartLine: 4, EndLine: 4}, receipt.Source{})
	if err != nil {
		t.Fatal(err)
	}
	r, err = receipt.BuildReceipt(receipt.ExportInput{Receipt: core.Receipt{Kind: core.ReceiptKindCallers, Symbol: "TestMain", IndexCommit: commit, Callers: []string{"@::pkg/pkg.Runner"}}, Facts: []receipt.FactInput{{Kind: receipt.FactCaller, Subject: "@::pkg/pkg.Runner", Anchors: []receipt.CitedAnchor{{Anchor: a, Excerpt: excerpt}}}}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || !out.OK {
		t.Fatalf("generated go test caller must be skipped: %+v, %v", out, err)
	}
}

func TestVerifyExportedMethodCallers(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for file, body := range map[string]string{
		"go.mod":   "module example.invalid/callers\n\ngo 1.22\n",
		"pkg/a.go": "package pkg\nfunc Greet() {}\ntype T struct{}\ntype S[X any] struct{}\nfunc (T) M() { Greet() }\nfunc (*T) P() { Greet() }\nfunc (S[X]) G() { Greet() }\n",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := initGitRepo(t, root)
	c, err := receipt.ResolveCommit(context.Background(), root, commit)
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range []string{"@", "generated"} {
		t.Run(service, func(t *testing.T) {
			in := receipt.ExportInput{Receipt: core.Receipt{Kind: core.ReceiptKindCallers, Symbol: "Greet", IndexCommit: commit}}
			for i, name := range []string{"T.M", "T.P", "S.G"} {
				a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "pkg/a.go", StartLine: i + 5, EndLine: i + 5}, receipt.Source{})
				if err != nil {
					t.Fatal(err)
				}
				subject := service + "::pkg/" + strings.Split(name, ".")[0] + "." + name
				in.Receipt.Callers = append(in.Receipt.Callers, subject)
				in.Facts = append(in.Facts, receipt.FactInput{Kind: receipt.FactCaller, Subject: subject, Anchors: []receipt.CitedAnchor{{Anchor: a, Excerpt: excerpt}}})
			}
			r, err := receipt.BuildReceipt(in)
			if err != nil {
				t.Fatal(err)
			}
			out, err := receiptverify.VerifyExported(context.Background(), root, r)
			if err != nil || !out.OK || strings.Count(fmt.Sprint(out.Rungs), "toolchain rung: confirmed") != 3 {
				t.Fatalf("method callers: %+v, %v", out, err)
			}
		})
	}
}
