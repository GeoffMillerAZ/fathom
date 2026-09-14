package custodian

// codefacts.go — VB-28 (docs/plans/verbs.md, wave H; vision 29 §2 thesis
// 3; ADR-035 decision 7): the custodian audits code-fact-shaped sentences
// in every auto-loaded file, not only the fathom stanza — the R8 gap
// named in ADR-035's context ("the harness's own memory carried a code
// claim the custodian never saw"). Behind `fathom config audit
// --code-facts`, default OFF until the labelled-set precision bar (VB-29,
// M14) holds: precision first, in doubt classify preference (§ below).
//
// Four classes, never a silent fifth:
//   - anchored-fresh:          a path/symbol anchor resolves at HEAD and
//                               any stated count still holds
//   - anchored-drifted:        it resolved once but does not hold now —
//                               the receipt names what the tree says
//   - unanchorable-code-claim: shaped like a code fact (a trigger phrase,
//                               a bare count) but names nothing this
//                               audit can check
//   - preference:               not a code fact — an instruction, a
//                               principle, an imperative
//
// "At HEAD" here means the working tree at root, read directly (git
// blob access is not needed: a checked-out worktree already IS HEAD for
// a clean repo, and a dirty one is the more honest thing to check
// against — an agent auditing its own instructions cares what the files
// say right now).
//
// ADR-027's lesson applies unchanged: a file this cannot read is named,
// never silently absorbed into a clean bill (see AuditCodeFacts).

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CodeFactClass is one of the four audit outcomes for a sentence.
type CodeFactClass string

const (
	ClassAnchoredFresh   CodeFactClass = "anchored-fresh"
	ClassAnchoredDrifted CodeFactClass = "anchored-drifted"
	ClassUnanchorable    CodeFactClass = "unanchorable-code-claim"
	ClassPreference      CodeFactClass = "preference"
)

// Sentence is one classifiable unit: a line (or line fragment) from an
// auto-loaded file, with its provenance.
type Sentence struct {
	File string `json:"file"` // display path: repo-relative, or "~/..." for auto-memory
	Line int    `json:"line"`
	Text string `json:"text"`
}

// CodeFactFinding is one sentence's classification.
type CodeFactFinding struct {
	File     string        `json:"file"`
	Line     int           `json:"line"`
	Class    CodeFactClass `json:"class"`
	Sentence string        `json:"sentence"` // excerpt, capped
	Receipt  string        `json:"receipt,omitempty"`
}

// AutoLoadedFile is one file a coding harness loads into context
// automatically, discovered by AutoLoadedFiles.
type AutoLoadedFile struct {
	// Path is repo-relative for files inside root, or absolute for
	// harness-global files (Claude Code's per-project auto-memory lives
	// under the user's home directory, outside the repo).
	Path string
	Kind string // agent-config | cursor-rule | windsurf-rule | memory-bank | claude-auto-memory
}

const excerptCap = 160

// AuditCodeFacts is the `--code-facts` entry point: discover every
// auto-loaded file for root, split each into sentences, and classify
// them. A file that cannot be read is named in unreadable, never
// dropped silently from the count (ADR-027) — the caller must report an
// audit that skipped files as PARTIAL, not clean.
//
// home is the harness home directory (os.UserHomeDir() in production,
// injectable for tests); "" skips Claude Code auto-memory discovery
// rather than erroring.
func AuditCodeFacts(root, home string) (findings []CodeFactFinding, unreadable []string, err error) {
	files, ferr := AutoLoadedFiles(root, home)
	if ferr != nil {
		return nil, nil, ferr
	}
	var sentences []Sentence
	for _, f := range files {
		abs := f.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, f.Path)
		}
		data, rerr := os.ReadFile(abs)
		if rerr != nil {
			unreadable = append(unreadable, f.Path)
			continue
		}
		for _, rs := range splitSentences(string(data)) {
			sentences = append(sentences, Sentence{File: f.Path, Line: rs.Line, Text: rs.Text})
		}
	}
	return auditCodeFactSentences(root, sentences), unreadable, nil
}

// auditCodeFactSentences classifies each given sentence against root —
// the anchor this row builds (VB-28). Split out from AuditCodeFacts so
// the classifier's precision/recall can be measured directly against a
// labelled set (codefacts_test.go), independent of file discovery and
// sentence splitting.
func auditCodeFactSentences(root string, sentences []Sentence) []CodeFactFinding {
	out := make([]CodeFactFinding, 0, len(sentences))
	for _, s := range sentences {
		class, receipt := classifySentence(root, s.File, s.Text)
		out = append(out, CodeFactFinding{
			File:     s.File,
			Line:     s.Line,
			Class:    class,
			Sentence: excerpt(s.Text),
			Receipt:  receipt,
		})
	}
	return out
}

func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= excerptCap {
		return s
	}
	return s[:excerptCap-1] + "…"
}

