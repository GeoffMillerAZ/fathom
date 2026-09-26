package main

// console_projects.go — the one read of every source the Now and Fleet
// pages share, and that `fathom brief --json` renders (vision 35 §3, plan
// section B). It gathers console.Projects — one row per registered project
// — from:
//
//   - the observatory model: the registry, freshness, wiring, the
//     realization ladder, open observations and the call log's per-day
//     traffic (one fold, shared with every model-backed page);
//   - the verify ledger: the newest run of each plan at the project's
//     registered root;
//   - the daemon's store: claims by status, certify status and the
//     obligations layer's latest compile;
//   - the daemon's /statusz: the typed call-edge pass.
//
// Inside the daemon the store is the daemon's own; a CLI verb opens the
// store the daemon at --url recorded (its daemon record), read-only — the
// same file, so the page and the verb read the same facts (P10).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// projectSources names the readers one gather uses.
type projectSources struct {
	baseURL string // the daemon (its /statusz)
	model   observatoryModel
	// st is the daemon's store, nil when none was resolved (storeRead then
	// says why); storePath names it in remedies.
	st        store.Store
	storeRead console.SourceRead
	storePath string
	now       time.Time
}

// projectsRead is one gather: the project set plus the figures beside it.
type projectsRead struct {
	projects     console.Projects
	gateRefusals console.Value[int]
}

// consolePlanExists reports whether a plan file is still on disk under a
// project root (an archived plan's last verify run is history). A variable
// so the page tests can point runs at plans the fixture repos lack.
var consolePlanExists = func(root, plan string) bool {
	if filepath.IsAbs(plan) {
		_, err := os.Stat(plan)
		return err == nil
	}
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(plan)))
	return err == nil
}

// gatherProjects reads every source once.
func gatherProjects(ctx context.Context, src projectSources) projectsRead {
	m := src.model
	p := console.Projects{Now: src.now, RegistryNote: m.DegradedNote, CallLogNote: m.CallLogNote, Store: src.storeRead}
	for _, d := range m.TrafficDays {
		p.Days = append(p.Days, d.Date)
		p.Logged = append(p.Logged, d.Calls > 0)
	}
	if len(m.TrafficDays) == 0 && p.CallLogNote == "" {
		switch {
		case m.DegradedNote != "":
			p.CallLogNote = "call log not read: " + m.DegradedNote
		case m.callFacts.AbsentReason != "":
			p.CallLogNote = m.callFacts.AbsentReason
		}
	}

	// The verify ledger: every run, then the newest per plan at each root.
	ledgerPath := verifyLedgerPath()
	var runs []console.VerifyRun
	p.Ledger = console.SourceRead{Name: console.SourceVerifyLedger, Path: ledgerPath}
	switch data, err := os.ReadFile(ledgerPath); {
	case ledgerPath == "":
		p.Ledger.State, p.Ledger.Reason = console.StateAbsent, "no home directory to find the verify ledger in"
	case os.IsNotExist(err):
		p.Ledger.State, p.Ledger.Reason = console.StateAbsent, "no verify ledger at "+ledgerPath+" — no `fathom verify` run is recorded"
	case err != nil:
		p.Ledger.State, p.Ledger.Reason = console.StateError, err.Error()+tccWedgeHint(ledgerPath)
	default:
		var malformed int
		runs, malformed = console.ParseVerifyLedger(data)
		p.Ledger.State = console.StatePresent
		if malformed > 0 {
			p.Ledger.Reason = fmt.Sprintf("%d malformed rows skipped", malformed)
		}
	}
	gate := gateRefusalFigure(runs, p.Ledger)
	sameRoot := sameRootMatcher()

	typedFailed := statuszTypedFailures(src.baseURL)
	for _, s := range m.Services {
		row := console.ProjectFacts{
			Service: s.Service, Path: s.Path, SymbolsNote: s.SymbolsNote, Languages: s.languages,
			Freshness: s.Freshness, IndexedCommit: s.IndexedCommit, RepoHead: s.RepoHead,
			RefreshRemedy: daemonRefreshRemedy(s.Path, s.Service), TypedEdgesFailed: typedFailed[strings.ToLower(s.Service)],
			OpenObservations: s.OpenObservations, ObservationsErr: s.ObservationsError,
			HasPlans: repoHasPlans(s.Path), HasGuide: repoHasGuide(s.Path),
		}
		if s.SymbolsNote == "" {
			n := s.Symbols
			row.Symbols = &n
		}
		if s.Realization != nil {
			row.Grade = s.Realization.Grade
			row.Realized, row.Available, row.Sessions = s.Realization.Realized, s.Realization.Available, s.Realization.Sessions
			for _, d := range s.Realization.Dims {
				row.Rungs = append(row.Rungs, rungOf(d))
			}
		}
		if p.CallLogNote == "" {
			row.Traffic = s.TrafficDays
			if row.Traffic == nil {
				row.Traffic = make([]int, len(p.Days)) // read, and nothing for this project: measured zeros
			}
		}
		if s.Path != "" && p.Ledger.State == console.StatePresent {
			root := s.Path
			row.Runs = console.NewestRunsAt(runs, root, func(recorded string) bool { return sameRoot(recorded, root) },
				func(plan string) bool { return consolePlanExists(root, plan) })
		}
		row.CertRemedy = "fathom certify gaps --service " + s.Service
		if src.storePath != "" {
			row.CertRemedy += " --store " + src.storePath
		}
		storeFacts(ctx, src, &row)
		p.Rows = append(p.Rows, row)
	}
	return projectsRead{projects: p, gateRefusals: gate}
}

