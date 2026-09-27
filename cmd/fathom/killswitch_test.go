package main

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// killSwitchHome makes a scratch HOME whose user config is BROKEN, so any
// path that loads config before the kill switch fails loudly (exit 1)
// instead of exiting 0.
func killSwitchHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XPC_SERVICE_NAME", "")
	if err := os.MkdirAll(filepath.Join(home, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".fathom", "config.toml"), []byte("this is [not toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func writeSwitch(t *testing.T, home, name string, body []byte) {
	t.Helper()
	dir := filepath.Join(home, ".fathom", "daemon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

// RT-3 (§9): the managed daemon's first action reads disabled.json and exits
// 0 in under 50 ms — before config (broken here), before the store (never
// created) and before the bind (the port stays free). Under a schema-1 plist
// launchd relaunches every 10 s, so the check writes no lifecycle row either
// (RT-157): nothing appears under ~/.fathom/daemon but the switch itself.
func TestDisabledFileCheckedBeforeStoreOpen(t *testing.T) {
	home := killSwitchHome(t)
	writeSwitch(t, home, tenancy.DisabledFile, nil) // `touch` — reason unknown
	store := filepath.Join(t.TempDir(), "nested", "fleet.sqlite")
	addr := freeAddr(t)

	var out, errBuf bytes.Buffer
	start := time.Now()
	rc := runServeHTTP([]string{"--managed", "--store", store, "--addr", addr}, &out, &errBuf)
	elapsed := time.Since(start)
	if rc != 0 {
		t.Fatalf("a disabled managed daemon must exit 0 (launchd stays down under schema 2), got %d; stderr:\n%s", rc, errBuf.String())
	}
	if elapsed >= 50*time.Millisecond {
		t.Fatalf("the kill-switch exit took %s, want < 50ms", elapsed)
	}
	if _, err := os.Stat(filepath.Dir(store)); !os.IsNotExist(err) {
		t.Fatalf("the store was touched before the kill switch: %v", err)
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("the port was bound before the kill switch: %v", err)
	}
	l.Close()
	if !strings.Contains(errBuf.String(), "reason unknown") {
		t.Fatalf("the exit must say why: %q", errBuf.String())
	}
	entries, _ := os.ReadDir(filepath.Join(home, ".fathom", "daemon"))
	if len(entries) != 1 || entries[0].Name() != tenancy.DisabledFile {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("the kill-switch check must write nothing (no lifecycle row, no record): %v", names)
	}

	// A timed stop exits 75 until it expires.
	writeSwitch(t, home, tenancy.DisabledFile, tenancy.MarshalSwitch(tenancy.SwitchRecord{By: "test", Since: time.Now(), Until: time.Now().Add(time.Hour)}))
	if rc := runServeHTTP([]string{"--managed", "--store", store, "--addr", addr}, &out, &errBuf); rc != tenancy.ExitRetryLater {
		t.Fatalf("a timed stop must exit %d, got %d", tenancy.ExitRetryLater, rc)
	}
}

// RT-167 (§2.3, §9): the switch files govern only the managed daemon. A
// scratch (unmanaged) serve-http ignores them — a breaker stop must never
// block a proof agent's scratch daemon. Managed is `--managed` or a launchd
// XPC_SERVICE_NAME naming a fathom label.
func TestKillSwitchIgnoredByUnmanagedDaemon(t *testing.T) {
	home := killSwitchHome(t)
	writeSwitch(t, home, tenancy.DisabledFile, nil)
	writeSwitch(t, home, tenancy.PausedFile, nil)
	var errBuf bytes.Buffer

	if _, stop := checkKillSwitch([]string{"--store", "x"}, os.Getenv, time.Now(), &errBuf); stop {
		t.Fatal("an unmanaged serve-http must ignore disabled.json")
	}
	// Past the switch, the unmanaged run reaches the (broken) config: exit 1.
	if rc := runServeHTTP([]string{"--store", filepath.Join(t.TempDir(), "s.sqlite"), "--addr", freeAddr(t)}, &bytes.Buffer{}, &errBuf); rc != 1 || !strings.Contains(errBuf.String(), "loading config") {
		t.Fatalf("an unmanaged serve-http must proceed past the switch (to config): rc=%d %s", rc, errBuf.String())
	}

	for _, args := range [][]string{{"--managed"}, {"-managed"}, {"--managed=true"}} {
		if code, stop := checkKillSwitch(args, os.Getenv, time.Now(), &errBuf); !stop || code != 0 {
			t.Fatalf("%v: the managed daemon must obey disabled.json (stop=%v code=%d)", args, stop, code)
		}
	}
	if _, stop := checkKillSwitch([]string{"--managed=false"}, os.Getenv, time.Now(), &errBuf); stop {
		t.Fatal("--managed=false is unmanaged")
	}
	t.Setenv("XPC_SERVICE_NAME", "dev.fathom.serve-http")
	if _, stop := checkKillSwitch(nil, os.Getenv, time.Now(), &errBuf); !stop {
		t.Fatal("a launchd job whose label is fathom's is managed")
	}
	t.Setenv("XPC_SERVICE_NAME", "application.com.mitchellh.ghostty.1234")
	if _, stop := checkKillSwitch(nil, os.Getenv, time.Now(), &errBuf); stop {
		t.Fatal("a terminal's XPC_SERVICE_NAME is not a managed fathom")
	}
}
