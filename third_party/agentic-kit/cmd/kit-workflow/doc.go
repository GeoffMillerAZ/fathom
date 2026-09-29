// Command kit-workflow provides a bounded JSON front door for local governed
// workflow checks. It has no model, network, shell, publication, or durable
// approval capability.
//
// Calls are independent and retain no mutable package state. Concurrent calls
// are safe when their readers and writers are independently safe.
package main
