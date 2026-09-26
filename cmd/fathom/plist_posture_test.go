package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// v1PlistFixture is what `fathom daemon install` wrote before schema 2
// (unconditional KeepAlive, no throttle, no --managed), with an operator's
// own WorkingDirectory and environment added by hand.
const v1PlistFixture = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.example.mine</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/fathom/fathom</string>
    <string>serve-http</string>
    <string>--store</string><string>/data/fleet.sqlite</string>
    <string>--addr</string><string>:7575</string>
    <string>--call-log</string><string>/data/mcp-calls.jsonl</string>
  </array>
  <key>EnvironmentVariables</key><dict>
    <!-- BP-37: PATH alone was not enough. -->
    <key>PATH</key><string>/usr/local/go/bin:/usr/bin</string>
    <key>FATHOM_LOG</key><string>debug</string>
  </dict>
  <key>WorkingDirectory</key><string>/data/work</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>/data/logs/fathom-http.log</string>
  <key>StandardErrorPath</key><string>/data/logs/fathom-http.err</string>
</dict></plist>
`

// handWrittenPlist is a plist fathom did not write (this machine's own).
const handWrittenPlist = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.example.fathom-http</string>
	<key>ProgramArguments</key>
	<array>
		<string>/Users/x/.local/libexec/fathom/fathom</string>
		<string>serve-http</string>
		<string>--store</string>
		<string>/Volumes/x/fleet.sqlite</string>
	</array>
	<key>KeepAlive</key>
	<true/>
	<key>ThrottleInterval</key>
	<integer>10</integer>
</dict>
</plist>
`

func topValue(t *testing.T, raw []byte, key string) string {
	t.Helper()
	p, err := parsePlist(raw)
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, raw)
	}
	e, ok := p.entry(key)
	if !ok {
		return "<absent>"
	}
	return e.render()
}

func plutilLint(t *testing.T, raw []byte) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		return
	}
	if _, err := exec.LookPath("plutil"); err != nil {
		return
	}
	p := filepath.Join(t.TempDir(), "x.plist")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("plutil", "-lint", p).CombinedOutput(); err != nil {
		t.Fatalf("plutil -lint rejects the plist: %v %s\n%s", err, out, raw)
	}
}

