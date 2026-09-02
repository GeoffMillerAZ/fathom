package store

import "errors"

// Store is an in-memory store.
type Store struct {
	items map[string]string
}

// Open creates a store.
func Open() *Store {
	s := &Store{}
	s.items = map[string]string{}
	return s
}

// Get returns an item.
func (s *Store) Get(key string) (string, error) {
	v, ok := s.items[key]
	if !ok {
		return "", errors.New("missing")
	}
	return v, nil
}

// Put stores an item.
func (s *Store) Put(key, value string) {
	s.items[key] = value
}
