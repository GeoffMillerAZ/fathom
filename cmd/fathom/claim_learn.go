package main

// claim_learn.go — `fathom claim learn` (GL-13, docs/plans/governance-learning.md;
// design docs/design/governance-learning.md §6 "Rules that learn"): the
// product caller of semantic.ApplyLearning. It reads the GL-10 rule-outcome
// log the hooks and verdicts write, the service's `.fathom/learning.cue`
// thresholds over the binary's defaults (GL-12, LoadLearningThresholds),
// and the service's governing claims, and prints each promotion
// (candidate → active) and retirement (active → retired) with the signals
// that moved it. The default is a dry run; `--apply` persists the lifecycle
// and closes a retired claim's validity window through the one
// claim-ending path (semantic.CloseClaimValidity, GL-15), then announces
// every decision with its signals (learningAnnouncements, GL-14) in the
// intent ledger the attention line and `fathom intent audit` read. Nothing
// is ever deleted.
//
// The owner runs it by hand: no daemon pass, no background job (TABLE
// FREEZE). A retirement for never being triggered is held, and named,
// while the signal log does not reach back over the whole window it reads:
// a log that started last week cannot say a rule went unserved for 180
// days.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// learningNow is `claim learn`'s clock (the determinism contract); a var
// so a test pins it.
var learningNow = time.Now

// learnHeld is a decision applyLearning made that the verb does not act on,
// with why.
type learnHeld struct {
	ClaimID string `json:"claim_id"`
	To      string `json:"to"`
	Reason  string `json:"reason"`
}

// learnDecision is one decision as the verb reports it: the transition and
// the signals applyLearning measured it against.
type learnDecision struct {
	ClaimID          string `json:"claim_id"`
	From             string `json:"from"`
	To               string `json:"to"`
	Reason           string `json:"reason"`
	Corroborations   int    `json:"corroborations"`
	DistinctSessions int    `json:"distinct_sessions"`
	Refutes          int    `json:"refutes"`
	Served           int    `json:"served"`
}

// learnServiceResult is one service's learning pass. On an --apply every
// decision in Decisions is also in Announced (GL-14): none is silent.
type learnServiceResult struct {
	Service        string                      `json:"service"`
	Root           string                      `json:"root,omitempty"`
	ThresholdsFrom string                      `json:"thresholds_from"`
	Thresholds     semantic.LearningThresholds `json:"thresholds"`
	Governing      int                         `json:"governing_claims"`
	Decisions      []learnDecision             `json:"decisions"`
	Held           []learnHeld                 `json:"held,omitempty"`
	Applied        bool                        `json:"applied"`
	// Announced are the lines learningAnnouncements (GL-14) recorded in
	// the intent ledger, 1:1 with Decisions on an --apply.
	Announced []string `json:"announced,omitempty"`
}

// learnReport is the verb's whole answer.
type learnReport struct {
	Store     string               `json:"store"`
	Log       string               `json:"signal_log"`
	LogRows   int                  `json:"signal_rows"`
	LogSince  string               `json:"signal_since,omitempty"`
	AsOf      string               `json:"as_of"`
	Apply     bool                 `json:"apply"`
	Services  []learnServiceResult `json:"services"`
	LogAbsent bool                 `json:"signal_absent,omitempty"`
}

// cmdClaimLearn implements `fathom claim learn [--service S] [--store DB] [--apply] [--json]`.
func cmdClaimLearn(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("claim learn", stderr)
	storeFlag := fs.String("store", "", "store to learn over (default: the store the daemon records, then [daemon].store)")
	service := fs.String("service", "", "only this project's claims (default: every project with claims)")
	apply := fs.Bool("apply", false, "persist each decision (lifecycle; a retirement closes the validity window) and announce it; the default is a dry run")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(reorderFlagsFirst(fs, args)); err != nil {
		return 1
	}
	ks := resolveKnowledgeStore(*storeFlag, "")
	if _, err := os.Stat(ks.Path); err != nil {
		fmt.Fprintf(stderr, "error: no store at %s — %v; pass --store\n", ks, err)
		return 1
	}
	var st store.Store
	var err error
	if *apply {
		st, err = store.Open(ks.Path)
	} else {
		st, err = openKnowledgeStore(ks)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %s: %v\n", ks, err)
		return 1
	}
	defer st.Close()

	rep, err := runClaimLearn(context.Background(), st, ks.String(), *service, *apply, learningNow().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return jsonExit(enc.Encode(rep), stderr)
	}
	printClaimLearn(stdout, rep)
	return 0
}

