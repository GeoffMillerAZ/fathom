package tokenization_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/security/tokenization"
)

func recordVault(ks tokenization.KeyStore, records tokenization.RecordStore) *tokenization.EncryptedVault {
	clock := fixedClock(time.Unix(123, 0))
	random := counterRandom()
	tz := tokenization.NewTokenizer([]byte("fixture-hmac"), clock, random)
	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{"admin": {tokenization.ClassPII, tokenization.ClassOther}})
	return tokenization.NewEncryptedVaultWithStore(tz, ks, policy, clock, records, random)
}

func counterRandom() tokenization.RandFunc {
	var mu sync.Mutex
	var count byte
	return func(n int) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		count++
		return bytes.Repeat([]byte{count}, n), nil
	}
}

// snapshotKeys is a test adapter reconstructed from caller-owned serialized key
// state. Unlike a fresh MemoryKeyStore, it never silently regenerates missing keys.
type snapshotKeys struct {
	KeyBytes []byte
	Revoked  bool
}

func (s *snapshotKeys) Key(string) ([]byte, error) {
	if s.Revoked {
		return nil, tokenization.ErrShredded
	}
	return append([]byte(nil), s.KeyBytes...), nil
}
func (s *snapshotKeys) Rotate(string) error { return errors.New("unsupported by fixture") }
func (s *snapshotKeys) Shred(string) error  { s.Revoked = true; clear(s.KeyBytes); return nil }

func TestEncryptedRecordReconstruction(t *testing.T) {
	ks := tokenization.NewMemoryKeyStoreWithRandom(counterRandom())
	records := tokenization.NewMemoryRecordStore()
	v := recordVault(ks, records)
	tok, err := v.TokenizeDeterministic("durable secret", tokenization.ClassPII, "namespace", encVaultAuthor)
	if err != nil {
		t.Fatal(err)
	}
	r, _, _ := records.Get(tok.Value)
	key, _ := ks.Key(tok.Namespace)
	state := struct {
		Keys   snapshotKeys
		Record tokenization.EncryptedRecord
	}{snapshotKeys{KeyBytes: key}, r}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("durable secret")) {
		t.Fatal("plaintext escaped into persistent state")
	}
	path := filepath.Join(t.TempDir(), "caller-owned.json")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	for _, revoked := range []bool{false, true} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var restored struct {
			Keys   snapshotKeys
			Record tokenization.EncryptedRecord
		}
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		if revoked {
			if err := restored.Keys.Shred(tok.Namespace); err != nil {
				t.Fatal(err)
			}
		}
		// Serialize the tombstone too: another key-store object must keep denial.
		keyData, _ := json.Marshal(restored.Keys)
		var keys snapshotKeys
		if err := json.Unmarshal(keyData, &keys); err != nil {
			t.Fatal(err)
		}
		rs := tokenization.NewMemoryRecordStore()
		if _, _, err := rs.PutIfAbsent(restored.Record); err != nil {
			t.Fatal(err)
		}
		restarted := recordVault(&keys, rs)
		got, err := restarted.TokenizeDeterministic("durable secret", tok.Class, tok.Namespace, encVaultAuthor)
		if revoked {
			if !errors.Is(err, tokenization.ErrShredded) {
				t.Fatalf("restart mint after shred: %v", err)
			}
			if value, err := restarted.Detokenize(tok, encAdminPrincipal, "test"); value != "" || !errors.Is(err, tokenization.ErrShredded) {
				t.Fatalf("restart resolve after shred: %q %v", value, err)
			}
			continue
		}
		if err != nil || got != tok {
			t.Fatalf("restart mint: %+v %v", got, err)
		}
		if len(restarted.AuditLog()) != 0 {
			t.Fatal("idempotent reuse appended audit event")
		}
		if value, err := restarted.Detokenize(tok, encAdminPrincipal, "test"); err != nil || value != "durable secret" {
			t.Fatalf("restart resolution: %q %v", value, err)
		}
	}
	random1, err := v.Tokenize("durable secret", tok.Class, tok.Namespace, encVaultAuthor)
	if err != nil {
		t.Fatal(err)
	}
	random2, err := v.Tokenize("durable secret", tok.Class, tok.Namespace, encVaultAuthor)
	if err != nil || random1.Value == random2.Value || random1.Value == tok.Value {
		t.Fatalf("random mint did not stay random: %v", err)
	}
	for _, other := range []struct {
		class tokenization.TokenClass
		ns    string
	}{{tokenization.ClassOther, tok.Namespace}, {tok.Class, "other"}} {
		got, err := v.TokenizeDeterministic("durable secret", other.class, other.ns, encVaultAuthor)
		if err != nil || got.Value == tok.Value {
			t.Fatalf("identity isolation: %v", err)
		}
	}
}

