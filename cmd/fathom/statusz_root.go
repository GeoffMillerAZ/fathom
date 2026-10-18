package main

// statusz_root.go — /statusz?format=json&root=<dir> (docs/design/
// claude-code-mods.md §7.3, ADR-072): the one indexed service whose recorded
// root holds dir, with its checkout against the index — HEAD, the commits
// from the indexed commit to HEAD, and whether a tracked file changed since
// HEAD. `fathom statusline` asks it once per refresh.
//
// Every git read goes through the read-only wrapper (internal/gitro: HEAD
// from the repository's files, rev-list and status with optional locks off
// and a private index copy) under one 200 ms deadline. A reading that could
// not be taken is null with a reason, never 0 or false (I38): a dirty tree
// or an unknown distance must never read as fresh. The fields are additive:
// without the query the JSON is exactly what it was.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// statuszGitDeadline bounds every git read one root answer makes (a var for
// tests).
var statuszGitDeadline = 200 * time.Millisecond

// StatuszHead is one service's checkout against its index. Each reading is
// null when it could not be taken, with the reason beside it.
type StatuszHead struct {
	HeadCommit *string `json:"head_commit"`
	HeadReason string  `json:"head_reason,omitempty"`
	// Behind counts the commits from the indexed commit to HEAD; 0 only
	// when HEAD is the indexed commit.
	Behind       *int   `json:"behind"`
	BehindReason string `json:"behind_reason,omitempty"`
	// Dirty reports a tracked file under the service root changed since
	// HEAD (staged or not); untracked files are not counted.
	Dirty       *bool  `json:"dirty"`
	DirtyReason string `json:"dirty_reason,omitempty"`
}

