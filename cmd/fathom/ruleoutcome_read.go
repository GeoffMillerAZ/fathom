package main

// ruleoutcome_read.go — SR-61's reader half (surface reckoning F-27/U4,
// design §6). ruleoutcome.go writes the signal log and ruleoutcome_track.go
// feeds it served/still_served/top_changed per claim id from the edit hook; this
// reads it back as the rows internal/semantic.RuleEffectiveness (GL-11)
// projects and internal/console.RuleOutcomesView renders, for `fathom
// obligations` and the serving explainer.
//
// The reader mirrors the writer's own bounds: one rotated generation
// (path.1, then path, exactly what dlog.RotateIfNeeded leaves), each at most
// ruleOutcomeLogRotateBytes. Only the last 90 days of rows are kept in
// memory — the widest window GL-11 reads — and the parsed result is reused
// until either file changes, so a console page that asks on every request
// re-reads nothing while no hook has fired.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// ruleOutcomeKeepDays is how much of the log the reader holds: GL-11's
// never-triggered window, the widest it reads.
const ruleOutcomeKeepDays = 90

// ruleOutcomeLog is one read of the signal log.
type ruleOutcomeLog struct {
	Events []semantic.RuleOutcomeEvent
	Read   console.RuleOutcomeRead
}

// readRuleOutcomeLog reads path's rotated generation then path itself. A
// missing file is not an error (no hook has recorded anything yet:
// Read.Present false); any other failure is Read.Err, never a silent empty
// log; a row that does not decode is counted in Read.Skipped and left out.
// "" (no resolvable log path) reads as no log. Every row's event is read under
// the current vocabulary (semantic.CanonicalRuleEvent, JF-2): a row an older
// binary wrote as complied or violated is a top_changed or a still_served
// here, never dropped.
func readRuleOutcomeLog(path string, now time.Time) ruleOutcomeLog {
	return readRuleOutcomeLogFrom(path, now.AddDate(0, 0, -ruleOutcomeKeepDays))
}

// readRuleOutcomeLogFrom is readRuleOutcomeLog keeping every row at or
// after keepFrom (the zero time keeps the whole log): `fathom claim learn`
// (GL-13) reads never-triggered retirement's 180-day window, wider than
// the 90 days the console holds.
func readRuleOutcomeLogFrom(path string, keepFrom time.Time) ruleOutcomeLog {
	var out ruleOutcomeLog
	if path == "" {
		return out
	}
	for _, p := range []string{path + ".1", path} {
		f, err := os.Open(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			out.Read.Err = fmt.Errorf("%w%s", err, tccWedgeHint(p))
			return ruleOutcomeLog{Read: out.Read}
		}
		out.Read.Present = true
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 {
				continue
			}
			var row struct {
				TS      string `json:"ts"`
				RuleID  string `json:"rule_id"`
				Event   string `json:"event"`
				Session string `json:"session"`
			}
			if json.Unmarshal(line, &row) != nil || row.RuleID == "" || row.Event == "" {
				out.Read.Skipped++
				continue
			}
			at, terr := time.Parse(time.RFC3339Nano, row.TS)
			if terr != nil {
				out.Read.Skipped++
				continue
			}
			out.Read.Rows++
			if out.Read.Since.IsZero() || at.Before(out.Read.Since) {
				out.Read.Since = at
			}
			if at.Before(keepFrom) {
				continue
			}
			out.Events = append(out.Events, semantic.RuleOutcomeEvent{RuleID: row.RuleID, Event: semantic.CanonicalRuleEvent(row.Event), At: at, Session: row.Session})
		}
		serr := sc.Err()
		f.Close()
		if serr != nil {
			// An incomplete scan discards the prefix: a partial read is
			// never presented as the whole log.
			out.Read.Err = fmt.Errorf("%w%s", serr, tccWedgeHint(p))
			return ruleOutcomeLog{Read: out.Read}
		}
	}
	return out
}

// ruleOutcomeCache holds the last parse, keyed by both files' identity.
var ruleOutcomeCache struct {
	sync.Mutex
	key string
	log ruleOutcomeLog
}

// ruleOutcomeFileKey names path's and its rotated generation's current
// size and mtime; "" when neither can be statted (nothing to reuse).
func ruleOutcomeFileKey(path string) string {
	key := path
	for _, p := range []string{path + ".1", path} {
		fi, err := os.Stat(p)
		if err != nil {
			key += "|-"
			continue
		}
		key += fmt.Sprintf("|%d@%d", fi.Size(), fi.ModTime().UnixNano())
	}
	return key
}

// cachedRuleOutcomeLog is readRuleOutcomeLog reused until either file
// changes. An error read is never cached: the next call tries again.
func cachedRuleOutcomeLog(path string, now time.Time) ruleOutcomeLog {
	if path == "" {
		return ruleOutcomeLog{}
	}
	key := ruleOutcomeFileKey(path)
	ruleOutcomeCache.Lock()
	if ruleOutcomeCache.key == key && ruleOutcomeCache.log.Read.Err == nil {
		log := ruleOutcomeCache.log
		ruleOutcomeCache.Unlock()
		return log
	}
	ruleOutcomeCache.Unlock()
	log := readRuleOutcomeLog(path, now)
	if log.Read.Err == nil {
		ruleOutcomeCache.Lock()
		ruleOutcomeCache.key, ruleOutcomeCache.log = key, log
		ruleOutcomeCache.Unlock()
	}
	return log
}

// withRuleOutcomes attaches the hook's per-rule record to a serving
// explanation: the served claim ids (a raw-fallback row has none) counted
// over the signal log. It runs AFTER the explanation cache, at request
// time, because a hook can record between two page loads without any event
// moving the cache's cursor.
func withRuleOutcomes(se console.ServingExplanation, now time.Time) console.ServingExplanation {
	ids := make([]string, 0, len(se.AgentSees))
	for _, r := range se.AgentSees {
		ids = append(ids, r.ClaimID)
	}
	log := cachedRuleOutcomeLog(ruleOutcomeLogPath(), now)
	se.Outcomes = console.RuleOutcomesView(ids, log.Events, log.Read, now)
	return se
}
