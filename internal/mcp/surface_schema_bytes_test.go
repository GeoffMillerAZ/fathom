package mcp

import (
	"context"
	"testing"
)

// Surface reckoning SR-M1 (docs/plans/surface-reckoning.md, design §1): the
// schema bytes a client pays for the tools/list of each surface, before any
// work happens. The default eight-tool /rpc/agent was 18,310 bytes over 25
// tools before the reckoning and must be at most 9,000 now; the opt-in
// /rpc/extended and the bare /rpc are pinned at what they measure so a widening
// is a decision, not drift. Bare /rpc was 34,246 bytes over 59 tools.
const (
	// defaultSurfaceSchemaByteBudget is the design's own bar for /rpc/agent.
	defaultSurfaceSchemaByteBudget = 9000
	// Measured 2026-09-29 on the base of the efficiency wave: /rpc/agent 8,339
	// (8 tools), /rpc/extended and /rpc/companion 18,308 (21), bare /rpc 20,143
	// (25). Ceilings carry ~3 percent headroom, never a license to widen: re-pin
	// with the number that moved and why. The design's ≤ 19,000 for bare /rpc is
	// a target this row does not gate (it stands 1,143 bytes over).
	extendedSurfaceSchemaByteCeiling = 18900
	fullSurfaceSchemaByteCeiling     = 20800
	// The before numbers (docs/design/surface-reckoning.md §1), the bound every
	// surface must now sit well under.
	beforeAgentSurfaceSchemaBytes = 18310
	beforeFullSurfaceSchemaBytes  = 34246
)

// TestDefaultSurfaceSchemaBytesBudget pins SR-M1: the default surface's
// tools/list is at most 9,000 bytes over exactly its eight tools, and every
// surface is measured through the same computation a connected session's own
// status(section:"tools") reports (surfaceSchemaBytes).
func TestDefaultSurfaceSchemaBytesBudget(t *testing.T) {
	s := NewServer(nil)
	measure := func(surface string) (tools, bytes int) {
		ctx := WithRequestScope(context.Background(), surface, true)
		return len(s.visibleToolsFor(surface)), s.surfaceSchemaBytes(ctx)
	}

	agentTools, agentBytes := measure(SurfaceAgent)
	extTools, extBytes := measure(SurfaceExtended)
	compTools, compBytes := measure(SurfaceCompanion)
	fullTools, fullBytes := measure("")
	t.Logf("/rpc/agent: %d tools, %d bytes (before %d over 25 tools)", agentTools, agentBytes, beforeAgentSurfaceSchemaBytes)
	t.Logf("/rpc/extended: %d tools, %d bytes", extTools, extBytes)
	t.Logf("/rpc/companion: %d tools, %d bytes", compTools, compBytes)
	t.Logf("/rpc: %d tools, %d bytes (before %d over 59 tools)", fullTools, fullBytes, beforeFullSurfaceSchemaBytes)

	if agentTools != len(DefaultSurfaceTools()) || agentTools != 8 {
		t.Errorf("/rpc/agent serves %d tools, want the eight-tool default", agentTools)
	}
	if agentBytes == 0 || agentBytes > defaultSurfaceSchemaByteBudget {
		t.Errorf("/rpc/agent tools/list = %d bytes, want 1..%d (SR-M1: the default surface's schema tax)", agentBytes, defaultSurfaceSchemaByteBudget)
	}
	if extBytes > extendedSurfaceSchemaByteCeiling {
		t.Errorf("/rpc/extended tools/list = %d bytes, want <= %d", extBytes, extendedSurfaceSchemaByteCeiling)
	}
	if extTools != compTools || extBytes != compBytes {
		t.Errorf("/rpc/companion (%d tools, %d bytes) must equal /rpc/extended (%d tools, %d bytes): it is its alias", compTools, compBytes, extTools, extBytes)
	}
	if fullBytes > fullSurfaceSchemaByteCeiling {
		t.Errorf("/rpc tools/list = %d bytes, want <= %d", fullBytes, fullSurfaceSchemaByteCeiling)
	}
	if !(agentBytes < extBytes && extBytes < fullBytes) {
		t.Errorf("surfaces must widen in order agent < extended < full, got %d, %d, %d bytes", agentBytes, extBytes, fullBytes)
	}
	if agentBytes >= beforeAgentSurfaceSchemaBytes || fullBytes >= beforeFullSurfaceSchemaBytes {
		t.Errorf("a surface is no smaller than before the reckoning: agent %d (before %d), full %d (before %d)", agentBytes, beforeAgentSurfaceSchemaBytes, fullBytes, beforeFullSurfaceSchemaBytes)
	}

	// The measure is the served payload: it moves with the served tools/list
	// (contracts=true adds outputSchema), so the budget reads what a client is
	// actually sent and never a separately tracked figure.
	withContracts := s.surfaceSchemaBytes(WithContracts(WithRequestScope(context.Background(), SurfaceAgent, true), true))
	if withContracts <= agentBytes {
		t.Errorf("contracts=true tools/list = %d bytes, want more than the default %d", withContracts, agentBytes)
	}
}
