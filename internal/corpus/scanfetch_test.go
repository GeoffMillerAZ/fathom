package corpus

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// RT-71 (§4.6): the request path's only tree read is a budgeted stream —
// ScanFetch hands one file at a time to fn and stops at the byte, file and
// hit bounds, naming why; a missing file is skipped, an oversize one named.
func TestScanFetch_StreamsUnderBudget(t *testing.T) {
	files := map[string]string{"a": "aaaa", "b": "bbbb", "c": "cccc", "gone": ""}
	fetch := func(_ context.Context, p string) ([]byte, error) {
		if p == "gone" {
			return nil, fs.ErrNotExist
		}
		if p == "huge" {
			return nil, &connector.OversizeError{Path: p, Size: 9 << 20, Cap: 2 << 20}
		}
		return []byte(files[p]), nil
	}
	var seen []string
	cov, err := ScanFetch(context.Background(), []string{"a", "gone", "huge", "b", "c"}, Budget{Bytes: 8}, fetch, func(p string, _ []byte) bool {
		seen = append(seen, p)
		return false
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || cov.Stopped != StopBytes || !cov.LowerBound() || cov.Scanned != 2 || cov.Total != 5 {
		t.Fatalf("seen=%v cov=%+v, want a,b then a bytes stop", seen, cov)
	}
	if len(cov.Excluded) != 1 || cov.Excluded[0].Path != "huge" || cov.Excluded[0].Reason != ReasonOversize {
		t.Fatalf("excluded = %+v, want huge oversize", cov.Excluded)
	}
	cov, _ = ScanFetch(context.Background(), []string{"a", "b", "c"}, Budget{}, fetch, func(p string, _ []byte) bool { return p == "b" })
	if cov.Stopped != StopHits || cov.Scanned != 2 {
		t.Fatalf("hit stop cov = %+v", cov)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ScanFetch(ctx, []string{"a"}, Budget{}, fetch, func(string, []byte) bool { return false }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled scan err = %v", err)
	}
}
