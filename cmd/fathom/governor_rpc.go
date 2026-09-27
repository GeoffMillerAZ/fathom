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

// rpcToolClass is a tool call's declared class; `references` over services
// ["*"] is fleet (§3.1's argument-dependent class); an unknown tool is
// index.
func rpcToolClass(tool string, args json.RawMessage) tenancy.Class {
	return mcp.DeclaredCostClass(tool, args)
}
