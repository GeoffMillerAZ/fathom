// Package workerheartbeat defines the CUE contract for worker heartbeat records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package workerheartbeat

// #Heartbeat records a single liveness signal from a worker.
#Heartbeat: {
	// WorkerID is the non-empty stable identifier for the worker process.
	WorkerID: string & !=""

	// At is the RFC 3339 timestamp of when the heartbeat was recorded.
	At: string & !=""
}
