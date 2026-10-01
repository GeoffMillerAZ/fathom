// Package fusion implements the source fusion pipeline (Layer 2). It
// discovers, fetches, hash-gates, and extracts symbols from source files
// delivered by a Connector, then writes core.Observation records to a Store.
//
// Design: 3-pass pipeline
//
//  1. Filter  — collect SourceRefs in deterministic order.
//  2. Fetch   — parallel I/O, results land in pre-allocated indexed slots.
//  3. PostFetch — sequential, input-ordered: hash-gate → budget-gate →
//     dispatch to extractor → PutObservations → SetFingerprint.
//
// After a successful ingest an orphan purge removes store rows for
// source_refs no longer present in the Discover set (see purgeOrphans).
//
// Lazy mode: when Config.Lazy is true, post-fetch enqueues pending files
// instead of extracting them. ExtractPending drains the queue on demand.
//
// Optional enrichment: when Config.Enrich is true and an *llm.Enricher is
// injected via WithEnricher, the pipeline generates KnowledgeCards for each
// new symbol and DocSummaries for Markdown/Doc observations.  Enrichment is
// hash-gated per symbol (skipped when the content hash is unchanged) and off
// by default — a nil enricher means no enrichment even when Enrich is true.
package fusion

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/docindex"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/llm"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// Config controls pipeline behaviour.
type Config struct {
	// Concurrency is the number of parallel Fetch workers (0 → 1).
	Concurrency int
	// MaxTokens is a token-budget ceiling; 0 means disabled.
	MaxTokens int64
	// MaxCostUSD is a cost-budget ceiling in USD; 0 means disabled.
	MaxCostUSD float64
	// Lazy defers extraction until ExtractPending is called.
	Lazy bool
	// Enrich enables LLM-powered enrichment after extraction. An *llm.Enricher
	// must also be wired via WithEnricher; if none is provided enrichment is
	// silently skipped even when Enrich is true.
	Enrich bool
	// DisableTypedEdges turns off the Go typed call-edge pass (ADR-021 §3)
	// entirely. Default false (the pass runs for Go services). For degraded
	// environments (no go toolchain on PATH) and for measuring a true
	// syntax-only ingest baseline (the CT-5 performance gate).
	DisableTypedEdges bool
	// InflightBytes is pass 2's window (RT-103): content bytes read and not
	// yet written. 0 → DefaultInflightBytes (64 MiB).
	InflightBytes int64
	// Rehash turns pass 1's stat gate off (RT-102): every file is read and
	// hashed (`fathom ingest --rehash`).
	Rehash bool
}

