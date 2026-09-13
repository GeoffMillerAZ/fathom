// sessions_workerdiscovery_test.go — RC-32 (docs/vision/28-receipts.md
// §4): workerDiscoveryBrief runs `fathom discoveries` against a wired
// cell's scratch daemon. A tiny fake `fathom` script stands in for the
// real binary so this test never shells to a live daemon.
package scenariobench

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFakeFathomDiscoveries(t *testing.T, cellDir, stdout string) string {
	t.Helper()
	if err := os.MkdirAll(cellDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(cellDir, "fathom")
	script := "#!/bin/sh\ncat <<'JSON'\n" + stdout + "\nJSON\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestWorkerDiscoveryBrief_ReturnsRenderedTextAndPointers(t *testing.T) {
	cellDir := t.TempDir()
	writeFakeFathomDiscoveries(t, cellDir, `{"rendered":"symbol internal/mcp.registerReferences -> references_tool.go:41\n","pointers":[{"pointer":"internal/mcp.registerReferences"}]}`)
	wt := Worktree{Path: t.TempDir(), CellDir: cellDir, DaemonURL: "http://127.0.0.1:1"}
	arm := Arm{Wired: true}

	brief, pointers := workerDiscoveryBrief(context.Background(), wt, arm, time.Now())
	if brief == "" {
		t.Fatal("workerDiscoveryBrief returned an empty brief")
	}
	if len(pointers) != 1 || pointers[0] != "internal/mcp.registerReferences" {
		t.Errorf("workerDiscoveryBrief pointers = %v, want [internal/mcp.registerReferences]", pointers)
	}
}

func TestWorkerDiscoveryBrief_EmptyOnUnwiredArm(t *testing.T) {
	cellDir := t.TempDir()
	writeFakeFathomDiscoveries(t, cellDir, `{"rendered":"should never be read","pointers":[]}`)
	wt := Worktree{Path: t.TempDir(), CellDir: cellDir, DaemonURL: "http://127.0.0.1:1"}
	arm := Arm{Wired: false}

	brief, pointers := workerDiscoveryBrief(context.Background(), wt, arm, time.Now())
	if brief != "" || pointers != nil {
		t.Errorf("workerDiscoveryBrief on an unwired arm = (%q, %v), want (\"\", nil)", brief, pointers)
	}
}

func TestWorkerDiscoveryBrief_EmptyWhenBinaryMissing(t *testing.T) {
	wt := Worktree{Path: t.TempDir(), CellDir: t.TempDir(), DaemonURL: "http://127.0.0.1:1"}
	arm := Arm{Wired: true}

	brief, pointers := workerDiscoveryBrief(context.Background(), wt, arm, time.Now())
	if brief != "" || pointers != nil {
		t.Errorf("workerDiscoveryBrief with no built binary = (%q, %v), want (\"\", nil)", brief, pointers)
	}
}

func TestWorkerDiscoveryBrief_EmptyOnEmptyRender(t *testing.T) {
	cellDir := t.TempDir()
	writeFakeFathomDiscoveries(t, cellDir, `{"rendered":"","pointers":[]}`)
	wt := Worktree{Path: t.TempDir(), CellDir: cellDir, DaemonURL: "http://127.0.0.1:1"}
	arm := Arm{Wired: true}

	brief, pointers := workerDiscoveryBrief(context.Background(), wt, arm, time.Now())
	if brief != "" || pointers != nil {
		t.Errorf("workerDiscoveryBrief on an empty render = (%q, %v), want (\"\", nil)", brief, pointers)
	}
}
