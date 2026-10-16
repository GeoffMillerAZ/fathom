package tokenization

import (
	"errors"
	"fmt"
	"sync"
	"time"

	aiprovenance "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
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

// EncryptedVault encrypts values under per-namespace AES-256-GCM keys.
// Durable reconstruction requires the same tokenizer HMAC key, RecordStore and
// KeyStore, including durable shred tombstones. AuditLog is local to this instance.
// Concurrency: methods are serialized via a mutex. Shared stores must themselves
// be concurrent-safe. Shred cannot recall plaintext or keys already returned to
// another instance; callers requiring that barrier must coordinate those users.
type EncryptedVault struct {
	mu       sync.RWMutex
	tz       *Tokenizer
	ks       KeyStore
	policy   AccessPolicy
	clock    func() time.Time
	records  RecordStore
	random   RandFunc
	auditLog []AuditEvent
}

// NewEncryptedVault constructs a vault with an in-memory encrypted record store
// and crypto/rand nonces. Existing callers retain random token minting.
func NewEncryptedVault(tz *Tokenizer, ks KeyStore, policy AccessPolicy, clock func() time.Time) *EncryptedVault {
	return NewEncryptedVaultWithStore(tz, ks, policy, clock, NewMemoryRecordStore(), nil)
}

// NewEncryptedVaultWithStore constructs a vault over caller-owned records and
// keys. Neither store is copied or owned by the vault. Both must survive restart
// for resolution; stable deterministic minting also requires the same HMAC key.
// random supplies encryption nonces; nil defaults to crypto/rand. Production
// sources must supply fresh cryptographic randomness, never seeded test bytes.
// The tokenizer owns its independent random-token source. All other args are required.
func NewEncryptedVaultWithStore(tz *Tokenizer, ks KeyStore, policy AccessPolicy, clock func() time.Time, records RecordStore, random RandFunc) *EncryptedVault {
	if tz == nil || ks == nil || policy == nil || clock == nil || records == nil {
		panic("tokenization: NewEncryptedVaultWithStore: required dependency is nil")
	}
	if random == nil {
		random = cryptoRandom
	}
	return &EncryptedVault{tz: tz, ks: ks, policy: policy, clock: clock, records: records, random: random}
}

// Tokenize encrypts a value under a fresh random token. Duplicate token values
// fail with ErrTokenCollision, without overwriting a record. Every call is audited.
func (v *EncryptedVault) Tokenize(value string, class TokenClass, namespace string, by aiprovenance.Author) (Token, error) {
	return v.tokenize(value, class, namespace, by, false)
}

// TokenizeDeterministic encrypts value under its HMAC-derived token. Existing
// identical records are authenticated and reused without a second audit event.
// Collision, invalid record and shredded-key failures are audited and fail closed.
// Deterministic tokens leak equality; use a secret non-empty tokenizer HMAC key.
// Rotation cannot overwrite old records; choose a new namespace for a new generation.
func (v *EncryptedVault) TokenizeDeterministic(value string, class TokenClass, namespace string, by aiprovenance.Author) (Token, error) {
	return v.tokenize(value, class, namespace, by, true)
}

func (v *EncryptedVault) tokenize(value string, class TokenClass, namespace string, by aiprovenance.Author, deterministic bool) (tok Token, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	audit := true
	defer func() {
		if audit {
			v.auditLog = append(v.auditLog, AuditEvent{At: v.clock(), Action: ActionTokenize, TokenValue: tok.Value, Class: class, PrincipalID: principalIDFromAuthor(by), Allowed: err == nil})
		}
		if err != nil {
			tok = Token{}
		}
	}()
	if deterministic {
		if len(v.tz.hmacKey) == 0 {
			return Token{}, fmt.Errorf("tokenization: encrypted deterministic tokens require a secret HMAC key")
		}
		tok = v.tz.DeterministicToken(value, class, namespace)
		err = tok.Validate()
	} else {
		tok, err = v.tz.RandomToken(class, namespace)
	}
	if err != nil {
		return tok, err
	}
	existing, found, err := v.records.Get(tok.Value)
	if err != nil {
		return tok, fmt.Errorf("tokenization: record lookup: %w", err)
	}
	if found {
		if !deterministic {
			return tok, ErrTokenCollision
		}
		err = v.matchRecord(existing, tok, value)
		audit = err != nil
		return tok, err
	}
	key, err := v.ks.Key(namespace)
	if err != nil {
		return tok, fmt.Errorf("tokenization: key: %w", err)
	}
	defer clear(key)
	record := EncryptedRecord{Version: EncryptedRecordVersion, Token: tok}
	record.Ciphertext, err = seal(key, []byte(value), record.aad(), v.random)
	if err != nil {
		return tok, err
	}
	actual, inserted, err := v.records.PutIfAbsent(record.Clone())
	if err != nil {
		return tok, fmt.Errorf("tokenization: record put: %w", err)
	}
	if !inserted {
		if !deterministic {
			return tok, ErrTokenCollision
		}
		err = v.matchRecord(actual, tok, value)
		audit = err != nil
	}
	return tok, err
}

func (v *EncryptedVault) matchRecord(record EncryptedRecord, tok Token, value string) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if record.Token != tok {
		return ErrTokenCollision
	}
	plaintext, err := v.decrypt(record)
	if err != nil {
		return err
	}
	defer clear(plaintext)
	if string(plaintext) != value {
		return ErrTokenCollision
	}
	return nil
}

