// Package catalog implements the product catalog HTTP service — the
// route-registration half of the internal/analysis/serviceref fixture
// (VB-30, docs/plans/verbs.md): checkout-service's client calls (see
// ../checkout-service/checkout.go) join here on normalized (method,
// path template).
package catalog

import "context"

// Item is a catalog entry.
type Item struct {
	ID    string
	Name  string
	Price int64
}

// Router is the minimal interface RegisterRoutes needs from a mux — a
// typed-method registration style (r.GET("/path", handler)) that
// internal/analysis/routes.Extract's Go patterns already parse.
type Router interface {
	GET(path string, handler HandlerFunc)
	POST(path string, handler HandlerFunc)
}

// HandlerFunc is this fixture's minimal handler signature.
type HandlerFunc func(ctx context.Context) error

// RegisterRoutes wires the catalog HTTP routes onto r. GET /items/:id is
// the route the fixture's cross-repo join is expected to find (checkout-
// service.FetchItem calls it); POST /items has no matching client call in
// the fixture fleet, and is here to prove an unmatched ROUTE produces no
// hop just as an unmatched CALL (checkout-service.HoldItem) does.
func RegisterRoutes(r Router) {
	r.GET("/items/:id", getItemHandler)
	r.POST("/items", createItemHandler)
}

func getItemHandler(ctx context.Context) error {
	return nil
}

func createItemHandler(ctx context.Context) error {
	return nil
}
