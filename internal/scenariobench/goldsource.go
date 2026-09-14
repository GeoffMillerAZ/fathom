// goldsource.go is VB-4 (docs/vision/29-verbs-not-lookups.md §1 R2,
// ADR-035 decision 5): "gold_source_tool rides every bench cell; a tool
// is a strength only when it earned the pass." A cell passing its oracle
// says the AGENT got there; GoldSourceTool says which tool's own result
// actually carried the gold signal first — the number that turns E1/O23
// ("obligations served six displaced rules and the arm passed the task by
// reading the file grep's way") into something computed, not narrated.
package scenariobench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
)

// GoldSourceNone is GoldSourceTool's return when task carries a gold
// signal to check but no tool result ever held it — the agent knew or
// guessed the answer without one surfacing it.
const GoldSourceNone = "none"

// goldMode is which of a task's three shapes GoldSourceTool is checking
// against — the three are mutually exclusive across this repo's corpora
// (testdata/scenariobench/*.json), checked in this priority order.
type goldMode int

const (
	goldModeNone goldMode = iota
	goldModeLookup
	goldModeChange
	goldModeGuards
)

// goldSignal picks task's gold mode and the token(s) that mode checks
// for: a lookup task's answer_all_of strings (plus every answer_any_of
// group's members flattened in — any one of a group counting the same as
// a plain required token, for gold-source purposes), a change task's
// Gold.Files paths, or a guards task's rule source path. goldModeNone (no
// tokens) when task carries none of the three.
func goldSignal(task Task) (goldMode, []string) {
	if len(task.Oracle.AnswerAllOf) > 0 || len(task.Oracle.AnswerAnyOf) > 0 {
		tokens := append([]string(nil), task.Oracle.AnswerAllOf...)
		for _, group := range task.Oracle.AnswerAnyOf {
			tokens = append(tokens, group...)
		}
		return goldModeLookup, tokens
	}
	if len(task.Gold.Files) > 0 {
		return goldModeChange, task.Gold.Files
	}
	if task.Rule != nil && task.Rule.Source != "" {
		return goldModeGuards, []string{task.Rule.Source}
	}
	return goldModeNone, nil
}

// GoldSourceTool scans sessions' own raw transcripts, in session order
// (SessionResult.Index), for the FIRST tool_result whose content held
// task's gold signal:
//
//   - lookup task (Oracle.AnswerAllOf/AnswerAnyOf non-empty): any of
//     those strings, matched case-insensitively as a WHOLE token in the
//     result body — not a substring inside a longer identifier — OR, for
//     a token shaped like a composed "path#Lnn" citation, against a
//     citation COMPOSED from the result body (composeCitations: a
//     grep-style "path:nn:" line, or a Read-shaped result's own
//     numbered lines paired with the tool_use's own input path) — Grep's
//     and Read's raw output never spells the citation out literally, so
//     without composing it a lookup task whose gold IS a citation could
//     never credit the tool that actually served it (O53 defect 6).
//   - change task (Gold.Files non-empty): a file from Gold.Files, either
//     named by the paired tool_use's own input path (a Read/read_source/
//     Edit-shaped call) or mentioned in the result body itself, matched
//     by path suffix (sameFile).
//   - guards task (Rule.Source non-empty): the rule's own source path,
//     as a substring of the result body.
//
// A result whose paired tool_use targeted a path in task.Subject's own
// ExcludePaths (O53 defect 6d), or a composed citation whose own path
// does, is never credited — the worktree's own answer key surfacing the
// gold text is contamination, not a tool teaching the agent anything.
//
// Returns (GoldSourceNone, "") when task carries none of the three
// signals, when a lookup task's own trial never produced a final answer
// at all (lastFinalAnswer — O53 defect 6c: crediting a tool on a trial
// that never answered is not a "this tool helped it pass" story), when
// no session has a raw transcript to read, or when no tool result
// anywhere carried the signal — the agent knew or guessed. evidence
// names the matched token, file, or path.
func GoldSourceTool(sessions []SessionResult, task Task) (tool string, evidence string) {
	mode, tokens := goldSignal(task)
	if mode == goldModeNone {
		return GoldSourceNone, ""
	}
	if mode == goldModeLookup && lastFinalAnswer(sessions) == "" {
		return GoldSourceNone, ""
	}
	ordered := append([]SessionResult(nil), sessions...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Index < ordered[j].Index })
	for _, s := range ordered {
		if s.RawPath == "" {
			continue
		}
		data, err := os.ReadFile(s.RawPath)
		if err != nil {
			continue
		}
		if t, ev, ok := scanGoldSource(data, mode, tokens, task.Subject.ExcludePaths); ok {
			return t, ev
		}
	}
	return GoldSourceNone, ""
}

