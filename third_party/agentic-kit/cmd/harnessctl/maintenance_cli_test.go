//go:build harness_maintenance

package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type cliNative struct {
	stamp     maintenanceHostStamp
	key       *ecdsa.PrivateKey
	status    string
	onApprove func()
	counts    maintenanceNativeCounts
	closed    bool
}

func (n *cliNative) Stamp(context.Context) (maintenanceHostStamp, error) {
	n.counts.StampAttempts++
	n.counts.StampInvoked++
	n.counts.StampCompleted++
	return n.stamp, nil
}
func (n *cliNative) Approve(_ context.Context, plan []byte, a maintenanceApprover) ([]byte, error) {
	n.counts.ApprovalAttempts++
	n.counts.ApprovalInvoked++
	if n.onApprove != nil {
		n.onApprove()
	}
	reply := maintenanceNativeApproval{Version: "harnessctl.native-approval/1", Status: n.status}
	if n.status == "signed" {
		pae := append([]byte("DSSEv1 "+strconv.Itoa(len(maintenancePayloadType))+" "+maintenancePayloadType+" "+strconv.Itoa(len(plan))+" "), plan...)
		h := sha256.Sum256(pae)
		sig, e := ecdsa.SignASN1(rand.Reader, n.key, h[:])
		if e != nil {
			return nil, e
		}
		reply.Signature = base64.StdEncoding.EncodeToString(sig)
		reply.PublicKey = a.PublicKey
	}
	raw, _ := json.Marshal(reply)
	n.counts.ApprovalCompleted++
	return raw, nil
}
func (n *cliNative) Counts() maintenanceNativeCounts { return n.counts }
func (n *cliNative) Close() error                    { n.closed = true; return nil }

type cliFixture struct {
	controllerPath, controllerHash, projectPath, candidatePath, stateRoot string
	f                                                                     fixture
	key                                                                   *ecdsa.PrivateKey
	native                                                                *cliNative
	entropyByte                                                           byte
}

