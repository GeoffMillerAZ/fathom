// Package receiptverify re-derives a VB-20 receipt's claim from a bare git
// checkout at the receipt's own index_commit, using golang.org/x/tools/go/
// packages (via internal/contextbench/publicloc's independent go/types
// walker) and git ALONE — never internal/mcp, never a daemon or store, so
// a stranger with the Go toolchain and this repository's git history gets
// the same verdict fathom's own gate does (ADR-034 receipts; docs/vision/
// 29-verbs-not-lookups.md §3 wave E; ADR-035 decision 4).
//
// publicloc is the right reuse here, not a new call-graph walker: it
// already never opens a fathom store and never asks a daemon (its own
// doc comment says so — it is cmd/loctruth's and the L0 index oracle's
// independent ground truth), and its GoTruth already carries exactly the
// two shapes a callers/references claim needs (CallersByName, UsesByName).
//
// Scope: LoadGoTruthTests, not LoadGoTruth. The live find_callers/
// references answers a receipt is built from correctly include _test.go
// callers and occurrences — they are real, indexed facts — so the
// independent oracle a receipt is checked against must cover the same
// scope, or every receipt for a tested symbol reports a spurious
// difference (docs/bench/readings/vb21-receipts-m11b.md: the M11b sample's
// one and only defect, 36/50 non-reproductions, all one cause).
package receiptverify

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Outcome is one Verify call's result. OK true means the receipt's claim
// reproduced exactly; OK false names the difference in Diff — never both
// zero-valued (a claim either reproduces or a difference is named).
type Outcome struct {
	OK   bool
	Diff string
}

// Verify checks out receipt.IndexCommit from the git repository at
// repoRoot via `git archive` into a fresh temporary directory, loads it
// with go/packages+go/types (publicloc.LoadGoTruthTests — independent of
// fathom's own extractor/fusion pipeline), and compares the re-derived
// facts against the receipt's own.
//
// Returns (Outcome, nil) whenever it could run at all — Outcome.OK false
// names the difference. A non-nil error means it could NOT run: repoRoot
// is not a git repository, the commit is missing, or the checkout has no
// go.mod — cmd/fathom maps every non-nil error to exit 2, never a false
// "OK" (VB-20's own contract: "exit 2 when it cannot run").
func Verify(ctx context.Context, repoRoot string, receipt core.Receipt) (Outcome, error) {
	if receipt.IndexCommit == "" {
		return Outcome{}, fmt.Errorf("receipt: index_commit is empty — nothing to check out")
	}
	if receipt.Symbol == "" {
		return Outcome{}, fmt.Errorf("receipt: symbol is empty — nothing to re-derive")
	}

	dir, cleanup, err := CheckoutModule(ctx, repoRoot, receipt.IndexCommit)
	if err != nil {
		return Outcome{}, err
	}
	defer cleanup()

	switch receipt.Kind {
	case core.ReceiptKindCallers:
		truth, loadErr := publicloc.LoadGoTruthTests(dir)
		if loadErr != nil {
			return Outcome{}, fmt.Errorf("receipt: go/types load: %w", loadErr)
		}
		return verifyCallers(receipt, truth), nil
	case core.ReceiptKindReferences:
		truth, loadErr := publicloc.LoadGoTruthTests(dir)
		if loadErr != nil {
			return Outcome{}, fmt.Errorf("receipt: go/types load: %w", loadErr)
		}
		return verifyReferences(receipt, truth), nil
	case core.ReceiptKindSymbolLocation:
		return verifySymbolLocation(dir, receipt)
	default:
		return Outcome{}, fmt.Errorf("receipt: unknown kind %q (want %q, %q or %q)",
			receipt.Kind, core.ReceiptKindCallers, core.ReceiptKindReferences, core.ReceiptKindSymbolLocation)
	}
}