type replacedRecordStore struct {
	tokenization.RecordStore
	record tokenization.EncryptedRecord
	getErr error
	putErr error
}

func (s replacedRecordStore) Get(string) (tokenization.EncryptedRecord, bool, error) {
	return s.record.Clone(), s.record.Token.Value != "", s.getErr
}
func (s replacedRecordStore) PutIfAbsent(r tokenization.EncryptedRecord) (tokenization.EncryptedRecord, bool, error) {
	if s.putErr != nil {
		return tokenization.EncryptedRecord{}, false, s.putErr
	}
	return s.RecordStore.PutIfAbsent(r)
}

func TestEncryptedRecordTamperAndInvalidState(t *testing.T) {
	ks := tokenization.NewMemoryKeyStoreWithRandom(counterRandom())
	rs := tokenization.NewMemoryRecordStore()
	v := recordVault(ks, rs)
	tok, err := v.TokenizeDeterministic("protected", tokenization.ClassPII, "ns", encVaultAuthor)
	if err != nil {
		t.Fatal(err)
	}
	record, _, _ := rs.Get(tok.Value)
	for _, tc := range []struct {
		name   string
		mutate func(*tokenization.EncryptedRecord)
	}{
		{"version", func(r *tokenization.EncryptedRecord) { r.Version++ }},
		{"ciphertext", func(r *tokenization.EncryptedRecord) { r.Ciphertext[13] ^= 1 }},
		{"nonce", func(r *tokenization.EncryptedRecord) { r.Ciphertext[0] ^= 1 }},
		{"namespace", func(r *tokenization.EncryptedRecord) { r.Token.Namespace = "elsewhere" }},
		{"class", func(r *tokenization.EncryptedRecord) { r.Token.Class = tokenization.ClassOther }},
		{"value", func(r *tokenization.EncryptedRecord) {
			r.Token.Value = "tok_pii_" + string(bytes.Repeat([]byte{'0'}, 64))
		}},
		{"short", func(r *tokenization.EncryptedRecord) { r.Ciphertext = r.Ciphertext[:10] }},
		{"empty namespace", func(r *tokenization.EncryptedRecord) { r.Token.Namespace = "" }},
		{"bad digest", func(r *tokenization.EncryptedRecord) { r.Token.Value = "tok_pii_invalid" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := record.Clone()
			tc.mutate(&r)
			// Same key for every namespace proves AAD detects namespace substitution.
			key, _ := ks.Key("ns")
			bad := recordVault(&snapshotKeys{KeyBytes: key}, replacedRecordStore{record: r})
			request := r.Token
			if value, err := bad.Detokenize(request, encAdminPrincipal, "test"); err == nil || value != "" {
				t.Fatalf("tampered record resolved: %q, %v", value, err)
			}
			if _, err := bad.TokenizeDeterministic("protected", tok.Class, tok.Namespace, encVaultAuthor); err == nil {
				t.Fatal("tampered record reused")
			}
			for _, a := range bad.AuditLog() {
				if a.Allowed {
					t.Fatal("failed operation audited allowed")
				}
			}
		})
	}
	for _, n := range []int{0, 16, 24, 31, 33} {
		bad := recordVault(&snapshotKeys{KeyBytes: make([]byte, n)}, rs)
		if _, err := bad.Tokenize("new", tok.Class, tok.Namespace, encVaultAuthor); err == nil {
			t.Fatalf("accepted %d-byte key", n)
		}
		if _, err := bad.Detokenize(tok, encAdminPrincipal, "test"); err == nil {
			t.Fatalf("resolved with %d-byte key", n)
		}
	}
	for _, bad := range []replacedRecordStore{{getErr: errors.New("offline")}, {RecordStore: rs, putErr: errors.New("write failed")}} {
		v := recordVault(ks, bad)
		if _, err := v.TokenizeDeterministic("new", tok.Class, tok.Namespace, encVaultAuthor); err == nil {
			t.Fatal("record-store error ignored")
		}
		if v.AuditLog()[0].Allowed {
			t.Fatal("storage error audited allowed")
		}
	}
}

