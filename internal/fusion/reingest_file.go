package fusion

// reingest_file.go — SU-5 (docs/plans/substrate.md §A, ADR-041 decisions 3
// and 4): the daemon's per-file freshness primitive. The pre-edit hook
// (SU-6, cmd/fathom) asks the daemon to refresh ONE file when its on-disk
// content hash differs from the index; ReingestFile is what answers that —
// re-extract just that file, replace its observations (PurgeSource then
// PutObservations, the same replace idiom every changed-file re-ingest
// already uses) and fingerprint, and report what happened. Typed edges are
// deliberately left untouched (ADR-041 decision 4: "typed edges stay
// package-granular until the next ingest") — a full `fathom ingest` (or
// the next scheduled one) is what makes them exact again; this path only
// ever answers "is the SYMBOL/SYNTAX picture for this file current".
//
// Scope (documented, not silent): unlike Ingest/extractAndStore, this path
// does not run the YAML/CUE classifier or the Dockerfile cross-ref pass —
// it resolves an extractor the same way the DEFAULT case there does (exact
// basename, then extension), which is every registered language and
// config extractor except the handful requiring sibling-file classification
// (compose/kustomize/Chart-family YAML, GitHub Actions YAML). Those still
// get a correct fingerprint-only skip (Refreshed:false) rather than a
// wrong classification — never silently misclassified.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// ReingestReport is ReingestFile's result.
type ReingestReport struct {
	// Refreshed is true when the file was actually re-extracted (its
	// observations and fingerprint were replaced). False for a binary,
	// oversize, or no-extractor file — the fingerprint is still recorded
	// for those (so a caller can tell "checked, nothing to extract" apart
	// from "never looked"), but Symbols is always 0 and TypedEdgesStale is
	// always false.
	Refreshed bool
	// Symbols is how many symbols the fresh extraction produced.
	Symbols int
	// TypedEdgesStale is true when this file's language has typed edges at
	// all (currently: Go) and Refreshed is true — meaning the store's
	// typed_edges rows for this file's package were computed against the
	// PREVIOUS symbol set and may no longer match. Always false when
	// Refreshed is false.
	TypedEdgesStale bool
}

// ReingestFile re-extracts relFile (repo-relative, forward-slash) under
// repoPath with p's registered extractor, replaces its observations and
// fingerprint in st, and leaves typed edges exactly as they are. serviceID
// is the observation/fingerprint scope; st is passed explicitly (rather
// than using p's own store) so a caller can target a store other than the
// one p was constructed with — the daemon's per-request store handle,
// say — without needing a fresh Pipeline per call.
func (p *Pipeline) ReingestFile(ctx context.Context, st store.Store, serviceID, repoPath, relFile string) (ReingestReport, error) {
	absPath := filepath.Join(repoPath, filepath.FromSlash(relFile))
	content, err := os.ReadFile(absPath)
	if err != nil {
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile read %q: %w", relFile, err)
	}
	hash := FingerprintHash(content)

	if connector.IsBinary(content) || len(content) > MaxExtractSize {
		if err := st.SetFingerprint(ctx, serviceID, relFile, hash); err != nil {
			return ReingestReport{}, fmt.Errorf("fusion: ReingestFile set fingerprint %q: %w", relFile, err)
		}
		return ReingestReport{}, nil
	}

	base := filepath.Base(relFile)
	ext := filepath.Ext(relFile)
	extExtractor, ok := p.exts[base]
	if !ok {
		extExtractor, ok = p.exts[ext]
	}
	if !ok {
		if err := st.SetFingerprint(ctx, serviceID, relFile, hash); err != nil {
			return ReingestReport{}, fmt.Errorf("fusion: ReingestFile set fingerprint %q: %w", relFile, err)
		}
		return ReingestReport{}, nil
	}

	nodes, extractErr := extExtractor.Extract(serviceID, relFile, content)
	if extractErr != nil {
		_ = st.SetFingerprint(ctx, serviceID, relFile, hash)
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile extract %q: %w", relFile, extractErr)
	}

	// Replace idiom (store.go's PutObservations doc comment): purge this
	// file's old observations immediately before writing the new ones.
	if err := st.PurgeSource(ctx, serviceID, relFile); err != nil {
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile purge %q: %w", relFile, err)
	}

	if len(nodes) > 0 {
		generated := connector.IsGenerated(relFile, content)
		now := time.Now().UTC()
		obs := make([]core.Observation, 0, len(nodes))
		for _, node := range nodes {
			obs = append(obs, core.Observation{
				ServiceID:    serviceID,
				SourceRef:    relFile,
				SourceKind:   node.Kind,
				EvidenceTier: node.Tier,
				Symbol:       node.Name,
				Package:      node.Package,
				Claim:        node.Signature,
				ContentHash:  hash,
				Calls:        node.Calls,
				Complexity:   node.Complexity,
				Line:         node.Line,
				EndLine:      node.EndLine,
				IngestedAt:   now,
				ToolKind:     node.ToolKind,
				Generated:    generated,
			})
		}
		if err := st.PutObservations(ctx, obs); err != nil {
			return ReingestReport{}, fmt.Errorf("fusion: ReingestFile put observations %q: %w", relFile, err)
		}
	}

	if err := st.SetFingerprint(ctx, serviceID, relFile, hash); err != nil {
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile set fingerprint %q: %w", relFile, err)
	}

	return ReingestReport{
		Refreshed:       true,
		Symbols:         len(nodes),
		TypedEdgesStale: ext == ".go",
	}, nil
}