// Report summarises a completed Ingest run.
type Report struct {
	// Discovered is the number of SourceRefs returned by Connector.Discover
	// (after ignore filtering; currently all refs are kept).
	Discovered int
	// Extracted is the number of files that passed the hash-gate and were sent
	// to an extractor.
	Extracted int
	// Skipped is the number of files whose fingerprint matched the stored hash
	// (hash-gate hits), including StatSkipped. Skipped + Extracted +
	// ReadFailed + Oversize == Discovered (ignoring budget aborts).
	Skipped int
	// StatSkipped is how many of Skipped were never read (RT-102): their
	// (size, mtime, inode) and stored fingerprint matched the last ingest's.
	StatSkipped int
	// PeakInflightBytes is the most file content this ingest held at once
	// (RT-103) — bounded by Config.InflightBytes whatever the corpus.
	PeakInflightBytes int64
	// ReadFailed is the number of discovered files whose bytes the connector
	// could not read this run (readfailures.go, IT-14): recorded, named in
	// ReadFailures, skipped — their stored evidence left exactly as it was.
	ReadFailed int
	// ReadFailures names every unreadable file (repo-relative path plus the
	// connector's errno text), sorted by Path.
	ReadFailures []ReadFailure
	// Symbols is the total number of core.SymbolNode values emitted by the
	// extractor(s) and stored as observations during this run. Zero in lazy mode
	// until ExtractPending is called.
	Symbols int
	// PurgedLegacyAnchors is true when this ingest found pre-ADR-017
	// absolute-path anchors and rebuilt the service's content wholesale.
	PurgedLegacyAnchors bool
	// Unclassified is the number of yaml/cue-family files whose extension
	// matched an extractor candidate but whose content/path matched no known
	// tool (ADR-021 §2 / TI-6) — never handed to an extractor, but counted
	// instead of silently producing zero symbols.
	Unclassified int
	// UnsupportedLanguages counts, by language display name (e.g. "kotlin"),
	// files whose extension belongs to a KNOWN programming language fathom
	// has no registered extractor for (O69, wave P1 — connector.
	// UnsupportedLanguageForPath). These files hit the same "no extractor
	// registered" fallback as any other unrecognized extension — fingerprinted
	// so they aren't re-sniffed, never extracted — but unlike a generic
	// unrecognized extension (a lockfile, an image, a config format with no
	// symbols to extract), a language name is knowable here, and staying
	// silent about it is exactly the failure the Long Code Arena bench run
	// found: bumble-tech/appyx, a real Kotlin repository, indexed as
	// near-empty with no signal anywhere that Kotlin was the reason. Nil
	// when every discovered extension either has an extractor or is not a
	// known programming language.
	UnsupportedLanguages map[string]int
	// DefaultIgnoredDirs names, repo-relative and sorted, each directory the
	// connector's default cache list pruned from discovery (RT-145: .cache,
	// target, dist, build, …) — counted and named, never a silent drop. A
	// corpus re-includes one with `[connector] include`. Nil when none.
	DefaultIgnoredDirs []string
	// SkippedGenerated is the number of dialect-owned files extracted this
	// run that the generated/minified gate skipped (LB-F10, design §5): a
	// `@generated` / `Code generated … DO NOT EDIT` marker in the first
	// 2 KiB, or an average line over 500 characters in the first 64 KiB.
	// Fingerprinted, never extracted.
	SkippedGenerated int
	// AmbiguousUnclassified is the number of files whose extension more
	// than one dialect claims and no manifest sniff settled (LB-F9) —
	// fingerprinted, never guessed.
	AmbiguousUnclassified int
	// ClassifiedBySniff is the number of files a manifest sniff classified
	// (LB-F9) — disclosed, since a sniff is a content judgement.
	ClassifiedBySniff int
	// Oversize is the number of files skipped from extraction because their
	// content exceeded MaxExtractSize (ADR-021 §2 / TI-6) — still
	// fingerprinted so they are not re-sniffed on every ingest.
	Oversize int
	// ClassifiedZero is the number of files that DID classify as a known
	// tool but whose extractor emitted zero symbols (ADR-021 §2, critic
	// F5) — e.g. a CI config whose top-level `services:` list matches the
	// compose signal but not the compose shape. Without this counter the
	// silent-zero class survived through the classified path; per-tool
	// detail is on Pipeline.ClassifiedZeroStats.
	ClassifiedZero int

	// Typed Go call-edge pass counters (ADR-021 §3; zero for non-Go
	// services, or when Config.DisableTypedEdges is set). TypedPackages is
	// packages processed (loaded + cache-hit); TypedPackagesFailed is
	// packages that failed typed loading and fell back to syntax-only edges
	// — a Go service whose ingest ALWAYS reports failures here has a broken
	// typed environment (e.g. no go toolchain on the daemon's PATH), which
	// is a different situation from a non-Go service's all-zeros. Edge
	// counts cover edges written this run (cache-hit packages' stored edges
	// are not recounted).
	TypedPackages           int
	TypedPackagesFailed     int
	TypedEdgesDirect        int
	TypedEdgesIfaceDispatch int
	TypedEdgesUnresolved    int

	// Ingest honesty disclosure (wave P3 / O82, .fathom/observations.md):
	// the same TypedEdgeStatus the daemon records for staleness_report and
	// doctor, carried on THIS ingest's own Report so the ingest response
	// itself (CLI text/--json, the MCP tool) can say plainly when the
	// typed-edge pass produced nothing to trust — instead of requiring a
	// SEPARATE staleness_report/doctor call to discover it, which is
	// exactly how the 2026-09-16 push-clone incident stayed invisible
	// through an {"ok":true} ingest and a refused push.
	//
	// TypedPackagesSeen is every package go/packages returned (before any
	// were dropped by the connector-scope filter or a load failure) — 0
	// when the load never got that far. TypedFailed is true when the pass
	// could not run at all, a package failed to type-check, OR packages
	// were seen but NONE were processed (every one excluded by the
	// connector scope filter — never a legitimate non-Go-tree zero).
	// TypedFailReason names the cause (I24); empty when TypedFailed is
	// false. TypedFallback names "GOWORK=off" when an ambient Go workspace
	// was discoverable and therefore overridden (defect 2, same wave).
	TypedPackagesSeen int
	TypedFailed       bool
	TypedFailReason   string
	TypedFallback     string
	// TypedSkipped names why the typed Go pass did not run this ingest
	// (RT-104: no Go input changed since its last successful pass); the
	// stored typed edges stand. "" when it ran (or the service has no Go).
	TypedSkipped string

	// CUE typed-edge pass counters (ADR-041 decision 5 / SU-10; zero for
	// services with no .cue sources or no cue.mod, or when
	// Config.DisableTypedEdges is set). Same package-granular replace
	// discipline as the Go counters above, over a disjoint store-key
	// namespace ("cue:"-prefixed) — see internal/fusion/typedcue.go.
	CUEPackages        int
	CUEPackagesFailed  int
	CUEEdgesDirect     int
	CUEEdgesUnresolved int
	// CUESkipped is TypedSkipped for the CUE pass (RT-105).
	CUESkipped string

	// FileResolvedEdges is the number of core.EdgeFileResolved edges the
	// per-file TypeScript/JavaScript/Python resolver produced this run
	// (ADR-041 decision 5 / SU-11; zero when the service has none of those
	// languages, a SCIP index is present for it, or Config.DisableTypedEdges
	// is set). Always heuristic tier — see core.EdgeConfidenceOf — never
	// folded into the Typed*/CUE* counters above. See
	// internal/fusion/fileresolve.go.
	FileResolvedEdges int
	// RT-108: FileResolveReused counts files whose imports came from
	// file_imports (not read, not parsed); FileResolveFaults counts files
	// whose import parse faulted, FileResolveFaultFiles names up to five;
	// DocXrefEdges is the number of doc-engine cross-file reference edges
	// the doc-xref pass stored (LB-F30); DocXrefSkipped names why it did
	// not run ("" when it ran).
	DocXrefEdges   int
	DocXrefSkipped string
	// FileResolveSkipped names why the pass did not run ("" when it ran).
	FileResolveReused     int
	FileResolveFaults     int
	FileResolveFaultFiles []string
	FileResolveSkipped    string

	// TSEngineFaults is the number of files this ingest whose tree-sitter
	// wasm extraction faulted (parse deadline on pathological input, wasm
	// trap, or query-run failure — ADR-021 §4, extractor.ErrTSEngineFault).
	// Each fault also emitted one stderr line, fingerprinted the file (not
	// retried until its bytes or the ExtractorEpoch change), and re-created
	// the language's parser — so a non-zero count here with later files
	// still extracting is DEGRADED (some files unindexed), while a count
	// that grows every ingest for the same content signals those files need
	// a .fathomignore entry or a deeper engine fix.
	TSEngineFaults int

	// DocIndex is the knowledge-retrieval doc-unit pass (KR-8, KR-9;
	// docindexpass.go): the Markdown the connector discovered, split into
	// sections and written to the store's FTS index, hash-gated per file,
	// with what the per-service byte budget cut named in DocIndex.Elided
	// and any unreadable or over-bound document in DocIndex.Failures.
	// DocIndexSkipped names why the pass did not run (no local corpus root,
	// a store with no doc index) or could not finish; "" when it ran clean.
	// The pass is additive: it never fails or unwinds the code ingest.
	DocIndex        docindex.IngestDocUnitsResult
	DocIndexSkipped string
}

