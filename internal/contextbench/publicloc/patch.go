// Package publicloc derives fathom's public localization corpus
// (testdata/contextbench/tasks-public-go.json) from pinned third-party
// datasets with no LLM in the loop (docs/plans/bench-public.md PB-14..17,
// ADR-031, docs/vision/26-proof-credibility.md §2.3).
//
// Every ground truth here is developer-authored: the files and functions a
// merged gold patch actually changed at a pinned base commit, and the static
// callers of those functions as computed by cmd/loctruth (go/types) — never
// by fathom's own index. The derivation is a pure function of the pinned
// inputs, so the same snapshots produce the same corpus_sha256 on any
// machine (TestDerivationIsDeterministic).
//
// Pre-registered rules (committed before any run; changing one is a visible
// edit to this file, not a quiet reinterpretation of the data):
//
//   - Selection: an instance qualifies when its gold patch modifies or
//     deletes between MinFiles and MaxFiles non-test Go source files that
//     exist at base_commit (added files cannot be localized, so they are not
//     gold). Candidates sort by instance_id; the first N are the corpus.
//   - file_localization gold = those files; scoring all-of with a predicted
//     cap of max(5, 2·|gold|) (H2-2).
//   - function_localization gold = the functions whose span at base_commit
//     contains a deleted line, or contains both neighbours of an inserted
//     run (an insertion between two functions is nobody's function).
//   - find_callers gold = the non-test static callers of one gold function,
//     as listed by cmd/loctruth; a function with no callers or more than
//     MaxCallers callers yields no task (recorded in the derivation).
package publicloc

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// PatchFile is one file section of a unified diff.
type PatchFile struct {
	// OldPath is the path at base_commit (empty for an added file);
	// NewPath is the path after the patch (empty for a deleted file).
	OldPath string
	NewPath string
	Added   bool
	Deleted bool
	Hunks   []Hunk
}

// Hunk is one @@ section with its body lines (prefix character retained).
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Body     []string
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// ParsePatch parses a unified diff as produced by git. It is deliberately
// small: file headers, hunk headers, and hunk bodies are all it needs, and
// a body line is only ever mistaken for a header when the hunk's own
// counts say the body is already complete.
func ParsePatch(patch string) ([]PatchFile, error) {
	var files []PatchFile
	var cur *PatchFile
	var hunk *Hunk
	oldRemain, newRemain := 0, 0

	for i, ln := range strings.Split(patch, "\n") {
		if hunk != nil && (oldRemain > 0 || newRemain > 0) {
			// An empty line inside an unfinished hunk is a blank context
			// line whose leading space a dataset stripped; git itself
			// always emits the space.
			if ln == "" {
				ln = " "
			}
			switch ln[0] {
			case ' ':
				oldRemain--
				newRemain--
			case '-':
				oldRemain--
			case '+':
				newRemain--
			case '\\':
				// "\ No newline at end of file" — not a counted line.
			default:
				return nil, fmt.Errorf("patch line %d: unexpected %q inside a hunk with %d old / %d new lines remaining", i+1, ln, oldRemain, newRemain)
			}
			hunk.Body = append(hunk.Body, ln)
			continue
		}
		switch {
		case strings.HasPrefix(ln, "diff --git "):
			files = append(files, PatchFile{})
			cur = &files[len(files)-1]
			hunk = nil
			rest := strings.TrimPrefix(ln, "diff --git ")
			if idx := strings.Index(rest, " b/"); idx >= 0 {
				cur.OldPath = strings.TrimPrefix(rest[:idx], "a/")
				cur.NewPath = rest[idx+3:]
			}
		case strings.HasPrefix(ln, "--- "):
			if cur == nil {
				return nil, fmt.Errorf("patch line %d: file header before any diff --git line", i+1)
			}
			p := headerPath(strings.TrimPrefix(ln, "--- "))
			if p == "/dev/null" {
				cur.Added = true
				cur.OldPath = ""
			} else {
				cur.OldPath = strings.TrimPrefix(p, "a/")
			}
		case strings.HasPrefix(ln, "+++ "):
			if cur == nil {
				return nil, fmt.Errorf("patch line %d: file header before any diff --git line", i+1)
			}
			p := headerPath(strings.TrimPrefix(ln, "+++ "))
			if p == "/dev/null" {
				cur.Deleted = true
				cur.NewPath = ""
			} else {
				cur.NewPath = strings.TrimPrefix(p, "b/")
			}
		case strings.HasPrefix(ln, "@@ "):
			if cur == nil {
				return nil, fmt.Errorf("patch line %d: hunk before any diff --git line", i+1)
			}
			m := hunkHeader.FindStringSubmatch(ln)
			if m == nil {
				return nil, fmt.Errorf("patch line %d: malformed hunk header %q", i+1, ln)
			}
			h := Hunk{OldStart: atoi(m[1], 0), OldLines: atoi(m[2], 1), NewStart: atoi(m[3], 0), NewLines: atoi(m[4], 1)}
			cur.Hunks = append(cur.Hunks, h)
			hunk = &cur.Hunks[len(cur.Hunks)-1]
			oldRemain, newRemain = h.OldLines, h.NewLines
		default:
			// index lines, mode lines, rename lines, blank trailers: ignored.
		}
	}
	return files, nil
}

func headerPath(s string) string {
	if i := strings.IndexByte(s, '\t'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func atoi(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// Touch is the set of base_commit lines a hunk changes: Deleted lines
// (present at base_commit, gone or replaced after) and Inserted points (the
// base_commit line each inserted run falls BEFORE; a run appended at end of
// file falls before line len+1).
type Touch struct {
	Deleted  []int
	Inserted []int
}

// TouchedOldLines walks a hunk body and reports exactly which base_commit
// lines it deletes and where it inserts, so context lines — which a hunk
// carries three of on each side — never attribute a change to a neighbouring
// function.
func TouchedOldLines(h Hunk) Touch {
	var t Touch
	old := h.OldStart
	inRun := false
	for _, ln := range h.Body {
		if ln == "" {
			ln = " "
		}
		switch ln[0] {
		case ' ':
			old++
			inRun = false
		case '-':
			t.Deleted = append(t.Deleted, old)
			old++
			inRun = false
		case '+':
			if !inRun {
				t.Inserted = append(t.Inserted, old)
				inRun = true
			}
		}
	}
	return t
}

// testPathSegments are directory names whose contents are test material by
// convention; a path under any of them is never gold (E4 §3: every
// predicted test path is a false positive, so none may be a true one).
var testPathSegments = map[string]bool{"testdata": true, "test": true, "tests": true, "fixtures": true}

// IsTestPath reports whether p is a test file or lives under a test
// directory. The rule is registered here, not inferred per corpus.
func IsTestPath(p string) bool {
	if strings.HasSuffix(p, "_test.go") {
		return true
	}
	for _, seg := range strings.Split(p, "/") {
		if testPathSegments[seg] {
			return true
		}
	}
	return false
}

// IsGoSource reports whether p is a non-test Go source path.
func IsGoSource(p string) bool {
	return path.Ext(p) == ".go" && !IsTestPath(p)
}
