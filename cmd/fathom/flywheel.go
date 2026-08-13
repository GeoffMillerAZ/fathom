package main

// flywheel.go — `fathom flywheel`: is each feedback loop actually closed?
//
// Owner ask, 2026-08-08: *"periodically check in on our flywheel and
// feedback loops to make sure they are working as expected. They are so
// critical to the feedback and tuning we'll need to advance the product to
// excellence."*
//
// The durable answer to "check periodically" is not a habit, it is a verb.
// Two failures in one day argued for it, and both were silent:
//
//   1. The OFF arm silenced its own measurement. Every hook exited on a
//      merged gate before recording anything, so the arm burned its
//      session budget collecting NOTHING for fourteen hours. Every
//      dashboard looked fine: no error, no warning, just an instrument
//      quietly reading zero.
//   2. Hours after that was fixed, the first OFF rows arrived CORRUPT —
//      sessions whose transcripts began ten days before the arm existed,
//      stamped arm="off" while carrying assisted activity.
//
// Neither was visible in any existing surface, because every surface asked
// "what do the numbers say?" and none asked "is the pipe connected?".
//
// A loop is CLOSED only when data is produced, arrives, and is consumed.
// Producing without consuming is a loop that is open at the far end —
// which looks identical to a healthy one from the producing side, and is
// exactly how both of today's failures hid.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// flywheelStatus is deliberately three-valued. "Broken" and "nothing has
// happened yet" demand opposite responses, and collapsing them into a
// binary is what makes a health check cry wolf until it is ignored.
type flywheelStatus string

const (
	loopClosed flywheelStatus = "CLOSED" // producing, arriving, consumed
	loopOpen   flywheelStatus = "OPEN"   // a defect — data should be flowing and is not
	loopIdle   flywheelStatus = "IDLE"   // honestly nothing to measure yet
)

// flywheelCheck is one loop.
type flywheelCheck struct {
	Loop string `json:"loop"`
	// What the loop is FOR, in one line, for a reader who does not already
	// know. A health check nobody can interpret is not a health check.
	Purpose  string         `json:"purpose"`
	Status   flywheelStatus `json:"status"`
	Evidence string         `json:"evidence"` // the numbers behind the verdict — always present
	Remedy   string         `json:"remedy,omitempty"`
}