// Pipeline is the source fusion pipeline.
type Pipeline struct {
	conn       connector.Connector
	st         store.Store
	exts       map[string]extractor.Extractor
	toolExts   map[ToolKind]extractor.Extractor // ADR-021 §2: yaml-family extractors keyed by classifier verdict
	sniffExts  map[string]extractor.Extractor   // dialect id -> extractor, for a dialect that claims an extension only through match.sniff (LB-Q-matlab)
	sniffExt   map[string]bool                  // the lowercased extensions those sniffs settle (sniffedExtractor reads only these files)
	cfg        Config
	enricher   *llm.Enricher // nil → enrichment disabled
	mu         sync.Mutex
	queue      map[string][]pendingFile // serviceID → pending files (lazy mode)
	siblingIdx map[string]*siblingIndex // serviceID → this ingest's sibling index (ADR-021 §2 classifier context), guarded by mu
	gitHeadFn  func(path string) string // injectable; default = realGitHead
	gitLogFn   func(path string) string // injectable; default = realGitLog

	// logger is DG-4's (diagnostics sweep) seam for the typed-edge pass's
	// own diagnostics (typedcalls.go's diagWarn/diagInfo): nil by default,
	// which preserves this package's original bare fmt.Fprintf(os.Stderr,
	// ...) behavior for every embedder that never calls SetLogger — a
	// Pipeline used directly via pkg/client.New (no cmd/fathom daemon
	// wrapping it) keeps working exactly as before. cmd/fathom's
	// serve-http/serve-mcp opt in via pkg/client.Client.WithLogger so these
	// lines gain the same timestamp/level/dedupe discipline as the rest of
	// the daemon's diagnostics.
	logger *dlog.Logger

	// Process-lifetime counters (ADR-021 §2 / TI-6), surfaced via Stats() and
	// (through pkg/client + the mcp server_stats/ingest tools) to operators —
	// "silent zero-symbol extraction dies" applies to the whole process, not
	// just one Report.
	unclassifiedTotal int64 // atomic
	oversizeTotal     int64 // atomic

	// typedPassTotal counts every enrichTypedEdges call that actually ran
	// the go/packages load (O108, cmd/fathom's /metrics fathom_typed_pass_total)
	// — incremented past the hasGoSource/hasGoModule/DisableTypedEdges guard
	// in internal/fusion/typedcalls.go, so a non-Go service's ingests never
	// inflate it. Process-lifetime, like unclassifiedTotal/oversizeTotal.
	typedPassTotal int64 // atomic

	// cuePassTotal counts CUE passes that ran past the RT-105 gate.
	cuePassTotal int64 // atomic

	// classifiedZero counts, per ToolKind, files that classified as that
	// tool but whose extractor emitted zero symbols (critic F5) — the
	// remaining silent-zero class after the Unclassified counter. Guarded
	// by czMu (a map, not an atomic).
	czMu           sync.Mutex
	classifiedZero map[string]int64

	// typedEdgeStatus records, per service, the last typed-edge pass
	// outcome this process observed (BP-35, internal/fusion/typedenv.go) —
	// the fact /statusz, staleness_report and doctor's typed_edges row
	// read. Guarded by teMu.
	teMu            sync.Mutex
	typedEdgeStatus map[string]TypedEdgeStatus

	// RT-102 (stream.go): pass 1's prior stats — statSource when the store's
	// file_stats is wired, else statMemo (service → path → record, this
	// pipeline's last successful ingest; guarded by mu); statSink receives
	// each successful ingest's live stats.
	statSource func(ctx context.Context, service string) (map[string]StatRecord, bool)
	statSink   func(ctx context.Context, service string, recs []StatRecord) error
	statMemo   map[string]map[string]StatRecord
	// statOrder is statMemo's services, least recently ingested first, and
	// statRecords their record count (bounded by statMemoMax). Guarded by mu.
	statOrder   []string
	statRecords int
}

