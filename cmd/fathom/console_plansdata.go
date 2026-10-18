package main

// console_plansdata.go — the Proof pages' plan data (OC-21..OC-23). A plan's
// proof state is what the verify engine computes, never the checkbox
// (SD-2: "computed at render via the plans-status engine"), and that engine
// is not cheap: 64 plans took 1m38s through the daemon on the real fleet.
// So the pages verify plans in the background, one cache entry per plan
// keyed by its content, the service's index watermark and the repository's
// HEAD — the inputs a verdict depends on — and a request waits a bounded
// time for the entries it needs. A plan whose verdicts have not arrived is
// shown as "not yet verified" (its claimed rows never read as proven), and
// when they arrive the plans panels are told so over /events (OC-14), never
// moved under the reader.
//
// The verdicts come from the same path `fathom plans status` takes — the
// daemon's verify_plan through an origin-stamped client (the console's own
// reads are never usage), the working tree's lexical oracles and the local
// self-skip comparison overlaid — so a page and the verb cannot disagree.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/internal/verify/goneev"
)

// The wait budgets: how long a request waits for background verification
// before it renders what has arrived. The portfolio covers the fleet and
// waits little; a plan page waits long enough for one large plan; the
// plans headline the Now and Fleet pages share (fleetPlanMetrics) never
// waits — those pages render what has arrived, pending named.
var (
	consolePlansPortfolioWait = 1500 * time.Millisecond
	consolePlansPlanWait      = 12 * time.Second
	consolePlansHeadlineWait  = time.Duration(0)
)

// planEntry is one plan's cached verification.
type planEntry struct {
	service, root, path string
	key                 string
	done                chan struct{}
	finished            bool
	result              planComputed
	daemonStale         bool
	err                 string
	storeCommit         string
	at                  time.Time
	// rawKey is the key of the daemon's own verdicts (the plan's content
	// and the index watermark); raw is that report before the working
	// tree is laid over it. When only HEAD moved, a new entry takes the
	// previous raw report (prevRaw) and re-applies the cheap local
	// overlays instead of asking the daemon again.
	rawKey  string
	raw     *verify.Report
	prevRaw *verify.Report
	// digest is what the plans panels show of this entry — every row's
	// verdict, the daemon's staleness, the failure — and prevDigest the
	// last finished predecessor's (hasPrev). A recomputation that yields
	// the predecessor's digest publishes no verify event (design §4: an
	// event means something moved, as a no-op re-ingest is no ingest).
	digest     string
	prevDigest string
	hasPrev    bool
	// job is the pool's flight verifying this entry (RT-117).
	job *console.Job
}

// dropped reports whether the pool dropped this entry's job unrun: a newer
// key of the same plan superseded it while it was queued.
func (e *planEntry) dropped() bool {
	if e.job == nil {
		return false
	}
	select {
	case <-e.job.Done():
		return e.job.Dropped()
	default:
		return false
	}
}

type plansStamp struct {
	value string
	at    time.Time
}

// consolePlans is one daemon's plan cache and its workers.
type consolePlans struct {
	deps    *consoleDeps
	mu      sync.Mutex
	entries map[string]*planEntry
	// flights are the entries submitted and not yet finished, by path and
	// key: a key still running is never queued again (RT-116) — a plan
	// whose key moves away and back while its first verification runs
	// rejoins that verification.
	flights map[string]*planEntry
	heads   map[string]plansStamp
	stale   map[string]plansStamp
	hist    map[string]historyEntry
	ledger  struct {
		size  int64
		mtime time.Time
		read  ledgerRead
		roots string // the registered roots' key the read was kept to (PE-24)
	}
}

var (
	consolePlansMu sync.Mutex
	//fathom:tenancy-allow pkg-map registry keyed by the daemon's consoleDeps (one entry per daemon); the plan cache it holds is bounded by the plan count but sits outside memo (I51, RT-95 follow-up)
	consolePlansFor = map[*consoleDeps]*consolePlans{}
)

