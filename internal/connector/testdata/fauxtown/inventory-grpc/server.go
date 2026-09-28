// Package main serves the Inventory gRPC service of the system-graph
// fixture fleet.
package main

type server struct{}

func (server) Reserve() {}
func (server) Release() {}

func main() {}
