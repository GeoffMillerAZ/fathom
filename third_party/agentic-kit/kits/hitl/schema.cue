package hitl

// Approval/commit v1 persistence shapes. Go validates temporal ordering, record
// relationships, duplicate IDs, capacity and transactional transitions.
#Digest:     string & =~"^[0-9a-f]{64}$"
#DecisionID: string & =~"^[0-9a-f]{32}$"
#Binding: {
	Action:          string & !=""
	Resource:        string & !=""
	OperationDigest: #Digest
	ExpectedVersion: uint64 & >0
	EvidenceDigest:  #Digest
	ConfigDigest:    #Digest
}
#Decision: {
	ID:        #DecisionID
	Actor:     string & !=""
	Binding:   #Binding
	IssuedAt:  string & !=""
	ExpiresAt: string & !=""
	Revoked:   bool
	Consumed:  bool
}
#CommitReceipt: {
	DecisionID:       #DecisionID
	Actor:            string & !=""
	Binding:          #Binding
	CommittedVersion: uint64 & >1
	Summary:          string
	At:               string & !=""
}
#Attempt: {
	ID:             #DecisionID
	DecisionID:     ID
	Actor:          string & !=""
	Binding:        #Binding
	IdempotencyKey: "hitl-" + ID
	Status:         "prepared" | "sending" | "succeeded" | "unknown" | "not-applied"
	Sends:          0 | 1
	Receipt:        string
	PreparedAt:     string & !=""
	UpdatedAt:      string & !=""
	if Status == "prepared" {Sends: 0}
	if Status != "prepared" {Sends: 1}
	if Status == "succeeded" {Receipt: !=""}
	if Status != "succeeded" {Receipt: ""}
}
#ResourceState: {
	Resource:       string & !=""
	Version:        uint64 & >0
	EvidenceDigest: #Digest
	ConfigDigest:   #Digest
	Value:          _ // Caller-owned JSON-compatible state.
}
#CommitSnapshot: {
	FormatVersion: 1
	Target:        #ResourceState
	Decisions: null | [...#Decision]
	Receipts: null | [...#CommitReceipt]
	Attempts: null | [...#Attempt]
}
