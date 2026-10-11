// Package cmd is svc/api's command tree; svc/api and svc/worker both declare
// NewRoot, so only each module's typed pass binds a main's call.
package cmd

// NewRoot runs the service's verb.
func NewRoot() int { return runApi() }

func runApi() int { return 0 }
