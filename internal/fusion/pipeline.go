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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/llm"
	"github.com/geoffmilleraz/fathom/internal/sdd"
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
	// PurgedLegacyAnchors is true when this ingest found pre-ADR-017
	// absolute-path anchors and rebuilt the service's content wholesale.
	PurgedLegacyAnchors bool
	// Unclassified is the number of yaml/cue-family files whose extension
	// matched an extractor candidate but whose content/path matched no known
	// tool (ADR-021 §2 / TI-6) — never handed to an extractor, but counted
	// instead of silently producing zero symbols.
	Unclassified int
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
}

// Pipeline is the source fusion pipeline.
type Pipeline struct {
	conn       connector.Connector
	st         store.Store
	exts       map[string]extractor.Extractor
	toolExts   map[ToolKind]extractor.Extractor // ADR-021 §2: yaml-family extractors keyed by classifier verdict
	cfg        Config
	enricher   *llm.Enricher // nil → enrichment disabled
	mu         sync.Mutex
	queue      map[string][]pendingFile // serviceID → pending files (lazy mode)
	siblingIdx map[string]*siblingIndex // serviceID → this ingest's sibling index (ADR-021 §2 classifier context), guarded by mu
	gitHeadFn  func(path string) string // injectable; default = realGitHead
	gitLogFn   func(path string) string // injectable; default = realGitLog

	// Process-lifetime counters (ADR-021 §2 / TI-6), surfaced via Stats() and
	// (through pkg/client + the mcp server_stats/ingest tools) to operators —
	// "silent zero-symbol extraction dies" applies to the whole process, not
	// just one Report.
	unclassifiedTotal int64 // atomic
	oversizeTotal     int64 // atomic

	// classifiedZero counts, per ToolKind, files that classified as that
	// tool but whose extractor emitted zero symbols (critic F5) — the
	// remaining silent-zero class after the Unclassified counter. Guarded
	// by czMu (a map, not an atomic).
	czMu           sync.Mutex
	classifiedZero map[string]int64
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

// ExtractorEpoch salts every file fingerprint (the pass-3 hash-gate). Bump it
// in any commit that changes EXTRACTION SEMANTICS (new call-edge forms,
// symbol naming, kind mappings …): the salt change invalidates all stored
// fingerprints, so the next ingest of every service re-extracts every file.
// Without it, hash-gated files keep observations produced by the OLD
// extractor forever — the stale-observation class dogfood O9 caught (the H1
// callgraph/extractor fixes were invisible on the fleet store until forced).
// Epoch 2 = the H1 extractor generation (aliased-selector canonicalization,
// constructor-scoped receiver inference) + this salt's introduction.
// Epoch 3 = the LT-4 CUE extractor v2 generation (docs/plans/lang-truth.md,
// ADR-021 §1): nested-field recursion emitting dotted names
// (#Layer.budgets.max_usd_per_run), package-clause attribution (was
// filename-derived), and import capture. Every already-indexed .cue file's
// stored observations were produced by the OLD top-level-only extractor, so
// the salt bump forces re-extraction on the next ingest rather than leaving
// them silently stale (the exact class dogfood O9 caught for epoch 2).
// Epoch 4 = the ADR-021 §2 tool-identity classifier generation
// (docs/plans/toolident.md): Helm templates/*.yaml now parse via
// placeholder substitution instead of emitting nothing; kustomization.yaml
// emits resource/patch/image symbols instead of nothing (no metadata.name);
// arbitrary compose files and OpenAPI/CRD documents are content-classified
// instead of silently zero-output through the k8s extractor;
// HelmValuesExtractor recurses into nested keys (dotted names) instead of
// top-level-only. Every already-indexed yaml/cue file's stored observations
// were produced by the OLD path/extension-only dispatch, so the salt bump
// forces re-extraction rather than leaving them silently stale.
// Epoch 5 = the ADR-021 §4 tree-sitter engine migration
// (docs/plans/extraction-treesitter.md): Python, JavaScript/TypeScript/JSX/
// TSX, and Ruby now extract via internal/extractor.NewTSExtractor — tree-
// sitter's own C runtime + grammar, compiled to a self-contained wasm32-wasi
// module and executed by the pure-Go wazero runtime (internal/extractor/
// tsengine) — instead of the CGO tree-sitter bindings (ADR-013). Symbol
// naming/qualification is generic containment-based post-processing over
// query captures rather than per-language hand-rolled tree walks; every
// already-indexed .py/.js/.jsx/.ts/.tsx/.rb file's stored observations were
// produced by the OLD CGO extractor, so the salt bump forces re-extraction.
const ExtractorEpoch = "5"

// FingerprintHash is the pass-3 hash-gate formula: sha256 of the raw file
// bytes salted with ExtractorEpoch, hex-encoded. Every reader that compares
// against stored fingerprints (e.g. Client.ObligationsIndexStale) MUST use
// this function — a plain content hash never matches a stored fingerprint.
func FingerprintHash(content []byte) string {
	sum := sha256.Sum256(append(content, []byte("\x00fathom-extractor-epoch:"+ExtractorEpoch)...))
	return hex.EncodeToString(sum[:])
}

// New constructs a Pipeline with extractors pre-registered for all supported
// file types. Additional extractors can be added or replaced by calling
// RegisterExtractor after construction.
func New(conn connector.Connector, st store.Store, cfg Config) *Pipeline {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	p := &Pipeline{
		conn:           conn,
		st:             st,
		cfg:            cfg,
		exts:           map[string]extractor.Extractor{},
		toolExts:       map[ToolKind]extractor.Extractor{},
		queue:          map[string][]pendingFile{},
		siblingIdx:     map[string]*siblingIndex{},
		classifiedZero: map[string]int64{},
		gitHeadFn:      realGitHead,
		gitLogFn:       realGitLog,
	}

	// Go
	p.exts[".go"] = extractor.NewGoExtractor()

	// Python — tree-sitter via wazero/WASM (ADR-021 §4), calibration-gated
	// against the incumbent CGO extractor (docs/plans/extraction-treesitter.md
	// XT-3); see PROVENANCE.md and the plan's recorded scores.
	py, err := extractor.NewTSExtractor("python")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract python: %v", err))
	}
	p.exts[".py"] = py

	// JavaScript / TypeScript / JSX / TSX — tree-sitter via wazero/WASM
	// (ADR-021 §4), calibration-gated (XT-2).
	js, err := extractor.NewTSExtractor("javascript")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract javascript: %v", err))
	}
	p.exts[".js"] = js
	p.exts[".jsx"] = js
	p.exts[".ts"] = js
	p.exts[".tsx"] = js

	// Java
	p.exts[".java"] = extractor.NewJavaExtractor()

	// Rust
	p.exts[".rs"] = extractor.NewRustExtractor()

	// C
	c := extractor.NewCExtractor()
	p.exts[".c"] = c
	p.exts[".h"] = c

	// C++
	cpp := extractor.NewCppExtractor()
	p.exts[".cc"] = cpp
	p.exts[".cpp"] = cpp
	p.exts[".cxx"] = cpp
	p.exts[".hpp"] = cpp
	p.exts[".hh"] = cpp

	// C#
	p.exts[".cs"] = extractor.NewCSharpExtractor()

	// Ruby — tree-sitter via wazero/WASM (ADR-021 §4), calibration-gated (XT-4).
	rb, err := extractor.NewTSExtractor("ruby")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract ruby: %v", err))
	}
	p.exts[".rb"] = rb

	// Terraform HCL
	p.exts[".tf"] = extractor.NewHCLExtractor()

	// YAML: route .github/workflows/* to GHA extractor, everything else to K8s.
	k8s := extractor.NewK8sExtractor()
	gha := extractor.NewGHAExtractor()
	yamlExt := extractor.NewYAMLExtractor(k8s, gha)
	p.exts[".yaml"] = yamlExt
	p.exts[".yml"] = yamlExt

	// CUE
	p.exts[".cue"] = extractor.NewCUEExtractor()

	// Markdown
	p.exts[".md"] = extractor.NewMarkdownExtractor()

	// Templates
	p.exts[".templ"] = extractor.NewTemplExtractor()

	// go.mod (matched by full basename via Ext-fallback in extractAndStore)
	p.exts["go.mod"] = extractor.NewGoModExtractor()

	// Helm (full-basename keys take precedence over .yaml k8s extractor)
	p.exts["Chart.yaml"] = extractor.NewHelmChartExtractor()
	p.exts["values.yaml"] = extractor.NewHelmValuesExtractor()
	p.exts[".tpl"] = extractor.NewHelmTemplateExtractor()

	// Dockerfile (full-basename keys take precedence over extension dispatch)
	p.exts["Dockerfile"] = extractor.NewDockerfileExtractor()
	p.exts[".dockerfile"] = extractor.NewDockerfileExtractor()

	// Docker Compose (full-basename keys take precedence over .yaml k8s extractor)
	compose := extractor.NewComposeExtractor()
	p.exts["docker-compose.yml"] = compose
	p.exts["docker-compose.yaml"] = compose
	p.exts["compose.yml"] = compose
	p.exts["compose.yaml"] = compose

	// Kustomize (full-basename keys take precedence over .yaml k8s extractor
	// — ADR-021 §2 / TI-3: kustomization.yaml is valid k8s-shaped YAML but
	// (almost) never has metadata.name, so the generic k8s extractor
	// silently produced zero symbols for it)
	kustomize := extractor.NewKustomizeExtractor()
	p.exts["kustomization.yaml"] = kustomize
	p.exts["kustomization.yml"] = kustomize

	// ToolKind-keyed extractors (ADR-021 §2): dispatched via the pass-3
	// classifier (classifyYAML) for .yaml/.yml files whose basename is NOT
	// one of the special keys above and which are not under
	// .github/workflows/ (GHA keeps its existing, unchanged dispatch via
	// yamlExt). CRD and generic k8s manifests share the same extractor —
	// the classifier only tags CRDs distinctly for visibility.
	p.toolExts[ToolK8sManifest] = k8s
	p.toolExts[ToolK8sCRD] = k8s
	p.toolExts[ToolCompose] = compose
	p.toolExts[ToolHelmTemplateYAML] = extractor.NewHelmTemplateYAMLExtractor()
	p.toolExts[ToolOpenAPI] = extractor.NewOpenAPIExtractor()

	// Protocol Buffers
	p.exts[".proto"] = extractor.NewProtoExtractor()

	// GraphQL
	gql := extractor.NewGraphQLExtractor()
	p.exts[".graphql"] = gql
	p.exts[".gql"] = gql

	// TOML
	p.exts[".toml"] = extractor.NewTOMLExtractor()

	// SQL (DDL)
	p.exts[".sql"] = extractor.NewSQLExtractor()

	// JSON
	p.exts[".json"] = extractor.NewJSONExtractor()

	// Scala
	scala := extractor.NewScalaExtractor()
	p.exts[".scala"] = scala
	p.exts[".sc"] = scala

	// PHP
	p.exts[".php"] = extractor.NewPhpExtractor()

	// Bash
	bash := extractor.NewBashExtractor()
	p.exts[".sh"] = bash
	p.exts[".bash"] = bash

	// Lua
	p.exts[".lua"] = extractor.NewLuaExtractor()

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
//
// Orphan purge: after a successful ingest, source_refs stored for svc that no
// longer appear in the Discover set (deleted or newly ignored files) have
// their observations and fingerprints removed. See purgeOrphans.
func (p *Pipeline) Ingest(ctx context.Context, svc core.ServiceRef) (Report, error) {
	// ── Pass 1: Filter ──────────────────────────────────────────────────────
	refs, err := p.conn.Discover(ctx, svc)
	if err != nil {
		return Report{}, fmt.Errorf("fusion: discover %q: %w", svc.ID, err)
	}

	var report Report
	report.Discovered = len(refs)

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

	// ADR-017 self-heal: stores written before repo-relative anchors keyed
	// fingerprints and observations by absolute path. Once the connector
	// emits relative refs, those legacy rows would never match the hash-gate
	// again and every symbol would exist twice (old absolute + new relative
	// anchor). Detect the mix and rebuild this service's content wholesale,
	// once.
	if len(refs) > 0 && !filepath.IsAbs(refs[0].Path) {
		if n, cErr := p.st.CountAbsoluteSourceRefs(ctx, svc.ID); cErr == nil && n > 0 {
			if pErr := p.st.PurgeService(ctx, svc.ID); pErr != nil {
				return report, fmt.Errorf("fusion: purge legacy-anchored service %q: %w", svc.ID, pErr)
			}
			report.PurgedLegacyAnchors = true
		}
	}

	// Record ingest metadata (freshness watermark + path) for this service.
	// Git-backed corpora record the HEAD SHA; non-git corpora fall back to
	// the connector's watermark (content-addressed manifest hash — ADR-017:
	// freshness is domain-neutral, so docs folders and findings exports get
	// honest drift detection too). Best-effort: a failure here does not abort
	// the ingest.
	{
		commit := p.gitHeadFn(svc.Path)
		if commit == "" {
			if wm, wmErr := p.conn.Watermark(ctx, svc); wmErr == nil {
				commit = wm
			}
		}
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
		// Still a full-service ingest: every stored ref is now an orphan.
		if err := p.purgeOrphans(ctx, svc.ID, refs); err != nil {
			return report, err
		}
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

		// Epoch-salted content hash (see FingerprintHash / ExtractorEpoch):
		// an extractor-semantics change must invalidate every stored
		// fingerprint, or improved extraction never reaches unchanged files —
		// the stale-observation class dogfood O9 caught.
		hash := FingerprintHash(content)

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
		// Lazy mode: enqueue and return without extracting. The orphan purge
		// still runs — it depends only on the Discover set vs the store, and
		// every pending file is in the Discover set, so deferred extraction is
		// unaffected.
		p.mu.Lock()
		p.queue[svc.ID] = append(p.queue[svc.ID], pending...)
		p.mu.Unlock()
		if err := p.purgeOrphans(ctx, svc.ID, refs); err != nil {
			return report, err
		}
		return report, nil
	}

	stats, err := p.extractAndStore(ctx, svc.ID, pending)
	if err != nil {
		return report, err
	}
	report.Extracted = len(pending)
	report.Symbols = stats.symbols
	report.Unclassified = stats.unclassified
	report.Oversize = stats.oversize
	report.ClassifiedZero = stats.classifiedZero
	report.TSEngineFaults = stats.tsFaults

	// Typed Go call-edge enrichment (ADR-021 §3): additive, per-package,
	// never blocks ingest. See internal/fusion/typedcalls.go.
	typedRep := p.enrichTypedEdges(ctx, svc, refs)
	report.TypedPackages = typedRep.PackagesLoaded + typedRep.PackagesCached
	report.TypedPackagesFailed = typedRep.PackagesFailed
	report.TypedEdgesDirect = typedRep.EdgesDirect
	report.TypedEdgesIfaceDispatch = typedRep.EdgesIfaceDispatch
	report.TypedEdgesUnresolved = typedRep.EdgesUnresolved

	// ── Orphan purge ─────────────────────────────────────────────────────────
	if err := p.purgeOrphans(ctx, svc.ID, refs); err != nil {
		return report, err
	}
	return report, nil
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
	}
	return nil
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
}

