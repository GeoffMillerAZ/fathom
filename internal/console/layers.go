package console

// layers.go — the semantic-layer projections (vision 35 §4.2 "the strata",
// design §3 "Semantic layers", plan OC-27..OC-29): the four strata with
// every layer in its band, the services × layers coverage matrix, and one
// layer's compile history. `fathom layers strata|coverage|show --json`
// print exactly these, and /layers and /layers/{layer} render them (P10).

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// The four strata (ADR-024, docs/design/pack-taxonomy.md), in band order.
const (
	StratumDialect  = "dialect"
	StratumProvider = "provider"
	StratumPlatform = "platform"
	StratumConcern  = "concern"
)

// StrataView is the strata view: four bands, fleet totals and the claim
// lifecycle.
type StrataView struct {
	Bands     []StrataBand `json:"bands"`
	Totals    LayerTotals  `json:"totals"`
	Lifecycle Lifecycle    `json:"lifecycle"`
	// Notes name what could not be read (a pack catalog, claim events).
	Notes []string `json:"notes,omitempty"`
}

// StrataBand is one stratum.
type StrataBand struct {
	Stratum string `json:"stratum"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	// Parked is true for a stratum that exists only as vocabulary (provider,
	// TX-3); Reason says why a band holds nothing.
	Parked   bool           `json:"parked,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	Dialects []DialectLevel `json:"dialects,omitempty"`
	Layers   []LayerBlock   `json:"layers,omitempty"`
}

// DialectLevel groups the registered languages by where their typed edges
// come from (connector.TypedEdgeProducer: native, scip, file, syntax).
type DialectLevel struct {
	Level     string   `json:"level"`
	Languages []string `json:"languages"`
	// Producers names the SCIP delegates for the scip level.
	Producers []string `json:"producers,omitempty"`
}

// LayerBlock is one layer in its band.
type LayerBlock struct {
	Name   string `json:"name"`
	Source string `json:"source"` // embedded | project | kit | orphan
	Origin string `json:"origin"`
	Kind   string `json:"kind,omitempty"`
	Role   string `json:"role,omitempty"`
	Evaled bool   `json:"evaled"`
	// Claims is the fleet's current claims (fresh, stale, contradicted).
	Claims   Value[int]   `json:"claims"`
	Statuses StatusCounts `json:"statuses"`
	// Runs counts real compile runs (never dry runs or refusals); CostUSD
	// is what they spent.
	Runs    Value[int]     `json:"runs"`
	CostUSD Value[float64] `json:"cost_usd"`
	// Services have current claims from this layer; InstalledIn carry it as
	// a project pack.
	Services    []string `json:"services"`
	InstalledIn []string `json:"installed_in,omitempty"`
	// Refusals and DryRuns are runs that spent nothing: refused on their
	// estimate, or estimate-only.
	Refusals int    `json:"refusals,omitempty"`
	DryRuns  int    `json:"dry_runs,omitempty"`
	LastRun  string `json:"last_run,omitempty"`
	Note     string `json:"note,omitempty"`
}

// LayerTotals is the fleet's semantic-layer totals.
type LayerTotals struct {
	Layers   int            `json:"layers"`
	Claims   Value[int]     `json:"claims"`
	Runs     Value[int]     `json:"runs"`
	CostUSD  Value[float64] `json:"cost_usd"`
	Emitted  Value[int]     `json:"emitted"`
	Rejected Value[int]     `json:"rejected"`
	// Refused counts runs refused before they started (estimate over the
	// layer's budget); their estimates are never counted as spend.
	Refused Value[int] `json:"refused"`
	// Services is how many projects hold at least one current claim, of
	// Projects indexed.
	Services Value[int] `json:"services"`
	Projects int        `json:"projects"`
}

// Lifecycle is what happened to claims after compile: statuses now, and
// the verdict events recorded against them.
type Lifecycle struct {
	Statuses StatusCounts `json:"statuses"`
	// Events counts claim_events by verdict (re-anchored, contradicted,
	// corroborated, refuted, superseded, annotated); absent when the events
	// could not be read.
	Events map[string]Value[int] `json:"events"`
}

// lifecycleVerdicts is every verdict the lifecycle reports, zero or not.
var lifecycleVerdicts = []string{"re-anchored", "contradicted", "corroborated", "refuted", "superseded", "annotated"}