func (v *EncryptedVault) decrypt(record EncryptedRecord) ([]byte, error) {
	key, err := v.ks.Key(record.Token.Namespace)
	if err != nil {
		return nil, fmt.Errorf("tokenization: key: %w", err)
	}
	defer clear(key)
	return open(key, record.Ciphertext, record.aad())
}

// Detokenize validates the stored identity and enforces its class policy before
// decryption. Empty namespace from Parse is accepted; a supplied namespace must
// match. Every failure is audited as denied. ErrShredded remains detectable via
// errors.Is, but policy denial takes precedence over key access.
func (v *EncryptedVault) Detokenize(t Token, principal Principal, purpose string) (value string, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	ae := AuditEvent{At: v.clock(), Action: ActionDetokenize, TokenValue: t.Value, Class: t.Class, PrincipalID: principal.ID, Purpose: purpose}
	defer func() { ae.Allowed = err == nil; v.auditLog = append(v.auditLog, ae) }()
	record, found, err := v.records.Get(t.Value)
	if err != nil {
		return "", fmt.Errorf("tokenization: record lookup: %w", err)
	}
	if !found {
		return "", fmt.Errorf("tokenization: unknown token")
	}
	if err := record.Validate(); err != nil {
		return "", err
	}
	ae.Class = record.Token.Class
	if !sameIdentity(t, record.Token) {
		return "", fmt.Errorf("tokenization: token identity mismatch")
	}
	if !v.policy.Allow(principal, record.Token.Class, purpose) {
		return "", fmt.Errorf("tokenization: access denied")
	}
	plaintext, err := v.decrypt(record)
	if err != nil {
		return "", err
	}
	defer clear(plaintext)
	return string(plaintext), nil
}

// AuditLog returns a defensive copy of this instance's audit log in append order.
func (v *EncryptedVault) AuditLog() []AuditEvent {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return append([]AuditEvent{}, v.auditLog...)
}

// Shred destroys the namespace key, preserving encrypted records. Calls through
// this instance are serialized; subsequent resolutions fail with ErrShredded.
// Durable stores must persist tombstones before success. Other instances may have
// an in-flight key copy; Shred does not revoke copies already returned by Key.
func (v *EncryptedVault) Shred(namespace string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.ks.Shred(namespace); err != nil {
		return fmt.Errorf("tokenization: shred: %w", err)
	}
	return nil
}

func sameIdentity(requested, stored Token) bool {
	return requested.Value == stored.Value && requested.Class == stored.Class &&
		(requested.Namespace == "" || requested.Namespace == stored.Namespace)
}
