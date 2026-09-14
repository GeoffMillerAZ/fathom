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
// for: a lookup task's answer_all_of strings, a change task's Gold.Files
// paths, or a guards task's rule source path. goldModeNone (no tokens)
// when task carries none of the three.
func goldSignal(task Task) (goldMode, []string) {
	if len(task.Oracle.AnswerAllOf) > 0 {
		return goldModeLookup, task.Oracle.AnswerAllOf
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
//   - lookup task (Oracle.AnswerAllOf non-empty): any of those strings,
//     matched case-insensitively as a WHOLE token in the result body —
//     not a substring inside a longer identifier.
//   - change task (Gold.Files non-empty): a file from Gold.Files, either
//     named by the paired tool_use's own input path (a Read/read_source/
//     Edit-shaped call) or mentioned in the result body itself, matched
//     by path suffix (sameFile).
//   - guards task (Rule.Source non-empty): the rule's own source path,
//     as a substring of the result body.
//
// Returns (GoldSourceNone, "") when task carries none of the three
// signals, when no session has a raw transcript to read, or when no tool
// result anywhere carried the signal — the agent knew or guessed.
// evidence names the matched token, file, or path.
func GoldSourceTool(sessions []SessionResult, task Task) (tool string, evidence string) {
	mode, tokens := goldSignal(task)
	if mode == goldModeNone {
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
		if t, ev, ok := scanGoldSource(data, mode, tokens); ok {
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

// scanGoldSource is one transcript's own pass: assistant tool_use blocks
// build the id->name map (and, in change mode, which id targeted a gold
// file), and user tool_result blocks are checked against mode's own gold
// signal in transcript order — the FIRST match anywhere wins.
func scanGoldSource(transcript []byte, mode goldMode, tokens []string) (tool, evidence string, found bool) {
	toolByID := map[string]string{}
	pendingFile := map[string]string{} // change mode only: tool_use id -> gold file it targeted

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
				}
				if mode == goldModeChange {
					if file := changeTargetFile(c.Name, c.Input); file != "" {
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
				if mode == goldModeChange {
					if gf, ok := pendingFile[c.ToolUseID]; ok {
						return name, gf, true
					}
				}
				body := rawResultText(c.Content)
				switch mode {
				case goldModeLookup:
					for _, tok := range tokens {
						if containsWholeToken(body, tok) {
							return name, tok, true
						}
					}
				case goldModeChange, goldModeGuards:
					for _, tok := range tokens {
						if strings.Contains(body, tok) {
							return name, tok, true
						}
					}
				}
			}
		}
	}
	return "", "", false
}