// Strata folds the knowledge read into the four bands.
func Strata(ctx context.Context, src KnowledgeSource) (StrataView, error) {
	k, err := readKnowledge(ctx, src)
	if err != nil {
		return StrataView{}, err
	}
	return strataFrom(k), nil
}

// LayersOverview is Strata and LayerCoverage from one read of the store —
// the /layers page shows both, and each equals its verb's own answer.
func LayersOverview(ctx context.Context, src KnowledgeSource) (StrataView, CoverageMatrix, error) {
	k, err := readKnowledge(ctx, src)
	if err != nil {
		return StrataView{}, CoverageMatrix{}, err
	}
	return strataFrom(k), coverageFrom(k), nil
}

func strataFrom(k knowledge) StrataView {
	out := StrataView{Bands: []StrataBand{
		{Stratum: StratumDialect, Title: "Dialect", Summary: "How code becomes an index: one extractor per language, typed edges where a checker or a SCIP index provides them.", Dialects: dialectLevels()},
		{Stratum: StratumProvider, Title: "Provider", Summary: "What an external technology means. Parked until a named consumer needs it.", Parked: true, Reason: "no provider packs — parked pack type (TX-3, docs/design/pack-taxonomy.md)"},
		{Stratum: StratumPlatform, Title: "Platform", Summary: "Rules an organization or a kit ships for its adopters, installed under .fathom/layers."},
		{Stratum: StratumConcern, Title: "Concern", Summary: "What rules, decisions and specs say, compiled by a model from each repository's own documents; every claim cites its lines."},
	}}
	for _, p := range k.PackErr {
		out.Notes = append(out.Notes, "pack catalog unreadable: "+p)
	}
	var total StatusCounts
	var runs, emitted, rejected, refused int
	var cost float64
	var readErrs []string
	for _, s := range k.Services {
		if s.Err != "" {
			readErrs = append(readErrs, s.Service)
		}
	}
	for _, name := range k.layerNames() {
		b := layerBlock(k, name)
		total.Fresh += b.Statuses.Fresh
		total.Stale += b.Statuses.Stale
		total.Contradicted += b.Statuses.Contradicted
		total.Superseded += b.Statuses.Superseded
		total.Refuted += b.Statuses.Refuted
		for _, s := range k.Services {
			c := k.cell(s, name)
			runs += c.Runs
			cost += c.CostUSD
			emitted += c.Emitted
			rejected += c.Rejected
			refused += c.Refusals
		}
		band := StratumConcern
		if p := k.Packs[name]; p != nil && p.Stratum == StratumPlatform {
			band = StratumPlatform
		}
		for i := range out.Bands {
			if out.Bands[i].Stratum == band {
				out.Bands[i].Layers = append(out.Bands[i].Layers, b)
			}
		}
		out.Totals.Layers++
	}
	for i := range out.Bands {
		if out.Bands[i].Stratum == StratumPlatform && len(out.Bands[i].Layers) == 0 {
			out.Bands[i].Reason = "no platform packs installed in any indexed repository"
		}
	}
	out.Totals.Projects = len(k.Services)
	covered := 0
	for _, s := range k.Services {
		for _, cl := range s.Claims {
			if cl.Status.Served() {
				covered++
				break
			}
		}
	}
	fleet := func(v Value[int]) Value[int] {
		if len(readErrs) > 0 {
			return Partial(v.V, fmt.Sprintf("lower bound; %s unreadable: %s", countNoun(len(readErrs), "project"), strings.Join(readErrs, ", "))).WithSource(storeSource)
		}
		return v
	}
	out.Totals.Claims = currentClaims(k)
	out.Totals.Runs = fleet(Measured(runs, storeSource).WithDenominator("real compile runs (dry runs and refusals excluded)"))
	out.Totals.Emitted = fleet(Measured(emitted, storeSource).WithDenominator("claims emitted by real compile runs"))
	out.Totals.Rejected = fleet(Measured(rejected, storeSource).WithDenominator("candidate claims the compile's verification rejected"))
	out.Totals.Refused = fleet(Measured(refused, storeSource).WithDenominator("runs refused before starting: the estimate exceeded the layer's budget; nothing was spent"))
	out.Totals.Services = fleet(Measured(covered, storeSource).WithDenominator(fmt.Sprintf("projects with a current claim, of %d indexed", len(k.Services))))
	if runs == 0 {
		out.Totals.CostUSD = Absent[float64]("no real compile run recorded").WithSource(storeSource)
	} else {
		c := Measured(roundCents(cost), storeSource).WithDenominator(countNoun(runs, "real compile run"))
		if len(readErrs) > 0 {
			c = Partial(roundCents(cost), "lower bound; some projects unreadable").WithSource(storeSource)
		}
		out.Totals.CostUSD = c
	}
	out.Lifecycle = lifecycle(k, total)
	return out
}

