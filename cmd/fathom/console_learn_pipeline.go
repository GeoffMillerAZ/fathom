package main

// console_learn_pipeline.go — How fathom works at /learn (OC-46; vision 35
// §4.1, design §3 Learn): the pipeline — source, extract, index, compile,
// serve, gate, evidence — drawn with viz.LayoutGraph over one chosen
// project's live counts, each stage a panel naming its instrument and
// linking to the page that shows it in full, and the project's
// `fathom capabilities` rows beside it. The Learn group's shared project
// choice and the guides are console_learn.go; Ask the right way (OC-47) is
// console_learn_ask.go.
//
// Every number is a projection in internal/console (PipelineView) over
// facts each cached on its own input watermarks (the store's, the call
// log's, the verify ledger's), never on the model's clock; a stage whose
// instrument did not answer says so, never 0.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/metrics"
)

func init() {
	declarePage(consolePage{
		ID: "learn", Route: "/learn", Href: "/learn", Group: "Learn", Order: 0, Nav: true,
		Title:    "How fathom works",
		Question: "How does fathom turn a project's source into rules at the edit and evidence — and what is each stage doing on this project?",
		Verbs:    []string{"fathom fleet report --json"},
		Shows:    []console.MetricID{console.MetricServices},
		Build:    buildLearnPage,
	})
}

// learnTrafficWindow is the traffic window the Learn pages count tool
// calls over — the exact --since their verb chip names.
const learnTrafficWindow = 720 * time.Hour

// learnModel is /learn's JSON projection.
type learnModel struct {
	Project  string           `json:"project"`
	Chosen   string           `json:"chosen"`
	Root     string           `json:"root,omitempty"`
	Projects []string         `json:"projects"`
	Pipeline console.Pipeline `json:"pipeline"`
	// Capabilities is the command whose --json the page carries as the
	// section of the same name: the project's `fathom capabilities`.
	Capabilities string `json:"capabilities_verb,omitempty"`
}

func buildLearnPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	choice, err := chooseLearnProject(cr, m)
	if err != nil {
		return consoleOut{}, err
	}
	facts := readLearnFacts(cr, m, choice)
	pipe := console.PipelineView(facts.pipeline)
	shared := pageMetrics(cr.page, consoleModelMetrics(m))
	out := consoleOut{
		Meta:    learnMeta(choice, shared[console.MetricServices], m),
		Actions: learnProjectPicker(cr.r.URL.Path, choice, m),
		Model: learnModel{
			Project: choice.Service, Chosen: choice.How, Root: choice.Row.Path,
			Projects: learnProjectNames(m), Pipeline: pipe, Capabilities: facts.capVerb,
		},
		Metrics: shared,
		Body: func() string {
			return renderLearnPipeline(cr.deps.reg, choice, pipe, facts, m)
		},
	}
	if wantsJSON(cr.r) {
		out.Sections = map[string]any{}
		// The same report the fleet page carries (P10): the extract stage's
		// symbols and the evidence stage's sessions and calls are this
		// report's row for the project.
		rep, rerr := buildFleetReport(consoleRPCClient(5*time.Second), cr.deps.selfURL, fleetReportNow())
		if rerr != nil {
			out.Sections["fathom fleet report --json"] = map[string]string{"error": rerr.Error()}
		} else {
			out.Sections["fathom fleet report --json"] = rep
		}
		if facts.capVerb != "" {
			out.Sections[facts.capVerb] = facts.capModel
		}
	}
	return out, nil
}

// ─── the facts, each read cached on its own input watermarks ───────────────

// learnFacts is one project's reads for the Learn pages.
type learnFacts struct {
	pipeline console.PipelineFacts
	// capModel is `fathom capabilities --repo <root> --service <svc>
	// --json` computed by the verb's own functions; capVerb is that exact
	// command ("" when the probe could not run).
	capModel capabilityModel
	capVerb  string
	// traffic is the project's tool calls in learnTrafficWindow, by tool.
	traffic learnTraffic
}

// learnTraffic is one read of the call log for a project, by tool — the
// numbers `fathom metrics traffic --service S --by tool --since 720h`
// prints.
type learnTraffic struct {
	Total   console.Value[int]
	ByTool  map[string]int
	Verb    string
	Window  string
	LogPath string
}

// learnKeyed is a small cache of one kind of read, keyed by its input
// watermarks (design §1): an entry is reused only while every input it was
// read from is unchanged, and a read that failed is never kept — a cleared
// wedge shows its counts on the next request, not at the next watermark.
type learnKeyed[T any] struct {
	sync.Mutex
	entries map[string]learnKeyedEntry[T]
}

type learnKeyedEntry[T any] struct {
	v       T
	expires time.Time // zero: kept until its key changes
}

func (c *learnKeyed[T]) get(key string, now time.Time) (T, bool) {
	c.Lock()
	defer c.Unlock()
	e, ok := c.entries[key]
	if !ok || (!e.expires.IsZero() && now.After(e.expires)) {
		var zero T
		return zero, false
	}
	return e.v, true
}

func (c *learnKeyed[T]) put(key string, v T, expires time.Time) {
	c.Lock()
	defer c.Unlock()
	if c.entries == nil || len(c.entries) > 64 {
		c.entries = map[string]learnKeyedEntry[T]{}
	}
	c.entries[key] = learnKeyedEntry[T]{v: v, expires: expires}
}

func (c *learnKeyed[T]) reset() {
	c.Lock()
	c.entries = nil
	c.Unlock()
}

var (
	learnStoreCache learnKeyed[console.PipelineFacts]
	learnGateCache  learnKeyed[learnGate]
	learnCapsCache  learnKeyed[learnCaps]
)

// resetLearnCache forgets every project's cached reads (tests).
func resetLearnCache() {
	learnStoreCache.reset()
	learnGateCache.reset()
	learnCapsCache.reset()
	resetCallLogTail()
}