// StatuszRoot is the root query's answer: Indexed is true when Service's
// recorded root holds Dir, false when no indexed service's does, and null
// when that could not be told (Reason says why).
type StatuszRoot struct {
	Dir     string `json:"dir"`
	Indexed *bool  `json:"indexed"`
	Service string `json:"service,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// statuszForRoot is the /statusz report for root=dir: the daemon-wide header,
// the root answer, and only the matched service's row, carrying its head
// fields (no row when none matched). It builds that one row alone: the root
// query runs on every status-line refresh.
func statuszForRoot(ctx context.Context, c *client.Client, startTime, now time.Time, dir string) StatuszReport {
	r := statuszReportHeader(startTime, now)
	ans := &StatuszRoot{Dir: dir}
	r.Root = ans
	r.Services = []StatuszServiceRow{}
	if !filepath.IsAbs(dir) {
		ans.Reason = "root is not an absolute path, so whether a service root holds it cannot be told"
		return r
	}
	services, err := c.Services(ctx)
	if err != nil {
		ans.Reason = "the service list could not be read: " + err.Error()
		return r
	}
	cands := make([]statuszRootCand, 0, len(services))
	for _, svc := range services {
		if path, _, found, merr := c.ServiceMeta(ctx, svc); merr == nil && found && path != "" {
			cands = append(cands, statuszRootCand{service: svc, root: path})
		}
	}
	svc, ok := statuszRootHolder(cands, dir)
	ans.Indexed = &ok
	if !ok {
		return r
	}
	ans.Service = svc
	row := statuszServiceRowFor(ctx, c, svc)
	row.StatuszHead = statuszHeadFor(ctx, row.root, row.IndexedCommit)
	r.Services = []StatuszServiceRow{row}
	return r
}

// statuszRootCand is one service's recorded corpus root.
type statuszRootCand struct{ service, root string }

// statuszRootHolder picks the service whose root holds dir (is dir or an
// ancestor of it), compared as recorded or with symlinks resolved: the
// deepest such root (its resolved length), so a nested service wins over
// its enclosing repository's, and between equal roots the first service
// name in sorted order — never map order.
func statuszRootHolder(cands []statuszRootCand, dir string) (string, bool) {
	dirClean, dirReal := statuszPathForms(dir)
	best, bestDepth := "", -1
	for _, c := range cands {
		clean, real := statuszPathForms(c.root)
		if !statuszPathHolds(real, dirReal) && !statuszPathHolds(clean, dirClean) {
			continue
		}
		if depth := len(real); depth > bestDepth || (depth == bestDepth && c.service < best) {
			best, bestDepth = c.service, depth
		}
	}
	return best, bestDepth >= 0
}

// statuszPathForms is p cleaned, and its symlink-resolved form (the cleaned
// one when it does not resolve).
func statuszPathForms(p string) (clean, real string) {
	clean = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(clean); err == nil {
		return clean, r
	}
	return clean, clean
}

// statuszPathHolds reports whether root is dir or an ancestor of it.
func statuszPathHolds(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// statuszHeadFor reads root's checkout against indexed under one
// statuszGitDeadline: HEAD from the repository's files, then the commits
// behind and the dirty state through the read-only git wrapper.
func statuszHeadFor(ctx context.Context, root, indexed string) *StatuszHead {
	h := &StatuszHead{}
	if root == "" {
		reason := "no service root is recorded, so its checkout cannot be read"
		h.HeadReason, h.BehindReason, h.DirtyReason = reason, reason, reason
		return h
	}
	gctx, cancel := context.WithTimeout(ctx, statuszGitDeadline)
	defer cancel()
	head, err := gitro.Head(gctx, root)
	if err != nil || !gitro.IsObjectName(head) {
		if err == nil {
			err = fmt.Errorf("HEAD read as %q", head)
		}
		reason := statuszGitReason(gctx, "HEAD could not be read at the service root", err)
		h.HeadReason, h.BehindReason, h.DirtyReason = reason, reason, reason
		return h
	}
	h.HeadCommit = &head
	h.Behind, h.BehindReason = statuszBehind(gctx, root, indexed, head)
	h.Dirty, h.DirtyReason = statuszDirty(gctx, root)
	return h
}

// statuszBehind counts the commits from indexed to head. Unknown (nil, with
// the reason) when no commit is recorded, the checkout lacks it, it is not
// an ancestor of HEAD (the count would read a moved-off checkout as fresh),
// or git did not answer in time.
func statuszBehind(ctx context.Context, root, indexed, head string) (*int, string) {
	switch {
	case indexed == "":
		return nil, "no indexed commit is recorded for this service"
	case !gitro.IsObjectName(indexed):
		return nil, fmt.Sprintf("the indexed commit %q is not a full commit name", indexed)
	case indexed == head:
		zero := 0
		return &zero, ""
	}
	out, err := gitro.Output(ctx, root, "rev-list", "--left-right", "--count", indexed+"..."+head)
	if err != nil {
		return nil, statuszGitReason(ctx, "git could not count the commits from the indexed commit to HEAD", err)
	}
	f := strings.Fields(string(out))
	if len(f) != 2 {
		return nil, fmt.Sprintf("git rev-list answered %q, not two counts", strings.TrimSpace(string(out)))
	}
	ahead, aerr := strconv.Atoi(f[0])
	behind, berr := strconv.Atoi(f[1])
	switch {
	case aerr != nil || berr != nil:
		return nil, fmt.Sprintf("git rev-list answered %q, not two counts", strings.TrimSpace(string(out)))
	case ahead > 0:
		return nil, fmt.Sprintf("the indexed commit is not an ancestor of HEAD (HEAD lacks %d of its commits), so commits behind cannot be counted", ahead)
	case behind <= 0:
		return nil, "git counted no commits between two different commits"
	}
	return &behind, ""
}

// statuszDirty reports whether a tracked file under root changed since HEAD.
func statuszDirty(ctx context.Context, root string) (*bool, string) {
	out, err := gitro.Output(ctx, root, "status", "--porcelain", "--untracked-files=no", "--no-renames", "--", ".")
	if err != nil {
		return nil, statuszGitReason(ctx, "git status could not read the work tree", err)
	}
	dirty := len(bytes.TrimSpace(out)) > 0
	return &dirty, ""
}

// statuszGitReason names a failed git read: the deadline when it passed,
// else git's own first line of complaint.
func statuszGitReason(ctx context.Context, what string, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return what + ": not answered within the " + statuszGitDeadline.String() + " deadline"
	}
	msg := err.Error()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if line, _, _ := strings.Cut(strings.TrimSpace(string(ee.Stderr)), "\n"); line != "" {
			msg = line
		}
	}
	return what + ": " + msg
}
