package receiptverify_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/receiptverify"
)

func round4CallerFixture(t *testing.T) (string, receipt.ExportedReceipt) {
	t.Helper()
	root, r := rungFixture(t)
	c, err := receipt.ResolveCommit(context.Background(), root, r.IndexCommit)
	if err != nil {
		t.Fatal(err)
	}
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "pkg/a.go", StartLine: 4, EndLine: 4}, receipt.Source{})
	if err != nil {
		t.Fatal(err)
	}
	r.Kind, r.Callers = core.ReceiptKindCallers, []string{"@::pkg/pkg.Hello"}
	r.Facts = append(r.Facts, receipt.Fact{Kind: receipt.FactCaller, Subject: r.Callers[0], Anchors: []receipt.ExportAnchor{{Anchor: a, Excerpt: string(excerpt)}}})
	return root, r
}

func cloneRound4Receipt(t *testing.T, r receipt.ExportedReceipt) receipt.ExportedReceipt {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var out receipt.ExportedReceipt
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestVerifyExportedUnknownFactKind(t *testing.T) {
	root, r := rungFixture(t)
	r.Facts = append(r.Facts, receipt.Fact{Kind: "judgment", Subject: "this code is safe"})
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || out.OK || !strings.Contains(out.Diff, `fact kind "judgment" is not one an exported receipt carries`) {
		t.Fatalf("unknown fact accepted: %+v, %v", out, err)
	}
}

func TestVerifyExportedNoToolchainCheck(t *testing.T) {
	root, original := round4CallerFixture(t)
	c, err := receipt.ResolveCommit(context.Background(), root, original.IndexCommit)
	if err != nil {
		t.Fatal(err)
	}
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "go.mod", StartLine: 1, EndLine: 1}, receipt.Source{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"caller", "definition", "mixed-uncommitted"} {
		t.Run(name, func(t *testing.T) {
			r := cloneRound4Receipt(t, original)
			if name == "caller" {
				r.Facts[1].Anchors = []receipt.ExportAnchor{{Anchor: a, Excerpt: string(excerpt)}}
			} else {
				r.Facts[0].Subject = "@::pkg/pkg.Missing"
				r.Facts[0].Anchors = []receipt.ExportAnchor{{Anchor: a, Excerpt: string(excerpt)}}
			}
			want := "not reproduced (1 unchecked: no toolchain check)"
			if name == "mixed-uncommitted" {
				r.Facts[1].Anchors[0].State = receipt.StateUncommitted
				want = "not reproduced (1 unchecked: uncommitted, 1 unchecked: no toolchain check)"
			}
			out, err := receiptverify.VerifyExported(context.Background(), root, r)
			if err != nil || out.OK || !strings.Contains(out.Diff, want) {
				t.Fatalf("unchecked fact accepted: %+v, %v", out, err)
			}
		})
	}
}

func TestVerifyExportedRelabelledAnchors(t *testing.T) {
	root, original := round4CallerFixture(t)
	for _, state := range []receipt.State{receipt.StateWithheld, receipt.StateFiltered} {
		for _, field := range []string{"excerpt", "digest", "blob", "fingerprint"} {
			t.Run(string(state)+"/"+field, func(t *testing.T) {
				r := cloneRound4Receipt(t, original)
				a := &r.Facts[1].Anchors[0]
				a.State = state
				if state == receipt.StateWithheld {
					a.BlobID, a.ExcerptSHA256, a.Excerpt, a.FathomFingerprint = "", "", "", ""
				}
				switch field {
				case "excerpt":
					a.Excerpt = "func Hello() { return rm_rf() }\n"
				case "digest":
					a.ExcerptSHA256 = strings.Repeat("0", 64)
				case "blob":
					a.BlobID = strings.Repeat("0", 40)
				case "fingerprint":
					if state == receipt.StateFiltered {
						// Filtered fingerprints describe the producer; pin the
						// checked excerpt even when the fingerprint is unchanged.
						a.Excerpt = "func Hello() { return rm_rf() }\n"
					} else {
						a.FathomFingerprint = "forged"
					}
				}
				out, err := receiptverify.VerifyExported(context.Background(), root, r)
				if err != nil || out.OK || !strings.Contains(fmt.Sprint(out.Rungs), "git rung: refuted") {
					t.Fatalf("relabelled bytes accepted: %+v, %v", out, err)
				}
			})
		}
	}
	// The producer's stripped withheld anchor remains verifiable.
	r := cloneRound4Receipt(t, original)
	a := &r.Facts[1].Anchors[0]
	a.State = receipt.StateWithheld
	a.BlobID, a.ExcerptSHA256, a.Excerpt, a.FathomFingerprint = "", "", "", ""
	out, err := receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || !out.OK {
		t.Fatalf("stripped withheld anchor refused: %+v, %v", out, err)
	}
}