// readLearnFacts assembles a project's facts for a request. Nothing here
// is keyed on the model's clock: the store's side is reused while the
// store's watermark and compile runs are unchanged, the traffic is the
// call log read incrementally, the gate is reused while the verify ledger
// and the plans directory are unchanged, and the switches while the model
// facts and the files the probe reads are unchanged.
func readLearnFacts(cr *consoleRequest, m observatoryModel, choice learnChoice) learnFacts {
	svc, row := choice.Service, choice.Row
	var out learnFacts
	f := console.PipelineFacts{Service: svc}
	if svc == "" {
		why := choice.How
		for _, v := range []*console.Value[int]{&f.Files, &f.Symbols, &f.Edges, &f.TypedEdges, &f.ClaimsCurrent, &f.ClaimsFlagged,
			&f.ClaimsRetired, &f.CompileRuns, &f.RulesServed, &f.WindowCalls, &f.GateRuns, &f.GatePlans, &f.GateVerified,
			&f.GateClaimed, &f.Sessions, &f.Calls} {
			*v = console.Absent[int](why)
		}
		f.CapabilitiesNote = why
		out.pipeline = f
		out.traffic = learnTraffic{Total: console.Absent[int](why)}
		return out
	}
	f.Freshness, f.IndexedCommit, f.RepoHead, f.FreshnessRemedy = row.Freshness, row.IndexedCommit, row.RepoHead, row.FreshnessRemedy

	// Extract: the registry's symbol count (the fleet report's row).
	if row.SymbolsNote != "" {
		f.Symbols = console.Absent[int](row.SymbolsNote)
	} else {
		f.Symbols = console.Measured(row.Symbols, "the daemon's service registry (list_services)").WithDenominator("symbols indexed for " + svc)
	}
	// Evidence: the fleet report's session and call columns.
	callsRead := console.CallTotal(modelCallFacts(m))
	switch {
	case !callsRead.Known():
		f.Calls = console.Value[int]{State: callsRead.State, Reason: callsRead.Reason, Source: callsRead.Source}
	default:
		f.Calls = console.Measured(row.CallsTotal, callsRead.Source).WithDenominator("calls attributed to " + svc + ", console reads excluded")
	}
	if row.SessionCount != nil {
		f.Sessions = console.Measured(*row.SessionCount, "session ledger "+sessionLedgerPath()).WithDenominator("distinct sessions whose repository label names " + svc)
	} else {
		ms := modelSessions(m)
		reason := ms.Reason
		if reason == "" {
			reason = "the session ledger could not be read"
		}
		f.Sessions = console.Absent[int](reason)
	}

	ctx, cancel := context.WithTimeout(cr.r.Context(), 10*time.Second)
	defer cancel()
	sf := cachedLearnStoreFacts(ctx, cr, svc)
	f.Files, f.Languages, f.UnclassifiedFiles = sf.Files, sf.Languages, sf.UnclassifiedFiles
	f.Edges, f.TypedEdges = sf.Edges, sf.TypedEdges
	f.ClaimsCurrent, f.ClaimsFlagged, f.ClaimsRetired, f.Layers, f.CompileRuns = sf.ClaimsCurrent, sf.ClaimsFlagged, sf.ClaimsRetired, sf.Layers, sf.CompileRuns

	out.traffic = readLearnTraffic(cr, svc)
	f.WindowCalls = out.traffic.Total
	switch {
	case out.traffic.Total.Known():
		f.RulesServed = console.Measured(out.traffic.ByTool["obligations"], out.traffic.Total.Source).
			WithDenominator("obligations calls for " + svc + " in " + out.traffic.Window + ", console reads excluded")
	default:
		f.RulesServed = console.Value[int]{State: out.traffic.Total.State, Reason: out.traffic.Total.Reason, Source: out.traffic.Total.Source}
	}

	g := cachedVerifyLedgerFor(verifyLedgerPath(), svc, row.Path)
	f.GateRuns, f.GatePlans, f.GateVerified, f.GateClaimed = g.runs, g.plans, g.verified, g.claimed

	// The switches: `fathom capabilities` for the project's root, by the
	// verb's own functions, so the page and the verb cannot disagree.
	if row.Path == "" {
		f.CapabilitiesNote = "no corpus root is recorded for " + svc + ", so its switches cannot be probed"
	} else if root, err := filepath.Abs(row.Path); err != nil {
		f.CapabilitiesNote = "the recorded root could not be resolved: " + err.Error()
	} else {
		caps := cachedLearnCapabilities(cr, m, row, root, svc)
		out.capModel, out.capVerb = caps.model, learnCapabilitiesVerb(root, svc)
		f.Capabilities = caps.states
	}
	out.pipeline = f
	return out
}

// learnCaps is one project's `fathom capabilities`: the verb's model, and
// the rows as the pipeline reads them (with ABSENT and BROKEN told apart).
type learnCaps struct {
	model  capabilityModel
	states []console.CapabilityState
}

// learnFactsMaxAge bounds how long a cached read is reused while its key
// holds, for the inputs a key cannot see: a decision record the probe reads
// from elsewhere, a claim's status moved by feedback (fathom claim refute).
const learnFactsMaxAge = 10 * time.Minute

// cachedLearnCapabilities runs the capability probe once per change of its
// inputs: the model facts it reads for the project and the files under the
// project's root it reads.
func cachedLearnCapabilities(cr *consoleRequest, m observatoryModel, row obsServiceRow, root, svc string) learnCaps {
	key := strings.Join([]string{svc, root, learnProbeModelKey(m, row), learnProbeFilesKey(root)}, "\x00")
	now := time.Now()
	if c, ok := learnCapsCache.get(key, now); ok {
		return c
	}
	reg := capability.Default()
	probe := buildCapabilityProbe(cr.deps.selfURL, root, svc, cr.now)
	res, _ := loadCapabilityConfig(reg, root)
	reports := reg.DetectAll(probe, res)
	out := learnCaps{model: capabilityJSON(probe, reports)}
	for _, r := range reports {
		remedy := r.Remedy.Cmd
		if remedy == "" {
			remedy = r.Remedy.Note
		}
		out.states = append(out.states, console.CapabilityState{
			ID: r.Capability.ID, Operating: r.State.Operating, Broken: r.State.Broken,
			Detail: r.State.Detail, Reason: r.State.Reason, Remedy: remedy,
		})
	}
	learnCapsCache.put(key, out, now.Add(learnFactsMaxAge))
	return out
}

