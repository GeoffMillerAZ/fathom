//go:build harness_maintenance

package main

type maintenanceController struct {
	Version   string                `json:"version"`
	Epoch     int                   `json:"epoch"`
	IssuedAt  string                `json:"issuedAt"`
	ExpiresAt string                `json:"expiresAt"`
	HostID    string                `json:"hostId"`
	Native    maintenanceNative     `json:"native"`
	Approvers []maintenanceApprover `json:"approvers"`
	Projects  []maintenanceProject  `json:"projects"`
	Hooks     []hookReference       `json:"hooks,omitempty"`
}
type maintenanceNative struct {
	Profile         string `json:"profile"`
	Executable      string `json:"executable"`
	SHA256          string `json:"sha256"`
	PreparationRoot string `json:"preparationRoot"`
	TimeoutMillis   int    `json:"timeoutMillis"`
}
type maintenanceApprover struct {
	ID                 string   `json:"id"`
	PrincipalID        string   `json:"principalId"`
	OSUserID           string   `json:"osUserId"`
	KeyTag             string   `json:"keyTag"`
	PublicKey          string   `json:"publicKey"`
	EnrollmentProfile  string   `json:"enrollmentProfile"`
	Revoked            bool     `json:"revoked"`
	Roles              []string `json:"roles"`
	ControlIDs         []string `json:"controlIds"`
	MaxDurationSeconds int      `json:"maxDurationSeconds"`
}
type maintenanceProject struct {
	ID           string `json:"id"`
	Root         string `json:"root"`
	PrincipalID  string `json:"principalId"`
	Registration string `json:"registration"`
	StateRoot    string `json:"stateRoot"`
}
type maintenanceCandidate struct {
	Version         string                        `json:"version"`
	Purpose         string                        `json:"purpose"`
	ApproverID      string                        `json:"approverId"`
	DurationSeconds int                           `json:"durationSeconds"`
	Reason          string                        `json:"reason"`
	Controls        []maintenanceCandidateControl `json:"controls"`
}
type maintenanceCandidateControl struct {
	ID        string `json:"id"`
	SamplePPM int    `json:"samplePPM"`
}
type maintenancePlan struct {
	Version                 string                   `json:"version"`
	Purpose                 string                   `json:"purpose"`
	InstanceID              string                   `json:"instanceId"`
	AuthorizationEpoch      uint64                   `json:"authorizationEpoch"`
	Nonce                   string                   `json:"nonce"`
	ControllerEpoch         int                      `json:"controllerEpoch"`
	ControllerSHA256        string                   `json:"controllerSHA256"`
	RegistrationEpoch       int                      `json:"registrationEpoch"`
	RegistrationSHA256      string                   `json:"registrationSHA256"`
	SnapshotSHA256          string                   `json:"snapshotSHA256"`
	RollbackSnapshotSHA256  string                   `json:"rollbackSnapshotSHA256"`
	HostID                  string                   `json:"hostId"`
	ProjectID               string                   `json:"projectId"`
	PrincipalID             string                   `json:"principalId"`
	ApproverID              string                   `json:"approverId"`
	ApproverPrincipalID     string                   `json:"approverPrincipalId"`
	KeyTag                  string                   `json:"keyTag"`
	CandidateSHA256         string                   `json:"candidateSHA256"`
	BootID                  string                   `json:"bootId"`
	PreparedUTCMicros       int64                    `json:"preparedUtcMicros"`
	PreparedContinuousNanos int64                    `json:"preparedContinuousNanos"`
	IssuedAt                string                   `json:"issuedAt"`
	ApprovalExpiresAt       string                   `json:"approvalExpiresAt"`
	LeaseExpiresAt          string                   `json:"leaseExpiresAt"`
	DeadlineContinuousNanos int64                    `json:"deadlineContinuousNanos"`
	DurationSeconds         int                      `json:"durationSeconds"`
	Reason                  string                   `json:"reason"`
	Controls                []maintenancePlanControl `json:"controls"`
}
type maintenancePlanControl struct {
	ID           string `json:"id"`
	OldSamplePPM int    `json:"oldSamplePPM"`
	SamplePPM    int    `json:"samplePPM"`
}
type maintenanceHostStamp struct {
	Version         string `json:"version"`
	OS              string `json:"os"`
	OSUserID        string `json:"osUserId"`
	BootID          string `json:"bootId"`
	UTCMicros       int64  `json:"utcMicros"`
	ContinuousNanos int64  `json:"continuousNanos"`
}
type maintenanceNativeApproval struct {
	Version   string `json:"version"`
	Status    string `json:"status"`
	Signature string `json:"signature"`
	PublicKey string `json:"publicKey"`
}
type maintenanceLease struct {
	Plan                    maintenancePlan `json:"plan"`
	ApprovedAt              string          `json:"approvedAt"`
	ExpiresAt               string          `json:"expiresAt"`
	DeadlineContinuousNanos int64           `json:"deadlineContinuousNanos"`
	ProofSHA256             string          `json:"proofSHA256"`
	ApprovalPayload         string          `json:"approvalPayload"`
}
type maintenanceState struct {
	Version            string               `json:"version"`
	InstanceID         string               `json:"instanceId"`
	Sequence           uint64               `json:"sequence"`
	AuthorizationEpoch uint64               `json:"authorizationEpoch"`
	Phase              string               `json:"phase"`
	ControllerEpoch    int                  `json:"controllerEpoch"`
	ControllerSHA256   string               `json:"controllerSHA256"`
	RegistrationEpoch  int                  `json:"registrationEpoch"`
	RegistrationSHA256 string               `json:"registrationSHA256"`
	SnapshotSHA256     string               `json:"snapshotSHA256"`
	HostID             string               `json:"hostId"`
	ProjectID          string               `json:"projectId"`
	PrincipalID        string               `json:"principalId"`
	Stamp              maintenanceHostStamp `json:"stamp"`
	ClockAnchor        maintenanceHostStamp `json:"clockAnchor"`
	Plan               *maintenancePlan     `json:"plan,omitempty"`
	PendingOrigin      *string              `json:"pendingOrigin,omitempty"`
	CandidatePayload   *string              `json:"candidatePayload,omitempty"`
	Lease              *maintenanceLease    `json:"lease,omitempty"`
}
type maintenanceReport struct {
	Version                 string `json:"version"`
	Status                  string `json:"status"`
	Operation               string `json:"operation"`
	Code                    string `json:"code"`
	Phase                   string `json:"phase,omitempty"`
	NoHostActionAuthority   bool   `json:"noHostActionAuthority"`
	Installation            string `json:"installation"`
	Coverage                string `json:"coverage"`
	StateWritesAttempted    int    `json:"stateWritesAttempted"`
	StateWritesCommitted    int    `json:"stateWritesCommitted"`
	NativeStampAttempts     int    `json:"nativeStampAttempts"`
	NativeStampInvoked      int    `json:"nativeStampInvoked"`
	NativeStampCompleted    int    `json:"nativeStampCompleted"`
	NativeApprovalAttempts  int    `json:"nativeApprovalAttempts"`
	NativeApprovalInvoked   int    `json:"nativeApprovalInvoked"`
	NativeApprovalCompleted int    `json:"nativeApprovalCompleted"`
	NativeSignatureVerified bool   `json:"nativeSignatureVerified"`
	StoredSignatureVerified bool   `json:"storedSignatureVerified"`
}