// runClaimLearn is the verb's body over an open store.
func runClaimLearn(ctx context.Context, st store.Store, storeName, only string, apply bool, now time.Time) (learnReport, error) {
	logPath := ruleOutcomeLogPath()
	rep := learnReport{Store: storeName, Log: logPath, AsOf: now.Format(time.RFC3339), Apply: apply, Services: []learnServiceResult{}}
	log := readRuleOutcomeLogFrom(logPath, time.Time{})
	if log.Read.Err != nil {
		return rep, fmt.Errorf("the rule-outcome log %s cannot be read (%v); nothing is decided from a partial signal", logPath, log.Read.Err)
	}
	rep.LogRows, rep.LogAbsent = len(log.Events), !log.Read.Present
	if !log.Read.Since.IsZero() {
		rep.LogSince = log.Read.Since.UTC().Format(time.RFC3339)
	}

	services := []string{only}
	if only == "" {
		ids, err := st.ClaimServiceIDs(ctx)
		if err != nil {
			return rep, fmt.Errorf("list services with claims: %w", err)
		}
		services = ids
	}
	for _, svc := range services {
		res, err := learnService(ctx, st, svc, log, apply, now)
		if err != nil {
			return rep, err
		}
		rep.Services = append(rep.Services, res)
	}
	return rep, nil
}

// learnService runs one service's pass: thresholds, governing claims,
// ApplyLearning, the coverage hold, and on apply the writes.
func learnService(ctx context.Context, st store.Store, svc string, log ruleOutcomeLog, apply bool, now time.Time) (learnServiceResult, error) {
	res := learnServiceResult{Service: svc, Decisions: []learnDecision{}}
	if meta, found, err := st.GetServiceMeta(ctx, svc); err == nil && found {
		res.Root = meta.Path
	}
	res.Thresholds, res.ThresholdsFrom = learningThresholdsFor(res.Root)

	claims, err := st.ClaimsForService(ctx, svc, "")
	if err != nil {
		return res, fmt.Errorf("%s: load claims: %w", svc, err)
	}
	// The governing rules: current claims in a governing layer, the same
	// universe the rule-effectiveness view classifies (console/ruleeffect.go).
	// An ended claim (refuted, superseded) already closed its window.
	roles := semantic.LayerRolesIn(res.Root)
	var governing []core.SemanticClaim
	for _, c := range claims {
		if c.Status.Served() && roles[c.Layer] != semantic.RoleClaims {
			governing = append(governing, c)
		}
	}
	res.Governing = len(governing)
	updated, decisions := semantic.ApplyLearning(governing, log.Events, res.Thresholds, now)

	// Never-triggered retirement reads "unserved for N days"; a log that
	// does not reach back N days has no evidence for the days before it.
	windowStart := now.Add(-time.Duration(res.Thresholds.RetireNeverTriggeredDays) * 24 * time.Hour)
	covered := !log.Read.Since.IsZero() && !log.Read.Since.After(windowStart)
	byID := make(map[string]int, len(updated))
	for i, c := range updated {
		byID[c.ClaimID] = i
	}
	var kept []semantic.LearningDecision
	for _, d := range decisions {
		if d.To == core.LifecycleRetired && d.Refutes == 0 && !covered {
			reach := "the rule-outcome log has no rows"
			if !log.Read.Since.IsZero() {
				reach = fmt.Sprintf("the rule-outcome log reaches back %d of the %d days it reads", int(now.Sub(log.Read.Since).Hours()/24), res.Thresholds.RetireNeverTriggeredDays)
			}
			res.Held = append(res.Held, learnHeld{ClaimID: d.ClaimID, To: string(d.To), Reason: "held: " + d.Reason + " is not yet evidenced — " + reach})
			continue
		}
		kept = append(kept, d)
	}
	for _, d := range kept {
		res.Decisions = append(res.Decisions, learnDecision{
			ClaimID: d.ClaimID, From: string(d.From), To: string(d.To), Reason: d.Reason,
			Corroborations: d.Corroborations, DistinctSessions: d.DistinctSessions, Refutes: d.Refutes, Served: d.Served,
		})
	}
	if !apply || len(kept) == 0 {
		return res, nil
	}
	for _, d := range kept {
		c := updated[byID[d.ClaimID]]
		if err := st.PutClaims(ctx, []core.SemanticClaim{c}, nil); err != nil {
			return res, fmt.Errorf("%s: persist %s %s → %s: %w", svc, d.ClaimID, d.From, d.To, err)
		}
		if d.To == core.LifecycleRetired {
			// Retirement ends the claim on the lifecycle axis; its status
			// (freshness) is unchanged, and its validity window closes at
			// now through the one claim-ending path.
			if err := semantic.CloseClaimValidity(ctx, st, svc, d.ClaimID, c.Status, now); err != nil {
				return res, fmt.Errorf("%s: close %s: %w", svc, d.ClaimID, err)
			}
		}
	}
	res.Applied = true
	// GL-14 (GL-X1: no silent learning): every persisted decision is
	// announced, with its signals, in the intent ledger the session
	// attention line and `fathom intent audit` read.
	res.Announced = learningAnnouncements(res.Root, kept, now)
	return res, nil
}