// containsWholeToken reports whether token appears in text case-
// insensitively, at a position not immediately preceded or followed by an
// identifier character — so a token like "New" does not match inside
// "renewal".
func containsWholeToken(text, token string) bool {
	if token == "" {
		return false
	}
	lowerText, lowerToken := strings.ToLower(text), strings.ToLower(token)
	start := 0
	for {
		i := strings.Index(lowerText[start:], lowerToken)
		if i < 0 {
			return false
		}
		i += start
		before, after := byte(0), byte(0)
		if i > 0 {
			before = lowerText[i-1]
		}
		if end := i + len(lowerToken); end < len(lowerText) {
			after = lowerText[end]
		}
		if !isIdentByte(before) && !isIdentByte(after) {
			return true
		}
		start = i + 1
	}
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// changeTargetFile returns the file path a tool_use's own input names,
// for the tool shapes that name one directly: Read/read_source
// (toolInputPath, stream.go) and Edit/Write/MultiEdit/NotebookEdit
// (editTargetFile, ruleserved_before_edit.go). "" for every other tool.
func changeTargetFile(name string, input json.RawMessage) string {
	if readToolNames[name] {
		if p := toolInputPath(name, input); p != "" {
			return p
		}
	}
	return editTargetFile(name, input)
}

// matchGoldFile reports whether file path-suffix-matches (sameFile) any
// of goldFiles, returning the matched entry.
func matchGoldFile(file string, goldFiles []string) (string, bool) {
	for _, gf := range goldFiles {
		if sameFile(file, gf) {
			return gf, true
		}
	}
	return "", false
}

// grepCiteRE matches a grep-style "path:nn:" result line — Grep's own
// output shape, or a Bash `grep -n`/`rg -n` call producing the exact same
// shape (composeCitations does not gate this on the tool NAME, only the
// text's own shape). The path is everything before the first colon.
var grepCiteRE = regexp.MustCompile(`(?m)^([^\s:][^:]*):(\d+):`)

// readLineRE matches one of Claude Code's own Read-tool numbered-line
// prefixes: a line number then an arrow ("    48→") or a tab ("48\t").
// Read's own result carries no file path — the caller already gave it as
// the tool_use's own input — so composeCitations pairs each match here
// with toolInputFile instead of parsing one out of body.
var readLineRE = regexp.MustCompile(`(?m)^\s*(\d+)(?:→|\t)`)

// citationRE recognizes a "path#Lnn" composed-citation TOKEN shape —
// what a lookup task's own gold token looks like when the fact being
// checked is "this exact line of this exact file" (fathom's obligations
// tool composes citations in exactly this shape, e.g.
// "internal/CLAUDE.md#L3").
var citationRE = regexp.MustCompile(`^(.+)#L(\d+)$`)

// isCitationToken reports whether tok is shaped like a composed citation.
// composeCitations/citationMatches apply only to tokens of this shape;
// every other token keeps matching via containsWholeToken exactly as
// before O53 defect 6.
func isCitationToken(tok string) bool { return citationRE.MatchString(tok) }

// composeCitations returns every "path#Lnn" citation body's own text
// implies (O53 defect 6a/6b): each grep-style "path:nn:" line composes
// directly (grepCiteRE), and each Read-shaped numbered line (readLineRE)
// composes against toolInputFile — "" toolInputFile skips that pass,
// since there is nothing to pair the bare line numbers with.
func composeCitations(body, toolInputFile string) []string {
	var out []string
	for _, m := range grepCiteRE.FindAllStringSubmatch(body, -1) {
		out = append(out, m[1]+"#L"+m[2])
	}
	if toolInputFile != "" {
		for _, m := range readLineRE.FindAllStringSubmatch(body, -1) {
			out = append(out, toolInputFile+"#L"+m[1])
		}
	}
	return out
}

// citationMatches reports whether tok (a gold "path#Lnn" citation) is
// satisfied by candidate (one of composeCitations' own "path#Lnn"
// strings): the SAME line number, and the path matching by suffix
// (sameFile) — a gold token's own path form ("internal/CLAUDE.md") need
// not equal the tool's exactly (an absolute worktree path, say).
func citationMatches(tok, candidate string) bool {
	tokM := citationRE.FindStringSubmatch(tok)
	candM := citationRE.FindStringSubmatch(candidate)
	if tokM == nil || candM == nil {
		return false
	}
	return tokM[2] == candM[2] && sameFile(tokM[1], candM[1])
}

// filterExcludedLines removes any line from body whose own grep-style
// "path:nn:" prefix (grepCiteRE) names a path excluded reports true for
// — so a plain substring/whole-token match against the REMAINING text
// never credits a line that itself came from the worktree's own answer
// key, even inside a result that also carries legitimate lines from
// other files (a repo-wide Grep, say). A body with no grep-style lines
// at all (a Read's plain numbered content, a Bash command's ordinary
// stdout) passes through unchanged here — that whole-result exclusion is
// already handled by the caller's toolInputFile gate before this
// function is ever reached.
func filterExcludedLines(body string, excluded func(string) bool) string {
	if !strings.Contains(body, "\n") {
		if m := grepCiteRE.FindStringSubmatch(body); m != nil && excluded(m[1]) {
			return ""
		}
		return body
	}
	lines := strings.Split(body, "\n")
	var kept []string
	for _, line := range lines {
		if m := grepCiteRE.FindStringSubmatch(line); m != nil && excluded(m[1]) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// citationPath returns candidate's own path half (the part before
// "#Lnn"), "" if candidate is not citation-shaped — used to check a
// composed citation's SOURCE against task.Subject.ExcludePaths
// independent of whatever path the tool_use itself was called with (a
// repo-wide Grep's per-line path, in particular, is never the tool_use's
// own single input path).
func citationPath(candidate string) string {
	m := citationRE.FindStringSubmatch(candidate)
	if m == nil {
		return ""
	}
	return m[1]
}

// scanGoldSource is one transcript's own pass: assistant tool_use blocks
// build the id->name and id->own-input-path maps (and, in change mode,
// which id targeted a gold file), and user tool_result blocks are
// checked against mode's own gold signal in transcript order — the FIRST
// match anywhere wins. excludePaths (task.Subject.ExcludePaths, O53
// defect 6d) disqualifies any candidate sourced from one of those paths,
// whether that is the paired tool_use's own input path or a composed
// citation's own path.
func scanGoldSource(transcript []byte, mode goldMode, tokens []string, excludePaths []string) (tool, evidence string, found bool) {
	toolByID := map[string]string{}
	inputFileByID := map[string]string{} // Read/read_source-shaped tool_use id -> its own input path; "" otherwise
	pendingFile := map[string]string{}   // change mode only: tool_use id -> gold file it targeted
	// excluded is a plain substring check (excludeNeedle, answerkey.go) —
	// never a rooted glob match — because a raw transcript's own tool_use
	// input paths are typically ABSOLUTE (Read's own "file_path" is
	// Claude Code's own convention) while excludePaths are worktree-root
	// relative, and this file has no reliable worktree root to relativize
	// against (a rescored cell's own worktree may no longer exist on
	// disk, or may have been reset by a later cell) — a substring check
	// on the excluded directory's own name needs neither.
	excluded := func(path string) bool {
		if path == "" {
			return false
		}
		for _, pattern := range excludePaths {
			if needle := excludeNeedle(pattern); needle != "" && strings.Contains(path, needle) {
				return true
			}
		}
		return false
	}

	sc := bufio.NewScanner(bytes.NewReader(transcript))
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
				if c.ID != "" {
					toolByID[c.ID] = c.Name
					if readToolNames[c.Name] {
						inputFileByID[c.ID] = toolInputPath(c.Name, c.Input)
					}
				}
				if mode == goldModeChange {
					if file := changeTargetFile(c.Name, c.Input); file != "" && !excluded(file) {
						if gf, ok := matchGoldFile(file, tokens); ok {
							pendingFile[c.ID] = gf
						}
					}
				}
			}
		case "user":
			for _, c := range ev.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				name := toolByID[c.ToolUseID]
				if name == "" {
					name = "unknown"
				}
				toolInputFile := inputFileByID[c.ToolUseID]
				if excluded(toolInputFile) {
					// The tool_use ITSELF read an excluded (answer-key)
					// path — nothing this result carries is a legitimate
					// gold source, whatever it happens to contain.
					continue
				}
				if mode == goldModeChange {
					if gf, ok := pendingFile[c.ToolUseID]; ok {
						return name, gf, true
					}
				}
				body := rawResultText(c.Content)
				// A result can carry lines from several files at once (a
				// repo-wide Grep, say) — filteredBody drops any line
				// whose OWN grep-style "path:nn:" prefix names an
				// excluded path before either substring check runs, so a
				// legitimate line from elsewhere in the SAME result still
				// counts (O53 defect 6d).
				filteredBody := filterExcludedLines(body, excluded)
				switch mode {
				case goldModeLookup:
					for _, tok := range tokens {
						if containsWholeToken(filteredBody, tok) {
							return name, tok, true
						}
						if !isCitationToken(tok) {
							continue
						}
						for _, cand := range composeCitations(body, toolInputFile) {
							if excluded(citationPath(cand)) {
								continue
							}
							if citationMatches(tok, cand) {
								return name, tok, true
							}
						}
					}
				case goldModeChange, goldModeGuards:
					for _, tok := range tokens {
						if strings.Contains(filteredBody, tok) {
							return name, tok, true
						}
					}
				}
			}
		}
	}
	return "", "", false
}