// pendingFile holds a fetched, hash-gated file waiting for extraction.
type pendingFile struct {
	ref     core.SourceRef
	content []byte
	hash    string
	verdict dialectVerdict
}

// Close releases the pipeline's tree-sitter grammar modules (RT-107). The
// pipeline stays usable: the next extraction instantiates afresh.
func (p *Pipeline) Close() {
	seen := map[*extractor.TSExtractor]bool{}
	for _, e := range p.exts {
		if ts, ok := e.(*extractor.TSExtractor); ok && !seen[ts] {
			seen[ts] = true
			ts.Close()
		}
	}
	for _, e := range p.sniffExts {
		if ts, ok := e.(*extractor.TSExtractor); ok && !seen[ts] {
			seen[ts] = true
			ts.Close()
		}
	}
}

// WithGitHead replaces the function used to resolve the git HEAD commit for a
// directory path. The replacement is called with svc.Path at ingest time and
// must return an empty string instead of an error when the commit is
// unavailable.  This is intended for unit tests; production code uses the
// default (realGitHead).
func (p *Pipeline) WithGitHead(fn func(path string) string) {
	p.gitHeadFn = fn
}

// WithGitLog replaces the function used to retrieve git commit history for a
// directory path. The replacement must return the git log output string, or ""
// when unavailable. Intended for unit tests.
func (p *Pipeline) WithGitLog(fn func(path string) string) {
	p.gitLogFn = fn
}

