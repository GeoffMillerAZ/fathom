// Package mcp — surviving source-tree read helper.
//
// The source-scanning analyzer tools this file used to register are gone:
// secret_scan, shell_out and cloc were unregistered by SR-19 (surface
// reckoning, F-77, 0 lifetime calls each); http_client_calls was
// unregistered earlier by SR-12 (F-22). fetchSources stays — meta.go still
// calls it for a whole-service read under the tree budget.
package mcp

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
)

// fetchSources gathers the service's files under the tree budget
// (collectSources, RT-71) and converts the error to rpcError. The coverage
// names a partial scan; callers disclose it with withPartial.
func fetchSources(ctx context.Context, backend QueryInterface, service string) ([]core.SourceFile, corpus.Coverage, *rpcError) {
	files, cov, err := collectSources(ctx, backend, service)
	if err != nil {
		return nil, cov, &rpcError{Code: ErrCodeInternal, Message: "sources: " + err.Error()}
	}
	return files, cov, nil
}