// Stats returns the pipeline's process-lifetime unclassified/oversize counts
// (ADR-021 §2 / TI-6) — every classifier miss and every size-cap skip across
// every Ingest this Pipeline has run, regardless of Lazy mode. Surfaced via
// pkg/client and the mcp server_stats/ingest tools so "silent zero-symbol
// extraction" has a durable, process-wide counter, not just a per-call one.
func (p *Pipeline) Stats() (unclassified, oversize int64) {
	return atomic.LoadInt64(&p.unclassifiedTotal), atomic.LoadInt64(&p.oversizeTotal)
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

// extractAndStore runs the extractor for each file in batch (in input order
// for full determinism), converts resulting SymbolNodes to Observations, calls
// PutObservations, and records fingerprints. It returns per-call stats
// (symbol count, unclassified count, oversize count).
//
// Dispatch (ADR-021 §2): binary and oversize content are excluded before any
// extractor runs. For .yaml/.yml files not resolved unambiguously by
// basename (Chart.yaml, values.yaml, kustomization.*, compose.*,
// docker-compose.*) and not under .github/workflows/, the pass-3 classifier
// (classifyYAML) decides the ToolKind and therefore the extractor; a
// ToolUnclassified verdict skips extraction (counted) rather than falling
// through to the k8s extractor for a silent zero-symbol result. .cue files
// always use the same CUEExtractor; classifyCUE only decides the ToolKind
// TAG recorded on the resulting observations. Every other extension keeps
// the original basename-then-extension lookup, unchanged.
func (p *Pipeline) extractAndStore(ctx context.Context, serviceID string, batch []pendingFile) (extractStats, error) {
	var stats extractStats

	p.mu.Lock()
	idx := p.siblingIdx[serviceID]
	p.mu.Unlock()

	// Process file-by-file in batch (input) order for determinism.
	for _, pf := range batch {
		// Binary content never carries extractable symbols, regardless of
		// whether its path happens to match a registered extension (ADR-021
		// §1 / LT-2). Record the fingerprint so it isn't re-sniffed on every
		// ingest, but never hand binary bytes to an extractor.
		if connector.IsBinary(pf.content) {
			if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
				return stats, fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
			}
			continue
		}

		// Size cap (ADR-021 §2 / TI-6): huge/minified files are never handed
		// to an extractor. Still fingerprinted and counted, never silent.
		if len(pf.content) > MaxExtractSize {
			stats.oversize++
			atomic.AddInt64(&p.oversizeTotal, 1)
			if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
				return stats, fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
			}
			continue
		}

		base := filepath.Base(pf.ref.Path)
		ext := filepath.Ext(pf.ref.Path)

		var extExtractor extractor.Extractor
		var toolKind ToolKind
		ok := false

		switch {
		case ext == ".yaml" || ext == ".yml":
			switch {
			case strings.Contains(filepath.ToSlash(pf.ref.Path), ".github/workflows/"):
				// GHA dispatch was never the grounded defect — keep the
				// existing YAMLExtractor wrapper unchanged.
				extExtractor, ok = p.exts[ext]
				toolKind = ToolGHA
			default:
				if e, hit := p.exts[base]; hit {
					// Already unambiguous by basename (Chart.yaml,
					// values.yaml, kustomization.*, compose.*,
					// docker-compose.*) — no content sniff needed.
					extExtractor, ok = e, true
					toolKind = toolKindForBasename(base)
				} else {
					kind := Classify(pf.ref.Path, pf.content, idx)
					toolKind = kind
					if kind == ToolUnclassified {
						stats.unclassified++
						atomic.AddInt64(&p.unclassifiedTotal, 1)
						if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
							return stats, fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
						}
						continue
					}
					extExtractor, ok = p.toolExts[kind]
				}
			}
		case ext == ".cue":
			// Classify never changes which extractor runs for .cue files —
			// only the ToolKind tag recorded on the resulting observations.
			extExtractor, ok = p.exts[ext]
			toolKind = Classify(pf.ref.Path, pf.content, idx)
		default:
			// Unchanged fast path: try full basename first (e.g. "go.mod"),
			// then extension.
			extExtractor, ok = p.exts[base]
			if !ok {
				extExtractor, ok = p.exts[ext]
			}
		}

		if !ok {
			// No extractor registered/resolved — write fingerprint only.
			if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
				return stats, fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
			}
			continue
		}

		nodes, extractErr := extExtractor.Extract(serviceID, pf.ref.Path, pf.content)
		if extractErr != nil {
			// Non-fatal: record the fingerprint so we don't retry on next ingest,
			// but don't abort the whole pipeline. A classified file erroring
			// out is a zero-symbol outcome too — count it (critic F5).
			if toolKind != "" && toolKind != ToolUnclassified {
				p.noteClassifiedZero(&stats, toolKind)
			}
			// Tree-sitter engine faults (critic M1) are counted per ingest
			// on Report.TSEngineFaults and get one stderr line each — never
			// a silent skip. The extractor has already dropped its poisoned
			// parser; the next file of the language rebuilds it.
			if errors.Is(extractErr, extractor.ErrTSEngineFault) {
				stats.tsFaults++
				fmt.Fprintf(os.Stderr, "fathom: fusion: tree-sitter engine fault on %s/%s (file fingerprinted, parser re-created): %v\n",
					serviceID, pf.ref.Path, extractErr)
			}
			_ = p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash)
			continue
		}

		// Classified-but-zero visibility (critic F5): a file the classifier
		// DID identify (e.g. a CI config whose top-level `services:` list
		// matches the compose signal) whose extractor then emits nothing is
		// the last surviving silent-zero path — count it per tool.
		if len(nodes) == 0 && toolKind != "" && toolKind != ToolUnclassified {
			p.noteClassifiedZero(&stats, toolKind)
		}

		// Cross-refs, cheap tier (ADR-021 §2 / TI-7): safe to call
		// unconditionally — a no-op when idx has no Dockerfiles or nodes
		// carries no image-bearing kind.
		if len(nodes) > 0 {
			nodes = append(nodes, crossRefImageToDockerfile(serviceID, pf.ref.Path, nodes, idx)...)
		}

		if len(nodes) > 0 {
			generated := connector.IsGenerated(pf.ref.Path, pf.content)
			obs := make([]core.Observation, 0, len(nodes))
			now := time.Now().UTC()
			for _, node := range nodes {
				// Tool identity: an extractor's per-symbol verdict (set for
				// multi-document formats where identity is per-document,
				// e.g. a CRD beside a Deployment in one YAML stream —
				// critic P15) overrides the classifier's per-file verdict.
				nodeToolKind := string(toolKind)
				if node.ToolKind != "" {
					nodeToolKind = node.ToolKind
				}
				obs = append(obs, core.Observation{
					ServiceID:    serviceID,
					SourceRef:    pf.ref.Path,
					SourceKind:   node.Kind,
					EvidenceTier: node.Tier,
					Symbol:       node.Name,
					Package:      node.Package,
					Claim:        node.Signature,
					ContentHash:  pf.hash,
					Calls:        node.Calls,
					Complexity:   node.Complexity,
					Line:         node.Line,
					IngestedAt:   now,
					ToolKind:     nodeToolKind,
					Generated:    generated,
				})
			}
			if err := p.st.PutObservations(ctx, obs); err != nil {
				return stats, fmt.Errorf("fusion: put observations %q: %w", pf.ref.Path, err)
			}
			stats.symbols += len(nodes)

			// Enrichment step: run only when enabled and an enricher is wired.
			// Files reaching this point have already passed the hash-gate (they
			// are new or changed), so no additional per-symbol hash check is
			// required here — the file-level hash is the gate.
			if p.cfg.Enrich && p.enricher != nil {
				if err := p.runEnrichment(ctx, serviceID, pf.ref.Path, nodes); err != nil {
					return stats, err
				}
			}
		}

		if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
			return stats, fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
		}
	}

	return stats, nil
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
