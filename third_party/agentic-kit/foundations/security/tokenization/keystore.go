package tokenization

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"sync"
)

// KeyStore is the seam for per-namespace envelope key management. Implementations
// determine where envelope keys are persisted and how key generations are tracked.
// The reference implementation is MemoryKeyStore; production code should use a
// durable adapter (e.g. backed by a KMS or encrypted file) that satisfies this
// interface.
//
// Key store implementations must be safe for concurrent use.
type KeyStore interface {
	// Key returns the current envelope key for namespace. If no key exists yet,
	// the implementation must mint a fresh random key and store it before
	// returning. The returned slice must not be shared; callers may not mutate it
	// (implementations return defensive copies or freshly derived bytes).
	//
	// Returns ErrShredded if the namespace has been shredded (key destroyed).
	Key(namespace string) ([]byte, error)

	// Rotate replaces the envelope key for namespace with a fresh random key and
	// increments the key generation counter. After Rotate, previously encrypted
	// ciphertexts are unrecoverable (the old key is gone). Use Rotate for
	// deliberate crypto-erasure.
	//
	// Rotate is idempotent with respect to shred: calling Rotate after Shred
	// makes the namespace active again under a new key.
	//
	// Returns ErrShredded if called on an already-shredded namespace — call
	// Rotate explicitly if you intend to re-activate after shred.
	Rotate(namespace string) error

	// Shred destroys the envelope key for namespace. After Shred, all ciphertexts
	// encrypted under any previous key for that namespace become permanently
	// unrecoverable — crypto-erasure. Key(namespace) returns ErrShredded until
	// Rotate is called.
	//
	// Shred is idempotent: calling Shred on an already-shredded namespace is not
	// an error.
	Shred(namespace string) error
}

// MemoryKeyStore is an in-memory KeyStore that generates, stores, and shreds
// 256-bit AES envelope keys per namespace. It is NOT suitable for production
// use: keys are lost on process exit and live in unprotected heap memory.
// Production code must use a durable adapter behind the KeyStore interface.
//
// MemoryKeyStore is safe for concurrent use via sync.RWMutex.
type MemoryKeyStore struct {
	mu   sync.RWMutex
	keys map[string][]byte // namespace → 32-byte AES-256 key; nil entry = shredded
	shrd map[string]bool   // set of shredded namespaces
}

// NewMemoryKeyStore returns an empty MemoryKeyStore.
func NewMemoryKeyStore() *MemoryKeyStore {
	return &MemoryKeyStore{
		keys: make(map[string][]byte),
		shrd: make(map[string]bool),
	}
}

// Key returns the current envelope key for namespace, minting a fresh key if
// none exists yet. Returns ErrShredded if the namespace has been shredded.
func (s *MemoryKeyStore) Key(namespace string) ([]byte, error) {
	s.mu.RLock()
	shredded := s.shrd[namespace]
	key := s.keys[namespace]
	s.mu.RUnlock()

	if shredded {
		return nil, fmt.Errorf("tokenization: %w: namespace %q", ErrShredded, namespace)
	}
	if key != nil {
		// Defensive copy.
		out := make([]byte, len(key))
		copy(out, key)
		return out, nil
	}

	// Mint a new key under the write lock (double-checked idiom).
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shrd[namespace] {
		return nil, fmt.Errorf("tokenization: %w: namespace %q", ErrShredded, namespace)
	}
	if k := s.keys[namespace]; k != nil {
		out := make([]byte, len(k))
		copy(out, k)
		return out, nil
	}
	k, err := generateKey()
	if err != nil {
		return nil, fmt.Errorf("tokenization: MemoryKeyStore: generate key: %w", err)
	}
	s.keys[namespace] = k
	out := make([]byte, len(k))
	copy(out, k)
	return out, nil
}

// Rotate replaces the envelope key for namespace with a fresh random key.
// If the namespace was previously shredded, Rotate re-activates it with a new
// key generation (previous ciphertexts remain unrecoverable). Rotate on an
// active namespace destroys the old key and replaces it.
func (s *MemoryKeyStore) Rotate(namespace string) error {
	k, err := generateKey()
	if err != nil {
		return fmt.Errorf("tokenization: MemoryKeyStore: rotate: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[namespace] = k
	delete(s.shrd, namespace)
	return nil
}

// Shred destroys the envelope key for namespace. All values encrypted under the
// destroyed key become permanently unrecoverable ciphertext. The token→ciphertext
// entries remain in the vault; they are not deleted (the append-only log is
// intact) — only the decryption ability is removed. Shred is idempotent.
func (s *MemoryKeyStore) Shred(namespace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Zero out the key bytes before releasing the slice (defense-in-depth).
	if k := s.keys[namespace]; k != nil {
		for i := range k {
			k[i] = 0
		}
	}
	delete(s.keys, namespace)
	s.shrd[namespace] = true
	return nil
}

// generateKey produces a fresh 256-bit AES key from crypto/rand. It uses the
// package-level rand.Reader directly because this is a seam implementation:
// the production path (MemoryKeyStore) intentionally depends on real entropy.
// Injected randomness is not appropriate here — key generation requires true
// randomness, not test-reproducible bytes.
func generateKey() ([]byte, error) {
	k := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil, fmt.Errorf("read random: %w", err)
	}
	return k, nil
}

// seal encrypts plaintext under key using AES-256-GCM with a random nonce.
// The nonce is prepended to the ciphertext: [12-byte nonce || GCM ciphertext].
// key must be exactly 32 bytes (AES-256).
func seal(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, plaintext, nil)
	return ct, nil
}

// open decrypts ciphertext produced by seal. The ciphertext must begin with the
// nonce (first NonceSize bytes) followed by GCM ciphertext+tag.
func open(key []byte, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:ns], ciphertext[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plain, nil
}