// plansFor returns the daemon's plan cache.
func plansFor(d *consoleDeps) *consolePlans {
	consolePlansMu.Lock()
	defer consolePlansMu.Unlock()
	p := consolePlansFor[d]
	if p == nil {
		p = &consolePlans{deps: d, entries: map[string]*planEntry{}, flights: map[string]*planEntry{},
			heads: map[string]plansStamp{}, stale: map[string]plansStamp{},
			hist: map[string]historyEntry{}}
		consolePlansFor[d] = p
	}
	return p
}

// consolePlansRetry is how long a verification that failed (the daemon
// busy, a file momentarily unreadable) stands before the next read asks
// again; the failure is shown, with its reason, until then.
const consolePlansRetry = 30 * time.Second

// serviceRoot is one registered service's name and root.
type serviceRoot struct{ Service, Root string }

// modelServiceRoots lists the registry's services with their roots.
func modelServiceRoots(m observatoryModel) []serviceRoot {
	out := make([]serviceRoot, 0, len(m.Services))
	for _, s := range m.Services {
		out = append(out, serviceRoot{Service: s.Service, Root: s.Path})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service < out[j].Service })
	return out
}

// servicePlans lists one service's active plans (the verb's discovery:
// docs/plans/*.md, non-recursive, README excluded) and snapshots each; the
// entries returned are the ones to wait on.
func (p *consolePlans) servicePlans(ctx context.Context, sr serviceRoot, only string) (console.ServicePlans, []*planEntry) {
	out := console.ServicePlans{Service: sr.Service, Root: sr.Root}
	files, errText := servicePlanFiles(sr.Root)
	if errText != "" {
		out.Err = errText
		return out, nil
	}
	var waits []*planEntry
	for _, f := range files {
		if only != "" && console.PlanSlug(f) != only {
			continue
		}
		facts, e := p.plan(ctx, sr, f)
		out.Plans = append(out.Plans, facts)
		if e != nil {
			waits = append(waits, e)
		}
	}
	return out, waits
}

// servicePlanFiles is a service's active plans: docs/plans/*.md under its
// root, non-recursive, README excluded (the verbs' discovery). A root with
// no docs/plans has none; a docs/plans that exists and cannot be read is
// the error, never "no plans" (I38).
func servicePlanFiles(root string) ([]string, string) {
	if root == "" {
		return nil, ""
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "plans")); err != nil {
		if os.IsNotExist(err) {
			return nil, ""
		}
		return nil, err.Error()
	}
	files, err := activePlanFiles(root)
	if err != nil {
		return nil, err.Error()
	}
	return files, ""
}

