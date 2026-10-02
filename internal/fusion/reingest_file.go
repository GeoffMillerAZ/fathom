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
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
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
	// Skipped names why the file was checked but not read or re-extracted
	// without an error: "oversize (N bytes; over the 2097152-byte cap; not
	// read)" (RT-27). Empty otherwise.
	Skipped string
	// Purged is true when the call removed the file's stored observations
	// without replacing them (a file that grew past the cap): the rows a
	// memoized graph was built from are gone, so a caller holding one must
	// patch or drop it exactly as after a refresh (SU-4 / O99).
	Purged bool
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
	// RT-145: the one-file refresh sees the corpus an ingest sees — a file
	// Discover would not walk is not indexed here only to be purged as an
	// orphan by the next ingest. Rows an earlier refresh left for it are
	// purged now (Purged tells the caller to drop its graph memo).
	if why, excluded, xerr := connector.ExcludedFromCorpus(repoPath, serviceID, relFile); xerr != nil {
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile corpus check %q: %w", relFile, xerr)
	} else if excluded {
		rep := ReingestReport{Skipped: "outside the corpus: " + why}
		if _, found, ferr := st.Fingerprint(ctx, serviceID, relFile); ferr == nil && found {
			if err := newPassInvalidator(st, serviceID).note(ctx, relFile); err != nil {
				return ReingestReport{}, err
			}
			if err := st.PurgeSource(ctx, serviceID, relFile); err != nil {
				return ReingestReport{}, fmt.Errorf("fusion: ReingestFile purge %q: %w", relFile, err)
			}
			if err := st.DeleteFingerprint(ctx, serviceID, relFile); err != nil {
				return ReingestReport{}, fmt.Errorf("fusion: ReingestFile delete fingerprint %q: %w", relFile, err)
			}
			if err := replaceFileServiceRefs(ctx, st, serviceID, relFile, nil); err != nil {
				return ReingestReport{}, err
			}
			rep.Purged = true
		}
		return rep, nil
	}
	// RT-27: stat before reading. A file over MaxExtractSize is never
	// opened: it is fingerprinted by size (as an ingest would) and reported
	// skipped, not failed.
	fi, err := os.Stat(absPath)
	if err != nil {
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile stat %q: %w", relFile, err)
	}
	if fi.Size() > MaxExtractSize {
		return p.reingestOversize(ctx, st, serviceID, relFile, fi.Size())
	}
	content, err := readCapped(absPath, MaxExtractSize)
	if err != nil {
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile read %q: %w", relFile, err)
	}
	if int64(len(content)) > MaxExtractSize {
		return p.reingestOversize(ctx, st, serviceID, relFile, int64(len(content)))
	}
	hash := FingerprintHash(content)

	// Design §7.2: this file's new fingerprint makes it look unchanged to
	// the next whole ingest, so the passes it is an input of lose their
	// success record first and that ingest runs them (TypedEdgesStale says
	// the same to the caller now).
	stored, found, err := st.Fingerprint(ctx, serviceID, relFile)
	if err != nil {
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile fingerprint %q: %w", relFile, err)
	}
	if !found || stored != hash {
		if err := newPassInvalidator(st, serviceID).note(ctx, relFile); err != nil {
			return ReingestReport{}, err
		}
	}

	// RT-89: this call moves the file's fingerprint, so the next ingest
	// hash-gates it — its routes and client calls are replaced here too,
	// or the rows from before the edit would outlive it.
	if err := replaceFileServiceRefs(ctx, st, serviceID, relFile, content); err != nil {
		return ReingestReport{}, err
	}

	if connector.IsBinary(content) {
		if err := st.SetFingerprint(ctx, serviceID, relFile, hash); err != nil {
			return ReingestReport{}, fmt.Errorf("fusion: ReingestFile set fingerprint %q: %w", relFile, err)
		}
		return ReingestReport{}, nil
	}

	// LB-F9/LB-F10: the same dialect gate an ingest applies — a generated,
	// minified or ambiguous dialect file loses its old observations and is
	// fingerprinted, never extracted.
	if gateDialectFile(relFile, content).excluded != "" {
		if err := st.PurgeSource(ctx, serviceID, relFile); err != nil {
			return ReingestReport{}, fmt.Errorf("fusion: ReingestFile purge %q: %w", relFile, err)
		}
		if err := st.SetFingerprint(ctx, serviceID, relFile, hash); err != nil {
			return ReingestReport{}, fmt.Errorf("fusion: ReingestFile set fingerprint %q: %w", relFile, err)
		}
		return ReingestReport{}, nil
	}

	base := filepath.Base(relFile)
	ext := filepath.Ext(relFile)
	extExtractor, ok := p.exts[base]
	if ce, _, cok := p.cloudPolicyDispatch(relFile, content, nil); cok {
		// IAM policies, Azure Policy and Cloud Custodian files (LB-F138): the decision resolveExtractor makes.
		extExtractor, ok = ce, true
	}
	if ge, _, gok := gatewayDispatch(relFile, content); gok {
		// Envoy, Kong and Traefik configuration (LB-F104): the decision
		// resolveExtractor makes, a manifest name AND the content sniff.
		extExtractor, ok = ge, true
	}
	if !ok && (ext == ".yml" || ext == ".yaml") && connector.ClassifyDialect(dialectRules(), relFile, content).Dialect == "gitlab-ci" {
		// A pipeline file named only by its path (.gitlab/ci/…, *.gitlab-ci.yml)
		// reads through the gitlab-ci dialect, as ingest's classifier sends it
		// there, not through the generic YAML extractor the extension names.
		extExtractor, ok = p.toolExts[ToolGitLabCI]
	}
	if !ok && isCloudFormation(relFile, content, nil) {
		// A CloudFormation template (LB-Q-cloudformation) is read by its
		// content, as ingest's dispatch reads it, not by the YAML or JSON
		// extractor its extension names.
		extExtractor, ok = p.toolExts[ToolCloudFormation]
	}
	if !ok && isOpenAPI(relFile, content, nil) {
		// An OpenAPI or Swagger document (LB-Q-openapi), YAML or JSON, is read
		// by its content as ingest's dispatch reads it.
		extExtractor, ok = p.toolExts[ToolOpenAPI]
	}
	if !ok {
		extExtractor, ok = p.exts[ext]
	}
	if !ok {
		// A dialect that owns the extension only through match.sniff
		// (MATLAB's .m): ingest's dispatch, resolveExtractor, asks the
		// same classifier.
		extExtractor, ok = p.sniffedExtractor(relFile, content)
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

// readCapped reads at most limit+1 bytes of path: a file that grew past the
// cap after its stat is still bounded, and reads as oversize.
func readCapped(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit+1))
}