// MetricClaims is the fleet's current governed claims — shown on /layers
// and /packs, computed once by currentClaims (OC-9's rule).
const MetricClaims MetricID = "claims.current"

// currentClaims counts the claims serving can still use (fresh, stale,
// contradicted) across every project; an unreadable project makes it a
// lower bound naming the project.
func currentClaims(k knowledge) Value[int] {
	var st StatusCounts
	var unreadable []string
	for _, s := range k.Services {
		if s.Err != "" {
			unreadable = append(unreadable, s.Service)
			continue
		}
		for _, c := range s.Claims {
			st.add(c.Status)
		}
	}
	if len(unreadable) > 0 {
		return Partial(st.Current(), fmt.Sprintf("lower bound; %s unreadable: %s", countNoun(len(unreadable), "project"), strings.Join(unreadable, ", "))).WithSource(storeSource)
	}
	return Measured(st.Current(), storeSource).WithDenominator("current claims (fresh, stale, contradicted) across " + countNoun(len(k.Services), "project"))
}

// lifecycle counts the claim events by verdict.
func lifecycle(k knowledge, statuses StatusCounts) Lifecycle {
	lc := Lifecycle{Statuses: statuses, Events: map[string]Value[int]{}}
	const evSource = "claim_events (store-wide; events carry no service)"
	if k.EventsErr != "" {
		for _, v := range lifecycleVerdicts {
			lc.Events[v] = Errored[int]("claim events unreadable: " + k.EventsErr).WithSource(evSource)
		}
		return lc
	}
	counts := map[string]int{}
	for _, e := range k.Events {
		counts[e.Verdict]++
	}
	for _, v := range lifecycleVerdicts {
		lc.Events[v] = Measured(counts[v], evSource).WithDenominator("recorded verdicts")
	}
	for v, n := range counts {
		if _, ok := lc.Events[v]; !ok && v != "" {
			lc.Events[v] = Measured(n, evSource).WithDenominator("recorded verdicts")
		}
	}
	return lc
}

// layerBlock folds one layer across the fleet.
func layerBlock(k knowledge, name string) LayerBlock {
	p := k.Packs[name]
	src, origin := packSourceOf(p)
	b := LayerBlock{Name: name, Source: src, Origin: origin, Services: []string{}}
	if p != nil {
		// LayerRole's normalization: anything but claims governs.
		b.Kind, b.Role, b.Evaled = p.Kind, semantic.RoleGoverns, p.Evaled
		if p.Role == semantic.RoleClaims {
			b.Role = semantic.RoleClaims
		}
		b.InstalledIn = p.InstalledIn
		if p.LoadError != "" {
			b.Note = "broken project pack ignored: " + p.LoadError
		}
	}
	var runs int
	var cost float64
	var last time.Time
	var errs []string
	compiledAnywhere := false
	for _, s := range k.Services {
		if s.Err != "" {
			errs = append(errs, s.Service)
			continue
		}
		c := k.cell(s, name)
		b.Statuses.Fresh += c.Statuses.Fresh
		b.Statuses.Stale += c.Statuses.Stale
		b.Statuses.Contradicted += c.Statuses.Contradicted
		b.Statuses.Superseded += c.Statuses.Superseded
		b.Statuses.Refuted += c.Statuses.Refuted
		runs += c.Runs
		cost += c.CostUSD
		b.Refusals += c.Refusals
		b.DryRuns += c.DryRuns
		if c.LastRun.After(last) {
			last = c.LastRun
		}
		if c.Statuses.Current() > 0 {
			b.Services = append(b.Services, s.Service)
		}
		if c.Runs > 0 || c.Statuses.Current()+c.Statuses.Superseded+c.Statuses.Refuted > 0 {
			compiledAnywhere = true
		}
	}
	b.LastRun = rfc3339(last)
	switch {
	case len(errs) > 0:
		b.Claims = Partial(b.Statuses.Current(), "lower bound; unreadable: "+strings.Join(errs, ", ")).WithSource(storeSource)
	case !compiledAnywhere:
		b.Claims = Absent[int]("never compiled or imported in any indexed project").WithSource(storeSource)
	default:
		b.Claims = Measured(b.Statuses.Current(), storeSource).WithDenominator(fmt.Sprintf("current claims across %s", countNoun(len(b.Services), "project")))
	}
	if runs == 0 {
		reason := "no real compile run recorded"
		if name == semantic.LayerFindings {
			reason = "findings are imported, not compiled"
		}
		b.Runs = Absent[int](reason).WithSource(storeSource)
		b.CostUSD = Absent[float64](reason).WithSource(storeSource)
	} else {
		b.Runs = Measured(runs, storeSource).WithDenominator("real compile runs").WithAsOf(last)
		b.CostUSD = Measured(roundCents(cost), storeSource).WithDenominator(countNoun(runs, "real compile run"))
	}
	return b
}

