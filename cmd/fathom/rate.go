package main

// rate.go — `fathom rate`: the OUTCOME half of the pre-registered A/B
// criterion (docs/design/production-tokenomics.md §5).
//
// The criterion registered before any run reads:
//
//   ON must show lower median tokens-per-session AT EQUAL-OR-BETTER
//   self-rated outcome across ≥20 sessions per arm.
//
// Tokens and turns were instrumented. The self-rating was not — there was
// no field for it anywhere. That gap is not cosmetic: without the outcome
// half, a token drop is indistinguishable from "the unaided session gave
// up sooner", and reporting that as a saving would be the single most
// misleading thing this program could do.
//
// A Stop hook cannot prompt a human, so the rating cannot be collected
// where the row is written. Instead ratings are appended to their own
// user-central log and JOINED to the ledger by session id at read time.
// Join-at-read rather than dual-write, so a rating given after a session
// ended still counts and the append-only ledger is never rewritten.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ratingSchemaVersion tracks this row's shape independently of the ledger's.
const ratingSchemaVersion = 1

// sessionRating is one self-rated outcome.
type sessionRating struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	Rating        int    `json:"rating"` // 1–5, as registered
	Note          string `json:"note,omitempty"`
	Repo          string `json:"repo,omitempty"`
	// Arm is stamped at rating time so a rating can be read without
	// re-joining the ledger, and so a rating given long after the fact is
	// still attributable to the arm the session RAN under rather than the
	// arm active when it was rated.
	Arm string `json:"arm"`
	At  string `json:"at"`
}

func ratingsPath() string {
	if p := os.Getenv("FATHOM_SESSION_RATINGS"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "session-ratings.jsonl")
}

// currentSessionID resolves the harness's session id. Claude Code exports
// it, which is what lets `fathom rate 4` work with no arguments — asking a
// human to paste a UUID is asking them not to bother.
func currentSessionID() string {
	for _, k := range []string{"CLAUDE_CODE_SESSION_ID", "CLAUDE_SESSION_ID"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func cmdRate(args []string, stdout, stderr io.Writer) int {
	// `fathom rate 4 --note "..."` is the form a human will actually type,
	// and Go's flag package stops parsing at the first positional — so the
	// rating is lifted off the front before the flags are parsed. Requiring
	// `--note "..." 4` instead would be technically simpler and would get
	// this verb skipped.
	var lead string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		lead, args = args[0], args[1:]
	}

	fs := newFlagSet("rate", stderr)
	note := fs.String("note", "", "one line on what worked or did not (optional, recommended)")
	session := fs.String("session", "", "session id (default: this session, from the harness)")
	repo := fs.String("repo", ".", "repository this session worked in")
	showList := fs.Bool("list", false, "list recorded ratings instead of adding one")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *showList {
		return listRatings(stdout, stderr)
	}

	if lead == "" && fs.NArg() == 1 {
		lead = fs.Arg(0)
	}
	if lead == "" {
		fmt.Fprintln(stderr, "usage: fathom rate <1-5> [--note \"...\"]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "  Rate whether you got where you wanted this session. This is the OUTCOME")
		fmt.Fprintln(stderr, "  half of the A/B criterion: without it, fewer tokens cannot be told apart")
		fmt.Fprintln(stderr, "  from giving up sooner.")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "    5  got exactly where I wanted")
		fmt.Fprintln(stderr, "    4  got there, with friction")
		fmt.Fprintln(stderr, "    3  partly")
		fmt.Fprintln(stderr, "    2  mostly not")
		fmt.Fprintln(stderr, "    1  did not get there")
		return 1
	}
	n, err := strconv.Atoi(lead)
	if err != nil || n < 1 || n > 5 {
		fmt.Fprintf(stderr, "error: the rating must be a whole number 1–5, got %q\n", lead)
		return 1
	}

	sid := *session
	if sid == "" {
		sid = currentSessionID()
	}
	if sid == "" {
		// Without a session id the rating cannot be joined to anything, so
		// it would be a number in a file pretending to be evidence.
		fmt.Fprintln(stderr, "error: no session id — the harness did not export one and --session was not given.")
		fmt.Fprintln(stderr, "       A rating that cannot be joined to a session is not data; refusing to record it.")
		return 1
	}

	root, _ := filepath.Abs(*repo)
	now := time.Now()
	r := sessionRating{
		SchemaVersion: ratingSchemaVersion,
		SessionID:     sid,
		Rating:        n,
		Note:          strings.TrimSpace(*note),
		Repo:          strings.ToLower(filepath.Base(root)),
		Arm:           currentArmLabel(now),
		At:            now.UTC().Format(time.RFC3339),
	}
	if err := appendRating(r); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "recorded: %d/5 for this session (arm %s, repo %s)\n", n, r.Arm, r.Repo)
	if r.Note == "" {
		fmt.Fprintln(stdout, "tip: --note \"…\" is what makes a rating readable in three weeks")
	}
	return 0
}