func maintenanceCLIFixture(t *testing.T) cliFixture {
	t.Helper()
	f := toy(t)
	f.reg.Controls[0].AllowMaintenance = true
	f.layer.Controls[0].AllowMaintenance = true
	f.save(t)
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	base := filepath.Dir(f.registrationPath)
	stateRoot := filepath.Join(base, "state")
	if e := os.Mkdir(stateRoot, 0700); e != nil {
		t.Fatal(e)
	}
	nativePath := filepath.Join(base, "native")
	prepRoot := filepath.Join(base, "preparation")
	if e := os.Mkdir(prepRoot, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(nativePath, []byte("fake"), 0700); e != nil {
		t.Fatal(e)
	}
	nh := sha256.Sum256([]byte("fake"))
	a := maintenanceApprover{ID: "human", PrincipalID: "operator", OSUserID: "501", KeyTag: "human-key", PublicKey: base64.StdEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), key.X, key.Y)), EnrollmentProfile: "secure-enclave-user-presence/1", Revoked: false, Roles: []string{"maintenance", "rearm-baseline"}, ControlIDs: []string{"review"}, MaxDurationSeconds: 300}
	c := maintenanceController{Version: "harnessctl.controller/1", Epoch: 1, IssuedAt: "2026-10-24T00:00:00Z", ExpiresAt: "2026-10-25T00:00:00Z", HostID: "host", Native: maintenanceNative{Profile: "darwin-protected-key/1", Executable: nativePath, SHA256: hex.EncodeToString(nh[:]), PreparationRoot: prepRoot, TimeoutMillis: 1000}, Approvers: []maintenanceApprover{a}, Projects: []maintenanceProject{{ID: "project", Root: f.root, PrincipalID: "operator", Registration: f.registrationPath, StateRoot: stateRoot}}}
	raw, _ := json.Marshal(c)
	controllerPath := filepath.Join(base, "controller.json")
	if e := os.WriteFile(controllerPath, raw, 0600); e != nil {
		t.Fatal(e)
	}
	ch := sha256.Sum256(raw)
	candidate := maintenanceCandidate{Version: "harnessctl.advisory-candidate/1", Purpose: "maintenance", ApproverID: "human", DurationSeconds: 30, Reason: "Short diagnostic", Controls: []maintenanceCandidateControl{{ID: "review", SamplePPM: 200000}}}
	candidateRaw, _ := json.Marshal(candidate)
	candidatePath := filepath.Join(base, "candidate.json")
	if e := os.WriteFile(candidatePath, candidateRaw, 0600); e != nil {
		t.Fatal(e)
	}
	stamp := maintenanceHostStamp{Version: "harnessctl.host-stamp/1", OS: "darwin", OSUserID: "501", BootID: "boot", UTCMicros: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC).UnixMicro(), ContinuousNanos: 1e11}
	return cliFixture{controllerPath, hex.EncodeToString(ch[:]), f.root, candidatePath, stateRoot, f, key, &cliNative{stamp: stamp, key: key, status: "signed"}, 7}
}
func (f cliFixture) args(op string) []string {
	out := []string{op, "--controller", f.controllerPath, "--controller-sha256", f.controllerHash, "--project", f.projectPath}
	if op == "prepare" {
		out = append(out, "--candidate", f.candidatePath)
	}
	return out
}
func (f cliFixture) deps() maintenanceCLIDeps {
	return maintenanceCLIDeps{read: readSnapshot, newNative: func(maintenanceNative) (maintenanceNativePort, error) { return f.native, nil }, newStore: func(root string, fault func(string) error) (maintenanceStorePort, error) {
		return openMaintenanceFileStore(root, fault)
	}, entropy: bytes.NewReader(bytes.Repeat([]byte{f.entropyByte}, 128))}
}
func cliCall(t *testing.T, f cliFixture, op string) (maintenanceReport, int, string) {
	t.Helper()
	var out bytes.Buffer
	code := runMaintenanceWithDeps(f.args(op), &out, f.deps())
	var report maintenanceReport
	if e := strictJSON(out.Bytes(), &report); e != nil {
		t.Fatalf("bad report %s: %s", e, out.String())
	}
	if validateMaintenanceReport(report) != nil {
		t.Fatal("invalid report")
	}
	return report, code, out.String()
}
func TestMaintenanceCLIFlowAndPrivacy(t *testing.T) {
	f := maintenanceCLIFixture(t)
	for _, tc := range []struct{ op, phase string }{{"init", "armed"}, {"prepare", "prepared"}, {"enter", "maintenance"}, {"check", "maintenance"}, {"end", "armed"}} {
		f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
		r, code, out := cliCall(t, f, tc.op)
		if code != 0 || r.Status != "complete" || r.Phase != tc.phase {
			t.Fatalf("%s: %v %v", tc.op, code, r)
		}
		for _, secret := range []string{f.projectPath, f.controllerPath, f.controllerHash, "Short diagnostic", "200000", "human-key", "approvalPayload", "effectiveControls"} {
			if strings.Contains(out, secret) {
				t.Fatalf("%s leaked %q", tc.op, secret)
			}
		}
		if tc.op == "enter" && (!r.NativeSignatureVerified || !r.StoredSignatureVerified || r.NativeApprovalCompleted != 1) {
			t.Fatalf("missing signature receipt %+v", r)
		}
		if tc.op == "check" && (!r.StoredSignatureVerified || r.NativeApprovalAttempts != 0) {
			t.Fatalf("stored proof receipt %+v", r)
		}
	}
}
func TestMaintenanceCLIConsumeAndDeny(t *testing.T) {
	f := maintenanceCLIFixture(t)
	_, _, _ = cliCall(t, f, "init")
	_, _, _ = cliCall(t, f, "prepare")
	f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "denied"}
	r, code, _ := cliCall(t, f, "enter")
	if code != 1 || r.Code != "native_denied" || r.StateWritesCommitted != 2 || r.NativeSignatureVerified {
		t.Fatal(r, code)
	}
	f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
	r, code, _ = cliCall(t, f, "enter")
	if code == 0 || r.NativeApprovalAttempts != 0 {
		t.Fatal("replay accepted", r, code)
	}
}
func TestMaintenanceCLIChangedControllerAfterBegin(t *testing.T) {
	f := maintenanceCLIFixture(t)
	_, _, _ = cliCall(t, f, "init")
	_, _, _ = cliCall(t, f, "prepare")
	original, e := os.ReadFile(f.controllerPath)
	if e != nil {
		t.Fatal(e)
	}
	f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "signed", onApprove: func() { _ = os.WriteFile(f.controllerPath, append(append([]byte(nil), original...), '\n'), 0600) }}
	r, code, _ := cliCall(t, f, "enter")
	if code == 0 || r.Code != "controller_changed" || r.StateWritesCommitted != 1 {
		t.Fatal(r, code)
	}
	if e := os.WriteFile(f.controllerPath, original, 0600); e != nil {
		t.Fatal(e)
	}
	f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
	r, code, _ = cliCall(t, f, "enter")
	if code == 0 || r.NativeApprovalAttempts != 0 {
		t.Fatal("consumed nonce retried", r, code)
	}
}
func TestMaintenanceCLIInputAndWriter(t *testing.T) {
	f := maintenanceCLIFixture(t)
	for _, args := range [][]string{{"prepare", "--controller", f.controllerPath}, {"init", "--candidate", ""}, {"init", "--unknown", "x"}, {"bogus"}, append(f.args("init"), "--project", f.projectPath), {"init", "-controller", f.controllerPath}} {
		var out bytes.Buffer
		code := runMaintenanceWithDeps(args, &out, f.deps())
		if code != 2 || !strings.Contains(out.String(), "input_invalid") {
			t.Fatalf("args %v: %d %s", args, code, out.String())
		}
	}
	bad := rejectWriter{}
	if code := runMaintenanceWithDeps(f.args("init"), bad, f.deps()); code != 2 {
		t.Fatal(code)
	}
	r, code, _ := cliCall(t, f, "init")
	if code == 0 || r.Code != "state_exists" {
		t.Fatal("writer error retried init", r, code)
	}
}

