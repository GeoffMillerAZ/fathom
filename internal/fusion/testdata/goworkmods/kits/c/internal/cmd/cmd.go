// Package cmd is kits/c's command tree.
package cmd

// NewRoot runs kits/c's only verb.
func NewRoot() int { return runCheck() }

func runCheck() int { return 0 }