func appendRating(r sessionRating) error {
	p := ratingsPath()
	if p == "" {
		return fmt.Errorf("cannot resolve the ratings path (no home directory)")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// loadRatings returns the LAST rating per session — re-rating a session
// corrects it rather than adding a second opinion, and the log stays
// append-only so the correction is visible in history.
func loadRatings(path string) map[string]sessionRating {
	out := map[string]sessionRating{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r sessionRating
		if json.Unmarshal([]byte(line), &r) != nil || r.SessionID == "" {
			continue // a malformed line is skipped, never guessed at
		}
		if r.Rating < 1 || r.Rating > 5 {
			continue
		}
		out[r.SessionID] = r
	}
	return out
}

func listRatings(stdout, stderr io.Writer) int {
	rs := loadRatings(ratingsPath())
	if len(rs) == 0 {
		fmt.Fprintln(stdout, "no ratings recorded yet — the A/B criterion cannot be evaluated without them")
		fmt.Fprintln(stdout, "run `fathom rate <1-5> --note \"...\"` at the end of a session")
		return 0
	}
	keys := make([]string, 0, len(rs))
	for k := range rs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	byArm := map[string][]int{}
	fmt.Fprintf(stdout, "%d rated session(s)\n\n", len(rs))
	for _, k := range keys {
		r := rs[k]
		byArm[r.Arm] = append(byArm[r.Arm], r.Rating)
		fmt.Fprintf(stdout, "  %d/5  arm %-3s  %-24s %s  %s\n", r.Rating, r.Arm, r.Repo, r.At[:10], r.Note)
	}
	fmt.Fprintln(stdout)
	for _, arm := range []string{"on", "off"} {
		v := byArm[arm]
		if len(v) == 0 {
			fmt.Fprintf(stdout, "  arm %-3s: no rated sessions yet\n", arm)
			continue
		}
		if m, ok := medianInt(v); ok {
			fmt.Fprintf(stdout, "  arm %-3s: %d rated, median %d/5\n", arm, len(v), m)
		}
	}
	return 0
}

// abMinSessionsPerArm is the pre-registered floor
// (docs/design/production-tokenomics.md §5): "across ≥20 sessions per
// arm". It is a constant here so that changing it is a visible, reviewable
// act rather than a quiet drift in a comparison.
const abMinSessionsPerArm = 20

// evaluateABCriterion states in words whether the registered criterion can
// be evaluated YET, and if not, exactly what is missing.
//
// The criterion is conjunctive — lower median tokens AND equal-or-better
// median rating, across ≥20 sessions per arm — so every unmet part is
// named. A criterion that quietly reports "not yet" without saying which
// part is missing is how a measurement program stalls without anyone
// noticing.
func evaluateABCriterion(imp obsImpact) string {
	var missing []string
	if imp.TokensOffSessions < abMinSessionsPerArm {
		missing = append(missing, fmt.Sprintf("the OFF arm has %d of the %d sessions registered as the minimum",
			imp.TokensOffSessions, abMinSessionsPerArm))
	}
	if imp.TokensOnSessions < abMinSessionsPerArm {
		missing = append(missing, fmt.Sprintf("the ON arm has %d of %d", imp.TokensOnSessions, abMinSessionsPerArm))
	}
	if imp.TokensOnMedian == nil || imp.TokensOffMedian == nil {
		missing = append(missing, "at least one arm has no usable token median")
	}
	if imp.RatingOnMed == nil || imp.RatingOffMed == nil {
		missing = append(missing, fmt.Sprintf("self-rated outcomes are missing (%d ON, %d OFF rated) — "+
			"without them, fewer tokens cannot be told apart from getting less done; run `fathom rate <1-5>` at session end",
			imp.RatedOn, imp.RatedOff))
	}
	if len(missing) > 0 {
		return "NOT YET EVALUABLE — " + strings.Join(missing, "; ")
	}

	cheaper := *imp.TokensOnMedian < *imp.TokensOffMedian
	notWorse := *imp.RatingOnMed >= *imp.RatingOffMed
	switch {
	case cheaper && notWorse:
		return fmt.Sprintf("MET: fathom-on is cheaper (%d vs %d median tokens) at equal-or-better outcome (%d vs %d of 5)",
			*imp.TokensOnMedian, *imp.TokensOffMedian, *imp.RatingOnMed, *imp.RatingOffMed)
	case !cheaper:
		return fmt.Sprintf("NOT MET: fathom-on is not cheaper (%d vs %d median tokens) — per the registered kill, "+
			"the production tokenomics claim is retracted wherever it appears",
			*imp.TokensOnMedian, *imp.TokensOffMedian)
	default:
		return fmt.Sprintf("NOT MET: fathom-on is cheaper (%d vs %d) but outcomes are WORSE (%d vs %d of 5) — "+
			"the criterion requires equal-or-better, and cheaper-but-worse is not a saving",
			*imp.TokensOnMedian, *imp.TokensOffMedian, *imp.RatingOnMed, *imp.RatingOffMed)
	}
}
