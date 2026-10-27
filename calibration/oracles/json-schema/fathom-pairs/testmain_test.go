package main

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/testisolate"
)

// TestMain runs the package under testisolate (RT-19): the pair counter reaches a
// daemon, state or call-log default through the packages it imports, so its
// tests must never touch the operator's HOME or live daemon.
func TestMain(m *testing.M) {
	testisolate.Main(m)
}
