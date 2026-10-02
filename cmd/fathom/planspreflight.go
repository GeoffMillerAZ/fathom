// planspreflight.go — WI-16..WI-19 (docs/design/work-intelligence.md §3
// U6, the Jev site): the task preflight of one plan row. `fathom plans
// context <plan.md> --row <id> --preflight` prints the row's difficulty
// profile and the published rule table's tier recommendation; `--score`
// also asks the decision port for its Score, only with [decide] configured
// and the `preflight` site allowed; `fathom plans outcome` records what
// happened (the model used, its effort, proven or not) to calibrate it.
// The plan-row console page shows the same console.Preflight model.
//
// Read-only and advisory: nothing here starts, assigns or routes work — the
// harness decides (WI-X3) — and the one write is the outcome verb's single
// appended ledger line.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/projectrules"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// Bounds on one preflight (tenancy: bounded store reads, no tree walk).
const (
	preflightMaxAnchorFiles = 8
	preflightLedgerTail     = 4 << 20
)

// preflightLedgerPath resolves the outcome ledger: $FATHOM_PREFLIGHT_LEDGER,
// else ~/.fathom/preflight-outcomes.jsonl.
func preflightLedgerPath() string {
	return fathomhome.PerFile("FATHOM_PREFLIGHT_LEDGER", "preflight-outcomes.jsonl")
}

// preflightSources is what one preflight reads from the index: the
// caller's open client, the call graph over the service's symbols and the
// service's root.
type preflightSources struct {
	c       *client.Client
	graph   *callgraph.Graph
	nodes   []core.SymbolNode
	service string
	root    string
}

// newPreflightSources loads the symbols and call graph the way the
// context pack's own index does (syntax edges plus the typed pass).
func newPreflightSources(ctx context.Context, c *client.Client, service, root string) (*preflightSources, error) {
	nodes, err := c.ListAllSymbols(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("listing symbols for %q: %w", service, err)
	}
	g := callgraph.Build(nodes)
	if typed, terr := c.TypedEdges(ctx, service); terr == nil {
		callgraph.AddTypedEdges(g, typed)
	}
	return &preflightSources{c: c, graph: g, nodes: nodes, service: service, root: root}, nil
}

