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

// statDigest walks svc's discovered files and digests their (path, size,
// mtime) — no reads. A corpus over manifestStatCap is ErrManifestTooLarge.
func (c *LocalConnector) statDigest(ctx context.Context, svc core.ServiceRef) (string, int, error) {
	refs, err := c.Discover(ctx, svc)
	if err != nil {
		return "", 0, fmt.Errorf("watermark discover %q: %w", svc.ID, err)
	}
	if len(refs) > manifestStatCap {
		return "", len(refs), fmt.Errorf("%w (%d files, cap %d)", ErrManifestTooLarge, len(refs), manifestStatCap)
	}
	h := sha256.New()
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return "", 0, err
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
	return hex.EncodeToString(h.Sum(nil)), len(refs), nil
}

// statGatedManifest is the content manifest (ADR-017's portable, content-
// addressed watermark — what an ingest records) recomputed only when the
// corpus's stat manifest moved (RT-110): each call walks the stats (no
// reads) and answers the memoized manifest while they stand. A corpus over
// the cap is ErrManifestTooLarge (freshness unknown), remembered for
// manifestTTL so it is not re-walked on every read. This is the ingest's
// path (Watermark); the request path uses FreshnessProbe, which never
// computes a manifest.
func (c *LocalConnector) statGatedManifest(ctx context.Context, svc core.ServiceRef) (string, error) {
	now := manifestNow()
	var memo manifestMemo
	if v, ok := c.manifests.Load(svc.Path); ok {
		memo = v.(manifestMemo)
		if memo.tooLarge > 0 && now.Sub(memo.at) < manifestTTL {
			return "", fmt.Errorf("%w (%d files, cap %d)", ErrManifestTooLarge, memo.tooLarge, manifestStatCap)
		}
	}
	stat, n, err := c.statDigest(ctx, svc)
	if errors.Is(err, ErrManifestTooLarge) {
		c.manifests.Store(svc.Path, manifestMemo{at: now, tooLarge: n})
		return "", err
	}
	if err != nil {
		return "", err
	}
	c.probes.Store(svc.Path, probeMemo{at: now, stat: stat})
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

// probeTTL is how long a freshness probe's stat walk stands before the
// next probe walks the tree again (RT-110, I48): a read's freshness cost is
// at most one stat walk per root per probeTTL, never per read. Var for tests.
var probeTTL = 5 * time.Second

// statDriftPrefix marks a freshness probe's drift token: the stats moved
// since the last recorded manifest, so the corpus may have changed. It never
// equals a recorded watermark, so it reads as drift and asks for a refresh;
// the refresh's ingest computes the real manifest.
const statDriftPrefix = "stat:"

// probeMemo is one root's last stat walk made for a probe or an ingest.
type probeMemo struct {
	at       time.Time
	stat     string
	tooLarge int
}

// FreshnessProber is the optional connector capability behind the request
// path's freshness check (RT-110, I48): a token that equals the recorded
// watermark while the corpus stands and never reads the corpus to say so.
type FreshnessProber interface {
	FreshnessProbe(ctx context.Context, svc core.ServiceRef) (string, error)
}

// FreshnessProbe is the request path's freshness token for svc (RT-110,
// I48): the git HEAD inside a work tree (as Watermark), else — for a
// non-git corpus — the content manifest the last ingest recorded while the
// corpus's stats stand, or a "stat:" drift token once they moved. It never
// reads a file: the content manifest is the ingest's to compute
// (Watermark). The stat walk is cached for probeTTL; a corpus over the cap
// is ErrManifestTooLarge (freshness unknown). With no manifest recorded by
// this connector yet (a fresh process), the answer is the drift token: one
// queued refresh, off the request path, re-establishes it.
func (c *LocalConnector) FreshnessProbe(ctx context.Context, svc core.ServiceRef) (string, error) {
	head, err := gitro.Head(ctx, svc.Path)
	switch {
	case err == nil:
		return head, nil
	case !errors.Is(err, gitro.ErrNotRepository) && !errors.Is(err, gitro.ErrUnbornHead):
		// An unborn branch (no commit yet) has no HEAD to compare: its
		// freshness is its content, as for a non-git corpus.
		return "", err
	}
	now := manifestNow()
	var pm probeMemo
	if v, ok := c.probes.Load(svc.Path); ok {
		pm = v.(probeMemo)
	}
	if pm.at.IsZero() || now.Sub(pm.at) >= probeTTL {
		stat, n, err := c.statDigest(ctx, svc)
		switch {
		case errors.Is(err, ErrManifestTooLarge):
			pm = probeMemo{at: now, tooLarge: n}
		case err != nil:
			return "", err
		default:
			pm = probeMemo{at: now, stat: stat}
		}
		c.probes.Store(svc.Path, pm)
	}
	if pm.tooLarge > 0 {
		return "", fmt.Errorf("%w (%d files, cap %d)", ErrManifestTooLarge, pm.tooLarge, manifestStatCap)
	}
	if v, ok := c.manifests.Load(svc.Path); ok {
		if m := v.(manifestMemo); m.manifest != "" && m.stat == pm.stat {
			return m.manifest, nil
		}
	}
	return statDriftPrefix + pm.stat, nil
}
