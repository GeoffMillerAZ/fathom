package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/gitro"
)

// manifestWatermarkPrefix marks a content-addressed (non-git) watermark, so a
// stored watermark is self-describing wherever the store travels (ADR-017).
// Git-backed watermarks stay bare SHAs for back-compat with stores that
// recorded IndexedCommit before the Watermark SPI existed.
const manifestWatermarkPrefix = "manifest:"

// ManifestWatermarkPrefix exports manifestWatermarkPrefix for callers that
// must recognize a stored manifest: watermark (e.g. to decide it must never
// be compared against a live git HEAD — see ManifestWatermark's doc
// comment) without needing their own copy of the literal.
const ManifestWatermarkPrefix = manifestWatermarkPrefix

// gitHead returns the HEAD commit for dir, or "" when dir is not inside a git
// work tree or HEAD cannot be read. RT-101: from the ref files, no fork.
func gitHead(dir string) string {
	sha, err := gitro.Head(context.Background(), dir)
	if err != nil {
		return ""
	}
	return sha
}

// manifestWatermark computes a content-addressed watermark over a corpus: the
// sha256 of every discovered file's (relative path, content hash) pair, in
// Discover order. Equal watermarks ⇔ identical corpus content, independent of
// machine, directory location, or mtimes — the portability property that
// claim freshness depends on (ADR-017). Cost is one full read of the corpus;
// it is intended for the non-git corpora (docs folders, findings exports)
// where corpora are small. Git-backed corpora use the HEAD SHA instead.
func manifestWatermark(ctx context.Context, c Connector, svc core.ServiceRef) (string, error) {
	refs, err := c.Discover(ctx, svc)
	if err != nil {
		return "", fmt.Errorf("watermark discover %q: %w", svc.ID, err)
	}
	h := sha256.New()
	for _, ref := range refs {
		data, err := c.Fetch(ctx, ref)
		var over *OversizeError
		if errors.As(err, &over) {
			// RT-69: a file over the read cap is never read; its size
			// stands in for its content hash.
			fmt.Fprintf(h, "%s\x00oversize:%d\n", ref.Path, over.Size)
			continue
		}
		if err != nil {
			return "", fmt.Errorf("watermark fetch %q: %w", ref.Path, err)
		}
		fh := sha256.Sum256(data)
		fmt.Fprintf(h, "%s\x00%s\n", ref.Path, hex.EncodeToString(fh[:]))
	}
	return manifestWatermarkPrefix + hex.EncodeToString(h.Sum(nil)), nil
}

// ManifestWatermark exports manifestWatermark for callers that must
// recompute a corpus's CONTENT-ADDRESSED watermark specifically —
// regardless of whether svc.Path happens to sit inside a git work tree.
// A bare Connector.Watermark call (LocalConnector's own implementation)
// checks git FIRST and only falls back to the manifest algorithm when no
// git repo is found; that is the wrong choice for a caller re-checking a
// corpus whose STORED watermark already carries the manifest: prefix (it
// was compiled as non-git) but is now served from a location that happens
// to be nested inside an unrelated ENCLOSING git repository (e.g. a docs
// folder inside a checkout, or a module store expanded into a repo-internal
// temp dir) — LocalConnector.Watermark would silently switch to that
// enclosing repo's HEAD, comparing an apples-to-oranges value against the
// manifest watermark forever (docs/plans/gke-module-host.md's module-host
// staleness layer, GKE-MH-6 critic finding 3). Callers that know a
// watermark was recorded with the manifest: prefix should call this
// directly instead of Connector.Watermark.
func ManifestWatermark(ctx context.Context, c Connector, svc core.ServiceRef) (string, error) {
	return manifestWatermark(ctx, c, svc)
}
