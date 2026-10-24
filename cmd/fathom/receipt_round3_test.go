package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
)

func round3CallersReceipt(t *testing.T) (*selfSkipGitRepo, receipt.ExportedReceipt) {
	t.Helper()
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module example.invalid/receipt\n\ngo 1.22\n")
	repo.writePlan("pkg/a.go", "package pkg\nfunc Greet() {}\nfunc Hello() { Greet() }\nfunc Other() { Greet() }\n")
	repo.writePlan("pkg/a_test.go", "package pkg\nimport \"testing\"\nfunc TestMain(m *testing.M) { m.Run() }\n")
	repo.commitAll("fixture")
	commit, err := receipt.ResolveCommit(context.Background(), repo.root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	in := receipt.ExportInput{Receipt: core.Receipt{Kind: core.ReceiptKindCallers, Symbol: "Greet", IndexCommit: commit.ID}}
	for i, name := range []string{"Hello", "Other"} {
		a, excerpt, err := receipt.AnchorExcerpt(context.Background(), commit, receipt.Span{Path: "pkg/a.go", StartLine: i + 3, EndLine: i + 3}, receipt.Source{})
		if err != nil {
			t.Fatal(err)
		}
		subject := "@::pkg/pkg." + name
		in.Receipt.Callers = append(in.Receipt.Callers, subject)
		in.Facts = append(in.Facts, receipt.FactInput{Kind: receipt.FactCaller, Subject: subject, Anchors: []receipt.CitedAnchor{{Anchor: a, Excerpt: excerpt}}})
	}
	r, err := receipt.BuildReceipt(in)
	if err != nil {
		t.Fatal(err)
	}
	return repo, r
}

func verifyRound3Receipt(t *testing.T, root string, r receipt.ExportedReceipt, wantCode int, wants ...string) {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := cmdReceipt([]string{"verify", "--repo", root, file}, &stdout, &stderr)
	if code != wantCode {
		t.Fatalf("verify exit %d, want %d: %s%s", code, wantCode, &stdout, &stderr)
	}
	if code != 0 && strings.Contains(stdout.String(), "receipt verify: OK") {
		t.Fatalf("failed verification claimed OK: %s", &stdout)
	}
	for _, want := range wants {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("verify output lacks %q: %s%s", want, &stdout, &stderr)
		}
	}
}

func TestCmdReceiptVerifyForgedUncommittedCallers(t *testing.T) {
	for _, all := range []bool{false, true} {
		for _, missing := range []bool{false, true} {
			name := "mixed"
			if all {
				name = "all"
			}
			if missing {
				name += "/missing-caller"
			}
			t.Run(name, func(t *testing.T) {
				repo, r := round3CallersReceipt(t)
				if len(r.Callers) != 2 || len(r.Facts) != 2 {
					t.Fatalf("expected two callers and facts: %+v", r)
				}
				r.Facts[0].Anchors[0].State = receipt.StateUncommitted
				want := "not reproduced (1 unchecked: uncommitted)"
				if all {
					r.Facts[1].Anchors[0].State = receipt.StateUncommitted
					want = "not reproduced (2 unchecked: uncommitted)"
				}
				if missing {
					r.Callers = r.Callers[:1]
					verifyRound3Receipt(t, repo.root, r, 1, want, "callers of Greet: receipt names 1")
				} else {
					verifyRound3Receipt(t, repo.root, r, 1, want)
				}
			})
		}
	}
}

func TestCmdReceiptVerifyForgedIndexCommit(t *testing.T) {
	repo, r := round3CallersReceipt(t)
	repo.writePlan("pkg/a.go", "package pkg\nfunc Greet() {}\nfunc Hello() {}\nfunc Other() {}\n")
	repo.commitAll("remove calls")
	commit, err := exec.Command("git", "-C", repo.root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	r.IndexCommit = strings.TrimSpace(string(commit))
	verifyRound3Receipt(t, repo.root, r, 1, "DIFFERENCE", "anchor commit differs from index_commit")
}

func TestCmdReceiptVerifyTestMainAbsence(t *testing.T) {
	repo, r := round3CallersReceipt(t)
	r.Symbol, r.Callers = "TestMain", nil
	tree, err := exec.Command("git", "-C", repo.root, "rev-parse", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	r.Facts = []receipt.Fact{{Kind: receipt.FactAbsence, Subject: "TestMain", ReadSet: &receipt.CoverageManifest{
		Files: []string{"pkg/a.go", "pkg/a_test.go"}, TreeHash: strings.TrimSpace(string(tree)), Tool: "fathom/find_callers",
	}}}
	if len(r.Callers) != 0 || len(r.Facts) != 1 || r.Facts[0].Kind != receipt.FactAbsence {
		t.Fatalf("expected absence fact: %+v", r)
	}
	verifyRound3Receipt(t, repo.root, r, 0, "receipt verify: OK", "toolchain rung: confirmed")
}
