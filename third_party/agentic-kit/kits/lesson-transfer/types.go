package lessontransfer

import (
	"context"
	"errors"
	"time"
)

// Version is the only lesson payload format supported by this experimental kit.
const Version = "lesson-transfer/v1"

// ErrInvalid identifies malformed, oversized, unbound, or inapplicable lesson data.
var ErrInvalid = errors.New("lessontransfer: invalid lesson")

// ErrUntrusted identifies missing or rejecting receiver-owned acceptance evidence.
var ErrUntrusted = errors.New("lessontransfer: receiver acceptance unavailable")

// ErrRevoked identifies a locally or currently revoked/superseded lesson.
var ErrRevoked = errors.New("lessontransfer: lesson revoked or superseded")

// Origin describes asserted provenance. Its strings are not authenticated by this kit.
type Origin struct {
	Workload       string `json:"workload"`
	SnapshotDigest string `json:"snapshot_digest"`
	IncidentDigest string `json:"incident_digest"`
	ScenarioDigest string `json:"scenario_digest"`
}

// Applicability controls which receiver snapshots may reproduce a lesson.
type Applicability struct {
	Workloads     []string `json:"workloads"`
	ConfigDigests []string `json:"config_digests"`
}

// ReproducerRef binds the independently specified synthetic reproducer bytes.
type ReproducerRef struct {
	Schema    string `json:"schema"`
	Version   int    `json:"version"`
	File      string `json:"file"`
	RawDigest string `json:"raw_digest"`
	Expected  string `json:"expected"`
	MaxBytes  int    `json:"max_bytes"`
}

// SyntheticInput is the complete inert v1 reproducer input shape. It is not a
// general payload channel: v1 admits only named, public synthetic cases.
type SyntheticInput struct {
	Case string `json:"case"`
}

// Reproducer is the bounded synthetic data stored in reproducer.json. Input is
// a closed case selector and is never interpreted as a command or instruction.
type Reproducer struct {
	Schema    string         `json:"schema"`
	Version   int            `json:"version"`
	Input     SyntheticInput `json:"input"`
	Expected  string         `json:"expected"`
	Invariant string         `json:"invariant"`
}

// RemedyRef names a remedy without granting permission to apply its bytes.
type RemedyRef struct {
	ID         string `json:"id"`
	Revision   int    `json:"revision"`
	Digest     string `json:"digest"`
	TestDigest string `json:"test_digest"`
}

// Revocation declares the payload's currently packaged state. CurrentRevocation
// is still required at adoption because this declaration can be stale.
type Revocation struct {
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Recheck bool   `json:"recheck"`
}

// Payload is the exact contents of lesson.json. It intentionally excludes derived
// digests and attestations, avoiding self-referential hashing.
type Payload struct {
	Version        string        `json:"version"`
	ID             string        `json:"id"`
	Revision       int           `json:"revision"`
	Status         string        `json:"status"`
	Classification string        `json:"classification"`
	Origin         Origin        `json:"origin"`
	Applicability  Applicability `json:"applicability"`
	Exclusions     []string      `json:"exclusions"`
	Reproducer     ReproducerRef `json:"reproducer"`
	Remedies       []RemedyRef   `json:"remedies"`
	Supersedes     []string      `json:"supersedes"`
	Revocation     Revocation    `json:"revocation"`
}

// Attestations is descriptive metadata bound to immutable payload/reproducer bytes.
// Reviewer is never an authorization credential.
type Attestations struct {
	PayloadDigest    string `json:"payload_digest"`
	ReproducerDigest string `json:"reproducer_digest"`
	Reviewer         string `json:"reviewer"`
	ReviewedAt       string `json:"reviewed_at"`
	Scope            string `json:"scope"`
}

// Bundle is an owned validated lesson with its immutable byte bindings.
type Bundle struct {
	Payload          Payload
	Reproducer       Reproducer
	Attestations     Attestations
	PayloadDigest    string
	ReproducerDigest string
	payloadRaw       []byte
	reproducerRaw    []byte
	attestationsRaw  []byte
}

// ReceiverSnapshot identifies the local receiver and exact configuration used.
type ReceiverSnapshot struct {
	Workload     string `json:"workload"`
	Revision     string `json:"revision"`
	ConfigDigest string `json:"config_digest"`
}

// ReproductionOutcome is the receiver's independently computed result.
type ReproductionOutcome struct {
	Classification string `json:"classification"`
	Evidence       string `json:"evidence"`
	Detail         string `json:"detail"`
}

// RevocationStatus is an owned receiver observation, not a distributed guarantee.
type RevocationStatus struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// ReviewAcceptance is the receiver's trusted review/policy boundary.
type ReviewAcceptance func(context.Context, Bundle, ReceiverSnapshot) error

// CurrentReceiver returns the receiver snapshot observed at a check boundary.
type CurrentReceiver func(context.Context, ReceiverSnapshot) (ReceiverSnapshot, error)

// CurrentRevocation returns the receiver's current revocation observation.
type CurrentRevocation func(context.Context, Bundle) (RevocationStatus, error)

// Reproduce runs a receiver-owned reproducer. No executable command is represented
// in Bundle, so the receiver retains all execution authority.
type Reproduce func(context.Context, Bundle, ReceiverSnapshot) (ReproductionOutcome, error)

// AdoptionReceipt records one local check. Fresh is true only after final current
// receiver and revocation rechecks succeed.
type AdoptionReceipt struct {
	Version          string              `json:"version"`
	PayloadDigest    string              `json:"payload_digest"`
	ReproducerDigest string              `json:"reproducer_digest"`
	Receiver         ReceiverSnapshot    `json:"receiver"`
	Outcome          ReproductionOutcome `json:"outcome"`
	Integrity        string              `json:"integrity"`
	Reproduction     string              `json:"reproduction"`
	Fresh            bool                `json:"fresh"`
	CheckedAt        time.Time           `json:"checked_at"`
}