// plan snapshots one plan: its verdicts when they are cached at the current
// key, else its authored rows (queued for verification).
func (p *consolePlans) plan(ctx context.Context, sr serviceRoot, path string) (console.PlanFacts, *planEntry) {
	rel := filepath.ToSlash(filepath.Join("docs", "plans", filepath.Base(path)))
	facts := console.PlanFacts{Service: sr.Service, Path: path, Rel: rel, Title: filepath.Base(path)}
	//fathom:tenancy-allow whole-read reads one plan document the page renders; plan files are small, one per request, cached by stamp
	raw, err := os.ReadFile(path)
	if err != nil {
		facts.Err = "the plan file could not be read: " + err.Error()
		return facts, nil
	}
	facts.Title = planTitle(raw, path)
	sum := sha256.Sum256(raw)
	wm := p.storeWatermark(ctx, sr.Service)
	rawKey := hex.EncodeToString(sum[:8]) + "|" + wm
	key := rawKey + "|" + p.head(sr.Root)

	p.mu.Lock()
	e := p.entries[path]
	if e == nil || e.key != key || (e.finished && e.err != "" && p.deps.now().Sub(e.at) >= consolePlansRetry) {
		old := e
		flightKey := path + "|" + key
		p.pruneDroppedLocked()
		if running := p.flights[flightKey]; running != nil && !running.finished {
			// The key is still being verified (it moved away and back):
			// rejoin that flight, never queue it again (RT-116).
			e = running
			p.entries[path] = e
		} else {
			e = &planEntry{service: sr.Service, root: sr.Root, path: path, key: key, rawKey: rawKey, done: make(chan struct{})}
			switch {
			case old != nil && old.finished:
				e.prevDigest, e.hasPrev = old.digest, true
			case old != nil && old.hasPrev:
				// Replaced before it finished: it never published, so the
				// last result a panel was told of is still its predecessor's.
				e.prevDigest, e.hasPrev = old.prevDigest, true
			}
			if old != nil && old.finished && old.err == "" && old.rawKey == rawKey && old.raw != nil {
				// Only HEAD moved: the daemon's verdicts are reused and only
				// the cheap local overlays run again.
				e.prevRaw = old.raw
			}
			// RT-117: one bounded pool; a newer key of this plan drops a
			// queued older one (the group), and a full queue leaves the
			// plan "not yet verified" for the next read to submit again.
			ne := e
			job, err := p.deps.jobPool().SubmitJob("plan:"+path, flightKey, func() { p.verify(ne) })
			if err != nil {
				p.mu.Unlock()
				facts.Rows = authoredRows(sr.Service, raw)
				facts.Pending = true
				return facts, nil
			}
			e.job = job
			p.flights[flightKey] = e
			p.entries[path] = e
		}
	}
	finished, result, stale, errText, storeCommit, at := e.finished, e.result, e.daemonStale, e.err, e.storeCommit, e.at
	p.mu.Unlock()

	switch {
	case finished && errText != "":
		facts.Err = errText
		facts.Rows = authoredRows(sr.Service, raw)
	case finished:
		facts.Rows = planRowFacts(result.rows())
		facts.Verified, facts.DaemonStale, facts.StoreCommit, facts.VerifiedAt = true, stale, storeCommit, at
	default:
		facts.Rows = authoredRows(sr.Service, raw)
		facts.Pending = true
	}
	return facts, e
}

// pruneDroppedLocked forgets flights the pool dropped unrun, releasing
// anyone still waiting on them: a dropped key asked for again is a new
// flight, never a join of one that will not run.
func (p *consolePlans) pruneDroppedLocked() {
	for k, e := range p.flights {
		if e.dropped() {
			delete(p.flights, k)
			close(e.done)
		}
	}
}

// authoredRows are a plan's rows before any verdict.
func authoredRows(service string, raw []byte) []console.PlanRowFact {
	plan := verify.ParseMarkdownPlan(service, raw)
	out := make([]console.PlanRowFact, 0, len(plan.Tasks))
	for _, t := range plan.Tasks {
		out = append(out, planTaskFact(t, nil))
	}
	return out
}

// entry returns the finished computation for a plan, if any.
func (p *consolePlans) entry(path string) (*planEntry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.entries[path]
	return e, e != nil && e.finished
}

// wait blocks until every entry has finished or the budget elapses.
func (p *consolePlans) wait(entries []*planEntry, budget time.Duration) {
	if len(entries) == 0 {
		return
	}
	timer := time.NewTimer(budget)
	defer timer.Stop()
	for _, e := range entries {
		select {
		case <-e.done:
		case <-timer.C:
			return
		}
	}
}

// consolePlansGate, when set, holds every verification until it is closed:
// a test hook that makes "still verifying" observable.
var consolePlansGate struct {
	sync.Mutex
	ch chan struct{}
}

func setConsolePlansGate(ch chan struct{}) {
	consolePlansGate.Lock()
	consolePlansGate.ch = ch
	consolePlansGate.Unlock()
}

