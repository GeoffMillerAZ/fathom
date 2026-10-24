// Package landing ranks local landing evidence without executing work.
package landing

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Commit is one commit in an oldest-first good..bad range. Files are
// repository-relative paths changed by that commit, including deleted paths.
type Commit struct {
	Hash    string
	Subject string
	Files   []string
	Merge   bool
}

// Evidence is the current index, not a reconstruction of a historical tree.
type Evidence struct {
	Nodes []core.SymbolNode
	Typed []core.Edge
	// IndexedCommit is the service's ingest watermark, if recorded.
	IndexedCommit string
}

// Candidate describes static evidence for a commit, never a causal verdict.
type Candidate struct {
	Commit       Commit
	ReachedTests []string
	BlastRadius  int // affected symbol count, a lower bound when files are missing
	MissingFiles []string
	Reason       string
}

// ResolveFailingTests returns sorted, deduplicated canonical test keys and
// unresolved inputs. An unresolved name is unknown, never negative evidence.
func ResolveFailingTests(failing []string, nodes []core.SymbolNode) (resolved, unresolved []string) {
	byName := make(map[string][]string)
	for _, n := range nodes {
		if callgraph.IsTestNode(n) {
			byName[n.Name] = append(byName[n.Name], n.Key())
			byName[n.Key()] = append(byName[n.Key()], n.Key())
		}
	}
	matched, missing := make(map[string]bool), make(map[string]bool)
	for _, name := range failing {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		keys := byName[name]
		if len(keys) == 0 {
			missing[name] = true
		}
		for _, key := range keys {
			matched[key] = true
		}
	}
	for key := range matched {
		resolved = append(resolved, key)
	}
	for name := range missing {
		unresolved = append(unresolved, name)
	}
	sort.Strings(resolved)
	sort.Strings(unresolved)
	return resolved, unresolved
}

// RankCulprits ranks commits by failing-test reach, then puts incomplete evidence
// ahead of indexed non-reaching commits, then uses descending affected-symbol
// count and recency (the position in commits). It reuses the same test reach
// and blast-radius analyses as review/tests and blast_radius, with one shared
// graph. A file change seeds all its indexed symbols. Missing or negative static
// evidence never exonerates a commit. Inputs are not modified.
func RankCulprits(commits []Commit, failing []string, evidence Evidence) []Candidate {
	g := callgraph.BuildBoth(evidence.Nodes, evidence.Typed)
	files := make(map[string]bool)
	wanted := make(map[string]bool)
	resolved, _ := ResolveFailingTests(failing, evidence.Nodes)
	for _, key := range resolved {
		wanted[key] = true
	}
	failingFiles := make(map[string][]string)
	for _, n := range evidence.Nodes {
		files[n.FilePath] = true
		if wanted[n.Key()] {
			failingFiles[n.FilePath] = append(failingFiles[n.FilePath], n.Key())
		}
	}
	type ranked struct {
		candidate Candidate
		position  int
	}
	ranks := make([]ranked, 0, len(commits))
	for i, commit := range commits {
		commit.Files = append([]string(nil), commit.Files...)
		sort.Strings(commit.Files)
		c := Candidate{Commit: commit}
		missing := make(map[string]bool)
		reached := make(map[string]bool)
		for _, file := range commit.Files {
			if !files[file] {
				missing[file] = true
			}
			// TestGapOn deliberately excludes test nodes from its seeds.
			// Editing a named failing test is direct reach evidence here.
			for _, key := range failingFiles[file] {
				reached[key] = true
			}
		}
		for file := range missing {
			c.MissingFiles = append(c.MissingFiles, file)
		}
		sort.Strings(c.MissingFiles)
		c.BlastRadius = len(impact.BlastRadiusOn(g, evidence.Nodes, commit.Files).Symbols)
		for _, reach := range impact.TestGapOn(g, evidence.Nodes, commit.Files).AffectedTests {
			if wanted[reach.Test] {
				reached[reach.Test] = true
			}
		}
		for key := range reached {
			c.ReachedTests = append(c.ReachedTests, key)
		}
		sort.Strings(c.ReachedTests)
		if len(c.ReachedTests) > 0 {
			c.Reason = "failing-test reach: " + strings.Join(c.ReachedTests, ", ")
		} else if len(failing) == 0 {
			c.Reason = "no reach evidence (no failing tests supplied)"
		} else if len(resolved) == 0 {
			c.Reason = "no reach evidence (no failing tests resolved)"
		} else {
			c.Reason = "no reach evidence to the resolved failing tests (not exonerated)"
		}
		if len(c.MissingFiles) > 0 {
			c.Reason += "; no reach evidence for files: " + strings.Join(c.MissingFiles, ", ")
		}
		if len(c.MissingFiles) > 0 || len(evidence.Nodes) == 0 {
			c.Reason += fmt.Sprintf("; blast radius >=%d indexed symbols (incomplete)", c.BlastRadius)
		} else {
			c.Reason += fmt.Sprintf("; blast radius %d indexed symbols", c.BlastRadius)
		}
		ranks = append(ranks, ranked{candidate: c, position: i})
	}
	sort.Slice(ranks, func(i, j int) bool {
		a, b := ranks[i], ranks[j]
		ar, br := len(a.candidate.ReachedTests) > 0, len(b.candidate.ReachedTests) > 0
		if ar != br {
			return ar
		}
		if !ar {
			am, bm := len(a.candidate.MissingFiles) > 0, len(b.candidate.MissingFiles) > 0
			if am != bm {
				return am
			}
		}
		if a.candidate.BlastRadius != b.candidate.BlastRadius {
			return a.candidate.BlastRadius > b.candidate.BlastRadius
		}
		return a.position > b.position
	})
	out := make([]Candidate, len(ranks))
	for i, r := range ranks {
		out[i] = r.candidate
	}
	return out
}
