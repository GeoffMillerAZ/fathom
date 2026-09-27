package main

// governor_rpc.go — a tool call's cost class for the governor and the
// console. The class is the one the tool declares (RT-55, internal/mcp
// costclass.go); dispatch's admission ticket registers every /rpc
// tools/call in the in-flight registry under it (RT-56), so the refuse
// rung can cancel every tree and fleet call, youngest first.

import (
	"encoding/json"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

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

// rpcToolClass is a tool call's declared class (RT-55, mcp.DeclaredCostClass);
// `references` over services ["*"] is fleet (§3.1's argument-dependent
// class); an unknown tool is index; a batch is its heaviest sub-call's (the
// lifecycle gate refuses it with them).
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
	return mcp.DeclaredCostClass(tool, args)
}
