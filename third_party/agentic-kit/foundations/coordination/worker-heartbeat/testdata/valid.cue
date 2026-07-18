// Package workerheartbeat — valid test data that must unify with schema.cue.
package workerheartbeat

import "github.com/geoffmilleraz/agentic-kit/foundations/coordination/worker-heartbeat:workerheartbeat"

// validHeartbeat is a well-formed liveness signal from a running worker.
validHeartbeat: workerheartbeat.#Heartbeat & {
	WorkerID: "worker-42"
	At:       "2024-11-01T09:00:00Z"
}

// validHeartbeat2 is another well-formed heartbeat from a different worker.
validHeartbeat2: workerheartbeat.#Heartbeat & {
	WorkerID: "worker-analytics-prod-7"
	At:       "2024-11-02T14:30:00Z"
}
