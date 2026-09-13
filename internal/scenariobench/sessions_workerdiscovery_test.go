// sessions_workerdiscovery_test.go — RC-32 (docs/vision/28-receipts.md
// §4): workerDiscoveryBrief runs `fathom discoveries` against a wired
// cell's scratch daemon. A tiny fake `fathom` script stands in for the
// real binary so this test never shells to a live daemon.
package scenariobench

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// writeFakeFathomEchoEnv writes a `fathom` stand-in that ignores its own
// discoveries logic and instead reports back, as its "rendered" text,
// exactly the env vars and flags finding 7 cares about — so a test can
// assert on what workerDiscoveryBrief's subprocess actually received
// without needing a real daemon or thread store.
func writeFakeFathomEchoEnv(t *testing.T, cellDir string) string {
	t.Helper()
	if err := os.MkdirAll(cellDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(cellDir, "fathom")
	script := "#!/bin/sh\n" +
		`echo "{\"rendered\":\"threads=$FATHOM_THREADS http=$FATHOM_HTTP args=$*\",\"pointers\":[]}"` + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// TestWorkerDiscoveryBrief_UsesTheCellsOwnEnvNeverTheOperators is finding
// 7's own proof: the `fathom discoveries` subprocess must run with
// FATHOM_THREADS pointed at the CELL's own threads file, FATHOM_HTTP
// pointed at the cell's own scratch daemon, and -service passed
// explicitly — never left to fall back to the operator's real
// ~/.fathom/threads.jsonl or default daemon.
func TestWorkerDiscoveryBrief_UsesTheCellsOwnEnvNeverTheOperators(t *testing.T) {
	cellDir := t.TempDir()
	writeFakeFathomEchoEnv(t, cellDir)
	wt := Worktree{
		Path:      t.TempDir(),
		CellDir:   cellDir,
		DaemonURL: "http://127.0.0.1:54321",
		Service:   "myservice",
	}
	arm := Arm{Wired: true}

	brief, _ := workerDiscoveryBrief(context.Background(), wt, arm, time.Now())
	if brief == "" {
		t.Fatal("workerDiscoveryBrief returned an empty brief")
	}
	wantThreads := filepath.Join(cellDir, "threads.jsonl")
	if !strings.Contains(brief, "threads="+wantThreads) {
		t.Errorf("brief = %q, want FATHOM_THREADS=%q (the cell's own, never ~/.fathom/threads.jsonl)", brief, wantThreads)
	}
	if !strings.Contains(brief, "http=http://127.0.0.1:54321") {
		t.Errorf("brief = %q, want FATHOM_HTTP=%q (the cell's own scratch daemon)", brief, wt.DaemonURL)
	}
	if !strings.Contains(brief, "-service myservice") {
		t.Errorf("brief = %q, want an explicit -service myservice flag", brief)
	}
}
