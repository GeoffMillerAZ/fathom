package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
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
			lp := wiring.LedgerPath(root)
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
func (w *initWrites) printDryRun(stdout io.Writer, root, service string) {
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

	printInitCost(stdout, root, service, semantic.Compiler{})
}

// printInitCost sizes the initial compile, using the compiler's own section
// and prompt accounting over the local corpus. The providers are never opened.
// A fresh in-memory store gives first-compile estimates without touching a
// daemon or a persisted index. Supplying a compiler also lets callers exercise
// the sizing path with rejecting providers.
func printInitCost(stdout io.Writer, root, service string, compiler semantic.Compiler) {
	fmt.Fprintln(stdout, "\nCost")
	for _, facet := range []string{"entry-point", "runs", "test-entry"} {
		fmt.Fprintf(stdout, "  %s: $0; command: fathom init --path %s --service %s\n", facet, shellQuote(root), shellQuote(service))
	}
	ctx := semantic.WithSizingEstimate(context.Background())
	conn := connector.NewLocalConnector()
	refs, discoverErr := conn.Discover(ctx, core.ServiceRef{ID: service, Path: root})
	model := compileModel()
	if loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root)); err == nil {
		if profile, _, err := config.ResolveProfile(loaded.Config, loaded.Provenance, ""); err == nil {
			model = compileModelForProfile(profile)
		}
	}
	compiler.St = store.NewMemoryStore()
	defer compiler.St.Close()
	compiler.Now = func() time.Time { return time.Unix(0, 0).UTC() }
	compiler.Model = model
	var priceKnown bool
	compiler.InRatePerKTok, compiler.OutRatePerKTok, priceKnown = rateForModel(model)
	for _, name := range semantic.DefaultLayersFor(root) {
		command := "fathom compile --service " + shellQuote(service) + " --layer " + shellQuote(name) + " --dry-run=false"
		def, err := semantic.LoadLayerIn(root, name)
		if err != nil || discoverErr != nil {
			fmt.Fprintf(stdout, "  %s: tokens: unknown (corpus unavailable); price: unknown; command: %s\n", name, command)
			continue
		}
		files := make([]core.SourceFile, 0, len(refs))
		for _, ref := range refs {
			files = append(files, core.SourceFile{Path: ref.Path})
		}
		docs := semantic.BuildCompileCorpus(root, files, def)
		for i := range docs {
			if docs[i].Content != nil {
				continue // rules sources and root-guide extras are already read
			}
			docs[i].Content, err = conn.Fetch(ctx, core.SourceRef{Service: service, Path: docs[i].Path})
			if err != nil {
				break
			}
		}
		if err != nil {
			fmt.Fprintf(stdout, "  %s: tokens: unknown (corpus unreadable); price: unknown; command: %s\n", name, command)
			continue
		}
		compiler.Layer = def
		run, err := compiler.Compile(ctx, semantic.CompileInput{ServiceID: service, Docs: docs, DryRun: true})
		if err != nil {
			fmt.Fprintf(stdout, "  %s: tokens: unknown (sizing unavailable); price: unknown; command: %s\n", name, command)
			continue
		}
		price := "unknown"
		if priceKnown {
			price = fmt.Sprintf("$%.6f-$%.6f", run.EstCostLowUSD, run.EstCostHighUSD)
		}
		fmt.Fprintf(stdout, "  %s: input %d tokens; output %d-%d tokens; price: %s (model %s); command: %s\n", name, run.TokensIn, run.EstTokensOutLow, run.EstTokensOutHigh, price, model, command)
	}
	// These are the commands that actually reach each decision site. The
	// coverage test holds this table to AllSites; opt-in differs from provider
	// configuration because most sites are already in the default allowlist.
	commands := map[string]string{
		decide.SiteRoute:             "fathom serve-http",
		decide.SiteCompilePrefilter:  "fathom compile --service " + shellQuote(service) + " --dry-run=false",
		decide.SiteObligationsRerank: "fathom serve-http",
		decide.SiteCertifySuggest:    "fathom certify suggest --service " + shellQuote(service),
		decide.SiteAssess:            "fathom assess --project " + shellQuote(root),
		decide.SitePreflight:         "fathom plans context <plan.md> --row <id> --service " + shellQuote(service) + " --store <db> --preflight --score",
	}
	sites := append([]string(nil), decide.AllSites...)
	sort.Strings(sites)
	for _, site := range sites {
		command, ok := commands[site]
		if !ok {
			command = "unknown"
		}
		gate := "site allowed by default"
		if !decide.SiteAllowed(config.DefaultDecide, site) {
			gate = "sites including " + site
		}
		if site == decide.SiteAssess {
			gate += "; fathom capabilities set assess on"
		}
		if site == decide.SiteObligationsRerank {
			gate += "; [decide] rank_within_tier = true"
		}
		fmt.Fprintf(stdout, "  %s: unpriced (no measurement yet); command: %s (requires [decide] provider configured; %s; init adds no decision opt-in; provider none: no call)\n", site, command, gate)
	}
}