// dialectLevels groups every registered language by its typed-edge
// producer level, nearest to a type checker first.
func dialectLevels() []DialectLevel {
	order := []string{"native", "scip", "file", "syntax"}
	byLevel := map[string]*DialectLevel{}
	for _, l := range order {
		byLevel[l] = &DialectLevel{Level: l}
	}
	producers := map[string]map[string]bool{}
	for lang, prod := range connector.TypedEdgeProducers() {
		level, tool, _ := strings.Cut(prod, ":")
		dl := byLevel[level]
		if dl == nil {
			dl = &DialectLevel{Level: level}
			byLevel[level] = dl
			order = append(order, level)
		}
		dl.Languages = append(dl.Languages, LanguageName(lang))
		if tool != "" {
			if producers[level] == nil {
				producers[level] = map[string]bool{}
			}
			producers[level][tool] = true
		}
	}
	var out []DialectLevel
	for _, l := range order {
		dl := byLevel[l]
		if len(dl.Languages) == 0 {
			continue
		}
		sort.Strings(dl.Languages)
		for t := range producers[l] {
			dl.Producers = append(dl.Producers, t)
		}
		sort.Strings(dl.Producers)
		out = append(out, *dl)
	}
	return out
}

// CoverageMatrix is the services × layers matrix: current claims per
// project and layer, a cell nothing compiled drawn as missing. Its columns
// are the layers compiled (or imported) somewhere in the fleet; a layer
// nothing ever compiled is listed in NeverCompiled with where its pack is
// installed, rather than as a column of empty cells.
type CoverageMatrix struct {
	Layers []string      `json:"layers"`
	Rows   []CoverageRow `json:"rows"`
	// Covered counts projects with at least one current claim.
	Covered       Value[int]     `json:"covered"`
	NeverCompiled []LayerAbsence `json:"never_compiled"`
}

// CoverageRow is one project.
type CoverageRow struct {
	Service string         `json:"service"`
	Total   int            `json:"total"`
	Cells   []CoverageCell `json:"cells"`
}

// CoverageCell is one (project, layer) cell.
type CoverageCell struct {
	Layer   string     `json:"layer"`
	Claims  Value[int] `json:"claims"`
	Runs    int        `json:"runs"`
	LastRun string     `json:"last_run,omitempty"`
}

// LayerAbsence is a layer no project has compiled or imported.
type LayerAbsence struct {
	Layer       string   `json:"layer"`
	Source      string   `json:"source"`
	Reason      string   `json:"reason"`
	InstalledIn []string `json:"installed_in,omitempty"`
}

// LayerCoverage computes the matrix. Rows are ordered by current claims,
// most first, then by name; columns by layer name.
func LayerCoverage(ctx context.Context, src KnowledgeSource) (CoverageMatrix, error) {
	k, err := readKnowledge(ctx, src)
	if err != nil {
		return CoverageMatrix{}, err
	}
	return coverageFrom(k), nil
}

