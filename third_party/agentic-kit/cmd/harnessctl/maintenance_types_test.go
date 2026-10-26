//go:build harness_maintenance

package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func maintenanceTestPoint() string {
	b := make([]byte, 32)
	b[31] = 1
	x, y := elliptic.P256().ScalarBaseMult(b)
	return base64.StdEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y))
}
func maintenanceTestApprover() maintenanceApprover {
	return maintenanceApprover{ID: "human", PrincipalID: "person", OSUserID: "501", KeyTag: "tag", PublicKey: maintenanceTestPoint(), EnrollmentProfile: "secure-enclave-user-presence/1", Roles: []string{"maintenance", "rearm-baseline"}, ControlIDs: []string{"a", "b"}, MaxDurationSeconds: 300}
}
func maintenanceTestCandidate() maintenanceCandidate {
	return maintenanceCandidate{Version: "harnessctl.advisory-candidate/1", Purpose: "maintenance", ApproverID: "human", DurationSeconds: 30, Reason: "Measured diagnostic window", Controls: []maintenanceCandidateControl{{ID: "a", SamplePPM: 200}}}
}
func maintenanceTestEffective() []effectiveControl {
	return []effectiveControl{{ID: "a", Class: "advisory", AllowMaintenance: true, MinSamplePPM: 100, MaxSamplePPM: 300, SamplePPM: 150}, {ID: "b", Class: "mandatory", AllowMaintenance: false, MinSamplePPM: 1000000, MaxSamplePPM: 1000000, SamplePPM: 1000000}}
}
func maintenanceTestPlan() maintenancePlan {
	h := hex.EncodeToString(bytes.Repeat([]byte{1}, 32))
	return maintenancePlan{Version: "harnessctl.maintenance-plan/1", Purpose: "maintenance", InstanceID: h, AuthorizationEpoch: 2, Nonce: h, ControllerEpoch: 1, ControllerSHA256: h, RegistrationEpoch: 1, RegistrationSHA256: h, SnapshotSHA256: h, RollbackSnapshotSHA256: h, HostID: "host", ProjectID: "project", PrincipalID: "person", ApproverID: "human", ApproverPrincipalID: "person", KeyTag: "tag", CandidateSHA256: h, BootID: "boot", PreparedUTCMicros: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC).UnixMicro(), PreparedContinuousNanos: 1, IssuedAt: "2026-10-24T12:00:00Z", ApprovalExpiresAt: "2026-10-24T12:00:20Z", LeaseExpiresAt: "2026-10-24T12:00:30Z", DeadlineContinuousNanos: 30000000001, DurationSeconds: 30, Reason: "Measured diagnostic window", Controls: []maintenancePlanControl{{ID: "a", OldSamplePPM: 150, SamplePPM: 200}}}
}
func maintenanceTestContext() maintenanceContext {
	p := maintenanceTestPlan()
	a := maintenanceTestApprover()
	return maintenanceContext{Controller: maintenanceController{Epoch: 1, ExpiresAt: "2026-10-25T00:00:00Z", Approvers: []maintenanceApprover{a}}, ControllerEpoch: 1, ControllerSHA256: p.ControllerSHA256, RegistrationEpoch: 1, RegistrationSHA256: p.RegistrationSHA256, SnapshotSHA256: p.SnapshotSHA256, HostID: p.HostID, ProjectID: p.ProjectID, PrincipalID: p.PrincipalID, ConfigExpiresAt: "2026-10-25T00:00:00Z", EffectiveControls: maintenanceTestEffective(), Stamp: maintenanceHostStamp{Version: "harnessctl.host-stamp/1", OS: "darwin", OSUserID: "501", BootID: "boot", UTCMicros: 1, ContinuousNanos: 1}}
}
func TestMaintenanceCandidateExactScope(t *testing.T) {
	a := maintenanceTestApprover()
	e := maintenanceTestEffective()
	for _, tc := range []struct {
		name   string
		change func(*maintenanceCandidate)
		want   bool
	}{{"valid", func(*maintenanceCandidate) {}, true}, {"mandatory", func(c *maintenanceCandidate) { c.Controls[0].ID = "b" }, false}, {"below floor", func(c *maintenanceCandidate) { c.Controls[0].SamplePPM = 99 }, false}, {"outside roster", func(c *maintenanceCandidate) { c.Controls[0].ID = "x" }, false}, {"duplicate", func(c *maintenanceCandidate) { c.Controls = append(c.Controls, c.Controls[0]) }, false}, {"rearm with rate", func(c *maintenanceCandidate) { c.Purpose = "rearm-baseline"; c.DurationSeconds = 0 }, false}, {"control text", func(c *maintenanceCandidate) { c.Reason = "bad\nreason" }, false}, {"bidi override", func(c *maintenanceCandidate) { c.Reason = "bad\u202ereason" }, false}, {"bidi isolate", func(c *maintenanceCandidate) { c.Reason = "bad\u2066reason" }, false}, {"line separator", func(c *maintenanceCandidate) { c.Reason = "bad\u2028reason" }, false}, {"paragraph separator", func(c *maintenanceCandidate) { c.Reason = "bad\u2029reason" }, false}, {"accented", func(c *maintenanceCandidate) { c.Reason = "Révision" }, true}} {
		t.Run(tc.name, func(t *testing.T) {
			c := maintenanceTestCandidate()
			tc.change(&c)
			if got := validateMaintenanceCandidate(c, a, e) == nil; got != tc.want {
				t.Fatalf("valid=%v want %v", got, tc.want)
			}
		})
	}
}
func TestMaintenanceRequiredZeroFields(t *testing.T) {
	v := maintenanceNativeApproval{Version: "harnessctl.native-approval/1", Status: "denied", Signature: "", PublicKey: ""}
	b, _ := json.Marshal(v)
	if _, e := decodeMaintenanceNativeApproval(b); e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"signature", "publicKey"} {
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		delete(m, field)
		broken, _ := json.Marshal(m)
		if _, e := decodeMaintenanceNativeApproval(broken); e == nil {
			t.Fatalf("missing %s accepted", field)
		}
	}
}
func TestMaintenanceCanonicalPlanDoesNotMutate(t *testing.T) {
	p := maintenanceTestPlan()
	p.Controls = []maintenancePlanControl{{ID: "z", OldSamplePPM: 100, SamplePPM: 200}, {ID: "a", OldSamplePPM: 150, SamplePPM: 200}}
	before := append([]maintenancePlanControl(nil), p.Controls...)
	b, e := canonicalMaintenancePlan(p)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(p.Controls, before) {
		t.Fatal("mutated caller")
	}
	var q maintenancePlan
	if e := strictJSON(b, &q); e != nil || q.Controls[0].ID != "a" {
		t.Fatal("not sorted exact wire")
	}
}
func TestMaintenancePlanBindings(t *testing.T) {
	ctx := maintenanceTestContext()
	for _, tc := range []struct {
		name   string
		change func(*maintenancePlan)
		want   bool
	}{{"valid", func(*maintenancePlan) {}, true}, {"controller", func(p *maintenancePlan) { p.ControllerSHA256 = hex.EncodeToString(bytes.Repeat([]byte{2}, 32)) }, false}, {"mandatory", func(p *maintenancePlan) { p.Controls[0].ID = "b" }, false}, {"old rate", func(p *maintenancePlan) { p.Controls[0].OldSamplePPM = 151 }, false}, {"extend lease", func(p *maintenancePlan) { p.LeaseExpiresAt = "2026-10-24T12:00:31Z" }, false}, {"revoke", func(p *maintenancePlan) { ctx.Controller.Approvers[0].Revoked = true }, false}} {
		t.Run(tc.name, func(t *testing.T) {
			p := maintenanceTestPlan()
			original := ctx.Controller.Approvers[0].Revoked
			tc.change(&p)
			got := validateMaintenancePlan(p, ctx) == nil
			ctx.Controller.Approvers[0].Revoked = original
			if got != tc.want {
				t.Fatalf("valid=%v want %v", got, tc.want)
			}
		})
	}
}
func TestMaintenanceDSSEP256(t *testing.T) {
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	a := maintenanceTestApprover()
	a.PublicKey = base64.StdEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), key.X, key.Y))
	p := maintenanceTestPlan()
	b, e := canonicalMaintenancePlan(p)
	if e != nil {
		t.Fatal(e)
	}
	pae := append([]byte("DSSEv1 "+strconv.Itoa(len(maintenancePayloadType))+" "+maintenancePayloadType+" "+strconv.Itoa(len(b))+" "), b...)
	h := sha256.Sum256(pae)
	sig, e := ecdsa.SignASN1(rand.Reader, key, h[:])
	if e != nil {
		t.Fatal(e)
	}
	approval := maintenanceNativeApproval{Version: "harnessctl.native-approval/1", Status: "signed", Signature: base64.StdEncoding.EncodeToString(sig), PublicKey: a.PublicKey}
	if e := verifyMaintenanceApproval(b, approval, a); e != nil {
		t.Fatal(e)
	}
	b[len(b)-2] ^= 1
	if e := verifyMaintenanceApproval(b, approval, a); e == nil {
		t.Fatal("modified bytes accepted")
	}
}
func TestMaintenanceStateEpoch(t *testing.T) {
	ctx := maintenanceTestContext()
	p := maintenanceTestPlan()
	c := maintenanceTestCandidate()
	raw, _ := json.Marshal(c)
	h := sha256.Sum256(raw)
	p.CandidateSHA256 = hex.EncodeToString(h[:])
	payload := base64.StdEncoding.EncodeToString(raw)
	origin := "armed"
	stamp := ctx.Stamp
	s := maintenanceState{Version: "harnessctl.maintenance-state/1", InstanceID: p.InstanceID, Sequence: 3, AuthorizationEpoch: p.AuthorizationEpoch, Phase: "approval-pending", ControllerEpoch: 1, ControllerSHA256: p.ControllerSHA256, RegistrationEpoch: 1, RegistrationSHA256: p.RegistrationSHA256, SnapshotSHA256: p.SnapshotSHA256, HostID: p.HostID, ProjectID: p.ProjectID, PrincipalID: p.PrincipalID, Stamp: stamp, ClockAnchor: stamp, Plan: &p, PendingOrigin: &origin, CandidatePayload: &payload}
	if e := validateMaintenanceState(s, ctx); e != nil {
		t.Fatal(e)
	}
	s.AuthorizationEpoch++
	if e := validateMaintenanceState(s, ctx); e == nil {
		t.Fatal("changed pending epoch accepted")
	}
}