// WithEnricher injects an *llm.Enricher into the pipeline. Enrichment only
// runs when Config.Enrich is also true; passing nil is a no-op.
func (p *Pipeline) WithEnricher(e *llm.Enricher) {
	p.enricher = e
}

// SetLogger routes the typed-edge pass's own diagnostics (typedcalls.go)
// through log instead of a bare fmt.Fprintf(os.Stderr, ...) (DG-4,
// diagnostics sweep). nil (the default) preserves the original os.Stderr
// behavior — every embedder that never calls this keeps working
// unchanged; cmd/fathom's daemon opts in via pkg/client.Client.WithLogger.
func (p *Pipeline) SetLogger(log *dlog.Logger) {
	p.logger = log
}

// ExtractPending drains the lazy queue for serviceID, extracts all pending
// files, stores observations, and updates fingerprints. If there is nothing
// pending it returns nil immediately.
func (p *Pipeline) ExtractPending(ctx context.Context, serviceID string) error {
	p.mu.Lock()
	batch := p.queue[serviceID]
	delete(p.queue, serviceID)
	p.mu.Unlock()

	if len(batch) == 0 {
		return nil
	}

	_, err := p.extractAndStore(ctx, serviceID, batch)
	return err
}

// extractStats summarises one extractAndStore call — the per-call counterpart
// of the Pipeline's cumulative unclassifiedTotal/oversizeTotal counters.
type extractStats struct {
	symbols        int
	unclassified   int
	oversize       int
	classifiedZero int
	tsFaults       int // extract errors matching extractor.ErrTSEngineFault (critic M1)
	// skippedGenerated, ambiguous and sniffed are the dialect gate's
	// per-ingest counts (LB-F9, LB-F10) over the files extracted this run.
	skippedGenerated int
	ambiguous        int
	sniffed          int
	// unsupportedLang tallies files, by language display name, that hit the
	// no-extractor fallback but matched a known-language extension (O69,
	// connector.UnsupportedLanguageForPath) — the per-call source for
	// Report.UnsupportedLanguages. Nil until the first such file is seen.
	unsupportedLang map[string]int
}

// Stats returns the pipeline's process-lifetime unclassified/oversize counts
// (ADR-021 §2 / TI-6) — every classifier miss and every size-cap skip across
// every Ingest this Pipeline has run, regardless of Lazy mode. Surfaced via
// pkg/client and the mcp server_stats/ingest tools so "silent zero-symbol
// extraction" has a durable, process-wide counter, not just a per-call one.
func (p *Pipeline) Stats() (unclassified, oversize int64) {
	return atomic.LoadInt64(&p.unclassifiedTotal), atomic.LoadInt64(&p.oversizeTotal)
}

// TypedPassTotal returns the process-lifetime count of typed-edge passes
// this Pipeline has actually run (O108) — every enrichTypedEdges call past
// its own skip guard, regardless of whether the pass succeeded. Surfaced
// via pkg/client and cmd/fathom's /metrics as fathom_typed_pass_total: a
// daemon that never stops growing it while heap keeps climbing points at
// the typed pass as a growth site; one that stays flat rules it out.
func (p *Pipeline) TypedPassTotal() int64 {
	return atomic.LoadInt64(&p.typedPassTotal)
}

