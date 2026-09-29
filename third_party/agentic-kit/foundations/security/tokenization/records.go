package tokenization

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// EncryptedRecordVersion identifies the AES-256-GCM record/AAD encoding.
const EncryptedRecordVersion = 1

// ErrTokenCollision means a token value is already bound and cannot be replaced.
var ErrTokenCollision = errors.New("tokenization: token collision")

// ErrRecordLimit means an in-memory record store cannot admit another record
// within its count/byte bounds. Existing bindings remain unchanged.
var ErrRecordLimit = errors.New("tokenization: record store capacity exceeded")

// EncryptedRecord is the portable ciphertext-only persistence contract. Version 1
// stores a 12-byte nonce followed by ciphertext and a 16-byte GCM authentication
// tag. Version and all Token fields are authenticated as additional data.
type EncryptedRecord struct {
	Version    int
	Token      Token
	Ciphertext []byte
}

// Validate rejects unsupported versions, malformed identities and short ciphertext.
// Cryptographic integrity is checked separately during resolution or reuse.
func (r EncryptedRecord) Validate() error {
	if r.Version != EncryptedRecordVersion {
		return fmt.Errorf("tokenization: unsupported encrypted record version")
	}
	if err := r.Token.Validate(); err != nil {
		return err
	}
	prefix := "tok_" + string(r.Token.Class) + "_"
	if !strings.HasPrefix(r.Token.Value, prefix) {
		return fmt.Errorf("tokenization: record token class mismatch")
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(r.Token.Value, prefix))
	if err != nil || len(digest) != 32 {
		return fmt.Errorf("tokenization: invalid record token digest")
	}
	if len(r.Ciphertext) < 28 {
		return fmt.Errorf("tokenization: record ciphertext too short")
	}
	return nil
}

// Clone returns a defensive copy, including ciphertext bytes.
func (r EncryptedRecord) Clone() EncryptedRecord {
	r.Ciphertext = append([]byte(nil), r.Ciphertext...)
	return r
}

func (r EncryptedRecord) aad() []byte {
	return []byte(fmt.Sprintf("tokenization-record\x00%d\x00%d\x00%s\x00%d\x00%s\x00%d\x00%s", r.Version, len(r.Token.Value), r.Token.Value, len(r.Token.Class), r.Token.Class, len(r.Token.Namespace), r.Token.Namespace))
}

// RecordStore is the caller-owned persistence port. Implementations must be safe
// for concurrent use, return defensive copies, and never overwrite a token value.
// Durable adapters acknowledge writes only after durability; keys and shred
// tombstones must also be durable. No plaintext or key material belongs here.
type RecordStore interface {
	// Get returns the record indexed by tokenValue, or found=false. Operational
	// failures return an error, never absence. Returned records must pass Validate.
	Get(tokenValue string) (record EncryptedRecord, found bool, err error)
	// PutIfAbsent atomically validates and inserts record if Token.Value is new.
	// It returns the winning record and inserted=true only for a new insertion.
	// A concurrent or existing binding is returned intact with inserted=false.
	PutIfAbsent(record EncryptedRecord) (actual EncryptedRecord, inserted bool, err error)
}

// MemoryRecordStore is the in-memory reference RecordStore. It is safe for
// concurrent use via a mutex, but loses records on process exit.
type MemoryRecordStore struct {
	mu         sync.RWMutex
	records    map[string]EncryptedRecord
	maxRecords int
	maxBytes   int
	bytes      int
}

// NewMemoryRecordStore returns an empty store bounded to 4096 records and 64 MiB
// of token metadata/ciphertext. At capacity new bindings return ErrRecordLimit;
// existing deterministic bindings are reusable. The zero value uses these bounds.
// Durable/custom adapters own their capacity and retention policy.
func NewMemoryRecordStore() *MemoryRecordStore {
	s, _ := NewMemoryRecordStoreWithLimits(4096, 64<<20)
	return s
}

// NewMemoryRecordStoreWithLimits constructs an explicitly bounded reference
// store. Both limits must be positive. Bytes include ciphertext and token metadata
// lengths; map/runtime overhead is not included.
func NewMemoryRecordStoreWithLimits(maxRecords, maxBytes int) (*MemoryRecordStore, error) {
	if maxRecords <= 0 || maxBytes <= 0 {
		return nil, fmt.Errorf("tokenization: record limits must be positive")
	}
	return &MemoryRecordStore{records: make(map[string]EncryptedRecord), maxRecords: maxRecords, maxBytes: maxBytes}, nil
}

// Get returns a defensive copy of a record, or found=false.
func (s *MemoryRecordStore) Get(tokenValue string) (EncryptedRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[tokenValue]
	return r.Clone(), ok, nil
}

// PutIfAbsent validates and atomically binds a token without overwriting it.
func (s *MemoryRecordStore) PutIfAbsent(record EncryptedRecord) (EncryptedRecord, bool, error) {
	if err := record.Validate(); err != nil {
		return EncryptedRecord{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.records[record.Token.Value]; ok {
		return r.Clone(), false, nil
	}
	if s.records == nil {
		s.records = make(map[string]EncryptedRecord)
	}
	if s.maxRecords == 0 {
		s.maxRecords, s.maxBytes = 4096, 64<<20
	}
	size := len(record.Ciphertext) + len(record.Token.Value) + len(record.Token.Namespace) + len(record.Token.Class)
	if len(s.records) >= s.maxRecords || size > s.maxBytes-s.bytes {
		return EncryptedRecord{}, false, ErrRecordLimit
	}
	s.records[record.Token.Value] = record.Clone()
	s.bytes += size
	return record.Clone(), true, nil
}
