package fusion

// oversize.go — RT-69 (docs/design/responsible-tenant.md §4.2): a file over
// the connector's read cap arrives as *connector.OversizeError, decided
// from a stat. It is neither a read failure nor fatal: the ingest counts it
// (Report.Oversize, I25/I38) and fingerprints it by size.

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// OversizeFingerprint is the stored fingerprint for a file never read
// because it is over the cap. It cannot collide with FingerprintHash (hex).
func OversizeFingerprint(size int64) string {
	return fmt.Sprintf("oversize:%d", size)
}

func asOversize(err error) *connector.OversizeError {
	var over *connector.OversizeError
	if err != nil && errors.As(err, &over) {
		return over
	}
	return nil
}

func isOversize(err error) bool { return asOversize(err) != nil }

// recordOversize applies the hash-gate to an oversize file by its size
// fingerprint. skipped is true when the stored fingerprint already names
// the same size; otherwise any observations it held are purged and the
// size fingerprint is recorded.
func (p *Pipeline) recordOversize(ctx context.Context, serviceID, ref string, size int64) (skipped bool, err error) {
	fp := OversizeFingerprint(size)
	stored, found, err := p.st.Fingerprint(ctx, serviceID, ref)
	if err != nil {
		return false, fmt.Errorf("fusion: fingerprint lookup %q: %w", ref, err)
	}
	// Counted whether or not it changed: the process-lifetime oversize
	// counter (the MCP ingest tool's before/after delta) names every
	// never-read file an ingest met, like Report.Oversize.
	atomic.AddInt64(&p.oversizeTotal, 1)
	if found && stored == fp {
		return true, nil
	}
	if found {
		if err := p.st.PurgeSource(ctx, serviceID, ref); err != nil {
			return false, fmt.Errorf("fusion: purge %q: %w", ref, err)
		}
	}
	if err := p.st.SetFingerprint(ctx, serviceID, ref, fp); err != nil {
		return false, fmt.Errorf("fusion: set fingerprint %q: %w", ref, err)
	}
	return false, nil
}
