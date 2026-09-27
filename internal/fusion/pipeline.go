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
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
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
	// (hash-gate hits). Skipped + Extracted + ReadFailed == Discovered
	// (ignoring budget aborts).
	Skipped int
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

	// CUE typed-edge pass counters (ADR-041 decision 5 / SU-10; zero for
	// services with no .cue sources or no cue.mod, or when
	// Config.DisableTypedEdges is set). Same package-granular replace
	// discipline as the Go counters above, over a disjoint store-key
	// namespace ("cue:"-prefixed) — see internal/fusion/typedcue.go.
	CUEPackages        int
	CUEPackagesFailed  int
	CUEEdgesDirect     int
	CUEEdgesUnresolved int

	// FileResolvedEdges is the number of core.EdgeFileResolved edges the
	// per-file TypeScript/JavaScript/Python resolver produced this run
	// (ADR-041 decision 5 / SU-11; zero when the service has none of those
	// languages, a SCIP index is present for it, or Config.DisableTypedEdges
	// is set). Always heuristic tier — see core.EdgeConfidenceOf — never
	// folded into the Typed*/CUE* counters above. See
	// internal/fusion/fileresolve.go.
	FileResolvedEdges int

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
//
// Format: \x00<hash>\x1f<author>\x1f<date>\x00<full raw body, %B>\x00,
// followed by --name-only's file list before the next record's leading
// \x00. %B — not %s — captures the FULL commit message (subject + blank
// line + body + trailers), so E2-1 widens commit ingestion beyond the old
// subject-only capture (0/763 stored messages contained a newline before
// this).
//
// The framing byte is NUL (%x00) — the ONE byte git guarantees can never
// appear in a commit message (git rejects NUL at commit time; every other
// control byte, including \x02/\x03/\x1f, is preserved verbatim). An
// earlier draft framed records with \x02/\x03 sentinels, which an attacker
// could embed in a crafted commit message to inject a forged record — a
// forged hash, forged author, and forged Co-Authored-By trailer that the
// authorship oracle would then mint a TierVerified claim for. NUL framing
// closes that: no message, author name, date, or filename can contain the
// framing byte, so record boundaries are unforgeable (see parseGitLog,
// which adds full-length hash validation as belt-and-braces).
func realGitLog(path string) string {
	if path == "" {
		return ""
	}
	var out bytes.Buffer
	cmd := exec.Command("git", "-C", path, "log",
		"--pretty=format:%x00%H%x1f%an%x1f%aI%x00%B%x00",
		"--name-only",
		"-n", "500")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return out.String()
}

// commitHashPattern validates a full git object name: 40 hex chars (SHA-1)
// or 64 (SHA-256 repos). %H always emits the full lowercase hash, so
// anything else in the hash field means a corrupt/truncated record —
// belt-and-braces on top of the unforgeable NUL framing (see realGitLog).
var commitHashPattern = regexp.MustCompile(`^[0-9a-f]{40}(?:[0-9a-f]{24})?$`)