func coverageFrom(k knowledge) CoverageMatrix {
	out := CoverageMatrix{Layers: []string{}, Rows: []CoverageRow{}, NeverCompiled: []LayerAbsence{}}
	for _, layer := range k.layerNames() {
		b := layerBlock(k, layer)
		if b.Claims.State == StateAbsent {
			abs := LayerAbsence{Layer: layer, Source: b.Source, Reason: b.Claims.Reason, InstalledIn: b.InstalledIn}
			if layer == semantic.LayerFindings {
				abs.Reason = "no findings imported (the layer is filled by `fathom findings` imports, never compiled)"
			}
			out.NeverCompiled = append(out.NeverCompiled, abs)
			continue
		}
		out.Layers = append(out.Layers, layer)
	}
	covered, unreadable := 0, 0
	for _, s := range k.Services {
		row := CoverageRow{Service: s.Service, Cells: []CoverageCell{}}
		for _, layer := range out.Layers {
			c := k.cell(s, layer)
			v := cellClaims(s, c)
			if p := k.Packs[layer]; v.State == StateAbsent && p != nil && p.Source == semantic.SourceProject && !containsStr(p.InstalledIn, s.Service) {
				v = Absent[int]("not installed — " + layer + " is a project pack this repository does not carry").WithSource(storeSource)
			}
			if v.Known() {
				row.Total += v.V
			}
			row.Cells = append(row.Cells, CoverageCell{Layer: layer, Claims: v, Runs: c.Runs, LastRun: rfc3339(c.LastRun)})
		}
		if s.Err != "" {
			unreadable++
		} else if row.Total > 0 {
			covered++
		}
		out.Rows = append(out.Rows, row)
	}
	sort.SliceStable(out.Rows, func(i, j int) bool {
		if out.Rows[i].Total != out.Rows[j].Total {
			return out.Rows[i].Total > out.Rows[j].Total
		}
		return out.Rows[i].Service < out.Rows[j].Service
	})
	denom := fmt.Sprintf("projects with a current claim, of %d indexed; superseded and refuted claims excluded", len(k.Services))
	if unreadable > 0 {
		out.Covered = Partial(covered, fmt.Sprintf("lower bound; %s unreadable", countNoun(unreadable, "project"))).WithSource(storeSource).WithDenominator(denom)
	} else {
		out.Covered = Measured(covered, storeSource).WithDenominator(denom)
	}
	return out
}

// LayerPage is one layer's page: its definition, compile runs with cost,
// accepted and rejected counts, the rejection reasons, what each project's
// latest run left uncompiled, and the dry-run command.
type LayerPage struct {
	Layer string `json:"layer"`
	// Known is false when neither a pack nor the store knows the layer.
	Known      bool             `json:"known"`
	Definition *LayerDefinition `json:"definition,omitempty"`
	Block      LayerBlock       `json:"summary"`
	Runs       []LayerRun       `json:"runs"`
	Rejections []RejectionCause `json:"rejections"`
	// RejectionsNote says why no reasons are listed when rejected > 0 (runs
	// recorded before reasons were kept).
	RejectionsNote string          `json:"rejections_note,omitempty"`
	Accepted       Value[int]      `json:"accepted"`
	Rejected       Value[int]      `json:"rejected"`
	Projects       []LayerProject  `json:"projects"`
	DryRun         string          `json:"dry_run"`
	Uncompiled     []UncompiledRun `json:"uncompiled"`
}

// LayerDefinition is the pack as the fleet resolves it.
type LayerDefinition struct {
	Version        string   `json:"version,omitempty"`
	Stratum        string   `json:"stratum"`
	Tier           string   `json:"tier,omitempty"`
	PromptFile     string   `json:"prompt_file,omitempty"`
	CorpusInclude  []string `json:"corpus_include,omitempty"`
	CorpusExclude  []string `json:"corpus_exclude,omitempty"`
	MaxTokensRun   int      `json:"max_tokens_per_run,omitempty"`
	MaxUSDRun      float64  `json:"max_usd_per_run,omitempty"`
	ServingMax     int      `json:"serving_max_rules,omitempty"`
	DriftTracked   bool     `json:"drift_tracked"`
	OverriddenIn   []string `json:"overridden_in,omitempty"`
	ProjectPackIn  []string `json:"project_pack_in,omitempty"`
	BrokenPackNote string   `json:"broken_pack,omitempty"`
}

