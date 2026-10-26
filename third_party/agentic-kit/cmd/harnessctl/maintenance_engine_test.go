//go:build harness_maintenance

package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
)

func engineContext(t *testing.T) (maintenanceContext, *ecdsa.PrivateKey) {
	t.Helper()
	ctx := maintenanceTestContext()
	ctx.Controller.Version = "harnessctl.controller/1"
	ctx.Controller.IssuedAt = "2026-10-24T00:00:00Z"
	ctx.Stamp.UTCMicros = 1792843200000000
	ctx.Stamp.ContinuousNanos = 100000000000
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	ctx.Controller.Approvers[0].PublicKey = base64.StdEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), key.X, key.Y))
	return ctx, key
}
func engineNext(t *testing.T, r maintenanceTransitionResult) *maintenanceState {
	t.Helper()
	if r.Exit != 0 || !r.Write || r.State == nil {
		t.Fatalf("result code=%s exit=%d write=%t", r.Code, r.Exit, r.Write)
	}
	return r.State
}
func engineInit(t *testing.T, ctx maintenanceContext) *maintenanceState {
	t.Helper()
	return engineNext(t, maintenanceTransition(nil, maintenanceTransitionInput{Operation: "init", InstanceEntropy: make([]byte, 32)}, ctx))
}
func enginePrepared(t *testing.T, ctx maintenanceContext) *maintenanceState {
	t.Helper()
	s := engineInit(t, ctx)
	raw, _ := json.Marshal(maintenanceTestCandidate())
	return engineNext(t, maintenanceTransition(s, maintenanceTransitionInput{Operation: "prepare", CandidateRaw: raw, Nonce: make([]byte, 32)}, ctx))
}
func engineSignedApproval(t *testing.T, p maintenancePlan, key *ecdsa.PrivateKey, a maintenanceApprover) []byte {
	t.Helper()
	plan, e := canonicalMaintenancePlan(p)
	if e != nil {
		t.Fatal(e)
	}
	pae := append([]byte("DSSEv1 "+strconv.Itoa(len(maintenancePayloadType))+" "+maintenancePayloadType+" "+strconv.Itoa(len(plan))+" "), plan...)
	h := sha256.Sum256(pae)
	sig, e := ecdsa.SignASN1(rand.Reader, key, h[:])
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(maintenanceNativeApproval{Version: "harnessctl.native-approval/1", Status: "signed", Signature: base64.StdEncoding.EncodeToString(sig), PublicKey: a.PublicKey})
	return raw
}
func TestMaintenanceEngineLifecycle(t *testing.T) {
	ctx, key := engineContext(t)
	init := engineInit(t, ctx)
	if init.Phase != "armed" || init.Sequence != 1 || init.AuthorizationEpoch != 1 {
		t.Fatal(init)
	}
	prepared := enginePrepared(t, ctx)
	if prepared.Phase != "prepared" || prepared.Sequence != 2 || prepared.AuthorizationEpoch != 2 || prepared.Plan.AuthorizationEpoch != 2 || prepared.PendingOrigin == nil || *prepared.PendingOrigin != "armed" {
		t.Fatal(prepared)
	}
	pending := engineNext(t, maintenanceTransition(prepared, maintenanceTransitionInput{Operation: "begin-approval"}, ctx))
	if pending.Phase != "approval-pending" || pending.Sequence != 3 || pending.AuthorizationEpoch != 2 {
		t.Fatal(pending)
	}
	approval := engineSignedApproval(t, *pending.Plan, key, ctx.Controller.Approvers[0])
	live := engineNext(t, maintenanceTransition(pending, maintenanceTransitionInput{Operation: "finish-approval", ApprovalRaw: approval}, ctx))
	if live.Phase != "maintenance" || live.Sequence != 4 || live.AuthorizationEpoch != 3 || live.Lease == nil || live.Lease.Plan.AuthorizationEpoch+1 != live.AuthorizationEpoch {
		t.Fatal(live)
	}
	ctx.Stamp.UTCMicros += 1e6
	ctx.Stamp.ContinuousNanos += 1e9
	checked := maintenanceTransition(live, maintenanceTransitionInput{Operation: "check"}, ctx)
	if checked.Exit != 0 || !checked.Write || checked.State.AuthorizationEpoch != 3 || len(checked.Controls) != 2 || checked.Controls[0].SamplePPM != 200 || checked.Controls[1].SamplePPM != 1000000 {
		t.Fatal(checked)
	}
	checked.Controls[0].SamplePPM = 0
	if ctx.EffectiveControls[0].SamplePPM != 150 {
		t.Fatal("caller control alias")
	}
	endResult := maintenanceTransition(checked.State, maintenanceTransitionInput{Operation: "end"}, ctx)
	ended := engineNext(t, endResult)
	if ended.Phase != "armed" || ended.AuthorizationEpoch != 4 || ended.Lease != nil || endResult.Controls[0].SamplePPM != 150 {
		t.Fatal(ended)
	}
	if pending.Phase != "approval-pending" || pending.Plan == nil {
		t.Fatal("mutated caller state")
	}
}
func TestMaintenanceEngineDenialAndReplay(t *testing.T) {
	ctx, _ := engineContext(t)
	prepared := enginePrepared(t, ctx)
	pending := engineNext(t, maintenanceTransition(prepared, maintenanceTransitionInput{Operation: "begin-approval"}, ctx))
	if r := maintenanceTransition(pending, maintenanceTransitionInput{Operation: "begin-approval"}, ctx); r.Write || r.Exit == 0 {
		t.Fatal("second begin accepted")
	}
	raw, _ := json.Marshal(maintenanceNativeApproval{Version: "harnessctl.native-approval/1", Status: "denied", Signature: "", PublicKey: ""})
	r := maintenanceTransition(pending, maintenanceTransitionInput{Operation: "finish-approval", ApprovalRaw: raw}, ctx)
	if r.Code != "native_denied" || r.Exit != 1 || !r.Write || r.State.Phase != "armed" || r.State.AuthorizationEpoch != 3 || r.State.Plan != nil {
		t.Fatal(r)
	}
	if again := maintenanceTransition(r.State, maintenanceTransitionInput{Operation: "finish-approval", ApprovalRaw: raw}, ctx); again.Write || again.Exit == 0 {
		t.Fatal("replay accepted")
	}
}
func TestMaintenanceEngineRearm(t *testing.T) {
	ctx, key := engineContext(t)
	armed := engineInit(t, ctx)
	armed.Phase = "locked"
	ctx.Stamp.BootID = "newboot"
	ctx.Stamp.ContinuousNanos = 5
	candidate := maintenanceTestCandidate()
	candidate.Purpose = "rearm-baseline"
	candidate.DurationSeconds = 0
	candidate.Controls = []maintenanceCandidateControl{}
	raw, _ := json.Marshal(candidate)
	prepared := engineNext(t, maintenanceTransition(armed, maintenanceTransitionInput{Operation: "prepare", CandidateRaw: raw, Nonce: make([]byte, 32)}, ctx))
	if prepared.Phase != "prepared" || prepared.ClockAnchor.BootID != "newboot" {
		t.Fatal(prepared)
	}
	wire, e := json.Marshal(prepared)
	if e != nil || strictJSON(wire, new(maintenanceState)) != nil {
		t.Fatal("rearm state fails strict round-trip")
	}
	if r := maintenanceTransition(prepared, maintenanceTransitionInput{Operation: "check"}, ctx); r.Exit == 0 || len(r.Controls) != 0 {
		t.Fatal("locked-origin controls exposed")
	}
	pending := engineNext(t, maintenanceTransition(prepared, maintenanceTransitionInput{Operation: "begin-approval"}, ctx))
	approval := engineSignedApproval(t, *pending.Plan, key, ctx.Controller.Approvers[0])

	rearmed := engineNext(t, maintenanceTransition(pending, maintenanceTransitionInput{Operation: "finish-approval", ApprovalRaw: approval}, ctx))
	if rearmed.Phase != "armed" || rearmed.Lease != nil || rearmed.Plan != nil || rearmed.AuthorizationEpoch != 3 {
		t.Fatal(rearmed)
	}
}
func TestMaintenanceEngineExpiryAndClock(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*maintenanceContext)
		code   string
		phase  string
	}{{"expiry", func(c *maintenanceContext) { c.Stamp.UTCMicros += 31e6; c.Stamp.ContinuousNanos += 31e9 }, "ok", "armed"}, {"reboot", func(c *maintenanceContext) { c.Stamp.BootID = "other" }, "clock_invalid", "locked"}, {"rollback", func(c *maintenanceContext) { c.Stamp.UTCMicros-- }, "clock_invalid", "locked"}, {"cumulative skew", func(c *maintenanceContext) { c.Stamp.UTCMicros += 2e6 }, "clock_invalid", "locked"}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, key := engineContext(t)
			prepared := enginePrepared(t, ctx)
			pending := engineNext(t, maintenanceTransition(prepared, maintenanceTransitionInput{Operation: "begin-approval"}, ctx))
			approval := engineSignedApproval(t, *pending.Plan, key, ctx.Controller.Approvers[0])
			live := engineNext(t, maintenanceTransition(pending, maintenanceTransitionInput{Operation: "finish-approval", ApprovalRaw: approval}, ctx))
			tc.change(&ctx)
			r := maintenanceTransition(live, maintenanceTransitionInput{Operation: "check"}, ctx)
			if !r.Write || r.Code != tc.code || r.State.Phase != tc.phase {
				t.Fatal(r)
			}
			if tc.phase == "locked" {
				if len(r.Controls) != 0 {
					t.Fatal("locked controls exposed")
				}
				again := maintenanceTransition(r.State, maintenanceTransitionInput{Operation: "check"}, ctx)
				if again.Write || again.Exit == 0 {
					t.Fatal("repeated lock write")
				}
			}
		})
	}
}
func TestMaintenanceEngineInitRecoverBoundaries(t *testing.T) {
	ctx, _ := engineContext(t)
	s := engineInit(t, ctx)
	if r := maintenanceTransition(s, maintenanceTransitionInput{Operation: "init", InstanceEntropy: make([]byte, 32)}, ctx); r.Write || r.Code != "state_exists" {
		t.Fatal(r)
	}
	if r := maintenanceTransition(nil, maintenanceTransitionInput{Operation: "recover", InstanceEntropy: []byte{1}}, ctx); r.Write || r.Code != "entropy_unavailable" {
		t.Fatal(r)
	}
	fresh := make([]byte, 32)
	fresh[0] = 1
	recovered := engineNext(t, maintenanceTransition(s, maintenanceTransitionInput{Operation: "recover", InstanceEntropy: fresh}, ctx))
	if recovered.Phase != "locked" || recovered.InstanceID == s.InstanceID || recovered.Sequence != 2 || recovered.AuthorizationEpoch != 2 {
		t.Fatal(recovered)
	}
}

