package mcp_test

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var extStoreSeq atomic.Int64

// extTestStore is storeid_internal_test.go's testStore for mcp_test fakes.
type extTestStore struct {
	once sync.Once
	id   string
}

func (s *extTestStore) StoreID() string {
	s.once.Do(func() { s.id = fmt.Sprintf("ext-test-store-%d", extStoreSeq.Add(1)) })
	return s.id
}
