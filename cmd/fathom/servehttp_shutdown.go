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
	if err := <-r.serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.flog.Error(fmt.Sprintf("http serve: %v", err))
		r.exitRow("serve_error", err.Error(), 1)
		r.removeRecord()
		return 1
	}
	return 0
}
