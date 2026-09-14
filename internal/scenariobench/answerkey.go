// answerkey.go is O53 defect 3: tasks copied from
// testdata/contextbench/tasks-home.json ran in worktrees that also
// CONTAINED that file (and its siblings under testdata/scenariobench/ and
// docs/bench/), so the answer key a task's ground truth was copied from
// sat inside the very worktree the agent could Read/Grep/Bash — it
// surfaced in tool results in 41 scored cells of pilot vs-pilot-3. Three
// mechanisms close it: prepare-time removal (removeExcludedPaths,
// subject.go), a load-time requirement that any task derived from the
// home corpus declares the exclusion (corpus.go's validateScenarioTask),
// and this file's post-session scan, which catches the case both of those
// missed — a subject setup command that re-creates an excluded path, a
// symlink, or a corpus that forgot to name a path the removal needed.
package scenariobench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/filter"
)

// homeCorpusMarker is the exact path a task's derivation names when its
// prompt or ground truth was copied from the home lookup corpus.
const homeCorpusMarker = "testdata/contextbench/tasks-home.json"

// requiredHomeCorpusExcludes are the exclude_paths glob patterns
// validateScenarioTask requires of any task whose derivation names the
// home corpus: the home corpus itself, this corpus (a scenariobench task
// can quote another scenariobench task's own answer, e.g. lookups-vs.json
// quoting a sibling task's gold), and docs/bench/, which narrates both in
// prose.
var requiredHomeCorpusExcludes = []string{
	"testdata/contextbench/**",
	"testdata/scenariobench/**",
	"docs/bench/**",
}

// derivesFromHomeCorpus reports whether t's own Derivation names the home
// lookup corpus as a source for its prompt or ground truth — the
// contamination risk requiredHomeCorpusExcludes guards against.
func derivesFromHomeCorpus(t Task) bool {
	return strings.Contains(t.Derivation, homeCorpusMarker)
}

// excludePathsCoverHomeCorpus reports whether paths names every glob in
// requiredHomeCorpusExcludes, verbatim (the loader refuses a task that
// derives from the home corpus but declares anything less).
func excludePathsCoverHomeCorpus(paths []string) []string {
	have := make(map[string]bool, len(paths))
	for _, p := range paths {
		have[p] = true
	}
	var missing []string
	for _, want := range requiredHomeCorpusExcludes {
		if !have[want] {
			missing = append(missing, want)
		}
	}
	return missing
}

// excludeNeedle turns one exclude_paths glob into the literal substring
// AnswerKeyLeakScan searches transcript text for: a "/**" suffix (the only
// shape requiredHomeCorpusExcludes or a hand-authored corpus is expected
// to use) becomes its directory prefix WITH the trailing slash, so
// "testdata/contextbench/**" catches any mention of a path under that
// directory without also matching an unrelated "testdata/contextbenchmark"
// sibling. A pattern with no "/**" suffix is used as-is — a plain file
// path glob names exactly the string that would leak.
func excludeNeedle(pattern string) string {
	if strings.HasSuffix(pattern, "/**") {
		return strings.TrimSuffix(pattern, "**")
	}
	return pattern
}

// AnswerKeyLeakScan scans session's raw transcript (session.RawPath) for
// any tool_result block whose flattened text contains one of
// excludePaths' literal directory prefixes (excludeNeedle) — proof that an
// excluded answer-key path reached the agent despite prepare-time removal.
// false whenever there is nothing to scan (no raw transcript, no
// excludePaths) or nothing was found — never a false positive from a
// pattern this function cannot confidently turn into a literal needle.
func AnswerKeyLeakScan(session SessionResult, excludePaths []string) bool {
	if session.RawPath == "" || len(excludePaths) == 0 {
		return false
	}
	needles := make([]string, 0, len(excludePaths))
	for _, p := range excludePaths {
		if n := excludeNeedle(p); n != "" {
			needles = append(needles, n)
		}
	}
	if len(needles) == 0 {
		return false
	}
	data, err := os.ReadFile(session.RawPath)
	if err != nil {
		return false
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev streamToolEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type != "user" {
			continue
		}
		for _, c := range ev.Message.Content {
			if c.Type != "tool_result" {
				continue
			}
			text := rawTextBlock(c.Content)
			for _, needle := range needles {
				if strings.Contains(text, needle) {
					return true
				}
			}
		}
	}
	return false
}

// removeExcludedPaths deletes every path under root matching one of
// patterns (gitignore-style globs, internal/filter) — PrepareSubject's own
// call, run after checkout and before any ingest, for EVERY arm, so an
// answer-key file a corpus was derived from never sits inside the
// worktree an agent's Read/Grep/Bash tools can reach. root's own ".git"
// is never descended into or considered for removal, whatever patterns
// says. A nil/empty patterns is a no-op.
func removeExcludedPaths(root string, patterns []string) error {
	if len(patterns) == 0 {
		return nil
	}
	m := filter.New(patterns)
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("removing excluded paths: reading %s: %w", root, err)
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if err := removeExcludedWalk(root, e.Name(), m); err != nil {
			return err
		}
	}
	return nil
}

// removeExcludedWalk checks rel (a path relative to root) against m and,
// on a match, removes it outright; otherwise, for a directory, recurses
// into it so a pattern anchored to a nested path (rather than one of
// root's own top-level entries) is still found.
func removeExcludedWalk(root, rel string, m *filter.Matcher) error {
	full := root + "/" + rel
	info, err := os.Lstat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("removing excluded paths: statting %s: %w", full, err)
	}
	target := rel
	if info.IsDir() {
		target += "/"
	}
	if m.Match(target) {
		if err := os.RemoveAll(full); err != nil {
			return fmt.Errorf("removing excluded path %s: %w", rel, err)
		}
		return nil
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return fmt.Errorf("removing excluded paths: reading %s: %w", full, err)
	}
	for _, e := range entries {
		if err := removeExcludedWalk(root, rel+"/"+e.Name(), m); err != nil {
			return err
		}
	}
	return nil
}
