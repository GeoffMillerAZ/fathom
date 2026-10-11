package fusion

// extract_sink.go — extractAndStore and the extractSink that batches one run's extraction
// output: add (one file through its extractor and every writer), flush and the pending
// fingerprints. Split out of pipeline.go unchanged.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

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
	sink := p.newExtractSink(serviceID)
	for _, pf := range batch {
		if err := sink.add(ctx, pf); err != nil {
			return sink.stats, err
		}
	}
	return sink.stats, sink.flush(ctx)
}

// extractSink extracts one file at a time (input order) and writes in
// batches (RT-103): observations accumulate to obsBatch before one write,
// and a file's fingerprint is written only after its observations are — a
// run stopped mid-batch leaves those files unfingerprinted, so the next
// ingest re-extracts them.
type extractSink struct {
	p         *Pipeline
	serviceID string
	idx       *siblingIndex
	stats     extractStats
	obs       []core.Observation
	fps       []pendingFingerprint
	scripts   heldScripts // scriptcalls.go: scoped scripts' rows wait for the stream's end (S1 lever)
}

type pendingFingerprint struct{ path, hash string }

func (p *Pipeline) newExtractSink(serviceID string) *extractSink {
	p.mu.Lock()
	idx := p.siblingIdx[serviceID]
	p.mu.Unlock()
	return &extractSink{p: p, serviceID: serviceID, idx: idx}
}

// flush ends the stream: the waiting scripts' rows are queued
// (releaseScripts), then everything buffered is written.
func (s *extractSink) flush(ctx context.Context) error {
	if err := s.releaseScripts(ctx); err != nil {
		return err
	}
	return s.write(ctx)
}

// write writes the buffered observations, then their files' fingerprints.
func (s *extractSink) write(ctx context.Context) error {
	defer passTimesFrom(ctx).sub("store_writes")() // EW-48: the extract stream's writes
	if len(s.obs) > 0 {
		if err := s.p.st.PutObservations(ctx, s.obs); err != nil {
			return fmt.Errorf("fusion: put observations %q: %w", s.serviceID, err)
		}
		s.obs = s.obs[:0]
	}
	for _, f := range s.fps {
		if err := s.p.st.SetFingerprint(ctx, s.serviceID, f.path, f.hash); err != nil {
			return fmt.Errorf("fusion: set fingerprint %q: %w", f.path, err)
		}
	}
	s.fps = s.fps[:0]
	return nil
}

// add extracts one file and buffers its writes.
func (s *extractSink) add(ctx context.Context, pf pendingFile) error {
	p, serviceID, idx, stats := s.p, s.serviceID, s.idx, &s.stats
	// RT-178: extraction is an ingest-class producer — asked of the
	// governor before each file, like the fetch that fed it.
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassIngest); err != nil {
		return fmt.Errorf("fusion: extract %q halted before %s: %w", serviceID, pf.ref.Path, err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fusion: extract %q: %w", serviceID, err)
	}
	// Binary content never carries extractable symbols, regardless of
	// whether its path happens to match a registered extension (ADR-021
	// §1 / LT-2). Record the fingerprint so it isn't re-sniffed on every
	// ingest, but never hand binary bytes to an extractor.
	if connector.IsBinary(pf.content) {
		if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
			return fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
		}
		return nil
	}

	// Size cap (ADR-021 §2 / TI-6): huge/minified files are never handed
	// to an extractor. Still fingerprinted and counted, never silent.
	if len(pf.content) > MaxExtractSize {
		stats.oversize++
		atomic.AddInt64(&p.oversizeTotal, 1)
		if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
			return fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
		}
		return nil
	}

	// LB-F9/LB-F10 (design §5): an ambiguous extension no sniff settled,
	// or a dialect file that is generated or minified, is fingerprinted
	// and counted — never extracted, never silent.
	if pf.verdict.sniffed {
		stats.sniffed++
	}
	switch pf.verdict.excluded {
	case ExcludedSkippedGenerated, ExcludedAmbiguous:
		if pf.verdict.excluded == ExcludedSkippedGenerated {
			stats.skippedGenerated++
		} else {
			stats.ambiguous++
		}
		if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
			return fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
		}
		return nil
	}

	extExtractor, toolKind, ok, unclassified := p.resolveExtractor(pf.ref.Path, pf.content, idx)
	if unclassified {
		stats.unclassified++
		atomic.AddInt64(&p.unclassifiedTotal, 1)
		if err := p.st.SetFingerprint(ctx, serviceID, pf.ref.Path, pf.hash); err != nil {
			return fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
		}
		return nil
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
			return fmt.Errorf("fusion: set fingerprint %q: %w", pf.ref.Path, err)
		}
		return nil
	}

	var nodes []core.SymbolNode
	var extractErr error
	if ts, one := p.oneParse(extExtractor, pf.ref.Path, pf.content); one && idx != nil {
		// EW-52: a file the file-resolved pass will read is parsed once;
		// its specs wait in the ingest's memo for resolveLang.
		var sp extractor.FileSpecs
		var spOK bool
		nodes, sp, spOK, extractErr = ts.ExtractWithFileSpecs(serviceID, pf.ref.Path, pf.content)
		if extractErr == nil && spOK {
			idx.specs.put(pf.ref.Path, ts.SpecLang(), pf.hash, sp)
		}
	} else {
		nodes, extractErr = extExtractor.Extract(serviceID, pf.ref.Path, pf.content)
	}
	if extractErr != nil {
		// Non-fatal: record the fingerprint so we don't retry on next ingest,
		// but don't abort the whole pipeline. A classified file erroring
		// out is a zero-symbol outcome too — count it (critic F5).
		if toolKind != "" && toolKind != ToolUnclassified {
			p.noteClassifiedZero(stats, toolKind)
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
		return nil
	}

	// Classified-but-zero visibility (critic F5): a file the classifier
	// DID identify (e.g. a CI config whose top-level `services:` list
	// matches the compose signal) whose extractor then emits nothing is
	// the last surviving silent-zero path — count it per tool.
	if len(nodes) == 0 && toolKind != "" && toolKind != ToolUnclassified {
		p.noteClassifiedZero(stats, toolKind)
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
		s.obs = append(s.obs, obs...)
		stats.symbols += len(nodes)

		// Enrichment step: run only when enabled and an enricher is wired.
		// Files reaching this point have already passed the hash-gate (they
		// are new or changed), so no additional per-symbol hash check is
		// required here — the file-level hash is the gate.
		if p.cfg.Enrich && p.enricher != nil {
			if err := p.runEnrichment(ctx, serviceID, pf.ref.Path, nodes); err != nil {
				return err
			}
		}
	}

	// The fingerprint follows the observations it vouches for.
	s.fps = append(s.fps, pendingFingerprint{pf.ref.Path, pf.hash})
	s.holdScript(nodes)
	if len(s.obs) >= obsBatch {
		return s.write(ctx)
	}
	return nil
}
