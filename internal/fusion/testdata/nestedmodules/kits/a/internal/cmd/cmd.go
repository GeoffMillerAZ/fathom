// Package cmd builds kits/a's command tree. NewRoot is declared by kits/b's
// cmd package too, so a name-only resolver cannot say which one a main
// calls; only the typed pass of each module can.
package cmd

import (
	"fmt"

	"example.com/cli"
	"example.com/nestedroot/shared"
)

type options struct{ verbose bool }

// NewRoot returns the root command with its doctor verb.
func NewRoot() *cli.Command {
	opts := &options{}
	root := &cli.Command{Use: "toola"}
	root.AddCommand(&cli.Command{Use: "doctor", RunE: runE(runDoctor, opts)})
	return root
}

// runE adapts a leaf to the framework's RunE signature: the leaf is passed
// as a function value through a wrapper, never called by name.
func runE(fn func(o *options, args []string) error, o *options) func(*cli.Command, []string) error {
	return func(_ *cli.Command, args []string) error { return fn(o, args) }
}

// runDoctor is the leaf: reached from main only through RunE.
func runDoctor(o *options, args []string) error {
	if o.verbose {
		fmt.Println(len(args))
	}
	fmt.Println(shared.Banner())
	return nil
}