// anchorNodes are the definitions of the row's symbol (under its package
// when it names one).
func (s *preflightSources) anchorNodes(ctx context.Context, symbol, pkg string) []core.SymbolNode {
	if symbol == "" {
		return nil
	}
	found, err := s.c.FindSymbol(ctx, s.service, symbol)
	if err != nil {
		return nil
	}
	var out []core.SymbolNode
	for _, n := range found {
		if n.Kind == core.KindDocChunk {
			continue
		}
		if pkg != "" && !strings.HasPrefix(n.FilePath, strings.TrimSuffix(pkg, "/")+"/") {
			continue
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// preflightInput resolves every fact a row's profile reads. A fact that
// cannot be read is named in its Value, never defaulted.
func (s *preflightSources) preflightInput(ctx context.Context, row console.PreflightRow, changes []console.ProvenChange, zones console.Zones) console.PreflightInput {
	in := console.PreflightInput{Row: row, Interest: console.InterestRate(changes)}
	anchors := s.anchorNodes(ctx, row.Symbol, row.Package)
	fileSet := map[string]bool{}
	for _, n := range anchors {
		fileSet[n.FilePath] = true
	}
	files := sortedStringKeys(fileSet)
	if len(files) > preflightMaxAnchorFiles {
		files = files[:preflightMaxAnchorFiles]
	}
	none := "the row has no symbol anchor"
	if row.Symbol != "" && len(anchors) == 0 {
		none = fmt.Sprintf("the row's symbol %q does not resolve in the index", row.Symbol)
	}

	for _, f := range files {
		in.Zones = append(in.Zones, console.ZoneOf(f, zones))
	}

	// Blast radius: the transitive callers of the anchor (the largest when
	// the name is ambiguous).
	if len(anchors) == 0 {
		in.Blast = console.Absent[console.BlastRadius](none)
	} else {
		entry := map[string]bool{}
		for _, n := range symbolnav.Entrypoints(s.nodes) {
			entry[n.Key()] = true
		}
		// DependentsDetail: a type's consumers (references, not calls,
		// since wave lg) are still what a change to it breaks.
		callersOf := func(k string) []string {
			ds := callgraph.DependentsDetail(s.graph, k)
			out := make([]string, 0, len(ds))
			for _, d := range ds {
				out = append(out, d.Key)
			}
			return out
		}
		best := console.Value[console.BlastRadius]{}
		for _, n := range anchors {
			v := console.BlastRadiusOf(n.Key(), callersOf, func(k string) bool { return entry[k] }, 0)
			if best.State == "" || v.V.Callers > best.V.Callers {
				best = v
			}
		}
		if len(anchors) > 1 {
			best.Source += fmt.Sprintf("; %d definitions match the name, the widest is shown", len(anchors))
		}
		in.Blast = best
	}

	// Governing rules and their levels, at the anchor's files.
	switch {
	case len(files) == 0:
		in.Rules = console.Absent[[]console.GovernedRule](none)
	default:
		in.Rules = s.governingRules(ctx, files)
	}

	// Certificates a change to the anchor would re-prove.
	switch {
	case len(files) == 0:
		in.Certs = console.Absent[[]console.CertToReprove](none)
	default:
		in.Certs = s.certificates(ctx, files)
	}

	if len(changes) > 0 {
		ix, err := console.RowFeatureIndexFor(ctx, s.service+"\x00"+s.root+"\x00"+changesWatermark(changes), func() []console.ProvenChange { return changes })
		if err == nil {
			in.Index = ix
		}
	}
	return in
}

// changesWatermark keys the row index's memo group by what it is built
// from: the newest proven change and how many there are.
func changesWatermark(changes []console.ProvenChange) string {
	newest := ""
	for _, c := range changes {
		if c.At > newest {
			newest = c.At
		}
	}
	return fmt.Sprintf("%d@%s", len(changes), newest)
}

func sortedStringKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// governingRules serves the obligations governing each anchor file — a
// stale or contradicted rule is level warn, a fresh one info, the change
// risk receipt's own mapping — and the architecture rules that forbid a
// dependency from the anchor's package, level error (the review engine's
// own severity for a broken one).
func (s *preflightSources) governingRules(ctx context.Context, files []string) console.Value[[]console.GovernedRule] {
	rules := []console.GovernedRule{}
	for _, f := range files {
		obl, err := semantic.ObligationsFor(ctx, s.c.Store(), s.service, f, 0)
		if err != nil {
			return console.Errored[[]console.GovernedRule](fmt.Sprintf("the obligations for %s could not be read: %v", f, err))
		}
		for _, r := range obl.Rules {
			level := console.RuleLevelInfo
			if r.Status == "stale" || r.Status == "contradicted" {
				level = console.RuleLevelWarn
			}
			rules = append(rules, console.GovernedRule{Rule: r.Rule, Level: level, File: f, Citation: r.Src})
		}
	}
	arch, _, _, checked, _, aerr := projectrules.ResolveRules(s.root, nil, false)
	if aerr == nil && checked {
		dirs := map[string]bool{}
		for _, f := range files {
			dirs[path.Dir(f)] = true
		}
		for _, r := range arch {
			if r.Allow || r.FromPkg == "" || r.FromPkg == "*" {
				continue
			}
			for _, d := range sortedStringKeys(dirs) {
				if d == r.FromPkg || strings.HasPrefix(d, strings.TrimSuffix(r.FromPkg, "/")+"/") {
					rules = append(rules, console.GovernedRule{Rule: fmt.Sprintf("architecture: %s must not depend on %s", r.FromPkg, r.ToPkg), Level: console.RuleLevelError, File: d})
					break
				}
			}
		}
	}
	return console.Present(rules, "the obligations layer and the architecture rules at the anchor's files")
}

// certificates lists the certifications whose scope includes an anchor file.
func (s *preflightSources) certificates(ctx context.Context, files []string) console.Value[[]console.CertToReprove] {
	claims, err := s.c.Store().ClaimsForService(ctx, s.service, certify.Layer)
	if err != nil {
		return console.Errored[[]console.CertToReprove]("the certifications layer could not be read: " + err.Error())
	}
	inFiles := map[string]bool{}
	for _, f := range files {
		inFiles[f] = true
	}
	seen := map[string]bool{}
	out := []console.CertToReprove{}
	for _, r := range certify.Records(claims) {
		for _, f := range r.Scope {
			if inFiles[f] && !seen[r.Subject] {
				seen[r.Subject] = true
				out = append(out, console.CertToReprove{Subject: r.Subject, State: r.State})
			}
		}
	}
	return console.Present(out, "the certifications layer, scoped to the anchor's files")
}

// preflightWork reads the repository's proven changes for U2 and WI-15
// the way /work does: the proof history joined to the session ledger and
// one read-only git show for the proving commits' files.
func preflightWork(ctx context.Context, service, root string, now time.Time) workPageModel {
	m := observatoryModel{}
	if p := sessionLedgerPath(); p != "" {
		if data, err := readFileTail(p, preflightSessionTail); err == nil {
			m.ledger = foldSessionLedger(data)
		} else if !os.IsNotExist(err) {
			m.ledgerErr = err
		}
	}
	return computeWork(ctx, m, serviceRoot{Service: service, Root: root}, now)
}

// preflightSessionTail bounds the CLI's read of the session ledger.
const preflightSessionTail = 8 << 20

// ─── plans context --preflight ──────────────────────────────────────────────

// preflightOpenPorts opens the decision port for `--score`; a variable so
// a test can inject a fake decider.
var preflightOpenPorts = func(stderr io.Writer) *decide.Ports {
	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		fmt.Fprintf(stderr, "warn: decision port disabled: loading config: %v\n", err)
		loaded = config.Loaded{Config: config.Config{Decide: config.DefaultDecide}}
	}
	return openDecisionPorts(loaded, "", resolveCallLogPath(), "fathom plans context", stderr)
}

// runPreflight prints the preflight of task to stdout. Direct mode only
// (--store): the profile reads the index's call graph, obligations and
// certifications, which the daemon path does not expose to the CLI.
func runPreflight(ctx context.Context, task verify.Task, planFile, service, storePath string, score bool, stdout, stderr io.Writer) {
	if storePath == "" {
		fmt.Fprintln(stdout, "\npreflight: needs direct mode — pass --store DB (the daemon path does not serve the call graph, obligations and certifications a profile reads)")
		return
	}
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		fmt.Fprintf(stdout, "\npreflight: opening store %q: %v\n", storePath, err)
		return
	}
	defer c.Close()
	root := verifyRepoRoot()
	src, err := newPreflightSources(ctx, c, service, root)
	if err != nil {
		fmt.Fprintf(stdout, "\npreflight: %v\n", err)
		return
	}
	now := time.Now().UTC()
	work := preflightWork(ctx, service, root, now)
	rel := planRelToRoot(root, planFile)
	retries, weight, _ := console.RowProofFacts(work.history, service, rel, task.ID)
	if task.Weight > 0 {
		weight = task.Weight
	}
	row := console.PreflightRow{Service: service, Plan: rel, ID: task.ID, Title: task.Title, Symbol: task.Expect.Symbol, Package: task.Expect.Package, Weight: weight, Retries: retries}
	in := src.preflightInput(ctx, row, work.changes, work.zones)
	profile := console.DifficultyProfile(in)
	sc := console.Value[console.ScoreReading]{}
	if score {
		sc = console.PreflightScoreFor(ctx, preflightOpenPorts(stderr), profile)
	}
	outcomes, _ := console.PreflightOutcomes(ctx, preflightLedgerPath(), preflightLedgerTail)
	fmt.Fprint(stdout, renderPreflightText(console.Preflight(in, sc, filterOutcomes(outcomes, service)), planFile, score))
}

// planRelToRoot names a plan file relative to the repository root when it
// lies under it (the spelling the proof history records).
func planRelToRoot(root, planFile string) string {
	abs, err := filepath.Abs(planFile)
	if err != nil || root == "" {
		return planFile
	}
	if rel, err := filepath.Rel(root, abs); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return planFile
}

func filterOutcomes(all []console.PreflightOutcome, service string) []console.PreflightOutcome {
	var out []console.PreflightOutcome
	for _, o := range all {
		if strings.EqualFold(o.Service, service) {
			out = append(out, o)
		}
	}
	return out
}

// renderPreflightText is the plain-text preflight `plans context` prints.
func renderPreflightText(m console.PreflightModel, planFile string, scoreAsked bool) string {
	var b strings.Builder
	p, rec := m.Profile, m.Recommendation
	fmt.Fprintf(&b, "\npreflight (%s)\n", console.PreflightAdvisory)
	val := func(known bool, reason string, s string) string {
		if known {
			return s
		}
		return "not measured — " + reason
	}
	fmt.Fprintf(&b, "  blast radius: %s\n", val(p.Blast.Known(), p.Blast.Reason,
		fmt.Sprintf("%s, %s%s", nOfWord(p.Blast.V.Callers, "transitive caller"), nOfWord(p.Blast.V.Entrypoints, "entrypoint"), lowerBound(p.Blast.State))))
	fmt.Fprintf(&b, "  governing rules: %s\n", val(p.Rules.Known(), p.Rules.Reason,
		fmt.Sprintf("%d (%d error, %d warn, %d info)", len(p.Rules.V), p.RuleLevels.Error, p.RuleLevels.Warn, p.RuleLevels.Info)))
	for _, r := range p.Rules.V {
		if r.Level != console.RuleLevelInfo {
			fmt.Fprintf(&b, "    %-5s %s (%s)\n", r.Level, r.Rule, r.File)
		}
	}
	fmt.Fprintf(&b, "  certificates to re-prove: %s\n", val(p.Certs.Known(), p.Certs.Reason, fmt.Sprint(len(p.Certs.V))))
	for _, c := range p.Certs.V {
		fmt.Fprintf(&b, "    %s (%s)\n", c.Subject, c.State)
	}
	if len(p.ZoneInterest) == 0 {
		fmt.Fprintf(&b, "  zone interest: not measured — %s\n", p.MaxZoneRate.Reason)
	}
	for _, z := range p.ZoneInterest {
		fmt.Fprintf(&b, "  zone %s: %s\n", z.Zone, val(z.Rate.Known(), z.Rate.Reason, fmt.Sprintf("interest rate %.2fx (n = %d)", z.Rate.V, z.N)))
	}
	fmt.Fprintf(&b, "  row: %s, plan weight %d\n", nOfWord(p.Row.Retries, "claimed-unproven reading"), p.Row.Weight)
	if p.Similar.Known() {
		s := p.Similar.V
		fmt.Fprintf(&b, "  %d nearest proven rows: median %.0f tokens, %.0f turns, %.0f s; %.0f%% proven at the first attempt%s\n",
			len(s.Rows), s.MedianTokens, s.MedianTurns, s.MedianWallSeconds, s.FirstAttemptRate*100, lowerBound(p.Similar.State))
		fmt.Fprintf(&b, "    similar on: %s\n", s.Basis)
	} else {
		fmt.Fprintf(&b, "  nearest proven rows: not measured — %s\n", p.Similar.Reason)
	}
	switch {
	case rec.Tier == "":
		fmt.Fprintf(&b, "  recommended tier: none — %s\n", strings.TrimSuffix(rec.Note, "; "+console.PreflightAdvisory))
	case rec.Complete:
		fmt.Fprintf(&b, "  recommended tier: %s (rule table revision %d)\n", rec.Tier, rec.TableVersion)
	default:
		fmt.Fprintf(&b, "  recommended tier: at least %s (rule table revision %d; %d rules could not be evaluated)\n", rec.Tier, rec.TableVersion, len(rec.Unevaluated))
	}
	for _, f := range rec.Fired {
		fmt.Fprintf(&b, "    %s %s → %s: %s\n", f.ID, f.When, f.Floor, f.Evidence)
	}
	for _, u := range rec.Unevaluated {
		fmt.Fprintf(&b, "    not evaluated %s\n", u)
	}
	switch {
	case m.Score.Known():
		fmt.Fprintf(&b, "  decision port score: %s (level %.2f; %s%s)\n", m.Score.V.Tier, m.Score.V.Level, m.Score.V.Provider, modelSuffix(m.Score.V.Model))
		if m.ScoreCalibration.Known() {
			for _, r := range m.ScoreCalibration.V {
				fmt.Fprintf(&b, "    calibration, score tier %s: %d of %d rows proven at the first attempt\n", r.Bucket, r.Hits, r.N)
			}
		} else {
			fmt.Fprintf(&b, "    calibration: none — %s\n", m.ScoreCalibration.Reason)
		}
	case scoreAsked:
		fmt.Fprintf(&b, "  decision port score: not shown — %s\n", m.Score.Reason)
	default:
		fmt.Fprintf(&b, "  decision port score: not asked — add --score, with [decide] configured\n")
	}
	for _, r := range m.Calibration.RuleTable {
		fmt.Fprintf(&b, "  calibration, table tier %s: %d of %d rows proven at the first attempt\n", r.Bucket, r.Hits, r.N)
	}
	fmt.Fprintf(&b, "  %s\n", m.Calibration.Note)
	outcomeCmd := fmt.Sprintf("fathom plans outcome %s --row %s --service %s --model <model> --effort <effort> --result proven|unproven", planFile, p.Row.ID, p.Row.Service)
	if rec.Tier != "" {
		outcomeCmd += " --tier " + string(rec.Tier)
	}
	if m.Score.Known() {
		outcomeCmd += " --score-tier " + string(m.Score.V.Tier) + " --score-provider " + m.Score.V.Provider
	}
	fmt.Fprintf(&b, "  when the row is worked, record what happened: %s\n", outcomeCmd)
	return b.String()
}

func nOfWord(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func lowerBound(s console.State) string {
	if s == console.StatePartial {
		return " (a lower bound)"
	}
	return ""
}

func modelSuffix(model string) string {
	if model == "" {
		return ""
	}
	return ", " + model
}

// ─── plans outcome ──────────────────────────────────────────────────────────

func printPlansOutcomeUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: fathom plans outcome <plan.md> --row <id> --service S --result proven|unproven --model M [--harness H] [--effort E] [--tier T] [--score-tier T --score-provider P] [--first-attempt true|false] [--ledger PATH]")
}

// cmdPlansOutcome appends one outcome to the preflight ledger (WI-19):
// the model used, its effort and whether the row was proven, with the tier
// the preflight recommended. The first-attempt reading comes from the proof
// history when the result is proven, unless --first-attempt says so.
func cmdPlansOutcome(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: plans outcome requires a plan file argument")
		printPlansOutcomeUsage(stderr)
		return 1
	}
	planFile := args[0]
	fs := newFlagSet("plans outcome", stderr)
	row := fs.String("row", "", "row id the outcome is for (required)")
	service := fs.String("service", "", "service ID (required)")
	result := fs.String("result", "", "proven or unproven (required)")
	harness := fs.String("harness", "", "the harness that worked the row (an agent identity, never a person)")
	model := fs.String("model", "", "the model that worked the row")
	effort := fs.String("effort", "", "the effort setting it ran at")
	tier := fs.String("tier", "", "the tier the rule table recommended (light, standard, heavy)")
	scoreTier := fs.String("score-tier", "", "the tier the decision port's Score gave, when it was asked")
	scoreProvider := fs.String("score-provider", "", "the provider that gave that Score")
	first := fs.String("first-attempt", "", "true or false; default: read from the proof history for a proven row")
	ledger := fs.String("ledger", preflightLedgerPath(), "outcome ledger path")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if *row == "" || *service == "" || *result == "" {
		fmt.Fprintln(stderr, "error: --row, --service and --result are required")
		printPlansOutcomeUsage(stderr)
		return 1
	}
	o := console.PreflightOutcome{
		Service: *service, Plan: planRelToRoot(verifyRepoRoot(), planFile), Row: *row, Harness: *harness, Model: *model, Effort: *effort,
		Result: *result, Tier: console.Tier(*tier), ScoreTier: console.Tier(*scoreTier), ScoreProvider: *scoreProvider,
	}
	switch *first {
	case "true", "false":
		v := *first == "true"
		o.FirstAttempt = &v
	case "":
		if *result == console.ResultProven {
			// PE-24: the registered roots' readings only, the rest named.
			h, _ := console.ProofHistory(context.Background(), verifyLedgerPath(), console.DefaultHistoryTail, cliLedgerRoots(defaultFathomURL()))
			if f, known := console.RowFirstAttempt(h, *service, o.Plan, *row); known {
				o.FirstAttempt = &f
			}
			if p := h.Ignored.Phrase("verify-ledger reading"); p != "" {
				fmt.Fprintf(stderr, "first attempt read from the proof history, registered roots only: %s\n", p)
			}
		}
	default:
		fmt.Fprintf(stderr, "error: --first-attempt must be true or false, got %q\n", *first)
		return 1
	}
	if err := console.RecordPreflightOutcome(*ledger, o, time.Now); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "recorded: row %s %s (%s) in %s\n", *row, *result, agentLabel(o), *ledger)
	return 0
}

// agentLabel is the identity an outcome is keyed by.
func agentLabel(o console.PreflightOutcome) string {
	h, m, e := o.Harness, o.Model, o.Effort
	if h == "" {
		h = "unknown"
	}
	if m == "" {
		m = "unknown"
	}
	if e == "" {
		e = "unknown"
	}
	if strings.ContainsAny(h+m+e, "@<>") {
		return console.IdentityWithheld
	}
	return h + "|" + m + "|" + e
}
