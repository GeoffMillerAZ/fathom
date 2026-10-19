package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
)

// O1233: on macOS, Background Task Management tracks the launchd agent with
// a launch constraint on its program's hash, every deploy changed that hash,
// and past BTM's per-item notification limit launchd stopped starting the
// daemon (exit 78). The agent's program is now fathomd: a copy of the fathom
// binary that `daemon install` writes once and no deploy rewrites, whose
// only rule is to exec its sibling fathom.

// fakeFathomBinary writes dir/fathom: bytes standing in for a build, with
// the launcher protocol marker a real build carries.
func fakeFathomBinary(t *testing.T, dir, build string) string {
	t.Helper()
	bin := filepath.Join(dir, "fathom")
	if err := os.WriteFile(bin, []byte("\x7fELF "+build+" "+launcherProtocolMarker+" end"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestDaemonPlistRunsTheStableLauncher(t *testing.T) {
	const bin = "/home/u/.local/libexec/fathom/fathom"
	body := []byte(plistBody("dev.fathom.serve-http", bin, "/Volumes/ext/fleet.sqlite", ":7581", "/home/u/.fathom/logs", "/Volumes/ext/mcp-calls.jsonl", "/usr/bin", nil))
	plutilLint(t, body)
	args := plistProgramArgs(body)
	if len(args) < 2 || args[0] != "/home/u/.local/libexec/fathom/fathomd" || args[1] != "serve-http" {
		t.Fatalf("ProgramArguments must start with the launcher beside the binary, then serve-http: %v", args)
	}
	// launchd enters WorkingDirectory before the launcher runs: the fathom
	// home, whose logs/ launchd already opens, never the store's volume.
	if got := topValue(t, body, "WorkingDirectory"); got != "/home/u/.fathom" {
		t.Fatalf("WorkingDirectory = %s, want the fathom home /home/u/.fathom", got)
	}
	if post, err := plistPosture(body); err != nil || len(postureDrift(post)) != 0 || post.Direct {
		t.Fatalf("the rendered launcher plist has no drift: %+v %v", post, err)
	}

	// --direct is today's shape: the binary itself, no WorkingDirectory.
	direct := []byte(plistBodyDirect("dev.fathom.serve-http", bin, "/Volumes/ext/fleet.sqlite", ":7581", "/home/u/.fathom/logs", "/Volumes/ext/mcp-calls.jsonl", "/usr/bin", nil))
	plutilLint(t, direct)
	if a := plistProgramArgs(direct); len(a) == 0 || a[0] != bin {
		t.Fatalf("--direct runs the binary itself: %v", a)
	}
	if got := topValue(t, direct, "WorkingDirectory"); got != "<absent>" {
		t.Fatalf("--direct carries no WorkingDirectory, got %s", got)
	}
	if bytes.Contains(direct, []byte(launcherName)) {
		t.Fatalf("--direct names no launcher:\n%s", direct)
	}

	// The install's choice: the launcher is written beside the binary (once),
	// --direct writes none.
	dir := t.TempDir()
	exe := fakeFathomBinary(t, dir, "build-1")
	got, launcher, wrote, err := installPlistBody(true, "l", exe, "/s", ":1", "/home/u/.fathom/logs", "/c", "/usr/bin", nil)
	if err != nil || launcher != "" || wrote || !strings.Contains(got, "<string>"+exe+"</string>") {
		t.Fatalf("--direct install: launcher=%q wrote=%v err=%v", launcher, wrote, err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, launcherName)); statErr == nil {
		t.Fatal("--direct must not write the launcher")
	}
	got, launcher, wrote, err = installPlistBody(false, "l", exe, "/s", ":1", "/home/u/.fathom/logs", "/c", "/usr/bin", nil)
	if err != nil || launcher != filepath.Join(dir, launcherName) || !wrote {
		t.Fatalf("install writes the launcher: launcher=%q wrote=%v err=%v", launcher, wrote, err)
	}
	if !strings.Contains(got, "<string>"+launcher+"</string>") {
		t.Fatalf("the plist names the launcher:\n%s", got)
	}
	src, _ := os.ReadFile(exe)
	if dst, _ := os.ReadFile(launcher); !bytes.Equal(src, dst) {
		t.Fatal("the launcher is a byte copy of the binary (its embedded signature included)")
	}
	if fi, _ := os.Stat(launcher); fi == nil || fi.Mode().Perm()&0o111 == 0 {
		t.Fatal("the launcher is executable")
	}
}

func TestLauncherExecsTheSiblingBinary(t *testing.T) {
	dir := t.TempDir()
	sibling := fakeFathomBinary(t, dir, "build-1")
	var gotPath string
	var gotArgv, gotEnv []string
	calls := 0
	sys := launcherSys{
		Stat: os.Stat,
		Exec: func(p string, argv, env []string) error {
			calls++
			gotPath, gotArgv, gotEnv = p, argv, env
			return nil
		},
		Executable: func() (string, error) { return filepath.Join(dir, launcherName), nil },
	}
	env := []string{"XPC_SERVICE_NAME=dev.fathom.serve-http", "PATH=/usr/bin"}
	argv := []string{filepath.Join(dir, launcherName), "serve-http", "--managed", "--store", "/s.sqlite"}
	var stderr bytes.Buffer
	code, handled := runLauncher(argv, env, sys, &stderr)
	if !handled || code != 0 || calls != 1 {
		t.Fatalf("fathomd must exec once: handled=%v code=%d calls=%d %s", handled, code, calls, stderr.String())
	}
	if gotPath != sibling || !reflect.DeepEqual(gotArgv, append([]string{sibling}, argv[1:]...)) || !reflect.DeepEqual(gotEnv, env) {
		t.Fatalf("exec(%q, %v, %v), want the sibling with the unchanged argv tail and env", gotPath, gotArgv, gotEnv)
	}

	// A bare name (run from PATH) resolves through the executable's path.
	calls = 0
	if _, handled := runLauncher([]string{launcherName, "version"}, env, sys, &stderr); !handled || gotPath != sibling || calls != 1 {
		t.Fatalf("bare fathomd: handled=%v exec %q", handled, gotPath)
	}

	// Any other name is not the launcher: nothing happens.
	calls = 0
	if _, handled := runLauncher([]string{sibling, "version"}, env, sys, &stderr); handled || calls != 0 {
		t.Fatalf("fathom itself must not take the launcher path: handled=%v calls=%d", handled, calls)
	}

	// A missing sibling exits 78 (EX_CONFIG) with one line naming it.
	empty := t.TempDir()
	stderr.Reset()
	code, handled = runLauncher([]string{filepath.Join(empty, launcherName), "serve-http"}, env, sys, &stderr)
	want := filepath.Join(empty, "fathom")
	if !handled || code != 78 || calls != 0 {
		t.Fatalf("missing sibling: handled=%v code=%d calls=%d", handled, code, calls)
	}
	if lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n"); len(lines) != 1 || !strings.Contains(lines[0], want) {
		t.Fatalf("missing sibling: want one stderr line naming %s, got %q", want, stderr.String())
	}

	// A failed exec also exits 78 with one line.
	sys.Exec = func(string, []string, []string) error { return errors.New("exec format error") }
	stderr.Reset()
	if code, _ := runLauncher(argv, env, sys, &stderr); code != 78 || !strings.Contains(stderr.String(), sibling) {
		t.Fatalf("failed exec: code=%d %q", code, stderr.String())
	}
}

// rowIDRe matches an internal row or record id (RT-14, O1233, ADR-066).
var rowIDRe = regexp.MustCompile(`\b[A-Z]{1,4}-?[0-9]+\b`)

func TestPostureReadsTheLaunchersSiblingAsTheBinary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(fathomhome.Env, "")
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	bin := fakeFathomBinary(t, binDir, "build-1")
	launcher := filepath.Join(binDir, launcherName)

	// A launcher plist: the daemon's binary is the sibling fathom.
	body := []byte(plistBody("com.example.mine", bin, "/data/fleet.sqlite", ":7581", filepath.Join(home, ".fathom", "logs"), "/data/c.jsonl", "/usr/bin", nil))
	post, err := plistPosture(body)
	if err != nil || post.Program != launcher || post.Binary != bin || post.Direct {
		t.Fatalf("launcher posture: program %q binary %q direct %v err %v", post.Program, post.Binary, post.Direct, err)
	}
	ops := newFakeDaemonOps(time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC))
	ops.env["CLAUDECODE"] = "1"
	if target, source, _ := runningDaemonBinary(ops, ":7581", post.Binary); target != bin || strings.Contains(source, launcherName) {
		t.Fatalf("with no running daemon the signature is read from %q (%s), want %s", target, source, bin)
	}

	// The version check reads fathom, not fathomd.
	v1 := strings.Replace(v1PlistFixture, "<string>/opt/fathom/fathom</string>", "<string>"+launcher+"</string>", 1)
	plistPath := filepath.Join(agents, "com.example.mine.plist")
	if err := os.WriteFile(plistPath, []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	ops.runOut[bin+" version --json"] = []byte(`{"version":"fathom v0.9.0","tenancy_contract":1}`)
	var out, errBuf bytes.Buffer
	if rc := runDaemonReconcile([]string{"--label", "com.example.mine"}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("reconcile of a launcher plist: rc=%d %s", rc, errBuf.String())
	}
	for _, c := range ops.calls {
		if strings.Contains(c, launcherName) {
			t.Fatalf("a check ran the launcher instead of the binary: %v", ops.calls)
		}
	}
	if !strings.Contains(strings.Join(ops.calls, "\n"), bin+" version --json") {
		t.Fatalf("the version check must read %s: %v", bin, ops.calls)
	}

	// A plist that runs the binary itself reads as drift, in plain words.
	direct := plistBodyDirect("com.example.mine", bin, "/data/fleet.sqlite", ":7581", filepath.Join(home, ".fathom", "logs"), "/data/c.jsonl", "/usr/bin", nil)
	direct = strings.Replace(direct, "<key>RunAtLoad</key>", "<key>WorkingDirectory</key><string>/Volumes/ext/repo</string>\n  <key>RunAtLoad</key>", 1)
	if err := os.WriteFile(plistPath, []byte(direct), 0o644); err != nil {
		t.Fatal(err)
	}
	c := checkDaemonPosture("com.example.mine")
	if c.Status != doctorWarn || !strings.Contains(c.Reason, "Background Task Management") || !strings.Contains(c.Reason, launcher) {
		t.Fatalf("a direct plist must WARN naming the cause and the launcher: %+v", c)
	}
	if id := rowIDRe.FindString(c.Reason); id != "" {
		t.Fatalf("the drift line names an internal id %q: %s", id, c.Reason)
	}
	if !strings.Contains(c.Remedy, "fathom daemon install --reconcile --label com.example.mine") {
		t.Fatalf("the remedy is the reconcile that migrates: %s", c.Remedy)
	}

	// --reconcile --direct keeps the direct shape and writes no launcher.
	out.Reset()
	errBuf.Reset()
	if rc := runDaemonReconcile([]string{"--label", "com.example.mine", "--direct"}, &out, &errBuf, ops); rc != 0 || !strings.Contains(out.String(), "nothing to reconcile") {
		t.Fatalf("--reconcile --direct on a direct schema-2 plist: rc=%d %s %s", rc, out.String(), errBuf.String())
	}
	if _, err := os.Stat(launcher); err == nil {
		t.Fatal("--direct must not write the launcher")
	}

	// The reconcile migrates it: the launcher is written beside the binary,
	// ProgramArguments[0] names it, WorkingDirectory leaves the store's volume.
	out.Reset()
	errBuf.Reset()
	if rc := runDaemonReconcile([]string{"--label", "com.example.mine"}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("migrate: rc=%d %s", rc, errBuf.String())
	}
	migrated, _ := os.ReadFile(plistPath)
	plutilLint(t, migrated)
	if a := plistProgramArgs(migrated); len(a) == 0 || a[0] != launcher {
		t.Fatalf("migrated ProgramArguments: %v", a)
	}
	if got := topValue(t, migrated, "WorkingDirectory"); got != filepath.Join(home, ".fathom") {
		t.Fatalf("migrated WorkingDirectory = %s", got)
	}
	if post, _ := plistPosture(migrated); len(postureDrift(post)) != 0 {
		t.Fatalf("a migrated plist still drifts: %v", postureDrift(post))
	}
	src, _ := os.ReadFile(bin)
	first, err := os.ReadFile(launcher)
	if err != nil || !bytes.Equal(first, src) {
		t.Fatalf("the launcher is a copy of the binary: %v", err)
	}
	for _, call := range ops.calls {
		if strings.HasPrefix(call, "launchctl") {
			t.Fatalf("an agent shell must never run launchctl: %v", ops.calls)
		}
	}

	// A new build (a deploy) never changes the launcher: install keeps a
	// launcher whose protocol is current.
	fakeFathomBinary(t, binDir, "build-2")
	if _, wrote, err := ensureLauncher(bin); err != nil || wrote {
		t.Fatalf("a current launcher must be kept: wrote=%v err=%v", wrote, err)
	}
	if again, _ := os.ReadFile(launcher); !bytes.Equal(again, first) {
		t.Fatal("the launcher's bytes changed: its hash is what launchd tracks")
	}
	// A launcher of another protocol is replaced.
	if err := os.WriteFile(launcher, []byte("fathom-launcher-protocol: 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, wrote, err := ensureLauncher(bin); err != nil || !wrote {
		t.Fatalf("a launcher of another protocol is rewritten: wrote=%v err=%v", wrote, err)
	}
	// A binary that predates the launcher cannot become one.
	old := filepath.Join(t.TempDir(), "fathom")
	if err := os.WriteFile(old, []byte("old build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ensureLauncher(old); err == nil {
		t.Fatal("a binary without the launcher rule must be refused")
	}
}

// The deploy swaps only fathom and kickstarts: it never writes fathomd, so
// the program launchd tracks keeps its hash across every deploy.
func TestDeployNeverRewritesTheLauncher(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRootForDocs(t), "Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	start := -1
	for i, l := range lines {
		if l == "  deploy:" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatal("Taskfile.yml has no deploy task")
	}
	taskRe := regexp.MustCompile(`^  [A-Za-z0-9:_-]+:\s*$`)
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if taskRe.MatchString(lines[i]) {
			end = i
			break
		}
	}
	deploy := strings.Join(lines[start:end], "\n")
	for _, want := range []string{`mv -f "$HOME/.local/libexec/fathom/fathom.new" "$HOME/.local/libexec/fathom/fathom"`, "launchctl kickstart -k"} {
		if !strings.Contains(deploy, want) {
			t.Fatalf("the deploy task no longer carries %q; re-read this guard against it:\n%s", want, deploy)
		}
	}
	if strings.Contains(deploy, launcherName) {
		t.Fatalf("the deploy task must never write the launcher %s:\n%s", launcherName, deploy)
	}
	// `daemon install` writes the launcher; the deploy may name it in a
	// hint, never run it.
	if m := regexp.MustCompile(`fathom daemon install\s+-|fathom" daemon install|bin/fathom daemon install`).FindString(deploy); m != "" {
		t.Fatalf("the deploy task must never run daemon install (%q):\n%s", m, deploy)
	}
}
