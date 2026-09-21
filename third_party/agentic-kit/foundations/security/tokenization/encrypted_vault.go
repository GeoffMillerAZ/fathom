package tokenization

import (
	"errors"
	"fmt"
	"sync"
	"time"

	aiprovenance "github.com/geoffmilleraz/agentic-kit/foundations/provenance/ai-provenance"
)

// ErrShredded is returned by Detokenize when the namespace's envelope key has
// been shredded (crypto-erased). The token→ciphertext record still exists in
// the append-only log, but decryption is permanently impossible because the
// key is gone. Callers that need to distinguish a shredded namespace from an
// unknown token must use errors.Is(err, ErrShredded).
var ErrShredded = errors.New("tokenization: namespace shredded")

// ShreddableVault extends Vault with crypto-erasure. Shred destroys the
// namespace's envelope key so that all stored values in that namespace become
// permanently unrecoverable ciphertext — the append-only token log is intact,
// but the ability to decrypt is removed.
//
// Existing consumers of the Vault interface are unaffected: they operate on the
// Vault subset and need not be changed. Only callers that need to exercise
// crypto-erasure use ShreddableVault.
type ShreddableVault interface {
	Vault

	// Shred destroys the envelope key for namespace. After Shred, all
	// Detokenize calls on tokens from that namespace return ErrShredded.
	// Shred is idempotent: shredding an already-shredded namespace is not
	// an error. Tokens in OTHER namespaces are unaffected.
	//
	// The token→ciphertext log is preserved — crypto-erasure leaves the
	// append-only record intact while removing the ability to decrypt.
	//
	// What Shred does NOT give you:
	//   - It does NOT erase ciphertexts from the append-only store; the log entry
	//     remains. Only decryption ability is removed.
	//   - It does NOT protect against key compromise that occurred BEFORE the shred
	//     (an adversary who captured the key before Shred can still decrypt).
	//   - Forward secrecy of backups is the consumer's problem: a backup taken
	//     before the shred retains the key and can decrypt. Use short-lived keys
	//     and key-rotation windows to bound the exposure window.
	//   - It does NOT affect tokens in other namespaces; namespace isolation is
	//     complete — shredding namespace A cannot affect namespace B.
	Shred(namespace string) error
}

// encryptedEntry is the internal vault record storing a token and its encrypted
// value. The plaintext is never stored; only the AES-GCM ciphertext is kept.
type encryptedEntry struct {
	token      Token
	ciphertext []byte // AES-256-GCM: [nonce || ciphertext || tag]
}

// EncryptedVault is a Vault implementation that encrypts sensitive values at
// rest under per-namespace AES-256-GCM envelope keys managed by a KeyStore.
// It implements ShreddableVault: calling Shred destroys the namespace key via
// the KeyStore, making all values in that namespace permanently unrecoverable.
//
// Token minting (Tokenize) uses the Tokenizer's RandomToken — the HMAC key
// used by DeterministicToken is DISTINCT from the envelope key used to encrypt
// values. This separation means:
//   - Minting is not storage: DeterministicToken always produces the same token
//     for a (value, class, namespace) triple regardless of whether the namespace
//     has been shredded. The token can still be minted after a shred.
//   - Resolution fails typed: after a shred, Detokenize returns ErrShredded even
//     for tokens whose value was previously resolvable.
//
// Concurrency: EncryptedVault is safe for concurrent use. KeyStore must also be
// safe for concurrent use (MemoryKeyStore is).
type EncryptedVault struct {
	mu       sync.RWMutex
	tz       *Tokenizer
	ks       KeyStore
	policy   AccessPolicy
	clock    func() time.Time
	store    map[string]encryptedEntry // keyed by token.Value
	auditLog []AuditEvent
}

// NewEncryptedVault constructs an EncryptedVault. tz provides token generation,
// ks manages envelope keys (and shredding), policy enforces detokenize access
// control, and clock timestamps audit events.
func NewEncryptedVault(tz *Tokenizer, ks KeyStore, policy AccessPolicy, clock func() time.Time) *EncryptedVault {
	if tz == nil {
		panic("tokenization: NewEncryptedVault: tokenizer must not be nil")
	}
	if ks == nil {
		panic("tokenization: NewEncryptedVault: key store must not be nil")
	}
	if policy == nil {
		panic("tokenization: NewEncryptedVault: policy must not be nil")
	}
	if clock == nil {
		panic("tokenization: NewEncryptedVault: clock must not be nil")
	}
	return &EncryptedVault{
		tz:     tz,
		ks:     ks,
		policy: policy,
		clock:  clock,
		store:  make(map[string]encryptedEntry),
	}
}

