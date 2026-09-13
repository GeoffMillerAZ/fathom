package scenariobench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
)

// ruleserved_before_edit.go is RC-12 (vision 28 §2.4, F12): "guards
// corpora: every rule sat in a file both arms read before the edit
// (CLAUDE.md, or the comment above the edit site); zero violations both
// arms is the corpus, not the claim" — M1 needs a rate computed from what
// actually reached the transcript, not read off which file the rule text
// happened to live in. RuleServed (ruleserved.go) already answers this
// from the edit-hook beat log's own turn-numbered evidence; this file
// answers it a second, more direct way, straight from the session's own
// raw transcript — the two are complementary readings, not a replacement.

// obligationsHookMarkers are the "a rule reached this session" texts
// vision 28 §2.4 names as any one sufficient — the exact headers
// cmd/fathom's obligations/guard hooks and its SessionStart stanza render:
//   - cmd/fathom/edithook_pre.go's PreToolUse guard render
//     (`"⚖️ fathom guard — %s:"`) — fires BEFORE the edit it guards, the
//     shape "served before edit" most directly describes.
//   - cmd/fathom/edithook.go's PostToolUse obligations render
//     (`"⚖️ fathom obligations — %s:"`, built right after markFileEdited)
//     — fires after an edit, so it only counts as "before" a LATER edit of
//     the same or another governed path in the same session.
//   - cmd/fathom/init.go's SessionStart stanza describing the feature
//     (obligationsPlain / obligationsWithIntent, both opening "Obligations
//     — governing rules for the file you are about to touch") — reaches
//     the transcript at session start, before any tool use at all.
var obligationsHookMarkers = []string{
	"⚖️ fathom guard — ",
	"⚖️ fathom obligations — ",
	"Obligations — governing rules for the file you are about to touch",
}

// editTargetFile returns the file path one Edit/Write/MultiEdit/
// NotebookEdit tool_use names (all four carry it under "file_path" in
// Claude Code's own tool schemas), or "" for every other tool — including
// Bash, which can also write a file, but not in a shape RuleServedBeforeEdit
// can name without shelling out to inspect a command's target.
func editTargetFile(name string, input json.RawMessage) string {
	switch name {
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
	default:
		return ""
	}
	if len(input) == 0 {
		return ""
	}
	var p struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return ""
	}
	return p.FilePath
}

// RuleServedBeforeEdit is RC-12: scans one session's raw stream-json
// transcript (session.RawPath) for the FIRST line carrying an
// obligationsHookMarkers text, and for the FIRST tool_use that is an
// Edit/Write/MultiEdit/NotebookEdit of one of governedPaths (matched via
// sameFile, so an absolute-vs-repo-relative mismatch does not read as a
// miss) — both by transcript line order, which is chronological since a
// stream-json transcript is append-only.
//
// Returns nil — not measurable, never a false "no" — when there is no raw
// transcript to read, no governed path was given, or the session never
// edited any governed path at all (the question "did the rule arrive
// before the edit" has no edit to be before). Otherwise: true when a
// served-rule line preceded the first governed edit, false when the edit
// came first or no served-rule line ever appeared.
func RuleServedBeforeEdit(session SessionResult, governedPaths []string) *bool {
	if session.RawPath == "" || len(governedPaths) == 0 {
		return nil
	}
	data, err := os.ReadFile(session.RawPath)
	if err != nil {
		return nil
	}

	servedAtLine := -1
	editAtLine := -1

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}

		if servedAtLine == -1 {
			for _, marker := range obligationsHookMarkers {
				if bytes.Contains(line, []byte(marker)) {
					servedAtLine = lineNo
					break
				}
			}
		}

		if editAtLine == -1 && line[0] == '{' {
			var ev streamToolEvent
			if err := json.Unmarshal(line, &ev); err == nil && ev.Type == "assistant" {
			contentLoop:
				for _, c := range ev.Message.Content {
					if c.Type != "tool_use" {
						continue
					}
					file := editTargetFile(c.Name, c.Input)
					if file == "" {
						continue
					}
					for _, gp := range governedPaths {
						if sameFile(file, gp) {
							editAtLine = lineNo
							break contentLoop
						}
					}
				}
			}
		}

		if servedAtLine != -1 && editAtLine != -1 {
			break
		}
	}

	if editAtLine == -1 {
		return nil // no edit of a governed path this session: not a measurable question
	}
	served := servedAtLine != -1 && servedAtLine < editAtLine
	return &served
}
