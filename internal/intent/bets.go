package intent

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
)

// bets.go — DP-4 (docs/plans/custodian.md; vision 23 §2 "GOALS gated
// hard"): a goal is a pre-registered bet — an instrument a human can run, a
// threshold it is judged against, and a date it is reviewed on — or it is
// refused at load. The uncheckable-concern refusal, applied to ambition: a
// goal of adjectives never becomes a fathom artifact.
//
// The declaration is a block on the goal's own `Done when:` condition,
// tokenized by the plan grammar's tokenizer exactly as an `{oracle: …}`
// block is (one grammar, one quoting rule):
//
//	**G2. The A/B becomes admissible.** Done when: … a keep/retract verdict …
//	{bet: arm-bench, instrument: "fathom flywheel", threshold: "≥20 …", review: 2026-10-25}
//
// Nothing here evaluates the threshold. The instrument is RESOLVED (it must
// name something runnable here), the date is parsed, and a passed review
// date is listed as due — the keep/kill decision is a human's, recorded on
// the goal (law 12). Fathom never edits scope.md.

// betKeys are the keys of a bet block. A block made only of these is a
// pre-registration, never an oracle annotation (Condition.Annotated).
var betKeys = map[string]bool{"bet": true, "instrument": true, "threshold": true, "review": true}

// GoalBet is one goal's verdict at load: a bet, or refused with the reason.
type GoalBet struct {
	Goal Goal
	// Condition is the id of the condition carrying the bet block ("G2#1").
	Condition  string
	Name       string // the optional `bet:` name ("arm-bench")
	Instrument string
	// InstrumentKind says how the instrument resolved: "fathom verb",
	// "fathom verb (unchecked)" (no resolver was given), "task target", or
	// "repo script".
	InstrumentKind string
	Threshold      string
	ReviewRaw      string
	Review         time.Time
	// Refused is non-empty when the goal is NOT a bet; it names every
	// missing or invalid part, so the remedy is exact.
	Refused string
}

// LoadBets returns every goal of the corpus in file order, each a bet or a
// refusal, and the manifest of what resolving their instruments read
// outside the corpus (Taskfile.yml, a script's path). isVerb resolves
// `fathom <verb>` instruments; nil leaves them unchecked (named as such,
// never guessed either way). An instrument the reader could not see — a
// Taskfile or script that exists and cannot be read — is likewise
// unchecked and named on the manifest, never a refusal that blames the
// author for the reader's blindness (ADR-027, I38).
func LoadBets(c Corpus, isVerb func(string) bool) ([]GoalBet, readmanifest.Manifest) {
	r := &instrumentResolver{root: c.Root, isVerb: isVerb, reads: readmanifest.NewSet()}
	r.reads.RelativeTo(c.Root)
	out := make([]GoalBet, 0, len(c.Goals))
	for _, g := range c.Goals {
		out = append(out, loadBet(c, g, r))
	}
	return out, r.reads.Manifest()
}