// learnProbeModelKey is every model fact buildCapabilityProbe reads for a
// project.
func learnProbeModelKey(m observatoryModel, row obsServiceRow) string {
	layers := "no configuration snapshot"
	if row.Config != nil {
		var names []string
		for _, l := range row.Config.Layers {
			names = append(names, l.Name)
		}
		layers = strings.Join(names, ",")
	}
	return fmt.Sprintf("%s|%d|%s|%s|%s|%s|%s|%v|%d|%d|%d", row.Path, row.Symbols, row.Freshness, row.HooksNote, row.StanzaNote,
		row.CompileNote, layers, m.Narrate.Active, m.Impact.LedgerSessions, m.AgentSurfaceTools, m.AgentSchemaBytes)
}

// learnProbeFilesKey is a stat signature of the files and directories the
// capability probe reads under a root (the rules, bindings, guards, layer
// packs and settings in .fathom, the lefthook files, the intent corpus and
// the decision records) plus the user's settings — a size or mtime change,
// or a file added or removed, changes it.
func learnProbeFilesKey(root string) string {
	var b strings.Builder
	stat := func(p string, listDir bool) {
		info, err := os.Stat(p)
		if err != nil {
			b.WriteString(p + ":-;")
			return
		}
		fmt.Fprintf(&b, "%s:%d:%d;", p, info.Size(), info.ModTime().UnixNano())
		if !info.IsDir() || !listDir {
			return
		}
		//fathom:tenancy-allow whole-read lists one directory's entries (names only), no file content
		entries, err := os.ReadDir(p)
		if err != nil {
			b.WriteString(p + ":unreadable;")
			return
		}
		for _, e := range entries {
			if fi, err := e.Info(); err == nil {
				fmt.Fprintf(&b, "%s:%d:%d;", e.Name(), fi.Size(), fi.ModTime().UnixNano())
			}
		}
	}
	for _, dir := range []string{".fathom", ".fathom/rules", ".fathom/layers", "docs/intent", "docs/adr"} {
		stat(filepath.Join(root, dir), true)
	}
	if nested, err := filepath.Glob(filepath.Join(root, "docs", "intent", "*", "")); err == nil {
		for _, d := range nested {
			stat(d, true)
		}
	}
	for _, f := range []string{"lefthook.yml", "lefthook.yaml", ".lefthook.yml", "ADR.md"} {
		stat(filepath.Join(root, f), false)
	}
	if p := capability.DefaultUserPath(); p != "" {
		stat(p, false)
	}
	return b.String()
}

// learnCapabilitiesVerb is the exact `fathom capabilities` command for a
// project, its root shell-quoted when it needs it.
func learnCapabilitiesVerb(root, svc string) string {
	return "fathom capabilities --repo " + shellWord(root) + " --service " + shellWord(svc) + " --json"
}

// cachedLearnStoreFacts reads the store's side of a project once per change
// of its inputs: the service's store watermark (every ingest moves it) and
// its compile runs (every compile records one). A read that failed is
// never kept.
func cachedLearnStoreFacts(ctx context.Context, cr *consoleRequest, svc string) console.PipelineFacts {
	var f console.PipelineFacts
	if cr.deps.c == nil {
		why := "this daemon has no store open"
		for _, v := range []*console.Value[int]{&f.Files, &f.Edges, &f.TypedEdges, &f.ClaimsCurrent, &f.ClaimsFlagged, &f.ClaimsRetired, &f.CompileRuns} {
			*v = console.Absent[int](why)
		}
		return f
	}
	runs, runsErr := cr.deps.c.Store().CompileRuns(ctx, svc, "")
	wm, wmErr := cr.deps.c.StoreWatermark(ctx, svc)
	key := ""
	if runsErr == nil && wmErr == nil {
		var last string
		if n := len(runs); n > 0 {
			last = fmt.Sprintf("%+v", runs[n-1])
		}
		key = fmt.Sprintf("%s\x00%s\x00%d\x00%s", svc, wm, len(runs), last)
		if cached, ok := learnStoreCache.get(key, time.Now()); ok {
			return cached
		}
	}
	readLearnStoreFacts(ctx, cr, svc, &f, runs, runsErr)
	failed := false
	for _, v := range []console.Value[int]{f.Files, f.Edges, f.ClaimsCurrent, f.CompileRuns} {
		failed = failed || v.State == console.StateError
	}
	if key != "" && !failed {
		// A claim's status can also move by feedback (fathom claim refute),
		// which neither input records: the age bounds that lag.
		learnStoreCache.put(key, f, time.Now().Add(learnFactsMaxAge))
	}
	return f
}

