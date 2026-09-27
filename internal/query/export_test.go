package query

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// FindSymbolFullScan and FindCallersFullScan are the whole-service answers
// the RT-85 point lookups must equal.
func (e *Engine) FindSymbolFullScan(ctx context.Context, svc, name string) ([]core.SymbolNode, error) {
	return e.findSymbolFull(ctx, svc, name)
}

func (e *Engine) FindCallersFullScan(ctx context.Context, svc, symbol string) ([]core.SymbolNode, error) {
	return e.findCallersFull(ctx, svc, symbol)
}
