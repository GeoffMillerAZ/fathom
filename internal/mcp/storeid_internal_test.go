package mcp

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var testStoreSeq atomic.Int64

// testStore gives a fake a store identity no other fake ever shares (an
// address can be reused after a fake is freed; a sequence number cannot),
// so it takes part in the graph store's caching (RT-74).
type testStore struct {
	once sync.Once
	id   string
}

func (s *testStore) StoreID() string {
	s.once.Do(func() { s.id = fmt.Sprintf("test-store-%d", testStoreSeq.Add(1)) })
	return s.id
}
