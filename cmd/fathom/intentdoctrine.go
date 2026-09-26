package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	readmanifest "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/read-manifest"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/readcoverage"
)

// intentdoctrine.go — `fathom intent doctrine` and doctor's `doctrine` row:
// the doctrine plane's two computed views (vision 23 §2; custodian plan
// DP-3, DP-4).
//
//   - The vision promotion pipeline: where each vision stands — its status
//     or trigger, the plans that carry it and their rows, whether its links
//     resolve, and how many days since its newest dated statement.
//   - Goals as pre-registered bets: every scope.md goal is a bet (an
//     instrument this repo can run, a threshold, a review date) or is
//     refused at load, named with what it lacks.
//
// Both are pure re-derivations from files: no daemon, no index, no ledger
// write, never an edit. The only side effect is the `intent-cli` beat that
// makes the verb's use measurable (law 7), as for every `fathom intent`
// subverb. No new MCP tool, no hook, no injected channel (ADR-026 §4).

// doctrineReport is one computation, shared by the verb and the doctor row
// so the two can never report different numbers for the same tree (RC-6).
type doctrineReport struct {
	Root     string
	Corpus   intent.Corpus
	Pipeline intent.VisionPipeline
	Bets     []intent.GoalBet
	// BetFindings are refusals, passed review dates, and bet blocks placed
	// anywhere but a scope.md goal (intent.MisplacedBets).
	BetFindings []intent.Claim
	Coverage    readmanifest.Manifest
	// Now is the one clock every date in the report was judged against.
	Now     time.Time
	Elapsed time.Duration
}

func buildDoctrine(root string, now time.Time) doctrineReport {
	start := time.Now()
	c, _ := intent.Load(root) // Load never fails; what it cannot read is on c.Coverage
	vp := intent.LoadVisions(c, now)
	bets, betReads := intent.LoadBets(c, isVerb)
	merged := readmanifest.NewSet()
	merged.Merge(c.Coverage)
	merged.Merge(vp.Coverage)
	merged.Merge(betReads) // an unreadable Taskfile or script makes every count a lower bound
	return doctrineReport{
		Root: root, Corpus: c, Pipeline: vp, Bets: bets,
		BetFindings: append(intent.BetFindings(bets, now), intent.MisplacedBets(c)...),
		Coverage:    merged.Manifest(),
		Now:         now,
		Elapsed:     time.Since(start),
	}
}

