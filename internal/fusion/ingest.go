package fusion

// ingest.go — Ingest, the three-pass pipeline over a service, and what closes it:
// finalCommit, recordIngestMeta, deletedRefs, purgeOrphans and the service-refs run. Split
// out of pipeline.go unchanged.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Ingest discovers, fetches, and (unless Lazy) extracts all files for svc.
//
// Pass 1 — Filter: discover all SourceRefs; input order is already
// deterministic (FauxConnector sorts by path).
//
// Pass 2 — Fetch: up to cfg.Concurrency goroutines fetch file content in
// parallel. Each worker writes into a pre-allocated slot in the fetched slice
// so no locks are needed during fetch. A per-file read failure is recorded
// on the Report and the file skipped (readfailures.go); the first NON-read
// error is captured via atomic pointer and aborts; all goroutines finish
// before Ingest returns. An ingest that could read nothing refuses to touch
// an index that already holds observations (IT-15 / O16).
//
// Pass 3 — PostFetch (sequential, input order): sha256 each file, compare
// against stored fingerprint. Skip unchanged files. On change: PurgeSource
// then mark pending. Honour MaxTokens / MaxCostUSD budget gates.
//
// Dispatch: pending files are grouped by extension and sent to the registered
// extractor. Each core.SymbolNode is mapped to a core.Observation and passed
// to store.PutObservations; store.SetFingerprint is called for every processed
// file.
//
// Orphan purge: after a successful ingest, source_refs stored for svc that no
// longer appear in the Discover set (deleted or newly ignored files) have
// their observations and fingerprints removed. See purgeOrphans.
//
// Report.PassTimes times each pass (EW-48, passtimes.go) by the clock
// WithPassClock put on ctx, SystemPassClock when none.
func (p *Pipeline) Ingest(ctx context.Context, svc core.ServiceRef) (Report, error) {
	ctx, passes := startPassTimes(ctx)
	report, err := p.ingest(ctx, svc)
	report.PassTimes = passes.times()
	return report, err
}

