package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// initWrites is the one door every file write `fathom init` makes goes
// through (RH-11, docs/design/reach.md §5.1). Each writer decides exactly
// as it always has — stat, compare, plan — and then hands the write to
// this door: a real run performs it, a dry run records it and stops. So
// the list `fathom init --dry-run` prints is produced by the same
// functions, deciding the same way, as the real run's writes: it is never
// a second, hand-kept list of what init touches.
//
// A nil *initWrites is the real run with nothing recorded, for the callers
// outside init (reconcile, the single-file stanza writer, tests).
type initWrites struct {
	dry     bool
	touched []initTouch
	// actions are effects a dry run skips that are not a file here: the
	// daemon ingest (the daemon's own store) and a SCIP import.
	actions []string
}

// initTouch is one file a run writes, changes, links or removes.
type initTouch struct {
	Path   string // absolute
	Action string // create | update | link | remove
}

// initLedgerFile is the wiring engine's ownership ledger, repo-relative
// (internal/wiring's ledgerRelPath): an apply that writes an owned key,
// file or link records it there.
const initLedgerFile = ".fathom/owned.json"

func (w *initWrites) isDry() bool { return w != nil && w.dry }

// record notes path; a later record of the same path replaces it (a stale
// index removed and then rewritten is one update, not a remove).
func (w *initWrites) record(path, action string) {
	if w == nil {
		return
	}
	path = filepath.Clean(path)
	for i := range w.touched {
		if w.touched[i].Path == path {
			w.touched[i].Action = action
			return
		}
	}
	w.touched = append(w.touched, initTouch{Path: path, Action: action})
}

// writeAction is create when nothing is at path yet, else update.
func writeAction(path string) string {
	if _, err := os.Lstat(path); err != nil {
		return "create"
	}
	return "update"
}

// note records a skipped effect that is not a file.
func (w *initWrites) note(action string) {
	if w != nil {
		w.actions = append(w.actions, action)
	}
}

// writeFile writes data at path with perm, or records that it would.
func (w *initWrites) writeFile(path string, data []byte, perm os.FileMode) error {
	w.record(path, writeAction(path))
	if w.isDry() {
		return nil
	}
	return os.WriteFile(path, data, perm)
}

// mkdirAll creates dir; a dry run creates nothing (a directory is not a
// file it lists, and a dry run writes none).
func (w *initWrites) mkdirAll(dir string) error {
	if w.isDry() {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// symlink links path at target, or records that it would.
func (w *initWrites) symlink(target, path string) error {
	w.record(path, "link")
	if w.isDry() {
		return nil
	}
	return os.Symlink(target, path)
}

// remove removes path, or records that it would; a path that is not there
// is nothing to remove.
func (w *initWrites) remove(path string) error {
	if _, err := os.Lstat(path); err != nil {
		if w.isDry() || os.IsNotExist(err) {
			return nil
		}
		return err
	}
	w.record(path, "remove")
	if w.isDry() {
		return nil
	}
	return os.Remove(path)
}

// applyWiring is wiring.ApplyWiring through the door. A dry run computes
// the plan — PlanWiring is ApplyWiring's own first step — and records each
// file its apply items would write, the ownership ledger when an owned key,
// file or link is among them, and the receipts log every apply appends to.
// The result carries the plan's apply items as Applied, as a real apply
// reports them.
func (w *initWrites) applyWiring(root string, spec wiring.Spec) (wiring.Result, error) {
	if !w.isDry() {
		return wiring.ApplyWiring(root, spec, wiring.ApplyOptions{Now: time.Now})
	}
	plan, err := wiring.PlanWiring(root, spec)
	if err != nil {
		return wiring.Result{}, err
	}
	todo := plan.Applicable()
	res := wiring.Result{Plan: plan, Applied: todo, Blocked: plan.Blocked(), Skipped: plan.Skipped()}
	owned := false
	for _, it := range todo {
		p := filepath.Join(root, filepath.FromSlash(it.File))
		if it.Kind == wiring.KindLink {
			w.record(p, "link")
		} else {
			w.record(p, writeAction(p))
		}
		owned = owned || it.Kind != wiring.KindRegion
	}
	if len(todo) > 0 {
		if owned {
			lp := filepath.Join(root, filepath.FromSlash(initLedgerFile))
			w.record(lp, writeAction(lp))
		}
		rp := wiring.ReceiptsPath(root)
		w.record(rp, writeAction(rp))
	}
	return res, nil
}

// displayPath names p for the reader: repo-relative under root, "~/" under
// home, absolute otherwise.
func displayPath(p, root, home string) string {
	if rel, err := filepath.Rel(root, p); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	if home != "" {
		if rel, err := filepath.Rel(home, p); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return p
}

// printDryRun prints what a dry run recorded: one "would <action> <path>"
// line per file — the repository's first, then the home's, then any other,
// each group sorted — then the skipped effects that are not files, then
// the count.
func (w *initWrites) printDryRun(stdout io.Writer, root string) {
	home, _ := os.UserHomeDir()
	type line struct {
		group  int
		action string
		shown  string
	}
	lines := make([]line, 0, len(w.touched))
	for _, t := range w.touched {
		shown := displayPath(t.Path, root, home)
		group := 0
		switch {
		case strings.HasPrefix(shown, "~/"):
			group = 1
		case filepath.IsAbs(shown):
			group = 2
		}
		lines = append(lines, line{group, t.Action, shown})
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].group != lines[j].group {
			return lines[i].group < lines[j].group
		}
		return lines[i].shown < lines[j].shown
	})
	touched := lines
	for _, l := range lines {
		fmt.Fprintf(stdout, "would %s %s\n", l.action, l.shown)
	}
	for _, a := range w.actions {
		fmt.Fprintf(stdout, "would %s\n", a)
	}
	switch len(touched) {
	case 0:
		fmt.Fprintln(stdout, "dry run: init would write or change no file; nothing was written")
	case 1:
		fmt.Fprintln(stdout, "dry run: init would write or change 1 file; nothing was written")
	default:
		fmt.Fprintf(stdout, "dry run: init would write or change %d files; nothing was written\n", len(touched))
	}
}
