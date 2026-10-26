package scenariobench

import (
	"context"
	"fmt"
	"strings"
)

// WrongPatchHarness supplies a deterministic negative control, without an agent.
// The killed-mutant variant waits for BX-10 (MineMutants); only the task's
// test patch is applied on F2 until that prerequisite exists.
type WrongPatchHarness struct {
	ctx  context.Context
	task Task
}

func NewWrongPatchHarness() *WrongPatchHarness  { return &WrongPatchHarness{} }
func (*WrongPatchHarness) Name() string         { return "wrong-patch" }
func (*WrongPatchHarness) Version() string      { return "wrong-patch-1" }
func (*WrongPatchHarness) ExpectsNoTurns() bool { return true }
func (h *WrongPatchHarness) withTask(ctx context.Context, task Task) Harness {
	return &WrongPatchHarness{ctx: ctx, task: task}
}
func (h *WrongPatchHarness) RunSession(req SessionRequest) (SessionResult, error) {
	result := SessionResult{Index: req.Index, Role: req.Role, RowID: req.RowID}
	if h.ctx == nil {
		return result, fmt.Errorf("wrong-patch harness requires RunSessions task context")
	}
	if h.task.Oracle.Localization {
		// Use visible tracked paths: dot directories contain the rendered arm settings.
		out, err := runIn(h.ctx, req.Worktree, "git", "ls-files", "-z")
		if err != nil {
			return result, err
		}
		var files []string
		for _, file := range dedupSorted(strings.Split(out, "\x00")) {
			if file == "" || strings.HasPrefix(file, ".") {
				continue
			}
			gold := false
			for _, g := range h.task.Gold.Files {
				if sameFile(file, g) {
					gold = true
					break
				}
			}
			if !gold {
				files = append(files, file)
				if len(files) == 3 {
					break
				}
			}
		}
		result.FinalAnswer = strings.Join(files, "\n")
		return result, nil
	}
	if h.task.Gold.TestPatch == "" {
		return result, fmt.Errorf("wrong-patch F2 task requires a test patch")
	}
	if req.Index == 0 {
		if err := runGitApply(h.ctx, req.Worktree, []byte(h.task.Gold.TestPatch), false); err != nil {
			return result, err
		}
		result.FilesEdited = dedupSorted(testPatchPaths(h.task.Gold.TestPatch))
	}
	return result, nil
}

var _ Harness = (*WrongPatchHarness)(nil)