func TestMaintenanceMissingRequiredWireFields(t *testing.T) {
	p := maintenanceTestPlan()
	p.Purpose = "rearm-baseline"
	p.DurationSeconds = 0
	p.DeadlineContinuousNanos = 0
	p.LeaseExpiresAt = ""
	p.Controls = []maintenancePlanControl{}
	stamp := maintenanceHostStamp{Version: "harnessctl.host-stamp/1", OS: "darwin", OSUserID: "501", BootID: "boot", UTCMicros: 1, ContinuousNanos: 0}
	c := maintenanceController{Version: "harnessctl.controller/1", Epoch: 1, IssuedAt: "2026-10-24T00:00:00Z", ExpiresAt: "2026-10-25T00:00:00Z", HostID: "host", Native: maintenanceNative{Profile: "darwin-protected-key/1", Executable: "/native", SHA256: p.ControllerSHA256, PreparationRoot: "/private", TimeoutMillis: 1}, Approvers: []maintenanceApprover{maintenanceTestApprover()}, Projects: []maintenanceProject{{ID: "project", Root: "/project", PrincipalID: "person", Registration: "/registration", StateRoot: "/state"}}}
	lease := maintenanceLease{Plan: maintenanceTestPlan(), ApprovedAt: "2026-10-24T12:00:00Z", ExpiresAt: "2026-10-24T12:00:30Z", DeadlineContinuousNanos: 30000000001, ProofSHA256: p.ControllerSHA256, ApprovalPayload: ""}
	report := maintenanceReport{Version: "harnessctl.maintenance-report/1", Status: "hold", Operation: "unknown", Code: "input_invalid", NoHostActionAuthority: true, Installation: "unobserved", Coverage: "selected-project-only"}
	state := maintenanceState{Version: "harnessctl.maintenance-state/1", InstanceID: p.InstanceID, Sequence: 1, AuthorizationEpoch: 1, Phase: "armed", ControllerEpoch: 1, ControllerSHA256: p.ControllerSHA256, RegistrationEpoch: 1, RegistrationSHA256: p.RegistrationSHA256, SnapshotSHA256: p.SnapshotSHA256, HostID: "host", ProjectID: "project", PrincipalID: "person", Stamp: stamp, ClockAnchor: stamp}
	for _, tc := range []struct {
		name string
		v    any
	}{{"controller", c}, {"native", c.Native}, {"approver", c.Approvers[0]}, {"project", c.Projects[0]}, {"candidate", maintenanceTestCandidate()}, {"candidate-control", maintenanceCandidateControl{ID: "a", SamplePPM: 0}}, {"plan", p}, {"plan-control", maintenancePlanControl{ID: "a", OldSamplePPM: 0, SamplePPM: 0}}, {"host-stamp", stamp}, {"native-approval", maintenanceNativeApproval{Version: "harnessctl.native-approval/1", Status: "denied"}}, {"lease", lease}, {"state", state}, {"report", report}} {
		t.Run(tc.name, func(t *testing.T) {
			raw, e := json.Marshal(tc.v)
			if e != nil {
				t.Fatal(e)
			}
			ptr := reflect.New(reflect.TypeOf(tc.v))
			if e := strictJSON(raw, ptr.Interface()); e != nil {
				t.Fatalf("valid shape rejected: %v", e)
			}
			var m map[string]any
			if e := json.Unmarshal(raw, &m); e != nil {
				t.Fatal(e)
			}
			typ := reflect.TypeOf(tc.v)
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				tag := f.Tag.Get("json")
				name := tag
				optional := false
				if at := strings.IndexByte(tag, ','); at >= 0 {
					name = tag[:at]
					optional = strings.Contains(tag[at+1:], "omitempty")
				}
				if optional {
					continue
				}
				v := m[name]
				delete(m, name)
				broken, _ := json.Marshal(m)
				target := reflect.New(typ)
				if e := strictJSON(broken, target.Interface()); e == nil {
					t.Errorf("missing %s accepted", name)
				}
				m[name] = v
			}
		})
	}
}
func TestMaintenancePlanExactDeadline(t *testing.T) {
	ctx := maintenanceTestContext()
	for _, tc := range []struct {
		name   string
		change func(*maintenancePlan)
		want   bool
	}{{"valid", func(*maintenancePlan) {}, true}, {"deadline longer", func(p *maintenancePlan) { p.DeadlineContinuousNanos++ }, false}, {"deadline shorter", func(p *maintenancePlan) { p.DeadlineContinuousNanos-- }, false}, {"prepared wall", func(p *maintenancePlan) { p.PreparedUTCMicros++ }, false}, {"issued floor", func(p *maintenancePlan) { p.IssuedAt = "2026-10-24T11:59:59Z" }, false}, {"controller cap", func(p *maintenancePlan) { ctx.Controller.ExpiresAt = "2026-10-24T12:00:25Z" }, false}} {
		t.Run(tc.name, func(t *testing.T) {
			p := maintenanceTestPlan()
			old := ctx.Controller.ExpiresAt
			tc.change(&p)
			got := validateMaintenancePlan(p, ctx) == nil
			ctx.Controller.ExpiresAt = old
			if got != tc.want {
				t.Fatalf("valid=%v want %v", got, tc.want)
			}
		})
	}
}