func round4Git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestVerifyExportedFilteredBytes(t *testing.T) {
	root, r := rungFixture(t)
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte("*.go text eol=crlf filter=receipt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.IndexCommit = initGitRepo(t, root)
	// A local filter proves the verifier sets the same offline LFS guard
	// as the producer, without any network or Git LFS dependency.
	round4Git(t, root, "config", "filter.receipt.required", "true")
	round4Git(t, root, "config", "filter.receipt.smudge", `test "$GIT_LFS_SKIP_SMUDGE" = 1 && cat`)
	if err := os.Remove(filepath.Join(root, "pkg/a.go")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_LFS_SKIP_SMUDGE", "1")
	round4Git(t, root, "checkout", "--", "pkg/a.go")
	c, err := receipt.ResolveCommit(context.Background(), root, r.IndexCommit)
	if err != nil {
		t.Fatal(err)
	}
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "pkg/a.go", StartLine: 3, EndLine: 3}, receipt.Source{})
	if err != nil || a.State != receipt.StateFiltered || !strings.Contains(string(excerpt), "\r\n") {
		t.Fatalf("filtered fixture: %+v %q, %v", a, excerpt, err)
	}
	r.Facts[0].Anchors[0] = receipt.ExportAnchor{Anchor: a, Excerpt: string(excerpt)}
	t.Setenv("GIT_LFS_SKIP_SMUDGE", "0")
	out, err := receiptverify.VerifyGitAnchors(context.Background(), root, r.Facts)
	if err != nil || !out.OK || !strings.Contains(fmt.Sprint(out.Rungs), "git rung: confirmed") {
		t.Fatalf("filtered git check did not set offline LFS guard: %+v, %v", out, err)
	}
	t.Setenv("GIT_LFS_SKIP_SMUDGE", "1")
	out, err = receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || !out.OK || !strings.Contains(fmt.Sprint(out.Rungs), "git rung: confirmed") {
		t.Fatalf("filtered checkout bytes not verified: %+v, %v", out, err)
	}
	round4Git(t, root, "config", "filter.receipt.smudge", "false")
	out, err = receiptverify.VerifyExported(context.Background(), root, r)
	if err != nil || out.OK || !strings.Contains(out.Diff, "1 unchecked: git filters unavailable") {
		t.Fatalf("unavailable filter not counted unchecked: %+v, %v", out, err)
	}
}

func TestVerifyExportedIncompleteFacts(t *testing.T) {
	root, original := round4CallerFixture(t)
	for _, name := range []string{"missing-caller-fact", "duplicate-fact", "duplicate-caller", "duplicate-caller-envelope-only"} {
		t.Run(name, func(t *testing.T) {
			r := cloneRound4Receipt(t, original)
			switch name {
			case "missing-caller-fact":
				r.Facts = r.Facts[:1]
			case "duplicate-fact":
				r.Facts = append(r.Facts, r.Facts[0])
			default:
				r.Callers = append(r.Callers, r.Callers[0])
				if name == "duplicate-caller-envelope-only" {
					r.Facts = nil
				}
			}
			out, err := receiptverify.VerifyExported(context.Background(), root, r)
			if err != nil || out.OK {
				t.Fatalf("incomplete or duplicated evidence accepted: %+v, %v", out, err)
			}
		})
	}
	original.Facts = nil
	out, err := receiptverify.VerifyExported(context.Background(), root, original)
	if err != nil || !out.OK {
		t.Fatalf("envelope-only receipt refused: %+v, %v", out, err)
	}
}

func TestVerifyExportedEmptyAbsenceReadSet(t *testing.T) {
	root, original := rungFixture(t)
	original.Kind, original.Symbol = core.ReceiptKindCallers, "Hello"
	for _, name := range []string{"missing", "empty", "no-files"} {
		t.Run(name, func(t *testing.T) {
			r := cloneRound4Receipt(t, original)
			f := receipt.Fact{Kind: receipt.FactAbsence, Subject: "@::pkg/pkg.Hello"}
			if name == "empty" {
				f.ReadSet = &receipt.CoverageManifest{}
			} else if name == "no-files" {
				f.ReadSet = &receipt.CoverageManifest{TreeHash: round4Git(t, root, "rev-parse", "HEAD^{tree}"), Tool: "fathom/find_callers"}
			}
			r.Facts = []receipt.Fact{f}
			out, err := receiptverify.VerifyExported(context.Background(), root, r)
			if err != nil || out.OK || !strings.Contains(out.Diff, "absence fact has no read set") {
				t.Fatalf("empty absence read set accepted: %+v, %v", out, err)
			}
		})
	}
}

func TestVerifySymbolLocationInvalidFile(t *testing.T) {
	root, original := rungFixture(t)
	for _, file := range []string{"../outside.go", "../../../../outside.go", "/outside.go", `..\outside.go`} {
		t.Run(file, func(t *testing.T) {
			r := original.Receipt
			r.File = file
			out, err := receiptverify.Verify(context.Background(), root, r)
			if err != nil || out.OK || out.Diff != "invalid repo-relative file" {
				t.Fatalf("invalid path was read: %+v, %v", out, err)
			}
		})
	}
}