// learningThresholdsFor loads <root>/.fathom/learning.cue over the
// binary's defaults (GL-12) and names where the numbers came from. A
// malformed file is named and the defaults stand — never a silent guess.
func learningThresholdsFor(root string) (semantic.LearningThresholds, string) {
	if root == "" {
		return semantic.DefaultLearningThresholds(), "the binary's defaults (no local root recorded for the service)"
	}
	path := filepath.Join(root, ".fathom", "learning.cue")
	if _, err := os.Stat(path); err != nil {
		return semantic.DefaultLearningThresholds(), "the binary's defaults (no " + path + ")"
	}
	t, err := semantic.LoadLearningThresholds(path)
	if err != nil {
		return semantic.DefaultLearningThresholds(), "the binary's defaults (" + path + " is not read: " + err.Error() + ")"
	}
	return t, path
}

// printClaimLearn is the text answer.
func printClaimLearn(w io.Writer, rep learnReport) {
	fmt.Fprintf(w, "store: %s\n", rep.Store)
	switch {
	case rep.LogAbsent:
		fmt.Fprintf(w, "signal: %s — absent: no hook or verdict has recorded a rule outcome on this machine\n", rep.Log)
	default:
		fmt.Fprintf(w, "signal: %s — %d row(s) since %s\n", rep.Log, rep.LogRows, rep.LogSince)
	}
	total := 0
	for _, s := range rep.Services {
		t := s.Thresholds
		fmt.Fprintf(w, "\n%s — %d governing claim(s); thresholds from %s\n", s.Service, s.Governing, s.ThresholdsFrom)
		fmt.Fprintf(w, "  promote: %d corroborations from %d sessions, 0 refutes, over %dd · retire: %d refutes > corroborations, or unserved %dd\n",
			t.PromoteCorroborations, t.PromoteMinSessions, t.PromoteWindowDays, t.RetireRefutes, t.RetireNeverTriggeredDays)
		if len(s.Decisions) == 0 && len(s.Held) == 0 {
			fmt.Fprintln(w, "  no promotion or retirement")
		}
		for _, d := range s.Decisions {
			verb := "would"
			if s.Applied {
				verb = "applied"
			}
			fmt.Fprintf(w, "  %s: %s %s → %s (%s)\n", verb, d.ClaimID, d.From, d.To, d.Reason)
		}
		for _, h := range s.Held {
			fmt.Fprintf(w, "  %s → %s %s\n", h.ClaimID, h.To, h.Reason)
		}
		for _, a := range s.Announced {
			fmt.Fprintf(w, "  announced: %s\n", a)
		}
		total += len(s.Decisions)
	}
	if !rep.Apply && total > 0 {
		fmt.Fprintf(w, "\ndry run: nothing written; --apply persists %d decision(s) (a retirement closes the claim's validity window; nothing is deleted)\n", total)
	}
}
