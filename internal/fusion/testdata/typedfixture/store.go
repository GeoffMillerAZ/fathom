package typedfixture

// Store mirrors the ADR-021 §3 Store-contract-suite residue case: a
// PutClaims method implemented by more than one backend, exercised through
// one shared contract-suite-style caller (RunContract) — the shape
// internal/store's own contract_test.go uses against SQLiteStore/DuckStore/
// MemoryStore in the real codebase.
type Store interface {
	PutClaims(claim string) error
}

// MemStore is the first concrete implementer.
type MemStore struct{ claims []string }

// PutClaims implements Store.
func (m *MemStore) PutClaims(claim string) error {
	m.claims = append(m.claims, claim)
	return nil
}

// DiskStore is the second concrete implementer.
type DiskStore struct{ path string }

// PutClaims implements Store.
func (d *DiskStore) PutClaims(claim string) error {
	return nil
}

// RunContract calls PutClaims through the interface — the contract-suite
// dispatch call site.
func RunContract(s Store) error {
	return s.PutClaims("x")
}
