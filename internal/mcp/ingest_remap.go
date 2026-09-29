package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/worktree"
)

// checkRemapOfLiveRoot is EW-19's guard on an ingest that carries remap:true
// (O127). remap:true is the deliberate-move escape hatch, but the refusal that
// names it is read by workers as well as by people, and a worker that passed
// it with its scratch clone's path captured the owner's own service; every
// answer in the orchestrating session then resolved against a tree deleted
// with the clone. So while the recorded root is a LIVE checkout of the same
// repository as requested — it exists, it is a git checkout, and it shares
// its root commit with requested — the caller must also name that root
// (remap_from), which a caller that never looked at what it replaces cannot.
// A recorded root that is gone, is no git checkout, or belongs to another
// repository is not a live checkout of this one, and remap:true alone moves
// the service (the O127 repair is exactly that: the root was deleted).
func checkRemapOfLiveRoot(ctx context.Context, svc, recorded, requested, from string) error {
	if fi, err := os.Stat(recorded); err != nil || !fi.IsDir() {
		return nil
	}
	recID, ok := worktree.RepositoryIdentity(ctx, recorded)
	if !ok {
		return nil
	}
	if reqID, ok := worktree.RepositoryIdentity(ctx, requested); !ok || reqID != recID {
		return nil
	}
	if from != "" {
		fromCanon := filepath.Clean(from)
		if r, err := filepath.EvalSymlinks(from); err == nil {
			fromCanon = r
		}
		if fromCanon == recorded {
			return nil
		}
		return fmt.Errorf("remap_from %s is not service %q's current root %s — name the root you are replacing, exactly as list_services shows it", from, svc, recorded)
	}
	return fmt.Errorf("service %q is rooted at %s, a live checkout of the same repository as %s — remap:true alone would let a scratch clone capture it (a worker's clone is deleted with the worker). If this is a deliberate move, also pass remap_from: %q, the root you are replacing", svc, recorded, requested, recorded)
}