func TestRecordStoreAtomicCopiesAndCollisions(t *testing.T) {
	ks := tokenization.NewMemoryKeyStoreWithRandom(counterRandom())
	rs := tokenization.NewMemoryRecordStore()
	v := recordVault(ks, rs)
	tok, err := v.TokenizeDeterministic("first", tokenization.ClassPII, "ns", encVaultAuthor)
	if err != nil {
		t.Fatal(err)
	}
	r, _, _ := rs.Get(tok.Value)
	original := r.Clone()
	r.Ciphertext[0] ^= 1
	got, _, _ := rs.Get(tok.Value)
	if !bytes.Equal(got.Ciphertext, original.Ciphertext) {
		t.Fatal("Get aliased storage")
	}
	winner, inserted, err := rs.PutIfAbsent(r)
	if err != nil || inserted || !bytes.Equal(winner.Ciphertext, original.Ciphertext) {
		t.Fatal("PutIfAbsent overwrote binding")
	}
	winner.Ciphertext[0] ^= 1
	got, _, _ = rs.Get(tok.Value)
	if !bytes.Equal(got.Ciphertext, original.Ciphertext) {
		t.Fatal("PutIfAbsent result aliased storage")
	}
	newStore := tokenization.NewMemoryRecordStore()
	if _, _, err := newStore.PutIfAbsent(original); err != nil {
		t.Fatal(err)
	}
	original.Ciphertext[0] ^= 1
	got, _, _ = newStore.Get(tok.Value)
	if bytes.Equal(got.Ciphertext, original.Ciphertext) {
		t.Fatal("PutIfAbsent retained input slice")
	}
	clock := fixedClock(time.Unix(1, 0))
	fixed := func(n int) ([]byte, error) { return bytes.Repeat([]byte{7}, n), nil }
	tz := tokenization.NewTokenizer([]byte("fixture"), clock, fixed)
	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{"admin": {tokenization.ClassPII}})
	for _, v := range []tokenization.Vault{tokenization.NewEncryptedVault(tz, ks, policy, clock), tokenization.NewMemoryVault(tz, policy, clock)} {
		token, err := v.Tokenize("first", tokenization.ClassPII, "ns", encVaultAuthor)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.Tokenize("second", token.Class, token.Namespace, encVaultAuthor); !errors.Is(err, tokenization.ErrTokenCollision) {
			t.Fatalf("random collision: %v", err)
		}
		if value, err := v.Detokenize(token, encAdminPrincipal, "test"); err != nil || value != "first" {
			t.Fatalf("collision replaced plaintext: %q %v", value, err)
		}
	}
	var wg sync.WaitGroup
	for range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			concurrent := recordVault(ks, rs)
			if got, err := concurrent.TokenizeDeterministic("shared", tok.Class, tok.Namespace, encVaultAuthor); err != nil || got.Value == "" {
				t.Errorf("concurrent deterministic mint: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestKeyStoreConcurrentShredAndCopies(t *testing.T) {
	ks := tokenization.NewMemoryKeyStoreWithRandom(counterRandom())
	key, err := ks.Key("ns")
	if err != nil {
		t.Fatal(err)
	}
	key[0] ^= 1
	copyKey, _ := ks.Key("ns")
	if bytes.Equal(key, copyKey) {
		t.Fatal("key slice alias")
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_, _ = ks.Key("ns")
				_ = ks.Rotate("ns")
				_ = ks.Shred("ns")
			}
		}()
	}
	wg.Wait()
	if err := ks.Shred("ns"); err != nil {
		t.Fatal(err)
	}
	if _, err := ks.Key("ns"); !errors.Is(err, tokenization.ErrShredded) {
		t.Fatalf("shred resurrected key: %v", err)
	}
}

