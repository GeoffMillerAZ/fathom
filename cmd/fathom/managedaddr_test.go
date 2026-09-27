package main

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/hookguard"
)

// managedAddr and managedPort name the managed daemon's default address
// symbolically: a test that is ABOUT the default (operator-state refusal,
// the kill switch's managed URL) says so without carrying the operator's
// live address as a literal (tenancy lint test-live-url, RT-91).
var (
	managedAddr = strings.TrimPrefix(hookguard.DefaultManagedURL, "http://")
	managedPort = managedAddr[strings.LastIndex(managedAddr, ":")+1:]
)
