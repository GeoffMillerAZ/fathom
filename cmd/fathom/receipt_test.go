package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/indexbench"
)

// TestCmdReceiptVerify_Reproduces is VB-20's CLI-level golden case: a
// receipt.json shaped like find_callers' own answer, checked with
// `fathom receipt verify`, exits 0 against a bare checkout of its own
// index_commit.
func TestCmdReceiptVerify_Reproduces(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initReceiptTestRepo(t, root)
	receiptPath := writeReceiptFile(t, core.Receipt{
		Kind:        core.ReceiptKindCallers,
		Tool:        "find_callers",
		Symbol:      "validateCharge",
		IndexCommit: commit,
		Callers:     []string{"Charge"},
	})

	var stdout, stderr bytes.Buffer
	code := cmdReceipt([]string{"verify", "--repo", root, receiptPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("stdout = %q, want an OK line", stdout.String())
	}
}

// TestCmdReceiptVerify_Tampered is VB-20's exit-1 case: a receipt claiming
// a caller that does not exist at this commit exits 1 and names the
// difference, never a silent pass.
func TestCmdReceiptVerify_Tampered(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	commit := initReceiptTestRepo(t, root)
	receiptPath := writeReceiptFile(t, core.Receipt{
		Kind:        core.ReceiptKindCallers,
		Symbol:      "validateCharge",
		IndexCommit: commit,
		Callers:     []string{"Charge", "NotARealCaller"},
	})

	var stdout, stderr bytes.Buffer
	code := cmdReceipt([]string{"verify", "--repo", root, receiptPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "NotARealCaller") {
		t.Errorf("stdout = %q, want the tampered caller named", stdout.String())
	}
}

// TestCmdReceiptVerify_CannotRun covers VB-20's exit-2 shape: a missing
// receipt file, invalid JSON, and a repo with no such commit all refuse
// operationally rather than falsely passing or failing.
func TestCmdReceiptVerify_CannotRun(t *testing.T) {
	root, err := indexbench.StageFauxtownModule("payment-service", t.TempDir())
	if err != nil {
		t.Fatalf("stage fixture: %v", err)
	}
	initReceiptTestRepo(t, root)

	t.Run("missing file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := cmdReceipt([]string{"verify", "--repo", root, filepath.Join(root, "no-such-receipt.json")}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2; stderr=%s", code, stderr.String())
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		bad := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := cmdReceipt([]string{"verify", "--repo", root, bad}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2; stderr=%s", code, stderr.String())
		}
	})

	t.Run("missing commit", func(t *testing.T) {
		receiptPath := writeReceiptFile(t, core.Receipt{
			Kind:        core.ReceiptKindCallers,
			Symbol:      "validateCharge",
			IndexCommit: "0000000000000000000000000000000000dead",
			Callers:     []string{"Charge"},
		})
		var stdout, stderr bytes.Buffer
		code := cmdReceipt([]string{"verify", "--repo", root, receiptPath}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("exit code = %d, want 2; stderr=%s", code, stderr.String())
		}
	})

	t.Run("no verb", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := cmdReceipt(nil, &stdout, &stderr); code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
	})
}

func writeReceiptFile(t *testing.T, r core.Receipt) string {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func initReceiptTestRepo(t *testing.T, dir string) string {
	t.Helper()
	env := append(
		os.Environ(),
		"GIT_AUTHOR_NAME=fathom-test", "GIT_AUTHOR_EMAIL=fathom-test@example.com",
		"GIT_COMMITTER_NAME=fathom-test", "GIT_COMMITTER_EMAIL=fathom-test@example.com",
	)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("add", "-A")
	run("commit", "-q", "-m", "fixture")
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}
