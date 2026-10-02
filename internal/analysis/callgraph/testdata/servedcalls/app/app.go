// Package app is the served-call fixture's caller package: every file here
// is compiled, so the typed pass resolves every call site in it.
package app

import (
	"bytes"
	"sync"

	"servedmod/svc"
)

var mu sync.Mutex

// Same calls only dependencies: sync.Mutex's Lock and bytes.Equal.
func Same(a, b []byte) bool {
	mu.Lock()
	return bytes.Equal(a, b)
}

// Compare calls the service's Equal for real.
func Compare(x, y *svc.Store) bool { return x.Equal(y) }

// Build calls a constructor and references a type.
func Build() *svc.Config {
	c := svc.NewConfig()
	var d svc.Config
	d.Name = c.Name
	return &d
}
