package main

// console_projects.go — the one read of every source the Now and Fleet
// pages share, and that `fathom brief --json` renders (vision 35 §3, plan
// section B). It gathers console.Projects — one row per registered project
// — from:
//
//   - the observatory model: the registry, freshness, wiring, the
//     realization ladder, open observations and the call log's per-day
//     traffic (one fold, shared with every model-backed page);
//   - the verify ledger: each plan's run at the project's HEAD — at its
//     registered root or in any clone of the same service (the pre-push
//     gate verifies in a detached clone) — else the root's newest run,
//     stale (console.PlanRuns);
//   - the daemon's store: claims by status, certify status and the
//     obligations layer's latest compile;
//   - the daemon's /statusz: the typed call-edge pass.
//
// Inside the daemon the store is the daemon's own; a CLI verb opens the
// store the daemon at --url recorded (its daemon record), read-only — the
// same file, so the page and the verb read the same facts (P10).
//
// Inside the daemon the expensive reads are cached by the watermarks of
// their inputs (design §1, Caching): the ledger's parse by its size and
// modification time, each project's store facts by the store file's
// write stamp, the project's HEAD and its intent corpus's stamps, and the
// typed-edge failures by the store stamp. A model rebuild (every 30 s)
// re-folds the project set from those caches without re-reading anything
// that did not move.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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
	// cache, when set, keeps the expensive reads between gathers (the
	// daemon's); a one-shot verb gathers without one.
	cache *projectsCaches
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

	// The verify ledger: every run, then each plan's proof per project.
	ledgerPath := verifyLedgerPath()
	var runs []console.VerifyRun
	runs, p.Ledger = src.cache.ledger(ledgerPath)
	gate := gateRefusalFigure(runs, p.Ledger)
	sameRoot := sameRootMatcher()

	storeStamp := ""
	if src.storePath != "" {
		storeStamp = fileStamp(src.storePath) + "|" + fileStamp(src.storePath+"-wal")
	}
	typedFailed := src.cache.typedFailures(src.baseURL, storeStamp)
	for _, s := range m.Services {
		row := console.ProjectFacts{
			Service: s.Service, Path: s.Path, SymbolsNote: s.SymbolsNote, Languages: s.languages,
			Freshness: s.Freshness, IndexedCommit: s.IndexedCommit, RepoHead: s.RepoHead,
			RefreshRemedy: daemonRefreshRemedy(s.Path, s.Service), TypedEdgesFailed: typedFailed[strings.ToLower(s.Service)],
			OpenObservations: s.OpenObservations, ObservationsErr: s.ObservationsError,
			HasGuide: repoHasGuide(s.Path),
		}
		if files, err := planFilesAt(s.Path); err != nil {
			row.PlansErr = console.NonEmptyReason(errnoOnly(err), "unreadable") + " (docs/plans)"
		} else {
			row.PlanFiles = files
			row.HasPlans = len(files) > 0
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
			headNote := ""
			if s.RepoHead == "" {
				headNote = strings.TrimPrefix(s.Freshness, "unknown: ")
			}
			row.Runs = console.PlanRuns(runs, console.ProjectRunsQuery{
				Service: s.Service, Root: root, Head: s.RepoHead, HeadNote: headNote,
				SameRoot: func(recorded string) bool { return sameRoot(recorded, root) },
				Exists:   func(plan string) bool { return consolePlanExists(root, plan) },
			})
		}
		row.CertRemedy = "fathom certify gaps --service " + s.Service
		if src.storePath != "" {
			row.CertRemedy += " --store " + src.storePath
		}
		src.cache.storeFacts(ctx, src, storeStamp, &row)
		p.Rows = append(p.Rows, row)
	}
	return projectsRead{projects: p, gateRefusals: gate}
}

