// Package nestedroot is the fixture's root module: the one "./..." from the
// repository root loads. Everything under kits/ and shared/ lives in a module
// of its own (EW-59), which "./..." never enters.
package nestedroot

// Version is the root module's only symbol.
func Version() string { return "v0" }