func TestMaintenanceCUEGoParity(t *testing.T) {
	cue, e := exec.LookPath("cue")
	if e != nil {
		t.Skip("cue unavailable")
	}
	vet := func(name, def string, record any, want bool, goValid func([]byte) bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			raw, e := json.Marshal(record)
			if e != nil {
				t.Fatal(e)
			}
			if got := goValid(raw); got != want {
				t.Fatalf("Go valid=%v want %v", got, want)
			}
			path := filepath.Join(t.TempDir(), "wire.json")
			if e := os.WriteFile(path, raw, 0600); e != nil {
				t.Fatal(e)
			}
			output, e := exec.Command(cue, "vet", "-c", "schema.cue", "maintenance_schema.cue", "maintenance_store_schema.cue", path, "-d", def).CombinedOutput()
			if got := e == nil; got != want {
				t.Fatalf("CUE valid=%v want %v: %s", got, want, output)
			}
		})
	}
	candidateGo := func(raw []byte) bool {
		c, e := decodeMaintenanceCandidate(raw)
		return e == nil && validateMaintenanceCandidate(c, maintenanceTestApprover(), maintenanceTestEffective()) == nil
	}
	c := maintenanceTestCandidate()
	c.Reason = strings.Repeat("é", 256)
	vet("candidate 512 bytes", "#MaintenanceCandidate", c, true, candidateGo)
	c.Reason = strings.Repeat("é", 257)
	vet("candidate 514 bytes", "#MaintenanceCandidate", c, false, candidateGo)
	for _, sep := range []string{"\u2028", "\u2029"} {
		c := maintenanceTestCandidate()
		c.Reason = "A" + sep + "B"
		vet("candidate separator "+strconv.Quote(sep), "#MaintenanceCandidate", c, false, candidateGo)
	}
	planGo := func(raw []byte) bool {
		var p maintenancePlan
		return strictJSON(raw, &p) == nil && validateMaintenancePlanShape(p) == nil
	}
	p := maintenanceTestPlan()
	p.Reason = strings.Repeat("é", 256)
	vet("plan 512 bytes", "#MaintenancePlan", p, true, planGo)
	p.Reason = strings.Repeat("é", 257)
	vet("plan 514 bytes", "#MaintenancePlan", p, false, planGo)
	planRaw, _ := json.Marshal(maintenanceTestPlan())
	var pm map[string]any
	_ = json.Unmarshal(planRaw, &pm)
	pm["authorizationEpoch"] = json.Number("18446744073709551616")
	vet("plan uint64 overflow", "#MaintenancePlan", pm, false, planGo)
	ctx := maintenanceTestContext()
	state := maintenanceState{Version: "harnessctl.maintenance-state/1", InstanceID: maintenanceTestPlan().InstanceID, Sequence: 1, AuthorizationEpoch: 1, Phase: "armed", ControllerEpoch: 1, ControllerSHA256: ctx.ControllerSHA256, RegistrationEpoch: 1, RegistrationSHA256: ctx.RegistrationSHA256, SnapshotSHA256: ctx.SnapshotSHA256, HostID: ctx.HostID, ProjectID: ctx.ProjectID, PrincipalID: ctx.PrincipalID, Stamp: ctx.Stamp, ClockAnchor: ctx.Stamp}
	stateGo := func(raw []byte) bool {
		var s maintenanceState
		return strictJSON(raw, &s) == nil && validateMaintenanceState(s, ctx) == nil
	}
	vet("armed state", "#MaintenanceState", state, true, stateGo)
	stateRaw, _ := json.Marshal(state)
	stateMap := func() map[string]any { var m map[string]any; _ = json.Unmarshal(stateRaw, &m); return m }
	sm := stateMap()
	sm["sequence"] = json.Number("18446744073709551616")
	vet("state uint64 overflow", "#MaintenanceState", sm, false, stateGo)
	sm = stateMap()
	stamp := sm["stamp"].(map[string]any)
	stamp["utcMicros"] = json.Number("9223372036854775808")
	vet("stamp int64 overflow", "#MaintenanceState", sm, false, stateGo)
	sm = stateMap()
	sm["lease"] = map[string]any{"plan": maintenanceTestPlan(), "approvedAt": "2026-01-01T00:00:00Z", "expiresAt": "2026-01-01T00:00:30Z", "deadlineContinuousNanos": 1, "proofSHA256": ctx.ControllerSHA256, "approvalPayload": ""}
	vet("armed with lease", "#MaintenanceState", sm, false, stateGo)
	sm = stateMap()
	sm["phase"] = "maintenance"
	sm["candidatePayload"] = "e30="
	vet("maintenance without lease", "#MaintenanceState", sm, false, stateGo)
	sm = stateMap()
	sm["phase"] = "prepared"
	sm["plan"] = maintenanceTestPlan()
	sm["pendingOrigin"] = "armed"
	vet("prepared without candidate", "#MaintenanceState", sm, false, stateGo)
	r := maintenanceBaseReport("check")
	maintenanceSuccess(&r, "armed")
	reportGo := func(raw []byte) bool {
		var x maintenanceReport
		return strictJSON(raw, &x) == nil && validateMaintenanceReport(x) == nil
	}
	vet("complete report", "#MaintenanceReport", r, true, reportGo)
	reportRaw, _ := json.Marshal(r)
	reportMap := func() map[string]any { var m map[string]any; _ = json.Unmarshal(reportRaw, &m); return m }
	rm := reportMap()
	rm["code"] = "signature_invalid"
	delete(rm, "phase")
	vet("complete failure code", "#MaintenanceReport", rm, false, reportGo)
	rm = reportMap()
	rm["nativeSignatureVerified"] = true
	vet("signature without completion", "#MaintenanceReport", rm, false, reportGo)
	rm = reportMap()
	rm["status"] = "hold"
	rm["code"] = "signature_invalid"
	vet("hold with phase", "#MaintenanceReport", rm, false, reportGo)
	rm = reportMap()
	rm["nativeApprovalCompleted"] = 1
	vet("completed without invoked", "#MaintenanceReport", rm, false, reportGo)
	rm = reportMap()
	rm["stateWritesCommitted"] = 1
	vet("committed without attempted", "#MaintenanceReport", rm, false, reportGo)
	rm = reportMap()
	rm["stateWritesAttempted"] = json.Number("9223372036854775808")
	vet("report int64 overflow", "#MaintenanceReport", rm, false, reportGo)
	pointerGo := func(raw []byte) bool {
		var x maintenanceStatePointer
		return strictJSON(raw, &x) == nil && maintenancePointerValid(x)
	}
	ptr := maintenanceStatePointer{Version: "harnessctl.state-pointer/1", Slot: "a", Sequence: 1, SHA256: ctx.ControllerSHA256}
	vet("pointer", "#MaintenanceStatePointer", ptr, true, pointerGo)
	ptrRaw, _ := json.Marshal(ptr)
	var m map[string]any
	_ = json.Unmarshal(ptrRaw, &m)
	m["sequence"] = json.Number("18446744073709551616")
	vet("pointer uint64 overflow", "#MaintenanceStatePointer", m, false, pointerGo)
}
