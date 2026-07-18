// Package workflowengine — valid test data that must unify with schema.cue.
package workflowengine

import "github.com/geoffmilleraz/agentic-kit/foundations/orchestration/workflow-engine:workflowengine"

// validMinimalDefinition — a minimal valid two-state workflow.
validMinimalDefinition: workflowengine.#WorkflowDefinition & {
	id:            "minimal-workflow"
	version:       "1.0.0"
	initial_state: "pending"
	states: [
		{id: "pending", terminal: false, description: "Waiting to start"},
		{id: "done", terminal: true, description: "Completed"},
	]
	transitions: [
		{from: "pending", to: "done", on: "complete", guards: [], actions: []},
	]
}

// validBranchingDefinition — a workflow with two terminal outcomes.
validBranchingDefinition: workflowengine.#WorkflowDefinition & {
	id:            "branching-workflow"
	version:       "2.0.0"
	initial_state: "idle"
	states: [
		{id: "idle", terminal: false, description: "Idle"},
		{id: "running", terminal: false, description: "Running"},
		{id: "failed", terminal: true, description: "Failed"},
		{id: "succeeded", terminal: true, description: "Succeeded"},
	]
	transitions: [
		{from: "idle", to: "running", on: "start", guards: [], actions: []},
		{from: "running", to: "succeeded", on: "success", guards: [], actions: []},
		{from: "running", to: "failed", on: "failure", guards: [], actions: []},
	]
}

// validWithPhases — a workflow with lifecycle phases.
validWithPhases: workflowengine.#WorkflowDefinition & {
	id:            "phased-workflow"
	version:       "1.0.0"
	initial_state: "draft"
	states: [
		{id: "draft", terminal: false, description: "Draft"},
		{id: "review", terminal: false, description: "Under review"},
		{id: "published", terminal: true, description: "Published"},
	]
	transitions: [
		{from: "draft", to: "review", on: "submit", guards: [], actions: []},
		{from: "review", to: "published", on: "approve", guards: [], actions: []},
		{from: "review", to: "draft", on: "reject", guards: [], actions: []},
	]
	phases: [
		{id: "authoring", states: ["draft"], entry_event: ""},
		{id: "approval", states: ["review"], entry_event: "submit"},
	]
}