// ingest is Ingest's passes; each closes its lap on the ingest's pass
// recorder (passes.mark) right after it runs.
func (p *Pipeline) ingest(ctx context.Context, svc core.ServiceRef) (Report, error) {
	passes := passTimesFrom(ctx)
	// RT-109 (design §6): the watermark this ingest will record is read
	// now, before the corpus is, and ingest_state = in_progress is written
	// first. The watermark moves only at the final commit (commitIngest); a
	// killed or halted ingest leaves in_progress and the old watermark.
	run := p.beginIngestState(ctx, svc)

	// ── Pass 1: Filter ──────────────────────────────────────────────────────
	refs, err := p.conn.Discover(ctx, svc)
	if err != nil {
		p.abandonIngestState(ctx, run) // before the first write
		return Report{}, fmt.Errorf("fusion: discover %q: %w", svc.ID, err)
	}

	var report Report
	report.Discovered = len(refs)
	if er, ok := p.conn.(connector.ExclusionReporter); ok {
		report.DefaultIgnoredDirs = er.Exclusions(svc.ID).DefaultIgnored
	}
	run.refs = refs
	run.defaultIgnore = len(report.DefaultIgnoredDirs)

	// Build this ingest's sibling index ONCE from the just-Discovered list
	// (ADR-021 §2 / TI-1, option (b) from the grounding: O(files), not
	// O(files²)) — it answers "does an ancestor dir contain Chart.yaml /
	// cue.mod?" and "is there a Dockerfile named after this image?" for
	// every file the classifier or cross-ref pass looks at below. Stored
	// per-service so a later ExtractPending (lazy mode) sees the SAME index
	// this Ingest computed, rather than an empty one.
	idx := buildSiblingIndex(refs)
	p.mu.Lock()
	p.siblingIdx[svc.ID] = idx
	p.mu.Unlock()

	if len(refs) == 0 {
		// Still a full-service ingest: every stored ref is now an orphan.
		run.changed = 1 // every stored symbol goes: centrality must follow (TQ-2)
		p.indexDocs(ctx, svc, refs, &report)
		if err := p.purgeOrphans(ctx, svc.ID, refs); err != nil {
			return report, err
		}
		p.commitIngest(ctx, svc, run)
		return report, nil
	}

	// ADR-017 self-heal: stores written before repo-relative anchors keyed
	// fingerprints and observations by absolute path. Once the connector
	// emits relative refs, those legacy rows would never match the hash-gate
	// again and every symbol would exist twice (old absolute + new relative
	// anchor). Detect the mix and rebuild this service's content wholesale,
	// once — right before the first write, so a wedge (nothing readable)
	// cannot trigger a rebuild it cannot complete. A rebuild reads every
	// file: the stat gate is off for it.
	legacy := false
	if !filepath.IsAbs(refs[0].Path) {
		if n, cErr := p.st.CountAbsoluteSourceRefs(ctx, svc.ID); cErr == nil && n > 0 {
			legacy = true
		}
	}
	wrote := false
	beforeWrite := func() error {
		if wrote {
			return nil
		}
		wrote = true
		if legacy {
			if pErr := p.st.PurgeService(ctx, svc.ID); pErr != nil {
				return fmt.Errorf("fusion: purge legacy-anchored service %q: %w", svc.ID, pErr)
			}
			report.PurgedLegacyAnchors = true
		}
		return nil
	}
	// A failure before the first write puts the previous ingest_state row
	// back (RT-109); after it, the ingest stays in_progress for the next
	// start to resume.
	abandon := func(err error) (Report, error) {
		if !wrote {
			p.abandonIngestState(context.WithoutCancel(ctx), run)
		}
		return report, err
	}

	// ── Pass 1: stat gate (RT-102) ───────────────────────────────────────────
	// RT-89: a service whose extracted routes and client calls are not yet
	// complete extracts them from every readable file, so this ingest reads
	// every file (the stat gate is off for it, once).
	svcRefs, err := p.beginServiceRefs(ctx, svc.ID)
	if err != nil {
		return abandon(err)
	}
	if legacy {
		svcRefs.complete = false // the rebuild purges the rows with the rest
	}
	gate := p.statChangedFiles(ctx, svc.ID, refs, legacy || !svcRefs.complete)
	report.Skipped += gate.skipped
	report.StatSkipped = gate.skipped
	// RT-76: a stat-skipped file's facts are the stat and the hash the gate
	// vouched for; it is never read.
	for i, ref := range refs {
		if gate.live[i].ContentHash != "" {
			run.noteFact(ref.Path, fileFact{size: gate.live[i].Size, hash: gate.live[i].ContentHash,
				excluded: gate.live[i].Excluded, sniffed: gate.live[i].Sniffed})
		}
	}

	// ── Pass 2 + 3: streamed read, hash-gate, extract, write (RT-103) ────────
	// Up to cfg.Concurrency reads run at once through the in-flight byte
	// window; results reach this consumer in input order (determinism), which
	// hash-gates, purges, extracts and writes each file and then releases its
	// bytes. A per-file READ failure (readfailures.go) is recorded and the
	// file skipped; any other fetch error is fatal and stops the reads.
	var failures []ReadFailure
	type oversizeRef struct {
		path string
		size int64
	}
	var oversize []oversizeRef
	var pending []pendingFile // lazy mode only
	var tokenAccum int64
	// changed collects every path this ingest found new, changed or
	// deleted — the typed and CUE passes' gate (RT-104, RT-105).
	var changed []string
	sink := p.newExtractSink(svc.ID)
	// A pass input's new fingerprint is written only after that pass's
	// success record is gone (design §7.2), so a run that fails between the
	// two cannot hide the change from the next ingest's gate.
	inv := newPassInvalidator(p.st, svc.ID)

	passes.mark("discover")
	peak, streamErr := p.fetchWindowed(ctx, refs, gate.toRead, p.cfg.InflightBytes, p.cfg.Concurrency, func(i int, content []byte, ferr error) error {
		ref := refs[i]
		if over := asOversize(ferr); over != nil {
			// RT-69: over the read cap, decided from the stat — never read;
			// recorded after the refusal check below.
			oversize = append(oversize, oversizeRef{ref.Path, over.Size})
			run.noteFact(ref.Path, fileFact{size: over.Size, excluded: "oversize"})
			return nil
		}
		if ferr != nil {
			if isReadFailure(ferr) {
				// Unreadable this run: no hash-gate, no purge, no
				// fingerprint — the stored evidence stays until the file
				// can be read again.
				failures = append(failures, ReadFailure{Path: ref.Path, Err: ferr.Error()})
				run.noteFact(ref.Path, fileFact{size: -1, excluded: "unreadable"})
				return nil
			}
			return fmt.Errorf("fusion: fetch: %w", ferr)
		}

		// Epoch-salted content hash (see FingerprintHash / ExtractorEpoch):
		// an extractor-semantics change must invalidate every stored
		// fingerprint, or improved extraction never reaches unchanged files —
		// the stale-observation class dogfood O9 caught.
		hash := FingerprintHash(content)
		// LB-F9/LB-F10: the dialect gate's verdict is a fact of the bytes,
		// recorded whether or not the hash-gate lets the file through.
		verdict := gateDialectFile(ref.Path, content)
		run.noteFact(ref.Path, fileFact{size: int64(len(content)), hash: hash, excluded: verdict.excluded, sniffed: verdict.sniffed})
		gate.live[i].ContentHash = hash
		gate.live[i].Excluded, gate.live[i].Sniffed = verdict.excluded, verdict.sniffed

		// RT-89: an incomplete service extracts every readable file's
		// routes and client calls; a complete one only the changed files'.
		if !svcRefs.complete {
			svcRefs.add(ref.Path, content)
		}

		// Hash-gate: compare against stored fingerprint.
		storedHash, found, fpErr := p.st.Fingerprint(ctx, svc.ID, ref.Path)
		if fpErr != nil {
			return fmt.Errorf("fusion: fingerprint lookup %q: %w", ref.Path, fpErr)
		}
		if found && storedHash == hash && !legacy {
			report.Skipped++ // file unchanged
			return nil
		}
		if err := beforeWrite(); err != nil {
			return err
		}
		if err := inv.note(ctx, ref.Path); err != nil {
			return err
		}
		if legacy {
			found = false // the rebuild purged every row
		}

		// File is new or changed: purge stale observations first.
		if found {
			if purgeErr := p.st.PurgeSource(ctx, svc.ID, ref.Path); purgeErr != nil {
				return fmt.Errorf("fusion: purge %q: %w", ref.Path, purgeErr)
			}
		}

		// Budget gate (token count heuristic: bytes / 4).
		if p.cfg.MaxTokens > 0 {
			tokenAccum += int64(len(content)) / 4
			if tokenAccum > p.cfg.MaxTokens {
				return fmt.Errorf("fusion: token budget exceeded at %q (%d tokens)",
					ref.Path, tokenAccum)
			}
		}
		if p.cfg.MaxCostUSD > 0 {
			// Cost estimate: $0.00003 per 1000 tokens (ballpark).
			costEst := float64(tokenAccum) / 1000.0 * 0.00003
			if costEst > p.cfg.MaxCostUSD {
				return fmt.Errorf("fusion: cost budget exceeded at %q ($%.6f)",
					ref.Path, costEst)
			}
		}

		changed = append(changed, ref.Path)
		report.Extracted++
		if svcRefs.complete {
			svcRefs.add(ref.Path, content)
		}
		pf := pendingFile{ref: ref, content: content, hash: hash, verdict: verdict}
		if p.cfg.Lazy {
			pending = append(pending, pf)
			return nil
		}
		return sink.add(ctx, pf)
	})
	report.PeakInflightBytes = peak
	if streamErr == nil {
		streamErr = sink.flush(ctx)
	}
	if streamErr != nil {
		return abandon(streamErr)
	}

	// Read failures (IT-14): counted, named, sorted by path — never a silent
	// zero and never fatal on their own. refs keeps every discovered file,
	// so the orphan purge below cannot mistake "unreadable" for "deleted".
	sort.Slice(failures, func(i, j int) bool { return failures[i].Path < failures[j].Path })
	report.ReadFailed = len(failures)
	report.ReadFailures = failures
	// finish wraps every exit below: an ingest that completed but hit read
	// failures returns them as a *ReadFailuresError beside the (complete,
	// honest) report, so callers that treat a fetch error as failure keep
	// doing so — the readable files were still indexed. When the orphan
	// purge ALSO failed, both signals survive in one joined error: an
	// err-only caller still sees the purge abort in the text, and an
	// errors.As caller still finds the read failures it must name.
	finish := func(err error) (Report, error) {
		if err == nil {
			// RT-102: the next ingest's pass 1 compares against these.
			p.rememberStats(ctx, svc.ID, gate)
		}
		if len(failures) == 0 {
			return report, err
		}
		rfe := &ReadFailuresError{Service: svc.ID, Failures: failures}
		if err != nil {
			return report, errors.Join(err, rfe)
		}
		return report, rfe
	}

	// IT-15 refusal (O16): an ingest that could read NOTHING extracted zero
	// symbols by construction; if the store already holds observations for
	// this service, replacing them — or even advancing the freshness
	// watermark over them — would let a wedged reader pass off an empty
	// index as a fresh one. Refuse before the first write (nothing has been
	// written: every read failed). A store with no evidence has nothing to
	// protect, but is not committed either: it is returned Incomplete and
	// stays in_progress (F2, below). A PARTIAL wedge is not refused: the
	// readable files are honest progress and the unreadable ones keep their
	// evidence; refusing there would also make a repo with one permanently
	// unreadable file un-ingestable forever.
	if len(failures) > 0 && len(failures) == len(refs) {
		existing, qErr := p.existingObservations(ctx, svc.ID)
		if qErr != nil {
			return abandon(fmt.Errorf("fusion: count observations %q: %w", svc.ID, qErr))
		}
		if existing > 0 {
			// Refused before the first write: the index is untouched and
			// still the one its watermark describes.
			return abandon(&ReadFailuresError{Service: svc.ID, Failures: failures, Refused: true, Existing: existing})
		}
		// F2 (docs/bench/readings/rt-waves-c-f-2026-09-27.md §6): with no
		// evidence to protect, an ingest that read nothing used to fall
		// through to the final commit — watermark = HEAD, ingest_state =
		// complete over zero fingerprints — and auto-refresh, seeing no
		// drift, served that empty index as fresh forever. RT-109: only a
		// final commit advances the watermark, and an ingest that indexed
		// nothing is not one. Nothing has been written; ingest_state stays
		// in_progress (naming the root), so the service never reads as
		// indexed at HEAD, the next process re-ingests it, and a retry
		// reads every file (no fingerprint, so no stat skip).
		return report, &ReadFailuresError{Service: svc.ID, Failures: failures, Incomplete: true}
	}

	// Oversize files (RT-69): fingerprinted by size so an unchanged one is
	// skipped next time; what one held when it was small is purged, since
	// it now contributes no observations. Counted under Oversize on every
	// ingest, changed or not: the file stays unindexed, so the summary names
	// it (I38) instead of folding it into the unchanged-and-skipped count.
	for _, o := range oversize {
		if err := beforeWrite(); err != nil {
			return abandon(err)
		}
		unchanged, err := p.recordOversize(ctx, inv, svc.ID, o.path, o.size)
		if err != nil {
			return report, err
		}
		if !unchanged {
			changed = append(changed, o.path)
		}
		report.Oversize++
	}

	// RT-89 (§6): routes and outbound HTTP client calls were extracted in
	// pass 2, from the content it held — never per call. They are written
	// just before the final commit, so an ingest halted anywhere before it
	// leaves the previous commit's rows.
	oversizePaths := make(map[string]bool, len(oversize))
	for _, o := range oversize {
		oversizePaths[o.path] = true
	}
	writeRefs := svcRefs.writer(ctx, p.st, refs, oversizePaths, len(failures) > 0)

	// ── Dispatch ─────────────────────────────────────────────────────────────
	if p.cfg.Lazy {
		// Lazy mode: enqueue and return without extracting. The orphan purge
		// still runs — it depends only on the Discover set vs the store, and
		// every pending file is in the Discover set, so deferred extraction is
		// unaffected.
		p.mu.Lock()
		p.queue[svc.ID] = append(p.queue[svc.ID], pending...)
		p.mu.Unlock()
		p.indexDocs(ctx, svc, refs, &report)
		if err := writeRefs(); err != nil {
			return finish(err)
		}
		return finish(p.finalCommit(ctx, svc, run, refs))
	}

	stats := sink.stats
	report.Symbols = stats.symbols
	report.Unclassified = stats.unclassified
	report.Oversize += stats.oversize
	report.ClassifiedZero = stats.classifiedZero
	report.TSEngineFaults = stats.tsFaults
	report.UnsupportedLanguages = stats.unsupportedLang
	report.SkippedGenerated = stats.skippedGenerated
	report.AmbiguousUnclassified = stats.ambiguous
	report.ClassifiedBySniff = stats.sniffed

	// Deleted files change the typed inputs too (a removed .go file).
	deleted, delErr := p.deletedRefs(ctx, svc.ID, refs)
	if delErr != nil {
		return finish(delErr)
	}
	changed = append(changed, deleted...)
	passes.mark("extract")

	// Typed Go call-edge enrichment (ADR-021 §3): additive, per-package,
	// never blocks ingest. See internal/fusion/typedcalls.go. RT-104: only
	// when a Go input changed since the last successful pass.
	typedRep, typedErr := p.enrichTypedEdges(ctx, svc, refs, changed)
	passes.mark("typed_go")
	if typedErr != nil {
		// RT-178: the typed pass was halted (the producer governor at
		// refuse, or a cancelled ctx) — fail before the final commit so the
		// watermark never moves and the ingest stays in_progress (RT-109).
		return finish(fmt.Errorf("fusion: typed-edge pass %q: %w", svc.ID, typedErr))
	}
	report.TypedPackages = typedRep.PackagesLoaded + typedRep.PackagesCached
	report.TypedPackagesFailed = typedRep.PackagesFailed
	report.TypedEdgesDirect = typedRep.EdgesDirect
	report.TypedEdgesIfaceDispatch = typedRep.EdgesIfaceDispatch
	report.TypedEdgesUnresolved = typedRep.EdgesUnresolved
	// Ingest honesty disclosure (wave P3 / O82): pull the SAME
	// TypedEdgeStatus enrichTypedEdges just recorded (it always calls
	// noteTypedEdgeStatus before returning, whenever it actually ran)
	// straight onto this Report — one source of truth, so the ingest
	// response can never disagree with staleness_report/doctor about
	// whether this service's typed pass is trustworthy.
	report.TypedSkipped = typedRep.Skipped
	if st, known := p.TypedEdgeStatus(svc.ID); known && typedRep.Skipped == "" {
		report.TypedPackagesSeen = st.PackagesSeen
		report.TypedFailed = st.Failed
		report.TypedFailReason = st.Reason
		report.TypedFallback = st.Fallback
	}

	// CUE typed-edge enrichment (ADR-041 decision 5 / SU-10): additive,
	// per-package, never blocks ingest. See internal/fusion/typedcue.go.
	cueRep, cueErr := p.enrichCUEEdges(ctx, svc, refs, changed)
	passes.mark("cue")
	if cueErr != nil {
		// Halted (the governor, a cancelled ctx) before the final commit:
		// the watermark never moves (RT-109).
		return finish(fmt.Errorf("fusion: CUE typed-edge pass %q: %w", svc.ID, cueErr))
	}
	report.CUESkipped = cueRep.Skipped
	report.CUEPackages = cueRep.PackagesLoaded
	report.CUEPackagesFailed = cueRep.PackagesFailed
	report.CUEEdgesDirect = cueRep.EdgesDirect
	report.CUEEdgesUnresolved = cueRep.EdgesUnresolved

	// Per-file resolver for TypeScript/JavaScript and Python (ADR-041
	// decision 5 / SU-11): additive, never blocks ingest. Unlike the Go/CUE
	// passes above it has no per-package content-hash cache yet (see
	// internal/fusion/fileresolve.go's package doc) — every ingest
	// re-fetches and re-parses the service's full TS/JS/Python corpus, the
	// same "whole tree, every run" shape those passes use, just without
	// their cache. See internal/fusion.ResolveFileEdges.
	fileRep, fileErr := p.enrichFileResolvedEdges(ctx, svc, refs, changed)
	passes.mark("file_resolved")
	report.FileResolvedEdges = fileRep.EdgesResolved
	report.FileResolveReused = fileRep.Reused
	report.FileResolveFaults = fileRep.Faults
	report.FileResolveFaultFiles = fileRep.FaultFiles
	if fileRep.Skipped {
		report.FileResolveSkipped = fileRep.SkipReason
	}
	if fileErr != nil {
		// RT-153 (I38): halted before the final commit — the watermark
		// never moved (RT-109), so the halted run never reads as fresh; fail
		// with the named cause instead of a shorter index.
		return finish(fmt.Errorf("fusion: file-resolved edge pass %q: %w", svc.ID, fileErr))
	}

	// Doc-engine cross-file references (LB-F30, docs/design/
	// language-breadth.md §4): additive, never blocks ingest.
	docN, docSkip, docErr := p.enrichDocXrefEdges(ctx, svc, refs, changed)
	passes.mark("doc_xref")
	report.DocXrefEdges, report.DocXrefSkipped = docN, docSkip
	if docErr != nil {
		return finish(fmt.Errorf("fusion: doc-xref edge pass %q: %w", svc.ID, docErr))
	}

	// Route references (LB-F97, wave px; docs/design/language-breadth.md
	// §4): a route's target host joined to the Kubernetes or compose
	// service declaring it, additive, never blocks ingest.
	routeRep, routeErr := p.enrichRouteRefEdges(ctx, svc, refs, changed)
	passes.mark("route_ref")
	report.RouteRefEdges, report.RouteRefUnresolved, report.RouteRefSkipped = routeRep.edges, routeRep.unresolved, routeRep.skipped
	if routeErr != nil {
		return finish(fmt.Errorf("fusion: route-ref edge pass %q: %w", svc.ID, routeErr))
	}

	// Policy references (LB-F136, wave pc; docs/design/language-breadth.md
	// §4): a policy's targets joined to the declared objects and resources,
	// additive, never blocks ingest.
	policyRep, policyErr := p.enrichPolicyRefEdges(ctx, svc, refs, changed)
	passes.mark("policy_ref")
	report.PolicyRefEdges, report.PolicyRefUnresolved, report.PolicyRefSkipped = policyRep.edges, policyRep.unresolved, policyRep.skipped
	if policyErr != nil {
		return finish(fmt.Errorf("fusion: policy-ref edge pass %q: %w", svc.ID, policyErr))
	}

	// Knowledge-retrieval doc units (KR-8/KR-9): additive, never blocks
	// ingest. See docindexpass.go.
	p.indexDocs(ctx, svc, refs, &report)
	passes.mark("doc_index")

	// ── Orphan purge, then the final commit ──────────────────────────────────
	// refs still holds every discovered file, unreadable ones included, so an
	// unreadable file is never mistaken for a deleted one.
	if err := writeRefs(); err != nil {
		return finish(err)
	}
	run.changed = len(changed)
	commitErr := p.finalCommit(ctx, svc, run, refs)
	passes.mark("commit") // less centrality, its own span inside it
	return finish(commitErr)
}

