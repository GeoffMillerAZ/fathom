package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

const codesignAdhoc = `Executable=/opt/fathom/fathom
Identifier=a.out
Format=Mach-O thin (arm64)
CodeDirectory v=20400 size=123 flags=0x20002(adhoc,linker-signed) hashes=1+0 location=embedded
Signature=adhoc
Info.plist=not bound
TeamIdentifier=not set
`

const codesignIdentity = `Executable=/opt/fathom/fathom
Identifier=fathom
Format=Mach-O thin (arm64)
CodeDirectory v=20500 size=123 flags=0x0(none) hashes=1+2 location=embedded
Signature size=1712
Authority=fathom-dev
Signed Time=26 Sep 2026 at 21:07:00
TeamIdentifier=not set
`

// RT-148 (§8.4): `fathom daemon status` reports the RUNNING binary's
// codesigning identity, and names an ad-hoc signature — the cause of the
// TCC wedge (a grant pinned to one build's hash, voided by every rebuild).
func TestDaemonStatusReportsSigningIdentity(t *testing.T) {
	ops := newFakeDaemonOps(stopT0)
	ops.runOut["codesign -dv /opt/fathom/fathom"] = []byte(codesignAdhoc)
	si := binarySigningIdentity(ops, "/opt/fathom/fathom")
	if !si.AdHoc || si.Identifier != "a.out" || si.Authority != "" {
		t.Fatalf("ad-hoc parse: %+v", si)
	}
	if line := si.String(); !strings.Contains(line, "AD-HOC") || !strings.Contains(line, "a.out") || !strings.Contains(line, "task deploy") {
		t.Fatalf("an ad-hoc signature must be named with its cause and remedy: %q", line)
	}
	ops.runOut["codesign -dv /opt/fathom/fathom"] = []byte(codesignIdentity)
	si = binarySigningIdentity(ops, "/opt/fathom/fathom")
	if si.AdHoc || si.Authority != "fathom-dev" || !strings.Contains(si.String(), "fathom-dev") {
		t.Fatalf("identity parse: %+v %q", si, si.String())
	}
	ops.runOut["codesign -dv /opt/fathom/fathom"] = []byte("/opt/fathom/fathom: code object is not signed at all\n")
	ops.runErr["codesign -dv /opt/fathom/fathom"] = errors.New("exit status 1")
	if si = binarySigningIdentity(ops, "/opt/fathom/fathom"); !si.Unsigned || !strings.Contains(si.String(), "unsigned") {
		t.Fatalf("unsigned parse: %+v", si)
	}

	// The status verb resolves the running binary from the daemon record's
	// pid (not this CLI's own binary) and prints its signature.
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") }))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")
	p, _ := console.DaemonRecordPath(home, addr)
	if err := console.WriteDaemonRecord(p, console.NewDaemonRecord(addr, 4242, "", "", "", stopT0)); err != nil {
		t.Fatal(err)
	}
	ops = newFakeDaemonOps(stopT0.Add(time.Hour))
	ops.procs[4242] = procInfo{PID: 4242, Start: stopT0.Add(-time.Second), Command: "/opt/fathom/fathom serve-http --managed"}
	// The running process is ad-hoc; the file now at its path was replaced
	// (a deploy's mv) by a signed one. Status must read the PROCESS.
	ops.runOut["codesign -dv 4242"] = []byte(codesignAdhoc)
	ops.runOut["codesign -dv /opt/fathom/fathom"] = []byte(codesignIdentity)
	var out, errBuf bytes.Buffer
	if rc := runDaemonStatus([]string{"--url", srv.URL}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("status: rc=%d %s %s", rc, out.String(), errBuf.String())
	}
	if runtime.GOOS == "darwin" {
		if !strings.Contains(out.String(), "signing:") || !strings.Contains(out.String(), "AD-HOC") || !strings.Contains(out.String(), "/opt/fathom/fathom") || !strings.Contains(out.String(), "running pid 4242") {
			t.Fatalf("status must name the running binary's signature:\n%s", out.String())
		}
	} else if strings.Contains(out.String(), "signing:") || strings.Contains(strings.Join(ops.calls, "\n"), "codesign") {
		// Off macOS there is no codesign and no TCC grant to pin to a
		// signature (CS-8: the Linux cloud VMs): status reads none and
		// prints none.
		t.Fatalf("status must not read or print a code signature on %s:\n%s\ncalls: %v", runtime.GOOS, out.String(), ops.calls)
	}
	out.Reset()
	runDaemonStatus([]string{"--url", srv.URL, "--json"}, &out, &errBuf, ops)
	var st daemonStatusJSON
	if runtime.GOOS == "darwin" {
		if err := json.Unmarshal(out.Bytes(), &st); err != nil || st.Signing == nil || !st.Signing.AdHoc || st.Signing.Binary != "/opt/fathom/fathom" || st.Signing.Source != "running pid 4242" {
			t.Fatalf("status --json must carry the running process's signature: %v %s", err, out.String())
		}
	} else if err := json.Unmarshal(out.Bytes(), &st); err != nil || st.Signing != nil || st.Platform != runtime.GOOS {
		t.Fatalf("status --json must name the platform and carry no signature on %s: %v %s", runtime.GOOS, err, out.String())
	}

	// No running daemon: the reading is of the on-disk file, and says so.
	ops = newFakeDaemonOps(stopT0.Add(time.Hour))
	ops.runOut["codesign -dv /opt/fathom/fathom"] = []byte(codesignIdentity)
	target, source, _ := runningDaemonBinary(ops, "127.0.0.1:1", "/opt/fathom/fathom")
	si = binarySigningIdentityOf(ops, target, source)
	if si.Binary != "/opt/fathom/fathom" || !strings.Contains(si.String(), "on-disk") {
		t.Fatalf("a reading of the file on disk must say so: %+v %q", si, si.String())
	}
}

