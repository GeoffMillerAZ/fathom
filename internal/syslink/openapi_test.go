package syslink

import "testing"

// TestOpenAPIProvidesSpecTier is SG-7's contract: an OpenAPI 3.x document
// provides route: keys at spec tier, each bound to its operationId, with
// the servers' shared base path applied; Swagger 2.0 and other YAML/JSON
// provide nothing.
func TestOpenAPIProvidesSpecTier(t *testing.T) {
	doc := []byte(`openapi: 3.0.3
info: {title: Orders, version: "1"}
servers:
  - url: https://orders.example/api/v1
paths:
  /orders:
    get:
      operationId: listOrders
    post:
      operationId: createOrder
  /orders/{orderId}:
    parameters:
      - name: orderId
        in: path
    get:
      operationId: getOrder
    delete: {}
`)
	checkRows(t, "yaml", openAPIProvides("api/openapi.yaml", doc), []rowWant{
		{"route:GET /api/v1/orders", "provides", "listOrders", TierSpec, 7},
		{"route:POST /api/v1/orders", "provides", "createOrder", TierSpec, 9},
		{"route:GET /api/v1/orders/{}", "provides", "getOrder", TierSpec, 15},
		{"route:DELETE /api/v1/orders/{}", "provides", "DELETE /api/v1/orders/{}", TierSpec, 17},
	})
	for _, r := range openAPIProvides("api/openapi.yaml", doc) {
		if r.NodeKind != "operation" {
			t.Errorf("row %v: node kind %q", r.Key, r.NodeKind)
		}
	}

	js := []byte(`{"openapi": "3.1.0", "paths": {"/items/{id}": {"put": {"operationId": "putItem"}}}}`)
	checkRows(t, "json", openAPIProvides("items.json", js), []rowWant{
		{"route:PUT /items/{}", "provides", "putItem", TierSpec, 1},
	})

	for _, neg := range []string{
		"swagger: \"2.0\"\npaths:\n  /x:\n    get: {operationId: x}\n",
		"kind: Deployment\nmetadata: {name: orders}\n",
		"{not yaml",
	} {
		if got := openAPIProvides("x.yaml", []byte(neg)); len(got) != 0 {
			t.Errorf("non-OpenAPI-3 document %q produced %v", neg, got)
		}
	}
}
