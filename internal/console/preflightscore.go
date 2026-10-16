package console

// preflightscore.go — WI-18, WI-19 (docs/design/work-intelligence.md §3
// U6, the Jev site): the decision port's Score over a profile's features,
// and the outcome ledger that calibrates the preflight. The Score is asked
// only with [decide] configured and the `preflight` site allowed (I42,
// I14) and only when a caller asks; it is one question over the profile's
// measured numbers — never the row's text, its symbol or any path — and it
// is advisory like the table's tier. An outcome (the model used, its
// effort, proven or not) is one appended ledger line keyed by agent
// identity only (I58); the calibration is read back from those lines.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// PreflightQuestionID keys the preflight Score in a decision request.
const PreflightQuestionID = "preflight.difficulty"

// preflightLevels describe each tier to the decider, low to high; the
// answer addresses a position in this list.
var preflightLevels = []string{
	"light: the smallest, fastest model at the lowest effort setting will prove this row",
	"standard: a mid-size model at the default effort setting is needed to prove this row",
	"heavy: the largest model at the highest effort setting is needed to prove this row",
}

// ScoreReading is the decision port's Score turned into a tier.
type ScoreReading struct {
	Tier         Tier    `json:"tier"`
	Level        float64 `json:"level"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model,omitempty"`
	LatencyMS    int64   `json:"latency_ms"`
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
}

// preflightState renders the profile's features as the text the decider is
// asked about. Numbers only: nothing that names the row or the code.
func preflightState(p Profile) string {
	var b strings.Builder
	b.WriteString("Measured features of a coding task, each a count or ratio taken from stored facts (\"unknown\" means not measured):\n")
	for _, f := range p.Features {
		if f.Value.Known() {
			fmt.Fprintf(&b, "- %s: %s\n", f.Name, trimFloat(f.Value.V))
		} else {
			fmt.Fprintf(&b, "- %s: unknown\n", f.Name)
		}
	}
	return b.String()
}

