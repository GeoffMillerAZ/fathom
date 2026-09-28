package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/specdialect"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// convergeTimeout bounds the converge pass's git and store reads for one
// spec: one diff of one range and the changed files' symbols.
const convergeTimeout = 60 * time.Second

// buildConverge is `fathom verify --converge` for one spec file (ADR-049,
// spec-interop design §5): the three sections from the parsed spec, its
// verify report and — when a store is at hand — the symbols changed on the
// spec's branch range. base is --base, or "" for the spec's own creation
// commit. The unclaimed-code section is left unevaluated, with the reason,
// when there is no store or no base.
func buildConverge(pf, dialect string, plan verify.Plan, rep verify.Report, service, storePath, base string) specdialect.ConvergeReport {
	ctx, cancel := context.WithTimeout(context.Background(), convergeTimeout)
	defer cancel()
	var (
		changed []specdialect.ChangedSymbol
		note    string
	)
	abs, err := filepath.Abs(pf)
	root := ""
	if err == nil {
		root = gitToplevel(filepath.Dir(abs))
	}
	switch {
	case root == "":
		note = "the spec is not inside a git work tree, so it has no branch range"
	default:
		if base == "" {
			base = specCreationCommit(ctx, root, abs)
		}
		switch {
		case base == "":
			note = "the spec file is not committed yet, so it has no creation commit — pass --base <ref>"
		case storePath == "":
			note = "the unclaimed-code pass reads the index's symbol ranges — run with --store"
		default:
			changed, err = convergeChangedSymbols(ctx, root, base, service, storePath)
			if err != nil {
				note = err.Error()
				changed = nil
			}
		}
	}
	r := specdialect.Converge(plan, rep, changed)
	r.Spec, r.Dialect, r.Base, r.UnclaimedNote = pf, dialect, base, note
	return r
}

// specCreationCommit is the commit that added the spec file (the oldest
// `git log --diff-filter=A` entry for it), or "" when it has none.
func specCreationCommit(ctx context.Context, root, abs string) string {
	rel, ok := relativeToRoot(root, abs)
	if !ok {
		return ""
	}
	out, err := gitro.Output(ctx, root, "log", "--diff-filter=A", "--format=%H", "--", filepath.ToSlash(rel))
	if err != nil {
		return ""
	}
	lines := strings.Fields(string(out))
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

// convergeChangedSymbols reads the range's changed hunks (`git diff -U0
// <base>` against the working tree, so uncommitted work counts) and the
// symbols the index holds for exactly those files, and returns the symbols
// whose definitions overlap a hunk. Bounded by the changed-file set: no
// whole-index read (design §7).
func convergeChangedSymbols(ctx context.Context, root, base, service, storePath string) ([]specdialect.ChangedSymbol, error) {
	diff, err := gitro.Output(ctx, root, "diff", "--no-ext-diff", "--no-color", "--no-renames", "--relative", "-U0", base)
	if err != nil {
		return nil, fmt.Errorf("git diff -U0 %s: %v", base, err)
	}
	hunks := specdialect.ParseHunks(string(diff))
	if len(hunks) == 0 {
		return nil, nil
	}
	if _, err := os.Stat(storePath); err != nil {
		return nil, fmt.Errorf("store %q: %v", storePath, err)
	}
	st, err := store.Open(storePath)
	if err != nil {
		return nil, fmt.Errorf("opening store %q: %v", storePath, err)
	}
	defer st.Close()

	files := make([]string, 0, len(hunks))
	for f := range hunks {
		files = append(files, f)
	}
	sort.Strings(files)
	var spans []specdialect.SymbolSpan
	for _, f := range files {
		names, err := st.SymbolNamesForSource(ctx, service, f)
		if err != nil {
			return nil, fmt.Errorf("symbols for %s: %v", f, err)
		}
		for _, name := range names {
			obs, err := st.ObservationsForSymbol(ctx, service, name)
			if err != nil {
				return nil, fmt.Errorf("observations for %s: %v", name, err)
			}
			for _, o := range obs {
				if file, _, _ := strings.Cut(o.SourceRef, ":"); file != f || o.Line == 0 {
					continue
				}
				spans = append(spans, specdialect.SymbolSpan{Name: o.Symbol, Package: o.Package, File: f, Line: o.Line, EndLine: o.EndLine})
			}
		}
	}
	return specdialect.ChangedSymbols(hunks, spans), nil
}

// writeFixupsFile rewrites pf's converge block (and only it) from r, via
// a temp file renamed into place so a crash never leaves a half file.
func writeFixupsFile(pf, dialect string, raw []byte, plan verify.Plan, r specdialect.ConvergeReport) error {
	out, err := specdialect.WriteFixups(dialect, raw, plan, r)
	if err != nil {
		return fmt.Errorf("plan %q: %w", pf, err)
	}
	if string(out) == string(raw) {
		return nil
	}
	info, err := os.Stat(pf)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(pf), ".fathom-converge-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), info.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), pf)
}
