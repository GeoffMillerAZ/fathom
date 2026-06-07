package store

import (
	"testing"
)

func TestMemoryStoreContract(t *testing.T) {
	s := NewMemoryStore()
	t.Cleanup(func() { _ = s.Close() })
	storeContract(t, s)
}

// TestMemoryStoreImplementsStore is a compile-time check that *MemoryStore satisfies
// the Store interface.
func TestMemoryStoreImplementsStore(t *testing.T) {
	var _ Store = (*MemoryStore)(nil)
}