// finalCommit purges orphans and, when that succeeds, commits the ingest:
// the watermark and ingest_state = complete (RT-109).
func (p *Pipeline) finalCommit(ctx context.Context, svc core.ServiceRef, run ingestRun, refs []core.SourceRef) error {
	if err := p.purgeOrphans(ctx, svc.ID, refs); err != nil {
		return err
	}
	p.commitIngest(ctx, svc, run)
	return nil
}

// recordIngestMeta records the ingest metadata (freshness watermark + corpus
// path) and the git commit history for svc. Best-effort: a failure here
// never aborts the ingest. RT-109 (design §6): called only at the ingest's
// final commit (commitIngest), with the watermark read when the ingest
// began — a refused (IT-15), halted or killed ingest never advances it.
func (p *Pipeline) recordIngestMeta(ctx context.Context, svc core.ServiceRef, commit string, run *ingestRun) {
	// Record the corpus root ABSOLUTE: auto-refresh, staleness_report,
	// and the ingest MCP tool all resolve this path from the DAEMON's
	// working directory — a relative path recorded from some other CWD
	// (CLI --path "." default, sibling-repo post-commit hooks) would
	// make the daemon refresh the wrong corpus under this service ID.
	// An EMPTY path stays empty: remote/faux services have no local
	// corpus root, and Abs("") would record the ingesting process's
	// CWD — auto-refresh treats "" as "skip", which is the truth.
	metaPath := absCorpusPath(svc.Path)
	meta := store.ServiceMeta{
		ServiceID:     svc.ID,
		Path:          metaPath,
		IndexedCommit: commit,
		IndexedAt:     time.Now().UnixNano(),
	}
	// RT-76 (§6): the watermark, file_stats and service_summary in one
	// transaction. Best-effort like the meta write it replaces: if the
	// summary write fails, the watermark is still recorded (a missing
	// summary is backfilled at first use, never a wrong one served).
	sum, files := ingestSummary(metaPath, run)
	if err := p.st.WriteServiceSummary(ctx, meta, sum, files); err != nil {
		_ = p.st.SetServiceMeta(ctx, meta) // best-effort; ignore error
	}

	// Collect git commit history (best-effort; injectable gitLogFn for tests).
	if raw := p.gitLogFn(svc.Path); raw != "" {
		if cs := parseGitLog(svc.ID, raw); len(cs) > 0 {
			_ = p.st.SetCommits(ctx, svc.ID, cs)
			// TQ-1 (design doc §3 T1): churn per file, computed from the
			// SAME commit set just stored, at the SAME ingest — never a
			// separate request-time walk (I48).
			_ = p.st.SetFileChurn(ctx, svc.ID, computeFileChurn(cs, time.Now()))
		}
	}

	// TQ-2 (design doc §1, §3 T1): centrality per symbol, at the SAME
	// final commit, memoized per watermark.
	changed := 0
	if run != nil {
		changed = run.changed
	}
	p.recordCentrality(ctx, svc.ID, commit, changed)

	// CX-20 (change-and-compliance design §5 step 2): the violation count
	// per archconformance deny rule, appended as one reading at the SAME
	// final commit — the drift radar's trend is an ingest-time fact.
	p.recordViolationTrend(ctx, svc, commit, changed)
}

