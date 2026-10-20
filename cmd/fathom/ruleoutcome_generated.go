package main

// ruleoutcome_generated.go -- FD-22: the rule-outcome log records each
// served generated line as served, and the session's next edit as
// next_edit_source (it touched a declared source), next_edit_output (the
// output again) or next_edit_other. State is the log itself: no new file,
// table or job, and no model. This feeds FD-M1's pre-registered reading
// (docs/design/fathom-dir.md 7.5 gate 6).

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/techpack"
)

const (
	ruleOutcomeNextEditSource = "next_edit_source"
	ruleOutcomeNextEditOutput = "next_edit_output"
	ruleOutcomeNextEditOther  = "next_edit_other"

	generatedRulePrefix = "generated:"
	// generatedLookback bounds how much of the log's tail is read to find
	// the session's pending generated line.
	generatedLookback int64 = 256 << 10
)

// recordGeneratedServed logs a served generated line (no-op without a
// session to join the next edit to).
func recordGeneratedServed(session, rel string, g *generatedServe) {
	if g == nil || session == "" {
		return
	}
	appendRuleOutcome(ruleOutcomeEvent{
		RuleID: generatedRulePrefix + g.output, Event: ruleOutcomeServed,
		Session: session, File: rel, Output: g.output, Sources: g.sources,
	})
}

// resolveGeneratedNextEdit reads the log's tail for the session's pending
// generated line (served, no next_edit row after it) and, when there is one,
// records what this edit touched. Any failure is silence.
func resolveGeneratedNextEdit(session, projDir, filePath string) {
	if session == "" {
		return
	}
	path := ruleOutcomeLogPath()
	if path == "" {
		return
	}
	pending := pendingGeneratedLine(path, session)
	if pending == nil {
		return
	}
	rel := filePath
	for _, cand := range serviceCandidates(projDir) {
		if r, ok := relativeToRoot(cand.root, filePath); ok {
			rel = r
			break
		}
	}
	event := ruleOutcomeNextEditOther
	switch {
	case rel == pending.Output:
		event = ruleOutcomeNextEditOutput
	case touchesSource(pending.Sources, rel):
		event = ruleOutcomeNextEditSource
	}
	recordRuleOutcome(pending.RuleID, event, session, rel)
}

func touchesSource(sources []string, rel string) bool {
	for _, s := range sources {
		if s == rel || (techpack.IsGlob(s) && techpack.GlobMatch(s, rel)) {
			return true
		}
	}
	return false
}

func pendingGeneratedLine(path, session string) *ruleOutcomeEvent {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil
	}
	var r io.Reader = f
	dropFirst := false
	if fi.Size() > generatedLookback {
		if _, err := f.Seek(fi.Size()-generatedLookback, io.SeekStart); err != nil {
			return nil
		}
		dropFirst = true
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	var pending *ruleOutcomeEvent
	for sc.Scan() {
		if dropFirst {
			dropFirst = false
			continue
		}
		var e ruleOutcomeEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Session != session || !strings.HasPrefix(e.RuleID, generatedRulePrefix) {
			continue
		}
		switch {
		case e.Event == ruleOutcomeServed:
			cp := e
			pending = &cp
		case strings.HasPrefix(e.Event, "next_edit_") && pending != nil && e.RuleID == pending.RuleID:
			pending = nil
		}
	}
	return pending
}
