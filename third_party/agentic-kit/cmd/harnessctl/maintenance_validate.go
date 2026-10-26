//go:build harness_maintenance

package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maintenancePayloadType = "application/vnd.agentic-kit.harness-maintenance-plan.v1+json"

var errMaintenance = errors.New("invalid maintenance record")

type maintenanceContext struct {
	Controller         maintenanceController
	ControllerEpoch    int
	ControllerSHA256   string
	RegistrationEpoch  int
	RegistrationSHA256 string
	SnapshotSHA256     string
	HostID             string
	ProjectID          string
	PrincipalID        string
	ConfigExpiresAt    string
	EffectiveControls  []effectiveControl
	Stamp              maintenanceHostStamp
}

func decodeMaintenanceController(raw []byte) (maintenanceController, error) {
	var v maintenanceController
	if len(raw) > 2<<20 || strictJSON(raw, &v) != nil {
		return v, errMaintenance
	}
	return v, nil
}
func decodeMaintenanceCandidate(raw []byte) (maintenanceCandidate, error) {
	var v maintenanceCandidate
	if len(raw) > 8192 || strictJSON(raw, &v) != nil {
		return v, errMaintenance
	}
	return v, nil
}
func decodeMaintenanceState(raw []byte) (maintenanceState, error) {
	var v maintenanceState
	if len(raw) > 65536 || strictJSON(raw, &v) != nil {
		return v, errMaintenance
	}
	return v, nil
}
func decodeMaintenanceHostStamp(raw []byte) (maintenanceHostStamp, error) {
	var v maintenanceHostStamp
	if strictJSON(raw, &v) != nil || validateMaintenanceHostStamp(v) != nil {
		return v, errMaintenance
	}
	return v, nil
}
func decodeMaintenanceNativeApproval(raw []byte) (maintenanceNativeApproval, error) {
	var v maintenanceNativeApproval
	if strictJSON(raw, &v) != nil || validateMaintenanceNativeApproval(v) != nil {
		return v, errMaintenance
	}
	return v, nil
}

func maintenanceAbs(p string) bool {
	return len(p) > 0 && len(p) <= 4096 && filepath.IsAbs(p) && filepath.Clean(p) == p && canonical(p)
}
func maintenanceHash(h string) bool { return hashPattern.MatchString(h) }
func maintenanceHex(h string) bool  { return maintenanceHash(h) }
func maintenanceDecimal(s string) bool {
	if s == "0" {
		return true
	}
	if len(s) == 0 || s[0] == '0' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	_, e := strconv.ParseUint(s, 10, 64)
	return e == nil
}
func maintenanceBoot(s string) bool {
	if len(s) < 1 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}
