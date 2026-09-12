package contextbench

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The corpus gate (L2 of the bench pyramid, ADR-032 §1; docs/design/
// bench-pyramid.md §3.3). Two questions a committed corpus must answer
// before a single token is spent on it:
//
//  1. Is any ground-truth token already in the agent's context before the
//     prompt arrives? Every `claude -p` session auto-loads the repo's root
//     CLAUDE.md / AGENTS.md, the project .claude/CLAUDE.md, and the user's
//     memory index. A gold token sitting verbatim in one of those makes the
//     task a recall exercise for every arm — the `none` arm answered H1-T02
//     in 5.5 seconds with no tools (O25), and it cost six tool-arm cells to
//     find that out.
//  2. Does the prompt name the file its own ground truth cites? H1-T01 told
//     the agent to look in `internal/CLAUDE.md`, which is where the answer
//     lives (O23, design §3.3) — discovery collapses to one `cat`.
//
// Both are free and deterministic, so they run in `task validate`
// (TestHomeCorpusTruthAtHead) rather than being discovered by a run.

// memoryIndexEnv names the environment variable that points the corpus
// gate at the user's memory index. The index lives under the harness's
// per-user state (`~/.claude/projects/<slug>/memory/MEMORY.md`), which is
// neither committed nor identical between machines, so the gate asserts
// against it only when the operator names it — never silently skipping a
// file it should have read (ADR-027/U14: an unreadable input must not
// quietly shrink the check), and never failing a stranger's clone for a
// file that is not theirs.
const memoryIndexEnv = "CONTEXT_BENCH_MEMORY_INDEX"

// corpusBaselineFiles returns absolute paths to the files a headless
// `claude -p` session rooted at root loads into context BEFORE the prompt:
// the repo's root CLAUDE.md and AGENTS.md, the project-scoped
// .claude/CLAUDE.md, and — when $CONTEXT_BENCH_MEMORY_INDEX names it — the
// user's memory index. Files that do not exist are omitted, so the list is
// what this tree actually auto-loads, not what a template says it should.
func corpusBaselineFiles(root string) []string {
	var out []string
	for _, rel := range []string{"CLAUDE.md", "AGENTS.md", filepath.Join(".claude", "CLAUDE.md")} {
		p := filepath.Join(root, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	if p := os.Getenv(memoryIndexEnv); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// citationRE matches a source citation of the shape the compiled layer
// serves and the corpus quotes as a premise: `CLAUDE.md#L83-L84`,
// `docs/plans/callgraph-types.md#L45`. A prompt may name a file inside one
// of these — a staleness task cannot ask whether a citation still resolves
// without quoting the citation — but nowhere else.
var citationRE = regexp.MustCompile(`[A-Za-z0-9_./-]+#L[0-9]+(?:-L[0-9]+)?`)

// anchorFiles returns the repo files a task's ground truth is derived
// from: the File of every file/line anchor, and the Token of a path anchor
// that names a file (kind path with no File is "this path exists").
func anchorFiles(t Task) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, a := range t.TruthAtHead {
		switch a.Kind {
		case AnchorFile, AnchorLine:
			add(a.File)
		case AnchorPath:
			if a.File != "" {
				add(a.File)
			} else {
				add(a.Token)
			}
		}
	}
	return out
}

// promptNamesAnchorFile reports the first file a task's prompt names that
// its own ground truth is derived from — the H1-T01 shape, where the
// prompt hands the agent the file the answer lives in. Citations
// (`file#L12-L13`) are stripped first: quoting the citation under test is
// the premise of a staleness or provenance task, not a pointer to the
// answer. Returns "" when the prompt names no anchor file.
func promptNamesAnchorFile(t Task) string {
	stripped := citationRE.ReplaceAllString(t.Prompt, " ")
	for _, f := range anchorFiles(t) {
		if strings.Contains(stripped, f) {
			return f
		}
	}
	return ""
}