// reingestOversize records an oversize file the way an ingest does
// (OversizeFingerprint; its old observations purged when the fingerprint
// moves) without reading it.
func (p *Pipeline) reingestOversize(ctx context.Context, st store.Store, serviceID, relFile string, size int64) (ReingestReport, error) {
	rep := ReingestReport{Skipped: fmt.Sprintf("oversize (%d bytes; over the %d-byte cap; not read)", size, MaxExtractSize)}
	fp := OversizeFingerprint(size)
	stored, found, err := st.Fingerprint(ctx, serviceID, relFile)
	if err != nil {
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile fingerprint %q: %w", relFile, err)
	}
	if found && stored == fp {
		return rep, nil
	}
	if err := newPassInvalidator(st, serviceID).note(ctx, relFile); err != nil {
		return ReingestReport{}, err
	}
	if found {
		if err := st.PurgeSource(ctx, serviceID, relFile); err != nil {
			return ReingestReport{}, fmt.Errorf("fusion: ReingestFile purge %q: %w", relFile, err)
		}
		rep.Purged = true
	}
	if err := replaceFileServiceRefs(ctx, st, serviceID, relFile, nil); err != nil {
		return ReingestReport{}, err
	}
	if err := st.SetFingerprint(ctx, serviceID, relFile, fp); err != nil {
		return ReingestReport{}, fmt.Errorf("fusion: ReingestFile set fingerprint %q: %w", relFile, err)
	}
	return rep, nil
}

// replaceFileServiceRefs replaces relFile's extracted routes and outbound
// HTTP client calls (RT-89) with content's — none for nil content (a file
// oversize or outside the corpus). It never marks the service's rows
// complete: one file cannot stand in for a service's first extraction.
func replaceFileServiceRefs(ctx context.Context, st store.Store, serviceID, relFile string, content []byte) error {
	var refs store.ServiceRefs
	if content != nil {
		refs = serviceref.ExtractFile(relFile, content)
	}
	if err := st.ReplaceServiceRefs(ctx, serviceID, []string{relFile}, refs, false); err != nil {
		return fmt.Errorf("fusion: ReingestFile service refs %q: %w", relFile, err)
	}
	return nil
}
