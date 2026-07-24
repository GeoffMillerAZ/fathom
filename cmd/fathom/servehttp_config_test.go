package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// servehttp_config_test.go — CF-7 (docs/plans/config-experience.md): proves
// runServeHTTP actually loads [daemon] config as flag DEFAULTS (a flag on
// the invocation still wins), without ever reaching http.ListenAndServe's
// blocking accept loop — every other serve-http test (servehttp_test.go)
// deliberately avoids a valid --store for the same reason; this one instead
// points [daemon].addr at a syntactically invalid port so ListenAndServe's
// net.Listen fails IMMEDIATELY (no blocking) once wiring has proven the
// config path was actually read.

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("configured", "fallback"); got != "configured" {
		t.Errorf("got %q, want configured wins", got)
	}
	if got := firstNonEmpty("", "fallback"); got != "fallback" {
		t.Errorf("got %q, want fallback when configured is empty", got)
	}
}

func TestDaemonAutoRefreshDefault(t *testing.T) {
	if !daemonAutoRefreshDefault(config.Daemon{}) {
		t.Error("unset AutoRefresh must default to true (pre-config behavior)")
	}
	f := false
	if daemonAutoRefreshDefault(config.Daemon{AutoRefresh: &f}) {
		t.Error("explicit false must be honored")
	}
	tr := true
	if !daemonAutoRefreshDefault(config.Daemon{AutoRefresh: &tr}) {
		t.Error("explicit true must be honored")
	}
}

// TestRunServeHTTP_DaemonConfig_FeedsFlagDefaults proves --store and --addr
// defaults come from [daemon] in config.toml when the flags themselves are
// not passed on the command line.
func TestRunServeHTTP_DaemonConfig_FeedsFlagDefaults(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	storePath := filepath.Join(projectDir, "daemon.sqlite")
	writeProjectConfig(t, projectDir, `
[daemon]
store = "`+escapeTOML(storePath)+`"
addr = "127.0.0.1:999999"
`)
	var stdout, stderr bytes.Buffer
	code := runServeHTTP(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected a nonzero exit from the deliberately invalid port; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "--store is required") {
		t.Fatalf("stderr = %s, want --store defaulted from [daemon].store, not the 'required' error", stderr.String())
	}
	if !strings.Contains(stderr.String(), "http serve") {
		t.Errorf("stderr = %s, want it to have reached ListenAndServe (proving --addr was ALSO defaulted from config) and failed on the bad port", stderr.String())
	}
}

// TestRunServeHTTP_MalformedConfig_FailsToStart pins the deliberate
// fail-to-start choice (S3): a daemon that silently ignored a malformed
// config file would run with whatever defaults remained — the exact
// silent-misconfiguration class this whole feature exists to kill. The
// error must name the offending file so the launchd crash-loop's stderr is
// actionable. Documented in docs/configuration.md's daemon section
// (pre-flight with `fathom config` before restarting).
func TestRunServeHTTP_MalformedConfig_FailsToStart(t *testing.T) {
	userDir, _ := withIsolatedConfigHome(t)
	cfgPath := filepath.Join(userDir, ".fathom", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("this is not valid toml =\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runServeHTTP(nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (fail-to-start); stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), cfgPath) {
		t.Errorf("stderr = %s, want it to name the offending config file %s", stderr.String(), cfgPath)
	}
}

// TestRunServeHTTP_FlagOverridesDaemonConfig proves an explicit --store flag
// still wins over [daemon].store (flags > project, the file's stated
// precedence) — an explicit but nonexistent store dir surfaces store.Open's
// own error instead of silently using the configured path.
func TestRunServeHTTP_FlagOverridesDaemonConfig(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	configuredStore := filepath.Join(projectDir, "configured.sqlite")
	writeProjectConfig(t, projectDir, `
[daemon]
store = "`+escapeTOML(configuredStore)+`"
`)
	flagStore := filepath.Join(projectDir, "flag-store", "sub", "dir", "flag.sqlite")

	var stdout, stderr bytes.Buffer
	// A bad --addr keeps this from blocking on ListenAndServe once the store
	// opens successfully (store.Open auto-creates parent dirs, so flagStore
	// itself is not the failure point here — the invalid port is).
	code := runServeHTTP([]string{"--store", flagStore, "--addr", "127.0.0.1:999999"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected nonzero exit from the invalid port; stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "configured.sqlite") {
		t.Errorf("stderr = %s, must not reference the config-file store once --store is given", stderr.String())
	}
}