// MisplacedBets names every bet block — a block carrying any bet key — on
// a condition that is not a scope.md goal's. LoadBets reads only goals and
// Condition.Annotated never counts a bet-only block toward K0, so without
// this list such a block would be read by nothing and named by nothing.
func MisplacedBets(c Corpus) []Claim {
	goals := map[string]bool{}
	for _, g := range c.Goals {
		goals[g.File+"\x00"+g.ID] = true
	}
	var out []Claim
	for _, cond := range c.Conditions {
		if goals[cond.File+"\x00"+cond.Record] {
			continue
		}
		var keys []string
		for k := range cond.Ann {
			if betKeys[k] {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 {
			continue
		}
		sort.Strings(keys)
		ref := cond.Record
		if ref == "" {
			ref = cond.File
		}
		out = append(out, Claim{
			Kind: "misplaced-bet", Ref: ref,
			Detail:   fmt.Sprintf("its %s carries a bet block (%s) — only a scope.md goal's Done when is a bet, so nothing reads this one", cond.Label, strings.Join(keys, ", ")),
			Remedy:   "move the block to the scope.md goal it measures, or drop it; a bet block evaluates nothing where it is (and is no oracle annotation)",
			Evidence: cond.File + ":" + itoa(cond.Line),
		})
	}
	return out
}

func loadBet(c Corpus, g Goal, r *instrumentResolver) GoalBet {
	gb := GoalBet{Goal: g}
	var conds []Condition
	for _, cond := range c.Conditions {
		if cond.File == g.File && cond.Record == g.ID {
			conds = append(conds, cond)
		}
	}
	if len(conds) == 0 {
		gb.Refused = "no `Done when:` condition — a bet needs a threshold to be judged against"
		return gb
	}
	var block *Condition
	for i := range conds {
		for k := range conds[i].Ann {
			if betKeys[k] {
				block = &conds[i]
				break
			}
		}
		if block != nil {
			break
		}
	}
	if block == nil {
		gb.Condition = conds[0].ID
		gb.Refused = "its Done when carries no bet block — missing instrument, threshold, review"
		return gb
	}
	gb.Condition = block.ID
	gb.Name = strings.TrimSpace(block.Ann["bet"])
	gb.Instrument = strings.TrimSpace(block.Ann["instrument"])
	gb.Threshold = strings.TrimSpace(block.Ann["threshold"])
	gb.ReviewRaw = strings.TrimSpace(block.Ann["review"])

	var missing []string
	for _, kv := range [][2]string{{"instrument", gb.Instrument}, {"threshold", gb.Threshold}, {"review", gb.ReviewRaw}} {
		if kv[1] == "" {
			missing = append(missing, kv[0])
		}
	}
	var why []string
	if len(missing) > 0 {
		why = append(why, "missing "+strings.Join(missing, ", "))
	}
	if gb.ReviewRaw != "" {
		d, err := time.Parse("2006-01-02", gb.ReviewRaw)
		if err != nil {
			why = append(why, fmt.Sprintf("review %q is not a YYYY-MM-DD date", gb.ReviewRaw))
		} else {
			gb.Review = d
		}
	}
	if gb.Instrument != "" {
		kind, reason := r.resolve(gb.Instrument)
		if reason != "" {
			why = append(why, reason)
		}
		gb.InstrumentKind = kind
	}
	if len(why) > 0 {
		gb.Refused = strings.Join(why, "; ")
	}
	return gb
}

// instrumentResolver resolves bet instruments, reading Taskfile.yml once
// and only when a `task` instrument needs it, and recording every read
// outside the corpus on its manifest.
type instrumentResolver struct {
	root    string
	isVerb  func(string) bool
	reads   *readmanifest.Set
	loaded  bool
	targets map[string]bool
	tfErr   error // Taskfile.yml exists and could not be read
}

func (r *instrumentResolver) taskTargets() (map[string]bool, error) {
	if !r.loaded {
		r.targets, r.tfErr = readTaskTargets(r.root, r.reads)
		r.loaded = true
	}
	return r.targets, r.tfErr
}

// resolve checks that an instrument names something a human can run in
// this repo: a registered `fathom` verb, a Taskfile target, or a script in
// the tree. Anything else — prose, a function name, a command this repo
// cannot resolve — is refused with the accepted forms named. What the
// reader cannot see is unchecked, never refused.
func (r *instrumentResolver) resolve(instrument string) (kind, refusal string) {
	fields := strings.Fields(strings.Trim(instrument, "`"))
	if len(fields) == 0 {
		return "", "instrument is empty"
	}
	switch fields[0] {
	case "fathom":
		if len(fields) < 2 {
			return "", "instrument `fathom` names no verb"
		}
		if r.isVerb == nil {
			return "fathom verb (unchecked)", ""
		}
		if !r.isVerb(fields[1]) {
			return "", fmt.Sprintf("instrument `%s` names no registered fathom verb (%q)", instrument, fields[1])
		}
		return "fathom verb", ""
	case "task":
		targets, err := r.taskTargets()
		if err != nil {
			return "task target (unchecked — Taskfile.yml could not be read: " + errnoOf(err) + ")", ""
		}
		if len(fields) >= 2 && targets[fields[1]] {
			return "task target", ""
		}
		return "", fmt.Sprintf("instrument `%s` names no Taskfile target", instrument)
	}
	if strings.Contains(fields[0], "/") && !filepath.IsAbs(fields[0]) && !strings.HasPrefix(fields[0], "..") {
		abs := filepath.Join(r.root, filepath.FromSlash(fields[0]))
		present, blind := statPresence(abs)
		if blind != nil {
			r.reads.Record(abs, readmanifest.Unreadable(blind))
			return "repo script (unchecked — " + fields[0] + " could not be read: " + errnoOf(blind) + ")", ""
		}
		if present {
			return "repo script", ""
		}
		return "", fmt.Sprintf("instrument `%s` names a script that is not in the tree", instrument)
	}
	return "", fmt.Sprintf("instrument `%s` is not runnable here — name a `fathom <verb>`, a `task <target>`, or a repo script", instrument)
}

// BetFindings lists what the bet surface must say out loud: every refused
// goal, and every bet whose review date has passed (the decision is due —
// a human records keep or kill on the goal; fathom never does).
func BetFindings(bets []GoalBet, now time.Time) []Claim {
	var out []Claim
	today := now.UTC().Truncate(24 * time.Hour)
	for _, b := range bets {
		ev := b.Goal.File + ":" + itoa(b.Goal.Line)
		if b.Refused != "" {
			out = append(out, Claim{
				Kind: "refused", Ref: b.Goal.ID,
				Detail:   "not a pre-registered bet: " + b.Refused,
				Remedy:   "add {instrument: …, threshold: …, review: YYYY-MM-DD} to its Done when, or drop the goal — a goal without an instrument is not a fathom artifact (vision 23 §2)",
				Evidence: ev,
			})
			continue
		}
		if !b.Review.IsZero() && b.Review.Before(today) {
			days := int(today.Sub(b.Review).Hours() / 24)
			out = append(out, Claim{
				Kind: "review-due", Ref: b.Goal.ID,
				Detail:   fmt.Sprintf("review date %s passed %d day(s) ago — run `%s` against %q", b.ReviewRaw, days, b.Instrument, b.Threshold),
				Remedy:   "decide it on the goal: keep (with a new review date), kill, or record the verdict (law 12) — fathom never decides",
				Evidence: ev,
			})
		}
	}
	return out
}
