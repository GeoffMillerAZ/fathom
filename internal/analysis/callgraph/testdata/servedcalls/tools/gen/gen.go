//go:build ignore

// gen is excluded from every default build: the typed pass never sees its
// call sites, so its syntax edges are all the graph has.
package main

import "sync"

var m sync.Mutex

func main() { m.Lock() }