// LayerRun is one compile run of the layer.
type LayerRun struct {
	RunID      string  `json:"run_id"`
	Service    string  `json:"service"`
	RanAt      string  `json:"ran_at"`
	Model      string  `json:"model,omitempty"`
	Kind       string  `json:"kind"` // compile | dry-run | refused
	CostUSD    float64 `json:"cost_usd"`
	TokensIn   int     `json:"tokens_in"`
	TokensOut  int     `json:"tokens_out"`
	Sections   int     `json:"sections_total"`
	Compiled   int     `json:"sections_compiled"`
	Failed     int     `json:"sections_failed"`
	Accepted   int     `json:"claims_accepted"`
	Rejected   int     `json:"claims_rejected"`
	Stopped    bool    `json:"budget_stopped,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	ResumedAt  string  `json:"resumed_from,omitempty"`
	Uncompiled int     `json:"uncompiled_scopes"`
}

// RejectionCause is one rejection cause across the layer's runs.
type RejectionCause struct {
	Cause   string   `json:"cause"`
	Count   int      `json:"count"`
	Samples []string `json:"samples,omitempty"`
}

// LayerProject is one project's slice of the layer.
type LayerProject struct {
	Service  string       `json:"service"`
	Claims   Value[int]   `json:"claims"`
	Statuses StatusCounts `json:"statuses"`
	Runs     int          `json:"runs"`
	LastRun  string       `json:"last_run,omitempty"`
	DryRun   string       `json:"dry_run"`
}

// UncompiledRun is what one project's latest real run left uncompiled.
type UncompiledRun struct {
	Service  string   `json:"service"`
	RunID    string   `json:"run_id"`
	RanAt    string   `json:"ran_at"`
	Sections int      `json:"sections_total"`
	Compiled int      `json:"sections_compiled"`
	Stopped  bool     `json:"budget_stopped,omitempty"`
	Scopes   []string `json:"scopes"`
	Resume   string   `json:"resume"`
}

// maxRejectionSamples bounds each cause's example messages.
const maxRejectionSamples = 3

// LayerDetail computes one layer's page. An unknown layer returns Known
// false and no error.
func LayerDetail(ctx context.Context, src KnowledgeSource, layer string) (LayerPage, error) {
	k, err := readKnowledge(ctx, src)
	if err != nil {
		return LayerPage{}, err
	}
	out := LayerPage{Layer: layer, Runs: []LayerRun{}, Rejections: []RejectionCause{}, Projects: []LayerProject{}, Uncompiled: []UncompiledRun{}}
	for _, n := range k.layerNames() {
		if n == layer {
			out.Known = true
		}
	}
	if !out.Known {
		return out, nil
	}
	out.Block = layerBlock(k, layer)
	out.DryRun = "fathom compile --layer " + layer + " --service <project>"
	if p := k.Packs[layer]; p != nil {
		d := &LayerDefinition{
			Version: p.Version, Stratum: p.Stratum, Tier: p.Tier, PromptFile: p.PromptFile,
			CorpusInclude: p.CorpusInclude, CorpusExclude: p.CorpusExclude,
			MaxTokensRun: p.Budgets.MaxTokensPerRun, MaxUSDRun: p.Budgets.MaxUSDPerRun,
			ServingMax: p.Serving.MaxRules, DriftTracked: p.Serving.DriftTracked,
			BrokenPackNote: p.LoadError,
		}
		if d.Stratum == "" {
			d.Stratum = StratumConcern
		}
		if p.Source == semantic.SourceEmbedded {
			d.OverriddenIn = p.InstalledIn
		} else {
			d.ProjectPackIn = p.InstalledIn
		}
		out.Definition = d
	}
	causes := map[string]*RejectionCause{}
	accepted, rejected, reasonedRuns, rejectingRuns := 0, 0, 0, 0
	var errs []string
	for _, s := range k.Services {
		if s.Err != "" {
			errs = append(errs, s.Service)
			continue
		}
		c := k.cell(s, layer)
		lp := LayerProject{Service: s.Service, Claims: cellClaims(s, c), Statuses: c.Statuses, Runs: c.Runs, LastRun: rfc3339(c.LastRun),
			DryRun: "fathom compile --layer " + layer + " --service " + s.Service}
		if c.Runs > 0 || c.DryRuns > 0 || c.Refusals > 0 || c.Statuses.Current()+c.Statuses.Superseded+c.Statuses.Refuted > 0 {
			out.Projects = append(out.Projects, lp)
		}
		latestSeen := false
		for _, r := range s.Runs { // newest first
			if r.Layer != layer {
				continue
			}
			out.Runs = append(out.Runs, layerRun(r))
			if !realRun(r) {
				continue
			}
			accepted += r.ClaimsEmitted
			rejected += r.ClaimsRejected
			if r.ClaimsRejected > 0 {
				rejectingRuns++
				if len(r.RejectionReasons) > 0 {
					reasonedRuns++
				}
			}
			for _, rb := range r.RejectionReasons {
				rc := causes[rb.Cause]
				if rc == nil {
					rc = &RejectionCause{Cause: rb.Cause}
					causes[rb.Cause] = rc
				}
				rc.Count += rb.Count
				for _, smp := range rb.Samples {
					if len(rc.Samples) < maxRejectionSamples && !containsStr(rc.Samples, smp) {
						rc.Samples = append(rc.Samples, smp)
					}
				}
			}
			if !latestSeen {
				latestSeen = true
				if len(r.UncompiledScopes) > 0 || r.BudgetStopped || r.SectionsCompiled < r.SectionsTotal {
					out.Uncompiled = append(out.Uncompiled, UncompiledRun{
						Service: s.Service, RunID: r.RunID, RanAt: rfc3339(r.RanAt),
						Sections: r.SectionsTotal, Compiled: r.SectionsCompiled, Stopped: r.BudgetStopped,
						Scopes: append([]string{}, r.UncompiledScopes...),
						Resume: "fathom compile --layer " + layer + " --service " + s.Service + " --continue",
					})
				}
			}
		}
	}
	sort.SliceStable(out.Runs, func(i, j int) bool {
		if out.Runs[i].RanAt != out.Runs[j].RanAt {
			return out.Runs[i].RanAt > out.Runs[j].RanAt
		}
		return out.Runs[i].RunID < out.Runs[j].RunID
	})
	for _, rc := range causes {
		out.Rejections = append(out.Rejections, *rc)
	}
	sort.Slice(out.Rejections, func(i, j int) bool {
		if out.Rejections[i].Count != out.Rejections[j].Count {
			return out.Rejections[i].Count > out.Rejections[j].Count
		}
		return out.Rejections[i].Cause < out.Rejections[j].Cause
	})
	if rejectingRuns > reasonedRuns {
		out.RejectionsNote = fmt.Sprintf("%d of %s with rejections predate rejection reasons (O19) and carry only a count", rejectingRuns-reasonedRuns, countNoun(rejectingRuns, "run"))
	}
	if len(out.Runs) == 0 {
		why := "no compile run recorded for this layer"
		out.Accepted = Absent[int](why).WithSource(storeSource)
		out.Rejected = Absent[int](why).WithSource(storeSource)
	} else {
		out.Accepted = Measured(accepted, storeSource).WithDenominator("claims written by real compile runs")
		out.Rejected = Measured(rejected, storeSource).WithDenominator("candidate claims verification rejected")
		if len(errs) > 0 {
			reason := "lower bound; unreadable: " + strings.Join(errs, ", ")
			out.Accepted = Partial(accepted, reason).WithSource(storeSource)
			out.Rejected = Partial(rejected, reason).WithSource(storeSource)
		}
	}
	return out, nil
}

func layerRun(r core.CompileRun) LayerRun {
	lr := LayerRun{
		RunID: r.RunID, Service: r.ServiceID, RanAt: rfc3339(r.RanAt), Model: r.Model, Kind: "compile",
		CostUSD: roundCents(r.CostUSD), TokensIn: r.TokensIn, TokensOut: r.TokensOut,
		Sections: r.SectionsTotal, Compiled: r.SectionsCompiled, Failed: r.SectionsFailed,
		Accepted: r.ClaimsEmitted, Rejected: r.ClaimsRejected, Stopped: r.BudgetStopped,
		Reason: r.RefusalReason, ResumedAt: r.ResumedFrom, Uncompiled: len(r.UncompiledScopes),
	}
	switch {
	case r.DryRun:
		lr.Kind = "dry-run"
	case r.Refused:
		lr.Kind = "refused"
	}
	return lr
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