func cmdIntentDoctrine(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("intent doctrine", stderr)
	root := fs.String("root", ".", "repository root")
	kind := fs.String("kind", "", "restrict to: visions|bets (comma-separated)")
	top := fs.Int("top", intentTopDefault, "max findings per section before eliding (0 = all)")
	asJSON := fs.Bool("json", false, "emit JSON (unbudgeted)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	kinds, kerr := doctrineKinds(*kind)
	if kerr != nil {
		fmt.Fprintf(stderr, "error: %v\n", kerr)
		return 1
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	rep := buildDoctrine(abs, time.Now().UTC())

	refused := 0
	for _, b := range rep.Bets {
		if b.Refused != "" {
			refused++
		}
	}
	emitBeat("intent-cli", "doctrine", map[string]string{
		"visions":          itoa(len(rep.Pipeline.Visions)),
		"vision_findings":  itoa(len(rep.Pipeline.Findings)),
		"goals":            itoa(len(rep.Bets)),
		"refused":          itoa(refused),
		"incomplete_reads": itoa(len(rep.Coverage.Unreadable())),
	})

	if *asJSON {
		return emitDoctrineJSON(stdout, rep)
	}
	renderDoctrine(stdout, rep, kinds, *top)
	return 0
}

// doctrineKindNames are the sections --kind selects.
var doctrineKindNames = []string{"visions", "bets"}

// doctrineKinds parses --kind. An unknown kind is an error naming the
// accepted ones: a typo that silently matched nothing printed a header-only
// report and exited 0 — a plausible zero (ADR-027).
func doctrineKinds(flag string) ([]string, error) {
	if strings.TrimSpace(flag) == "" {
		return nil, nil
	}
	var out []string
	for _, k := range strings.Split(flag, ",") {
		k = strings.ToLower(strings.TrimSpace(k))
		if !sliceContains(doctrineKindNames, k) {
			return nil, fmt.Errorf("unknown --kind %q (want %s, comma-separated)", k, strings.Join(doctrineKindNames, "|"))
		}
		out = append(out, k)
	}
	return out, nil
}

// stageOrder is the pipeline's reading order: furthest along first, the
// named gaps last.
var stageOrder = []intent.VisionStage{
	intent.StageLanded, intent.StagePlanned, intent.StageAccepted, intent.StageForReview,
	intent.StageDeferred, intent.StageConditional, intent.StageStated,
	intent.StageSuperseded, intent.StageRejected, intent.StageUnstated, intent.StageUnreadable,
}

func renderStages(vs []intent.Vision) string {
	by := map[intent.VisionStage]int{}
	for _, v := range vs {
		by[v.Stage]++
	}
	var parts []string
	for _, s := range stageOrder {
		if by[s] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", s, by[s]))
		}
	}
	return strings.Join(parts, ", ")
}

// renderDoctrine writes the report; kinds (already validated by
// doctrineKinds) selects sections, nil for all.
func renderDoctrine(w io.Writer, rep doctrineReport, kinds []string, top int) {
	want := func(k string) bool {
		return len(kinds) == 0 || sliceContains(kinds, k)
	}
	vp := rep.Pipeline
	fmt.Fprintf(w, "doctrine plane for %s\n", rep.Root)

	if want("visions") {
		if vp.Home == "" {
			fmt.Fprintf(w, "\nvisions: none — %s\n", vp.Absent)
		} else {
			fmt.Fprintf(w, "  vision series: %s", vp.Home)
			if vp.Index != "" {
				fmt.Fprintf(w, " (index %s)", vp.Index)
			}
			fmt.Fprintln(w)
			if vp.IndexStatus != "" {
				fmt.Fprintf(w, "  index status (context only, never applied per vision): %s\n", vp.IndexStatus)
			}
			fmt.Fprintf(w, "\nvisions: %s — %s\n", strings.TrimSpace(readcoverage.RenderCount(vp.Coverage, len(vp.Visions), "")), renderStages(vp.Visions))
			for _, v := range vp.Visions {
				fmt.Fprintf(w, "  %-3s %-11s %s\n", v.Number, v.Stage, visionLine(v))
			}
			renderFindings(w, "vision findings", vp.Findings, top)
			for _, u := range vp.Uncomputable {
				fmt.Fprintf(w, "  NOT COMPUTED — %s\n", u)
			}
			fmt.Fprintln(w, "  plan counts are CLAIMED rows; `fathom verify --plan <plan>` is the proof of record")
		}
	}

	if want("bets") {
		bets, refused := 0, 0
		for _, b := range rep.Bets {
			if b.Refused != "" {
				refused++
			} else {
				bets++
			}
		}
		if len(rep.Bets) == 0 {
			fmt.Fprintln(w, "\ngoals: none — no `**G<n>.**` goal in docs/intent/scope.md")
			if len(rep.BetFindings) > 0 {
				renderFindings(w, "goal findings", rep.BetFindings, top) // a misplaced bet block still shows
			}
		} else {
			fmt.Fprintf(w, "\ngoals: %d — %d bet(s), %d refused\n", len(rep.Bets), bets, refused)
			now := rep.Now.UTC().Truncate(24 * time.Hour)
			for _, b := range rep.Bets {
				if b.Refused != "" {
					fmt.Fprintf(w, "  %-4s refused  %s\n", b.Goal.ID, trimTo(b.Goal.Title, 60))
					continue
				}
				due := ""
				if !b.Review.IsZero() {
					d := int(b.Review.Sub(now).Hours() / 24)
					if d >= 0 {
						due = fmt.Sprintf(" (in %d d)", d)
					} else {
						due = fmt.Sprintf(" (DUE %d d ago)", -d)
					}
				}
				name := b.Name
				if name == "" {
					name = "—"
				}
				fmt.Fprintf(w, "  %-4s bet      %s · `%s` [%s] · threshold %s · review %s%s\n",
					b.Goal.ID, name, b.Instrument, b.InstrumentKind, b.Threshold, b.ReviewRaw, due)
			}
			renderFindings(w, "goal findings", rep.BetFindings, top)
		}
	}

	if !rep.Coverage.Complete() {
		fmt.Fprintf(w, "  NOT COMPUTED — COVERAGE INCOMPLETE — every count above is a LOWER BOUND: %s\n", readcoverage.Note(rep.Coverage))
	}
}

func visionLine(v intent.Vision) string {
	var parts []string
	if v.Status != "" {
		parts = append(parts, trimTo(v.Status, 70)+" ["+v.StatusFrom+"]")
	}
	for _, p := range v.Plans {
		s := fmt.Sprintf("plan %s %d/%d claimed", p.Path, p.Claimed, p.Total)
		if p.Open > 0 {
			s += fmt.Sprintf(", %d open", p.Open)
		}
		if p.Archived {
			s += " (archived)"
		}
		parts = append(parts, s)
	}
	if len(v.Triggers) > 0 {
		parts = append(parts, fmt.Sprintf("%d trigger(s)", len(v.Triggers)))
	}
	if v.Dated != "" {
		parts = append(parts, fmt.Sprintf("dated %s (%d d)", v.Dated, v.AgeDays))
	} else {
		parts = append(parts, "undated")
	}
	title := trimTo(titleNumberRe.ReplaceAllString(v.Title, ""), 60)
	if title == "" {
		title = v.File
	}
	return title + " — " + strings.Join(parts, " · ")
}

// titleNumberRe strips the "23 — " / "Vision 17 — " a vision's heading
// repeats from its number column.
var titleNumberRe = regexp.MustCompile(`^(?:Vision\s+)?\d+\s*[—–:-]\s*`)

func renderFindings(w io.Writer, label string, fs []intent.Claim, top int) {
	if len(fs) == 0 {
		fmt.Fprintf(w, "  %s: none\n", label)
		return
	}
	fmt.Fprintf(w, "  %s: %d\n", label, len(fs))
	for i, f := range fs {
		if top > 0 && i >= top {
			fmt.Fprintf(w, "    … %d more elided (--top 0 for all)\n", len(fs)-i)
			break
		}
		fmt.Fprintf(w, "    [%s] %s — %s\n      → %s\n", f.Kind, f.Ref, f.Detail, f.Remedy)
	}
}

func trimTo(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

type doctrineFindingJSON struct {
	Plane    string `json:"plane"` // "visions" | "goals"
	Kind     string `json:"kind"`
	Ref      string `json:"ref"`
	Detail   string `json:"detail"`
	Remedy   string `json:"remedy"`
	Evidence string `json:"evidence,omitempty"`
}

type doctrineGoalJSON struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	File           string `json:"file"`
	Line           int    `json:"line"`
	Condition      string `json:"condition,omitempty"`
	Bet            string `json:"bet,omitempty"`
	Instrument     string `json:"instrument,omitempty"`
	InstrumentKind string `json:"instrument_kind,omitempty"`
	Threshold      string `json:"threshold,omitempty"`
	Review         string `json:"review,omitempty"`
	Refused        string `json:"refused,omitempty"`
}

func emitDoctrineJSON(w io.Writer, rep doctrineReport) int {
	var findings []doctrineFindingJSON
	for _, f := range rep.Pipeline.Findings {
		findings = append(findings, doctrineFindingJSON{"visions", f.Kind, f.Ref, f.Detail, f.Remedy, f.Evidence})
	}
	for _, f := range rep.BetFindings {
		findings = append(findings, doctrineFindingJSON{"goals", f.Kind, f.Ref, f.Detail, f.Remedy, f.Evidence})
	}
	goals := make([]doctrineGoalJSON, 0, len(rep.Bets))
	for _, b := range rep.Bets {
		goals = append(goals, doctrineGoalJSON{
			ID: b.Goal.ID, Title: b.Goal.Title, File: b.Goal.File, Line: b.Goal.Line,
			Condition: b.Condition, Bet: b.Name, Instrument: b.Instrument, InstrumentKind: b.InstrumentKind,
			Threshold: b.Threshold, Review: b.ReviewRaw, Refused: b.Refused,
		})
	}
	stages := map[string]int{}
	for _, v := range rep.Pipeline.Visions {
		stages[string(v.Stage)]++
	}
	visions := rep.Pipeline.Visions
	if visions == nil {
		visions = []intent.Vision{}
	}
	payload := map[string]any{
		"root":          rep.Root,
		"visions_home":  rep.Pipeline.Home,
		"visions_index": rep.Pipeline.Index,
		"index_status":  rep.Pipeline.IndexStatus,
		"absent":        rep.Pipeline.Absent,
		"stale_days":    rep.Pipeline.StaleDays,
		"visions":       visions,
		"vision_stages": stages,
		"goals":         goals,
		"findings":      findings,
		"uncomputable":  rep.Pipeline.Uncomputable,
		// ADR-027: a machine reader must tell a real zero from a blind one.
		"coverage": map[string]any{
			"complete":   rep.Coverage.Complete(),
			"note":       readcoverage.Note(rep.Coverage),
			"unreadable": rep.Coverage.Unreadable(),
			"entries":    len(rep.Coverage.Entries),
		},
		"ms": rep.Elapsed.Milliseconds(),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return 1
	}
	return 0
}

// checkDoctrine is doctor's `doctrine` row. Like the other intent rows it
// never FAILs: an unstated vision or a refused goal is honest debt, not a
// broken repository, and a row that is red everywhere teaches the reader to
// skim past the ones that matter (O23).
func checkDoctrine(root string) doctorCheck {
	const name = "doctrine"
	if capabilityOffAt(root, "intent-corpus") {
		return doctorCheck{
			Name: name, Status: doctorPass,
			Reason: "switched off by this repository (.fathom/capabilities.cue) — `fathom intent doctrine` still answers on demand",
		}
	}
	rep := buildDoctrine(root, time.Now().UTC())
	vp := rep.Pipeline
	if vp.Home == "" && len(rep.Bets) == 0 && len(rep.BetFindings) == 0 {
		if !rep.Coverage.Complete() {
			return doctorCheck{
				Name: name, Status: doctorSkipped,
				Reason: "could not read the doctrine sources: " + readcoverage.Note(rep.Coverage),
				Remedy: "check the named path(s) are readable; on an external volume this is usually a wedged TCC verdict — `tccutil reset SystemPolicyRemovableVolumes <terminal bundle id>`",
			}
		}
		return doctorCheck{
			Name: name, Status: doctorPass,
			Reason: "no vision series and no scope goals — the doctrine plane has nothing to track here, which is a choice, not a defect",
		}
	}

	var parts []string
	if vp.Home == "" {
		parts = append(parts, "no vision series")
	} else {
		parts = append(parts, fmt.Sprintf("%s [%s]",
			readcoverage.RenderCount(vp.Coverage, len(vp.Visions), "vision(s)"), renderStages(vp.Visions)))
		if n := len(vp.Findings); n > 0 {
			parts = append(parts, "vision findings: "+renderClaimKindsOf(vp.Findings))
		}
	}
	bets, refused := 0, 0
	var betNames []string
	for _, b := range rep.Bets {
		if b.Refused != "" {
			refused++
			continue
		}
		bets++
		label := b.Goal.ID
		if b.Name != "" {
			label += " " + b.Name
		}
		betNames = append(betNames, label+" review "+b.ReviewRaw)
	}
	goalPart := fmt.Sprintf("goals: %d bet(s)", bets)
	if len(betNames) > 0 {
		goalPart += " [" + strings.Join(betNames, "; ") + "]"
	}
	goalPart += fmt.Sprintf(", %d refused", refused)
	parts = append(parts, goalPart)
	due, misplaced := 0, 0
	for _, f := range rep.BetFindings {
		switch f.Kind {
		case "review-due":
			due++
		case "misplaced-bet":
			misplaced++
		}
	}
	if due > 0 {
		parts = append(parts, fmt.Sprintf("%d bet review(s) DUE", due))
	}
	if misplaced > 0 {
		parts = append(parts, fmt.Sprintf("%d misplaced bet block(s) outside a scope goal", misplaced))
	}

	status, remedy := doctorPass, ""
	if len(vp.Findings)+len(rep.BetFindings) > 0 {
		status = doctorWarn
		remedy = "fathom intent doctrine  # every vision and goal, each finding with its remedy"
	}
	if !rep.Coverage.Complete() {
		status = doctorWarn
		parts = append(parts, "COVERAGE INCOMPLETE — counts are lower bounds: "+readcoverage.Note(rep.Coverage))
		if remedy == "" {
			remedy = "check the named path(s) are readable; on an external volume this is usually a wedged TCC verdict"
		}
	}
	return doctorCheck{Name: name, Status: status, Reason: strings.Join(parts, "; "), Remedy: remedy}
}

// renderClaimKindsOf renders "N kind" counts in kind order.
func renderClaimKindsOf(cs []intent.Claim) string {
	by := map[string]int{}
	for _, c := range cs {
		by[c.Kind]++
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d %s", by[k], k))
	}
	return strings.Join(parts, ", ")
}
