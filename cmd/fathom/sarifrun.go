package main

// sarifrun.go — EVX-3 (docs/plans/portable-receipts.md; ADR-068 decision
// 9): every CLI SARIF caller (verify, obligations, rules replay, config
// audit, intent audit, `review --receipt`) renders through one path:
// receipt.AnchorRun names the run's commit and anchors each finding at it,
// then verify.RenderSARIFRun writes the run's provenance and each anchored
// finding's excerpt fingerprint. review_diff's receipt_sarif takes the same
// two steps inside the daemon (internal/mcp/review_tool.go).

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/receipt"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// cliSARIF renders tool's results as one SARIF run with run's provenance;
// the repository URI is the repository's own `[receipts] repository_uri`.
func cliSARIF(run receipt.Run, tool string, results []verify.SARIFResult) []byte {
	if run.RepositoryURIAt == nil {
		run.RepositoryURIAt = receipt.ConfiguredRepositoryURI
	}
	prov, anchored := receipt.AnchorRun(context.Background(), run, results)
	return verify.RenderSARIFRun(tool, prov, anchored)
}
