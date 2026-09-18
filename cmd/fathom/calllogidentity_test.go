package main

// calllogidentity_test.go — J5's shared test table for resolveServiceCallLog
// (app-kit O25, O15/O17/O19). One resolver, one table: every case a caller
// (instrument --adoption, research report, flywheel) could hit.

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveServiceCallLog(t *testing.T) {
	t.Run("FATHOM_CALL_LOG always wins, even with a service named", func(t *testing.T) {
		t.Setenv("FATHOM_CALL_LOG", "/explicit/override.jsonl")
		daemon := fakeDoctorDaemon(t, fakeDaemonCfg{
			services:     []string{"app-kit"},
			servicePaths: map[string]string{"app-kit": t.TempDir()},
		})
		defer daemon.Close()
		hc := &http.Client{}
		path, root := resolveServiceCallLog(hc, daemon.URL, "app-kit")
		if path != "/explicit/override.jsonl" {
			t.Errorf("path = %q, want the env override", path)
		}
		if root != "" {
			t.Errorf("root = %q, want empty (env override names its own location)", root)
		}
	})

	t.Run("named service with its own configured call_log", func(t *testing.T) {
		svcRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(svcRoot, ".fathom"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(svcRoot, ".fathom", "config.toml"),
			[]byte("[daemon]\ncall_log = \"workspace/logs/mcp-calls.jsonl\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		daemon := fakeDoctorDaemon(t, fakeDaemonCfg{
			services:     []string{"app-kit"},
			servicePaths: map[string]string{"app-kit": svcRoot},
		})
		defer daemon.Close()
		hc := &http.Client{}
		path, root := resolveServiceCallLog(hc, daemon.URL, "app-kit")
		want := filepath.Join(svcRoot, "workspace", "logs", "mcp-calls.jsonl")
		if path != want {
			t.Errorf("path = %q, want %q (app-kit's OWN configured call_log, not the caller's cwd)", path, want)
		}
		if root != svcRoot {
			t.Errorf("root = %q, want %q", root, svcRoot)
		}
	})

	t.Run("named service, no config, falls back to the daemon default for that root", func(t *testing.T) {
		svcRoot := t.TempDir()
		daemon := fakeDoctorDaemon(t, fakeDaemonCfg{
			services:     []string{"app-kit"},
			servicePaths: map[string]string{"app-kit": svcRoot},
		})
		defer daemon.Close()
		hc := &http.Client{}
		path, root := resolveServiceCallLog(hc, daemon.URL, "app-kit")
		want := callLogPathFor(svcRoot)
		if path != want {
			t.Errorf("path = %q, want the daemon default for %q: %q", path, svcRoot, want)
		}
		if root != svcRoot {
			t.Errorf("root = %q, want %q", root, svcRoot)
		}
	})

	t.Run("unknown service falls back to the pre-existing CWD-relative default", func(t *testing.T) {
		daemon := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"other-svc"}})
		defer daemon.Close()
		hc := &http.Client{}
		path, root := resolveServiceCallLog(hc, daemon.URL, "app-kit")
		if path != resolveCallLogPath() {
			t.Errorf("path = %q, want resolveCallLogPath()'s answer", path)
		}
		if root != "" {
			t.Errorf("root = %q, want empty — nothing service-specific was resolved", root)
		}
	})

	t.Run("no service named uses the CWD-relative default", func(t *testing.T) {
		daemon := fakeDoctorDaemon(t, fakeDaemonCfg{services: []string{"app-kit"}, servicePaths: map[string]string{"app-kit": t.TempDir()}})
		defer daemon.Close()
		hc := &http.Client{}
		path, root := resolveServiceCallLog(hc, daemon.URL, "")
		if path != resolveCallLogPath() {
			t.Errorf("path = %q, want resolveCallLogPath()'s answer", path)
		}
		if root != "" {
			t.Errorf("root = %q, want empty", root)
		}
	})

	t.Run("daemon unreachable falls back to the CWD-relative default", func(t *testing.T) {
		hc := &http.Client{}
		path, root := resolveServiceCallLog(hc, "http://127.0.0.1:1", "app-kit")
		if path != resolveCallLogPath() {
			t.Errorf("path = %q, want resolveCallLogPath()'s answer", path)
		}
		if root != "" {
			t.Errorf("root = %q, want empty", root)
		}
	})
}

func TestConfiguredCallLogPathAt(t *testing.T) {
	t.Run("relative call_log joins against root, never cwd", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".fathom", "config.toml"),
			[]byte("[daemon]\ncall_log = \"logs/mcp-calls.jsonl\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got, ok := configuredCallLogPathAt(root)
		if !ok {
			t.Fatal("configuredCallLogPathAt: ok = false, want true")
		}
		if want := filepath.Join(root, "logs", "mcp-calls.jsonl"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("no config at root: false", func(t *testing.T) {
		if _, ok := configuredCallLogPathAt(t.TempDir()); ok {
			t.Error("ok = true, want false for a root with no .fathom/config.toml")
		}
	})
}

func TestCallLogIdentityLine(t *testing.T) {
	if got := callLogIdentityLine("/x/mcp-calls.jsonl", os.ErrNotExist); got != "call log: unknown (/x/mcp-calls.jsonl: no such file)" {
		t.Errorf("got %q", got)
	}
	if got, want := callLogIdentityLine("/x/mcp-calls.jsonl", &os.PathError{Op: "open", Path: "/x/mcp-calls.jsonl", Err: os.ErrPermission}), "call log: unknown (/x/mcp-calls.jsonl: open /x/mcp-calls.jsonl: permission denied)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
