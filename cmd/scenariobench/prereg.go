package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/benchprep"
	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// checkRunPrereg is `run -prereg`'s gate (BX-62). It refuses -rescore-lost
// outright (D12: a rescored cell has no session start time), then runs
// D13's checks in order, each refused by name so each is testable alone:
//
//  1. the prereg resolves under <top>/docs/bench/prereg/, where <top> is
//     the git repository that holds it;
//  2. LoadPrereg accepts it;
//  3. it is committed and matches `git show HEAD:<path>` (a modified,
//     untracked or ignored file is refused as such);
//  4. `git status --porcelain` is clean;
//  5. -root, when set, names that same top level, so a clean prereg
//     repository never covers a dirty fathom root;
//  6. no stored cell under a planned key carries another prereg's sha, or
//     none (D2): the sha is provenance, not a cell-key invariant, so two
//     preregs over the same invariants address the same cell, and only a
//     cell filed under this prereg may be borrowed.
//
// It returns the prereg's sha256. Every git call is local (status, show,
// rev-parse) under the scrubbed environment (I14).
func checkRunPrereg(preregPath, root, cellsDir string, rescoreLost bool, cells []cellPlan) (string, error) {
	if rescoreLost {
		return "", errors.New("-prereg refuses -rescore-lost: a rescored cell is rebuilt from raw streams and has no session start time (D12)")
	}
	abs, err := filepath.Abs(preregPath)
	if err != nil {
		return "", fmt.Errorf("-prereg %s: %w", preregPath, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("-prereg %s: %w", preregPath, err)
	}
	top, err := preregGitOutput(filepath.Dir(resolved), "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("-prereg %s is not inside a git repository: %w", preregPath, err)
	}
	if top, err = filepath.EvalSymlinks(strings.TrimSpace(top)); err != nil {
		return "", fmt.Errorf("-prereg %s: resolving its repository: %w", preregPath, err)
	}
	rel, err := filepath.Rel(top, resolved)
	if err != nil {
		return "", fmt.Errorf("-prereg %s: %w", preregPath, err)
	}
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "docs/bench/prereg/") {
		return "", fmt.Errorf("-prereg %s is not under its repository's docs/bench/prereg/ (D13)", rel)
	}

	_, sha, err := benchprep.LoadPrereg(resolved)
	if err != nil {
		return "", err
	}

	status, err := preregGitOutput(top, "status", "--porcelain=v1", "-z", "--ignored=matching", "--untracked-files=all", "--", rel)
	if err != nil {
		return "", err
	}
	for _, e := range preregStatusEntries(status) {
		switch e.code {
		case "??":
			return "", fmt.Errorf("-prereg %s is untracked, so it is not in HEAD: commit it before the run (D13)", rel)
		case "!!":
			return "", fmt.Errorf("-prereg %s is ignored by git, so it is not in HEAD: commit it before the run (D13)", rel)
		default:
			return "", fmt.Errorf("-prereg %s is modified against HEAD (status %q): commit it before the run (D13)", rel, e.code)
		}
	}
	head, err := preregGitBytes(top, "show", "HEAD:"+rel)
	if err != nil {
		return "", fmt.Errorf("-prereg %s is not committed at HEAD: %w (D13)", rel, err)
	}
	if headSHA := preregSHA256Hex(head); headSHA != sha {
		return "", fmt.Errorf("-prereg %s differs from HEAD (sha256 %s on disk, %s at HEAD) (D13)", rel, sha, headSHA)
	}

	dirty, err := preregGitOutput(top, "status", "--porcelain=v1", "-z", "--untracked-files=normal")
	if err != nil {
		return "", err
	}
	if entries := preregStatusEntries(dirty); len(entries) > 0 {
		paths := make([]string, len(entries))
		for i, e := range entries {
			paths[i] = e.code + " " + e.path
		}
		sort.Strings(paths)
		return "", fmt.Errorf("-prereg's repository is dirty, %d path(s): %s; a run under a prereg needs a clean tree (D13)", len(paths), strings.Join(paths, ", "))
	}

	if root != "" {
		r, err := filepath.Abs(root)
		if err == nil {
			r, err = filepath.EvalSymlinks(r)
		}
		if err != nil {
			return "", fmt.Errorf("-root %s: %w", root, err)
		}
		if r != top {
			return "", fmt.Errorf("-root %s is not the repository that holds -prereg (%s): a clean prereg repository must not cover another root (D13)", root, top)
		}
	}

	if err := scanPreregCells(cellsDir, sha, cells); err != nil {
		return "", err
	}
	return sha, nil
}