// parseGitLog parses output of:
//
//	git log --pretty=format:%x00%H%x1f%an%x1f%aI%x00%B%x00 --name-only -n 500
//
// Raw shape per record: a leading NUL, the header (hash\x1fauthor\x1fdate),
// a NUL, the raw body (%B), a NUL, then --name-only's file list up to the
// next record's leading NUL. Splitting the whole output on NUL therefore
// yields one empty leading segment followed by positional TRIPLES of
// (header, body, files-chunk) — and because git guarantees NUL can never
// appear inside a commit message, author name, ISO date, or filename, no
// crafted commit content can shift those boundaries: bytes like \x02/\x03/
// \x1f inside a message are inert content here, not framing (the forged-
// record injection class the earlier STX/ETX framing was vulnerable to;
// TestParseGitLog_ForgedRecordInjection pins this closed). A record whose
// hash field fails commitHashPattern is skipped; a tail-truncated final
// record (fewer than three remaining segments) is dropped.
//
// Commit.Message carries the FULL raw commit message (subject, body, and
// trailers) so a downstream deterministic pass (E2-2's authorship oracle)
// can find trailers such as "Co-Authored-By:" wherever they fall.
func parseGitLog(serviceID, raw string) []store.Commit {
	if raw == "" {
		return nil
	}
	segs := strings.Split(raw, "\x00")
	var commits []store.Commit
	// segs[0] is whatever precedes the first record's leading NUL ("" in
	// well-formed output); records are the positional triples after it.
	for i := 1; i+2 < len(segs); i += 3 {
		header, body, filesChunk := segs[i], segs[i+1], segs[i+2]

		// header: hash\x1fauthor\x1fdate
		parts := strings.SplitN(header, "\x1f", 3)
		if len(parts) < 3 {
			continue
		}
		hash, author, dateStr := parts[0], parts[1], parts[2]
		if !commitHashPattern.MatchString(hash) {
			continue
		}
		// git's pretty format appends its own trailing newline after %B; the
		// body itself may or may not already end in one. Trim it so Message
		// never carries a spurious trailing blank line, while any INTERNAL
		// blank lines (paragraph breaks, the subject/body separator) survive.
		message := strings.TrimRight(body, "\n")

		var dateNano int64
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			dateNano = t.UnixNano()
		} else if t, err := time.Parse(time.RFC3339Nano, dateStr); err == nil {
			dateNano = t.UnixNano()
		}
		var files []string
		for _, l := range strings.Split(filesChunk, "\n") {
			l = strings.TrimSpace(l)
			if l != "" {
				files = append(files, l)
			}
		}
		// Scanning the full message (not just the subject) means a spec ID
		// mentioned only in the body is now found too — a strict widening of
		// the prior subject-only recall, never a narrowing.
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
// Epoch 6 (docs/plans/cpp-toolchain.md): C++ names normalize "::" to "."
// (CT-1), .h headers route through the C/C++ sniffing router (CT-3), and
// CMake files extract (CT-4) — stored observations for every .h/.cc/.cpp/
// .cxx/.hpp/.hh file predate all three, so the salt bump forces
// re-extraction.
// Epoch 7 (wave P2, O69/O79): Kotlin (.kt/.kts) now extracts via
// internal/extractor.NewTSExtractor("kotlin") — tree-sitter via wazero/WASM,
// ADR-021 §4 new-language breadth, no CGO incumbent — where every such file
// previously fell through to "no extractor registered" and was fingerprinted
// (never re-sniffed) by that dead-end path; the salt bump forces those files
// through the new extractor on the next ingest instead of leaving them
// silently stuck on a stale "unsupported" fingerprint. The same epoch bump
// also covers JavaExtractor (.java, still CGO/ADR-013) additionally emitting
// KindJavaField and KindJavaImport, which the epoch-6 extractor never
// captured — every already-indexed .java file's stored observations predate
// both new kinds.
const ExtractorEpoch = "7"

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
	// J1 (app-kit O18/O32/O33): .mjs/.cjs/.mts/.cts were classified as
	// "js"/"ts" (connector.extClassMap) but never DISPATCHED to an
	// extractor here — the actual root cause of find_symbol missing every
	// exported symbol in a new .mjs/.cjs file after ingest. Same extractor
	// instance; tsextract.go's own variantForExt picks the right grammar
	// per extension.
	p.exts[".mjs"] = js
	p.exts[".cjs"] = js
	p.exts[".mts"] = js
	p.exts[".cts"] = js

	// Java
	p.exts[".java"] = extractor.NewJavaExtractor()

	// Rust
	p.exts[".rs"] = extractor.NewRustExtractor()

	// C (.h is polyglot: the header router sniffs C++ signals and picks the
	// grammar per file — docs/plans/cpp-toolchain.md CT-3)
	c := extractor.NewCExtractor()
	p.exts[".c"] = c
	p.exts[".h"] = extractor.NewCHeaderExtractor()

	// C++
	cpp := extractor.NewCppExtractor()
	p.exts[".cc"] = cpp
	p.exts[".cpp"] = cpp
	p.exts[".cxx"] = cpp
	p.exts[".hpp"] = cpp
	p.exts[".hh"] = cpp
	p.exts[".hxx"] = cpp

	// CMake (full-basename key for CMakeLists.txt, extension for modules;
	// docs/plans/cpp-toolchain.md CT-4/CT-5)
	cmake := extractor.NewCMakeExtractor()
	p.exts["CMakeLists.txt"] = cmake
	p.exts[".cmake"] = cmake

	// C#
	p.exts[".cs"] = extractor.NewCSharpExtractor()

	// Ruby — tree-sitter via wazero/WASM (ADR-021 §4), calibration-gated (XT-4).
	rb, err := extractor.NewTSExtractor("ruby")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract ruby: %v", err))
	}
	p.exts[".rb"] = rb

	// Kotlin — tree-sitter via wazero/WASM (ADR-021 §4 new-language
	// breadth, wave P2 / O69 / O79): no CGO incumbent, so no calibration
	// gate — this closes the Kotlin extraction gap directly.
	kt, err := extractor.NewTSExtractor("kotlin")
	if err != nil {
		panic(fmt.Sprintf("fusion: tsextract kotlin: %v", err))
	}
	p.exts[".kt"] = kt
	p.exts[".kts"] = kt

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