// verify computes one plan through the verb's own path.
func (p *consolePlans) verify(e *planEntry) {
	var (
		wm       string
		computed planComputed
		rawRep   *verify.Report
		err      error
		stale    bool
	)
	// A verification that panics finishes as a failure the page names:
	// the job pool recovers the panic, but the entry must still be
	// finished, its flight deleted and its waiters released, or every
	// later render rejoins a flight that never ends.
	func() {
		defer func() {
			if v := recover(); v != nil {
				err = fmt.Errorf("the verification panicked: %v", v)
			}
		}()
		consolePlansGate.Lock()
		gate := consolePlansGate.ch
		consolePlansGate.Unlock()
		if gate != nil {
			<-gate
		}
		//fathom:tenancy-allow bg-ctx plan verification is a shared single flight whose result outlives the request that started it; bounded by consolePlanVerifyDeadline / consoleGitTimeout
		ctx := context.Background()
		wm = p.storeWatermark(ctx, e.service)
		computed, rawRep, err = p.compute(e)
		stale = p.daemonStale(e.service, e.root)
	}()
	p.mu.Lock()
	if err != nil {
		e.err = err.Error()
	} else {
		e.result, e.raw = computed, rawRep
		e.daemonStale = stale
	}
	e.storeCommit = strings.SplitN(wm, "@", 2)[0]
	e.at = p.deps.now()
	e.digest = verdictDigest(e.result.Report, e.daemonStale, e.err)
	e.finished = true
	delete(p.flights, e.path+"|"+e.key)
	current := p.entries[e.path] == e
	changed := !e.hasPrev || e.digest != e.prevDigest
	p.mu.Unlock()
	if current && changed && p.deps.bus != nil {
		ref := "pass"
		if err != nil || verifyGateFailures([]verify.Report{e.result.Report}, false, false) != nil {
			ref = "fail"
		}
		_, _ = p.deps.bus.Publish(console.Event{Kind: console.EventVerify, Service: e.service, Name: filepath.Base(e.path), Ref: ref})
	}
	// Waiters are released once the event is out: a page rendered from
	// these verdicts starts its /events cursor after the event that
	// announced them, so it is never told "Updated" about itself.
	close(e.done)
}

