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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/geoffmilleraz/sextant/internal/connector"
	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/extractor"
	"github.com/geoffmilleraz/sextant/internal/llm"
	"github.com/geoffmilleraz/sextant/internal/sdd"
	"github.com/geoffmilleraz/sextant/internal/store"
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
	// (hash-gate hits). Skipped + Extracted == Discovered (ignoring budget aborts).
	Skipped int
	// Symbols is the total number of core.SymbolNode values emitted by the
	// extractor(s) and stored as observations during this run. Zero in lazy mode
	// until ExtractPending is called.
	Symbols int
}

// Pipeline is the source fusion pipeline.
type Pipeline struct {
	conn      connector.Connector
	st        store.Store
	exts      map[string]extractor.Extractor
	cfg       Config
	enricher  *llm.Enricher // nil → enrichment disabled
	mu        sync.Mutex
	queue     map[string][]pendingFile // serviceID → pending files (lazy mode)
	gitHeadFn func(path string) string // injectable; default = realGitHead
	gitLogFn  func(path string) string // injectable; default = realGitLog
}

// pendingFile holds a fetched, hash-gated file waiting for extraction.
type pendingFile struct {
	ref     core.SourceRef
	content []byte
	hash    string
}

// realGitHead shells out to git to resolve HEAD for the given directory path.
// Returns an empty string when git is unavailable, the directory is not a repo,
// or any other error occurs.  Never panics, never returns an error.
func realGitHead(path string) string {
	if path == "" {
		return ""
	}
	var out bytes.Buffer
	cmd := exec.Command("git", "-C", path, "rev-parse", "HEAD")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// realGitLog shells out to git to get the last 500 commits for the given
// directory path. Returns an empty string on any error (best-effort).
func realGitLog(path string) string {
	if path == "" {
		return ""
	}
	var out bytes.Buffer
	cmd := exec.Command("git", "-C", path, "log",
		"--pretty=format:%H\x1f%an\x1f%aI\x1f%s",
		"--name-only",
		"-n", "500")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return out.String()
}

// parseGitLog parses output of:
//
//	git log --pretty=format:%H%x1f%an%x1f%aI%x1f%s --name-only -n 500
//
// Each commit block: one header line (%H\x1f%an\x1f%aI\x1f%s) + file lines + blank separator.
func parseGitLog(serviceID, raw string) []store.Commit {
	if raw == "" {
		return nil
	}
	var commits []store.Commit
	blocks := strings.Split(raw, "\n\n")
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		if len(lines) == 0 {
			continue
		}
		// First line: hash\x1fauthor\x1fdate\x1fsubject
		parts := strings.SplitN(lines[0], "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		hash, author, dateStr, message := parts[0], parts[1], parts[2], parts[3]
		if hash == "" {
			continue
		}
		var dateNano int64
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			dateNano = t.UnixNano()
		} else if t, err := time.Parse(time.RFC3339Nano, dateStr); err == nil {
			dateNano = t.UnixNano()
		}
		var files []string
		for _, l := range lines[1:] {
			l = strings.TrimSpace(l)
			if l != "" {
				files = append(files, l)
			}
		}
		specIDs, _ := sdd.ExtractRefsFromText(message, "")
		commits = append(commits, store.Commit{
			ServiceID: serviceID,
			Hash:      hash,
			Author:    author,
			Date:      dateNano,
			Message:   message,
			Files:     files,
			SpecIDs:   specIDs,
		})
	}
	return commits
}

// New constructs a Pipeline with extractors pre-registered for all supported
// file types. Additional extractors can be added or replaced by calling
// RegisterExtractor after construction.
func New(conn connector.Connector, st store.Store, cfg Config) *Pipeline {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	p := &Pipeline{
		conn:      conn,
		st:        st,
		cfg:       cfg,
		exts:      map[string]extractor.Extractor{},
		queue:     map[string][]pendingFile{},
		gitHeadFn: realGitHead,
		gitLogFn:  realGitLog,
	}

	// Go
	p.exts[".go"] = extractor.NewGoExtractor()

	// Python
	py := extractor.NewPythonExtractor()
	p.exts[".py"] = py

	// JavaScript / TypeScript / JSX / TSX
	js := extractor.NewJSExtractor()
	p.exts[".js"] = js
	p.exts[".jsx"] = js
	p.exts[".ts"] = js
	p.exts[".tsx"] = js

	// Java
	p.exts[".java"] = extractor.NewJavaExtractor()

	// Ruby
	p.exts[".rb"] = extractor.NewRubyExtractor()

	// Terraform HCL
	p.exts[".tf"] = extractor.NewHCLExtractor()

	// Kubernetes YAML
	k8s := extractor.NewK8sExtractor()
	p.exts[".yaml"] = k8s
	p.exts[".yml"] = k8s

	// CUE
	p.exts[".cue"] = extractor.NewCUEExtractor()

	// Markdown
	p.exts[".md"] = extractor.NewMarkdownExtractor()

	// Templates
	p.exts[".templ"] = extractor.NewTemplExtractor()

	// go.mod (matched by full basename via Ext-fallback in extractAndStore)
	p.exts["go.mod"] = extractor.NewGoModExtractor()

	return p
}

