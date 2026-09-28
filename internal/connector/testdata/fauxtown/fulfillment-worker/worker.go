// Package main is a Go consumer in the system-graph fixture fleet: it
// calls orders-api over HTTP and inventory-grpc through the generated stub.
package main

import (
	"context"
	"net/http"

	inventoryv1 "fauxtown/fulfillment-worker/gen/inventory/v1"
)

const ordersBase = "http://orders.fauxtown.example"

func fulfil(ctx context.Context, conn any, orderID string) {
	_, _ = http.Get(ordersBase + "/orders/" + orderID)

	inv := inventoryv1.NewInventoryClient(conn)
	inv.Reserve(ctx, nil)

	// Seeded negative: an endpoint no service provides.
	_, _ = http.Get(ordersBase + "/orders-legacy/export")
}

func main() {}
