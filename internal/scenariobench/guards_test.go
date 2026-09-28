package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// guards_test.go is SB-24's anchor: TestGuardsCorpusDetectorsExist proves
// testdata/scenariobench/guards-fathom.json is not just prose — every
// detector command it names is something this tree can actually run, and
// every rule citation actually points at the invariant it claims to.

const guardsCorpusPath = "../../testdata/scenariobench/guards-fathom.json"

// moduleRoot is the repo root from this package's location
// (internal/scenariobench), for detectors that walk the tree.
const moduleRoot = "../.."

func loadGuardsCorpus(t *testing.T) Corpus {
	t.Helper()
	return loadGuardsCorpusAt(t, guardsCorpusPath)
}

// guardsCorpora are every committed guards corpus; a second one
// (guards-adr.json, SB-49) must pass the same detector gate as the first.
func guardsCorpora(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("../../testdata/scenariobench/guards-*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no guards corpora found: %v", err)
	}
	sort.Strings(paths)
	return paths
}

func loadGuardsCorpusAt(t *testing.T, path string) Corpus {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read guards corpus: %v", err)
	}
	var c Corpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("parse guards corpus: %v", err)
	}
	return c
}

// allowedDetectorVerbs are the invariant-enforcing command families a
// guards-corpus detector may name (docs/intent/invariants.md's own gate
// vocabulary: go test, task <name>, cue vet, bash scripts/*.sh). Anything
// else must at least resolve on PATH — never a command this repo cannot
// actually run.
var allowedDetectorVerbs = map[string]bool{"go": true, "task": true, "cue": true, "bash": true}

// runDetectorRe extracts a `go test <pkgDir> ... -run <Name>` detector's
// package directory and test name.
func parseGoTestDetector(cmd string) (pkgDir, testName string, ok bool) {
	fields := strings.Fields(cmd)
	if len(fields) < 2 || fields[0] != "go" || fields[1] != "test" {
		return "", "", false
	}
	for i := 2; i < len(fields); i++ {
		f := fields[i]
		if strings.HasPrefix(f, "./") {
			pkgDir = strings.TrimPrefix(f, "./")
		}
		if f == "-run" && i+1 < len(fields) {
			testName = strings.Trim(fields[i+1], `'"`)
		}
	}
	return pkgDir, testName, testName != ""
}

// testFuncExistsInTree reports whether some "func <name>(" declaration
// exists among the *_test.go files under root/pkgDir (root's whole tree,
// skipping skipTreeDir, when pkgDir is ""). Unlike findFuncInTree (which
// deliberately excludes _test.go — it locates PRODUCTION symbols for
// FalseDoneRate/ContractDrift), a guards-corpus detector names a Test*
// function, which lives only in a _test.go file.
func testFuncExistsInTree(root, pkgDir, name string) (bool, error) {
	var files []string
	if pkgDir != "" {
		dir := filepath.Join(root, filepath.FromSlash(pkgDir))
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.go") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	} else {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if path != root && skipTreeDir(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return false, err
		}
	}

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		spans, err := publicloc.FuncSpans(src, path)
		if err != nil {
			continue
		}
		for _, sp := range spans {
			if sp.Name == name {
				return true, nil
			}
		}
	}
	return false, nil
}

// TestGuardsCorpusDetectorsExist is SB-24's anchor: every task's detector
// command is runnable in this tree (a go/task/cue/bash verb, or something
// on PATH), every `go test -run TestX` detector names a TestX this tree
// actually declares, and every rule's `source` citation — "file#Lnn" or,
// for an ADR record, the heading-anchored "file#ADR-nnn" (item 3, wave N8,
// ResolveCitationLine in citation.go) — names a line that actually carries
// that rule's id.
func TestGuardsCorpusDetectorsExist(t *testing.T) {
	for _, path := range guardsCorpora(t) {
		t.Run(filepath.Base(path), func(t *testing.T) { checkGuardsCorpusDetectors(t, loadGuardsCorpusAt(t, path)) })
	}
}