// deletedRefs lists the source_refs stored for serviceID that are absent
// from the just-Discovered refs — the files this ingest found deleted (or
// newly ignored), sorted (Store contract).
func (p *Pipeline) deletedRefs(ctx context.Context, serviceID string, refs []core.SourceRef) ([]string, error) {
	stored, err := p.st.ListSourceRefs(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("fusion: list source refs %q: %w", serviceID, err)
	}
	if len(stored) == 0 {
		return nil, nil
	}
	discovered := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		discovered[r.Path] = struct{}{}
	}
	var out []string
	for _, ref := range stored {
		if _, ok := discovered[ref]; !ok {
			out = append(out, ref)
		}
	}
	return out, nil
}

// purgeOrphans removes observations and fingerprints for source_refs that are
// stored for serviceID but absent from the just-Discovered set — files that
// were deleted from disk or newly ignored since the previous ingest.
//
// The purge is keyed on the Discover-set difference, never on file changes:
// the hash-gate skips unchanged files, so a change-triggered purge would never
// fire for rows whose source files simply vanished (e.g. stale agent-worktree
// copies that stop being discovered once their directory is ignored).
//
// Scope: Ingest is the pipeline's only entry point and Connector.Discover
// always returns the complete ingestible set for a service — there is no
// partial or filtered ingest — so every Ingest may safely treat undiscovered
// stored refs as orphans. It runs only after the ingest has otherwise
// succeeded; fetch, budget, and extraction failures return before the purge.
func (p *Pipeline) purgeOrphans(ctx context.Context, serviceID string, refs []core.SourceRef) error {
	stored, err := p.st.ListSourceRefs(ctx, serviceID)
	if err != nil {
		return fmt.Errorf("fusion: list source refs %q: %w", serviceID, err)
	}
	if len(stored) == 0 {
		return nil
	}
	discovered := make(map[string]struct{}, len(refs))
	for _, r := range refs {
		discovered[r.Path] = struct{}{}
	}
	// stored is sorted (Store contract), so the purge order is deterministic.
	for _, ref := range stored {
		if _, ok := discovered[ref]; ok {
			continue
		}
		if err := p.st.PurgeSource(ctx, serviceID, ref); err != nil {
			return fmt.Errorf("fusion: purge orphan %q: %w", ref, err)
		}
		// Drop the fingerprint too: if the file reappears with identical
		// content, a stale fingerprint would hash-gate it and its symbols
		// would never be re-extracted.
		if err := p.st.DeleteFingerprint(ctx, serviceID, ref); err != nil {
			return fmt.Errorf("fusion: delete orphan fingerprint %q: %w", ref, err)
		}
		// RT-108: and its recorded imports.
		if fis, ok := p.st.(store.FileImportsStore); ok {
			if err := fis.DeleteFileImports(ctx, serviceID, ref); err != nil {
				return fmt.Errorf("fusion: delete orphan file imports %q: %w", ref, err)
			}
		}
	}
	return nil
}