// AutoLoadedFiles discovers every file a coding harness auto-loads for
// root, sorted for determinism: the agent-config corpus (CLAUDE.md /
// AGENTS.md at any depth — AgentConfigFiles, which already covers
// `.claude/**/CLAUDE.md`), Cursor project rules (`.cursor/rules/*`),
// Windsurf rules (`.windsurf/rules/*`), the Cline memory bank
// (`memory-bank/*.md`), and — when home is non-empty — Claude Code's
// per-project auto-memory (`~/.claude/projects/<encoded-root>/memory/*.md`).
func AutoLoadedFiles(root, home string) ([]AutoLoadedFile, error) {
	var out []AutoLoadedFile

	agentFiles, err := AgentConfigFiles(root)
	if err != nil {
		return nil, err
	}
	for _, f := range agentFiles {
		out = append(out, AutoLoadedFile{Path: f, Kind: "agent-config"})
	}

	globs := []struct{ pattern, kind string }{
		{".cursor/rules/*", "cursor-rule"},
		{".windsurf/rules/*", "windsurf-rule"},
		{"memory-bank/*.md", "memory-bank"},
	}
	for _, g := range globs {
		matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(g.pattern)))
		sort.Strings(matches)
		for _, m := range matches {
			fi, statErr := os.Stat(m)
			if statErr != nil || fi.IsDir() {
				continue
			}
			rel, relErr := filepath.Rel(root, m)
			if relErr != nil {
				continue
			}
			out = append(out, AutoLoadedFile{Path: filepath.ToSlash(rel), Kind: g.kind})
		}
	}

	if home != "" {
		memDir := ClaudeCodeMemoryDir(root, home)
		matches, _ := filepath.Glob(filepath.Join(memDir, "*.md"))
		sort.Strings(matches)
		for _, m := range matches {
			fi, statErr := os.Stat(m)
			if statErr != nil || fi.IsDir() {
				continue
			}
			out = append(out, AutoLoadedFile{Path: displayHomePath(home, m), Kind: "claude-auto-memory"})
		}
	}

	return out, nil
}

// ClaudeCodeMemoryDir resolves the per-project auto-memory directory the
// way Claude Code does: the project's absolute path with every "/"
// replaced by "-" (observed convention — this repo's own auto-memory
// lives at exactly this path for its own checkout).
func ClaudeCodeMemoryDir(root, home string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	encoded := strings.ReplaceAll(filepath.ToSlash(abs), "/", "-")
	return filepath.Join(home, ".claude", "projects", encoded, "memory")
}

func displayHomePath(home, abs string) string {
	if rel, err := filepath.Rel(home, abs); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(filepath.Join("~", rel))
	}
	return abs
}

// --- sentence splitting -----------------------------------------------

type rawSentence struct {
	Line int
	Text string
}

var (
	fenceRe         = regexp.MustCompile("^```")
	sentenceSplitRe = regexp.MustCompile(`([.!?])\s+`)
	// abbreviationEndRe matches a candidate sentence fragment ending in a
	// known abbreviation ("e.g.", "i.e.", …) — a period there is not a
	// sentence boundary, so the split folds into the next fragment.
	abbreviationEndRe = regexp.MustCompile(`(?i)\b(?:e\.g|i\.e|etc|approx|cf)\.$`)
	mdLeadRe          = regexp.MustCompile(`^(\s*[-*]\s+|\s*\|\s*|#+\s+|\d+\.\s+)+`)
)

// splitSentences breaks file content into (line, sentence-fragment)
// pairs: markdown fences are skipped, blank lines dropped, leading list /
// heading / table markup stripped, and each remaining line further split
// into sentences on `. ` / `! ` / `? ` boundaries — guarded against the
// common abbreviations ("e.g. ", "i.e. ") that would otherwise fragment a
// path like `CLAUDE.md` is already excluded, since that period is never
// followed by whitespace.
func splitSentences(text string) []rawSentence {
	var out []rawSentence
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if fenceRe.MatchString(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence || trimmed == "" {
			continue
		}
		content := mdLeadRe.ReplaceAllString(trimmed, "")
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		for _, sent := range splitLineSentences(content) {
			sent = strings.TrimSpace(sent)
			if sent != "" {
				out = append(out, rawSentence{Line: i + 1, Text: sent})
			}
		}
	}
	return out
}

func splitLineSentences(line string) []string {
	locs := sentenceSplitRe.FindAllStringSubmatchIndex(line, -1)
	if len(locs) == 0 {
		return []string{line}
	}
	var out []string
	start := 0
	for _, loc := range locs {
		// loc[2]:loc[3] is the punctuation char; loc[1] is where the
		// following whitespace run ends.
		candidate := line[start:loc[3]]
		if abbreviationEndRe.MatchString(candidate) {
			continue // not a sentence boundary — fold into the next split
		}
		out = append(out, line[start:loc[3]])
		start = loc[1]
	}
	out = append(out, line[start:])
	return out
}
