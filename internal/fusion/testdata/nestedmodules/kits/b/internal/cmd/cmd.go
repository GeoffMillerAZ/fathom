// Package cmd is kits/b's command tree: a second NewRoot.
package cmd

// NewRoot returns the exit code of kits/b's only verb.
func NewRoot() int { return runStatus() }

func runStatus() int { return 0 }