// RT-15 (§8.3): doctor's daemon_posture row and `daemon status` name plist
// drift with the exact reconcile command for the resolved label (with
// --adopt for a hand-written plist) followed by the reload line; the reload
// is printed, never run, from an agent shell — and run from a terminal.
func TestDoctorDaemonPostureNamesReconcile(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd plists are darwin-only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if c := checkDaemonPosture(""); c.Status != doctorSkipped {
		t.Fatalf("no plist installed: skip, got %+v", c)
	}
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	plistPath := filepath.Join(agents, "com.example.mine.plist")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(plistPath, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(v1PlistFixture)
	c := checkDaemonPosture("")
	if c.Name != "daemon_posture" || c.Status != doctorWarn {
		t.Fatalf("a v1 plist must WARN (never FAIL): %+v", c)
	}
	want := "fathom daemon install --reconcile --label com.example.mine"
	if !strings.Contains(c.Remedy, want) || strings.Contains(c.Remedy, "--adopt") {
		t.Fatalf("the remedy must be the exact reconcile command %q: %q", want, c.Remedy)
	}
	reload := fmt.Sprintf("launchctl bootout gui/%d/com.example.mine && launchctl bootstrap gui/%d %s", os.Getuid(), os.Getuid(), plistPath)
	if !strings.Contains(c.Remedy, reload) || strings.Index(c.Remedy, want) > strings.Index(c.Remedy, "launchctl") {
		t.Fatalf("the remedy must name reconcile THEN the reload line %q: %q", reload, c.Remedy)
	}
	if !strings.Contains(c.Reason, "KeepAlive: true") || !strings.Contains(c.Reason, "ThrottleInterval: absent") {
		t.Fatalf("the reason must name the drifted keys: %q", c.Reason)
	}

	// daemon status carries the same line.
	ops := newFakeDaemonOps(stopT0)
	var out, errBuf bytes.Buffer
	runDaemonStatus([]string{"--url", "http://127.0.0.1:1"}, &out, &errBuf, ops)
	if !strings.Contains(out.String(), "posture:") || !strings.Contains(out.String(), want) {
		t.Fatalf("daemon status must name the drift and the reconcile command:\n%s", out.String())
	}

	write(handWrittenPlist)
	if c := checkDaemonPosture(""); !strings.Contains(c.Remedy, want+" --adopt") {
		t.Fatalf("a hand-written plist's remedy needs --adopt: %q", c.Remedy)
	}

	write(plistBody("com.example.mine", "/opt/fathom/fathom", "/s", ":7581", "/l", "/c", "/usr/bin", nil))
	if c := checkDaemonPosture(""); c.Status != doctorPass {
		t.Fatalf("a schema-2 plist passes: %+v", c)
	}

	// From a terminal (no agent env) reconcile runs the reload itself.
	write(v1PlistFixture)
	ops = newFakeDaemonOps(stopT0)
	ops.runOut["/opt/fathom/fathom version --json"] = []byte(`{"tenancy_contract":1}`)
	ops.runOut[fmt.Sprintf("launchctl bootout gui/501/com.example.mine")] = nil
	ops.runOut[fmt.Sprintf("launchctl bootstrap gui/501 %s", plistPath)] = nil
	out.Reset()
	if rc := runDaemonReconcile([]string{"--reconcile"}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("reconcile from a terminal: rc=%d %s", rc, errBuf.String())
	}
	ran := strings.Join(ops.calls, "\n")
	if !strings.Contains(ran, "launchctl bootout gui/501/com.example.mine") || !strings.Contains(ran, "launchctl bootstrap gui/501 "+plistPath) {
		t.Fatalf("from a terminal the reload runs: %v", ops.calls)
	}
}
