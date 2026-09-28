// ruleoutcome.go — GL-10 (docs/plans/governance-learning.md, design doc §5
// "The signal source and rule effectiveness"): every hook decision already
// passes through the edit and PreToolUse hooks (ADR-037/038, guarded
// edits); this is where those decisions become a per-rule SIGNAL instead
// of evaporating the moment the hook process exits. recordRuleOutcome
// appends one JSONL row per rule per event — served, complied, violated,
// blocked, plus the existing refuted/corroborated human claim verdicts
// (`fathom claim refute`/`corroborate`, wired in claim.go) — to a bounded,
// rotated log under ~/.fathom/, NEVER the transcript (GL-X4: counts and
// tool events only, no prose inference over transcripts, ADR-041 decision
// 8).
//
// Precedents this file reuses rather than re-inventing: internal/mcp's
// call-log JSONL shape (internal/mcp/calllog.go), the beat log's rotation
// (cmd/fathom/beats.go's rotateBeatLogIfNeeded, itself a thin wrapper over
// internal/dlog.RotateIfNeeded).
//
// internal/semantic.RuleEffectiveness (GL-11) is this log's reader — the
// per-rule counts-and-rates projection that classifies each rule as
// ignored / never triggered / effective / contested (design doc §5's
// table).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// Rule outcome event kinds (design doc §5's table plus the existing claim
// verdicts). Every event recordRuleOutcome accepts is one of these six
// literals; any other string is still recorded as-is (the log is an
// append-only signal, not a closed enum enforced at the write site) but
// internal/semantic.RuleEffectiveness (GL-11) only recognizes these.
const (
	ruleOutcomeServed       = "served"
	ruleOutcomeComplied     = "complied"
	ruleOutcomeViolated     = "violated"
	ruleOutcomeBlocked      = "blocked"
	ruleOutcomeRefuted      = "refuted"
	ruleOutcomeCorroborated = "corroborated"
)

// ruleOutcomeSchemaVersion identifies this record's field semantics (mirrors
// callLogSchemaVersion's convention, internal/mcp/calllog.go) — bump on any
// semantic change to an existing field; additive fields alone do not
// require a bump.
const ruleOutcomeSchemaVersion = 1

// ruleOutcomeLogRotateBytes bounds the signal log's on-disk size at 16 MiB
// (design doc §5's own stated cap, plan row GL-10) — smaller than the 64
// MiB hooks beat log because every row here is one small fixed-shape
// event, never a beat's free-form meta. A package var (not a const) so
// tests can shrink it rather than writing a real 16 MiB fixture file
// (mirrors beatLogRotateBytes's own reason, cmd/fathom/beats.go).
var ruleOutcomeLogRotateBytes int64 = 16 << 20

// ruleOutcomeNow is the injected clock. cmd/fathom is a log-writer, not
// internal/'s query/analysis logic (internal/CLAUDE.md's determinism
// contract targets THAT layer's time.Now ban) — this package var mirrors
// cmd/fathom's own existing precedent (certifyGapsNow, callLogNow) so a
// test can pin the timestamp without a real wall-clock race.
var ruleOutcomeNow = time.Now

