package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/receipt"
)

func TestCmdReceiptVerifyExportedRungs(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{"go.mod": "module example.invalid/receipt\n\ngo 1.22\n", "a.go": "package pkg\n\nfunc Greet() {}\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := initReceiptTestRepo(t, root)
	// A withheld anchor must be explicitly reported even when the envelope verifies.
	r := receipt.ExportedReceipt{Receipt: core.Receipt{Kind: core.ReceiptKindSymbolLocation, Symbol: "Greet", File: "a.go", Line: 3, IndexCommit: commit}, Facts: []receipt.Fact{{Kind: receipt.FactDefinition, Subject: "@::pkg.Greet", Anchors: []receipt.ExportAnchor{{Anchor: receipt.Anchor{State: receipt.StateWithheld, Commit: commit, Path: "a.go", StartLine: 3, EndLine: 3}}}}}}
	file := filepath.Join(t.TempDir(), "receipt.json")
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cmdReceipt([]string{"verify", "--repo", root, file}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s%s", code, &stdout, &stderr)
	}
	for _, want := range []string{"definition @::pkg.Greet", "no git rung (withheld)", "toolchain rung: confirmed"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("output %q missing %q", stdout.String(), want)
		}
	}
}

func TestCmdReceiptVerifyExportedCallersRoundTrip(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{"go.mod": "module example.invalid/receipt\n\ngo 1.22\n", "a.go": "package pkg\n\nfunc Greet() {}\nfunc Hello() { Greet() }\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := initReceiptTestRepo(t, root)
	c, err := receipt.ResolveCommit(context.Background(), root, commit)
	if err != nil {
		t.Fatal(err)
	}
	a, excerpt, err := receipt.AnchorExcerpt(context.Background(), c, receipt.Span{Path: "a.go", StartLine: 4, EndLine: 4}, receipt.Source{})
	if err != nil {
		t.Fatal(err)
	}
	original, err := receipt.BuildReceipt(receipt.ExportInput{Receipt: core.Receipt{Kind: core.ReceiptKindCallers, Symbol: "Greet", IndexCommit: commit, Callers: []string{"rcpt::pkg.Hello"}}, Facts: []receipt.FactInput{{Kind: receipt.FactCaller, Subject: "rcpt::pkg.Hello", Anchors: []receipt.CitedAnchor{{Anchor: a, Excerpt: excerpt}}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range []string{"rcpt", "@"} {
		t.Run(service, func(t *testing.T) {
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var r receipt.ExportedReceipt
			if err := json.Unmarshal(data, &r); err != nil {
				t.Fatal(err)
			}
			r.Callers[0] = service + "::pkg.Hello"
			r.Facts[0].Subject = r.Callers[0]
			file := filepath.Join(t.TempDir(), "receipt.json")
			data, err = json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, data, 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := cmdReceipt([]string{"verify", "--repo", root, file}, &stdout, &stderr); code != 0 {
				t.Fatalf("exit %d: %s%s", code, &stdout, &stderr)
			}
			if !strings.Contains(stdout.String(), "caller "+r.Callers[0]+": toolchain rung: confirmed") {
				t.Fatalf("no caller confirmation: %s", &stdout)
			}
		})
	}
}

func TestCmdReceiptVerifyCertificationAnchors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/receipt\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := initReceiptTestRepo(t, root)
	r := receipt.ExportedReceipt{Receipt: core.Receipt{Kind: core.ReceiptKindCertification, IndexCommit: commit}, Facts: []receipt.Fact{{Kind: receipt.FactDefinition, Subject: "@::pkg.TestGreet", Anchors: []receipt.ExportAnchor{{Anchor: receipt.Anchor{State: receipt.StateCommitted, Commit: commit, ObjectFormat: "sha1", Path: "missing.go"}}}}}}
	file := filepath.Join(t.TempDir(), "receipt.json")
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cmdReceipt([]string{"verify", "--repo", root, file}, &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), "git rung: refuted") {
		t.Fatalf("certification skipped anchor: exit %d: %s%s", code, &stdout, &stderr)
	}
}
