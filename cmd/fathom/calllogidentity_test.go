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

	t.Run("named service whose repository sets call_log: ignored, the daemon default for that root", func(t *testing.T) {
		// SEC-2: [daemon] call_log is user-only (a path the daemon writes);
		// a repository's own value is dropped at Load, so the resolver
		// answers with the per-root default the daemon actually writes.
		t.Setenv("FATHOM_HOME", t.TempDir())
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
		if want := callLogPathFor(svcRoot); path != want {
			t.Errorf("path = %q, want the daemon default for %q: %q (a repository's call_log is ignored)", path, svcRoot, want)
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

func TestCallLogIdentityLine(t *testing.T) {
	if got := callLogIdentityLine("/x/mcp-calls.jsonl", os.ErrNotExist); got != "call log: unknown (/x/mcp-calls.jsonl: no such file)" {
		t.Errorf("got %q", got)
	}
	if got, want := callLogIdentityLine("/x/mcp-calls.jsonl", &os.PathError{Op: "open", Path: "/x/mcp-calls.jsonl", Err: os.ErrPermission}), "call log: unknown (/x/mcp-calls.jsonl: open /x/mcp-calls.jsonl: permission denied)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
