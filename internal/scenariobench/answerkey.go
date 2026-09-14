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
// any tool_use OR tool_result block naming one of excludePaths' literal
// directory prefixes (excludeNeedle) — proof that an excluded answer-key
// path reached the agent despite prepare-time removal. Both directions
// matter: a Read of an excluded file returns raw file content with no
// path anywhere in the RESULT (fix-first review, wave-n1 — the earlier
// version scanned tool_result text only, so a plain Read of the answer
// key escaped the scan entirely), so the tool_use's own INPUT is scanned
// too (toolUseInputText: Read/read_source's file path, Grep/Glob's own
// path/pattern/glob fields, a Bash command's own text). false whenever
// there is nothing to scan (no raw transcript, no excludePaths) or
// nothing was found — never a false positive from a pattern this
// function cannot confidently turn into a literal needle.
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
		switch ev.Type {
		case "assistant":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_use" {
					continue
				}
				text := toolUseInputText(c.Name, c.Input)
				for _, needle := range needles {
					if containsPathNeedle(text, needle) {
						return true
					}
				}
			}
		case "user":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				text := rawTextBlock(c.Content)
				for _, needle := range needles {
					if containsPathNeedle(text, needle) {
						return true
					}
				}
			}
		}
	}
	return false
}

// toolUseInputText flattens one tool_use's own input into a single string
// AnswerKeyLeakScan can needle-search: Read/read_source's own file path
// (toolInputPath, stream.go — covers "file_path" and "file"), Grep/Glob's
// own "path"/"pattern"/"glob" fields (any of these can name or restrict to
// an excluded directory), and a Bash "command" field verbatim. Every
// field that is absent or not a string is simply "" and contributes
// nothing; unmarshaling into a struct rather than a map keeps this
// tolerant of whatever other fields a tool's own schema carries.
func toolUseInputText(name string, input json.RawMessage) string {
	var b strings.Builder
	if p := toolInputPath(name, input); p != "" {
		b.WriteString(p)
		b.WriteByte('\n')
	}
	var fields struct {
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
		Glob    string `json:"glob"`
		Command string `json:"command"`
	}
	if json.Unmarshal(input, &fields) == nil {
		for _, s := range []string{fields.Path, fields.Pattern, fields.Glob, fields.Command} {
			if s != "" {
				b.WriteString(s)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// containsPathNeedle reports whether needle (excludeNeedle's own
// directory-prefix shape, always ending in "/") appears in text at a
// position not immediately preceded by a path/identifier character — so
// "testdata/contextbench/" matches "cat testdata/contextbench/x.json" but
// not a differently-named directory that merely ends the same way
// ("mytestdata/contextbench/x.json"). needle's own trailing "/" is
// already the right-hand boundary — a directory needle is deliberately
// followed by more path text, unlike containsWholeToken's (goldsource.go)
// two-sided identifier check.
func containsPathNeedle(text, needle string) bool {
	if needle == "" {
		return false
	}
	start := 0
	for {
		i := strings.Index(text[start:], needle)
		if i < 0 {
			return false
		}
		i += start
		if i == 0 || !isPathIdentByte(text[i-1]) {
			return true
		}
		start = i + 1
	}
}

func isPathIdentByte(b byte) bool {
	return b == '_' || b == '-' || b == '.' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
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
