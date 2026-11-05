package client

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/memo"
)

// MemoScope names a service's shared snapshot and graph state. A backend
// without a store identity or watermark is never cached. Suppression changes
// invalidate the same state for reviews, navigation and the console.
func MemoScope(ctx context.Context, backend interface{ StoreID() string }, service string) memo.Scope {
	id := backend.StoreID()
	if id == "" {
		return memo.Scope{Service: service}
	}
	w := ""
	if source, ok := backend.(interface {
		StoreWatermark(context.Context, string) (string, error)
	}); ok {
		if mark, err := source.StoreWatermark(ctx, service); err == nil {
			w = mark
		}
	}
	if w != "" {
		if source, ok := backend.(interface{ SuppressionToken(string) string }); ok {
			if token := source.SuppressionToken(service); token != "" {
				w += "|suppressed:" + token
			}
		}
	}
	return memo.Scope{Store: id, Service: service, Watermark: w}
}