// readLearnStoreFacts reads the store's side: the files and languages the
// index holds, the typed-edge table and the claims, beside the compile runs
// the caller read.
func readLearnStoreFacts(ctx context.Context, cr *consoleRequest, svc string, f *console.PipelineFacts, runs []core.CompileRun, runsErr error) {
	st := cr.deps.c.Store()
	const storeSource = "the daemon's store"

	refs, err := st.ListSourceRefs(ctx, svc)
	if err != nil {
		f.Files = console.Errored[int]("the store's source refs could not be read: " + err.Error()).WithSource(storeSource)
	} else {
		f.Files = console.Measured(len(refs), storeSource+" (source refs: files with symbols or a fingerprint)").WithDenominator("files the index holds for " + svc)
		byLang := map[string]int{}
		for _, ref := range refs {
			switch lang := connector.LanguageForPath(ref); lang {
			case "", core.LangUnclassified:
				f.UnclassifiedFiles++
			default:
				byLang[lang]++
			}
		}
		for lang, n := range byLang {
			f.Languages = append(f.Languages, console.LanguageFiles{Lang: lang, Files: n, Producer: connector.TypedEdgeProducer(lang)})
		}
		sort.Slice(f.Languages, func(i, j int) bool {
			if f.Languages[i].Files != f.Languages[j].Files {
				return f.Languages[i].Files > f.Languages[j].Files
			}
			return f.Languages[i].Lang < f.Languages[j].Lang
		})
	}

	edges, err := st.TypedEdges(ctx, svc)
	switch {
	case err != nil:
		f.Edges = console.Errored[int]("the typed-edge table could not be read: " + err.Error())
		f.TypedEdges = f.Edges
	case len(edges) == 0:
		why := "no typed-pass edges are recorded for " + svc + " — no typed pass has run for its languages (who-calls then answers from syntax edges)"
		f.Edges = console.Absent[int](why).WithSource(storeSource)
		f.TypedEdges = f.Edges
	default:
		resolved := 0
		for _, e := range edges {
			if core.EdgeConfidenceOf(e) == core.EdgeConfidenceResolved {
				resolved++
			}
		}
		src := storeSource + " (typed_edges)"
		f.Edges = console.Measured(len(edges), src).WithDenominator("edges the typed pass recorded for " + svc)
		f.TypedEdges = console.Measured(resolved, src).WithDenominator(fmt.Sprintf("of %s, resolved by a type checker or SCIP index", fmtThousands(len(edges))))
	}

	real := 0
	for _, r := range runs {
		if !r.DryRun {
			real++
		}
	}
	claims, err := st.ClaimsForService(ctx, svc, "")
	switch {
	case err != nil:
		e := console.Errored[int]("the claims could not be read: " + err.Error())
		f.ClaimsCurrent, f.ClaimsFlagged, f.ClaimsRetired = e, e, e
	case len(claims) == 0 && (runsErr != nil || real == 0):
		a := console.Absent[int]("no layer has been compiled for " + svc).WithSource(storeSource)
		f.ClaimsCurrent, f.ClaimsFlagged, f.ClaimsRetired = a, a, a
	default:
		cur, flagged, retired := 0, 0, 0
		layers := map[string]bool{}
		for _, c := range claims {
			layers[c.Layer] = true
			switch c.Status {
			case core.ClaimFresh:
				cur++
			case core.ClaimStale, core.ClaimContradicted:
				flagged++
			case core.ClaimSuperseded, core.ClaimRefuted:
				retired++
			}
		}
		src := storeSource + " (semantic_claims)"
		denom := fmt.Sprintf("of %s compiled for %s", plural(len(claims), "claim"), svc)
		f.ClaimsCurrent = console.Measured(cur, src).WithDenominator(denom)
		f.ClaimsFlagged = console.Measured(flagged, src).WithDenominator(denom)
		f.ClaimsRetired = console.Measured(retired, src).WithDenominator(denom)
		for l := range layers {
			f.Layers = append(f.Layers, l)
		}
		sort.Strings(f.Layers)
	}
	if runsErr != nil {
		f.CompileRuns = console.Errored[int]("the compile runs could not be read: " + runsErr.Error())
	} else {
		f.CompileRuns = console.Measured(real, storeSource+" (compile_runs)").WithDenominator(fmt.Sprintf("real runs for %s; %d dry runs not counted", svc, len(runs)-real))
	}
}

// readLearnTraffic counts the project's tool calls in the window, by tool,
// from the resolved call log — the verb's own line decoding and window,
// read incrementally (callLogRows), console reads excluded. A missing log
// is absent and an unreadable one an error naming what failed, never 0.
func readLearnTraffic(cr *consoleRequest, svc string) learnTraffic {
	t := learnTraffic{
		Window: "the last 30 days",
		Verb:   "fathom metrics traffic --service " + shellWord(svc) + " --by tool --since 720h --json",
	}
	resolved := resolveConsoleCallLogFor(callLogQuery{BaseURL: cr.deps.selfURL})
	t.LogPath = resolved.Path
	source := "call log " + resolved.Path + " (" + string(resolved.Rule) + ")"
	if resolved.Path == "" {
		reason := resolved.Note
		if reason == "" {
			reason = "no call log was resolved"
		}
		t.Total = console.Absent[int](reason)
		return t
	}
	// The read itself decides, not a stat: on the external-volume wedge
	// stat succeeds and the read fails, and an unread log is never 0.
	rotated, live, err := callLogRowSegments(resolved.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Total = console.Absent[int]("no call log at " + resolved.Path + " — nothing recorded yet; traffic is not measured, never zero").WithSource(source)
		} else {
			t.Total = console.Errored[int]("call log unreadable: " + err.Error()).WithSource(source)
		}
		return t
	}
	until := cr.now.UTC()
	since := until.Add(-learnTrafficWindow)
	entries := callEntriesFor(svc, since, until, rotated, live)
	rep := metrics.AggregateTraffic(entries, metrics.TrafficOptions{Since: since, Until: until, GroupBy: metrics.TrafficGroupByTool})
	t.ByTool = map[string]int{}
	for _, r := range rep.Rows {
		t.ByTool[r.Group] = r.Calls
	}
	t.Total = console.Measured(rep.TotalCalls, source).WithDenominator("calls for " + svc + " in " + t.Window + ", console reads excluded")
	return t
}

// learnGate is one project's gate history from the verify ledger.
type learnGate struct {
	runs, plans, verified, claimed console.Value[int]
}

var verifyLedgerCache struct {
	sync.Mutex
	path    string
	size    int64
	mod     time.Time
	rows    []verifyLedgerRow
	readErr error
}

// cachedVerifyLedgerFor is readVerifyLedgerFor reused while the ledger
// (its size and mtime) and the project's plans directory (its mtime: a plan
// added or removed) are unchanged and both can still be read; a failed read
// is never kept, and one that fails now is reported now.
func cachedVerifyLedgerFor(path, svc, root string) learnGate {
	key := ""
	if path != "" && root != "" && learnReadable(path) && learnReadable(filepath.Join(root, "docs", "plans")) {
		li, lerr := os.Stat(path)
		pi, perr := os.Stat(filepath.Join(root, "docs", "plans"))
		plansSig := "-"
		if perr == nil {
			plansSig = fmt.Sprintf("%d", pi.ModTime().UnixNano())
		}
		if lerr == nil && (perr == nil || errors.Is(perr, os.ErrNotExist)) {
			key = fmt.Sprintf("%s\x00%d\x00%d\x00%s\x00%s\x00%s", path, li.Size(), li.ModTime().UnixNano(), svc, root, plansSig)
			if g, ok := learnGateCache.get(key, time.Now()); ok {
				return g
			}
		}
	}
	g := readVerifyLedgerFor(path, svc, root)
	if key != "" && g.runs.State != console.StateError {
		learnGateCache.put(key, g, time.Time{})
	}
	return g
}

