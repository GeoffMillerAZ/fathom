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
	"os"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/assess"
	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
	"github.com/geoffmilleraz/fathom/internal/metrics"
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
	// J5 (app-kit O25): flywheel had no --service at all, so it could
	// never resolve a NAMED service's own call log the way instrument
	// --adoption/research report do — one of the three disagreements the
	// dogfood journal's P0 names. Optional and empty by default: with no
	// --service, resolution is byte-for-byte what it always was
	// (resolveCallLogPath, then resolveCallLogCandidates rooted at ".").
	service := fs.String("service", "", "scope the call-log loop check to one service's own recorded call log (resolveServiceCallLog, calllogidentity.go) — answers \"is fathom's loop closed for repo X\" from anywhere")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	now := time.Now()
	checks := runFlywheelChecks(*url, *service, now)

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{
			"generated_at": now.UTC().Format(time.RFC3339),
			"loops":        checks,
			"call_log":     resolveConsoleCallLogFor(callLogQuery{Service: *service, BaseURL: *url}),
		}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return flywheelExit(checks)
	}

	closed, open, idle := 0, 0, 0
	fmt.Fprintf(stdout, "fathom flywheel — are the feedback loops closed?\n")
	fmt.Fprintf(stdout, "%s\n\n", callLogLine(resolveConsoleCallLogFor(callLogQuery{Service: *service, BaseURL: *url})))
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

func runFlywheelChecks(baseURL, service string, now time.Time) []flywheelCheck {
	// Obligation traffic lives in the CALL LOG, not the impact model —
	// buildObservatoryModel folds it separately. Reading the wrong source
	// here produced a false IDLE on a loop carrying 6,992 injections, which
	// is precisely the class of error this verb exists to catch. Every
	// check must read the same source the surface it mirrors reads.
	entries, callLogErr := flywheelCallLog(baseURL, service)
	return flywheelChecksFrom(buildImpactModel(now), entries, callLogErr, now, cliLedgerRoots(baseURL))
}

// flywheelChecksFrom is runFlywheelChecks over an impact model and call-log
// entries already read — the console's Operate page passes the observatory
// model's impact fold and its own read of the log instead of re-reading
// both (the impact fold alone reads the whole hooks log).
// roots are the registered service roots the verify-gate loop reads the
// ledger under (PE-24).
func flywheelChecksFrom(imp obsImpact, entries []adoptionCallEntry, callLogErr error, now time.Time, roots ledgerroots.Roots) []flywheelCheck {
	return flywheelChecksFold(imp, foldCallLog(entries), callLogErr, now, roots)
}

