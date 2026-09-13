package scenariobench

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ruleserved.go is SB-25 (vision 27 §4, M1): did an obligation naming a
// guards task's rule reach the wired session before its first `[x]` claim
// (the obligation ceiling, Q21, as a per-task number)?
//
// Honest scope: cmd/fathom's real edit-hook beat log
// (~/.fathom/hooks.jsonl, heartbeat.Beat JSONL — beats.go, edithook.go)
// carries a rendered-RULE-COUNT and a wall-clock timestamp today, never an
// individual rule id and never a turn number (postEditMeta.rules is
// len(renderRules), an int). RuleServed cannot read a rule id out of
// today's beats because the beats do not carry one — reporting a false
// precise answer would violate this repo's own instrument-honesty
// doctrine (I38) more than a documented approximation does. So this
// reader is forward-compatible plumbing: it looks for a beat whose meta
// carries the rule id under an explicit "rule" or "rules" key (a
// comma-separated list), and for a "turn" meta key giving the beat's
// session-turn number; a future, richer beat writer can start stamping
// both without this reader's contract changing. Absent an explicit "turn"
// key, ordinal position among matching beats (chronological, since a beat
// log is append-only) stands in for turn number — a loose proxy, and the
// coarsest part of this reading.
//
// hooksLog is one arm's own beat log for one cell (never the shared
// per-user ~/.fathom/hooks.jsonl during a scenario run — the harness scopes
// FATHOM_HOOKS_LOG per cell so concurrent cells cannot interleave).

// ruleServedBeat mirrors third_party/agentic-kit's
// foundations/telemetry/heartbeat.Beat JSONL shape: {"channel":...,
// "kind":...,"at":...,"meta":{...}}.
type ruleServedBeat struct {
	Channel string            `json:"channel"`
	Kind    string            `json:"kind"`
	At      time.Time         `json:"at"`
	Meta    map[string]string `json:"meta"`
}

// beatCarriesRule reports whether b names ruleID, via an explicit "rule"
// meta key or a comma-separated "rules" meta key.
func beatCarriesRule(b ruleServedBeat, ruleID string) bool {
	if b.Meta == nil {
		return false
	}
	if b.Meta["rule"] == ruleID {
		return true
	}
	for _, tok := range strings.Split(b.Meta["rules"], ",") {
		if strings.TrimSpace(tok) == ruleID {
			return true
		}
	}
	return false
}

// beatTurn returns b's explicit "turn" meta value and true when present
// and parseable.
func beatTurn(b ruleServedBeat) (int, bool) {
	if b.Meta == nil {
		return 0, false
	}
	n, err := strconv.Atoi(b.Meta["turn"])
	if err != nil {
		return 0, false
	}
	return n, true
}

// RuleServed is SB-25: nil when hooksLog cannot be read at all (no log —
// the reading is not_measurable, never a false "no"). Otherwise it scans
// the edit-hook beats naming ruleID (kind "seen" or "answered" — either is
// evidence the obligation was rendered for this session), in chronological
// order, and returns whether the FIRST such beat precedes firstClaimTurn:
// by its own "turn" meta when the beat carries one, else by its ordinal
// position among matching beats (the 1st matching beat stands in for turn
// 1, the 2nd for turn 2, …). A log with no matching beat at all returns
// false — no evidence the rule ever reached the session.
func RuleServed(hooksLog string, ruleID string, firstClaimTurn int) *bool {
	f, err := os.Open(hooksLog)
	if err != nil {
		return nil
	}
	defer f.Close()

	var matches []ruleServedBeat
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var b ruleServedBeat
		if err := json.Unmarshal([]byte(line), &b); err != nil {
			continue
		}
		if b.Channel != "edit-hook" {
			continue
		}
		if b.Kind != "seen" && b.Kind != "answered" {
			continue
		}
		if beatCarriesRule(b, ruleID) {
			matches = append(matches, b)
		}
	}
	if sc.Err() != nil && len(matches) == 0 {
		return nil // an unreadable log reads as no log, not as "not served"
	}

	served := false
	if len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool { return matches[i].At.Before(matches[j].At) })
		first := matches[0]
		turn, ok := beatTurn(first)
		if !ok {
			turn = 1 // ordinal fallback: first matching beat ~ turn 1
		}
		served = turn < firstClaimTurn
	}
	return &served
}