func trimFloat(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

// preflightScore asks the decision port how demanding the profile's task
// is (WI-18). No call is made — and the Score is absent, saying why — when
// no [decide] provider is configured, or when [decide] sites omits
// `preflight` (I42, I14). The ruling is recorded in the decisions log as
// never applied: a Score changes nothing.
func preflightScore(ctx context.Context, ports *decide.Ports, p Profile) Value[ScoreReading] {
	switch {
	case ports == nil || ports.Decider == nil || ports.Decider.Name() == "none" || ports.Cfg.Provider == "" || ports.Cfg.Provider == "none":
		return Absent[ScoreReading]("[decide] has no provider (provider = none): no call was made (I42)")
	case !decide.SiteAllowed(ports.Cfg, decide.SitePreflight):
		return Absent[ScoreReading](fmt.Sprintf("[decide] sites does not include %q: no call was made (I14)", decide.SitePreflight))
	}
	req := decision.Request{
		State: preflightState(p),
		Questions: []decision.Question{{
			ID: PreflightQuestionID, Kind: decision.KindScore,
			Instructions: "How demanding is this coding task for an AI coding agent, judged only from the measured features in the state? Answer with the level whose description fits best.",
			Levels:       preflightLevels,
			Rubric:       "Wide blast radius, strict governing rules, many certificates to re-prove, an expensive zone, earlier failed attempts and costly similar tasks all raise the level; a feature marked unknown says nothing either way.",
		}},
	}
	started := time.Now()
	resp, err := ports.Decider.Decide(ctx, req)
	if err != nil {
		ports.Recorder.RecordError(decide.SitePreflight, ports.Decider.Name(), resp.Model, len(req.Questions), time.Since(started), err)
		return Errored[ScoreReading]("the decision port failed: " + err.Error()).WithSource("the decision port (" + ports.Decider.Name() + ")")
	}
	ports.Recorder.Record(decide.SitePreflight, resp, func(decision.Answer) bool { return false })
	src := "the decision port (" + ports.Decider.Name() + ")"
	if why, bad := resp.Invalid[PreflightQuestionID]; bad {
		return Errored[ScoreReading]("the decision port's answer was invalid: " + why).WithSource(src)
	}
	ans, ok := resp.Answers[PreflightQuestionID]
	if !ok {
		return Errored[ScoreReading]("the decision port answered nothing for the preflight question").WithSource(src)
	}
	if decision.BandOf(ans.Confidence) == decision.BandUnsure {
		return Absent[ScoreReading](fmt.Sprintf("the port answered in the unsure band (confidence %.2f): shown as no answer (I42)", ans.Confidence)).WithSource(src)
	}
	idx := int(math.Round(ans.Score))
	idx = min(max(idx, 0), len(preflightTiers)-1)
	return Present(ScoreReading{
		Tier: preflightTiers[idx], Level: ans.Score, Provider: resp.Provider, Model: resp.Model,
		LatencyMS: resp.Latency.Milliseconds(), InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens,
	}, src)
}

// PreflightScoreFor asks the decision port for the profile's Score; see
// preflightScore for when it does not.
func PreflightScoreFor(ctx context.Context, ports *decide.Ports, p Profile) Value[ScoreReading] {
	return preflightScore(ctx, ports, p)
}

// ─── outcomes (WI-19) ───────────────────────────────────────────────────────

// Outcome results.
const (
	ResultProven   = "proven"
	ResultUnproven = "unproven"
)

// preflightOutcomeSchema is the ledger row's schema version.
const preflightOutcomeSchema = 1

// PreflightOutcome is one row worked: which model and effort did it and
// whether it was proven, with what the preflight had recommended.
type PreflightOutcome struct {
	SchemaVersion int    `json:"schema_version"`
	At            string `json:"at"`
	Service       string `json:"service"`
	Plan          string `json:"plan,omitempty"`
	Row           string `json:"row"`
	// The agent identity that worked the row (I58): harness, model, effort.
	Harness string `json:"harness"`
	Model   string `json:"model"`
	Effort  string `json:"effort"`
	Agent   string `json:"agent"`
	Result  string `json:"result"`
	// FirstAttempt is whether the row was proven with no claimed-unproven
	// reading before; nil when the caller could not tell.
	FirstAttempt *bool `json:"first_attempt,omitempty"`
	// Tier is what the published rule table recommended, ScoreTier and
	// ScoreProvider what the decision port's Score did, when asked.
	Tier          Tier   `json:"tier,omitempty"`
	TableVersion  int    `json:"table_version,omitempty"`
	ScoreTier     Tier   `json:"score_tier,omitempty"`
	ScoreProvider string `json:"score_provider,omitempty"`
}

// recordPreflightOutcome appends one outcome to the ledger at path (WI-19).
// The identity parts pass through agentIdentityPart, so a person's name or
// email is never recorded (I58); a refused outcome writes nothing. now is
// the injected clock, used only when the outcome names no time.
func recordPreflightOutcome(path string, o PreflightOutcome, now func() time.Time) error {
	switch {
	case path == "":
		return errors.New("preflight: no outcome ledger path")
	case o.Service == "" || o.Row == "":
		return errors.New("preflight: an outcome names its service and row")
	case o.Result != ResultProven && o.Result != ResultUnproven:
		return fmt.Errorf("preflight: outcome result %q is neither %q nor %q", o.Result, ResultProven, ResultUnproven)
	case o.Tier != "" && o.Tier.rank() < 0:
		return fmt.Errorf("preflight: unknown recommended tier %q", o.Tier)
	case o.ScoreTier != "" && o.ScoreTier.rank() < 0:
		return fmt.Errorf("preflight: unknown score tier %q", o.ScoreTier)
	}
	o.SchemaVersion = preflightOutcomeSchema
	if o.At == "" {
		o.At = now().UTC().Format(time.RFC3339)
	}
	o.Harness, o.Model, o.Effort = agentIdentityPart(o.Harness, "unknown"), agentIdentityPart(o.Model, "unknown"), agentIdentityPart(o.Effort, "unknown")
	o.Agent = agentIdentityKey(o.Harness, o.Model, o.Effort)
	if o.Tier != "" {
		o.TableVersion = TierRuleTableVersion
	}
	line, err := json.Marshal(o)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// RecordPreflightOutcome appends an outcome to the ledger at path; see
// recordPreflightOutcome.
func RecordPreflightOutcome(path string, o PreflightOutcome, now func() time.Time) error {
	return recordPreflightOutcome(path, o, now)
}

// PreflightOutcomes reads the newest tail bytes of the outcome ledger,
// oldest first, skipping (and counting) a malformed line. An absent ledger
// is absent — no outcome is recorded yet — and an unreadable one an error.
func PreflightOutcomes(ctx context.Context, path string, tail int64) ([]PreflightOutcome, SourceRead) {
	read := SourceRead{Name: "preflight outcome ledger", Path: path}
	if path == "" {
		read.State, read.Reason = StateAbsent, "no home directory to find the preflight outcome ledger in"
		return nil, read
	}
	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		read.State, read.Reason = StateAbsent, "no outcome ledger at "+path+" — no preflight outcome is recorded"
		return nil, read
	case err != nil:
		read.State, read.Reason = StateError, err.Error()
		return nil, read
	}
	if err := ctx.Err(); err != nil {
		read.State, read.Reason = StateError, err.Error()
		return nil, read
	}
	if tail <= 0 {
		tail = DefaultHistoryTail
	}
	data, truncated, err := readTail(path, fi.Size(), tail)
	if err != nil {
		read.State, read.Reason = StateError, err.Error()
		return nil, read
	}
	var out []PreflightOutcome
	malformed := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var o PreflightOutcome
		if json.Unmarshal([]byte(line), &o) != nil || o.Row == "" {
			malformed++
			continue
		}
		out = append(out, o)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	read.State = StatePresent
	var notes []string
	if truncated {
		notes = append(notes, fmt.Sprintf("the newest %d MiB read", tail>>20))
	}
	if malformed > 0 {
		notes = append(notes, fmt.Sprintf("%d malformed lines skipped", malformed))
	}
	read.Reason = strings.Join(notes, "; ")
	return out, read
}

// PreflightCalibration is what the recorded outcomes say about the
// recommendations: per recommended tier, how many rows were proven at the
// first attempt. It is a descriptive table — no causal claim (law 12) —
// and is uncalibrated until a source has ScorecardMinN outcomes.
type PreflightCalibration struct {
	Outcomes   int              `json:"outcomes"`
	Proven     int              `json:"proven"`
	Unproven   int              `json:"unproven"`
	RuleTable  []CalibrationRow `json:"rule_table"`
	Score      []CalibrationRow `json:"score"`
	Calibrated bool             `json:"calibrated"`
	Note       string           `json:"note"`
}

// PreflightCalibrationOf tabulates outcomes: a bucket per recommended
// tier, hits the rows proven at the first attempt among the outcomes whose
// first-attempt reading is known.
func PreflightCalibrationOf(outcomes []PreflightOutcome) PreflightCalibration {
	c := PreflightCalibration{RuleTable: []CalibrationRow{}, Score: []CalibrationRow{}}
	type acc struct{ hits, n int }
	table, score := map[Tier]*acc{}, map[Tier]*acc{}
	add := func(m map[Tier]*acc, t Tier, o PreflightOutcome) {
		if t == "" || (o.Result == ResultProven && o.FirstAttempt == nil) {
			return
		}
		a := m[t]
		if a == nil {
			a = &acc{}
			m[t] = a
		}
		a.n++
		if o.Result == ResultProven && *o.FirstAttempt {
			a.hits++
		}
	}
	for _, o := range outcomes {
		c.Outcomes++
		if o.Result == ResultProven {
			c.Proven++
		} else {
			c.Unproven++
		}
		add(table, o.Tier, o)
		add(score, o.ScoreTier, o)
	}
	rows := func(m map[Tier]*acc) ([]CalibrationRow, int) {
		var out []CalibrationRow
		total := 0
		for _, t := range preflightTiers {
			if a := m[t]; a != nil {
				out = append(out, CalibrationRow{Bucket: string(t), Hits: a.hits, N: a.n, Fraction: round2(float64(a.hits) / float64(a.n))})
				total += a.n
			}
		}
		return out, total
	}
	var tn, sn int
	c.RuleTable, tn = rows(table)
	c.Score, sn = rows(score)
	if c.RuleTable == nil {
		c.RuleTable = []CalibrationRow{}
	}
	if c.Score == nil {
		c.Score = []CalibrationRow{}
	}
	c.Calibrated = tn >= ScorecardMinN || sn >= ScorecardMinN
	switch {
	case c.Outcomes == 0:
		c.Note = "uncalibrated: no outcome is recorded yet; a tier is a rule's reading, not an estimate"
	case !c.Calibrated:
		c.Note = fmt.Sprintf("uncalibrated: %s recorded, and a reading needs %d against one source; a tier is a rule's reading, not an estimate", nOf(c.Outcomes, "outcome", "outcomes"), ScorecardMinN)
	default:
		c.Note = fmt.Sprintf("descriptive: the share of rows proven at the first attempt per recommended tier over %s; it says nothing about cause", nOf(c.Outcomes, "recorded outcome", "recorded outcomes"))
	}
	return c
}

// PreflightModel is the whole preflight of one row: the profile, the
// table's recommendation, the decision port's Score when it was asked, and
// the calibration read back from recorded outcomes.
type PreflightModel struct {
	Profile        Profile             `json:"profile"`
	Recommendation Recommendation      `json:"recommendation"`
	Score          Value[ScoreReading] `json:"score"`
	// ScoreCalibration is the Score's calibration table: present only when
	// a Score is shown with it.
	ScoreCalibration Value[[]CalibrationRow] `json:"score_calibration"`
	Calibration      PreflightCalibration    `json:"calibration"`
	RuleTable        []TierRuleDoc           `json:"rule_table"`
}

// Preflight is the one projection the CLI verb and the console page share:
// the row's profile and recommendation, the Score the caller asked for (a
// zero Value when it did not), and the calibration over outcomes.
func Preflight(in PreflightInput, score Value[ScoreReading], outcomes []PreflightOutcome) PreflightModel {
	p := DifficultyProfile(in)
	m := PreflightModel{Profile: p, Recommendation: recommendTier(p), Score: score, Calibration: PreflightCalibrationOf(outcomes), RuleTable: TierRules()}
	if m.Score.State == "" {
		m.Score = Absent[ScoreReading]("the decision port's Score is asked for only on request, with [decide] configured")
	}
	if m.Score.Known() {
		m.ScoreCalibration = Present(m.Calibration.Score, "recorded preflight outcomes")
		if len(m.Calibration.Score) == 0 {
			m.ScoreCalibration = Absent[[]CalibrationRow]("no recorded outcome carries a Score tier yet")
		}
	} else {
		m.ScoreCalibration = Absent[[]CalibrationRow]("no Score is shown")
	}
	return m
}
