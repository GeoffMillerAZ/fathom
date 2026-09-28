// Package main is the OpenAPI-first orders service of the system-graph
// fixture fleet (docs/design/system-graph.md §13): openapi.yaml is the
// contract, and these handlers implement it.
package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /orders", listOrders)
	mux.HandleFunc("POST /orders", createOrder)
	mux.HandleFunc("GET /orders/{orderId}", getOrder)
	// Seeded negative: a dead route, registered and specified, that no
	// client in the fleet calls.
	mux.HandleFunc("POST /orders/{orderId}/archive", archiveOrder)
	_ = http.ListenAndServe(":8080", mux)
}

func listOrders(w http.ResponseWriter, r *http.Request)   {}
func createOrder(w http.ResponseWriter, r *http.Request)  {}
func getOrder(w http.ResponseWriter, r *http.Request)     {}
func archiveOrder(w http.ResponseWriter, r *http.Request) {}
