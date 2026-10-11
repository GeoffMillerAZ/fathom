package main

// doctor_compile_test.go — GL-27 (G6): doctor names a configured LLM
// profile whose binary is gone, a daemon that started with compile_layer
// disabled, and stale rule layers. Fixture configs and a fake daemon only:
// no real claude or codex is ever executed (testisolate's PATH guard stays).

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func loadedWithProfiles(t *testing.T, toml string) config.Loaded {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := config.Load(p, filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return l
}

func TestProfileBins_StaleBinFailsNamingPathAndFix(t *testing.T) {
	stale := filepath.Join(t.TempDir(), ".dev-cache", "npm", "global", "bin", "claude")
	loaded := loadedWithProfiles(t, fmt.Sprintf("[profiles.sub]\nprovider = \"claude-cli\"\nbin = %q\n", stale))
	moved := "/home/u/.local/bin/claude"
	got := profileBinsVerdict(loaded, func(name string) (string, error) {
		if name == "claude" {
			return moved, nil
		}
		return "", errors.New("not found")
	})
	if got.Name != "profile_bins" || got.Status != doctorFail {
		t.Fatalf("got %+v, want a profile_bins FAIL", got)
	}
	for _, want := range []string{stale, "[profiles.sub]", "does not exist", moved} {
		if !strings.Contains(got.Reason, want) {
			t.Errorf("reason missing %q: %s", want, got.Reason)
		}
	}
	if !strings.Contains(got.Remedy, "[profiles.sub] bin") || !strings.Contains(got.Remedy, "~/.fathom/config.toml") {
		t.Errorf("remedy must name editing [profiles.sub] bin in ~/.fathom/config.toml: %s", got.Remedy)
	}
}

func TestProfileBins_ExistingBinPassesAndNoBinIsNotChecked(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	loaded := loadedWithProfiles(t, fmt.Sprintf("[profiles.a]\nprovider = \"claude-cli\"\nbin = %q\n[profiles.b]\nprovider = \"anthropic\"\n", bin))
	got := profileBinsVerdict(loaded, func(string) (string, error) { return "", errors.New("never consulted") })
	if got.Status != doctorPass {
		t.Fatalf("got %+v, want pass", got)
	}
	none := profileBinsVerdict(loadedWithProfiles(t, "[profiles.b]\nprovider = \"anthropic\"\n"), nil)
	if none.Status != doctorSkipped {
		t.Fatalf("no bin configured: got %+v, want skipped", none)
	}
}

func fakeStatusDaemon(t *testing.T, compileLayer string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cl := ""
		if compileLayer != "" {
			cl = `,"compile_layer":` + compileLayer
		}
		inner := `{"uptime_seconds":1` + cl + `}`
		fmt.Fprintf(w, `{"result":{"content":[{"type":"text","text":%q}]}}`, inner)
	}))
}

func TestCompileLayerState_DisabledDaemonFailsNamingReasonAndFix(t *testing.T) {
	reason := `claude-cli: binary "/x/claude" not found`
	srv := fakeStatusDaemon(t, fmt.Sprintf(`{"enabled":false,"configured":true,"reason":%q}`, reason))
	defer srv.Close()
	got := checkCompileLayerEnabled(http.DefaultClient, srv.URL, true)
	if got.Name != "compile_layer" || got.Status != doctorFail {
		t.Fatalf("got %+v, want a compile_layer FAIL", got)
	}
	if !strings.Contains(got.Reason, reason) {
		t.Errorf("reason must carry the daemon's own reason: %s", got.Reason)
	}
	if !strings.Contains(got.Remedy, "[profiles.<name>] bin") || !strings.Contains(got.Remedy, "~/.fathom/config.toml") {
		t.Errorf("remedy must name the config edit: %s", got.Remedy)
	}
}