// RegisterExtractor registers an extractor for the given file extension
// (including leading dot, e.g. ".py"). It replaces any existing extractor for
// that extension.
func (p *Pipeline) RegisterExtractor(ext string, e extractor.Extractor) {
	p.exts[ext] = e
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

// Ingest discovers, fetches, and (unless Lazy) extracts all files for svc.
//
// Pass 1 — Filter: discover all SourceRefs; input order is already
// deterministic (FauxConnector sorts by path).
//
// Pass 2 — Fetch: up to cfg.Concurrency goroutines fetch file content in
// parallel. Each worker writes into a pre-allocated slot in the fetched slice
// so no locks are needed during fetch. The first fetch error is captured via
// atomic pointer; all goroutines finish before Ingest returns.
//
// Pass 3 — PostFetch (sequential, input order): sha256 each file, compare
// against stored fingerprint. Skip unchanged files. On change: PurgeSource
// then mark pending. Honour MaxTokens / MaxCostUSD budget gates.
//
// Dispatch: pending files are grouped by extension and sent to the registered
// extractor. Each core.SymbolNode is mapped to a core.Observation and passed
// to store.PutObservations; store.SetFingerprint is called for every processed
// file.
func (p *Pipeline) Ingest(ctx context.Context, svc core.ServiceRef) (Report, error) {
	// ── Pass 1: Filter ──────────────────────────────────────────────────────
	refs, err := p.conn.Discover(ctx, svc)
	if err != nil {
		return Report{}, fmt.Errorf("fusion: discover %q: %w", svc.ID, err)
	}

	var report Report
	report.Discovered = len(refs)

	// Record ingest metadata (git HEAD + path) for this service.  This is
	// best-effort: a failure here does not abort the ingest.
	{
		commit := p.gitHeadFn(svc.Path)
		meta := store.ServiceMeta{
			ServiceID:     svc.ID,
			Path:          svc.Path,
			IndexedCommit: commit,
			IndexedAt:     time.Now().UnixNano(),
		}
		_ = p.st.SetServiceMeta(ctx, meta) // best-effort; ignore error
	}

	// Collect git commit history (best-effort; injectable gitLogFn for tests).
	{
		raw := p.gitLogFn(svc.Path)
		if raw != "" {
			if cs := parseGitLog(svc.ID, raw); len(cs) > 0 {
				_ = p.st.SetCommits(ctx, svc.ID, cs)
			}
		}
	}

	if len(refs) == 0 {
		return report, nil
	}

	// ── Pass 2: Fetch ────────────────────────────────────────────────────────
	// Pre-allocate indexed result slots — one per ref. Workers write into their
	// own slot without contention.
	type fetchResult struct {
		content []byte
		err     error
	}
	fetched := make([]fetchResult, len(refs))

	// firstErr stores the first fetch error via atomic pointer swap.
	// Using unsafe.Pointer so we can do a compare-and-swap on an error interface.
	var firstErrPtr unsafe.Pointer // *error

	sem := make(chan struct{}, p.cfg.Concurrency)
	var wg sync.WaitGroup
	for i, ref := range refs {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, r core.SourceRef) {
			defer wg.Done()
			defer func() { <-sem }()

			content, fetchErr := p.conn.Fetch(ctx, r)
			fetched[idx] = fetchResult{content: content, err: fetchErr}
			if fetchErr != nil {
				var nilPtr unsafe.Pointer
				heapErr := fetchErr // capture on heap
				atomic.CompareAndSwapPointer(&firstErrPtr, nilPtr, unsafe.Pointer(&heapErr))
			}
		}(i, ref)
	}
	wg.Wait()

	if firstErrPtr != nil {
		return report, fmt.Errorf("fusion: fetch: %w", *(*error)(firstErrPtr))
	}

	// ── Pass 3: PostFetch (sequential, input order) ──────────────────────────
	var pending []pendingFile
	var tokenAccum int64

	for i, ref := range refs {
		content := fetched[i].content

		// SHA-256 hash of the raw file content.
		sum := sha256.Sum256(content)
		hash := hex.EncodeToString(sum[:])

		// Hash-gate: compare against stored fingerprint.
		storedHash, found, fpErr := p.st.Fingerprint(ctx, svc.ID, ref.Path)
		if fpErr != nil {
			return report, fmt.Errorf("fusion: fingerprint lookup %q: %w", ref.Path, fpErr)
		}
		if found && storedHash == hash {
			// File unchanged — skip.
			report.Skipped++
			continue
		}

		// File is new or changed: purge stale observations first.
		if found {
			if purgeErr := p.st.PurgeSource(ctx, svc.ID, ref.Path); purgeErr != nil {
				return report, fmt.Errorf("fusion: purge %q: %w", ref.Path, purgeErr)
			}
		}

		// Budget gate (token count heuristic: bytes / 4).
		if p.cfg.MaxTokens > 0 {
			tokenAccum += int64(len(content)) / 4
			if tokenAccum > p.cfg.MaxTokens {
				return report, fmt.Errorf("fusion: token budget exceeded at %q (%d tokens)",
					ref.Path, tokenAccum)
			}
		}
		if p.cfg.MaxCostUSD > 0 {
			// Cost estimate: $0.00003 per 1000 tokens (ballpark).
			costEst := float64(tokenAccum) / 1000.0 * 0.00003
			if costEst > p.cfg.MaxCostUSD {
				return report, fmt.Errorf("fusion: cost budget exceeded at %q ($%.6f)",
					ref.Path, costEst)
			}
		}

		pending = append(pending, pendingFile{ref: ref, content: content, hash: hash})
	}

	// ── Dispatch ─────────────────────────────────────────────────────────────
	if p.cfg.Lazy {
		// Lazy mode: enqueue and return without extracting.
		p.mu.Lock()
		p.queue[svc.ID] = append(p.queue[svc.ID], pending...)
		p.mu.Unlock()
		return report, nil
	}

	symbols, err := p.extractAndStore(ctx, svc.ID, pending)
	if err != nil {
		return report, err
	}
	report.Extracted = len(pending)
	report.Symbols = symbols
	return report, nil
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

// extractAndStore runs the extractor for each file in batch (in input order
// for full determinism), converts resulting SymbolNodes to Observations, calls
// PutObservations, and records fingerprints. It returns the total symbol count.
func (p *Pipeline) extractAndStore(ctx context.Context, serviceID string, batch []pendingFile) (int, error) {
	totalSymbols := 0

	// Process file-by-file in batch (input) order for determinism.
	for _, pf := range batch {
		// Resolve extractor: try full basename first (e.g. "go.mod"), then extension.
		base := filepath.Base(pf.ref.Path)
		ext := filepath.Ext(pf.ref.Path)
		extExtractor, ok := p.exts[base]
		if !ok {
			extExtractor, ok = p.exts[ext]
		}
		if !ok {
			// No extractor registered for this extension — write fingerprint only.
			if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
				return totalSymbols, fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
			}
			continue
		}

		nodes, extractErr := extExtractor.Extract(serviceID, pf.ref.Path, pf.content)
		if extractErr != nil {
			// Non-fatal: record the fingerprint so we don't retry on next ingest,
			// but don't abort the whole pipeline.
			_ = p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash)
			continue
		}

		if len(nodes) > 0 {
			obs := make([]core.Observation, 0, len(nodes))
			now := time.Now().UTC()
			for _, node := range nodes {
				obs = append(obs, core.Observation{
					ServiceID:    serviceID,
					SourceRef:    pf.ref.Path,
					SourceKind:   node.Kind,
					EvidenceTier: node.Tier,
					Symbol:       node.Name,
					Claim:        node.Signature,
					ContentHash:  pf.hash,
					Calls:        node.Calls,
					Complexity:   node.Complexity,
					Line:         node.Line,
					IngestedAt:   now,
				})
			}
			if err := p.st.PutObservations(ctx, obs); err != nil {
				return totalSymbols, fmt.Errorf("fusion: put observations %q: %w", pf.ref.Path, err)
			}
			totalSymbols += len(nodes)

			// Enrichment step: run only when enabled and an enricher is wired.
			// Files reaching this point have already passed the hash-gate (they
			// are new or changed), so no additional per-symbol hash check is
			// required here — the file-level hash is the gate.
			if p.cfg.Enrich && p.enricher != nil {
				if err := p.runEnrichment(ctx, serviceID, pf.ref.Path, nodes); err != nil {
					return totalSymbols, err
				}
			}
		}

		if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
			return totalSymbols, fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
		}
	}

	return totalSymbols, nil
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
