// Package accessbroker provides generic credential negotiation for agentic
// workloads. It is deliberately org-agnostic: no hard-coded providers, org
// names, or backend-specific logic. Providers and policies are supplied as
// data/config at construction time.
//
// CUE is authoritative for the shape of CredentialScope; Go is authoritative
// for Broker.Grant behaviour.
//
// Concurrency contract: Broker is immutable after construction (NewBroker) and
// safe for concurrent use; Grant holds no mutable state and delegates entirely
// to the caller-supplied PolicyFunc, which must itself be safe for concurrent
// use if Broker is shared across goroutines.
package accessbroker

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Level is the access level requested or granted for a resource.
// The valid values mirror #Level in schema.cue.
type Level string

const (
	// LevelRead permits read-only access to the resource.
	LevelRead Level = "read"

	// LevelWrite permits read and write access to the resource.
	LevelWrite Level = "write"

	// LevelAdmin permits full administrative access to the resource.
	LevelAdmin Level = "admin"
)

var validLevels = map[Level]struct{}{
	LevelRead:  {},
	LevelWrite: {},
	LevelAdmin: {},
}

// CredentialScope describes a resource and the desired access level.
// It mirrors #CredentialScope in schema.cue.
type CredentialScope struct {
	// Resource is the logical identifier of the resource being accessed.
	// Must be non-empty (e.g. "db/analytics", "bucket/reports").
	Resource string `json:"Resource"`

	// Level is the access level requested. Must be one of read / write / admin.
	Level Level `json:"Level"`
}

// Validate enforces the same constraints that #CredentialScope encodes in
// schema.cue. It returns a non-nil error when any constraint is violated.
func (s CredentialScope) Validate() error {
	var errs []string

	if strings.TrimSpace(s.Resource) == "" {
		errs = append(errs, "Resource must be non-empty")
	}
	if _, ok := validLevels[s.Level]; !ok {
		errs = append(errs, fmt.Sprintf(
			"Level %q is not valid (must be one of: read, write, admin)",
			s.Level,
		))
	}

	if len(errs) > 0 {
		return errors.New("CredentialScope: " + strings.Join(errs, "; "))
	}
	return nil
}

// GrantRequest carries a CredentialScope alongside caller context that the
// policy may inspect when deciding whether to grant access.
type GrantRequest struct {
	// Scope is the resource and level being requested.
	Scope CredentialScope

	// CallerID is an opaque, non-empty identifier for the requesting agent or
	// component (e.g. "agent/recon-01"). Used by policy functions.
	CallerID string
}

// Grant is the successful outcome of a Broker.Grant call. It is a transient
// token; callers should treat it as ephemeral and not store it durably.
type Grant struct {
	// Scope is the scope that was granted (mirrors the request scope).
	Scope CredentialScope

	// Token is the opaque credential produced by the policy function. Its
	// encoding is defined by the provider, not by this package.
	Token string

	// ExpiresAt is when the grant expires. Zero means no expiry.
	ExpiresAt time.Time
}

// PolicyFunc is a function that a Broker calls to obtain a credential token
// for a given scope and caller. It returns the token string and an optional
// expiry time. Returning an error causes Broker.Grant to propagate that error.
//
// Policy functions must be deterministic for a given (scope, callerID) pair at
// a given wall-clock instant. They must not hard-code org-specific identifiers.
type PolicyFunc func(scope CredentialScope, callerID string) (token string, expiresAt time.Time, err error)

// Broker performs credential negotiation by delegating to a configurable
// PolicyFunc. It validates inputs before calling the function, so the function
// can assume all arguments are well-formed.
type Broker struct {
	policy PolicyFunc
}

// NewBroker constructs a Broker with the given policy function.
// policy must be non-nil.
func NewBroker(policy PolicyFunc) (*Broker, error) {
	if policy == nil {
		return nil, errors.New("NewBroker: policy must be non-nil")
	}
	return &Broker{policy: policy}, nil
}

// Grant evaluates the GrantRequest against the configured policy and returns a
// Grant on success. It returns an error if:
//   - req.Scope fails validation,
//   - req.CallerID is empty, or
//   - the policy function returns an error.
func (b *Broker) Grant(req GrantRequest) (Grant, error) {
	if err := req.Scope.Validate(); err != nil {
		return Grant{}, fmt.Errorf("Grant: invalid scope: %w", err)
	}
	if strings.TrimSpace(req.CallerID) == "" {
		return Grant{}, errors.New("Grant: CallerID must be non-empty")
	}

	token, expiresAt, err := b.policy(req.Scope, req.CallerID)
	if err != nil {
		return Grant{}, fmt.Errorf("Grant: policy error: %w", err)
	}

	return Grant{
		Scope:     req.Scope,
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}