// scanPreregCells refuses, before any session starts, every stored cell
// under a planned key (each planned trial and any other trial stored under
// that key) whose prereg_sha256 is not sha, an unstamped cell included (D2).
func scanPreregCells(cellsDir, sha string, cells []cellPlan) error {
	var foreign []string
	seen := map[string]bool{}
	for _, cp := range cells {
		trials := []int{cp.trial}
		stored, err := scenariobench.StoredScenarioTrials(cellsDir, cp.key)
		if err != nil {
			return fmt.Errorf("scanning the cell store for %s/%s: %w", cp.task.ID, cp.arm, err)
		}
		trials = append(trials, stored...)
		for _, trial := range trials {
			id := fmt.Sprintf("%s/t%d", cp.key, trial)
			if seen[id] {
				continue
			}
			seen[id] = true
			cell, ok, err := scenariobench.BorrowScenarioCell(cellsDir, cp.key, trial)
			if err != nil {
				return fmt.Errorf("scanning cell %s/%s/t%d: %w", cp.task.ID, cp.arm, trial, err)
			}
			if !ok || cell.PreregSHA256 == sha {
				continue
			}
			stamp := cell.PreregSHA256
			if stamp == "" {
				stamp = "unstamped"
			}
			foreign = append(foreign, fmt.Sprintf("%s/%s/t%d (cell %s, prereg_sha256 %s)", cp.task.ID, cp.arm, trial, cp.key, stamp))
		}
	}
	if len(foreign) > 0 {
		sort.Strings(foreign)
		return fmt.Errorf("-prereg %s: %d stored cell(s) under -cells %s were not run under this prereg: %s; borrowing is allowed only under the same prereg, so use a fresh -cells directory (D2)",
			sha, len(foreign), cellsDir, strings.Join(foreign, "; "))
	}
	return nil
}

// ledgerPreregStamp returns the prereg sha every filed cell shares, "" when
// none carries one. Cells under two shas, or stamped beside unstamped, are
// refused with every cell named, so a row never claims one prereg for cells
// run under another (BX-62).
func ledgerPreregStamp(cells []scenariobench.CellResult) (string, error) {
	byStamp := map[string][]string{}
	for _, c := range cells {
		byStamp[c.PreregSHA256] = append(byStamp[c.PreregSHA256], fmt.Sprintf("%s/%s/t%d", c.TaskID, c.Arm, c.Trial))
	}
	if len(byStamp) <= 1 {
		for stamp := range byStamp {
			return stamp, nil
		}
		return "", nil
	}
	stamps := make([]string, 0, len(byStamp))
	for stamp := range byStamp {
		stamps = append(stamps, stamp)
	}
	sort.Strings(stamps)
	parts := make([]string, len(stamps))
	for i, stamp := range stamps {
		names := byStamp[stamp]
		sort.Strings(names)
		label := stamp
		if label == "" {
			label = "unstamped"
		}
		parts[i] = fmt.Sprintf("%s: %s", label, strings.Join(names, ", "))
	}
	return "", fmt.Errorf("refusing to file cells run under %d different preregs (prereg_sha256): %s", len(stamps), strings.Join(parts, "; "))
}

type preregStatusEntry struct{ code, path string }

// preregStatusEntries parses `git status --porcelain=v1 -z`; a rename or
// copy's second (original) path is folded into its entry.
func preregStatusEntries(out string) []preregStatusEntry {
	var entries []preregStatusEntry
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		e := preregStatusEntry{code: f[:2], path: f[3:]}
		if e.code[0] == 'R' || e.code[0] == 'C' {
			i++
		}
		entries = append(entries, e)
	}
	return entries
}

// preregGitOutput runs one local git subcommand in dir under the scrubbed
// environment and returns its stdout.
func preregGitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = scenariobench.GitEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// preregGitBytes is preregGitOutput returning stdout's bytes unchanged.
func preregGitBytes(dir string, args ...string) ([]byte, error) {
	out, err := preregGitOutput(dir, args...)
	return []byte(out), err
}

func preregSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