// learnReadable reports whether a file or directory can be read now — or
// does not exist, which its reader reports as such. A cached read is used
// only while its source is readable: the wedge makes stat succeed and
// open fail, and an unreadable source is an error now, not an old count.
func learnReadable(p string) bool {
	fh, err := os.Open(p)
	if err != nil {
		return errors.Is(err, os.ErrNotExist)
	}
	defer fh.Close()
	if info, err := fh.Stat(); err == nil && info.IsDir() {
		_, err = fh.ReadDir(1)
		return err == nil || errors.Is(err, io.EOF)
	}
	return true
}

// readVerifyLedgerFor is the gate's history on a project's current plans —
// the plans `fathom plans status --service S` reads (docs/plans/*.md under
// the recorded root, README excluded): the verify runs recorded on them,
// how many of them the gate has run on, and, over each plan's latest run,
// its claimed rows and the verified ones. A plan is its repository-relative
// path, whichever checkout ran it — the root itself, a worktree, the
// pre-push gate's fresh clone — and however the row named it (relative to
// its repo_root, or absolute), so one plan is counted once. A row naming
// anything else — a scratch plan under /tmp, a plan since deleted — is not
// a run on this project's plans: it is named in the denominator as left
// out, never counted as a plan gated.
//
// No ledger is absent (the gate has not run on this machine), an unreadable
// ledger or plans directory an error; a readable ledger with no run on the
// project's plans is a measured zero.
func readVerifyLedgerFor(path, svc, root string) learnGate {
	source := "verify ledger " + path
	absent := func(reason string) learnGate {
		a := console.Absent[int](reason).WithSource(source)
		return learnGate{a, a, a, a}
	}
	errored := func(reason string) learnGate {
		e := console.Errored[int](reason).WithSource(source)
		return learnGate{e, e, e, e}
	}
	if path == "" {
		return absent("no verify ledger path could be resolved")
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return absent("no verify ledger at " + path + " — the gate has not run on this machine")
		}
		return errored("verify ledger unreadable: " + err.Error())
	}
	rows, rerr := verifyLedgerRows(path, info)
	if rerr != nil {
		return errored("verify ledger unreadable: " + rerr.Error())
	}
	if root == "" {
		return absent("no corpus root is recorded for " + svc + ", so the gate's runs cannot be matched to its plans")
	}
	plans, perr := currentPlanPaths(root)
	if perr != nil {
		return errored("the project's plans could not be listed: " + perr.Error())
	}
	type latest struct {
		at                string
		verified, claimed int
	}
	byPlan := map[string]latest{}
	resolved := map[[2]string]string{}
	runs, elsewhere, forSvc := 0, 0, 0
	for _, r := range rows {
		if r.Service != svc {
			continue
		}
		forSvc++
		k := [2]string{r.RepoRoot, r.Plan}
		plan, seen := resolved[k]
		if !seen {
			plan = ledgerPlanPath(r, root)
			resolved[k] = plan
		}
		if !plans[plan] {
			elsewhere++
			continue
		}
		runs++
		if cur, ok := byPlan[plan]; !ok || r.At >= cur.at {
			byPlan[plan] = latest{at: r.At, verified: r.Verified, claimed: r.Claimed}
		}
	}
	plansDir := filepath.ToSlash(filepath.Join("docs", "plans"))
	left := ""
	if elsewhere > 0 {
		verb := "name"
		if elsewhere == 1 {
			verb = "names"
		}
		left = fmt.Sprintf("; %s %s no current plan (a scratch copy, a deleted plan) and are not counted", fmtThousands(elsewhere), verb)
	}
	g := learnGate{
		runs: console.Measured(runs, source).WithDenominator(fmt.Sprintf("gate runs on %s's %s in %s, from any checkout, of %s for %s in the ledger%s",
			svc, plural(len(plans), "current plan"), plansDir, plural(forSvc, "row"), svc, left)),
		plans: console.Measured(len(byPlan), source).WithDenominator(fmt.Sprintf("of the %s in %s the gate has run on", plural(len(plans), "plan"), plansDir)),
	}
	if runs == 0 {
		a := console.Absent[int]("the gate has never run on " + svc + "'s current plans on this machine").WithSource(source)
		g.verified, g.claimed = a, a
		return g
	}
	v, c := 0, 0
	for _, l := range byPlan {
		v += l.verified
		c += l.claimed
	}
	denom := "summed over each current plan's latest gate run for " + svc
	g.verified = console.Measured(v, source).WithDenominator(denom)
	g.claimed = console.Measured(c, source).WithDenominator(denom)
	return g
}

// currentPlanPaths is a project's current plans — docs/plans/*.md under
// root, README excluded, as activePlanFiles lists them — by repository-
// relative path. A missing directory is no plans; an unreadable one is an
// error (a glob would swallow it).
func currentPlanPaths(root string) (map[string]bool, error) {
	//fathom:tenancy-allow whole-read lists one directory's entries (names only), no file content
	entries, err := os.ReadDir(filepath.Join(root, "docs", "plans"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	out := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" && !strings.EqualFold(e.Name(), "README.md") {
			out[filepath.ToSlash(filepath.Join("docs", "plans", e.Name()))] = true
		}
	}
	return out, nil
}

// ledgerPlanPath is the repository-relative path of the plan a ledger row
// names ("" when it names none): a relative plan is already relative to the
// checkout it ran in (the row's repo_root); an absolute one is made
// relative to that checkout, else to the project's root — lexically, then
// with symlinks resolved (a symlinked home or checkout path is the same
// place).
func ledgerPlanPath(r verifyLedgerRow, root string) string {
	p := r.Plan
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		return filepath.ToSlash(filepath.Clean(p))
	}
	for _, base := range []string{r.RepoRoot, root} {
		if rel, ok := relInside(base, p); ok {
			return rel
		}
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return ""
	}
	for _, base := range []string{r.RepoRoot, root} {
		if base == "" {
			continue
		}
		if canon, err := filepath.EvalSymlinks(base); err == nil {
			if rel, ok := relInside(canon, filepath.Join(dir, filepath.Base(p))); ok {
				return rel
			}
		}
	}
	return ""
}