// RT-13 + RT-14 (§8.1, §8.3): schema 2 carries the tenancy posture, and
// `--reconcile` brings an installed plist to it rewriting ONLY the tenancy
// keys — label, ProgramArguments paths, store, call log, WorkingDirectory
// and the environment are kept byte-for-byte; a plist fathom did not write
// needs --adopt; the command writes a backup and a receipt and refuses to
// run before the installed binary reports tenancy_contract >= 1.
func TestPlistV2RoundTripAndReconcileKeepsOperatorKeys(t *testing.T) {
	// Schema 2 as rendered: no drift, reconcile is a byte-identical no-op.
	body := []byte(plistBody("dev.fathom.serve-http", "/usr/local/bin/fathom", "/s.sqlite", ":7575", "/logs", "/logs/c.jsonl", "/usr/bin", []string{"GOPATH=/home/u/go"}))
	plutilLint(t, body)
	for key, want := range map[string]string{
		"KeepAlive":               "{SuccessfulExit: false}",
		"ThrottleInterval":        "60",
		"ProcessType":             "Background",
		"LowPriorityIO":           "true",
		"LowPriorityBackgroundIO": "true",
		"Nice":                    "5",
		"ExitTimeOut":             "20",
	} {
		if got := topValue(t, body, key); got != want {
			t.Fatalf("schema 2 %s = %s, want %s", key, got, want)
		}
	}
	if !strings.Contains(string(body), "<string>serve-http</string>\n    <string>--managed</string>") {
		t.Fatalf("schema 2 passes --managed right after serve-http:\n%s", body)
	}
	if !strings.Contains(string(body), "fathom-plist-schema: 2") {
		t.Fatal("schema 2 carries its marker")
	}
	post, err := plistPosture(body)
	if err != nil || len(post.Drift) != 0 || post.Schema != 2 || !post.FathomWritten {
		t.Fatalf("a rendered schema-2 plist has no drift: %+v %v", post, err)
	}
	same, changes, err := reconcilePlist(body, false)
	if err != nil || len(changes) != 0 || !bytes.Equal(same, body) {
		t.Fatalf("reconciling schema 2 must be a no-op: %v %v", changes, err)
	}

	// Schema 1 → 2: only tenancy keys change.
	v1 := []byte(v1PlistFixture)
	post, _ = plistPosture(v1)
	if post.Schema != 1 || !post.FathomWritten || len(post.Drift) == 0 || !post.UnconditionalKeepAlive || post.ThrottleSeconds != 10 {
		t.Fatalf("v1 posture: %+v", post)
	}
	got, changes, err := reconcilePlist(v1, false)
	if err != nil {
		t.Fatalf("reconcile v1: %v", err)
	}
	plutilLint(t, got)
	if len(changes) == 0 {
		t.Fatal("reconcile of v1 reported no changes")
	}
	if post, _ := plistPosture(got); len(post.Drift) != 0 || post.Schema != 2 {
		t.Fatalf("after reconcile the plist still drifts: %+v\n%s", post, got)
	}
	for _, key := range []string{"Label", "EnvironmentVariables", "WorkingDirectory", "RunAtLoad", "StandardOutPath", "StandardErrorPath"} {
		if a, b := topValue(t, v1, key), topValue(t, got, key); a != b {
			t.Fatalf("reconcile changed operator key %s: %s → %s", key, a, b)
		}
	}
	for _, keep := range []string{
		"<!-- BP-37: PATH alone was not enough. -->",
		"<key>FATHOM_LOG</key><string>debug</string>",
		"<string>--store</string><string>/data/fleet.sqlite</string>",
		"<string>--call-log</string><string>/data/mcp-calls.jsonl</string>",
		"<string>/opt/fathom/fathom</string>",
	} {
		if !strings.Contains(string(got), keep) {
			t.Fatalf("reconcile lost %q:\n%s", keep, got)
		}
	}
	if again, ch, _ := reconcilePlist(got, false); len(ch) != 0 || !bytes.Equal(again, got) {
		t.Fatalf("reconcile is not idempotent: %v", ch)
	}

	// A hand-written plist needs --adopt.
	if _, _, err := reconcilePlist([]byte(handWrittenPlist), false); !errors.Is(err, errPlistNeedsAdopt) {
		t.Fatalf("a hand-written plist must need --adopt, got %v", err)
	}
	adopted, _, err := reconcilePlist([]byte(handWrittenPlist), true)
	if err != nil {
		t.Fatal(err)
	}
	plutilLint(t, adopted)
	if post, _ := plistPosture(adopted); len(post.Drift) != 0 {
		t.Fatalf("adopted plist drifts: %+v\n%s", post, adopted)
	}
	if a, b := topValue(t, []byte(handWrittenPlist), "Label"), topValue(t, adopted, "Label"); a != b {
		t.Fatalf("adopt changed the label: %s → %s", a, b)
	}

	// The command: refusal before the binary speaks the contract, then the
	// backup, the receipt and the printed (not run) reload in an agent shell.
	home := t.TempDir()
	t.Setenv("HOME", home)
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	plistPath := filepath.Join(agents, "com.example.mine.plist")
	if err := os.WriteFile(plistPath, v1, 0o644); err != nil {
		t.Fatal(err)
	}
	ops := newFakeDaemonOps(time.Date(2026, 9, 26, 21, 7, 0, 0, time.UTC))
	ops.env["CLAUDECODE"] = "1"
	ops.runOut["/opt/fathom/fathom version --json"] = []byte(`fathom v0.8.0 @ abc`) // an old binary: no JSON
	var out, errBuf bytes.Buffer
	if rc := runDaemonReconcile([]string{"--label", "com.example.mine"}, &out, &errBuf, ops); rc == 0 {
		t.Fatalf("reconcile must refuse before the installed binary reports tenancy_contract >= 1:\n%s", out.String())
	}
	if !strings.Contains(errBuf.String(), "task deploy") {
		t.Fatalf("the refusal names the order (deploy first): %s", errBuf.String())
	}
	if raw, _ := os.ReadFile(plistPath); !bytes.Equal(raw, v1) {
		t.Fatal("a refused reconcile must not touch the plist")
	}

	ops.runOut["/opt/fathom/fathom version --json"] = []byte(`{"version":"fathom v0.9.0","tenancy_contract":1}`)
	out.Reset()
	errBuf.Reset()
	if rc := runDaemonReconcile([]string{"--label", "com.example.mine"}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("reconcile: rc=%d %s", rc, errBuf.String())
	}
	backup := plistPath + ".bak.20260926T210700Z"
	if raw, err := os.ReadFile(backup); err != nil || !bytes.Equal(raw, v1) {
		t.Fatalf("the backup must hold the original plist: %v", err)
	}
	receiptPath := filepath.Join(home, ".fathom", "daemon", "com.example.mine", "reconcile-20260926T210700Z.json")
	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("no receipt: %v", err)
	}
	var rec plistReceipt
	if err := json.Unmarshal(raw, &rec); err != nil || rec.Backup != backup || len(rec.Changes) == 0 || rec.SHA256Before == rec.SHA256After {
		t.Fatalf("receipt: %+v %v", rec, err)
	}
	for _, c := range ops.calls {
		if strings.HasPrefix(c, "launchctl") {
			t.Fatalf("an agent shell must never run launchctl: %v", ops.calls)
		}
	}
	if !strings.Contains(out.String(), "launchctl bootout gui/") || !strings.Contains(out.String(), "launchctl bootstrap gui/") {
		t.Fatalf("the reload line must be printed: %s", out.String())
	}
}
