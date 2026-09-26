//go:build !darwin

package main

import "errors"

// waitProcessExit has no event source off darwin; watchParentExit's
// getppid poll carries the watch.
func waitProcessExit(int) error { return errors.New("no process-exit event source on this platform") }