// relInside is p relative to base when p lies inside base.
func relInside(base, p string) (string, bool) {
	if base == "" {
		return "", false
	}
	rel, err := filepath.Rel(base, p)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// verifyLedgerRows parses the ledger, reusing the last parse while its size
// and modification time are unchanged.
func verifyLedgerRows(path string, info os.FileInfo) ([]verifyLedgerRow, error) {
	// Opened on every read: a ledger that cannot be read now is an error
	// now, never the rows of an earlier read.
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	c := &verifyLedgerCache
	c.Lock()
	defer c.Unlock()
	if c.path == path && c.size == info.Size() && c.mod.Equal(info.ModTime()) {
		return c.rows, c.readErr
	}
	var rows []verifyLedgerRow
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r verifyLedgerRow
		if json.Unmarshal([]byte(line), &r) == nil {
			rows = append(rows, r)
		}
	}
	if err := sc.Err(); err != nil {
		// A failed read is never kept: the next request reads again.
		return nil, err
	}
	c.path, c.size, c.mod, c.rows, c.readErr = path, info.Size(), info.ModTime(), rows, nil
	return c.rows, nil
}

// ─── rendering ──────────────────────────────────────────────────────────────

// resolveConsoleHref picks the first page a route list names that the
// console serves — "/packs" until a wave registers it falls through to the
// project workspace's tab, and that tab until the workspace declares it
// (consolePage.Tabs) to the workspace itself — and returns it as a link.
func resolveConsoleHref(reg *consoleRegistry, pages []string) (href, title string, ok bool) {
	for _, p := range pages {
		path, query, _ := strings.Cut(p, "?")
		tab := ""
		if q, err := url.ParseQuery(query); err == nil {
			tab = q.Get("tab")
		}
		for _, page := range reg.pages {
			if routeMatches(page.Route, path) && (tab == "" || containsString(page.Tabs, tab)) {
				return p, page.Title, true
			}
		}
	}
	return "", "", false
}

// routeMatches reports whether a concrete path is served by a ServeMux
// pattern ("/projects/{svc}" matches "/projects/fathom").
func routeMatches(pattern, path string) bool {
	if pattern == "/{$}" {
		return path == "/"
	}
	ps, xs := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(ps) != len(xs) {
		return false
	}
	for i := range ps {
		if strings.HasPrefix(ps[i], "{") && strings.HasSuffix(ps[i], "}") {
			if xs[i] == "" {
				return false
			}
			continue
		}
		if ps[i] != xs[i] {
			return false
		}
	}
	return true
}

// stageLinkLabel names a stage's destination for its link: the page it
// opens, and the tab only when the link is to one.
func stageLinkLabel(href, title string, choice learnChoice) string {
	path, query, _ := strings.Cut(href, "?")
	if strings.HasPrefix(path, "/projects/") {
		if q, err := url.ParseQuery(query); err == nil && q.Get("tab") != "" {
			return "Open " + choice.Service + " — " + q.Get("tab") + " tab"
		}
		// A workspace tab is its own route (/projects/{svc}/<tab>).
		if parts := strings.Split(strings.Trim(path, "/"), "/"); len(parts) == 3 {
			return "Open " + choice.Service + " — " + parts[2] + " tab"
		}
		return "Open " + choice.Service + "'s project page"
	}
	return "Open " + title
}

// pipelineNodeLabel is a headline value as the pipeline draws it: the
// number, "≥ n" for a lower bound, "—" for not measured, "unreadable".
func pipelineNodeLabel(v console.Value[int]) string {
	switch v.State {
	case console.StateAbsent:
		return "—"
	case console.StateError:
		return "unreadable"
	case console.StatePartial:
		return "≥ " + fmtThousands(v.V)
	}
	return fmtThousands(v.V)
}

func renderLearnPipeline(reg *consoleRegistry, choice learnChoice, pipe console.Pipeline, facts learnFacts, m observatoryModel) string {
	var b strings.Builder
	b.WriteString(renderPipelineGraph(reg, choice, pipe, m))

	b.WriteString(`<ol class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2" aria-label="The stages on ` + html.EscapeString(orDash(choice.Service)) + `" data-learn-stages>`)
	for _, st := range pipe.Stages {
		b.WriteString(`<li class="min-w-0">` + renderPipelineStage(reg, choice, st, facts, m) + `</li>`)
	}
	b.WriteString(`</ol>`)

	b.WriteString(`<div class="mt-4">` + renderLearnSwitches(choice, pipe, facts, m) + `</div>`)
	return b.String()
}

