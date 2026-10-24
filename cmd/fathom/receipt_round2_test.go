package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
)

func TestCmdReceiptVerifyIngestedMethods(t *testing.T) {
	t.Setenv("FATHOM_HOME", t.TempDir())
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module example.invalid/methods\n\ngo 1.22\n")
	repo.writePlan("pkg/a.go", "package pkg\nfunc Greet() {}\ntype T struct{}\ntype S[X any] struct{}\nfunc (T) M() { Greet() }\nfunc (*T) P() { Greet() }\nfunc (S[X]) G() { Greet() }\n")
	repo.commitAll("fixture")
	storePath := filepath.Join(t.TempDir(), "scratch.sqlite")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"ingest", "--service", "rcpt", "--path", repo.root, "--store", storePath}, &stdout, &stderr); code != 0 {
		t.Fatalf("ingest exit %d: %s%s", code, &stdout, &stderr)
	}
	callers, _ := exportReceipt(t, storePath, "--keep-service", "--symbol", "Greet", "--kind", "callers")
	want := []string{"rcpt::pkg/T.T.M", "rcpt::pkg/T.T.P", "rcpt::pkg/S.S.G"}
	slices.Sort(want)
	if !slices.Equal(callers.Callers, want) || len(callers.Facts) != 3 {
		t.Fatalf("ingested method callers: %+v, want %v", callers, want)
	}
	// Export keeps the service (--keep-service); exercise the same exported
	// keys with the retained and portable service prefixes, without replacing
	// the real ingest/export path with a hand-built receipt.
	for _, service := range []string{"rcpt", "@"} {
		t.Run(service, func(t *testing.T) {
			data, err := json.Marshal(callers)
			if err != nil {
				t.Fatal(err)
			}
			var r receipt.ExportedReceipt
			if err := json.Unmarshal(data, &r); err != nil {
				t.Fatal(err)
			}
			for i := range r.Callers {
				r.Callers[i] = strings.Replace(r.Callers[i], "rcpt::", service+"::", 1)
			}
			for i := range r.Facts {
				r.Facts[i].Subject = strings.Replace(r.Facts[i].Subject, "rcpt::", service+"::", 1)
			}
			verifyRound2Receipt(t, repo.root, r, "toolchain rung: confirmed")
		})
	}
	for _, name := range []string{"T.M", "T.P", "S.G"} {
		t.Run(name, func(t *testing.T) {
			r, _ := exportReceipt(t, storePath, "--symbol", name)
			if r.Symbol != name {
				t.Fatalf("method envelope lost receiver: %q, want %q", r.Symbol, name)
			}
			verifyRound2Receipt(t, repo.root, r, "toolchain rung: confirmed")
		})
	}
}

func TestCmdReceiptVerifyUncommittedSkipsToolchain(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/newfile\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := initReceiptTestRepo(t, root)
	r := receipt.ExportedReceipt{Receipt: core.Receipt{Kind: core.ReceiptKindSymbolLocation, Symbol: "New", File: "new.go", Line: 2, IndexCommit: commit}, Facts: []receipt.Fact{{Kind: receipt.FactDefinition, Subject: "@::pkg.New", Anchors: []receipt.ExportAnchor{{Anchor: receipt.Anchor{State: receipt.StateUncommitted, Path: "new.go", StartLine: 2}}}}}}
	verifyRound3Receipt(t, root, r, 1, "no toolchain rung (uncommitted)", "not reproduced (1 unchecked: uncommitted)")
}

func verifyRound2Receipt(t *testing.T, root string, r receipt.ExportedReceipt, want string) {
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
	if code := cmdReceipt([]string{"verify", "--repo", root, file}, &stdout, &stderr); code != 0 {
		t.Fatalf("verify exit %d: %s%s", code, &stdout, &stderr)
	}
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("verify output lacks %q: %s", want, &stdout)
	}
	if want == "toolchain rung: confirmed" && strings.Count(stdout.String(), want) != len(r.Facts) {
		t.Fatalf("not every fact confirmed: %s", &stdout)
	}
}