func cmdFlywheel(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("flywheel", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	asJSON := fs.Bool("json", false, "emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	now := time.Now()
	checks := runFlywheelChecks(*url, now)

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{
			"generated_at": now.UTC().Format(time.RFC3339),
			"loops":        checks,
		}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return flywheelExit(checks)
	}

	closed, open, idle := 0, 0, 0
	fmt.Fprintf(stdout, "fathom flywheel — are the feedback loops closed?\n\n")
	for _, c := range checks {
		switch c.Status {
		case loopClosed:
			closed++
		case loopOpen:
			open++
		default:
			idle++
		}
		fmt.Fprintf(stdout, "  %-6s %-22s %s\n", c.Status, c.Loop, c.Evidence)
		if c.Remedy != "" {
			fmt.Fprintf(stdout, "  %-6s %-22s → %s\n", "", "", c.Remedy)
		}
	}
	fmt.Fprintf(stdout, "\n%d loops: %d closed, %d open, %d idle\n", len(checks), closed, open, idle)
	if open > 0 {
		fmt.Fprintln(stdout, "\nAn OPEN loop is collecting nothing while looking healthy from the producing side.")
		fmt.Fprintln(stdout, "That is how both 2026-08-08 measurement failures hid; treat it as urgent.")
	}
	return flywheelExit(checks)
}

// flywheelExit: 1 when any loop is OPEN. IDLE never fails — a loop with
// nothing to measure yet is not a defect, and failing on it would train
// people to ignore the exit code.
func flywheelExit(checks []flywheelCheck) int {
	for _, c := range checks {
		if c.Status == loopOpen {
			return 1
		}
	}
	return 0
}

func runFlywheelChecks(baseURL string, now time.Time) []flywheelCheck {
	ledger := readLedgerRows(sessionLedgerPath())
	imp := buildImpactModel(now)
	// Obligation traffic lives in the CALL LOG, not the impact model —
	// buildObservatoryModel folds it separately. Reading the wrong source
	// here produced a false IDLE on a loop carrying 6,992 injections, which
	// is precisely the class of error this verb exists to catch. Every
	// check must read the same source the surface it mirrors reads.
	entries := flywheelCallLog(baseURL)
	obligationCalls := 0
	for _, e := range entries {
		if e.Tool == "obligations" {
			obligationCalls++
		}
	}
	return []flywheelCheck{
		checkLoopSessionLedger(ledger, now),
		checkLoopArm(ledger, imp, now),
		checkLoopCallLogEntries(entries, now),
		checkLoopObligations(obligationCalls),
		checkLoopVerifyGate(imp),
		checkLoopExploration(imp),
		checkLoopObservations(),
		checkLoopContinuity(),
	}
}

// checkLoopContinuity — vision 21's conveyor. CLOSED needs the loop to
// actually MOVE: deposits landing AND briefs being injected. Briefs
// injected against a store nobody deposits to is a conveyor carrying
// nothing — OPEN, not healthy-looking; that asymmetry is the flywheel's
// founding rule.
func checkLoopContinuity() flywheelCheck {
	c := flywheelCheck{
		Loop:    "continuity",
		Purpose: "threads carry intent across /clear and compaction; the bet: fresh sessions get as good as resumed, cheaper",
	}
	items := loadThreadItems(threadsPath())
	rows := dedupedLedgerRows()
	bet := threadBetCheck(rows)

	deposited, briefed := len(items) > 0, false
	for _, r := range rows {
		if r.BriefInjected {
			briefed = true
			break
		}
	}
	switch {
	case !deposited && !briefed:
		c.Status = loopIdle
		c.Evidence = "no thread items and no briefs yet — the feature is present but nothing has flowed"
		c.Remedy = "file one: `fathom thread note` — or just /clear a session with work in flight (the hook deposits mechanically)"
	case briefed && !deposited:
		c.Status = loopOpen
		c.Evidence = "briefs are being injected but NOTHING has ever been deposited — the conveyor is moving empty"
		c.Remedy = "check the SessionEnd/PreCompact hooks are registered (fathom doctor) — an injection with no deposits spends tokens on nothing"
	default:
		c.Status = loopClosed
		open := 0
		for _, it := range items {
			if it.ClosedAt == "" {
				open++
			}
		}
		c.Evidence = fmt.Sprintf("%s item(s), %s open; bet %s — %s",
			fmtThousands(len(items)), fmtThousands(open), bet.Verdict, bet.Detail)
	}
	return c
}

// ledgerRow is the subset the flywheel reads. It parses defensively: a row
// it cannot decode is skipped rather than failing the check, because a
// health check that dies on one malformed line is worse than none.
type ledgerRow struct {
	At          string `json:"at"`
	Arm         string `json:"arm"`
	ArmCoverage string `json:"arm_coverage"`
	// ArmWindow: only presence and cleanliness matter to the flywheel.
	ArmWindow *struct {
		Turns     *int `json:"turns"`
		Truncated bool `json:"truncated"`
	} `json:"arm_window"`
	Repo        string `json:"repo"`
	TokensIn    *int   `json:"tokens_in"`
	TokensOut   *int   `json:"tokens_out"`
	FathomCalls int    `json:"fathom_calls"`
	RawCalls    int    `json:"raw_discovery_calls"`
}

// HasCleanWindow reports whether the row carries an admissible arm-covered
// window (the amended unit): present, untruncated, ≥ the turns floor.
func (r ledgerRow) HasCleanWindow() bool {
	return r.ArmWindow != nil && !r.ArmWindow.Truncated &&
		r.ArmWindow.Turns != nil && *r.ArmWindow.Turns >= abMinTurnsPerUnit
}

func readLedgerRows(path string) []ledgerRow {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []ledgerRow
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r ledgerRow
		if json.Unmarshal([]byte(line), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}

// checkLoopSessionLedger — the loop every denominator depends on. If this
// is open, every other figure loses its "of N sessions".
func checkLoopSessionLedger(rows []ledgerRow, now time.Time) flywheelCheck {
	c := flywheelCheck{
		Loop:    "session-ledger",
		Purpose: "one row per finished session — the denominator under every other number",
	}
	if len(rows) == 0 {
		c.Status = loopIdle
		c.Evidence = "no rows yet — no session has ended since the ledger was enabled"
		c.Remedy = "nothing to run; finish a coding session"
		return c
	}
	last := rows[len(rows)-1].At
	age := ledgerRowAge(last, now)
	c.Evidence = fmt.Sprintf("%s rows, most recent %s", fmtThousands(len(rows)), describeAge(age))
	// A ledger that has not moved in 48h while sessions are plainly
	// happening is the exact 2026-08-08 signature: producing side silent,
	// everything downstream reading a stale zero.
	if age > 48*time.Hour {
		c.Status = loopOpen
		c.Evidence += " — nothing recorded in over two days"
		c.Remedy = "fathom doctor (check the Stop hook is installed AND firing); FATHOM_CHANNELS=off silences it entirely"
		return c
	}
	c.Status = loopClosed
	return c
}

// checkLoopArm — the counterfactual. This is the loop that failed twice in
// one day, so it is checked hardest: armed, decrementing, and producing
// rows whose coverage makes them admissible.
func checkLoopArm(rows []ledgerRow, imp obsImpact, now time.Time) flywheelCheck {
	c := flywheelCheck{
		Loop:    "counterfactual-arm",
		Purpose: "OFF-arm sessions — the ONLY instrument that can license a savings claim (law 12)",
	}
	st, active := loadArmState(now)
	if !active {
		if imp.TokensOffSessions > 0 {
			c.Status = loopClosed
			c.Evidence = fmt.Sprintf("no arm running; %s clean OFF session(s) already banked", fmtThousands(imp.TokensOffSessions))
			return c
		}
		c.Status = loopIdle
		c.Evidence = "no arm running and no clean OFF sessions banked — the savings question has no answer at all"
		c.Remedy = "fathom arm off --sessions 20  (after pre-registering criteria)"
		return c
	}

	// Armed. Two distinct ways this silently collects nothing.
	var since, clean, unclean int
	setAt, _ := time.Parse(time.RFC3339, st.SetAt)
	for _, r := range rows {
		at, err := time.Parse(time.RFC3339, r.At)
		if err != nil || !at.After(setAt) {
			continue
		}
		since++
		if r.Arm == "off" && (r.ArmCoverage == "full" || r.HasCleanWindow()) {
			clean++
		} else if r.Arm == "off" {
			unclean++
		}
	}
	armAge := now.Sub(setAt)
	c.Evidence = fmt.Sprintf("armed %s; %s row(s) since, %s admissible, %s excluded for coverage",
		describeAge(armAge), fmtThousands(since), fmtThousands(clean), fmtThousands(unclean))
	if st.SessionsRemaining != nil {
		c.Evidence += fmt.Sprintf("; %s session(s) of budget left", fmtThousands(*st.SessionsRemaining))
	}

	switch {
	case since == 0 && armAge > 6*time.Hour:
		// The 2026-08-08 failure, verbatim: armed, hours elapsed, nothing
		// recorded. The arm spends its budget and buys no data.
		c.Status = loopOpen
		c.Evidence += " — the arm is burning its budget and collecting NOTHING"
		c.Remedy = "fathom doctor; confirm the deployed binary records under an arm (the OFF arm must silence assistance, never measurement)"
	case clean == 0 && unclean > 0:
		// The second failure: rows arrive but none are admissible.
		c.Status = loopOpen
		c.Evidence += " — rows are arriving but NONE are admissible; the comparison still has no data"
		c.Remedy = "keep the session running past " + itoa(abMinTurnsPerUnit) + " turns under the arm — a resumed session's ARM-COVERED WINDOW counts once it clears the floor (§5 amendment); a fresh session also works"
	case clean == 0:
		c.Status = loopIdle
		c.Evidence += " — no admissible session has finished yet"
		c.Remedy = "nothing to run; finish a session that began after the arm was set"
	default:
		c.Status = loopClosed
	}
	return c
}

// flywheelCallLog locates and reads the daemon's call log once, so every
// check that needs it reads the SAME rows.
func flywheelCallLog(baseURL string) []adoptionCallEntry {
	path := resolveCallLogPath()
	if _, err := os.Stat(path); err != nil {
		// A real client: resolveCallLogCandidates asks the daemon where its
		// own log lives, and passing nil made that a nil dereference on any
		// machine lacking a local call log — which is every clean checkout.
		hc := &http.Client{Timeout: 4 * time.Second}
		for _, cand := range resolveCallLogCandidates(hc, baseURL, true, ".") {
			if _, e := os.Stat(cand); e == nil {
				path = cand
				break
			}
		}
	}
	entries, _, read := readCallLog(path)
	if !read {
		return nil
	}
	return entries
}

// checkLoopCallLogEntries — traffic reaching the daemon at all.
func checkLoopCallLogEntries(entries []adoptionCallEntry, now time.Time) flywheelCheck {
	c := flywheelCheck{
		Loop:    "call-log",
		Purpose: "every tool call recorded, so adoption and per-tool cost are measurable",
	}
	if len(entries) == 0 {
		c.Status = loopIdle
		c.Evidence = "no call log readable — the daemon may not have served a call yet, or its log lives elsewhere"
		c.Remedy = "fathom doctor (the hooks_firing check locates the daemon's own log)"
		return c
	}
	var lastTS string
	agentCalls := 0
	for _, e := range entries {
		if e.TS > lastTS {
			lastTS = e.TS
		}
		if strings.Contains(e.Surface, "agent") {
			agentCalls++
		}
	}
	age := ledgerRowAge(lastTS, now)
	c.Evidence = fmt.Sprintf("%s calls, %s via the agent surface, most recent %s",
		fmtThousands(len(entries)), fmtThousands(agentCalls), describeAge(age))
	if age > 24*time.Hour {
		c.Status = loopOpen
		c.Evidence += " — no traffic in over a day"
		c.Remedy = "fathom doctor (daemon reachability and MCP registration)"
		return c
	}
	c.Status = loopClosed
	return c
}

func checkLoopObligations(obligationCalls int) flywheelCheck {
	c := flywheelCheck{
		Loop:    "obligations",
		Purpose: "governing rules injected before an edit — the prevention side of the story",
	}
	if obligationCalls == 0 {
		c.Status = loopIdle
		c.Evidence = "no rule injections recorded — no repository is serving obligations yet"
		c.Remedy = "fathom capabilities  (the obligations row names what is missing here)"
		return c
	}
	c.Status = loopClosed
	c.Evidence = fmt.Sprintf("%s injections recorded", fmtThousands(obligationCalls))
	return c
}

func checkLoopVerifyGate(imp obsImpact) flywheelCheck {
	c := flywheelCheck{
		Loop:    "verify-gate",
		Purpose: "plan claims re-checked against code — the clearest single number for what fathom stopped",
	}
	ran := verifyRanSessions(imp.VerifyCatchesNote)
	switch {
	case imp.VerifyCatchesTotal == nil || ran <= 0:
		c.Status = loopIdle
		c.Evidence = "the gate has never run in a recorded session, so catches are unknown — not zero"
		c.Remedy = "add `fathom verify --service <svc>` to the pre-push hook (lefthook.yml)"
	case imp.LedgerSessions > 0 && ran*4 < imp.LedgerSessions:
		// Not broken, but measuring over a sliver. Named rather than
		// reported as a healthy CLOSED, because a catch count over 1% of
		// sessions is not the number anyone thinks it is.
		c.Status = loopIdle
		c.Evidence = fmt.Sprintf("ran in only %s of %s sessions — measured over a sliver of the work",
			fmtThousands(ran), fmtThousands(imp.LedgerSessions))
		c.Remedy = "wire the gate into pre-push so it runs on every push rather than by hand"
	default:
		c.Status = loopClosed
		c.Evidence = fmt.Sprintf("ran in %s sessions, %s catch(es)", fmtThousands(ran), fmtThousands(*imp.VerifyCatchesTotal))
	}
	return c
}

func checkLoopExploration(imp obsImpact) flywheelCheck {
	c := flywheelCheck{
		Loop:    "exploration-ledger",
		Purpose: "per-question cost of grepping versus asking the index (observational, never causal)",
	}
	if imp.ExpSessions == 0 {
		c.Status = loopIdle
		c.Evidence = "no session carries the exploration ledger yet — rows predate the instrument"
		c.Remedy = "nothing to run; sessions started after the current binary fill it in"
		return c
	}
	c.Status = loopClosed
	c.Evidence = fmt.Sprintf("%s session(s) measured; %s raw vs %s indexed call(s)",
		fmtThousands(imp.ExpSessions), fmtThousands(imp.ExpRawCalls), fmtThousands(imp.ExpFathomCalls))
	return c
}

// checkLoopObservations — the correction loop. Defects being FILED is only
// half of it; a backlog that only grows is a loop open at the far end.
func checkLoopObservations() flywheelCheck {
	c := flywheelCheck{
		Loop:    "observations",
		Purpose: "defects filed against fathom's own tools, and closed again — the correction loop",
	}
	data, err := os.ReadFile("docs/dogfood-observations.md")
	if err != nil {
		c.Status = loopIdle
		c.Evidence = "no observations file readable from here — the fleet-wide log lives in fathom's own tree"
		c.Remedy = "run this from the fathom checkout, or use `fathom observe` to file friction into this repo's .fathom/observations.md"
		return c
	}
	body := string(data)
	open := strings.Count(body, "\n### O")
	c.Status = loopClosed
	c.Evidence = fmt.Sprintf("%s observation(s) on file", fmtThousands(open))
	return c
}

// ledgerRowAge is tolerant: an unparseable timestamp reports a huge age so
// the check errs toward "look at this" rather than silently passing.
func ledgerRowAge(ts string, now time.Time) time.Duration {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return 365 * 24 * time.Hour
	}
	return now.Sub(t)
}

func describeAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
