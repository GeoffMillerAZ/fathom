// Package proberuntime defines the CUE contract for probe registration and results.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package proberuntime

// #ProbeStatus enumerates the possible probe execution outcomes.
#ProbeStatus: "pass" | "fail" | "error"

// #ProbeResult is the outcome of a single probe execution.
#ProbeResult: {
	// Name is the registered name of the probe that produced this result.
	Name: string & !=""

	// Status is the execution outcome.
	Status: #ProbeStatus

	// Detail is an optional human-readable description of the outcome.
	Detail: string
}