// serviceRefsRun accumulates one ingest's extracted routes and client calls
// (RT-89): the changed files' when the service's rows are complete, every
// readable file's when they are not (its first ingest since RT-89, after a
// purge, or a legacy rebuild) — the ingest then turns its stat gate off so
// every file is read. Pass 2 hands it each file's content while it holds
// it (add); the rows are small, the content is not kept.
type serviceRefsRun struct {
	serviceID string
	stored    store.ServiceRefs
	complete  bool
	paths     []string
	out       store.ServiceRefs
}

// beginServiceRefs reads what the store holds for the service.
func (p *Pipeline) beginServiceRefs(ctx context.Context, serviceID string) (*serviceRefsRun, error) {
	stored, complete, err := p.st.ServiceRefsFor(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("fusion: service refs %q: %w", serviceID, err)
	}
	return &serviceRefsRun{serviceID: serviceID, stored: stored, complete: complete}, nil
}

// add extracts one file's routes and client calls.
func (r *serviceRefsRun) add(path string, content []byte) {
	r.paths = append(r.paths, path)
	x := serviceref.ExtractFile(path, content)
	r.out.Routes = append(r.out.Routes, x.Routes...)
	r.out.Calls = append(r.out.Calls, x.Calls...)
}

// writer returns the replacement write, which the caller runs just before
// the final commit (§6): an ingest halted in between writes nothing. Rows
// of files no longer discovered, or no longer readable as source
// (oversize), are dropped.
//
// A file unreadable this run (a read failure, not oversize) keeps its rows,
// as pass 3 keeps its observations: when it reads again unchanged it is
// never re-extracted, so dropped rows would never return. So does a file
// the stat gate vouched for unchanged. For the same reason a first
// extraction with any unreadable file is not marked complete — the next
// ingest extracts every readable file again.
func (r *serviceRefsRun) writer(ctx context.Context, st store.Store, refs []core.SourceRef, oversize map[string]bool, anyUnreadable bool) func() error {
	files := append([]string(nil), r.paths...)
	discovered := make(map[string]bool, len(refs))
	for _, ref := range refs {
		discovered[ref.Path] = true
	}
	gone := map[string]bool{}
	dropped := func(file string) bool { return !discovered[file] || oversize[file] }
	for _, x := range r.stored.Routes {
		if dropped(x.File) {
			gone[x.File] = true
		}
	}
	for _, c := range r.stored.Calls {
		if dropped(c.File) {
			gone[c.File] = true
		}
	}
	for f := range gone {
		files = append(files, f)
	}
	sort.Strings(files)
	if len(files) == 0 && r.complete {
		return func() error { return nil }
	}
	markComplete := r.complete || !anyUnreadable
	out, serviceID := r.out, r.serviceID
	return func() error {
		if err := st.ReplaceServiceRefs(ctx, serviceID, files, out, markComplete); err != nil {
			return fmt.Errorf("fusion: write service refs %q: %w", serviceID, err)
		}
		return nil
	}
}