// storeFacts reads one project's claims, certificates and latest
// obligations compile from the store.
func storeFacts(ctx context.Context, src projectSources, row *console.ProjectFacts) {
	if src.st == nil {
		reason := console.NonEmptyReason(src.storeRead.Reason, "no store was resolved")
		row.Claim = console.Absent[console.ClaimCounts](reason)
		row.Certs = console.Absent[console.CertCounts](reason)
		row.Guide = console.Absent[console.GuideCoverage](reason)
		return
	}
	const source = "the daemon's store"
	if claims, err := src.st.ClaimsForService(ctx, row.Service, ""); err != nil {
		row.Claim = console.Errored[console.ClaimCounts]("claims unreadable: " + err.Error())
	} else {
		var c console.ClaimCounts
		for _, cl := range claims {
			c.Total++
			switch cl.Status {
			case core.ClaimFresh:
				c.Fresh++
			case core.ClaimStale:
				c.Stale++
			case core.ClaimSuperseded:
				c.Superseded++
			default:
				c.Other++
			}
		}
		row.Claim = console.Present(c, source)
	}
	if gaps, err := certify.Gaps(ctx, src.st, row.Service, row.Path, nil); err != nil {
		row.Certs = console.Errored[console.CertCounts]("certify status failed: " + err.Error())
	} else {
		row.Certs = console.Present(console.CertCounts{Declared: gaps.Summary.Declared, Live: gaps.Summary.Live, Gaps: gaps.Summary.Gaps}, source)
	}
	compileRuns, err := src.st.CompileRuns(ctx, row.Service, semantic.LayerObligations)
	if err != nil {
		row.Guide = console.Errored[console.GuideCoverage]("compile runs unreadable: " + err.Error())
		return
	}
	for _, run := range compileRuns { // newest first
		if run.DryRun || run.Refused {
			continue // an estimate or a refusal compiled nothing
		}
		cov := console.GuideCoverage{SectionsTotal: run.SectionsTotal, BudgetStopped: run.BudgetStopped, RanAt: run.RanAt.UTC().Format(time.RFC3339)}
		for _, doc := range run.UncompiledScopes {
			if isGoverningGuide(doc) {
				cov.Uncompiled = append(cov.Uncompiled, doc)
			}
		}
		row.Guide = console.Present(cov, source)
		return
	}
	row.Guide = console.Absent[console.GuideCoverage]("no obligations compile run is recorded for " + row.Service)
}

// isGoverningGuide: an instruction file the obligations layer compiles.
func isGoverningGuide(doc string) bool {
	switch path.Base(doc) {
	case "CLAUDE.md", "AGENTS.md":
		return true
	}
	return false
}

// repoHasGuide reports whether a project root carries a governing guide.
func repoHasGuide(root string) bool {
	if root == "" {
		return false
	}
	for _, f := range []string{"CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			return true
		}
	}
	return false
}

// rungOf maps one realization dimension onto the console's rung: a rung
// that does not apply (no plans to gate) is "not applicable"; any other
// rung nobody could decide is unknown — never met.
func rungOf(d realizationDim) console.Rung {
	r := console.Rung{Name: d.Name, Evidence: d.Evidence}
	switch {
	case d.Met == nil && d.Name == "gating":
		r.State = console.RungNA
	case d.Met == nil:
		r.State = console.RungUnknown
	case *d.Met:
		r.State = console.RungMet
	default:
		r.State = console.RungUnmet
	}
	return r
}

// gateRefusalFigure is PC-13's one verify-gate figure (verifyFigure) over
// the ledger read above, as a projected value.
func gateRefusalFigure(runs []console.VerifyRun, ledger console.SourceRead) console.Value[int] {
	const source = "the verify ledger (every recorded `fathom verify` run)"
	switch ledger.State {
	case console.StateError:
		return console.Errored[int]("verify ledger unreadable: " + ledger.Reason).WithSource(source)
	case console.StateAbsent:
		return console.Absent[int](ledger.Reason).WithSource(source)
	}
	rows := make([]verifyLedgerRow, 0, len(runs))
	for _, r := range runs {
		rows = append(rows, verifyLedgerRow{At: r.At, RepoRoot: r.RepoRoot, Plan: r.Plan, Failing: r.Failing})
	}
	f := verifyFigure(rows)
	if f.Runs == 0 {
		return console.Absent[int](f.Sentence()).WithSource(source)
	}
	return console.Measured(f.Refusals, source).WithDenominator(fmt.Sprintf("of %s between %s and %s", plural(f.Runs, "recorded verify run"), dayOnly(f.First), dayOnly(f.Last)))
}

