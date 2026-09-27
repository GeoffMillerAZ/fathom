package main

import (
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// fleetAdapter is client.FleetAdapter: the single multi-service store
// client as mcp.FleetBackend. It moved beside the client (RT-93) so the
// cost harness measures the daemon's own adapter.
type fleetAdapter = client.FleetAdapter

// newFleetAdapter adapts c, logging store errors (deduped) to log.
func newFleetAdapter(c *client.Client, log *dlog.Logger) *fleetAdapter {
	return client.NewFleetAdapter(c, log)
}

// classifyStoreError renders a store-layer error for logging
// (client.ClassifyStoreError: store distress and SQLITE_CANTOPEN named).
func classifyStoreError(err error) string { return client.ClassifyStoreError(err) }

// storeDistressKind names a distress error (tenancy.StoreDistressKind).
func storeDistressKind(err error) string { return tenancy.StoreDistressKind(err) }