// CheckoutModule checks out commit from the git repository at repoRoot via
// `git archive` into a fresh temporary directory and confirms a go.mod
// exists there — the bare-checkout precondition every receipt kind's
// go/types re-derivation shares. Verify uses it for its own three kinds;
// cmd/fathom's certification-kind verifier (TC-13) reuses it directly so
// there is exactly one implementation of "produce an independent bare
// checkout of a receipt's index_commit". A non-nil error means the
// checkout could not be produced at all (repoRoot is not a git
// repository, the commit is missing, or the checkout has no go.mod) —
// cmd/fathom maps every non-nil error to exit 2, never a false OK.
func CheckoutModule(ctx context.Context, repoRoot, commit string) (dir string, cleanup func(), err error) {
	dir, cleanup, err = archiveCommit(ctx, repoRoot, commit)
	if err != nil {
		return "", nil, err
	}
	if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("receipt: no go.mod at %s@%s — cannot re-derive with go/types (%w)", repoRoot, commit, statErr)
	}
	return dir, cleanup, nil
}

// BareSymbolName reduces a fathom SymbolKey ("service::dir/pkg.Name" or
// "service::dir/pkg.Recv.Method") — or an already-bare name, unchanged —
// to go/types' own vocabulary: "Name" or "Recv.Method", the form
// publicloc.GoTruth's CallersByName/UsesByName keys carry. A directory or
// package-path segment never contains a dot in valid Go, so stripping the
// service prefix, then the directory, then the FIRST remaining dot always
// isolates the package name from what follows it.
func BareSymbolName(s string) string {
	if i := strings.LastIndex(s, "::"); i >= 0 {
		s = s[i+2:]
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, "."); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// declaredNames flattens truth.DeclsByFile into the set of every function/
// method name go/types actually saw declared at this commit — the check a
// map miss on CallersByName/UsesByName cannot substitute for (review F2):
// a symbol that was RENAMED OR REMOVED since IndexCommit has no entry in
// either map, which is indistinguishable from a real, currently-declared
// symbol that genuinely has zero callers/uses unless declaration itself is
// checked first.
func declaredNames(truth publicloc.GoTruth) map[string]bool {
	names := make(map[string]bool)
	for _, ns := range truth.DeclsByFile {
		for _, n := range ns {
			names[n] = true
		}
	}
	return names
}

func verifyCallers(r core.Receipt, truth publicloc.GoTruth) Outcome {
	symbol := BareSymbolName(r.Symbol)
	if !declaredNames(truth)[symbol] {
		return Outcome{OK: false, Diff: fmt.Sprintf("%s is not declared at %s", symbol, r.IndexCommit)}
	}
	want := sortedCopy(r.Callers)
	got := sortedCopy(truth.CallersByName[symbol])
	if equalSlices(want, got) {
		return Outcome{OK: true}
	}
	missing := setDiff(want, got)
	extra := setDiff(got, want)
	return Outcome{OK: false, Diff: fmt.Sprintf(
		"callers of %s: receipt names %d (%v), re-derived %d (%v) — missing %v, extra %v",
		symbol, len(want), want, len(got), got, missing, extra,
	)}
}

func verifyReferences(r core.Receipt, truth publicloc.GoTruth) Outcome {
	symbol := BareSymbolName(r.Symbol)
	if !declaredNames(truth)[symbol] {
		return Outcome{OK: false, Diff: fmt.Sprintf("%s is not declared at %s", symbol, r.IndexCommit)}
	}
	want := sortedCopy(r.Files)
	got := sortedCopy(truth.UsesByName[symbol])
	if equalSlices(want, got) {
		return Outcome{OK: true}
	}
	missing := setDiff(want, got)
	extra := setDiff(got, want)
	return Outcome{OK: false, Diff: fmt.Sprintf(
		"files mentioning %s: receipt names %d (%v), re-derived %d (%v) — missing %v, extra %v",
		symbol, len(want), want, len(got), got, missing, extra,
	)}
}

func verifySymbolLocation(dir string, r core.Receipt) (Outcome, error) {
	if r.File == "" {
		return Outcome{}, fmt.Errorf("receipt: symbol_location claim has no file")
	}
	full := filepath.Join(dir, r.File)
	src, err := os.ReadFile(full) //nolint:gosec — full is joined from the archived checkout, not caller input
	if err != nil {
		return Outcome{}, fmt.Errorf("receipt: read %s at this commit: %w", r.File, err)
	}
	spans, err := publicloc.FuncSpans(src, full)
	if err != nil {
		return Outcome{}, fmt.Errorf("receipt: parse %s: %w", r.File, err)
	}
	symbol := BareSymbolName(r.Symbol)
	for _, s := range spans {
		if s.Name != symbol {
			continue
		}
		if s.Start == r.Line {
			return Outcome{OK: true}, nil
		}
		return Outcome{OK: false, Diff: fmt.Sprintf(
			"%s in %s: receipt names line %d, re-derived line %d", symbol, r.File, r.Line, s.Start,
		)}, nil
	}
	return Outcome{OK: false, Diff: fmt.Sprintf("%s: not declared in %s at this commit", symbol, r.File)}, nil
}

func sortedCopy(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return out
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// setDiff returns the sorted elements of a not present in b.
func setDiff(a, b []string) []string {
	inB := make(map[string]bool, len(b))
	for _, s := range b {
		inB[s] = true
	}
	var out []string
	for _, s := range a {
		if !inB[s] {
			out = append(out, s)
		}
	}
	return out
}

// archiveCommit exports commit from the git repository at repoRoot via
// `git archive`, extracting it into a fresh temporary directory — a bare
// checkout at that exact commit, independent of the caller's working
// tree. cleanup removes the temporary directory; call it even on error
// (archiveCommit only ever returns a non-empty dir alongside a non-nil
// cleanup).
func archiveCommit(ctx context.Context, repoRoot, commit string) (dir string, cleanup func(), err error) {
	if err := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--git-dir").Run(); err != nil {
		return "", nil, fmt.Errorf("receipt: %s is not a git repository: %w", repoRoot, err)
	}
	if err := exec.CommandContext(ctx, "git", "-C", repoRoot, "cat-file", "-e", commit+"^{commit}").Run(); err != nil {
		return "", nil, fmt.Errorf("receipt: commit %s not found in %s: %w", commit, repoRoot, err)
	}

	dir, err = os.MkdirTemp("", "fathom-receipt-*")
	if err != nil {
		return "", nil, fmt.Errorf("receipt: temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "archive", "--format=tar", commit)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("receipt: git archive: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("receipt: git archive: %w", err)
	}
	extractErr := extractTar(pipe, dir)
	waitErr := cmd.Wait()
	if extractErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("receipt: extract archive of %s: %w", commit, extractErr)
	}
	if waitErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("receipt: git archive %s: %w", commit, waitErr)
	}
	return dir, cleanup, nil
}

// extractTar extracts a tar stream (as `git archive --format=tar` emits)
// into dir. Every entry is confined under dir — a path that would escape
// it (e.g. via "..") is refused rather than followed, though a commit
// git itself produced never carries one.
func extractTar(r io.Reader, dir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dir, hdr.Name) //nolint:gosec — Clean+containment check below
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) && target != filepath.Clean(dir) {
			return fmt.Errorf("archive entry %q escapes the checkout directory", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode&0o777)) //nolint:gosec — target is containment-checked above
			if err != nil {
				return err
			}
			if _, err := io.CopyN(f, tr, hdr.Size); err != nil && err != io.EOF { //nolint:gosec — hdr.Size comes from git's own archive, not attacker input
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			// Symlinks and other special entries: a Go module tree never
			// legitimately needs one for this purpose — skip rather than
			// fail the whole extraction.
		}
	}
}