func dayOnly(ts string) string {
	if i := strings.IndexByte(ts, 'T'); i > 0 {
		return ts[:i]
	}
	return ts
}

// sameRootMatcher compares a run's recorded root with a project root by
// their resolved paths (~/code is a symlink to the external volume), each
// resolution done once.
func sameRootMatcher() func(recorded, root string) bool {
	resolved := map[string]string{}
	canon := func(p string) string {
		if c, ok := resolved[p]; ok {
			return c
		}
		c := filepath.Clean(p)
		if r, err := filepath.EvalSymlinks(c); err == nil {
			c = r
		}
		resolved[p] = c
		return c
	}
	return func(recorded, root string) bool {
		if recorded == "" || root == "" {
			return false
		}
		return recorded == root || canon(recorded) == canon(root)
	}
}

// statuszTypedFailures reads the daemon's /statusz once: each service
// whose typed call-edge pass failed, with the reason.
func statuszTypedFailures(baseURL string) map[string]string {
	out := map[string]string{}
	if baseURL == "" {
		return out
	}
	resp, err := consoleRPCClient(5 * time.Second).Get(baseURL + "/statusz?format=json")
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out
	}
	var rep struct {
		Services []struct {
			Service    string             `json:"service"`
			TypedEdges *StatuszTypedEdges `json:"typed_edges"`
		} `json:"services"`
	}
	if json.NewDecoder(resp.Body).Decode(&rep) != nil {
		return out
	}
	for _, s := range rep.Services {
		if s.TypedEdges != nil && s.TypedEdges.Failed {
			out[strings.ToLower(s.Service)] = console.NonEmptyReason(s.TypedEdges.Reason, "the pass failed without a reason")
		}
	}
	return out
}

// splitLangs splits the registry's "go/python" language list.
func splitLangs(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "/")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// ─── the daemon's cache ─────────────────────────────────────────────────

// consoleProjectsCache keeps one gather per observatory model build (the
// model is cached for 30 s) and verify-ledger state, so a page render never
// re-reads the store or the ledger while neither moved.
var consoleProjectsCache struct {
	sync.Mutex
	key  string
	read projectsRead
}

// cachedProjects is the request's project set.
func (cr *consoleRequest) projects() projectsRead {
	m := cr.model()
	key := fmt.Sprintf("%s|%p|%s|%s", cr.deps.selfURL, m.Services, m.GeneratedAt, fileStamp(verifyLedgerPath()))
	consoleProjectsCache.Lock()
	defer consoleProjectsCache.Unlock()
	if consoleProjectsCache.key == key {
		return consoleProjectsCache.read
	}
	src := projectSources{baseURL: cr.deps.selfURL, model: m, now: cr.now}
	src.st, src.storeRead, src.storePath = daemonStore(cr.deps)
	read := gatherProjects(cr.r.Context(), src)
	consoleProjectsCache.key, consoleProjectsCache.read = key, read
	return read
}

// daemonStore is this daemon's own store: open already, named by the
// record this daemon wrote.
func daemonStore(d *consoleDeps) (store.Store, console.SourceRead, string) {
	read := console.SourceRead{Name: console.SourceStore}
	if d.c == nil {
		read.State, read.Reason = console.StateAbsent, "this daemon has no store open"
		return nil, read, ""
	}
	storePath := ""
	if rec, ok := readDaemonRecordFor(d.selfURL); ok {
		storePath = rec.Store
	}
	read.State, read.Path = console.StatePresent, storePath
	return d.c.Store(), read, storePath
}

// verbStore resolves the store a CLI verb reads: --store, else the store
// the daemon at baseURL recorded; opened genuinely read-only. The closer is
// never nil.
func verbStore(flag, baseURL string) (store.Store, console.SourceRead, string, func()) {
	read := console.SourceRead{Name: console.SourceStore}
	p := flag
	if p == "" {
		if rec, ok := readDaemonRecordFor(baseURL); ok {
			p = rec.Store
		}
	}
	if p == "" {
		read.State, read.Reason = console.StateAbsent, "the daemon at "+baseURL+" recorded no store, and no --store was given"
		return nil, read, "", func() {}
	}
	read.Path = p
	st, err := store.MountReadOnly(p, store.LevelL0)
	if err != nil {
		read.State, read.Reason = console.StateError, err.Error()+tccWedgeHint(p)
		return nil, read, p, func() {}
	}
	read.State = console.StatePresent
	return st, read, p, func() { _ = st.Close() }
}

// fileStamp is a file's size and modification time, "" when absent.
func fileStamp(p string) string {
	fi, err := os.Stat(p)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d@%d", fi.Size(), fi.ModTime().UnixNano())
}