func TestMaintenanceEngineStoredProofAndApprovalWindow(t *testing.T) {
	ctx, key := engineContext(t)
	prepared := enginePrepared(t, ctx)
	pending := engineNext(t, maintenanceTransition(prepared, maintenanceTransitionInput{Operation: "begin-approval"}, ctx))
	approval := engineSignedApproval(t, *pending.Plan, key, ctx.Controller.Approvers[0])
	live := engineNext(t, maintenanceTransition(pending, maintenanceTransitionInput{Operation: "finish-approval", ApprovalRaw: approval}, ctx))
	ctx.Stamp.UTCMicros += 21e6
	ctx.Stamp.ContinuousNanos += 21e9
	if r := maintenanceTransition(live, maintenanceTransitionInput{Operation: "check"}, ctx); r.Exit != 0 || r.State.Phase != "maintenance" || len(r.Controls) == 0 {
		t.Fatalf("active lease died after approval window: %+v", r)
	}
	for _, tc := range []struct {
		name   string
		change func(*maintenanceState, *maintenanceContext)
	}{{"proof hash", func(s *maintenanceState, _ *maintenanceContext) { s.Lease.ProofSHA256 = s.Lease.Plan.Nonce }}, {"proof bytes", func(s *maintenanceState, _ *maintenanceContext) {
		s.Lease.ApprovalPayload = base64.StdEncoding.EncodeToString([]byte("{}"))
	}}, {"late approval", func(s *maintenanceState, _ *maintenanceContext) { s.Lease.ApprovedAt = s.Lease.Plan.ApprovalExpiresAt }}, {"revoked", func(_ *maintenanceState, c *maintenanceContext) { c.Controller.Approvers[0].Revoked = true }}, {"controller expired", func(_ *maintenanceState, c *maintenanceContext) { c.Controller.ExpiresAt = "2026-10-24T12:00:20Z" }}} {
		t.Run(tc.name, func(t *testing.T) {
			s := maintenanceClone(live)
			c := ctx
			c.Controller.Approvers = append([]maintenanceApprover(nil), ctx.Controller.Approvers...)
			tc.change(s, &c)
			r := maintenanceTransition(s, maintenanceTransitionInput{Operation: "check"}, c)
			if r.Exit == 0 || len(r.Controls) != 0 || r.Write {
				t.Fatalf("corrupt proof yielded authority: %+v", r)
			}
		})
	}
}

func TestMaintenanceEngineRecoverRejectsForeignBinding(t *testing.T) {
	ctx, _ := engineContext(t)
	state := engineInit(t, ctx)
	foreign := ctx
	foreign.ControllerSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if r := maintenanceTransition(state, maintenanceTransitionInput{Operation: "recover", InstanceEntropy: bytes.Repeat([]byte{9}, 32)}, foreign); r.Write || r.Exit == 0 || r.Code != "binding_mismatch" {
		t.Fatal(r)
	}
}