func maintenanceReason(s string) bool {
	if s == "" || len(s) > 512 || !utf8.ValidString(s) || strings.TrimSpace(s) == "" {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
			return false
		}
	}
	return true
}
func maintenancePoint(s string) (*ecdsa.PublicKey, error) {
	b, e := base64.StdEncoding.DecodeString(s)
	if e != nil || len(b) != 65 || base64.StdEncoding.EncodeToString(b) != s || b[0] != 4 {
		return nil, errMaintenance
	}
	x, y := elliptic.Unmarshal(elliptic.P256(), b)
	if x == nil || y == nil {
		return nil, errMaintenance
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}
func maintenanceApproverValid(a maintenanceApprover) bool {
	if !validID(a.ID) || !validID(a.PrincipalID) || !maintenanceDecimal(a.OSUserID) || !validID(a.KeyTag) || a.EnrollmentProfile != "secure-enclave-user-presence/1" || a.MaxDurationSeconds < 1 || a.MaxDurationSeconds > 3600 || !uniqueIDs(a.ControlIDs, 1, 64) || len(a.Roles) < 1 || len(a.Roles) > 2 {
		return false
	}
	if _, e := maintenancePoint(a.PublicKey); e != nil {
		return false
	}
	seen := map[string]bool{}
	for _, r := range a.Roles {
		if !oneOf(r, "maintenance", "rearm-baseline") || seen[r] {
			return false
		}
		seen[r] = true
	}
	return true
}
func validateMaintenanceController(c maintenanceController, reg registration, selected project, registrationPath string, now time.Time) (maintenanceProject, error) {
	var out maintenanceProject
	if c.Version != "harnessctl.controller/1" || c.Epoch < 1 || c.Epoch > math.MaxInt32 || c.HostID != reg.HostID || !validID(c.HostID) || len(c.Projects) < 1 || len(c.Projects) > 256 || len(c.Approvers) < 1 || len(c.Approvers) > 64 || !hookControllerValid(c) {
		return out, errMaintenance
	}
	if _, _, e := interval(c.IssuedAt, c.ExpiresAt, now, 366*24*time.Hour); e != nil {
		return out, errMaintenance
	}
	n := c.Native
	if n.Profile != "darwin-protected-key/1" || !maintenanceAbs(n.Executable) || !maintenanceHash(n.SHA256) || !maintenanceAbs(n.PreparationRoot) || n.TimeoutMillis < 1 || n.TimeoutMillis > 120000 {
		return out, errMaintenance
	}
	aid := map[string]bool{}
	tags := map[string]bool{}
	for _, a := range c.Approvers {
		if !maintenanceApproverValid(a) || aid[a.ID] || tags[a.KeyTag] {
			return out, errMaintenance
		}
		aid[a.ID] = true
		tags[a.KeyTag] = true
	}
	ids := map[string]bool{}
	roots := map[string]bool{}
	states := map[string]bool{}
	found := false
	for _, p := range c.Projects {
		if !validID(p.ID) || !validID(p.PrincipalID) || !maintenanceAbs(p.Root) || !maintenanceAbs(p.Registration) || !maintenanceAbs(p.StateRoot) || ids[p.ID] || roots[p.Root] || states[p.StateRoot] {
			return out, errMaintenance
		}
		ids[p.ID] = true
		roots[p.Root] = true
		states[p.StateRoot] = true
		if p.ID == selected.ID {
			if p.Root != selected.Root || p.PrincipalID != selected.PrincipalID || p.Registration != registrationPath {
				return out, errMaintenance
			}
			out = p
			found = true
		}
	}
	if !found {
		return out, errMaintenance
	}
	return out, nil
}
func maintenanceApproverFor(c maintenanceController, id, role string) (maintenanceApprover, error) {
	for _, a := range c.Approvers {
		if a.ID == id && !a.Revoked && contains(a.Roles, role) && maintenanceApproverValid(a) {
			return a, nil
		}
	}
	return maintenanceApprover{}, errMaintenance
}
func validateMaintenanceCandidate(c maintenanceCandidate, a maintenanceApprover, controls []effectiveControl) error {
	if c.Version != "harnessctl.advisory-candidate/1" || !oneOf(c.Purpose, "maintenance", "rearm-baseline") || c.ApproverID != a.ID || a.Revoked || !contains(a.Roles, c.Purpose) || !maintenanceReason(c.Reason) || len(c.Controls) > 8 {
		return errMaintenance
	}
	if c.Purpose == "rearm-baseline" {
		if c.DurationSeconds != 0 || len(c.Controls) != 0 {
			return errMaintenance
		}
		return nil
	}
	if c.DurationSeconds < 1 || c.DurationSeconds > 3600 || c.DurationSeconds > a.MaxDurationSeconds || len(c.Controls) < 1 {
		return errMaintenance
	}
	seen := map[string]bool{}
	byID := map[string]effectiveControl{}
	for _, e := range controls {
		byID[e.ID] = e
	}
	for _, x := range c.Controls {
		e, ok := byID[x.ID]
		if !ok || seen[x.ID] || !contains(a.ControlIDs, x.ID) || e.Class != "advisory" || !e.AllowMaintenance || x.SamplePPM < e.MinSamplePPM || x.SamplePPM > e.MaxSamplePPM || x.SamplePPM < 0 || x.SamplePPM > 1000000 {
			return errMaintenance
		}
		seen[x.ID] = true
	}
	return nil
}
func validateMaintenanceHostStamp(s maintenanceHostStamp) error {
	if s.Version != "harnessctl.host-stamp/1" || s.OS != "darwin" || !maintenanceDecimal(s.OSUserID) || !maintenanceBoot(s.BootID) || s.UTCMicros <= 0 || s.ContinuousNanos < 0 {
		return errMaintenance
	}
	return nil
}
func validateMaintenanceNativeApproval(a maintenanceNativeApproval) error {
	if a.Version != "harnessctl.native-approval/1" || !oneOf(a.Status, "signed", "denied", "unavailable", "canceled", "invalid") {
		return errMaintenance
	}
	if a.Status != "signed" {
		if a.Signature != "" || a.PublicKey != "" {
			return errMaintenance
		}
		return nil
	}
	if _, e := maintenancePoint(a.PublicKey); e != nil {
		return errMaintenance
	}
	s, e := base64.StdEncoding.DecodeString(a.Signature)
	if e != nil || len(s) < 8 || len(s) > 72 || base64.StdEncoding.EncodeToString(s) != a.Signature {
		return errMaintenance
	}
	return nil
}
func canonicalMaintenancePlan(p maintenancePlan) ([]byte, error) {
	if validateMaintenancePlanShape(p) != nil {
		return nil, errMaintenance
	}
	q := p
	q.Controls = append([]maintenancePlanControl{}, p.Controls...)
	sort.Slice(q.Controls, func(i, j int) bool { return q.Controls[i].ID < q.Controls[j].ID })
	return json.Marshal(q)
}
func validateMaintenancePlanShape(p maintenancePlan) error {
	if p.Version != "harnessctl.maintenance-plan/1" || !oneOf(p.Purpose, "maintenance", "rearm-baseline") || !maintenanceHex(p.InstanceID) || p.AuthorizationEpoch < 1 || !maintenanceHex(p.Nonce) || p.ControllerEpoch < 1 || p.ControllerEpoch > math.MaxInt32 || !maintenanceHash(p.ControllerSHA256) || p.RegistrationEpoch < 1 || p.RegistrationEpoch > math.MaxInt32 || !maintenanceHash(p.RegistrationSHA256) || !maintenanceHash(p.SnapshotSHA256) || !maintenanceHash(p.RollbackSnapshotSHA256) || !validID(p.HostID) || !validID(p.ProjectID) || !validID(p.PrincipalID) || !validID(p.ApproverID) || !validID(p.ApproverPrincipalID) || !validID(p.KeyTag) || !maintenanceHash(p.CandidateSHA256) || !maintenanceBoot(p.BootID) || p.PreparedUTCMicros <= 0 || p.PreparedContinuousNanos < 0 || !maintenanceReason(p.Reason) || len(p.Controls) > 8 {
		return errMaintenance
	}
	issued, e := parseTime(p.IssuedAt)
	if e != nil || issued.Unix() != time.UnixMicro(p.PreparedUTCMicros).UTC().Unix() {
		return errMaintenance
	}
	approval, e := parseTime(p.ApprovalExpiresAt)
	if e != nil || !approval.After(issued) || approval.Sub(issued) > 300*time.Second {
		return errMaintenance
	}
	if p.Purpose == "rearm-baseline" {
		if p.DurationSeconds != 0 || p.DeadlineContinuousNanos != 0 || p.LeaseExpiresAt != "" || len(p.Controls) != 0 {
			return errMaintenance
		}
		return nil
	}
	if p.DurationSeconds < 1 || p.DurationSeconds > 3600 || len(p.Controls) < 1 || p.DeadlineContinuousNanos <= p.PreparedContinuousNanos {
		return errMaintenance
	}
	lease, e := parseTime(p.LeaseExpiresAt)
	if e != nil || !lease.After(issued) || lease.After(issued.Add(time.Duration(p.DurationSeconds)*time.Second)) || approval.After(lease) || lease.UnixMicro() <= p.PreparedUTCMicros {
		return errMaintenance
	}
	deltaMicros := lease.UnixMicro() - p.PreparedUTCMicros
	if deltaMicros > math.MaxInt64/1000 || p.PreparedContinuousNanos > math.MaxInt64-deltaMicros*1000 || p.DeadlineContinuousNanos != p.PreparedContinuousNanos+deltaMicros*1000 {
		return errMaintenance
	}
	seen := map[string]bool{}
	for _, x := range p.Controls {
		if !validID(x.ID) || seen[x.ID] || x.OldSamplePPM < 0 || x.OldSamplePPM > 1000000 || x.SamplePPM < 0 || x.SamplePPM > 1000000 {
			return errMaintenance
		}
		seen[x.ID] = true
	}
	return nil
}
func validateMaintenancePlan(p maintenancePlan, ctx maintenanceContext) error {
	for i := 1; i < len(p.Controls); i++ {
		if p.Controls[i-1].ID >= p.Controls[i].ID {
			return errMaintenance
		}
	}
	if validateMaintenancePlanShape(p) != nil || ctx.Controller.Epoch != ctx.ControllerEpoch || p.ControllerEpoch != ctx.ControllerEpoch || p.ControllerSHA256 != ctx.ControllerSHA256 || p.RegistrationEpoch != ctx.RegistrationEpoch || p.RegistrationSHA256 != ctx.RegistrationSHA256 || p.SnapshotSHA256 != ctx.SnapshotSHA256 || p.RollbackSnapshotSHA256 != ctx.SnapshotSHA256 || p.HostID != ctx.HostID || p.ProjectID != ctx.ProjectID || p.PrincipalID != ctx.PrincipalID || p.BootID != ctx.Stamp.BootID {
		return errMaintenance
	}
	approver, e := maintenanceApproverFor(ctx.Controller, p.ApproverID, p.Purpose)
	if e != nil || approver.PrincipalID != p.ApproverPrincipalID || approver.KeyTag != p.KeyTag || approver.OSUserID != ctx.Stamp.OSUserID || validateMaintenanceHostStamp(ctx.Stamp) != nil {
		return errMaintenance
	}
	if p.Purpose == "maintenance" && p.DurationSeconds > approver.MaxDurationSeconds {
		return errMaintenance
	}
	expiry, e := parseTime(ctx.ConfigExpiresAt)
	if e != nil {
		return errMaintenance
	}
	approval, _ := parseTime(p.ApprovalExpiresAt)
	controllerExpiry, e := parseTime(ctx.Controller.ExpiresAt)
	if e != nil || approval.After(controllerExpiry) {
		return errMaintenance
	}
	if approval.After(expiry) {
		return errMaintenance
	}
	if p.Purpose == "maintenance" {
		lease, _ := parseTime(p.LeaseExpiresAt)
		if lease.After(expiry) || lease.After(controllerExpiry) {
			return errMaintenance
		}
		byID := map[string]effectiveControl{}
		for _, x := range ctx.EffectiveControls {
			byID[x.ID] = x
		}
		for _, x := range p.Controls {
			base, ok := byID[x.ID]
			if !ok || !contains(approver.ControlIDs, x.ID) || base.Class != "advisory" || !base.AllowMaintenance || x.OldSamplePPM != base.SamplePPM || x.SamplePPM < base.MinSamplePPM || x.SamplePPM > base.MaxSamplePPM {
				return errMaintenance
			}
		}
	}
	return nil
}
func verifyMaintenanceApproval(planBytes []byte, a maintenanceNativeApproval, enrolled maintenanceApprover) error {
	if validateMaintenanceNativeApproval(a) != nil || a.Status != "signed" || a.PublicKey != enrolled.PublicKey || enrolled.Revoked || !maintenanceApproverValid(enrolled) {
		return errMaintenance
	}
	var plan maintenancePlan
	if strictJSON(planBytes, &plan) != nil {
		return errMaintenance
	}
	canonical, e := canonicalMaintenancePlan(plan)
	if e != nil || !bytes.Equal(canonical, planBytes) || plan.ApproverID != enrolled.ID || plan.ApproverPrincipalID != enrolled.PrincipalID || plan.KeyTag != enrolled.KeyTag || !contains(enrolled.Roles, plan.Purpose) {
		return errMaintenance
	}
	pk, e := maintenancePoint(enrolled.PublicKey)
	if e != nil {
		return errMaintenance
	}
	sig, e := base64.StdEncoding.DecodeString(a.Signature)
	if e != nil {
		return errMaintenance
	}
	pae := []byte("DSSEv1 " + strconv.Itoa(len(maintenancePayloadType)) + " " + maintenancePayloadType + " " + strconv.Itoa(len(planBytes)) + " ")
	pae = append(pae, planBytes...)
	digest := sha256.Sum256(pae)
	if !ecdsa.VerifyASN1(pk, digest[:], sig) {
		return errMaintenance
	}
	return nil
}
func validateMaintenanceState(s maintenanceState, ctx maintenanceContext) error {
	if s.Version != "harnessctl.maintenance-state/1" || !maintenanceHex(s.InstanceID) || s.Sequence < 1 || s.AuthorizationEpoch < 1 || !oneOf(s.Phase, "armed", "prepared", "approval-pending", "maintenance", "locked") || s.ControllerEpoch != ctx.ControllerEpoch || s.ControllerSHA256 != ctx.ControllerSHA256 || s.RegistrationEpoch != ctx.RegistrationEpoch || s.RegistrationSHA256 != ctx.RegistrationSHA256 || s.SnapshotSHA256 != ctx.SnapshotSHA256 || s.HostID != ctx.HostID || s.ProjectID != ctx.ProjectID || s.PrincipalID != ctx.PrincipalID || validateMaintenanceHostStamp(s.Stamp) != nil || validateMaintenanceHostStamp(s.ClockAnchor) != nil {
		return errMaintenance
	}
	if s.Phase == "armed" || s.Phase == "locked" {
		if s.Plan != nil || s.PendingOrigin != nil || s.CandidatePayload != nil || s.Lease != nil {
			return errMaintenance
		}
		return nil
	}
	if s.CandidatePayload == nil {
		return errMaintenance
	}
	raw, e := base64.StdEncoding.DecodeString(*s.CandidatePayload)
	if e != nil || len(raw) > 8192 || base64.StdEncoding.EncodeToString(raw) != *s.CandidatePayload {
		return errMaintenance
	}
	candidate, e := decodeMaintenanceCandidate(raw)
	if e != nil {
		return errMaintenance
	}
	var p maintenancePlan
	if s.Phase == "maintenance" {
		if s.Lease == nil || s.Plan != nil || s.PendingOrigin != nil || s.Lease.Plan.Purpose != "maintenance" || s.Lease.Plan.AuthorizationEpoch == math.MaxUint64 || s.Lease.Plan.AuthorizationEpoch+1 != s.AuthorizationEpoch {
			return errMaintenance
		}
		p = s.Lease.Plan
		if s.Lease.ExpiresAt != p.LeaseExpiresAt || s.Lease.DeadlineContinuousNanos != p.DeadlineContinuousNanos || !maintenanceHash(s.Lease.ProofSHA256) {
			return errMaintenance
		}
		approved, e := parseTime(s.Lease.ApprovedAt)
		issued, _ := parseTime(p.IssuedAt)
		approvalUntil, _ := parseTime(p.ApprovalExpiresAt)
		leaseUntil, _ := parseTime(p.LeaseExpiresAt)
		if e != nil || approved.Before(issued) || !approved.Before(approvalUntil) || !approved.Before(leaseUntil) {
			return errMaintenance
		}
		if len(s.Lease.ApprovalPayload) > 1368 {
			return errMaintenance
		}
		proof, e := base64.StdEncoding.DecodeString(s.Lease.ApprovalPayload)
		if e != nil || len(proof) == 0 || len(proof) > 1024 || base64.StdEncoding.EncodeToString(proof) != s.Lease.ApprovalPayload {
			return errMaintenance
		}
		h := sha256.Sum256(proof)
		if hex.EncodeToString(h[:]) != s.Lease.ProofSHA256 {
			return errMaintenance
		}
		var approval maintenanceNativeApproval
		if strictJSON(proof, &approval) != nil {
			return errMaintenance
		}
		canon, e := json.Marshal(approval)
		if e != nil || !bytes.Equal(canon, proof) {
			return errMaintenance
		}
		approver, e := maintenanceApproverFor(ctx.Controller, p.ApproverID, p.Purpose)
		if e != nil {
			return errMaintenance
		}
		planBytes, e := canonicalMaintenancePlan(p)
		if e != nil || verifyMaintenanceApproval(planBytes, approval, approver) != nil {
			return errMaintenance
		}
	} else {
		if s.Plan == nil || s.Lease != nil || s.PendingOrigin == nil || !oneOf(*s.PendingOrigin, "armed", "locked") || s.Plan.AuthorizationEpoch != s.AuthorizationEpoch {
			return errMaintenance
		}
		p = *s.Plan
		if (p.Purpose == "maintenance") != (*s.PendingOrigin == "armed") {
			return errMaintenance
		}
	}
	if validateMaintenanceCandidate(candidate, maintenanceApproverByID(ctx.Controller, p.ApproverID), ctx.EffectiveControls) != nil {
		return errMaintenance
	}
	if p.InstanceID != s.InstanceID || p.AuthorizationEpoch < 1 || p.ControllerEpoch != s.ControllerEpoch || p.ControllerSHA256 != s.ControllerSHA256 || p.RegistrationEpoch != s.RegistrationEpoch || p.RegistrationSHA256 != s.RegistrationSHA256 || p.SnapshotSHA256 != s.SnapshotSHA256 || p.HostID != s.HostID || p.ProjectID != s.ProjectID || p.PrincipalID != s.PrincipalID || validateMaintenancePlan(p, ctx) != nil {
		return errMaintenance
	}
	h := sha256.Sum256(raw)
	if p.CandidateSHA256 != hex.EncodeToString(h[:]) || candidate.Purpose != p.Purpose || candidate.ApproverID != p.ApproverID || candidate.DurationSeconds != p.DurationSeconds || candidate.Reason != p.Reason || len(candidate.Controls) != len(p.Controls) {
		return errMaintenance
	}
	byID := map[string]int{}
	for _, x := range p.Controls {
		byID[x.ID] = x.SamplePPM
	}
	for _, x := range candidate.Controls {
		rate, ok := byID[x.ID]
		if !ok || rate != x.SamplePPM {
			return errMaintenance
		}
	}
	return nil
}

func validateMaintenanceReport(r maintenanceReport) error {
	if r.Version != "harnessctl.maintenance-report/1" || !oneOf(r.Status, "complete", "hold") || !oneOf(r.Operation, "init", "prepare", "enter", "check", "end", "recover", "unknown") || !oneOf(r.Code, "ok", "unsupported", "input_invalid", "controller_invalid", "controller_changed", "project_unregistered", "config_incomplete", "policy_conflict", "candidate_invalid", "candidate_denied", "state_unavailable", "state_invalid", "state_busy", "state_ambiguous", "state_exists", "clock_invalid", "rearm_required", "proposal_expired", "proposal_missing", "native_unavailable", "native_denied", "native_canceled", "native_incomplete", "signature_invalid", "binding_mismatch", "nonce_replayed", "storage_incomplete", "output_incomplete", "entropy_unavailable") || !r.NoHostActionAuthority || r.Installation != "unobserved" || r.Coverage != "selected-project-only" {
		return errMaintenance
	}
	if r.Status == "complete" {
		if r.Code != "ok" || !oneOf(r.Phase, "armed", "prepared", "approval-pending", "maintenance", "locked") {
			return errMaintenance
		}
	} else if r.Code == "ok" || r.Phase != "" {
		return errMaintenance
	}
	if r.StateWritesAttempted < 0 || r.StateWritesCommitted < 0 || r.StateWritesCommitted > r.StateWritesAttempted || r.NativeStampAttempts < 0 || r.NativeStampInvoked < 0 || r.NativeStampCompleted < 0 || r.NativeStampCompleted > r.NativeStampInvoked || r.NativeStampInvoked > r.NativeStampAttempts || r.NativeApprovalAttempts < 0 || r.NativeApprovalAttempts > 1 || r.NativeApprovalInvoked < 0 || r.NativeApprovalInvoked > r.NativeApprovalAttempts || r.NativeApprovalCompleted < 0 || r.NativeApprovalCompleted > r.NativeApprovalInvoked || r.NativeSignatureVerified && r.NativeApprovalCompleted != 1 {
		return errMaintenance
	}
	return nil
}

func maintenanceApproverByID(c maintenanceController, id string) maintenanceApprover {
	for _, a := range c.Approvers {
		if a.ID == id {
			return a
		}
	}
	return maintenanceApprover{}
}
