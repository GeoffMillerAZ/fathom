package main

// readouthook_joins.go: what the readout hook computes for the engine beyond
// the output (docs/design/readouts.md §5 and §7), each local and inside one
// budget: TR-1's declared-test answer (Input.DeclaredTest), the persisted
// output's text (Input.PersistedText), HEAD's stamp (Joins["head_stamp"]),
// the edit check a whole-file read is raw under (Input.Edited) and the
// outline (Joins["outline"], 150 ms of the budget). A join that cannot be
// computed is left out and named in the row's joins_missed, never guessed.

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// readoutJoinBudget is every local join together (design §5): a join past it
// is missed.
var readoutJoinBudget = 300 * time.Millisecond

// readoutPersistedCap bounds the persisted-output read; a larger file is a
// missed join, never a count over a cut text.
const readoutPersistedCap = 8 << 20

// The joins the hook computes, by the names its row carries.
const (
	readoutJoinPersisted = "persisted"
	readoutJoinHeadStamp = "head_stamp"
	readoutJoinEdited    = "edited_in_session"
	readoutJoinOutline   = "outline"
)

// readoutJoinRun is which joins one call ran and which it missed.
type readoutJoinRun struct {
	ran    []string
	missed []string
}

func (j *readoutJoinRun) add(name string, ok bool) {
	if ok {
		j.ran = append(j.ran, name)
		return
	}
	j.missed = append(j.missed, name)
}

// readoutMayMatch reports whether any rule could match the command: the
// engine serves an unreadable, raw-prefixed, rtk or interrupted call raw
// before it matches, so its declarations are not read.
func readoutMayMatch(c *shellcmd.Command, in harness.HookInput) bool {
	return !c.Unreadable && c.ReadoutPrefix() != "raw" && !c.Rtk() && !in.Shell.Interrupted
}

// readoutDeclaredTest is TR-1's answer as the test-run hook asks it
// (cmdTestRunHook): root's declarations, then with the configured `[tests]
// commands` when a config layer names them.
func readoutDeclaredTest(root, command string) bool {
	if root == "" {
		return false
	}
	if _, ok := matchDeclaredTestCommand(root, command, nil); ok {
		return true
	}
	proj := projectdir.Config.Path(root)
	userRaw, projRaw := readConfigLayer(config.DefaultUserPath()), readConfigLayer(proj)
	if !bytes.Contains(userRaw, []byte("commands")) && !(config.RepoMaySet("tests.commands") && bytes.Contains(projRaw, []byte("commands"))) {
		return false
	}
	l, err := config.Load(config.DefaultUserPath(), proj)
	if err != nil {
		return false
	}
	_, ok := matchDeclaredTestCommand(root, command, l.Config.Tests.Commands)
	return ok
}

// readoutHookJoins computes the joins the matched rule reads into rin, inside
// ctx (readoutJoinBudget). dir is the command's working directory.
func readoutHookJoins(ctx context.Context, hook harness.HookInput, dir string, c *shellcmd.Command, rule *readout.Rule, rin *readout.Input) readoutJoinRun {
	var j readoutJoinRun
	if rin.Joins == nil {
		rin.Joins = map[string][]string{}
	}
	if rin.Persisted != "" {
		text, ok := readPersistedText(ctx, rin.Persisted)
		rin.PersistedText = text
		j.add(readoutJoinPersisted, ok)
	}
	if ruleJoins(rule, readoutJoinHeadStamp) {
		stamp, ok := readoutHeadStamp(ctx, dir, c)
		if ok {
			rin.Joins[readoutJoinHeadStamp] = []string{stamp}
		}
		j.add(readoutJoinHeadStamp, ok)
	}
	if !rule.Match.WholeRead || !ruleOutlines(rule) {
		return j
	}
	path, ok := readout.WholeFileRead(c)
	if !ok {
		return j
	}
	edited, known := readoutFileEdited(ctx, hook, dir, path)
	j.add(readoutJoinEdited, known)
	if edited {
		rin.Edited = true
		return j
	}
	text := rin.Stdout
	if rin.Persisted != "" {
		text = rin.PersistedText // the file as printed, never the harness's preview
	}
	if text == "" {
		j.add(readoutJoinOutline, false)
		return j
	}
	octx, cancel := context.WithTimeout(ctx, readout.OutlineDeadline)
	lines, missed := readout.Outline(octx, path, text)
	cancel()
	if lines != nil {
		rin.Joins[readoutJoinOutline] = lines
	}
	j.add(readoutJoinOutline, !missed)
	return j
}