// ruleOutcomeLogPath resolves the signal log's path: FATHOM_RULE_OUTCOME_LOG
// overrides for tests (mirrors beatLogPath's FATHOM_HOOKS_LOG override,
// cmd/fathom/beats.go), defaulting to ~/.fathom/rule-outcomes.jsonl — a
// per-user central log, since hooks fire in every fleet repo, exactly like
// the hooks beat log. "" (no override, no resolvable home dir) means
// "cannot record" — recordRuleOutcome's caller treats that as silence.
func ruleOutcomeLogPath() string {
	if p := os.Getenv("FATHOM_RULE_OUTCOME_LOG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".fathom", "rule-outcomes.jsonl")
}

// ruleOutcomeEvent is one JSONL row.
type ruleOutcomeEvent struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`
	TS            string `json:"ts"`
	RuleID        string `json:"rule_id"`
	Event         string `json:"event"`
	// Session/File are the join keys the effectiveness projection and any
	// future retro tooling need (mirrors firingMeta's session/file
	// convention, cmd/fathom/beats.go); both may be "" when the caller has
	// neither (e.g. a claim verdict recorded outside any live edit).
	Session string `json:"session,omitempty"`
	File    string `json:"file,omitempty"`
}

// recordRuleOutcome appends one signal-log row for ruleID's event. Both
// ruleID and event are required — a row with either blank would be
// unattributable to a rule or meaningless to classify, so it is silently
// dropped rather than half-recorded.
//
// Every other failure (no resolvable log path, mkdir, open, marshal,
// write) is ALSO swallowed: recording a signal must never be the reason a
// hook or a claim-verdict command breaks its own contract — the exact
// silent-failure discipline emitBeat already established
// (cmd/fathom/beats.go).
func recordRuleOutcome(ruleID, event, session, file string) {
	if ruleID == "" || event == "" {
		return
	}
	path := ruleOutcomeLogPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = dlog.RotateIfNeeded(path, ruleOutcomeLogRotateBytes)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	entry := ruleOutcomeEvent{
		SchemaVersion: ruleOutcomeSchemaVersion,
		FathomVersion: buildinfo.Version(),
		TS:            ruleOutcomeNow().UTC().Format(time.RFC3339Nano),
		RuleID:        ruleID,
		Event:         event,
		Session:       session,
		File:          file,
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// One record = one Write: the file is opened O_APPEND, so a single
	// write keeps each JSONL line atomic even across a concurrent hook
	// invocation (mirrors internal/mcp/calllog.go's CallLogger.log doc
	// comment).
	b = append(b, '\n')
	_, _ = f.Write(b)
}

// ruleOutcomeReadMax bounds a read of the signal log to the size the writer
// rotates it at (GL-10's 16 MiB): a log past it — a rotation not yet run —
// is read from its newest bytes, so a request never reads more than the
// writer would have kept. A var so a test can shrink it (as
// ruleOutcomeLogRotateBytes is).
var ruleOutcomeReadMax int64 = 16 << 20

// readRuleOutcomeSignal reads the signal log at path for the rule-
// effectiveness view (GL-18): the newest ruleOutcomeReadMax bytes, whole
// lines only, each row that parses becoming one semantic.RuleOutcomeEvent.
// A missing log is an absent signal, never a signal of nothing; a log that
// exists and cannot be read is an error naming why; a row that does not parse
// is counted in Skipped, not silently dropped. The rotated archive is not
// read: the view's windows are 30 and 90 days and it says how far back the
// log it read reaches.
func readRuleOutcomeSignal(path string) console.RuleSignal {
	sig := console.RuleSignal{Source: path}
	if path == "" {
		sig.State, sig.Reason = console.StateAbsent, "no home directory to keep a rule-outcome log in"
		return sig
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			sig.State, sig.Reason = console.StateAbsent, "no rule-outcome log at "+path+": the edit hook records one row for each rule it serves"
			return sig
		}
		sig.State, sig.Reason = console.StateError, err.Error()
		return sig
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		sig.State, sig.Reason = console.StateError, err.Error()
		return sig
	}
	var r io.Reader = f
	dropFirst := false
	if fi.Size() > ruleOutcomeReadMax {
		if _, err := f.Seek(fi.Size()-ruleOutcomeReadMax, io.SeekStart); err != nil {
			sig.State, sig.Reason = console.StateError, err.Error()
			return sig
		}
		dropFirst = true // the first line is cut mid-row
	}
	sc := bufio.NewScanner(io.LimitReader(r, ruleOutcomeReadMax))
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	sig.State = console.StatePresent
	for sc.Scan() {
		if dropFirst {
			dropFirst = false
			continue
		}
		var e ruleOutcomeEvent
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil || e.RuleID == "" || e.Event == "" {
			sig.Skipped++
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, e.TS)
		if err != nil {
			sig.Skipped++
			continue
		}
		sig.Events = append(sig.Events, semantic.RuleOutcomeEvent{RuleID: e.RuleID, Event: e.Event, At: at.UTC(), Session: e.Session})
	}
	if err := sc.Err(); err != nil {
		sig.State, sig.Reason, sig.Events = console.StateError, fmt.Sprintf("reading %s: %v", path, err), nil
	}
	return sig
}
