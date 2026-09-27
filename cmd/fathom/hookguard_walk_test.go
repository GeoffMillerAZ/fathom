package main

import (
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/custodian"
)

// RT-134 hygiene: every hook — not only ConfigChange/FileChanged — bounds
// the custodian's config walk by its deadline and refuses a non-git root,
// so SessionStart's attention line never walks $HOME; the policy ends with
// the hook.
func TestEveryHookBoundsTheConfigWalk(t *testing.T) {
	custodian.SetWalkPolicy(nil)
	end, silent := beginHookGuard("session-hook", "fathom session-hook", "claude-code", "http://127.0.0.1:1")
	if silent {
		t.Skip("kill switch set in this environment")
	}
	var ng *custodian.NotGitRootError
	if _, err := custodian.AgentConfigFiles(t.TempDir()); !errors.As(err, &ng) {
		end()
		t.Fatalf("inside SessionStart a non-git root was walked: %v", err)
	}
	end()
	if _, err := custodian.AgentConfigFiles(t.TempDir()); err != nil {
		t.Fatalf("after the hook the CLI default walks again: %v", err)
	}
}
