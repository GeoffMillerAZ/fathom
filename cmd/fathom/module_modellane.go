package main

// module_modellane.go — classifies HOW a layer's compiled claims in a
// store were actually produced (deterministic / a named real model / the
// CLI's scripted mock lane / an ambiguous mix), from the distinct set of
// core.SemanticClaim.Model values recorded for (service, layer). This is
// the single source of truth `module doctor`'s published criteria (Phase J
// critic finding 2: a mock-lane compile must never present as real, and
// `module init`'s own "deterministic" default must never launder an
// un-compiled module into an oracle-class claim) and `module pack`'s
// README (which must name the lane honestly) both consult — computed once
// from the store's OWN claims, never from what a manifest merely claims
// about itself.

import (
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// mockModelName is the exact model name moduleCompileMock (module_compile
// eval.go) and `module eval --mock` stamp on every claim/eval artifact
// they produce — the literal string doctor checks for to refuse a mock
// compile/eval at the published level.
const mockModelName = "mock-model"

// compileLane is the result of classifying a (service, layer)'s compiled
// claims by their core.SemanticClaim.Model field.
type compileLane struct {
	// Lane is one of "none" (no claims), "deterministic" (every claim
	// carries an empty Model — an ADR-018 oracle-class pass),
	// "real" (every claim carries the SAME non-empty, non-mock model),
	// "mock" (every claim carries mockModelName), or "mixed" (more than
	// one distinct Model value — an ambiguous store this phase refuses to
	// attribute a single compiled_by to).
	Lane string
	// Model is the single model identity: "deterministic" for that lane
	// (matching store-format-v0 §7's oracle-class provenance convention),
	// the actual model name for "real", mockModelName for "mock". Empty
	// for "none"/"mixed".
	Model string
	// Detail is a human-readable explanation, always populated.
	Detail string
}

// classifyCompileLane inspects claims' Model field (never anything the
// manifest itself claims) and reports what actually compiled them.
func classifyCompileLane(claims []core.SemanticClaim) compileLane {
	models := map[string]bool{}
	for _, cl := range claims {
		models[cl.Model] = true
	}
	if len(models) == 0 {
		return compileLane{Lane: "none", Detail: "no claims compiled"}
	}
	if len(models) > 1 {
		names := make([]string, 0, len(models))
		for name := range models {
			if name == "" {
				name = "(deterministic/empty)"
			}
			names = append(names, name)
		}
		sort.Strings(names)
		return compileLane{Lane: "mixed", Detail: fmt.Sprintf("claims were compiled under multiple, disagreeing models: %v — cannot attribute a single provenance.compiled_by", names)}
	}
	var model string
	for name := range models {
		model = name
	}
	switch model {
	case "":
		return compileLane{Lane: "deterministic", Model: "deterministic", Detail: "every claim carries an empty Model (a deterministic, oracle-class compile pass — store-format-v0 §7)"}
	case mockModelName:
		return compileLane{Lane: "mock", Model: mockModelName, Detail: fmt.Sprintf("every claim was compiled under %q — the CLI's scripted/dev MockClient lane (--mock), never a real model", mockModelName)}
	default:
		return compileLane{Lane: "real", Model: model, Detail: fmt.Sprintf("every claim was compiled under model %q", model)}
	}
}

// sameClaimIDSet reports whether a and b contain exactly the same set of
// ClaimIDs — used by the published pack criterion (finding 4) to prove a
// packed store's claims are the SAME ones --store currently holds, not a
// stale or foreign copy.
func sameClaimIDSet(a, b []core.SemanticClaim) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, cl := range a {
		set[cl.ClaimID] = true
	}
	for _, cl := range b {
		if !set[cl.ClaimID] {
			return false
		}
		delete(set, cl.ClaimID)
	}
	return len(set) == 0
}