func TestDeterministicCollisionAndRotationFailClosed(t *testing.T) {
	clock := fixedClock(time.Unix(1, 0))
	key := []byte("fixture-hmac")
	deterministic := tokenization.NewTokenizer(key, clock, counterRandom())
	wanted := deterministic.DeterministicToken("expected", tokenization.ClassPII, "ns")
	raw, err := hex.DecodeString(wanted.Value[len("tok_pii_"):])
	if err != nil {
		t.Fatal(err)
	}
	// Force a real authenticated ciphertext for a different plaintext into the
	// desired token slot, without modifying ciphertext or bypassing AAD.
	forced := tokenization.NewTokenizer(key, clock, func(int) ([]byte, error) { return append([]byte(nil), raw...), nil })
	ks := tokenization.NewMemoryKeyStoreWithRandom(counterRandom())
	rs := tokenization.NewMemoryRecordStore()
	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{"admin": {tokenization.ClassPII}})
	v := tokenization.NewEncryptedVaultWithStore(forced, ks, policy, clock, rs, counterRandom())
	tok, err := v.Tokenize("different", wanted.Class, wanted.Namespace, encVaultAuthor)
	if err != nil || tok != wanted {
		t.Fatalf("seed collision: %v", err)
	}
	restarted := recordVault(ks, rs)
	if _, err := restarted.TokenizeDeterministic("expected", tok.Class, tok.Namespace, encVaultAuthor); !errors.Is(err, tokenization.ErrTokenCollision) {
		t.Fatalf("plaintext collision: %v", err)
	}
	if got, err := restarted.Detokenize(tok, encAdminPrincipal, "test"); err != nil || got != "different" {
		t.Fatalf("collision overwrote record: %q %v", got, err)
	}
	if err := ks.Rotate(tok.Namespace); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.TokenizeDeterministic("expected", tok.Class, tok.Namespace, encVaultAuthor); err == nil {
		t.Fatal("rotation resurrected old binding")
	}
}

func TestEncryptionRandomAndInputFailures(t *testing.T) {
	clock := fixedClock(time.Unix(1, 0))
	policy := tokenization.NewRolePolicy(map[string][]tokenization.TokenClass{"admin": {tokenization.ClassPII}})
	for _, random := range []tokenization.RandFunc{
		func(int) ([]byte, error) { return nil, errors.New("entropy unavailable") },
		func(n int) ([]byte, error) { return make([]byte, n-1), nil },
	} {
		ks := tokenization.NewMemoryKeyStoreWithRandom(random)
		if _, err := ks.Key("ns"); err == nil {
			t.Fatal("invalid key entropy accepted")
		}
		if err := ks.Rotate("ns"); err == nil {
			t.Fatal("invalid rotation entropy accepted")
		}
		ks = tokenization.NewMemoryKeyStoreWithRandom(counterRandom())
		tz := tokenization.NewTokenizer([]byte("fixture"), clock, counterRandom())
		v := tokenization.NewEncryptedVaultWithStore(tz, ks, policy, clock, tokenization.NewMemoryRecordStore(), random)
		if _, err := v.TokenizeDeterministic("secret", tokenization.ClassPII, "ns", encVaultAuthor); err == nil {
			t.Fatal("invalid nonce entropy accepted")
		}
		if v.AuditLog()[0].Allowed {
			t.Fatal("nonce failure audited allowed")
		}
		tz = tokenization.NewTokenizer(nil, clock, random)
		if _, err := tz.RandomToken(tokenization.ClassPII, "ns"); err == nil {
			t.Fatal("invalid token entropy accepted")
		}
	}
	v := recordVault(tokenization.NewMemoryKeyStore(), tokenization.NewMemoryRecordStore())
	for _, tc := range []struct {
		class     tokenization.TokenClass
		namespace string
	}{{"invalid", "ns"}, {tokenization.ClassPII, ""}} {
		if _, err := v.TokenizeDeterministic("secret", tc.class, tc.namespace, encVaultAuthor); err == nil {
			t.Fatal("invalid deterministic identity accepted")
		}
	}
}

type racingRecordStore struct {
	tokenization.RecordStore
	ready sync.WaitGroup
}

func (s *racingRecordStore) Get(value string) (tokenization.EncryptedRecord, bool, error) {
	r, ok, err := s.RecordStore.Get(value)
	s.ready.Done()
	s.ready.Wait()
	return r, ok, err
}

func TestRecordStoreConcurrentPutWinner(t *testing.T) {
	const n = 12
	rs := &racingRecordStore{RecordStore: tokenization.NewMemoryRecordStore()}
	rs.ready.Add(n)
	ks := tokenization.NewMemoryKeyStoreWithRandom(counterRandom())
	var wg sync.WaitGroup
	wg.Add(n)
	results := make(chan *tokenization.EncryptedVault, n)
	for range n {
		go func() {
			defer wg.Done()
			v := recordVault(ks, rs)
			if _, err := v.TokenizeDeterministic("concurrent", tokenization.ClassPII, "ns", encVaultAuthor); err != nil {
				t.Errorf("concurrent insert: %v", err)
			}
			results <- v
		}()
	}
	wg.Wait()
	close(results)
	var writes int
	for v := range results {
		writes += len(v.AuditLog())
	}
	if writes != 1 {
		t.Fatalf("got %d committed tokenize events, want 1", writes)
	}
}
