package main

import (
	"errors"
	"fmt"
	"net/http"
)

// serveUntilDone swaps the full handler in and waits for the server to
// stop; the registered cleanups then run as runServeHTTP returns.
func (r *serveRun) serveUntilDone() int {
	r.swap.set(r.handler)
	err := <-r.serveErr
	// Shutdown closes the listener before its in-flight requests finish.
	// A managed drain starts before Shutdown and publishes its exit only
	// after the drain attempt, so do not close the store in between.
	select {
	case <-r.managedDrain:
		<-r.exitRequested
	default:
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		if code, exiting := r.requestedExitCode(); exiting {
			return code
		}
		r.flog.Error(fmt.Sprintf("http serve: %v", err))
		r.exitRow("serve_error", err.Error(), 1)
		r.removeRecord()
		return 1
	}
	if code, exiting := r.requestedExitCode(); exiting {
		return code
	}
	return 0
}
