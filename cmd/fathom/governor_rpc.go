package main

// governor_rpc.go — every /rpc tools/call is registered in the in-flight
// registry under its cost class (RT-172), so the refuse rung can cancel
// every tree and fleet call, youngest first, and the exit rung can name
// the top in-flight work. The class table is §3.1's, by today's cost; it is
// provisional until the admission gate declares a class on every tool
// (RT-55, costClassOf), which replaces it.

import (
	"context"
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// rpcToolClasses is §3.1's table (tools absent from it are index class).
var rpcToolClasses = map[string]tenancy.Class{
	// point
	"capabilities": tenancy.ClassPoint, "server_stats": tenancy.ClassPoint, "staleness_report": tenancy.ClassPoint,
	"read_source": tenancy.ClassPoint, "memory_search": tenancy.ClassPoint, "history": tenancy.ClassPoint,
	// tree
	"symbol_context": tenancy.ClassTree, "obligations": tenancy.ClassTree, "search": tenancy.ClassTree,
	"secret_scan": tenancy.ClassTree, "cloc": tenancy.ClassTree, "spec_coverage": tenancy.ClassTree,
	"traceability": tenancy.ClassTree, "safe_delete_check": tenancy.ClassTree, "review_change": tenancy.ClassTree,
	"list_sources": tenancy.ClassTree, "discoveries": tenancy.ClassTree, "references": tenancy.ClassTree,
	// fleet
	"list_services": tenancy.ClassFleet, "fleet_drift": tenancy.ClassFleet, "cross_repo_blast_radius": tenancy.ClassFleet,
	"review_diff": tenancy.ClassFleet, "dep_drift": tenancy.ClassFleet,
	// ingest
	"ingest": tenancy.ClassIngest, "onboard_service": tenancy.ClassIngest, "compile_layer": tenancy.ClassIngest,
}

// classWeight orders classes by cost for a batch: its class is its
// heaviest sub-call's.
func classWeight(c tenancy.Class) int {
	switch c {
	case tenancy.ClassPoint:
		return 0
	case tenancy.ClassIndex, tenancy.ClassEgress:
		return 1
	case tenancy.ClassTree:
		return 2
	case tenancy.ClassFleet:
		return 3
	case tenancy.ClassIngest:
		return 4
	}
	return 1
}

// rpcBatchClasses is each sub-call's class of a batch's arguments.
func rpcBatchClasses(args json.RawMessage) []tenancy.Class {
	var a struct {
		Calls []struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"calls"`
	}
	if json.Unmarshal(args, &a) != nil {
		return nil
	}
	out := make([]tenancy.Class, 0, len(a.Calls))
	for _, c := range a.Calls {
		if c.Name == "batch" {
			// A nested batch dispatches its own sub-calls: theirs count
			// (bounded by the message's JSON depth).
			out = append(out, rpcBatchClasses(c.Arguments)...)
			continue
		}
		out = append(out, rpcToolClass(c.Name, c.Arguments))
	}
	return out
}

// rpcToolClass is a tool call's class; `references` over services ["*"]
// is fleet (§3.1's argument-dependent class); a batch is its heaviest
// sub-call's (the refuse rung then cancels it with them).
func rpcToolClass(tool string, args json.RawMessage) tenancy.Class {
	if tool == "batch" {
		heaviest := tenancy.ClassIndex
		for _, c := range rpcBatchClasses(args) {
			if classWeight(c) > classWeight(heaviest) {
				heaviest = c
			}
		}
		return heaviest
	}
	if tool == "references" {
		var a struct {
			Services []string `json:"services"`
		}
		if json.Unmarshal(args, &a) == nil {
			for _, s := range a.Services {
				if s == "*" {
					return tenancy.ClassFleet
				}
			}
		}
	}
	if c, ok := rpcToolClasses[tool]; ok {
		return c
	}
	return tenancy.ClassIndex
}

// governorTicket registers one /rpc message in the in-flight registry when
// it is a tools/call; the returned ctx is cancelled when the refuse rung
// cancels it. The release is idempotent.
func governorTicket(ctx context.Context, raw []byte, origin string) (context.Context, func()) {
	var msg struct {
		Method string `json:"method"`
		Params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"params"`
	}
	if json.Unmarshal(raw, &msg) != nil || msg.Method != "tools/call" || msg.Params.Name == "" {
		return ctx, func() {}
	}
	var svc struct {
		Service string `json:"service"`
	}
	_ = json.Unmarshal(msg.Params.Arguments, &svc)
	return tenancy.DefaultInFlight.Begin(ctx, rpcToolClass(msg.Params.Name, msg.Params.Arguments), msg.Params.Name, svc.Service, origin)
}