// flywheelChecksFold is flywheelChecksFrom over the call log's fold.
func flywheelChecksFold(imp obsImpact, fold *callLogFold, callLogErr error, now time.Time, roots ledgerroots.Roots) []flywheelCheck {
	ledger := readLedgerRows(sessionLedgerPath())
	obligationCalls := fold.obligations
	return []flywheelCheck{
		checkLoopSessionLedger(ledger, now),
		checkLoopArm(ledger, imp, now),
		checkLoopCallLog(fold, callLogErr, now),
		checkLoopObligations(obligationCalls),
		checkLoopVerifyGate(imp, roots),
		checkLoopExploration(imp),
		checkLoopObservations(),
		checkLoopContinuity(),
		checkLoopCustodian(now),
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
	// SpendLabel marks a row written inside a `fathom assess` judge
	// (IR-4): never admissible to the arm loop.
	SpendLabel string `json:"spend_label,omitempty"`
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
	var since, clean, unclean, isolated int
	setAt, _ := time.Parse(time.RFC3339, st.SetAt)
	for _, r := range rows {
		at, err := time.Parse(time.RFC3339, r.At)
		if err != nil || !at.After(setAt) {
			continue
		}
		since++
		if assess.IsolatedSpendLabel(r.SpendLabel) {
			isolated++ // IR-4: judging spend, never an admissible arm row
			continue
		}
		if r.Arm == "off" && (r.ArmCoverage == "full" || r.HasCleanWindow()) {
			clean++
		} else if r.Arm == "off" {
			unclean++
		}
	}
	armAge := now.Sub(setAt)
	c.Evidence = fmt.Sprintf("armed %s; %s row(s) since, %s admissible, %s excluded for coverage",
		describeAge(armAge), fmtThousands(since), fmtThousands(clean), fmtThousands(unclean))
	if isolated > 0 {
		c.Evidence += fmt.Sprintf(", %s excluded as judging spend (IR-4)", fmtThousands(isolated))
	}
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
// check that needs it reads the SAME rows. The returned error is nil on a
// clean read; readCallLog itself only reports a bool (O24: it swallows the
// real os error), so a failed read re-opens the path directly to recover
// it — the caller (checkLoopCallLogEntries) must be able to tell "no log
// yet" (ENOENT — honestly IDLE) from "the log is there but unreadable"
// (any other error — a real defect, OPEN), which the collapsed bool could
// not distinguish.
func flywheelCallLog(baseURL, service string) ([]adoptionCallEntry, error) {
	// OC-2 (console_calllog.go): the one call-log resolution every verb and
	// page shares — the daemon's record, then J5's named-service identity,
	// then config and the legacy default with the daemon-workspace probes.
	path := resolveConsoleCallLogFor(callLogQuery{Service: service, BaseURL: baseURL}).Path
	entries, _, read := readCallLog(path)
	if read {
		return entries, nil
	}
	_, openErr := os.Open(path)
	if openErr == nil {
		// readCallLog just failed and a fresh Open didn't (a race, or a
		// permission state that changed between the two calls) — report
		// generically rather than claim success with zero entries.
		return nil, fmt.Errorf("call log at %s: unreadable", path)
	}
	if os.IsNotExist(openErr) {
		return nil, openErr // honest "no log yet" — the caller renders IDLE
	}
	// Any other error (EPERM is the common one): the log is right there and
	// unreadable, a real defect — the hint travels WITH the error so the
	// caller never needs to re-derive which path actually failed.
	return nil, fmt.Errorf("%w%s", openErr, tccWedgeHint(path))
}

// tccWedgeHint appends the exact remedy for the failure signature CLAUDE.md's
// own external-volume playbook names: stat succeeds (the file is right
// there) but read fails — a wedged macOS TCC verdict, not a real
// permissions problem, fixed by re-running one command. "" for any path not
// under /Volumes/, where this specific cause cannot apply.
func tccWedgeHint(path string) string {
	if strings.HasPrefix(path, "/Volumes/") {
		return " — stat succeeds but read fails on an external volume = macOS TCC wedge; run: tccutil reset SystemPolicyRemovableVolumes com.mitchellh.ghostty"
	}
	return ""
}

// checkLoopCallLogEntries — traffic reaching the daemon at all.
// checkLoopCallLogEntries — traffic reaching the daemon at all. callLogErr
// is flywheelCallLog's read error (nil on a clean read): an absent file
// (ENOENT) is the honest "no log yet" case, IDLE like always, but any OTHER
// error — permission denied is the common one, exactly the external-volume
// TCC-wedge signature CLAUDE.md's own playbook names — is a real defect
// (the log is right there and unreadable) and must report OPEN, never the
// silent-nothing-to-see IDLE a missing file gets (O24: the previous
// collapsed bool could not tell the two apart, so an EPERM read rendered
// exactly like a clean checkout that had simply never run a call).
func checkLoopCallLogEntries(entries []adoptionCallEntry, callLogErr error, now time.Time) flywheelCheck {
	return checkLoopCallLog(foldCallLog(entries), callLogErr, now)
}

// checkLoopCallLog is checkLoopCallLogEntries over the call log's fold.
func checkLoopCallLog(fold *callLogFold, callLogErr error, now time.Time) flywheelCheck {
	c := flywheelCheck{
		Loop:    "call-log",
		Purpose: "every tool call recorded, so adoption and per-tool cost are measurable",
	}
	if callLogErr != nil && !os.IsNotExist(callLogErr) {
		c.Status = loopOpen
		c.Evidence = fmt.Sprintf("call log unreadable: %v", callLogErr)
		c.Remedy = "fathom doctor (the hooks_firing check locates the daemon's own log)"
		return c
	}
	if fold.rows == 0 {
		c.Status = loopIdle
		c.Evidence = "no call log readable — the daemon may not have served a call yet, or its log lives elsewhere"
		c.Remedy = "fathom doctor (the hooks_firing check locates the daemon's own log)"
		return c
	}
	lastTS, agentCalls := fold.lastTS, fold.agentSurface
	age := ledgerRowAge(lastTS, now)
	c.Evidence = fmt.Sprintf("%s calls, %s via the agent surface, most recent %s",
		fmtThousands(fold.rows), fmtThousands(agentCalls), describeAge(age))
	if age > 24*time.Hour {
		// IT-2(a): silence over the stale window can be the OFF arm doing
		// exactly what it is for — hook channels quiet BY DESIGN — not a
		// broken pipe. Check before flipping OPEN, or this check itself
		// becomes a false-defect report against a working experiment.
		if st, active := metrics.ArmStateForWindow("", now.Add(-24*time.Hour), now); active {
			c.Status = loopIdle
			c.Evidence += "; " + st.Note()
			return c
		}
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

func checkLoopVerifyGate(imp obsImpact, roots ledgerroots.Roots) (c flywheelCheck) {
	c = flywheelCheck{
		Loop:    "verify-gate",
		Purpose: "plan claims re-checked against code — the clearest single number for what fathom stopped",
	}
	ran := verifyRanSessions(imp.VerifyCatchesNote)
	// O30: git-native pre-push runs never enter the session-scoped
	// instrumentation this loop originally read, so a repo gating on
	// every push still read "never ran". The verify ledger (SD-1) is the
	// recorded-run signal that sees them.
	// PE-24: only the registered roots' runs, the ignored ones named.
	ledgered, ignored := readRegisteredVerifyLedger(roots)
	defer func() { c.Evidence = withIgnored(c.Evidence, ignored, verifyLedgerNoun) }()
	switch {
	case (imp.VerifyCatchesTotal == nil || ran <= 0) && len(ledgered) > 0:
		last := ledgered[len(ledgered)-1]
		c.Status = loopClosed
		c.Evidence = fmt.Sprintf("%s ledger'd gate run(s), most recent %s (git-native pre-push; per-session catch attribution still needs a session-scoped run)",
			fmtThousands(len(ledgered)), last.At)
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

// checkLoopCustodian — CC-8 (docs/plans/custodian.md): the config
// custodian's conveyor. IDLE = no concerns registered (normal, named).
// OPEN = violations exist AND no audit has looked in 7 days — violations
// accumulating unread are a conveyor moving nothing. CLOSED = audits are
// running and either the config is clean or its violations are being
// looked at (the looking is what this loop can observe; the fixing shows
// up as the violation count going to zero).
func checkLoopCustodian(now time.Time) flywheelCheck {
	c := flywheelCheck{
		Loop:    "custodian",
		Purpose: "config contracts audited and violations consumed — instructed means true",
	}
	root, _ := os.Getwd()
	concerns, loadErrs := loadConcernsBoth(root)
	if len(loadErrs) > 0 {
		c.Status = loopOpen
		c.Evidence = "a concerns.cue failed to load — the contracts themselves are unreadable"
		c.Remedy = "fathom config audit  (names every problem)"
		return c
	}
	if len(concerns) == 0 {
		c.Status = loopIdle
		c.Evidence = "no concerns registered — nothing for the custodian to hold this repo to"
		c.Remedy = "write .fathom/concerns.cue (fathom init scaffolds a commented template)"
		return c
	}
	violated := 0
	for _, f := range custodian.Audit(root, concerns, custodian.WithWiringDrift(wiringCurrentEvaluator)) {
		if f.Status == custodian.StatusViolated {
			violated++
		}
	}
	// The latest audit from the hooks-log index (RT-114), never a whole
	// read of the log.
	lastAudit := hooksBeatIndex.lastCustodianAudit(beatLogPath(), root)
	switch {
	case violated > 0 && (lastAudit.IsZero() || now.Sub(lastAudit) > 7*24*time.Hour):
		c.Status = loopOpen
		c.Evidence = fmt.Sprintf("%d violation(s) standing and no audit has looked in over a week — accumulating unread", violated)
		c.Remedy = "fathom config audit — then fix the config (fathom reports; you decide)"
	case violated > 0:
		c.Status = loopClosed
		c.Evidence = fmt.Sprintf("%d violation(s) standing, last looked at %s — being consumed, not ignored", violated, lastAudit.UTC().Format(time.RFC3339))
	default:
		c.Status = loopClosed
		c.Evidence = fmt.Sprintf("%d concern(s) audited clean", len(concerns))
	}
	return c
}

// flywheelRunner runs `fathom flywheel` (registered below; EW-45).
type flywheelRunner struct{}

func (flywheelRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdFlywheel(args, stdout, stderr)
}

func init() {
	// SR-31: fathom metrics flywheel covers this; stays fully dispatched
	declareVerb(verb{Name: "flywheel", Group: "operate", Order: 150, Hidden: true, Run: flywheelRunner{},
		Summary: "Are the feedback loops closed? Each loop CLOSED/OPEN/IDLE with its evidence and remedy"})
}