// verdictDigest is what the plans panels draw of one verification: each
// row's verdict and label, the daemon's staleness, and the failure.
func verdictDigest(rep verify.Report, stale bool, errText string) string {
	h := sha256.New()
	fmt.Fprintf(h, "stale=%v\x00err=%s\x00", stale, errText)
	for _, t := range rep.Tasks {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", t.TaskID, t.Status, t.Label)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// compute verifies one plan the way `plans status` does — the plan parsed
// with the self-skip comparison, the daemon's verdicts, the working tree
// laid over them — reusing the daemon's verdicts when only HEAD moved. It
// returns the overlaid result and the daemon's report before the overlays.
func (p *consolePlans) compute(e *planEntry) (planComputed, *verify.Report, error) {
	//fathom:tenancy-allow whole-read reads one plan document the page renders; plan files are small, one per request, cached by stamp
	raw, err := os.ReadFile(e.path)
	if err != nil {
		return planComputed{}, nil, fmt.Errorf("reading plan %q: %w", e.path, err)
	}
	lp := parsePlanForVerify(e.service, e.root, e.path, raw)
	var rep verify.Report
	switch {
	case e.prevRaw != nil:
		rep = cloneReport(*e.prevRaw)
	case p.deps.c != nil && inProcessDaemon(p.deps.selfURL) != nil:
		// Inside the daemon: its own verify_plan handler, in process and
		// under a deadline — never a loopback request (RT-116).
		rep, err = verifyPlanInProcess(p.deps, e.service, raw, lp, goneTreeFor(e.root, lp))
		if err != nil {
			return planComputed{}, nil, err
		}
	default:
		hc := consoleRPCClient(90 * time.Second)
		indexed, ierr := daemonServiceIndexed(hc, p.deps.selfURL, e.service)
		switch {
		case ierr != nil:
			return planComputed{}, nil, fmt.Errorf("probing daemon for service %q: %w", e.service, ierr)
		case !indexed:
			return planComputed{}, nil, fmt.Errorf("service %q is not indexed on the daemon at %s — ingest it and retry", e.service, p.deps.selfURL)
		}
		rep, err = verifyViaHTTP(hc, p.deps.selfURL, e.service, "", raw, lp, goneTreeFor(e.root, lp))
		if err != nil {
			return planComputed{}, nil, err
		}
	}
	rawRep := cloneReport(rep)
	overlayPlanReport(&rep, lp, verify.DirFiles(e.root))
	return planComputed{File: e.path, Plan: lp, Report: rep}, &rawRep, nil
}

// consolePlanVerifyDeadline bounds one plan's in-process verification,
// follow-up calls included.
var consolePlanVerifyDeadline = 90 * time.Second

// verifyPlanInProcess verifies one plan through this daemon's own
// verify_plan handler in process (RT-116): admitted by the server's gate
// as a console call like any request, under consolePlanVerifyDeadline, the
// service's index probed in the store rather than over /indexed. The
// verdicts are the ones `fathom plans status` merges (verifyWithCaller).
func verifyPlanInProcess(d *consoleDeps, service string, raw []byte, lp verify.Plan, tree *goneev.PlanTree) (verify.Report, error) {
	//fathom:tenancy-allow bg-ctx plan verification is a shared single flight whose result outlives the request that started it; bounded by consolePlanVerifyDeadline / consoleGitTimeout
	ctx, cancel := context.WithTimeout(context.Background(), consolePlanVerifyDeadline)
	// O1169: like the CLI, ask for the whole plan in one answer; the follow-up
	// loop in verifyWithCaller stays the fallback.
	ctx = mcp.WithDeclaredBudget(ctx, verifyPlanDeclaredBudget)
	defer cancel()
	if d.c != nil {
		indexed, _, err := d.c.IsIndexed(ctx, service)
		switch {
		case err != nil:
			return verify.Report{}, fmt.Errorf("probing the store for service %q: %w", service, err)
		case !indexed:
			return verify.Report{}, fmt.Errorf("service %q is not indexed on the daemon at %s — ingest it and retry", service, d.selfURL)
		}
	}
	rep, err := verifyWithCaller(func(md []byte) (httpVerifyResult, error) {
		body, err := verifyPlanRequest(service, "", md, tree)
		if err != nil {
			return httpVerifyResult{}, err
		}
		resp, ok := inProcessCall(ctx, d.selfURL, "", body)
		if ctx.Err() != nil {
			return httpVerifyResult{}, fmt.Errorf("verify_plan did not answer within the console's %s deadline", consolePlanVerifyDeadline)
		}
		if !ok {
			return httpVerifyResult{}, fmt.Errorf("verify_plan: this daemon's handler is not available in process")
		}
		return parseVerifyPlanResponse(resp)
	}, raw, lp)
	return rep, err
}

// cloneReport deep-copies a report, so the overlays can never write into a
// cached one.
func cloneReport(r verify.Report) verify.Report {
	out := r
	out.Tasks = make([]verify.TaskVerdict, len(r.Tasks))
	for i, t := range r.Tasks {
		t.Criteria = append([]verify.Criterion(nil), t.Criteria...)
		out.Tasks[i] = t
	}
	return out
}

// storeWatermark is the service's index watermark ("" when unknown).
func (p *consolePlans) storeWatermark(ctx context.Context, service string) string {
	if p.deps.c == nil {
		return ""
	}
	wm, _ := p.deps.c.StoreWatermark(ctx, service)
	return wm
}

// headTTL bounds how often a repository's HEAD is re-read.
const headTTL = 10 * time.Second

// head is the repository's HEAD commit, re-read at most every headTTL.
func (p *consolePlans) head(root string) string {
	p.mu.Lock()
	st, ok := p.heads[root]
	p.mu.Unlock()
	now := time.Now()
	if ok && now.Sub(st.at) < headTTL {
		return st.value
	}
	// Read from the repository's files, never a git process (RT-115).
	//fathom:tenancy-allow bg-ctx HEAD read from the ref files (gitro.Head, no fork) for a shared per-root cache entry
	v, _ := gitro.Head(context.Background(), root)
	p.mu.Lock()
	p.heads[root] = plansStamp{value: v, at: now}
	p.mu.Unlock()
	return v
}

// daemonStale is PC-5's measured-stale reading for the service's root,
// re-read at most every headTTL.
func (p *consolePlans) daemonStale(service, root string) bool {
	p.mu.Lock()
	st, ok := p.stale[service]
	p.mu.Unlock()
	now := time.Now()
	if ok && now.Sub(st.at) < headTTL {
		return st.value == "stale"
	}
	stale, _ := liveDaemonStale(consoleRPCClient(5*time.Second), p.deps.selfURL, root)
	v := ""
	if stale {
		v = "stale"
	}
	p.mu.Lock()
	p.stale[service] = plansStamp{value: v, at: now}
	p.mu.Unlock()
	return stale
}

// ledger is the verify ledger under the registered roots (PE-24), re-read
// only when its size or mtime or the registered roots moved.
func (p *consolePlans) ledgerRead() ledgerRead {
	path := verifyLedgerPath()
	roots := registeredLedgerRoots(consoleRPCClient(5*time.Second), p.deps.selfURL)
	st, err := os.Stat(path)
	p.mu.Lock()
	cached := p.ledger
	p.mu.Unlock()
	if err == nil && cached.read.Path == path && st.Size() == cached.size && st.ModTime().Equal(cached.mtime) && cached.roots == roots.Key() {
		return cached.read
	}
	read := readLedgerRuns(roots)
	if err == nil {
		p.mu.Lock()
		p.ledger.size, p.ledger.mtime, p.ledger.read, p.ledger.roots = st.Size(), st.ModTime(), read, roots.Key()
		p.mu.Unlock()
	}
	return read
}

// historyEntry is one plan file's git history, read at one HEAD.
type historyEntry struct {
	revs  []planRevisionText
	older int
	note  string
}

// history is a plan file's git history, re-read only when the
// repository's HEAD moves.
func (p *consolePlans) history(root, rel string) ([]planRevisionText, int, string) {
	key := root + "|" + rel + "|" + p.head(root)
	p.mu.Lock()
	h, ok := p.hist[key]
	p.mu.Unlock()
	if ok {
		return h.revs, h.older, h.note
	}
	//fathom:tenancy-allow bg-ctx plan verification is a shared single flight whose result outlives the request that started it; bounded by consolePlanVerifyDeadline / consoleGitTimeout
	ctx, cancel := context.WithTimeout(context.Background(), consoleGitTimeout)
	revs, older, note := planGitHistory(ctx, root, rel)
	timedOut := ctx.Err() != nil
	cancel()
	if timedOut {
		return revs, older, note // not kept: the next render asks again
	}
	p.mu.Lock()
	if len(p.hist) > 256 {
		p.hist = map[string]historyEntry{}
	}
	p.hist[key] = historyEntry{revs: revs, older: older, note: note}
	p.mu.Unlock()
	return revs, older, note
}

// graph is the service's call graph — the verify engine's own (syntax
// edges plus the typed pass) — from the daemon's graph store (RT-74): one
// per (service, watermark), shared with the MCP tools, never a private copy.
func (p *consolePlans) graph(ctx context.Context, service string) (*callgraph.Graph, error) {
	if p.deps.c == nil {
		return nil, errNoStore
	}
	_, _, g, _, err := consoleBothGraph(ctx, p.deps.c, service) // a typed read failure degrades to the syntax edges
	return g, err
}

// errNoStore is the row evidence's answer on a daemon with no store.
var errNoStore = &consoleNoStoreError{}

type consoleNoStoreError struct{}

func (*consoleNoStoreError) Error() string {
	return "this console has no store to read the call graph from"
}