// ClassifiedZeroStats returns a copy of the process-lifetime per-tool
// classified-but-zero-symbols counters (critic F5): files that classified
// as a known tool but whose extractor emitted zero symbols. Companion to
// Stats(); surfaced via server_stats.
func (p *Pipeline) ClassifiedZeroStats() map[string]int64 {
	p.czMu.Lock()
	defer p.czMu.Unlock()
	out := make(map[string]int64, len(p.classifiedZero))
	for k, v := range p.classifiedZero {
		out[k] = v
	}
	return out
}

// noteClassifiedZero records one classified-but-zero-symbols outcome for
// tool, on both the per-call stats and the process-lifetime map.
func (p *Pipeline) noteClassifiedZero(stats *extractStats, tool ToolKind) {
	stats.classifiedZero++
	p.czMu.Lock()
	p.classifiedZero[string(tool)]++
	p.czMu.Unlock()
}

// runEnrichment generates and stores KnowledgeCards for non-doc symbols and
// a DocSummary for Markdown/doc source files.
//
// Markdown observations (KindDocChunk) are treated as documentation: the
// pipeline calls PutDocSummaries with a summary derived from all the doc chunk
// nodes' signatures (headings). All other symbol kinds are enriched
// individually via Enricher.EnrichBatch and stored as KnowledgeCards.
func (p *Pipeline) runEnrichment(ctx context.Context, serviceID, sourcePath string, nodes []core.SymbolNode) error {
	ext := filepath.Ext(sourcePath)

	// Separate doc chunks from code symbols.
	var codeNodes []core.SymbolNode
	var docNodes []core.SymbolNode
	for _, n := range nodes {
		if n.Kind == core.KindDocChunk || ext == ".md" {
			docNodes = append(docNodes, n)
		} else {
			codeNodes = append(codeNodes, n)
		}
	}

	// Enrich code symbols → KnowledgeCards.
	if len(codeNodes) > 0 {
		llmCards, err := p.enricher.EnrichBatch(ctx, codeNodes)
		if err != nil {
			return fmt.Errorf("fusion: enrich %q: %w", sourcePath, err)
		}
		storeCards := make([]store.KnowledgeCard, len(llmCards))
		for i, c := range llmCards {
			storeCards[i] = store.KnowledgeCard{
				ServiceID: serviceID,
				Symbol:    c.Symbol,
				File:      c.File,
				Summary:   c.Summary,
				Tags:      c.Tags,
			}
		}
		if err := p.st.PutKnowledgeCards(ctx, storeCards); err != nil {
			return fmt.Errorf("fusion: put knowledge cards %q: %w", sourcePath, err)
		}
	}

	// Enrich doc/Markdown files → DocSummary.
	if len(docNodes) > 0 {
		// Summarise the whole document as one unit: use the first heading as
		// title and all headings concatenated as the summary input.
		title := docNodes[0].Name
		summaryNode := core.SymbolNode{
			Service:   serviceID,
			Name:      title,
			FilePath:  sourcePath,
			Signature: joinDocHeadings(docNodes),
		}
		card, err := p.enricher.Enrich(ctx, summaryNode)
		if err != nil {
			return fmt.Errorf("fusion: enrich doc %q: %w", sourcePath, err)
		}
		summaries := []store.DocSummary{{
			ServiceID: serviceID,
			SourceRef: sourcePath,
			Title:     title,
			Summary:   card.Summary,
		}}
		if err := p.st.PutDocSummaries(ctx, summaries); err != nil {
			return fmt.Errorf("fusion: put doc summaries %q: %w", sourcePath, err)
		}
	}

	return nil
}

// joinDocHeadings concatenates the Name fields of doc-chunk nodes into a
// single space-separated string for use as a summary prompt.
func joinDocHeadings(nodes []core.SymbolNode) string {
	if len(nodes) == 0 {
		return ""
	}
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		parts[i] = n.Name
	}
	// Use a simple join; import strings avoided via manual construction.
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}
