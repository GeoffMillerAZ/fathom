package tokenization

import (
	"fmt"
	"sync"
	"time"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// Vault is the port that app adapters must implement to provide tokenization
// services. The reference implementation is MemoryVault; production code should
// use an adapter that encrypts values at rest.
type Vault interface {
	// Tokenize stores value in the vault and returns an opaque token. The
	// returned token is associated with class and namespace. by identifies the
	// human or AI author initiating the tokenization (for provenance). Every
	// call appends an AuditEvent regardless of outcome.
	Tokenize(value string, class TokenClass, namespace string, by aiprovenance.Author) (Token, error)

	// Detokenize retrieves the original value for token t. principal is the
	// actor requesting access and purpose is the stated reason. An error is
	// returned and an AuditEvent is appended if access is denied, the token is
	// unknown, or any other failure occurs.
	Detokenize(t Token, principal Principal, purpose string) (string, error)

	// AuditLog returns a defensive copy of every AuditEvent appended so far, in
	// append order. The caller may freely mutate the returned slice and events
	// without affecting the vault's internal state.
	AuditLog() []AuditEvent
}

// entry is the internal vault record associating a token with its plaintext.
type entry struct {
	token Token
	value string // the raw secret — never surfaced in logs
}

// MemoryVault is an in-memory reference implementation of the Vault interface.
// It stores tokens and plaintext values in a plain Go map. It is NOT suitable
// for production use: values are unencrypted in memory and are lost on process
// exit. Production code must use an app adapter that encrypts values at rest
// behind this Vault interface.
//
// MemoryVault is safe for concurrent use.
type MemoryVault struct {
	mu       sync.RWMutex
	tz       *Tokenizer
	policy   AccessPolicy
	clock    func() time.Time
	store    map[string]entry // keyed by token.Value
	auditLog []AuditEvent
}

// NewMemoryVault constructs an empty MemoryVault. tz provides token generation,
// policy enforces detokenize access control, and clock is the injected time
// source used to timestamp audit events.
func NewMemoryVault(tz *Tokenizer, policy AccessPolicy, clock func() time.Time) *MemoryVault {
	if tz == nil {
		panic("tokenization: NewMemoryVault: tokenizer must not be nil")
	}
	if policy == nil {
		panic("tokenization: NewMemoryVault: policy must not be nil")
	}
	if clock == nil {
		panic("tokenization: NewMemoryVault: clock must not be nil")
	}
	return &MemoryVault{
		tz:     tz,
		policy: policy,
		clock:  clock,
		store:  make(map[string]entry),
	}
}

// Tokenize stores value in the vault and returns an opaque token. The by
// Author determines the PrincipalID recorded in the audit event:
//   - For human authors: Author.Human.ID
//   - For AI authors: "ai:" + Author.AI.ModelID
//
// A tokenize AuditEvent is always appended, even on error.
func (v *MemoryVault) Tokenize(value string, class TokenClass, namespace string, by aiprovenance.Author) (Token, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	tok, err := v.tz.RandomToken(class, namespace)

	principalID := principalIDFromAuthor(by)

	if err == nil {
		if _, exists := v.store[tok.Value]; exists {
			err = ErrTokenCollision
		}
	}

	ae := AuditEvent{
		At:          v.clock(),
		Action:      ActionTokenize,
		TokenValue:  tok.Value,
		Class:       class,
		PrincipalID: principalID,
		Purpose:     "", // purpose is not applicable for tokenize
		Allowed:     err == nil,
	}
	v.auditLog = append(v.auditLog, ae)

	if err != nil {
		return Token{}, fmt.Errorf("tokenization: Tokenize: %w", err)
	}

	v.store[tok.Value] = entry{token: tok, value: value}
	return tok, nil
}

// TokenizeDeterministic stores value under a deterministic HMAC-derived token
// (same value+class+namespace always yields the same token value) and returns
// the token. The action is audited identically to Tokenize.
//
// If the triple (value, class, namespace) is already in the vault under the
// derived token, the existing entry is reused and the same token is returned
// without a second audit event — the call is idempotent. If the same derived
// token value is somehow already associated with a DIFFERENT plaintext (which
// is an HMAC collision — practically impossible with a keyed 256-bit digest),
// an error is returned.
//
// TokenizeDeterministic satisfies the llmegress.DeterministicVault interface,
// allowing GateFromVaultOpts(…, WithDeterministicTokens()) to produce
// value-stable per-namespace tokens from a MemoryVault.
func (v *MemoryVault) TokenizeDeterministic(value string, class TokenClass, namespace string, by aiprovenance.Author) (Token, error) {
	tok := v.tz.DeterministicToken(value, class, namespace)

	principalID := principalIDFromAuthor(by)

	v.mu.Lock()
	defer v.mu.Unlock()

	// Idempotency check: if this token value is already stored, confirm the
	// plaintext matches and return without a new audit event.
	if err := tok.Validate(); err != nil {
		v.auditLog = append(v.auditLog, AuditEvent{At: v.clock(), Action: ActionTokenize, TokenValue: tok.Value, Class: class, PrincipalID: principalID, Allowed: false})
		return Token{}, err
	}
	if existing, ok := v.store[tok.Value]; ok {
		if existing.value != value || existing.token != tok {
			v.auditLog = append(v.auditLog, AuditEvent{At: v.clock(), Action: ActionTokenize, TokenValue: tok.Value, Class: class, PrincipalID: principalID, Allowed: false})
			return Token{}, ErrTokenCollision
		}
		return tok, nil
	}

	ae := AuditEvent{
		At:          v.clock(),
		Action:      ActionTokenize,
		TokenValue:  tok.Value,
		Class:       class,
		PrincipalID: principalID,
		Purpose:     "",
		Allowed:     true,
	}
	v.auditLog = append(v.auditLog, ae)
	v.store[tok.Value] = entry{token: tok, value: value}
	return tok, nil
}

// Detokenize retrieves the original value for token t. Access is denied and an
// audit event is appended (with Allowed=false) if the policy rejects the
// principal, or if the token is unknown.
func (v *MemoryVault) Detokenize(t Token, principal Principal, purpose string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	e, ok := v.store[t.Value]
	allowed := ok && sameIdentity(t, e.token) && v.policy.Allow(principal, e.token.Class, purpose)

	ae := AuditEvent{
		At:          v.clock(),
		Action:      ActionDetokenize,
		TokenValue:  t.Value,
		Class:       t.Class,
		PrincipalID: principal.ID,
		Purpose:     purpose,
		Allowed:     allowed,
	}
	if ok {
		ae.Class = e.token.Class
	}
	v.auditLog = append(v.auditLog, ae)

	if !allowed {
		return "", fmt.Errorf("tokenization: Detokenize: principal %q denied access to class %q", principal.ID, t.Class)
	}

	return e.value, nil
}

// AuditLog returns a defensive copy of the audit log in append order.
// The caller may freely mutate the returned slice and its elements without
// affecting the vault's internal state.
func (v *MemoryVault) AuditLog() []AuditEvent {
	v.mu.RLock()
	defer v.mu.RUnlock()

	out := make([]AuditEvent, len(v.auditLog))
	copy(out, v.auditLog)
	return out
}

// principalIDFromAuthor derives a stable PrincipalID string from an
// aiprovenance.Author. For human authors this is Human.ID; for AI authors it
// is "ai:" + AI.ModelID.
func principalIDFromAuthor(a aiprovenance.Author) string {
	if a.Kind == aiprovenance.KindHuman && a.Human != nil {
		return a.Human.ID
	}
	if a.Kind == aiprovenance.KindAI && a.AI != nil {
		return "ai:" + a.AI.ModelID
	}
	return "unknown"
}