func checkGuardsCorpusDetectors(t *testing.T, c Corpus) {
	t.Helper()
	// >= 6: guards-fathom.json and guards-adr.json carry 8 rules each;
	// guards-m1.json (RC-50) is a deliberately smaller, harder-to-derive
	// six — each rule placed far from its edit site by construction, which
	// costs more per task to source honestly than a wider, easier eight.
	if len(c.Tasks) < 6 {
		t.Fatalf("guards corpus has %d tasks, want >= 6", len(c.Tasks))
	}
	if c.Criterion.ID == "" {
		t.Fatalf("guards corpus has no criterion")
	}

	for _, task := range c.Tasks {
		task := task
		t.Run(task.ID, func(t *testing.T) {
			if task.Rule == nil {
				t.Fatalf("task %s: no rule", task.ID)
			}
			if len(task.Oracle.Detectors) == 0 {
				t.Fatalf("task %s: no detectors", task.ID)
			}
			for _, d := range task.Oracle.Detectors {
				if d.ExpectExit != 0 {
					t.Errorf("task %s: detector %s expect_exit = %d, want 0", task.ID, d.Name, d.ExpectExit)
				}
				fields := strings.Fields(d.Command)
				if len(fields) == 0 {
					t.Fatalf("task %s: detector %s has an empty command", task.ID, d.Name)
				}
				verb := fields[0]
				if !allowedDetectorVerbs[verb] {
					if _, err := exec.LookPath(verb); err != nil {
						t.Errorf("task %s: detector %s's command %q starts with %q, which is none of go/task/cue/bash and is not on PATH", task.ID, d.Name, d.Command, verb)
					}
				}
				if pkgDir, testName, ok := parseGoTestDetector(d.Command); ok {
					if task.Disposition == "dropped" {
						// A dropped task never runs (PlanCells skips it), so
						// its detector need not resolve in this tree — e.g.
						// guards-adr-028, whose internal/memory SR-13
						// (ADR-053) retired and which exists only at its
						// base_commit. The drop must still say why.
						if task.Reason == "" {
							t.Errorf("task %s: disposition dropped with no reason", task.ID)
						}
						continue
					}
					if len(task.Subject.Setup) > 0 {
						// RC-50: a task whose subject.setup materializes its
						// own edit-site scaffold (a package this repo does
						// not actually commit, e.g. guards-m1's
						// fanout-determinism/store-search-accelerator) has
						// nothing for a static tree search to find — the
						// package exists only inside a prepared worktree.
						// TestGuardsM1ViolationProbesFireAndClear is the
						// real proof for these: it runs task.Subject.Setup
						// and then this exact detector command for real.
						continue
					}
					if found, err := testFuncExistsInTree(moduleRoot, pkgDir, testName); err != nil {
						t.Errorf("task %s: detector %s: searching for %s in %q: %v", task.ID, d.Name, testName, pkgDir, err)
					} else if !found {
						t.Errorf("task %s: detector %s names test %s, which does not exist under %q", task.ID, d.Name, testName, pkgDir)
					}
				}
			}

			// ResolveCitationLine (citation.go, item 3) accepts both the
			// original "path#Lnn" form and the heading-anchored
			// "path#ADR-nnn" form, resolving the latter to wherever the
			// "## ADR-nnn …" heading CURRENTLY sits — so a citation into
			// an ADR record survives an insertion above it, unlike a
			// hardcoded line number.
			path, line, err := ResolveCitationLine(moduleRoot, task.Rule.Source)
			if err != nil {
				t.Fatalf("task %s: rule source %q: %v", task.ID, task.Rule.Source, err)
			}
			data, err := os.ReadFile(moduleRoot + "/" + path)
			if err != nil {
				t.Fatalf("task %s: reading rule source %q: %v", task.ID, path, err)
			}
			lines := strings.Split(string(data), "\n")
			if line < 1 || line > len(lines) {
				t.Fatalf("task %s: rule source %q names line %d, file has %d lines", task.ID, task.Rule.Source, line, len(lines))
			}
			// The source line must carry the rule's id: an invariants table row
			// ("| I31 |") or, for a rule that lives in an ADR section, the
			// record's heading ("## ADR-006: …" — the id before "§").
			marker := fmt.Sprintf("| %s |", task.Rule.ID)
			head, _, _ := strings.Cut(task.Rule.ID, "§")
			if !strings.Contains(lines[line-1], marker) && !strings.Contains(lines[line-1], head) {
				// A rule quoted from inside an ADR section: the nearest
				// preceding "## " heading must be that record.
				ok := false
				for i := line - 1; i >= 0; i-- {
					if strings.HasPrefix(lines[i], "## ") {
						ok = strings.Contains(lines[i], head)
						break
					}
				}
				if !ok {
					t.Errorf("task %s: %s line %d is neither a %q row nor inside the %s section:\n%s", task.ID, path, line, marker, head, lines[line-1])
				}
			}
		})
	}
}