// readVerifyRuns reads and parses the verify ledger, naming how the read
// went.
func readVerifyRuns(ledgerPath string) ([]console.VerifyRun, console.SourceRead) {
	read := console.SourceRead{Name: console.SourceVerifyLedger, Path: ledgerPath}
	switch data, err := os.ReadFile(ledgerPath); {
	case ledgerPath == "":
		read.State, read.Reason = console.StateAbsent, "no home directory to find the verify ledger in"
	case os.IsNotExist(err):
		read.State, read.Reason = console.StateAbsent, "no verify ledger at "+ledgerPath+" — no `fathom verify` run is recorded"
	case err != nil:
		read.State, read.Reason = console.StateError, err.Error()+tccWedgeHint(ledgerPath)
	default:
		runs, malformed := console.ParseVerifyLedger(data)
		read.State = console.StatePresent
		if malformed > 0 {
			read.Reason = fmt.Sprintf("%d malformed rows skipped", malformed)
		}
		return runs, read
	}
	return nil, read
}

// planFilesAt lists a project's active plans (docs/plans/*.md but its
// README), relative to the root — the set the pre-push gate verifies. A
// missing directory is no plans; one that exists and cannot be read is an
// error, never "no plans".
func planFilesAt(root string) ([]string, error) {
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(filepath.Join(root, "docs", "plans"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.EqualFold(e.Name(), "README.md") {
			continue
		}
		out = append(out, "docs/plans/"+e.Name())
	}
	sort.Strings(out)
	return out, nil
}

// errnoOnly is an error's operating-system wording, its path stripped.
func errnoOnly(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
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
	row.Certs = certCounts(ctx, src.st, row.Service, row.Path)
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

// certCounts is certify status for one project as a projected value: the
// counts when every target source was read; a named lower bound when some
// could not be (an unreadable intent corpus file — the volume's EPERM
// wedge); unknown when nothing was declared and a source could not be
// read — never "declares none" (I38).
func certCounts(ctx context.Context, st store.Store, service, root string) console.Value[console.CertCounts] {
	const source = "the daemon's store"
	gaps, err := certify.Gaps(ctx, st, service, root, nil)
	if err != nil {
		return console.Errored[console.CertCounts]("certify status failed: " + err.Error())
	}
	c := console.CertCounts{Declared: gaps.Summary.Declared, Live: gaps.Summary.Live, Gaps: gaps.Summary.Gaps,
		Cases: gaps.Summary.Cases, JoinedCases: gaps.Summary.JoinedCases, Unreadable: gaps.Unreadable}
	if len(gaps.Unreadable) == 0 {
		return console.Present(c, source)
	}
	unread := boundedList(gaps.Unreadable, 3)
	if c.Declared == 0 {
		// Unknown, not none: the value carries no number (its JSON omits
		// v), but keeps what could not be read for the verb's own shape.
		v := console.Errored[console.CertCounts]("certification targets unreadable: " + unread).WithSource(source)
		v.V = c
		return v
	}
	return console.Partial(c, "targets read in part — unreadable: "+unread+"; a lower bound").WithSource(source)
}

// boundedList names up to max items and how many more.
func boundedList(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, "; ")
	}
	return strings.Join(items[:max], "; ") + fmt.Sprintf("; and %d more", len(items)-max)
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

// ─── the daemon's caches ────────────────────────────────────────────────

// projectsCaches keeps the gather's expensive reads, each keyed by the
// watermarks of its inputs (design §1, Caching), so a model rebuild re-folds
// the project set without re-reading a ledger, a store or a corpus that
// did not move. A nil *projectsCaches reads everything (the verbs).
type projectsCaches struct {
	mu        sync.Mutex
	ledgerKey string
	runs      []console.VerifyRun
	ledgerRd  console.SourceRead
	typedKey  string
	typed     map[string]string
	facts     map[string]storeFactsEntry
}

type storeFactsEntry struct {
	key   string
	claim console.Value[console.ClaimCounts]
	certs console.Value[console.CertCounts]
	guide console.Value[console.GuideCoverage]
}

// consoleGatherCache is the daemon's.
var consoleGatherCache = &projectsCaches{}

// ledger is the verify ledger's parse, re-read only when its size or
// modification time moved.
func (c *projectsCaches) ledger(path string) ([]console.VerifyRun, console.SourceRead) {
	if c == nil {
		return readVerifyRuns(path)
	}
	key := path + "|" + fileStamp(path)
	c.mu.Lock()
	if c.ledgerKey == key && key != path+"|" {
		runs, rd := c.runs, c.ledgerRd
		c.mu.Unlock()
		return runs, rd
	}
	c.mu.Unlock()
	runs, rd := readVerifyRuns(path)
	if rd.State == console.StatePresent {
		c.mu.Lock()
		c.ledgerKey, c.runs, c.ledgerRd = key, runs, rd
		c.mu.Unlock()
	}
	return runs, rd
}

// typedFailures is statuszTypedFailures, re-read only when the store moved.
func (c *projectsCaches) typedFailures(baseURL, storeStamp string) map[string]string {
	if c == nil || storeStamp == "" {
		return statuszTypedFailures(baseURL)
	}
	key := baseURL + "|" + storeStamp
	c.mu.Lock()
	if c.typedKey == key {
		out := c.typed
		c.mu.Unlock()
		return out
	}
	c.mu.Unlock()
	out := statuszTypedFailures(baseURL)
	c.mu.Lock()
	c.typedKey, c.typed = key, out
	c.mu.Unlock()
	return out
}

// storeFacts is the package storeFacts, re-read for a project only when
// the store, the project's HEAD or its intent corpus moved.
func (c *projectsCaches) storeFacts(ctx context.Context, src projectSources, storeStamp string, row *console.ProjectFacts) {
	if c == nil || storeStamp == "" || src.st == nil {
		storeFacts(ctx, src, row)
		return
	}
	key := storeStamp + "|" + row.Path + "|" + row.RepoHead + "|" + intentStamp(row.Path)
	c.mu.Lock()
	if e, ok := c.facts[row.Service]; ok && e.key == key {
		c.mu.Unlock()
		row.Claim, row.Certs, row.Guide = e.claim, e.certs, e.guide
		return
	}
	c.mu.Unlock()
	storeFacts(ctx, src, row)
	c.mu.Lock()
	if c.facts == nil {
		c.facts = map[string]storeFactsEntry{}
	}
	c.facts[row.Service] = storeFactsEntry{key: key, claim: row.Claim, certs: row.Certs, guide: row.Guide}
	c.mu.Unlock()
}

// intentStamp is the stamp of every file certify status reads from a
// project's intent corpus (docs/intent/*.md and one level below), so an
// edited invariant — or a file that turned unreadable — re-reads it.
func intentStamp(root string) string {
	if root == "" {
		return ""
	}
	dir := filepath.Join(root, "docs", "intent")
	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	nested, _ := filepath.Glob(filepath.Join(dir, "*", "*.md"))
	files = append(files, nested...)
	sort.Strings(files)
	var b strings.Builder
	b.WriteString(fileStamp(dir))
	for _, f := range files {
		b.WriteString("|" + f + "=" + fileStamp(f) + readableMark(f))
	}
	return b.String()
}

// readableMark distinguishes a file that can be opened from one that
// cannot (a TCC wedge changes no stamp).
func readableMark(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return "!"
	}
	_ = f.Close()
	return ""
}

// consoleProjectsCache keeps one project set per observatory model build
// and input watermarks; the reads behind it are cached by theirs
// (projectsCaches), so a model rebuild costs a fold, not a re-read.
var consoleProjectsCache struct {
	sync.Mutex
	key  string
	read projectsRead
}

// projects is the request's project set.
func (cr *consoleRequest) projects() projectsRead {
	m := cr.model()
	st, storeRead, storePath := daemonStore(cr.deps)
	storeStamp := ""
	if storePath != "" {
		storeStamp = fileStamp(storePath) + "|" + fileStamp(storePath+"-wal")
	}
	key := fmt.Sprintf("%s|%p|%s|%s|%s", cr.deps.selfURL, m.Services, m.GeneratedAt, fileStamp(verifyLedgerPath()), storeStamp)
	consoleProjectsCache.Lock()
	defer consoleProjectsCache.Unlock()
	if consoleProjectsCache.key == key {
		return consoleProjectsCache.read
	}
	src := projectSources{baseURL: cr.deps.selfURL, model: m, now: cr.now, st: st, storeRead: storeRead, storePath: storePath, cache: consoleGatherCache}
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