type rejectWriter struct{}

func (rejectWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

var _ io.Writer = rejectWriter{}

func TestMaintenanceCLILateApprovalDoesNotExtendLease(t *testing.T) {
	f := maintenanceCLIFixture(t)
	_, _, _ = cliCall(t, f, "init")
	_, _, _ = cliCall(t, f, "prepare")
	n := &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
	n.onApprove = func() { n.stamp.UTCMicros += 31e6; n.stamp.ContinuousNanos += 31e9 }
	f.native = n
	r, code, _ := cliCall(t, f, "enter")
	if code == 0 || r.Code != "proposal_expired" || r.StateWritesCommitted != 2 || r.NativeSignatureVerified {
		t.Fatal(r, code)
	}
	f.native = &cliNative{stamp: n.stamp, key: f.key, status: "signed"}
	checked, exit, _ := cliCall(t, f, "check")
	if exit != 0 || checked.Phase != "armed" || checked.StoredSignatureVerified {
		t.Fatal(checked, exit)
	}
}
func TestMaintenanceCLISourceChangeHolds(t *testing.T) {
	f := maintenanceCLIFixture(t)
	_, _, _ = cliCall(t, f, "init")
	_, _, _ = cliCall(t, f, "prepare")
	f.f.layer.Controls[0].MaxSamplePPM = 800000
	f.f.save(t)
	f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
	r, code, _ := cliCall(t, f, "enter")
	if code == 0 || r.NativeApprovalAttempts != 0 || r.StateWritesCommitted != 0 {
		t.Fatal(r, code)
	}
}
func TestMaintenanceCLITextReport(t *testing.T) {
	f := maintenanceCLIFixture(t)
	args := append(f.args("init"), "--format", "text")
	var out bytes.Buffer
	code := runMaintenanceWithDeps(args, &out, f.deps())
	if code != 0 || !bytes.Contains(out.Bytes(), []byte("\n  \"status\": \"complete\"")) {
		t.Fatal(code, out.String())
	}
	var m map[string]any
	if e := json.Unmarshal(out.Bytes(), &m); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"projectId", "snapshotSHA256", "effectiveControls", "plan", "reason"} {
		if _, ok := m[key]; ok {
			t.Fatalf("private key %s", key)
		}
	}
}

func TestMaintenanceCLIReportCUE(t *testing.T) {
	cue, e := exec.LookPath("cue")
	if e != nil {
		t.Skip("cue unavailable")
	}
	f := maintenanceCLIFixture(t)
	for _, format := range []string{"json", "text"} {
		args := append(f.args("init"), "--format", format)
		var out bytes.Buffer
		_ = runMaintenanceWithDeps(args, &out, f.deps())
		path := filepath.Join(t.TempDir(), "report.json")
		if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		cmd := exec.Command(cue, "vet", "-c", "schema.cue", "maintenance_schema.cue", "maintenance_store_schema.cue", path, "-d", "#MaintenanceReport")
		if result, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("%s cue: %v %s", format, e, result)
		}
	}
}