// Tokenize encrypts value under the namespace's envelope key and stores the
// ciphertext alongside the token. Each call fetches (or lazily mints) the
// namespace key from the KeyStore. A tokenize AuditEvent is always appended.
//
// If the namespace has been shredded and the KeyStore returns ErrShredded,
// Tokenize returns an error — you cannot store new values in a shredded
// namespace without first calling Rotate on the KeyStore.
func (v *EncryptedVault) Tokenize(value string, class TokenClass, namespace string, by aiprovenance.Author) (Token, error) {
	tok, tokErr := v.tz.RandomToken(class, namespace)

	principalID := principalIDFromAuthor(by)

	// Attempt encryption only if token minting succeeded.
	var storeErr error
	if tokErr == nil {
		key, err := v.ks.Key(namespace)
		if err != nil {
			storeErr = fmt.Errorf("tokenization: EncryptedVault.Tokenize: key: %w", err)
		} else {
			ct, err := seal(key, []byte(value))
			if err != nil {
				storeErr = fmt.Errorf("tokenization: EncryptedVault.Tokenize: seal: %w", err)
			} else {
				v.mu.Lock()
				v.store[tok.Value] = encryptedEntry{token: tok, ciphertext: ct}
				v.mu.Unlock()
			}
		}
	}

	finalErr := tokErr
	if finalErr == nil {
		finalErr = storeErr
	}

	v.mu.Lock()
	v.auditLog = append(v.auditLog, AuditEvent{
		At:          v.clock(),
		Action:      ActionTokenize,
		TokenValue:  tok.Value,
		Class:       class,
		PrincipalID: principalID,
		Purpose:     "",
		Allowed:     finalErr == nil,
	})
	v.mu.Unlock()

	if finalErr != nil {
		return Token{}, finalErr
	}
	return tok, nil
}

// Detokenize retrieves and decrypts the original value for token t.
// Returns ErrShredded (via errors.Is) if the namespace has been shredded.
// Access policy is checked before decryption; a denied principal receives an
// access-denied error, not ErrShredded (policy takes precedence).
func (v *EncryptedVault) Detokenize(t Token, principal Principal, purpose string) (string, error) {
	allowed := v.policy.Allow(principal, t.Class, purpose)

	ae := AuditEvent{
		At:          v.clock(),
		Action:      ActionDetokenize,
		TokenValue:  t.Value,
		Class:       t.Class,
		PrincipalID: principal.ID,
		Purpose:     purpose,
		Allowed:     allowed,
	}

	if !allowed {
		v.mu.Lock()
		v.auditLog = append(v.auditLog, ae)
		v.mu.Unlock()
		return "", fmt.Errorf("tokenization: Detokenize: principal %q denied access to class %q", principal.ID, t.Class)
	}

	// Look up ciphertext.
	v.mu.RLock()
	e, ok := v.store[t.Value]
	v.mu.RUnlock()

	if !ok {
		ae.Allowed = false
		v.mu.Lock()
		v.auditLog = append(v.auditLog, ae)
		v.mu.Unlock()
		return "", fmt.Errorf("tokenization: Detokenize: unknown token %q", t.Value)
	}

	// Fetch the namespace key — this returns ErrShredded if shredded.
	key, err := v.ks.Key(e.token.Namespace)
	if err != nil {
		ae.Allowed = false
		v.mu.Lock()
		v.auditLog = append(v.auditLog, ae)
		v.mu.Unlock()
		// Wrap so callers can use errors.Is(err, ErrShredded).
		if errors.Is(err, ErrShredded) {
			return "", fmt.Errorf("tokenization: Detokenize: %w", ErrShredded)
		}
		return "", fmt.Errorf("tokenization: Detokenize: key: %w", err)
	}

	plaintext, err := open(key, e.ciphertext)
	if err != nil {
		ae.Allowed = false
		v.mu.Lock()
		v.auditLog = append(v.auditLog, ae)
		v.mu.Unlock()
		return "", fmt.Errorf("tokenization: Detokenize: decrypt: %w", err)
	}

	v.mu.Lock()
	v.auditLog = append(v.auditLog, ae)
	v.mu.Unlock()

	return string(plaintext), nil
}

// AuditLog returns a defensive copy of the audit log in append order.
func (v *EncryptedVault) AuditLog() []AuditEvent {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make([]AuditEvent, len(v.auditLog))
	copy(out, v.auditLog)
	return out
}

// Shred destroys the namespace's envelope key via the KeyStore. After Shred,
// all subsequent Detokenize calls for tokens in that namespace return
// ErrShredded. Tokens in other namespaces are entirely unaffected.
//
// Shred is idempotent: shredding an already-shredded namespace is not an error.
//
// See the ShreddableVault documentation for what Shred does NOT guarantee.
func (v *EncryptedVault) Shred(namespace string) error {
	if err := v.ks.Shred(namespace); err != nil {
		return fmt.Errorf("tokenization: EncryptedVault.Shred: %w", err)
	}
	return nil
}
