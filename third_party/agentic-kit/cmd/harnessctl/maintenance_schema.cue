package harnessctl

import (
	"list"
	"strings"
)

_maintenanceEpoch:   int & >=1 & <=2147483647
_maintenanceSeconds: int & >=0 & <=3600
_maintenanceHex:     _hash
_maintenancePurpose: "maintenance" | "rearm-baseline"
_maintenancePoint64: string & =~"^[A-Za-z0-9+/]{87}=$"
_maintenanceBytes64: string & strings.MaxRunes(10924) & =~"^([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$"
_maintenanceReason:  string & !="" & strings.MaxRunes(512) & !~"[\\p{Cc}\\p{Cf}\\p{Zl}\\p{Zp}]"
_maintenanceBoot:    string & !="" & strings.MaxRunes(128) & =~"^[!-~]+$"
_maintenanceUint64:  int & >=0 & <=18446744073709551615
_maintenanceCounter: int & >=0 & <=9223372036854775807

#MaintenanceNative: close({profile!: "darwin-protected-key/1", executable!: _abs, sha256!: _hash, preparationRoot!: _abs, timeoutMillis!: int & >=1 & <=120000})
#MaintenanceApprover: close({id!: _id, principalId!: _id, osUserId!: string & =~"^(0|[1-9][0-9]*)$", keyTag!: _id, publicKey!: _maintenancePoint64, enrollmentProfile!: "secure-enclave-user-presence/1", revoked!: bool, roles!: [...("maintenance" | "rearm-baseline")] & list.MinItems(1) & list.MaxItems(2) & list.UniqueItems(), controlIds!: _ids1_64, maxDurationSeconds!: int & >=1 & <=3600})
#MaintenanceProject: close({id!: _id, root!: _abs, principalId!: _id, registration!: _abs, stateRoot!: _abs})
#HookReference: close({projectId!: _id, host!: "claude" | "codex", path!: _abs, sha256!: _hash})
#MaintenanceController: close({version!: "harnessctl.controller/1", epoch!: _maintenanceEpoch, issuedAt!: _time, expiresAt!: _time, hostId!: _id, native!: #MaintenanceNative, approvers!: [...#MaintenanceApprover] & list.MinItems(1) & list.MaxItems(64), projects!: [...#MaintenanceProject] & list.MinItems(1) & list.MaxItems(256), hooks?: [...#HookReference] & list.MinItems(1) & list.MaxItems(512)})
#MaintenanceCandidateControl: close({id!: _id, samplePPM!: _ppm})
#MaintenanceCandidate: close({version!: "harnessctl.advisory-candidate/1", purpose!: _maintenancePurpose, approverId!: _id, durationSeconds!: _maintenanceSeconds, reason!: _maintenanceReason, controls!: [...#MaintenanceCandidateControl] & list.MaxItems(8), _reasonBytes: true & (len(reason) <= 512)})
#MaintenancePlanControl: close({id!: _id, oldSamplePPM!: _ppm, samplePPM!: _ppm})
#MaintenancePlan: close({version!: "harnessctl.maintenance-plan/1", purpose!: _maintenancePurpose, instanceId!: _maintenanceHex, authorizationEpoch!: _maintenanceUint64 & >=1, nonce!: _maintenanceHex, controllerEpoch!: _maintenanceEpoch, controllerSHA256!: _hash, registrationEpoch!: _maintenanceEpoch, registrationSHA256!: _hash, snapshotSHA256!: _hash, rollbackSnapshotSHA256!: _hash, hostId!: _id, projectId!: _id, principalId!: _id, approverId!: _id, approverPrincipalId!: _id, keyTag!: _id, candidateSHA256!: _hash, bootId!: _maintenanceBoot, preparedUtcMicros!: _maintenanceCounter & >=1, preparedContinuousNanos!: _maintenanceCounter, issuedAt!: _time, approvalExpiresAt!: _time, leaseExpiresAt!: string, deadlineContinuousNanos!: _maintenanceCounter, durationSeconds!: _maintenanceSeconds, reason!: _maintenanceReason, controls!: [...#MaintenancePlanControl] & list.MaxItems(8), _reasonBytes: true & (len(reason) <= 512)})
#MaintenanceHostStamp: close({version!: "harnessctl.host-stamp/1", os!: "darwin", osUserId!: string & =~"^(0|[1-9][0-9]*)$", bootId!: _maintenanceBoot, utcMicros!: _maintenanceCounter & >=1, continuousNanos!: _maintenanceCounter})
#MaintenanceNativeApproval: close({version!: "harnessctl.native-approval/1", status!: "signed" | "denied" | "unavailable" | "canceled" | "invalid", signature!: string, publicKey!: string})
#MaintenanceLease: close({plan!: #MaintenancePlan, approvedAt!: _time, expiresAt!: _time, deadlineContinuousNanos!: _maintenanceCounter, proofSHA256!: _hash, approvalPayload!: string & strings.MaxRunes(1368)})
_maintenanceStateBase: {version!: "harnessctl.maintenance-state/1", instanceId!: _maintenanceHex, sequence!: _maintenanceUint64 & >=1, authorizationEpoch!: _maintenanceUint64 & >=1, controllerEpoch!: _maintenanceEpoch, controllerSHA256!: _hash, registrationEpoch!: _maintenanceEpoch, registrationSHA256!: _hash, snapshotSHA256!: _hash, hostId!: _id, projectId!: _id, principalId!: _id, stamp!: #MaintenanceHostStamp, clockAnchor!: #MaintenanceHostStamp}
#MaintenanceState: (close(_maintenanceStateBase & {phase!: "armed" | "locked"}) | close(_maintenanceStateBase & {phase!: "prepared" | "approval-pending", plan!: #MaintenancePlan & {purpose: "maintenance"}, pendingOrigin!: "armed", candidatePayload!: _maintenanceBytes64}) | close(_maintenanceStateBase & {phase!: "prepared" | "approval-pending", plan!: #MaintenancePlan & {purpose: "rearm-baseline"}, pendingOrigin!: "locked", candidatePayload!: _maintenanceBytes64}) | close(_maintenanceStateBase & {phase!: "maintenance", lease!: #MaintenanceLease & {plan: #MaintenancePlan & {purpose: "maintenance"}}, candidatePayload!: _maintenanceBytes64}))
_maintenanceReportFailureCode: "unsupported" | "input_invalid" | "controller_invalid" | "controller_changed" | "project_unregistered" | "config_incomplete" | "policy_conflict" | "candidate_invalid" | "candidate_denied" | "state_unavailable" | "state_invalid" | "state_busy" | "state_ambiguous" | "state_exists" | "clock_invalid" | "rearm_required" | "proposal_expired" | "proposal_missing" | "native_unavailable" | "native_denied" | "native_canceled" | "native_incomplete" | "signature_invalid" | "binding_mismatch" | "nonce_replayed" | "storage_incomplete" | "output_incomplete" | "entropy_unavailable"
_maintenanceReportBase: {version!: "harnessctl.maintenance-report/1", operation!: "init" | "prepare" | "enter" | "check" | "end" | "recover" | "unknown", noHostActionAuthority!: true, installation!: "unobserved", coverage!: "selected-project-only", stateWritesAttempted!: _maintenanceCounter, stateWritesCommitted!: _maintenanceCounter, nativeStampAttempts!: _maintenanceCounter, nativeStampInvoked!: _maintenanceCounter, nativeStampCompleted!: _maintenanceCounter, nativeApprovalAttempts!: _maintenanceCounter & <=1, nativeApprovalInvoked!: _maintenanceCounter, nativeApprovalCompleted!: _maintenanceCounter, nativeSignatureVerified!: bool, storedSignatureVerified!: bool, _witness: true & (stateWritesCommitted <= stateWritesAttempted) & (nativeStampInvoked <= nativeStampAttempts) & (nativeStampCompleted <= nativeStampInvoked) & (nativeApprovalInvoked <= nativeApprovalAttempts) & (nativeApprovalCompleted <= nativeApprovalInvoked)}
#MaintenanceReport: (close(_maintenanceReportBase & {status!: "complete", code!: "ok", phase!: "armed" | "prepared" | "approval-pending" | "maintenance" | "locked"}) | close(_maintenanceReportBase & {status!: "hold", code!: _maintenanceReportFailureCode})) & ({nativeSignatureVerified: false} | {nativeSignatureVerified: true, nativeApprovalCompleted: 1})
