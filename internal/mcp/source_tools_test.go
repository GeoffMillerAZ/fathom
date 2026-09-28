// Package mcp_test — sourcesFakeBackend, a fake backend returning canned
// SourceFiles, shared by other test files (treescan_test.go,
// extra_tools_test.go, new_tools_test.go). The source-scanning analyzer
// tools this file used to test (secret_scan, shell_out, cloc) were
// unregistered by SR-19 (surface reckoning, F-77, 0 lifetime calls each);
// http_client_calls was unregistered earlier by SR-12 (F-22).
package mcp_test

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── sourcesFakeBackend ───────────────────────────────────────────────────────

// sourcesFakeBackend embeds the existing extendedFakeBackend and adds a Sources
// implementation that returns canned SourceFiles.
type sourcesFakeBackend struct {
	extendedFakeBackend
	sources []core.SourceFile
}

func (s *sourcesFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return s.sources, nil
}

// compile-time check: sourcesFakeBackend must implement QueryInterface.
var _ mcp.QueryInterface = (*sourcesFakeBackend)(nil)

func (s *sourcesFakeBackend) ScanSources(ctx context.Context, svc string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	files, err := s.Sources(ctx, svc)
	if err != nil {
		return corpus.Coverage{}, err
	}
	return corpus.ScanSourceFiles(ctx, files, bud, fn)
}