// renderPipelineGraph draws the pipeline with the kit's layered graph
// (FATHOM-KIT-02): one column per step, the stage and its headline count in
// its box ("Source: 2,093", the unit beneath), the box linking to the
// stage's page; the kit's own table alternative behind a disclosure. The
// node label carries the stage's name because the table alternative — the
// keyboard and screen-reader path, the drawing's links being pointer-only —
// names each node, and each hand-off, by that label alone.
func renderPipelineGraph(reg *consoleRegistry, choice learnChoice, pipe console.Pipeline, m observatoryModel) string {
	nodes := make([]viz.GraphNode, 0, len(pipe.Stages))
	labels := make([]string, 0, len(pipe.Stages))
	for _, st := range pipe.Stages {
		href, _, _ := resolveConsoleHref(reg, st.Pages)
		nodes = append(nodes, viz.GraphNode{
			ID: string(st.ID), Label: st.Title + ": " + pipelineNodeLabel(st.Headline.Value), Group: st.Headline.Label,
			Rank: st.Step - 1, Href: href,
		})
		labels = append(labels, fmt.Sprintf("Step %d", st.Step))
	}
	edges := make([]viz.GraphEdge, 0, len(pipe.Edges))
	for _, e := range pipe.Edges {
		edges = append(edges, viz.GraphEdge{From: string(e.From), To: string(e.To)})
	}
	subject := orDash(choice.Service)
	layout, err := viz.LayoutGraph(nodes, edges, viz.GraphOptions{
		AriaLabel:   "How fathom works on " + subject,
		Description: "Seven stages from the project's source to its evidence, each with its live count.",
		RankMode:    viz.GraphRankGiven,
		RankLabels:  labels,
		SolidLabel:  "hands off to",
	})
	var body string
	if err != nil {
		body = `<p class="text-danger" data-truth-line="error">The pipeline could not be drawn: ` + html.EscapeString(err.Error()) + `</p>`
	} else {
		body = `<div class="overflow-x-auto rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="The pipeline drawing (scrolls sideways on a narrow screen)" data-learn-graph>` + layout.SVG() + `</div>` +
			`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{
			Summary: "The pipeline as a table",
			Body:    template.HTML(`<div class="overflow-x-auto [&_a]:inline-flex [&_a]:min-h-11 [&_a]:items-center [&_a]:text-accent [&_a]:underline-offset-4 hover:[&_a]:underline [&_caption]:py-1 [&_caption]:text-left [&_caption]:font-medium [&_table]:mb-3 [&_table]:w-full [&_td]:border-t [&_td]:border-line [&_td]:px-2 [&_td]:py-1 [&_th]:px-2 [&_th]:py-1 [&_th]:text-left [&_th]:text-ink-muted" data-learn-graph-table>` + layout.Table() + `</div>`),
		}) + `</div>`
	}
	return renderPanel(consolePanel{
		ID: "pipeline", Title: "From source to evidence, on " + subject,
		Question:    "What happens between this project's source and the evidence fathom keeps, and how much of it is happening here?",
		Source:      "each stage's own instrument, named on its panel below",
		Denominator: "one project: " + subject,
		Freshness:   "as of " + m.GeneratedAt,
		SourceKey:   pipelineLiveKeys(pipe),
		Body:        body,
	})
}

// pipelineLiveKeys is every source the pipeline's numbers move with.
func pipelineLiveKeys(pipe console.Pipeline) string {
	seen := map[string]bool{}
	var keys []string
	for _, st := range pipe.Stages {
		if k := liveKeyIfValid(st.Live); k != "" && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	return strings.Join(keys, " ")
}

// stageVerb is the exact CLI command that prints a stage's headline for
// the project, or "" when none does and the footer names the instrument:
// the source stage's files are the store's source refs, which no verb
// prints (the fleet report carries its commits and freshness, offered in
// the body); compile's current claims are printed by the agent surface's
// staleness_report, a tool call rather than a command (offered in the body,
// labelled as a call); the gate's headline is the verify ledger's (the body
// offers `fathom plans status` to prove it now).
func stageVerb(st console.PipelineStage, choice learnChoice, facts learnFacts) string {
	svc := shellWord(choice.Service)
	switch st.ID {
	case console.StageExtract, console.StageEvidence:
		return "fathom fleet report --json"
	case console.StageIndex:
		return "fathom report evidence --service " + svc + " --json"
	case console.StageServe:
		return facts.traffic.Verb
	}
	return ""
}

func renderPipelineStage(reg *consoleRegistry, choice learnChoice, st console.PipelineStage, facts learnFacts, m observatoryModel) string {
	var body strings.Builder
	body.WriteString(`<p class="mb-3 text-ink">` + html.EscapeString(st.Does) + `</p>`)
	body.WriteString(stageStatusLine(st))

	items := []chrome.DescItem{{Term: st.Headline.Label, DetailHTML: template.HTML(valueText(st.Headline.Value))}}
	for _, c := range st.Counts {
		items = append(items, chrome.DescItem{Term: c.Label, DetailHTML: template.HTML(valueText(c.Value))})
	}
	body.WriteString(`<div class="[&_dl]:grid-cols-1 sm:[&_dl]:grid-cols-[max-content_1fr] [&_dd]:[overflow-wrap:anywhere]">` + chrome.RenderDescriptionList(items) + `</div>`)
	if len(st.Notes) > 0 {
		body.WriteString(`<ul class="mt-2 list-disc space-y-1 pl-5 text-ink-muted [overflow-wrap:anywhere]" data-stage-notes>`)
		for _, n := range st.Notes {
			body.WriteString(`<li>` + html.EscapeString(n) + `</li>`)
		}
		body.WriteString(`</ul>`)
	}
	if len(st.Capabilities) > 0 {
		body.WriteString(`<ul class="mt-3 space-y-1" aria-label="Switches that deliver this stage" data-stage-switches>`)
		for _, c := range st.Capabilities {
			body.WriteString(`<li class="[overflow-wrap:anywhere]">` + stageSwitchLine(c) + `</li>`)
		}
		body.WriteString(`</ul>`)
	}
	switch st.ID {
	case console.StageSource:
		body.WriteString(`<p class="mt-3 text-ink-muted">The indexed commit, HEAD and freshness: ` + renderVerbChip("fathom fleet report --json") + `</p>`)
	case console.StageCompile:
		body.WriteString(`<p class="mt-3 text-ink-muted">Each layer's claims by status, from the agent surface: ` + renderVerbChip(console.AskCall("staleness_report", map[string]any{"service": choice.Service})) + `</p>`)
	case console.StageGate:
		body.WriteString(`<p class="mt-3 text-ink-muted">Prove it now: ` + renderVerbChip("fathom plans status --service "+shellWord(choice.Service)) + `</p>`)
	}
	if href, title, ok := resolveConsoleHref(reg, st.Pages); ok {
		body.WriteString(`<p class="mt-3"><a href="` + html.EscapeString(href) + `" class="inline-flex min-h-11 items-center font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" data-stage-link>` + html.EscapeString(stageLinkLabel(href, title, choice)) + ` →</a></p>`)
	} else if len(st.Pages) > 0 {
		body.WriteString(`<p class="mt-3 italic text-ink-muted [overflow-wrap:anywhere]" data-stage-nopage>No console page shows this stage in full yet (` + html.EscapeString(st.Pages[0]) + ` is planned).</p>`)
	}

	source := st.Headline.Value.Source
	if source == "" {
		source = "not read"
	}
	return renderPanel(consolePanel{
		ID:       "stage-" + string(st.ID),
		Title:    fmt.Sprintf("%d · %s — %s", st.Step, st.Title, st.Pieces),
		Question: st.Question, Verb: stageVerb(st, choice, facts),
		Source: source, Denominator: st.Headline.Value.Denominator,
		Freshness: "as of " + m.GeneratedAt,
		State:     st.Headline.Value.State, Reason: st.Headline.Value.Reason,
		SourceKey: liveKeyIfValid(st.Live),
		Body:      body.String(),
	})
}

