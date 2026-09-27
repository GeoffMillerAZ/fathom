package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
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

// manifestWatermark computes a content-addressed watermark over a corpus: the
// sha256 of every discovered file's (relative path, content hash) pair, in
// Discover order. Equal watermarks ⇔ identical corpus content, independent of
// machine, directory location, or mtimes — the portability property that
// claim freshness depends on (ADR-017). Cost is one full read of the corpus;
// it is intended for the non-git corpora (docs folders, findings exports)
// where corpora are small. Git-backed corpora use the HEAD SHA instead.
func manifestWatermark(ctx context.Context, c Connector, svc core.ServiceRef) (string, error) {
	manifestComputations.Add(1)
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

// manifestComputations counts content-manifest computations (whole-corpus
// reads) in this process — RT-110's instrument.
var manifestComputations atomic.Int64

// ManifestComputations is the number of content-manifest computations —
// each one a read of every file of a corpus — this process has made.
func ManifestComputations() int64 { return manifestComputations.Load() }

// ErrManifestTooLarge is Watermark's answer for a non-git corpus with more
// files than the stat manifest's cap: freshness unknown, never a
// whole-corpus read on the request path.
var ErrManifestTooLarge = errors.New("connector: non-git corpus over the stat-manifest cap — freshness unknown")

// manifestStatCap bounds the files a non-git corpus's stat manifest covers
// (RT-110); manifestTTL is how long an over-the-cap verdict stands before
// the tree is walked again (an ordinary answer re-walks the stats every
// time, so a content change is seen at once). Vars for tests.
var (
	manifestStatCap = 20000
	manifestTTL     = 30 * time.Second
	manifestNow     = time.Now
)

// manifestMemo is one root's memoized watermark.
type manifestMemo struct {
	at       time.Time
	stat     string // digest of (path, size, mtime) over the discovered files
	manifest string // the content manifest computed for those stats
	tooLarge int    // > 0: the file count that exceeded the cap at `at`
}

// statGatedManifest is the content manifest (ADR-017's portable, content-
// addressed watermark — what an ingest records) recomputed only when the
// corpus's stat manifest moved (RT-110): each call walks the stats (no
// reads) and answers the memoized manifest while they stand. A corpus over
// the cap is ErrManifestTooLarge (freshness unknown), remembered for
// manifestTTL so it is not re-walked on every read.
func (c *LocalConnector) statGatedManifest(ctx context.Context, svc core.ServiceRef) (string, error) {
	now := manifestNow()
	var memo manifestMemo
	if v, ok := c.manifests.Load(svc.Path); ok {
		memo = v.(manifestMemo)
		if memo.tooLarge > 0 && now.Sub(memo.at) < manifestTTL {
			return "", fmt.Errorf("%w (%d files, cap %d)", ErrManifestTooLarge, memo.tooLarge, manifestStatCap)
		}
	}
	refs, err := c.Discover(ctx, svc)
	if err != nil {
		return "", fmt.Errorf("watermark discover %q: %w", svc.ID, err)
	}
	if len(refs) > manifestStatCap {
		c.manifests.Store(svc.Path, manifestMemo{at: now, tooLarge: len(refs)})
		return "", fmt.Errorf("%w (%d files, cap %d)", ErrManifestTooLarge, len(refs), manifestStatCap)
	}
	h := sha256.New()
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		p := ref.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(svc.Path, filepath.FromSlash(p))
		}
		fi, serr := os.Stat(p)
		if serr != nil {
			fmt.Fprintf(h, "%s\x00missing\n", ref.Path)
			continue
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\n", ref.Path, fi.Size(), fi.ModTime().UnixNano())
	}
	stat := hex.EncodeToString(h.Sum(nil))
	if memo.manifest != "" && memo.stat == stat {
		memo.at = now
		c.manifests.Store(svc.Path, memo)
		return memo.manifest, nil
	}
	manifest, err := manifestWatermark(ctx, c, svc)
	if err != nil {
		return "", err
	}
	c.manifests.Store(svc.Path, manifestMemo{at: now, stat: stat, manifest: manifest})
	return manifest, nil
}