// SetLogger routes the typed-edge pass's own diagnostics (typedcalls.go)
// through log instead of a bare fmt.Fprintf(os.Stderr, ...) (DG-4,
// diagnostics sweep). nil (the default) preserves the original os.Stderr
// behavior — every embedder that never calls this keeps working
// unchanged; cmd/fathom's daemon opts in via pkg/client.Client.WithLogger.
func (p *Pipeline) SetLogger(log *dlog.Logger) {
	p.logger = log
}

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
func (p *Pipeline) Ingest(ctx context.Context, svc core.ServiceRef) (Report, error) {
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
		if err := p.purgeOrphans(ctx, svc.ID, refs); err != nil {
			return report, err
		}
		p.commitIngest(ctx, svc, run)
		return report, nil
	}

	// ── Pass 2: Fetch ────────────────────────────────────────────────────────
	// Pre-allocate indexed result slots — one per ref. Workers write into their
	// own slot without contention. A per-file READ failure (readfailures.go)
	// stays in its slot and the file is skipped below; any other error is
	// fatal — the first one is captured via atomic pointer swap and aborts
	// before a single write.
	type fetchResult struct {
		content []byte
		err     error
	}
	fetched := make([]fetchResult, len(refs))

	// firstErr stores the first FATAL fetch error via atomic pointer swap.
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
			if fetchErr != nil && !isReadFailure(fetchErr) && !isOversize(fetchErr) {
				var nilPtr unsafe.Pointer
				heapErr := fetchErr // capture on heap
				atomic.CompareAndSwapPointer(&firstErrPtr, nilPtr, unsafe.Pointer(&heapErr))
			}
		}(i, ref)
	}
	wg.Wait()

	if firstErrPtr != nil {
		p.abandonIngestState(ctx, run) // before the first write
		return report, fmt.Errorf("fusion: fetch: %w", *(*error)(firstErrPtr))
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		p.abandonIngestState(context.WithoutCancel(ctx), run) // before the first write
		return report, fmt.Errorf("fusion: fetch: %w", ctxErr)
	}

	// Read failures (IT-14): counted, named, sorted by path — never a silent
	// zero and never fatal on their own. refs keeps every discovered file,
	// so the orphan purge below cannot mistake "unreadable" for "deleted".
	var failures []ReadFailure
	for i, ref := range refs {
		if fetched[i].err != nil && !isOversize(fetched[i].err) {
			failures = append(failures, ReadFailure{Path: ref.Path, Err: fetched[i].err.Error()})
		}
	}
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
	// index as a fresh one. Refuse before the first write. A store with no
	// evidence has nothing to protect and falls through (the failures are
	// still returned loudly). A PARTIAL wedge is not refused: the readable
	// files are honest progress and the unreadable ones keep their evidence;
	// refusing there would also make a repo with one permanently unreadable
	// file un-ingestable forever.
	if len(failures) > 0 && len(failures) == len(refs) {
		existing, qErr := p.st.QueryObservations(ctx, svc.ID, "")
		if qErr != nil {
			p.abandonIngestState(ctx, run) // before the first write
			return report, fmt.Errorf("fusion: count observations %q: %w", svc.ID, qErr)
		}
		if len(existing) > 0 {
			// Refused before the first write: the index is untouched and
			// still the one its watermark describes.
			p.abandonIngestState(ctx, run)
			return report, &ReadFailuresError{Service: svc.ID, Failures: failures, Refused: true, Existing: len(existing)}
		}
	}

	// ADR-017 self-heal: stores written before repo-relative anchors keyed
	// fingerprints and observations by absolute path. Once the connector
	// emits relative refs, those legacy rows would never match the hash-gate
	// again and every symbol would exist twice (old absolute + new relative
	// anchor). Detect the mix and rebuild this service's content wholesale,
	// once. Runs after the fetch so a wedge cannot trigger a rebuild it
	// cannot complete.
	if !filepath.IsAbs(refs[0].Path) {
		if n, cErr := p.st.CountAbsoluteSourceRefs(ctx, svc.ID); cErr == nil && n > 0 {
			if pErr := p.st.PurgeService(ctx, svc.ID); pErr != nil {
				return report, fmt.Errorf("fusion: purge legacy-anchored service %q: %w", svc.ID, pErr)
			}
			report.PurgedLegacyAnchors = true
		}
	}

	// ── Pass 3: PostFetch (sequential, input order) ──────────────────────────
	var pending []pendingFile
	var tokenAccum int64

	for i, ref := range refs {
		if over := asOversize(fetched[i].err); over != nil {
			// RT-69: over the read cap, decided from the stat — never read.
			// Fingerprinted by size so an unchanged one is skipped next
			// time; what it held when it was small is purged, since it
			// now contributes no observations.
			// Counted under Oversize on every ingest, changed or not: the
			// file stays unindexed, so the summary names it (I38) instead
			// of folding it into the unchanged-and-skipped count.
			if _, err := p.recordOversize(ctx, svc.ID, ref.Path, over.Size); err != nil {
				return report, err
			}
			report.Oversize++
			continue
		}
		if fetched[i].err != nil {
			// Unreadable this run: no hash-gate, no purge, no fingerprint —
			// the stored evidence stays until the file can be read again.
			continue
		}
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
		return finish(p.finalCommit(ctx, svc, run, refs))
	}

	stats, err := p.extractAndStore(ctx, svc.ID, pending)
	if err != nil {
		return report, err
	}
	report.Extracted = len(pending)
	report.Symbols = stats.symbols
	report.Unclassified = stats.unclassified
	report.Oversize += stats.oversize
	report.ClassifiedZero = stats.classifiedZero
	report.TSEngineFaults = stats.tsFaults
	report.UnsupportedLanguages = stats.unsupportedLang

	// Typed Go call-edge enrichment (ADR-021 §3): additive, per-package,
	// never blocks ingest. See internal/fusion/typedcalls.go.
	typedRep := p.enrichTypedEdges(ctx, svc, refs)
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
	if st, known := p.TypedEdgeStatus(svc.ID); known {
		report.TypedPackagesSeen = st.PackagesSeen
		report.TypedFailed = st.Failed
		report.TypedFailReason = st.Reason
		report.TypedFallback = st.Fallback
	}

	// CUE typed-edge enrichment (ADR-041 decision 5 / SU-10): additive,
	// per-package, never blocks ingest. See internal/fusion/typedcue.go.
	cueRep := p.enrichCUEEdges(ctx, svc, refs)
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
	fileRep, fileErr := p.enrichFileResolvedEdges(ctx, svc, refs)
	report.FileResolvedEdges = fileRep.EdgesResolved
	if fileErr != nil {
		// RT-153 (I38): halted before the final commit — the watermark
		// never moved (RT-109), so the halted run never reads as fresh; fail
		// with the named cause instead of a shorter index.
		return finish(fmt.Errorf("fusion: file-resolved edge pass %q: %w", svc.ID, fileErr))
	}

	// ── Orphan purge, then the final commit ──────────────────────────────────
	// refs still holds every discovered file, unreadable ones included, so an
	// unreadable file is never mistaken for a deleted one.
	return finish(p.finalCommit(ctx, svc, run, refs))
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
func (p *Pipeline) recordIngestMeta(ctx context.Context, svc core.ServiceRef, commit string) {
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
	_ = p.st.SetServiceMeta(ctx, meta) // best-effort; ignore error

	// Collect git commit history (best-effort; injectable gitLogFn for tests).
	if raw := p.gitLogFn(svc.Path); raw != "" {
		if cs := parseGitLog(svc.ID, raw); len(cs) > 0 {
			_ = p.st.SetCommits(ctx, svc.ID, cs)
		}
	}
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
			// No extractor registered/resolved — write fingerprint only. When
			// the extension names a known programming language fathom simply
			// has no extractor for (O69), count it by name instead of letting
			// it vanish indistinguishably among lockfiles, images, and other
			// genuinely non-code paths — see Report.UnsupportedLanguages.
			if lang, known := connector.UnsupportedLanguageForPath(pf.ref.Path); known {
				if stats.unsupportedLang == nil {
					stats.unsupportedLang = make(map[string]int)
				}
				stats.unsupportedLang[lang]++
			}
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
					EndLine:      node.EndLine,
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
