package mcp_test

// verify_plan_path_test.go — O1069: verify_plan with plan_path and no
// plan_markdown answered total:0 and no error for a relative, an absolute
// and a nonexistent path alike (it never read the file). plan_path now
// names a file under the service's indexed root; every other form is an
// error naming the accepted one, never a silent empty plan.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func verifyPlanPathCall(t *testing.T, srv *mcp.Server, planPath string) map[string]any {
	t.Helper()
	return roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "verify_plan",
			"arguments": map[string]any{"service": "svc", "plan_path": planPath},
		},
	})
}

func TestVerifyPlanTool_PlanPath_ReadsTheFileUnderTheIndexedRoot(t *testing.T) {
	root := newSelfSkipRepo(t, "- [x] do work {id: T-1, symbol: DoWork}\n- [ ] untested {id: T-2, symbol: Untested}\n")
	srv := mcp.NewServer(&repoMetaBackend{verifyFakeBackend: newVerifyBackend(), repoPath: root})

	resp := verifyPlanPathCall(t, srv, "docs/plans/ss.md")
	if resp["error"] != nil {
		t.Fatalf("relative plan_path: error %v", resp["error"])
	}
	p := xtraPayload(t, resp)
	sum := p["summary"].(map[string]any)
	if sum["total"] != float64(2) {
		t.Fatalf("relative plan_path: summary %v, want the file's 2 rows", sum)
	}
	if got := p["tasks"].([]any)[0].(map[string]any); got["id"] != "T-1" || got["status"] != "pass" {
		t.Fatalf("T-1 = %v, want pass", got)
	}
}

func TestVerifyPlanTool_PlanPath_RefusesEveryOtherForm(t *testing.T) {
	root := newSelfSkipRepo(t, "- [x] do work {id: T-1, symbol: DoWork}\n")
	outside := filepath.Join(t.TempDir(), "elsewhere.md")
	if err := os.WriteFile(outside, []byte("- [x] do work {id: T-1, symbol: DoWork}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "docs", "plans", "link.md")); err != nil {
		t.Fatal(err)
	}
	srv := mcp.NewServer(&repoMetaBackend{verifyFakeBackend: newVerifyBackend(), repoPath: root})

	for _, tc := range []struct{ name, path, want string }{
		{"absolute", filepath.Join(root, "docs", "plans", "ss.md"), "absolute"},
		{"escaping", "../elsewhere.md", "escapes the indexed root"},
		{"missing", "docs/plans/nope.md", "no such file under the indexed root"},
		{"symlink out", "docs/plans/link.md", "resolves outside the indexed root"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := verifyPlanPathCall(t, srv, tc.path)
			e, ok := resp["error"].(map[string]any)
			if !ok {
				t.Fatalf("plan_path %q: no error (result %v), want one naming %q", tc.path, resp["result"], tc.want)
			}
			msg, _ := e["message"].(string)
			if !strings.Contains(msg, tc.want) {
				t.Fatalf("plan_path %q: error %q, want it to name %q", tc.path, msg, tc.want)
			}
			if tc.name != "missing" && !strings.Contains(msg, "docs/plans/<plan>.md") {
				t.Fatalf("plan_path %q: error %q does not name the accepted form", tc.path, msg)
			}
		})
	}
}

func TestVerifyPlanTool_PlanPath_NoIndexedRootIsAnError(t *testing.T) {
	srv := mcp.NewServer(newVerifyBackend())
	resp := verifyPlanPathCall(t, srv, "docs/plans/ss.md")
	e, ok := resp["error"].(map[string]any)
	if !ok || !strings.Contains(e["message"].(string), "pass plan_markdown") {
		t.Fatalf("no recorded root: %v, want an error pointing at plan_markdown", resp)
	}
}