// liveKeyIfValid keeps a project's live key when its project part is
// name-shaped, and falls back to the fleet-wide key ("index", "sessions")
// when there is no such project — every panel follows something an event
// can name.
func liveKeyIfValid(key string) string {
	fleet, svc, ok := strings.Cut(key, ":")
	if !ok {
		return key
	}
	if svc == "" || !console.IsNameOrID(svc) {
		return fleet
	}
	return key
}

// stageStatusLine says whether the stage works here, in words and with a
// distinct treatment per status; color is never the only channel.
func stageStatusLine(st console.PipelineStage) string {
	var cls, word string
	switch st.Status {
	case console.StageOperating:
		cls, word = "text-positive", "Operating here"
	case console.StageWorking:
		cls, word = "text-ink", "Working here"
	case console.StageOff:
		cls, word = "font-medium text-danger", "Off here"
	case console.StageStale:
		cls, word = "text-caution", "Stale here"
	default:
		cls, word = "italic text-ink-muted", "Unknown here"
	}
	line := `<p class="mb-3 ` + cls + ` [overflow-wrap:anywhere]" data-stage-status="` + html.EscapeString(string(st.Status)) + `">` + word
	if st.StatusReason != "" {
		line += ` — ` + html.EscapeString(st.StatusReason)
	}
	line += `</p>`
	switch {
	case strings.HasPrefix(st.Remedy, "fathom "):
		line += `<p class="-mt-2 mb-3 text-ink-muted">Remedy: ` + renderVerbChip(st.Remedy) + `</p>`
	case st.Remedy != "":
		// A remedy that is a note ("nothing to run — …") is not a command.
		line += `<p class="-mt-2 mb-3 text-ink-muted [overflow-wrap:anywhere]">Remedy: ` + html.EscapeString(st.Remedy) + `</p>`
	}
	return line
}

// stageSwitchLine is one switch: on or off in words, why, and whether the
// stage needs it.
func stageSwitchLine(c console.CapabilityState) string {
	need := "optional"
	if c.Required {
		need = "required"
	}
	if c.Operating {
		detail := ""
		if c.Detail != "" {
			detail = ` — ` + html.EscapeString(c.Detail)
		}
		return `<span class="font-mono text-xs text-ink">` + html.EscapeString(c.ID) + `</span> <span class="text-positive">on</span> <span class="text-ink-muted">(` + need + `)` + detail + `</span>`
	}
	// Absent (its inputs do not exist here) and broken (they exist and fail)
	// are different to act on, so they read differently — in words first.
	state := `<span class="font-medium text-ink-muted">off</span>`
	if c.Broken {
		state = `<span class="font-medium text-caution">broken</span>`
	}
	return `<span class="font-mono text-xs text-ink">` + html.EscapeString(c.ID) + `</span> ` + state + ` <span class="text-ink-muted">(` + need + `) — ` + html.EscapeString(c.Reason) + `</span>`
}

// renderLearnSwitches is the project's `fathom capabilities` as a table —
// every switch, the stage it delivers, and what turns an off one on.
func renderLearnSwitches(choice learnChoice, pipe console.Pipeline, facts learnFacts, m observatoryModel) string {
	stageOf := map[string]string{}
	for _, st := range pipe.Stages {
		for _, c := range st.Capabilities {
			if _, ok := stageOf[c.ID]; !ok {
				stageOf[c.ID] = fmt.Sprintf("%d · %s", st.Step, st.Title)
			}
		}
	}
	p := consolePanel{
		ID: "switches", Title: "Switched on here",
		Question:  "Which of fathom's switches are operating for this project, and what turns an off one on?",
		Verb:      facts.capVerb,
		Freshness: "as of " + m.GeneratedAt,
		SourceKey: liveKeyIfValid("index:" + choice.Service),
	}
	if facts.capVerb == "" {
		p.State, p.Reason = console.StateAbsent, facts.pipeline.CapabilitiesNote
		p.Source = "the capability probe (not run)"
		return renderPanel(p)
	}
	cm := facts.capModel
	p.Source = "the capability probe of " + cm.RepoRoot + " (internal/capability)"
	p.Denominator = fmt.Sprintf("%d of %d switches operating", cm.Operating, cm.Total)
	rows := make([]table.Row, 0, len(cm.Capabilities))
	for _, c := range cm.Capabilities {
		state := `<span class="text-positive">on</span>`
		why := c.Detail
		if !c.Operating {
			state = `<span class="font-medium text-caution">off</span>`
			why = c.Reason
			if c.Cmd != "" {
				why += " — run: " + c.Cmd
			} else if c.Note != "" {
				why += " — " + c.Note
			}
		}
		stage := stageOf[c.ID]
		if stage == "" {
			stage = "—"
		}
		rows = append(rows, table.Row{
			ID:        "cap-" + c.ID,
			Cells:     []string{c.ID, stage, "", why},
			HTMLCells: []templ.Component{templ.Raw(`<span class="font-mono text-xs">` + html.EscapeString(c.ID) + `</span>`), nil, templ.Raw(state), templ.Raw(`<span class="[overflow-wrap:anywhere]">` + html.EscapeString(why) + `</span>`)},
		})
	}
	p.Body = renderKitTable(table.TableParams{
		TableID: "learn-switches",
		Caption: fmt.Sprintf("%s's switches, from fathom capabilities", orDash(choice.Service)),
		Columns: []table.Column{{Key: "id", Label: "Switch"}, {Key: "stage", Label: "Stage"}, {Key: "state", Label: "State"}, {Key: "why", Label: "Evidence, or what turns it on"}},
		Rows:    rows,
	})
	return renderPanel(p)
}
