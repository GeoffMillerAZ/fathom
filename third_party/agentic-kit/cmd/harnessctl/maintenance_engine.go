//go:build harness_maintenance

package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"time"
)

type maintenanceTransitionInput struct {
	Operation       string
	CandidateRaw    []byte
	Nonce           []byte
	InstanceEntropy []byte
	ApprovalRaw     []byte
}
type maintenanceTransitionResult struct {
	State    *maintenanceState
	Write    bool
	Controls []effectiveControl
	Code     string
	Exit     int
}

func maintenanceHold(code string, exit int) maintenanceTransitionResult {
	return maintenanceTransitionResult{Code: code, Exit: exit}
}
func maintenanceDone(s *maintenanceState, write bool, controls []effectiveControl) maintenanceTransitionResult {
	return maintenanceTransitionResult{State: s, Write: write, Controls: controls, Code: "ok", Exit: 0}
}
func maintenanceClone(s *maintenanceState) *maintenanceState {
	if s == nil {
		return nil
	}
	n := *s
	if s.Plan != nil {
		p := *s.Plan
		p.Controls = append([]maintenancePlanControl{}, p.Controls...)
		n.Plan = &p
	}
	if s.PendingOrigin != nil {
		x := *s.PendingOrigin
		n.PendingOrigin = &x
	}
	if s.CandidatePayload != nil {
		x := *s.CandidatePayload
		n.CandidatePayload = &x
	}
	if s.Lease != nil {
		l := *s.Lease
		l.Plan.Controls = append([]maintenancePlanControl{}, l.Plan.Controls...)
		n.Lease = &l
	}
	return &n
}
func maintenanceControlsCopy(src []effectiveControl) []effectiveControl {
	out := make([]effectiveControl, len(src))
	copy(out, src)
	for i := range out {
		out[i].TunableBy = append([]string(nil), src[i].TunableBy...)
		out[i].ConstrainedBy = append([]string(nil), src[i].ConstrainedBy...)
	}
	return out
}
func maintenanceClearProposal(s *maintenanceState) {
	s.Plan = nil
	s.PendingOrigin = nil
	s.CandidatePayload = nil
	s.Lease = nil
}
func maintenanceStampTime(s maintenanceHostStamp) time.Time { return time.UnixMicro(s.UTCMicros).UTC() }
func maintenanceTimeText(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z")
}
func maintenanceMinTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
func maintenanceEpochWrite(s *maintenanceState) bool {
	if s.Sequence == math.MaxUint64 || s.AuthorizationEpoch == math.MaxUint64 {
		return false
	}
	s.Sequence++
	s.AuthorizationEpoch++
	return true
}
func maintenanceClockWrite(s *maintenanceState, stamp maintenanceHostStamp) bool {
	if s.Stamp == stamp {
		return true
	}
	if s.Sequence == math.MaxUint64 {
		return false
	}
	s.Sequence++
	s.Stamp = stamp
	return true
}
func maintenanceClockValid(s *maintenanceState, stamp maintenanceHostStamp) bool {
	if validateMaintenanceHostStamp(stamp) != nil || stamp.BootID != s.Stamp.BootID || stamp.BootID != s.ClockAnchor.BootID || stamp.OSUserID != s.Stamp.OSUserID || stamp.OSUserID != s.ClockAnchor.OSUserID || stamp.UTCMicros < s.Stamp.UTCMicros || stamp.ContinuousNanos < s.Stamp.ContinuousNanos || stamp.UTCMicros < s.ClockAnchor.UTCMicros || stamp.ContinuousNanos < s.ClockAnchor.ContinuousNanos {
		return false
	}
	wallMicros := uint64(stamp.UTCMicros - s.ClockAnchor.UTCMicros)
	if wallMicros > math.MaxUint64/1000 {
		return false
	}
	wallElapsed := wallMicros * 1000
	continuousElapsed := uint64(stamp.ContinuousNanos - s.ClockAnchor.ContinuousNanos)
	if wallElapsed > continuousElapsed {
		return wallElapsed-continuousElapsed <= 1e9
	}
	return continuousElapsed-wallElapsed <= 1e9
}
func maintenanceLock(s *maintenanceState, stamp maintenanceHostStamp, code string) maintenanceTransitionResult {
	n := maintenanceClone(s)
	if !maintenanceEpochWrite(n) {
		return maintenanceHold("state_invalid", 2)
	}
	n.Phase = "locked"
	maintenanceClearProposal(n)
	if validateMaintenanceHostStamp(stamp) == nil {
		n.Stamp = stamp
	}
	return maintenanceTransitionResult{State: n, Write: true, Code: code, Exit: 2}
}
func maintenanceOrigin(s *maintenanceState, stamp maintenanceHostStamp, code string, exit int) maintenanceTransitionResult {
	n := maintenanceClone(s)
	if n.PendingOrigin == nil || !maintenanceEpochWrite(n) {
		return maintenanceHold("state_invalid", 2)
	}
	n.Phase = *n.PendingOrigin
	maintenanceClearProposal(n)
	n.Stamp = stamp
	return maintenanceTransitionResult{State: n, Write: true, Code: code, Exit: exit}
}
func maintenanceFreshContext(ctx maintenanceContext) bool {
	controllerExpiry, e := parseTime(ctx.Controller.ExpiresAt)
	if e != nil {
		return false
	}
	configExpiry, e := parseTime(ctx.ConfigExpiresAt)
	if e != nil {
		return false
	}
	now := maintenanceStampTime(ctx.Stamp)
	if !now.Before(controllerExpiry) || !now.Before(configExpiry) {
		return false
	}
	return validateMaintenanceHostStamp(ctx.Stamp) == nil && ctx.Controller.Epoch == ctx.ControllerEpoch && maintenanceHash(ctx.ControllerSHA256) && ctx.RegistrationEpoch >= 1 && maintenanceHash(ctx.RegistrationSHA256) && maintenanceHash(ctx.SnapshotSHA256) && validID(ctx.HostID) && validID(ctx.ProjectID) && validID(ctx.PrincipalID)
}
func maintenanceNewState(ctx maintenanceContext, instance []byte, phase string) (*maintenanceState, bool) {
	if !maintenanceFreshContext(ctx) || len(instance) != 32 {
		return nil, false
	}
	h := hex.EncodeToString(instance)
	s := &maintenanceState{Version: "harnessctl.maintenance-state/1", InstanceID: h, Sequence: 1, AuthorizationEpoch: 1, Phase: phase, ControllerEpoch: ctx.ControllerEpoch, ControllerSHA256: ctx.ControllerSHA256, RegistrationEpoch: ctx.RegistrationEpoch, RegistrationSHA256: ctx.RegistrationSHA256, SnapshotSHA256: ctx.SnapshotSHA256, HostID: ctx.HostID, ProjectID: ctx.ProjectID, PrincipalID: ctx.PrincipalID, Stamp: ctx.Stamp, ClockAnchor: ctx.Stamp}
	return s, true
}
func maintenanceTransition(state *maintenanceState, in maintenanceTransitionInput, ctx maintenanceContext) maintenanceTransitionResult {
	if in.Operation == "init" {
		if state != nil {
			return maintenanceHold("state_exists", 2)
		}
		s, ok := maintenanceNewState(ctx, in.InstanceEntropy, "armed")
		if !ok {
			if len(in.InstanceEntropy) != 32 {
				return maintenanceHold("entropy_unavailable", 2)
			}
			return maintenanceHold("config_incomplete", 2)
		}
		return maintenanceDone(s, true, nil)
	}
	if in.Operation == "recover" {
		if state != nil && (state.ControllerEpoch != ctx.ControllerEpoch || state.ControllerSHA256 != ctx.ControllerSHA256 || state.RegistrationEpoch != ctx.RegistrationEpoch || state.RegistrationSHA256 != ctx.RegistrationSHA256 || state.SnapshotSHA256 != ctx.SnapshotSHA256 || state.HostID != ctx.HostID || state.ProjectID != ctx.ProjectID || state.PrincipalID != ctx.PrincipalID) {
			return maintenanceHold("binding_mismatch", 2)
		}
		s, ok := maintenanceNewState(ctx, in.InstanceEntropy, "locked")
		if !ok {
			if len(in.InstanceEntropy) != 32 {
				return maintenanceHold("entropy_unavailable", 2)
			}
			return maintenanceHold("config_incomplete", 2)
		}
		if state != nil {
			if state.InstanceID == s.InstanceID || state.Sequence == math.MaxUint64 || state.AuthorizationEpoch == math.MaxUint64 {
				return maintenanceHold("state_invalid", 2)
			}
			s.Sequence = state.Sequence + 1
			s.AuthorizationEpoch = state.AuthorizationEpoch + 1
		}
		return maintenanceDone(s, true, nil)
	}
	if state == nil {
		return maintenanceHold("state_unavailable", 2)
	}
	if !maintenanceFreshContext(ctx) || state.Version != "harnessctl.maintenance-state/1" || !maintenanceHex(state.InstanceID) || state.Sequence < 1 || state.AuthorizationEpoch < 1 || validateMaintenanceHostStamp(state.Stamp) != nil || validateMaintenanceHostStamp(state.ClockAnchor) != nil || !oneOf(state.Phase, "armed", "prepared", "approval-pending", "maintenance", "locked") || state.ControllerEpoch != ctx.ControllerEpoch || state.ControllerSHA256 != ctx.ControllerSHA256 || state.RegistrationEpoch != ctx.RegistrationEpoch || state.RegistrationSHA256 != ctx.RegistrationSHA256 || state.SnapshotSHA256 != ctx.SnapshotSHA256 || state.HostID != ctx.HostID || state.ProjectID != ctx.ProjectID || state.PrincipalID != ctx.PrincipalID {
		return maintenanceHold("binding_mismatch", 2)
	}
	if state.Phase == "locked" && in.Operation != "prepare" {
		if validateMaintenanceState(*state, ctx) != nil {
			return maintenanceHold("state_invalid", 2)
		}
		return maintenanceHold("rearm_required", 1)
	}
	if !(in.Operation == "prepare" && state.Phase == "locked") && !maintenanceClockValid(state, ctx.Stamp) {
		return maintenanceLock(state, ctx.Stamp, "clock_invalid")
	}
	if validateMaintenanceState(*state, ctx) != nil {
		return maintenanceHold("state_invalid", 2)
	}
	switch in.Operation {
	case "prepare":
		return maintenancePrepare(state, in, ctx)
	case "begin-approval":
		return maintenanceBegin(state, ctx)
	case "finish-approval":
		return maintenanceFinish(state, in, ctx)
	case "check":
		return maintenanceCheck(state, ctx)
	case "end":
		return maintenanceEnd(state, ctx)
	default:
		return maintenanceHold("input_invalid", 2)
	}
}
func maintenancePrepare(state *maintenanceState, in maintenanceTransitionInput, ctx maintenanceContext) maintenanceTransitionResult {
	if state.Phase != "armed" && state.Phase != "locked" {
		return maintenanceHold("state_busy", 2)
	}
	c, e := decodeMaintenanceCandidate(in.CandidateRaw)
	if e != nil {
		return maintenanceHold("candidate_invalid", 2)
	}
	if state.Phase == "armed" && c.Purpose != "maintenance" || state.Phase == "locked" && c.Purpose != "rearm-baseline" {
		return maintenanceHold("candidate_denied", 1)
	}
	a, e := maintenanceApproverFor(ctx.Controller, c.ApproverID, c.Purpose)
	if e != nil || validateMaintenanceCandidate(c, a, ctx.EffectiveControls) != nil {
		return maintenanceHold("candidate_denied", 1)
	}
	if len(in.Nonce) != 32 {
		return maintenanceHold("entropy_unavailable", 2)
	}
	if state.Sequence == math.MaxUint64 || state.AuthorizationEpoch == math.MaxUint64 {
		return maintenanceHold("state_invalid", 2)
	}
	now := maintenanceStampTime(ctx.Stamp)
	configExpiry, e := parseTime(ctx.ConfigExpiresAt)
	if e != nil {
		return maintenanceHold("config_incomplete", 2)
	}
	controllerExpiry, e := parseTime(ctx.Controller.ExpiresAt)
	if e != nil {
		return maintenanceHold("controller_invalid", 2)
	}
	validUntil := maintenanceMinTime(configExpiry, controllerExpiry)
	issued := now.Truncate(time.Second)
	if !validUntil.After(now) {
		return maintenanceHold("config_incomplete", 2)
	}
	approvalUntil := maintenanceMinTime(now.Add(300*time.Second), validUntil)
	leaseUntil := time.Time{}
	deadline := int64(0)
	if c.Purpose == "maintenance" {
		leaseUntil = maintenanceMinTime(now.Add(time.Duration(c.DurationSeconds)*time.Second), validUntil)
		approvalUntil = maintenanceMinTime(approvalUntil, leaseUntil)
		leaseUTC := leaseUntil.UTC().Truncate(time.Second)
		deltaMicros := leaseUTC.UnixMicro() - ctx.Stamp.UTCMicros
		if deltaMicros <= 0 || deltaMicros > math.MaxInt64/1000 || ctx.Stamp.ContinuousNanos > math.MaxInt64-deltaMicros*1000 {
			return maintenanceHold("proposal_expired", 2)
		}
		deadline = ctx.Stamp.ContinuousNanos + deltaMicros*1000
	}
	approvalText := maintenanceTimeText(approvalUntil)
	approvalParsed, _ := parseTime(approvalText)
	if !approvalParsed.After(issued) {
		return maintenanceHold("proposal_expired", 2)
	}
	p := maintenancePlan{Version: "harnessctl.maintenance-plan/1", Purpose: c.Purpose, InstanceID: state.InstanceID, AuthorizationEpoch: state.AuthorizationEpoch + 1, Nonce: hex.EncodeToString(in.Nonce), ControllerEpoch: ctx.ControllerEpoch, ControllerSHA256: ctx.ControllerSHA256, RegistrationEpoch: ctx.RegistrationEpoch, RegistrationSHA256: ctx.RegistrationSHA256, SnapshotSHA256: ctx.SnapshotSHA256, RollbackSnapshotSHA256: ctx.SnapshotSHA256, HostID: ctx.HostID, ProjectID: ctx.ProjectID, PrincipalID: ctx.PrincipalID, ApproverID: a.ID, ApproverPrincipalID: a.PrincipalID, KeyTag: a.KeyTag, CandidateSHA256: "", BootID: ctx.Stamp.BootID, PreparedUTCMicros: ctx.Stamp.UTCMicros, PreparedContinuousNanos: ctx.Stamp.ContinuousNanos, IssuedAt: maintenanceTimeText(now), ApprovalExpiresAt: approvalText, DurationSeconds: c.DurationSeconds, Reason: c.Reason, Controls: make([]maintenancePlanControl, 0, len(c.Controls))}
	h := sha256.Sum256(in.CandidateRaw)
	p.CandidateSHA256 = hex.EncodeToString(h[:])
	if c.Purpose == "maintenance" {
		p.LeaseExpiresAt = maintenanceTimeText(leaseUntil)
		p.DeadlineContinuousNanos = deadline
	}
	byID := map[string]effectiveControl{}
	for _, x := range ctx.EffectiveControls {
		byID[x.ID] = x
	}
	for _, x := range c.Controls {
		p.Controls = append(p.Controls, maintenancePlanControl{ID: x.ID, OldSamplePPM: byID[x.ID].SamplePPM, SamplePPM: x.SamplePPM})
	}
	sort.Slice(p.Controls, func(i, j int) bool { return p.Controls[i].ID < p.Controls[j].ID })
	if validateMaintenancePlan(p, ctx) != nil {
		return maintenanceHold("binding_mismatch", 2)
	}
	n := maintenanceClone(state)
	n.Sequence++
	n.AuthorizationEpoch++
	n.Phase = "prepared"
	origin := state.Phase
	n.PendingOrigin = &origin
	n.Plan = &p
	payload := base64.StdEncoding.EncodeToString(in.CandidateRaw)
	n.CandidatePayload = &payload
	n.Stamp = ctx.Stamp
	if origin == "locked" {
		n.ClockAnchor = ctx.Stamp
	}
	return maintenanceDone(n, true, nil)
}
func maintenanceProposalExpired(s *maintenanceState, stamp maintenanceHostStamp) bool {
	until, e := parseTime(s.Plan.ApprovalExpiresAt)
	return e != nil || !maintenanceStampTime(stamp).Before(until)
}
func maintenanceBegin(s *maintenanceState, ctx maintenanceContext) maintenanceTransitionResult {
	if s.Phase != "prepared" || s.Plan == nil {
		return maintenanceHold("proposal_missing", 2)
	}
	if maintenanceProposalExpired(s, ctx.Stamp) {
		return maintenanceOrigin(s, ctx.Stamp, "proposal_expired", 2)
	}
	if s.Sequence == math.MaxUint64 {
		return maintenanceHold("state_invalid", 2)
	}
	n := maintenanceClone(s)
	n.Sequence++
	n.Phase = "approval-pending"
	n.Stamp = ctx.Stamp
	return maintenanceDone(n, true, nil)
}
func maintenanceFinish(s *maintenanceState, in maintenanceTransitionInput, ctx maintenanceContext) maintenanceTransitionResult {
	if s.Phase != "approval-pending" || s.Plan == nil {
		return maintenanceHold("proposal_missing", 2)
	}
	if maintenanceProposalExpired(s, ctx.Stamp) {
		return maintenanceOrigin(s, ctx.Stamp, "proposal_expired", 2)
	}
	var approval maintenanceNativeApproval
	if len(in.ApprovalRaw) > 65536 || strictJSON(in.ApprovalRaw, &approval) != nil || validateMaintenanceNativeApproval(approval) != nil {
		return maintenanceOrigin(s, ctx.Stamp, "native_incomplete", 2)
	}
	switch approval.Status {
	case "denied":
		return maintenanceOrigin(s, ctx.Stamp, "native_denied", 1)
	case "canceled":
		return maintenanceOrigin(s, ctx.Stamp, "native_canceled", 1)
	case "unavailable":
		return maintenanceOrigin(s, ctx.Stamp, "native_unavailable", 2)
	case "invalid":
		return maintenanceOrigin(s, ctx.Stamp, "native_incomplete", 2)
	}
	a, e := maintenanceApproverFor(ctx.Controller, s.Plan.ApproverID, s.Plan.Purpose)
	if e != nil {
		return maintenanceOrigin(s, ctx.Stamp, "candidate_denied", 1)
	}
	planBytes, e := canonicalMaintenancePlan(*s.Plan)
	if e != nil || verifyMaintenanceApproval(planBytes, approval, a) != nil {
		return maintenanceOrigin(s, ctx.Stamp, "signature_invalid", 2)
	}
	if s.Sequence == math.MaxUint64 || s.AuthorizationEpoch == math.MaxUint64 {
		return maintenanceHold("state_invalid", 2)
	}
	n := maintenanceClone(s)
	n.Sequence++
	n.AuthorizationEpoch++
	n.Stamp = ctx.Stamp
	if s.Plan.Purpose == "rearm-baseline" {
		n.Phase = "armed"
		n.ClockAnchor = ctx.Stamp
		maintenanceClearProposal(n)
		return maintenanceDone(n, true, nil)
	}
	leaseUntil, e := parseTime(s.Plan.LeaseExpiresAt)
	if e != nil || !maintenanceStampTime(ctx.Stamp).Before(leaseUntil) || ctx.Stamp.ContinuousNanos >= s.Plan.DeadlineContinuousNanos {
		return maintenanceOrigin(s, ctx.Stamp, "proposal_expired", 2)
	}
	proof, e := json.Marshal(approval)
	if e != nil || len(proof) > 1024 {
		return maintenanceOrigin(s, ctx.Stamp, "native_incomplete", 2)
	}
	h := sha256.Sum256(proof)
	n.Phase = "maintenance"
	n.Lease = &maintenanceLease{Plan: *n.Plan, ApprovedAt: maintenanceTimeText(maintenanceStampTime(ctx.Stamp)), ExpiresAt: n.Plan.LeaseExpiresAt, DeadlineContinuousNanos: n.Plan.DeadlineContinuousNanos, ProofSHA256: hex.EncodeToString(h[:]), ApprovalPayload: base64.StdEncoding.EncodeToString(proof)}
	n.Plan = nil
	n.PendingOrigin = nil
	return maintenanceDone(n, true, nil)
}
func maintenanceCheck(s *maintenanceState, ctx maintenanceContext) maintenanceTransitionResult {
	switch s.Phase {
	case "locked":
		return maintenanceHold("rearm_required", 1)
	case "prepared", "approval-pending":
		if maintenanceProposalExpired(s, ctx.Stamp) {
			return maintenanceOrigin(s, ctx.Stamp, "proposal_expired", 2)
		}
		if s.PendingOrigin == nil || *s.PendingOrigin != "armed" {
			return maintenanceHold("rearm_required", 1)
		}
	case "maintenance":
		if s.Lease == nil {
			return maintenanceHold("state_invalid", 2)
		}
		expiry, e := parseTime(s.Lease.ExpiresAt)
		if e != nil {
			return maintenanceHold("state_invalid", 2)
		}
		if !maintenanceStampTime(ctx.Stamp).Before(expiry) || ctx.Stamp.ContinuousNanos >= s.Lease.DeadlineContinuousNanos {
			n := maintenanceClone(s)
			if !maintenanceEpochWrite(n) {
				return maintenanceHold("state_invalid", 2)
			}
			n.Phase = "armed"
			maintenanceClearProposal(n)
			n.Stamp = ctx.Stamp
			return maintenanceDone(n, true, maintenanceControlsCopy(ctx.EffectiveControls))
		}
	}
	n := maintenanceClone(s)
	write := n.Stamp != ctx.Stamp
	if !maintenanceClockWrite(n, ctx.Stamp) {
		return maintenanceHold("state_invalid", 2)
	}
	controls := maintenanceControlsCopy(ctx.EffectiveControls)
	if s.Phase == "maintenance" {
		updates := map[string]int{}
		for _, x := range s.Lease.Plan.Controls {
			updates[x.ID] = x.SamplePPM
		}
		for i := range controls {
			if sample, ok := updates[controls[i].ID]; ok {
				controls[i].SamplePPM = sample
			}
		}
	}
	return maintenanceDone(n, write, controls)
}
func maintenanceEnd(s *maintenanceState, ctx maintenanceContext) maintenanceTransitionResult {
	if s.Phase == "armed" {
		n := maintenanceClone(s)
		write := n.Stamp != ctx.Stamp
		if !maintenanceClockWrite(n, ctx.Stamp) {
			return maintenanceHold("state_invalid", 2)
		}
		return maintenanceDone(n, write, maintenanceControlsCopy(ctx.EffectiveControls))
	}
	if s.Phase != "maintenance" {
		return maintenanceHold("proposal_missing", 2)
	}
	n := maintenanceClone(s)
	if !maintenanceEpochWrite(n) {
		return maintenanceHold("state_invalid", 2)
	}
	n.Phase = "armed"
	maintenanceClearProposal(n)
	n.Stamp = ctx.Stamp
	return maintenanceDone(n, true, maintenanceControlsCopy(ctx.EffectiveControls))
}
