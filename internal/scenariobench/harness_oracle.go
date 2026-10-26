package scenariobench

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// OracleHarness supplies the known answer without an agent or paid calls.
// Patch bytes enter only through FetchGoldPatch, never through the subject.
type OracleHarness struct {
	source   SourcePin
	cacheDir string
	fetch    GoldPatchFetcher
	ctx      context.Context
	task     Task
}

// NewOracleHarness uses an operator-supplied offline fetcher (nil permits
// verified cache hits only). cacheDir must live beneath workspace/.
func NewOracleHarness(source SourcePin, cacheDir string, fetch GoldPatchFetcher) *OracleHarness {
	return &OracleHarness{source: source, cacheDir: cacheDir, fetch: fetch}
}

func (*OracleHarness) Name() string         { return "oracle" }
func (*OracleHarness) Version() string      { return "oracle-1" }
func (*OracleHarness) ExpectsNoTurns() bool { return true }

// withTask copies configuration so concurrent cells cannot exchange task gold.
func (h *OracleHarness) withTask(ctx context.Context, task Task) Harness {
	copy := *h
	copy.ctx, copy.task = ctx, task
	return &copy
}

func (h *OracleHarness) RunSession(req SessionRequest) (SessionResult, error) {
	result := SessionResult{Index: req.Index, Role: req.Role, RowID: req.RowID}
	if h.ctx == nil {
		return result, fmt.Errorf("oracle harness requires RunSessions task context")
	}
	if h.task.Oracle.Localization {
		result.FinalAnswer = strings.Join(dedupSorted(h.task.Gold.Files), "\n")
		return result, nil
	}
	if !strings.Contains("/"+filepath.ToSlash(filepath.Clean(h.cacheDir))+"/", "/workspace/") {
		return result, fmt.Errorf("oracle gold patch cache must be under workspace/")
	}
	patch, err := FetchGoldPatch(h.ctx, h.task, h.source, h.cacheDir, h.fetch)
	if err != nil {
		return result, err
	}
	// A multi-session task applies its patch once. Every cell has a fresh worktree.
	if req.Index == 0 {
		if err := runGitApply(h.ctx, req.Worktree, patch, false); err != nil {
			return result, err
		}
		result.FilesEdited = dedupSorted(testPatchPaths(string(patch)))
	}
	return result, nil
}

var _ Harness = (*OracleHarness)(nil)
