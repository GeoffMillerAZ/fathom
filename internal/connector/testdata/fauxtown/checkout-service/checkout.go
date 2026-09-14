// Package checkout implements the checkout HTTP client — the client-call
// half of the internal/analysis/serviceref fixture (VB-30, docs/plans/
// verbs.md): FetchItem's (GET, "/items/{}") matches catalog-service's
// registered "GET /items/:id" route (see ../catalog-service/catalog.go)
// after both sides normalize their param syntax to "{}". HoldItem is the
// decoy: its (DELETE, "/items/{}/hold") matches no route anywhere in the
// fixture fleet, so the cross-repo join must not manufacture an edge for
// it.
package checkout

import (
	"context"
	"fmt"
	"net/http"
)

// FetchItem calls catalog-service to look up an item by id.
func FetchItem(ctx context.Context, id string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("http://catalog-service/items/%s", id), nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

// HoldItem is the decoy call: DELETE /items/{id}/hold matches no route any
// fixture service registers.
func HoldItem(ctx context.Context, id string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("http://catalog-service/items/%s/hold", id), nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}
