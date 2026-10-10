package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
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

// TestCmdReceiptVerify_OfflineMissingModuleExits2 is EVX-6 (I56) at the
// command: `fathom receipt verify` over a checkout requiring a module no
// cache holds exits 2 and names the module, and asks no module proxy —
// for the callers kind and for the certification kind's --run-tests path.
func TestCmdReceiptVerify_OfflineMissingModuleExits2(t *testing.T) {
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.NotFound(w, r)
	}))
	t.Cleanup(proxy.Close)
	t.Setenv("GOMODCACHE", t.TempDir())
	t.Setenv("GOPROXY", proxy.URL)
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOFLAGS", "")

	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod": "module receipt.test/offline\n\ngo 1.21\n\nrequire example.invalid/dep v1.0.0\n",
		"go.sum": "example.invalid/dep v1.0.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n" +
			"example.invalid/dep v1.0.0/go.mod h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n",
		"main.go":      "package main\n\nimport \"example.invalid/dep\"\n\nfunc run() { dep.Do() }\n\nfunc main() { run() }\n",
		"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { run() }\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := initReceiptTestRepo(t, root)

	for name, r := range map[string]core.Receipt{
		"callers":       {Kind: core.ReceiptKindCallers, Symbol: "run", IndexCommit: commit, Callers: []string{"main"}},
		"certification": {Kind: core.ReceiptKindCertification, Test: "TestRun", Reached: []string{"run"}, IndexCommit: commit},
	} {
		var stdout, stderr bytes.Buffer
		code := cmdReceipt([]string{"verify", "--repo", root, "--run-tests", writeReceiptFile(t, r)}, &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "example.invalid/dep@v1.0.0") {
			t.Errorf("%s: exit %d, stderr %q; want exit 2 naming example.invalid/dep@v1.0.0", name, code, stderr.String())
		}
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("receipt verify asked the module proxy %d time(s); it must never download", n)
	}
}
