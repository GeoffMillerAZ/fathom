// Package svc holds the first-party types the served-call fixture's callers
// use (wave lg, LB-F199/LB-F200): each method's name is one a dependency
// also declares, so the syntax pass's unambiguous bare-name rule binds a
// dependency's call to it.
package svc

// Store is a first-party type with methods.
type Store struct{ n int }

// Equal is the service's only Equal: bytes.Equal's bare-name target.
func (s *Store) Equal(o *Store) bool { return s.n == o.n }

// Lock is the service's only Lock: a sync.Mutex's bare-name target.
func (s *Store) Lock() { s.n++ }

// Config is a type: a reference target, never a callee.
type Config struct{ Name string }

// NewConfig is a constructor: a real call target.
func NewConfig() *Config { return &Config{Name: "x"} }