// ruleJoins reports whether rule has a join stage named name.
func ruleJoins(r *readout.Rule, name string) bool {
	for _, s := range r.Stages {
		if s.Kind == readout.StageJoin && s.Join == name {
			return true
		}
	}
	return false
}

// ruleOutlines reports whether rule has an outline stage.
func ruleOutlines(r *readout.Rule) bool {
	for _, s := range r.Stages {
		if s.Kind == readout.StageOutline {
			return true
		}
	}
	return false
}

// readPersistedText reads the harness's persisted output up to
// readoutPersistedCap; ok is false on any miss, a larger file included.
func readPersistedText(ctx context.Context, path string) (string, bool) {
	if ctx.Err() != nil {
		return "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, readoutPersistedCap+1))
	if err != nil || len(b) > readoutPersistedCap || ctx.Err() != nil {
		return "", false
	}
	return string(b), true
}

// readoutHeadStamp is the `HEAD <sha>` line of the work tree a git command
// ran in (its -C followed from dir), read as the pin check's HEAD stamp
// reads it (gitro.Head, an object name or nothing).
func readoutHeadStamp(ctx context.Context, dir string, c *shellcmd.Command) (string, bool) {
	if len(c.Pipelines) == 0 {
		return "", false
	}
	u := &c.Pipelines[len(c.Pipelines)-1].Units[0]
	args := u.Args()
	for i := 1; i+1 < len(args); i++ {
		if args[i] == "-C" {
			dir = resolveBashDir(dir, args[i+1])
			i++
			continue
		}
		if !strings.HasPrefix(args[i], "-") {
			break
		}
	}
	root := gitToplevelByStat(dir)
	if root == "" {
		return "", false
	}
	head, err := gitro.Head(ctx, root)
	if err != nil || !gitro.IsObjectName(head) {
		return "", false
	}
	if len(head) > 12 {
		head = head[:12]
	}
	return "HEAD " + head, true
}

// readoutFileEdited reports whether a whole-file read's file was edited this
// session (the edit set, editset.go) or differs from HEAD (a read-only git
// status, inside ctx). known is false when the git check failed inside a
// work tree: the read is then treated as edited, which serves it raw.
func readoutFileEdited(ctx context.Context, hook harness.HookInput, dir, path string) (edited, known bool) {
	abs := resolveBashDir(dir, path)
	if fileInEditSet(abs, hook.Cwd, hook.SessionID) {
		return true, true
	}
	if real, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		abs = filepath.Join(real, filepath.Base(abs))
	}
	root := gitToplevelByStat(filepath.Dir(abs))
	if root == "" {
		return false, true
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false, true
	}
	out, err := gitro.Output(ctx, root, "status", "--porcelain", "--untracked-files=all", "--", filepath.ToSlash(rel))
	if err != nil {
		return true, false
	}
	return len(bytes.TrimSpace(out)) > 0, true
}

// readoutRowJoins is the row's joins: those the hook ran, and those it or the
// engine missed, each sorted and distinct.
func readoutRowJoins(j readoutJoinRun, engineMissed []string) (ran, missed []string) {
	seen := map[string]bool{}
	for _, m := range append(append([]string(nil), j.missed...), engineMissed...) {
		if !seen[m] {
			seen[m] = true
			missed = append(missed, m)
		}
	}
	for _, r := range j.ran {
		if !seen[r] {
			seen[r] = true
			ran = append(ran, r)
		}
	}
	sort.Strings(ran)
	sort.Strings(missed)
	return ran, missed
}