func TestCompileLayerState_OtherAnswers(t *testing.T) {
	cases := []struct {
		name, body string
		daemonUp   bool
		want       doctorStatus
	}{
		{"enabled passes", `{"enabled":true,"configured":true,"reason":""}`, true, doctorPass},
		{"nothing configured is not a failure", `{"enabled":false,"configured":false,"reason":"no LLM provider configured"}`, true, doctorSkipped},
		{"older daemon with no block is skipped", ``, true, doctorSkipped},
		{"daemon down is skipped", ``, false, doctorSkipped},
	}
	for _, tc := range cases {
		srv := fakeStatusDaemon(t, tc.body)
		got := checkCompileLayerEnabled(http.DefaultClient, srv.URL, tc.daemonUp)
		srv.Close()
		if got.Status != tc.want {
			t.Errorf("%s: got %+v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestStaleLayers_WarnsPerServiceWithDryRunFirst(t *testing.T) {
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{
		services: []string{"alpha"}, obligRules: 2, watermark: "wm",
		obligLayerCoverage: &fakeLayerCoverage{SectionsTotal: 60, SectionsCompiled: 18, BudgetStopped: true},
	})
	defer srv.Close()
	got := checkStaleLayers(http.DefaultClient, srv.URL, namedDir(t, "alpha"), "", true)
	if got.Name != "layers_stale" || got.Status != doctorWarn {
		t.Fatalf("got %+v, want a layers_stale WARN", got)
	}
	if !strings.Contains(got.Reason, "alpha") || !strings.Contains(got.Reason, "18/60") {
		t.Errorf("reason must name the service and its sections compiled vs total: %s", got.Reason)
	}
	dry := strings.Index(got.Remedy, "fathom compile --service alpha --layer obligations --dry-run")
	spend := strings.Index(got.Remedy, "--dry-run=false")
	if dry < 0 || (spend >= 0 && spend < dry) {
		t.Errorf("remedy must name the --dry-run compile first: %s", got.Remedy)
	}
	if !strings.Contains(got.Remedy, "owner approval") {
		t.Errorf("remedy must say spending needs owner approval: %s", got.Remedy)
	}
}

func TestStaleLayers_FullyCompiledPassesAndDownIsSkipped(t *testing.T) {
	srv := fakeDoctorDaemon(t, fakeDaemonCfg{
		services: []string{"alpha"}, obligRules: 2, watermark: "wm",
		obligLayerCoverage: &fakeLayerCoverage{SectionsTotal: 60, SectionsCompiled: 60},
	})
	defer srv.Close()
	if got := checkStaleLayers(http.DefaultClient, srv.URL, namedDir(t, "alpha"), "", true); got.Status != doctorPass {
		t.Fatalf("got %+v, want pass", got)
	}
	if got := checkStaleLayers(http.DefaultClient, "http://127.0.0.1:1", t.TempDir(), "", false); got.Status != doctorSkipped {
		t.Fatalf("got %+v, want skipped", got)
	}
}

// GL-27: the daemon's startup records WHY compile_layer is off, so the
// state doctor reads names the stale bin (and nothing is executed: the
// claude-cli lane fails to resolve a missing path before any exec).
func TestWireLLM_StaleProfileBinRecordsCompileDisabledReason(t *testing.T) {
	for _, v := range []string{"FATHOM_CLAUDE_BIN", "FATHOM_LLM_PROVIDER", "FATHOM_COMPILE_MODEL", "ANTHROPIC_API_KEY"} {
		t.Setenv(v, "")
	}
	t.Setenv("PATH", t.TempDir())
	stale := filepath.Join(t.TempDir(), "gone", "claude")
	loaded := loadedWithProfiles(t, fmt.Sprintf("default_profile = \"sub\"\n[profiles.sub]\nprovider = \"claude-cli\"\nbin = %q\n", stale))
	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	r := &serveRun{flog: dlog.New(io.Discard, dlog.LevelInfo, time.Now), loaded: loaded, c: c}
	r.wireLLM()
	enabled, configured, reason := c.CompileLayerState()
	if enabled || !configured || !strings.Contains(reason, stale) {
		t.Fatalf("state = enabled %v configured %v reason %q, want a configured-but-off state naming %s", enabled, configured, reason, stale)
	}
}
