// Package proberuntime — valid test data that must unify with schema.cue.
package proberuntime

import "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/probe-runtime:proberuntime"

// validPassResult is a well-formed passing probe result.
validPassResult: proberuntime.#ProbeResult & {
	Name:   "db-ping"
	Status: "pass"
	Detail: "database responded in 2ms"
}

// validFailResult is a well-formed failing probe result.
validFailResult: proberuntime.#ProbeResult & {
	Name:   "cache-health"
	Status: "fail"
	Detail: "cache miss rate exceeded threshold"
}

// validErrorResult is a well-formed error probe result.
validErrorResult: proberuntime.#ProbeResult & {
	Name:   "auth-service"
	Status: "error"
	Detail: "connection refused"
}

// validResultEmptyDetail verifies that Detail may be empty.
validResultEmptyDetail: proberuntime.#ProbeResult & {
	Name:   "queue-depth"
	Status: "pass"
	Detail: ""
}

// validStatuses verifies all status enum values are accepted.
_s1: proberuntime.#ProbeStatus & "pass"
_s2: proberuntime.#ProbeStatus & "fail"
_s3: proberuntime.#ProbeStatus & "error"
