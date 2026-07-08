package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// manifestWatermarkPrefix marks a content-addressed (non-git) watermark, so a
// stored watermark is self-describing wherever the store travels (ADR-017).
// Git-backed watermarks stay bare SHAs for back-compat with stores that
// recorded IndexedCommit before the Watermark SPI existed.
const manifestWatermarkPrefix = "manifest:"

// gitHead returns the HEAD commit for dir, or "" when dir is not inside a git
// work tree (or git is unavailable).
func gitHead(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
		if err != nil {
			return "", fmt.Errorf("watermark fetch %q: %w", ref.Path, err)
		}
		fh := sha256.Sum256(data)
		fmt.Fprintf(h, "%s\x00%s\n", ref.Path, hex.EncodeToString(fh[:]))
	}
	return manifestWatermarkPrefix + hex.EncodeToString(h.Sum(nil)), nil
}
