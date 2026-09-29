package main

// console_work_signals.go — the readers /work adds to its proof join
// (docs/design/work-intelligence.md §3): the CI summary files a
// repository names (WI-2w). Every read is bounded, every file it could not
// read or parse is named on the page, and a figure it did not read is an
// absence with its reason — never a zero (I38).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
)

// workCIMaxBytes bounds one CI summary file's read.
const workCIMaxBytes = 8 << 20

// workRepoConfig reads the repository's own .fathom/config.toml (its
// `[zones]` and `[work]` tables). An absent file is an empty config.
func workRepoConfig(root string) (config.Config, error) {
	if root == "" {
		return config.Config{}, nil
	}
	loaded, err := config.Load("", workConfigPath(root))
	if err != nil {
		return config.Config{}, err
	}
	return loaded.Config, nil
}

func workConfigPath(root string) string { return filepath.Join(root, ".fathom", "config.toml") }

// workCIPath is a named CI summary's path: absolute as written, else
// relative to the repository root.
func workCIPath(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

// workCIRuns reads each CI summary file cfg names (console.ParseCISummary)
// and binds every run to the proving commit it built, in the spelling the
// proof history used (a ledger abbreviates HEAD; a workflow run carries
// the full head_sha). It returns the runs and one line per file saying
// what was read or why not. A run bound to no proving commit is kept: U1
// then names the changes it could not bind.
func workCIRuns(root string, sums []config.CISummary, cfgErr error, changes []console.ProvenChange) ([]console.CIRun, []string) {
	if cfgErr != nil {
		return nil, []string{"CI minutes: .fathom/config.toml could not be read: " + cfgErr.Error()}
	}
	if len(sums) == 0 {
		return nil, []string{"CI minutes: no CI summary file named ([[work.ci_summaries]] in .fathom/config.toml)"}
	}
	spellings := map[string]bool{}
	for _, c := range changes {
		if c.Commit != "" {
			spellings[c.Commit] = true
		}
	}
	var runs []console.CIRun
	var read []string
	for _, s := range sums {
		path := workCIPath(root, s.Path)
		fi, err := os.Stat(path)
		switch {
		case err != nil:
			read = append(read, fmt.Sprintf("%s: not read: %v", s.Path, err))
			continue
		case fi.Size() > workCIMaxBytes:
			read = append(read, fmt.Sprintf("%s: not read: %d bytes, over the %d MiB bound", s.Path, fi.Size(), workCIMaxBytes>>20))
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			read = append(read, fmt.Sprintf("%s: not read: %v", s.Path, err))
			continue
		}
		parsed, err := console.ParseCISummary(s.Path, data, s.Commit)
		if err != nil {
			read = append(read, fmt.Sprintf("%s: not parsed: %v", s.Path, err))
			continue
		}
		minutes, bound := 0.0, 0
		var commits []string
		for _, r := range parsed {
			minutes += r.Minutes
			commits = append(commits, shortCommit(r.Commit))
			matched := false
			for sp := range spellings {
				if strings.HasPrefix(sp, r.Commit) || strings.HasPrefix(r.Commit, sp) {
					rr := r
					rr.Commit = sp
					runs = append(runs, rr)
					matched = true
				}
			}
			if matched {
				bound++
			} else {
				runs = append(runs, r)
			}
		}
		read = append(read, fmt.Sprintf("%s: %s, %.1f CI minutes, built %s (%d bound to a proving commit)",
			s.Path, plural(len(parsed), "run"), minutes, strings.Join(commits, ", "), bound))
	}
	return runs, read
}

// workCIWatermarks is the memo key part for the files cfg names, so a
// changed summary is read again.
func workCIWatermarks(root string, cfg config.Config) string {
	var b strings.Builder
	for _, s := range cfg.Work.CISummaries {
		b.WriteString(s.Path + "@" + fileWatermark(workCIPath(root, s.Path)) + ";")
	}
	return b.String()
}
