package mcp_test

// selfskip_test.go — O56 defect 2, server-side half: verify_plan's optional
// plan_path argument, resolved against a fake backend's ServiceMeta corpus
// root pointing at a real throwaway git repo, must catch a self-skip (and
// let a skip that predates the claim stand) exactly like the CLI's own
// local overlay does (cmd/fathom/selfskip_test.go).

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// repoMetaBackend wraps verifyFakeBackend, overriding ServiceMeta to report
// a real git checkout on disk — the shape selfSkipPriorPlan needs.
type repoMetaBackend struct {
	*verifyFakeBackend
	repoPath string
}

func (b *repoMetaBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return b.repoPath, "", true, nil
}

// newSelfSkipRepo creates a throwaway git repo at docs/plans/ss.md holding
// committedMD, committed once, and returns its root.
func newSelfSkipRepo(t *testing.T, committedMD string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out.String())
		}
	}
	full := filepath.Join(root, "docs", "plans", "ss.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(committedMD), 0o644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q")
	run("config", "commit.gpgsign", "false")
	run("config", "user.name", "Alice")
	run("config", "user.email", "alice@example.com")
	run("add", "-A")
	run("commit", "-q", "-m", "seed")
	return root
}

// TestVerifyPlanTool_PlanPath_SelfSkipped: the committed (HEAD) row has no
// skip_test; the CALL claims it with a freshly added skip_test — refused
// self-skipped, exactly like the CLI's uncommitted-edit scenario.
func TestVerifyPlanTool_PlanPath_SelfSkipped(t *testing.T) {
	root := newSelfSkipRepo(t, "- [ ] do work {id: T-1, symbol: DoWork}\n")
	backend := &repoMetaBackend{verifyFakeBackend: newVerifyBackend(), repoPath: root}
	srv := mcp.NewServer(backend)

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service":       "svc",
				"plan_markdown": "- [x] do work {id: T-1, symbol: DoWork, skip_test: true, skip: \"added at claim time\"}\n",
				"plan_path":     "docs/plans/ss.md",
			},
		},
	})
	p := xtraPayload(t, resp)
	tasks := p["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("want 1 task, got %d: %+v", len(tasks), tasks)
	}
	tm := tasks[0].(map[string]any)
	if tm["status"] != "fail" || tm["label"] != "self-skipped" {
		t.Fatalf("T-1 = %+v, want fail/self-skipped", tm)
	}
}

// TestVerifyPlanTool_PlanPath_SkipPredatesClaim_Passes: HEAD already carries
// the SAME skip_test/skip on the (unclaimed) row — the call claims it with
// the identical skip — the skip predates the claim and stands.
func TestVerifyPlanTool_PlanPath_SkipPredatesClaim_Passes(t *testing.T) {
	root := newSelfSkipRepo(t, "- [ ] do work {id: T-1, symbol: DoWork, skip_test: true, skip: \"long-standing\"}\n")
	backend := &repoMetaBackend{verifyFakeBackend: newVerifyBackend(), repoPath: root}
	srv := mcp.NewServer(backend)

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service":       "svc",
				"plan_markdown": "- [x] do work {id: T-1, symbol: DoWork, skip_test: true, skip: \"long-standing\"}\n",
				"plan_path":     "docs/plans/ss.md",
			},
		},
	})
	p := xtraPayload(t, resp)
	tasks := p["tasks"].([]any)
	if len(tasks) != 1 {
		t.Fatalf("want 1 task, got %d: %+v", len(tasks), tasks)
	}
	tm := tasks[0].(map[string]any)
	if tm["status"] != "pass" || tm["label"] != "verified" {
		t.Fatalf("T-1 = %+v, want pass/verified (skip predates the claim)", tm)
	}
}

// TestVerifyPlanTool_NoPlanPath_SelfSkipNeverFlagged: without plan_path the
// self-skip half is simply not evaluated — never a false positive.
func TestVerifyPlanTool_NoPlanPath_SelfSkipNeverFlagged(t *testing.T) {
	srv := mcp.NewServer(newVerifyBackend())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name": "verify_plan",
			"arguments": map[string]any{
				"service":       "svc",
				"plan_markdown": "- [x] do work {id: T-1, symbol: DoWork, skip_test: true, skip: \"no plan_path given\"}\n",
			},
		},
	})
	p := xtraPayload(t, resp)
	tasks := p["tasks"].([]any)
	tm := tasks[0].(map[string]any)
	if tm["status"] != "pass" || tm["label"] != "verified" {
		t.Fatalf("T-1 = %+v, want pass/verified (no plan_path, self-skip inapplicable)", tm)
	}
}