func TestMaintenanceCLIClockChangeAfterApprovalLocks(t *testing.T) {
	f := maintenanceCLIFixture(t)
	_, _, _ = cliCall(t, f, "init")
	_, _, _ = cliCall(t, f, "prepare")
	n := &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
	n.onApprove = func() { n.stamp.UTCMicros-- }
	f.native = n
	r, code, _ := cliCall(t, f, "enter")
	if code == 0 || r.Code != "clock_invalid" || r.StateWritesCommitted != 2 || r.NativeSignatureVerified {
		t.Fatal(r, code)
	}
	f.native = &cliNative{stamp: n.stamp, key: f.key, status: "signed"}
	checked, exit, _ := cliCall(t, f, "check")
	if exit == 0 || checked.StoredSignatureVerified {
		t.Fatal(checked, exit)
	}
}
func TestMaintenanceCLIBusyAndCandidateCap(t *testing.T) {
	f := maintenanceCLIFixture(t)
	s, e := openMaintenanceFileStore(f.stateRoot, nil)
	if e != nil {
		t.Fatal(e)
	}
	r, code, _ := cliCall(t, f, "init")
	if code == 0 || r.Code != "state_busy" || r.StateWritesAttempted != 0 {
		t.Fatal(r, code)
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	_, _, _ = cliCall(t, f, "init")
	if e := os.WriteFile(f.candidatePath, bytes.Repeat([]byte{'x'}, 8193), 0600); e != nil {
		t.Fatal(e)
	}
	r, code, _ = cliCall(t, f, "prepare")
	if code == 0 || r.Code != "candidate_invalid" || r.StateWritesAttempted != 0 {
		t.Fatal(r, code)
	}
}

func TestMaintenanceCLIRecoverCreatesLocked(t *testing.T) {
	f := maintenanceCLIFixture(t)
	_, _, _ = cliCall(t, f, "init")
	f.entropyByte = 8
	if e := os.WriteFile(filepath.Join(f.stateRoot, "PENDING"), []byte("interrupted"), 0600); e != nil {
		t.Fatal(e)
	}
	r, code, _ := cliCall(t, f, "recover")
	if code != 0 || r.Phase != "locked" || r.StateWritesCommitted != 1 {
		t.Fatal(r, code)
	}
	f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
	r, code, _ = cliCall(t, f, "check")
	if code == 0 || r.Code != "rearm_required" {
		t.Fatal(r, code)
	}
}

func TestMaintenanceCLIForeignControllerCannotRecoverOrRearm(t *testing.T) {
	f := maintenanceCLIFixture(t)
	_, _, _ = cliCall(t, f, "init")
	original, e := os.ReadFile(f.controllerPath)
	if e != nil {
		t.Fatal(e)
	}
	var foreign maintenanceController
	if e := strictJSON(original, &foreign); e != nil {
		t.Fatal(e)
	}
	foreign.Epoch = 2
	foreignRaw, _ := json.Marshal(foreign)
	foreignHash := sha256.Sum256(foreignRaw)
	if e := os.WriteFile(filepath.Join(f.stateRoot, "PENDING"), []byte("bad"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(f.controllerPath, foreignRaw, 0600); e != nil {
		t.Fatal(e)
	}
	f.controllerHash = hex.EncodeToString(foreignHash[:])
	f.entropyByte = 8
	f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
	r, code, _ := cliCall(t, f, "recover")
	if code == 0 || r.Code != "binding_mismatch" || r.StateWritesCommitted != 0 {
		t.Fatal(r, code)
	}
	if e := os.WriteFile(f.controllerPath, original, 0600); e != nil {
		t.Fatal(e)
	}
	oldHash := sha256.Sum256(original)
	f.controllerHash = hex.EncodeToString(oldHash[:])
	f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
	r, code, _ = cliCall(t, f, "recover")
	if code != 0 || r.Phase != "locked" {
		t.Fatal(r, code)
	}
	if e := os.WriteFile(f.controllerPath, foreignRaw, 0600); e != nil {
		t.Fatal(e)
	}
	f.controllerHash = hex.EncodeToString(foreignHash[:])
	candidate := maintenanceCandidate{Version: "harnessctl.advisory-candidate/1", Purpose: "rearm-baseline", ApproverID: "human", DurationSeconds: 0, Reason: "Restore baseline", Controls: []maintenanceCandidateControl{}}
	candidateRaw, _ := json.Marshal(candidate)
	if e := os.WriteFile(f.candidatePath, candidateRaw, 0600); e != nil {
		t.Fatal(e)
	}
	f.native = &cliNative{stamp: f.native.stamp, key: f.key, status: "signed"}
	r, code, _ = cliCall(t, f, "prepare")
	if code == 0 || r.StateWritesCommitted != 0 {
		t.Fatal(r, code)
	}
}
