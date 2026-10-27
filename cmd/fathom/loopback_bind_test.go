package main

// loopback_bind_test.go — ADR-083: a fresh `daemon install` listens on
// loopback and refuses a wider address; serve-http binds loopback in place
// of a wider one; reconcile rewrites a wide plist and doctor fails on one.
// The peer check is admission_peer_test.go's.

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubLaunchctl keeps a test's install from ever running launchctl (run from
// an agent's shell it wedges) and records what install would have run.
func stubLaunchctl(t *testing.T) *[]string {
	t.Helper()
	var calls []string
	prev := daemonInstallLaunchctl
	daemonInstallLaunchctl = func(args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		return nil, nil
	}
	t.Cleanup(func() { daemonInstallLaunchctl = prev })
	return &calls
}

// A fresh `daemon install` listens on loopback (defaultDaemonAddr), not
// every interface.
func TestDaemonInstallDefaultsToLoopback(t *testing.T) {
	if !hostIsLoopback(defaultDaemonAddr) {
		t.Fatalf("the install default %q must be loopback", defaultDaemonAddr)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := stubLaunchctl(t)
	bin := filepath.Join(t.TempDir(), "fathom")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	rc := cmdDaemonInstall([]string{"--label", "com.example.loopback", "--bin", bin, "--direct", "--store", filepath.Join(home, "s.sqlite")}, &out, &errb)
	if runtime.GOOS != "darwin" {
		if rc != 2 || !strings.Contains(errb.String(), "--addr "+defaultDaemonAddr) {
			t.Fatalf("off macOS the printed line must name the loopback default: rc=%d %s", rc, errb.String())
		}
		return
	}
	if rc != 0 {
		t.Fatalf("install: rc=%d\n%s\n%s", rc, out.String(), errb.String())
	}
	raw, err := os.ReadFile(filepath.Join(home, "Library", "LaunchAgents", "com.example.loopback.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "<string>--addr</string><string>"+defaultDaemonAddr+"</string>") {
		t.Fatalf("the default plist must listen on loopback:\n%s", raw)
	}
	if post, err := plistPosture(raw); err != nil || len(post.Drift) != 0 || post.Addr != defaultDaemonAddr {
		t.Fatalf("a fresh loopback plist has no drift: %+v %v", post, err)
	}
	if !strings.Contains(out.String(), "addr     "+defaultDaemonAddr) {
		t.Fatalf("install names the address it wrote:\n%s", out.String())
	}
	if len(*calls) != 2 {
		t.Fatalf("install runs bootout then bootstrap, got %v", *calls)
	}
}

// A wide --addr is refused (exit 2) with the named reason, before anything
// is written: remote serving needs token auth (Q5). --allow-non-loopback is
// gone: no opt-in widens the bind.
func TestDaemonInstallRefusesAWideAddr(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := stubLaunchctl(t)
	bin := filepath.Join(t.TempDir(), "fathom")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, addr := range []string{":7591", "0.0.0.0:7591", "[::]:7591", "192.168.1.20:7591", "fathom.example:7591"} {
		var out, errb bytes.Buffer
		rc := cmdDaemonInstall([]string{"--label", "com.example.wide", "--bin", bin, "--direct", "--addr", addr}, &out, &errb)
		if rc != 2 {
			t.Fatalf("install --addr %s: rc=%d, want 2\n%s", addr, rc, errb.String())
		}
		for _, want := range []string{addr + " is not loopback", "remote serving needs token auth, which fathom does not have yet (open question Q5)", "127.0.0.1:7591"} {
			if !strings.Contains(errb.String(), want) {
				t.Fatalf("the refusal must say %q: %s", want, errb.String())
			}
		}
	}
	var out, errb bytes.Buffer
	if rc := cmdDaemonInstall([]string{"--label", "com.example.wide", "--bin", bin, "--direct", "--addr", ":7591", "--allow-non-loopback"}, &out, &errb); rc == 0 || !strings.Contains(errb.String(), "allow-non-loopback") {
		t.Fatalf("--allow-non-loopback no longer exists: rc=%d %s", rc, errb.String())
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", "com.example.wide.plist")); err == nil {
		t.Fatal("a refused install must write no plist")
	}
	if len(*calls) != 0 {
		t.Fatalf("a refused install runs no launchctl: %v", *calls)
	}
}

// serve-http given a non-loopback --addr, or [daemon].addr in the user
// config, binds 127.0.0.1 on the same port instead of refusing, with one
// warning naming the configured address and the reconcile command: an
// install written before ADR-083 (--addr :7575) keeps serving after an
// upgrade. Every port here is unbindable, so a regression that binds the
// configured address fails fast and never listens.
func TestServeHTTPBindsLoopbackInPlaceOfAWideAddr(t *testing.T) {
	_, projectDir := withIsolatedConfigHome(t)
	t.Setenv("FATHOM_HOME", t.TempDir())
	store := filepath.Join(projectDir, "s.sqlite")
	configure := func(t *testing.T, args ...string) (*serveRun, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		r := &serveRun{args: append([]string{"--store", store, "--call-log", ""}, args...), stdout: &stdout, stderr: &stderr}
		t.Cleanup(r.runCleanups)
		if code, done := r.configure(); done {
			t.Fatalf("serve-http %v: configure stopped with %d, want it to go on to the bind:\n%s", args, code, stderr.String())
		}
		return r, stderr.String()
	}
	for _, addr := range []string{":999999", "0.0.0.0:999999", "[::]:999999", "[::ffff:0.0.0.0]:999999", "192.168.1.20:999999", "fathom.example:999999"} {
		r, stderr := configure(t, "--addr", addr)
		if r.addr != "127.0.0.1:999999" {
			t.Fatalf("--addr %s: serve-http would bind %q, want 127.0.0.1:999999", addr, r.addr)
		}
		for _, want := range []string{"WARN", "--addr " + addr + " is not loopback", "listening on 127.0.0.1:999999 instead", "fathom daemon install --reconcile", "open question Q5"} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("--addr %s: the warning must say %q:\n%s", addr, want, stderr)
			}
		}
		if n := strings.Count(stderr, "is not loopback"); n != 1 {
			t.Fatalf("--addr %s: one warning, got %d:\n%s", addr, n, stderr)
		}
	}
	// A loopback address is kept as given, with no warning.
	for _, addr := range []string{"127.0.0.1:999999", "[::1]:999999", "localhost:999999"} {
		if r, stderr := configure(t, "--addr", addr); r.addr != addr || strings.Contains(stderr, "is not loopback") {
			t.Fatalf("--addr %s: bound %q, stderr:\n%s", addr, r.addr, stderr)
		}
	}
	// The user config's [daemon].addr is named as the source.
	writeUserConfig(t, "[daemon]\naddr = \"0.0.0.0:999999\"\n")
	if r, stderr := configure(t); r.addr != "127.0.0.1:999999" || !strings.Contains(stderr, "[daemon].addr 0.0.0.0:999999 is not loopback") {
		t.Fatalf("[daemon].addr 0.0.0.0:999999: bound %q, stderr:\n%s", r.addr, stderr)
	}
	// --allow-non-loopback is gone.
	var stdout, stderr bytes.Buffer
	if code := runServeHTTP([]string{"--store", store, "--addr", ":999999", "--allow-non-loopback"}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "allow-non-loopback") {
		t.Fatalf("--allow-non-loopback no longer exists: code %d\n%s", code, stderr.String())
	}

	// End to end: the warning, then the bind fails on the unbindable port
	// without listening.
	stdout.Reset()
	stderr.Reset()
	if code := runServeHTTP([]string{"--store", store, "--addr", "[::]:999999", "--call-log", ""}, &stdout, &stderr); code == 0 {
		t.Fatalf("the unbindable port must fail the bind:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "listening on 127.0.0.1:999999 instead") || strings.Contains(stdout.String(), "serve-http: listening on") {
		t.Fatalf("serve-http --addr [::]:999999:\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}

// --reconcile rewrites a wide --addr to loopback on the same port and is
// idempotent; the command writes the result.
func TestReconcileRewritesAWidePlistToLoopback(t *testing.T) {
	wide := []byte(plistBody("dev.fathom.serve-http", "/usr/local/bin/fathom", "/s.sqlite", ":7591", "/logs", "/logs/c.jsonl", "/usr/bin", nil))
	post, err := plistPosture(wide)
	if err != nil || post.Addr != ":7591" {
		t.Fatalf("posture of a wide plist: %+v %v", post, err)
	}
	if !strings.Contains(strings.Join(post.Drift, "\n"), "--addr :7591 is not loopback (want 127.0.0.1:7591") {
		t.Fatalf("a wide plist is drift: %v", post.Drift)
	}
	got, changes, err := reconcilePlist(wide, false)
	if err != nil {
		t.Fatal(err)
	}
	plutilLint(t, got)
	if !strings.Contains(string(got), "<string>--addr</string><string>127.0.0.1:7591</string>") {
		t.Fatalf("reconcile must rewrite the address to loopback:\n%s", got)
	}
	if strings.Join(changes, "\n") != "ProgramArguments: --addr :7591 → 127.0.0.1:7591 (loopback; the daemon has no authentication)" {
		t.Fatalf("a schema-2 plist changes only its address: %q", changes)
	}
	if post, _ := plistPosture(got); len(post.Drift) != 0 {
		t.Fatalf("after reconcile the plist still drifts: %v", post.Drift)
	}
	if again, ch, _ := reconcilePlist(got, false); len(ch) != 0 || !bytes.Equal(again, got) {
		t.Fatalf("reconcile is not idempotent: %v", ch)
	}

	// The inline form and an IPv6 wildcard keep their port.
	inline := bytes.Replace(wide, []byte("<string>--addr</string><string>:7591</string>"), []byte("<string>--addr=[::]:7600</string>"), 1)
	got, _, err = reconcilePlist(inline, false)
	if err != nil || !strings.Contains(string(got), "<string>--addr=127.0.0.1:7600</string>") {
		t.Fatalf("reconcile of --addr=[::]:7600: %v\n%s", err, got)
	}

	// The command end to end.
	home := t.TempDir()
	t.Setenv("HOME", home)
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	plistPath := filepath.Join(agents, "dev.fathom.serve-http.plist")
	if err := os.WriteFile(plistPath, wide, 0o644); err != nil {
		t.Fatal(err)
	}
	ops := newFakeDaemonOps(time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC))
	ops.env["CLAUDECODE"] = "1"
	ops.runOut["/usr/local/bin/fathom version --json"] = []byte(`{"version":"fathom v0.23.0","tenancy_contract":1}`)
	var out, errb bytes.Buffer
	if rc := runDaemonReconcile([]string{"--label", "dev.fathom.serve-http"}, &out, &errb, ops); rc != 0 {
		t.Fatalf("reconcile: rc=%d %s", rc, errb.String())
	}
	raw, _ := os.ReadFile(plistPath)
	if !strings.Contains(string(raw), "<string>--addr</string><string>127.0.0.1:7591</string>") {
		t.Fatalf("the installed plist must now listen on loopback:\n%s", raw)
	}
	if !strings.Contains(out.String(), "--addr :7591 → 127.0.0.1:7591") {
		t.Fatalf("reconcile names the change:\n%s", out.String())
	}
}

// doctor's daemon_bind row FAILs on a wide plist or a running non-loopback
// listener, naming it with the one command that fixes it; it WARNs on a
// plist it cannot read and on a wide [daemon].addr in the user config
// (never a silent pass, I38); stale records and loopback listeners pass.
func TestDoctorDaemonBindRow(t *testing.T) {
	loop := &plistPostureReport{Addr: "127.0.0.1:7591", FathomWritten: true}
	wide := &plistPostureReport{Addr: ":7591", FathomWritten: true}
	cases := []struct {
		name         string
		facts        daemonBindFacts
		status       doctorStatus
		reason       []string
		remedy       string
		reasonAbsent string
	}{
		{name: "nothing wide", facts: daemonBindFacts{Label: "L", PlistPath: "/p/L.plist", Post: loop, CensusTaken: true, ConfigPath: "/h/config.toml", ConfigAddr: "127.0.0.1:7591", Rows: []censusRow{
			{PID: 10, Addr: "127.0.0.1:7591", Managed: true, Record: "record"},
			{PID: 11, Addr: "[::]:7591", Record: "stale"}, // a dead record is no listener
		}}, status: doctorPass, reason: []string{"loopback"}},
		{name: "no agent, no census", facts: daemonBindFacts{}, status: doctorSkipped},
		{name: "wide plist", facts: daemonBindFacts{Label: "L", PlistPath: "/p/L.plist", Post: wide, CensusTaken: true},
			status: doctorFail, reason: []string{"/p/L.plist", "--addr :7591", "127.0.0.1:7591", "no authentication"}, remedy: "fathom daemon install --reconcile --label L"},
		{name: "wide managed listener", facts: daemonBindFacts{Label: "L", PlistPath: "/p/L.plist", Post: loop, CensusTaken: true, Rows: []censusRow{
			{PID: 4242, Addr: "[::]:7591", Managed: true, Record: "record"},
		}}, status: doctorFail, reason: []string{"pid 4242 listens on [::]:7591"}, remedy: "launchctl kickstart -k gui/$(id -u)/L", reasonAbsent: "/p/L.plist"},
		{name: "wide scratch listener", facts: daemonBindFacts{CensusTaken: true, Rows: []censusRow{
			{PID: 4243, Addr: "0.0.0.0:7600", Record: "none"},
		}}, status: doctorFail, reason: []string{"pid 4243 listens on 0.0.0.0:7600"}, remedy: "kill 4243"},
		{name: "unreadable plist", facts: daemonBindFacts{Label: "L", PlistPath: "/p/L.plist", PlistErr: "plist: root is <array>, want <dict>", CensusTaken: true},
			status: doctorWarn, reason: []string{"/p/L.plist", "root is <array>"}, remedy: "plutil -lint /p/L.plist", reasonAbsent: "every fathom listener is loopback"},
		{name: "wide user config", facts: daemonBindFacts{CensusTaken: true, ConfigPath: "/h/config.toml", ConfigAddr: "0.0.0.0:7591"},
			status: doctorWarn, reason: []string{"/h/config.toml", "[daemon].addr 0.0.0.0:7591", "127.0.0.1:7591"}, remedy: `set addr = "127.0.0.1:7591" under [daemon] in /h/config.toml`},
		{name: "wide user config, no agent, no census", facts: daemonBindFacts{ConfigPath: "/h/config.toml", ConfigAddr: ":7591"},
			status: doctorWarn, reason: []string{"[daemon].addr :7591"}},
		{name: "unloadable user config", facts: daemonBindFacts{CensusTaken: true, ConfigPath: "/h/config.toml", ConfigErr: "toml: line 2: bad"},
			status: doctorWarn, reason: []string{"/h/config.toml", "toml: line 2: bad"}, remedy: "fathom config"},
		{name: "wide plist and wide user config", facts: daemonBindFacts{Label: "L", PlistPath: "/p/L.plist", Post: wide, CensusTaken: true, ConfigPath: "/h/config.toml", ConfigAddr: "0.0.0.0:7591"},
			status: doctorFail, reason: []string{"--addr :7591", "[daemon].addr 0.0.0.0:7591"}, remedy: "fathom daemon install --reconcile --label L"},
	}
	for _, c := range cases {
		got := daemonBindCheck(c.facts)
		if got.Name != "daemon_bind" || got.Status != c.status {
			t.Fatalf("%s: %+v, want status %s", c.name, got, c.status)
		}
		for _, w := range c.reason {
			if !strings.Contains(got.Reason, w) {
				t.Fatalf("%s: reason must name %q: %q", c.name, w, got.Reason)
			}
		}
		if c.reasonAbsent != "" && strings.Contains(got.Reason, c.reasonAbsent) {
			t.Fatalf("%s: reason must not name %q: %q", c.name, c.reasonAbsent, got.Reason)
		}
		if c.remedy != "" && got.Remedy != c.remedy {
			t.Fatalf("%s: remedy %q, want %q", c.name, got.Remedy, c.remedy)
		}
		if got.Status == doctorFail && got.Remedy == "" {
			t.Fatalf("%s: every fail carries a remedy", c.name)
		}
	}
}

// checkDaemonBind reads the census doctor's resources projection took (not
// only the pure row), the installed plist and the user config.
func TestDoctorDaemonBindReadsThisMachine(t *testing.T) {
	home, _ := withIsolatedConfigHome(t)
	t.Setenv("FATHOM_HOME", t.TempDir())
	t.Run("the census reaches the row", func(t *testing.T) {
		f := newResourcesFixture(t)
		f.census = []censusRow{{PID: 4242, Addr: "[::]:7591", Record: "none", Verdict: "ok"}}
		got := checkDaemonBind(buildResources(f.env(), "", "", 24*time.Hour))
		if got.Status != doctorFail || !strings.Contains(got.Reason, "pid 4242 listens on [::]:7591") || got.Remedy != "kill 4242" {
			t.Fatalf("a wide scratch listener in the census must FAIL daemon_bind: %+v", got)
		}
	})
	t.Run("a wide user [daemon].addr warns", func(t *testing.T) {
		writeUserConfig(t, "[daemon]\naddr = \"0.0.0.0:7591\"\n")
		f := newResourcesFixture(t)
		got := checkDaemonBind(buildResources(f.env(), "", "", 24*time.Hour))
		if got.Status != doctorWarn || !strings.Contains(got.Reason, "[daemon].addr 0.0.0.0:7591") {
			t.Fatalf("a wide [daemon].addr in the user config must WARN: %+v", got)
		}
	})
	t.Run("an unreadable plist warns", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("installed agents are launchd's")
		}
		t.Setenv("FATHOM_HOME", t.TempDir())
		agents := filepath.Join(home, "Library", "LaunchAgents")
		if err := os.MkdirAll(agents, 0o755); err != nil {
			t.Fatal(err)
		}
		// Found as a fathom agent, but its root is not a dict.
		body := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><array><string>/usr/local/bin/fathom</string><string>serve-http</string></array></plist>`
		if err := os.WriteFile(filepath.Join(agents, "dev.fathom.serve-http.plist"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkDaemonBind(resourcesReport{})
		if got.Status != doctorWarn || !strings.Contains(got.Reason, "dev.fathom.serve-http.plist") || !strings.Contains(got.Reason, "want <dict>") {
			t.Fatalf("an installed plist daemon_bind cannot read must WARN naming it: %+v", got)
		}
	})
}
