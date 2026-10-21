package connector

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// Frozen independent oracle: the original fetchCapped implementation from the
// qualified H3 source. It deliberately retains the original ReadAll algorithm.
func legacyFetchCapped(ctx context.Context, path string, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() > limit {
		return nil, &OversizeError{Path: path, Size: fi.Size(), Cap: limit}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &OversizeError{Path: path, Size: int64(len(data)), Cap: limit}
	}
	return data, nil
}

func TestCappedFetchFileDifferential(t *testing.T) {
	for _, size := range []int{0, 1, 511, 512, 8192, 1 << 20, MaxFetchSize, MaxFetchSize + 1} {
		path := filepath.Join(t.TempDir(), "input")
		data := bytes.Repeat([]byte{0, 255, 'x', '\n'}, (size+3)/4)[:size]
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		want, oldErr := legacyFetchCapped(context.Background(), path, MaxFetchSize)
		got, newErr := fetchCapped(context.Background(), path, MaxFetchSize)
		if !reflect.DeepEqual(got, want) || !sameFetchError(oldErr, newErr) {
			t.Fatalf("size=%d: bytes/errors differ: old=%d/%v new=%d/%v", size, len(want), oldErr, len(got), newErr)
		}
		if newErr == nil && !bytes.Equal(got, data) {
			t.Fatalf("size=%d: delivered bytes changed", size)
		}
	}
	missing := filepath.Join(t.TempDir(), "absent")
	_, oldErr := legacyFetchCapped(context.Background(), missing, MaxFetchSize)
	_, newErr := fetchCapped(context.Background(), missing, MaxFetchSize)
	if !errors.Is(oldErr, os.ErrNotExist) || !errors.Is(newErr, os.ErrNotExist) {
		t.Fatalf("missing-file semantics changed: %v, %v", oldErr, newErr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, oldErr = legacyFetchCapped(ctx, missing, MaxFetchSize)
	_, newErr = fetchCapped(ctx, missing, MaxFetchSize)
	if !errors.Is(oldErr, context.Canceled) || !errors.Is(newErr, context.Canceled) {
		t.Fatalf("pre-cancel precedence changed: %v, %v", oldErr, newErr)
	}
}

func sameFetchError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	var x, y *OversizeError
	if errors.As(a, &x) && errors.As(b, &y) {
		return *x == *y
	}
	return a.Error() == b.Error()
}

type fetchScriptReader struct {
	data   []byte
	step   int
	err    error
	cancel context.CancelFunc
}

func (r *fetchScriptReader) Read(p []byte) (int, error) {
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if len(r.data) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.EOF
	}
	n := min(len(p), len(r.data))
	if r.step > 0 {
		n = min(n, r.step)
	}
	copy(p, r.data[:n])
	r.data = r.data[n:]
	if len(r.data) == 0 && r.err != nil {
		return n, r.err
	}
	return n, nil
}

func TestCappedFetchGrowthShrinkAndReadErrors(t *testing.T) {
	sentinel := errors.New("read failure with bytes")
	for _, tc := range []struct {
		name string
		size int64
		data []byte
		step int
		err  error
	}{
		{"stable", 8, []byte("abcdefgh"), 0, nil},
		{"shrink", 12, []byte("abc"), 1, nil},
		{"grow-under-cap", 1, []byte("abcdefghi"), 2, nil},
		{"zero-stat-grow", 0, []byte("abcdef"), 1, nil},
		{"grow-over-cap", 2, bytes.Repeat([]byte("x"), 32), 1, nil},
		{"error-with-bytes", 2, []byte("abc"), 1, sentinel},
		{"initial-read-error", 2, nil, 1, sentinel},
		{"EOF-with-bytes", 2, []byte("abc"), 2, io.EOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const limit = int64(16)
			old := &fetchScriptReader{data: tc.data, step: tc.step, err: tc.err}
			fresh := &fetchScriptReader{data: tc.data, step: tc.step, err: tc.err}
			want, oldErr := io.ReadAll(io.LimitReader(old, limit+1))
			if oldErr != nil {
				want = nil // fetchCapped discards partial bytes on a read error.
			}
			got, newErr := readFetchSized(context.Background(), fresh, tc.size, limit)
			if !bytes.Equal(got, want) || !sameFetchError(oldErr, newErr) {
				t.Fatalf("old=%q/%v new=%q/%v", want, oldErr, got, newErr)
			}
			if len(tc.data)-len(fresh.data) > int(limit+1) {
				t.Fatal("read beyond the existing limit+1 probe")
			}
		})
	}
}

func TestCappedFetchOwnedBytesAndCancellation(t *testing.T) {
	input := []byte("mutable source")
	got, err := readFetchSized(context.Background(), bytes.NewReader(input), int64(len(input)), 64)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = 'X'
	if string(got) != "mutable source" {
		t.Fatal("returned bytes alias the reader's storage")
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &fetchScriptReader{data: []byte("a long read"), step: 1, cancel: cancel}
	got, err = readFetchSized(ctx, r, 12, 64)
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("cancellation between reads returned data/success: %q, %v", got, err)
	}
	// Cancellation on the final successful EOF is not a new late refusal.
	ctx, cancel = context.WithCancel(context.Background())
	r = &fetchScriptReader{data: []byte("a"), err: io.EOF, cancel: cancel}
	got, err = readFetchSized(ctx, r, 1, 64)
	if err != nil || string(got) != "a" {
		t.Fatalf("completed final read was discarded: %q, %v", got, err)
	}
}

var fetchSizedSink []byte

func allocatedFetch(t *testing.T, path string, fn func(context.Context, string, int64) ([]byte, error)) uint64 {
	t.Helper()
	runtime.GC() // Synthetic measurement only; no production GC change.
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got, err := fn(context.Background(), path, MaxFetchSize)
	runtime.ReadMemStats(&after)
	if err != nil || len(got) != 1<<20 {
		t.Fatalf("allocation workload changed: %d, %v", len(got), err)
	}
	fetchSizedSink = got
	return after.TotalAlloc - before.TotalAlloc
}

func TestCappedFetchAllocationBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one-mebibyte")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		oldBytes := allocatedFetch(t, path, legacyFetchCapped)
		newBytes := allocatedFetch(t, path, fetchCapped)
		saved := int64(oldBytes) - int64(newBytes)
		t.Logf("sample=%d delivered_B=%d legacy_B=%d sized_B=%d saved_B=%d", i, 1<<20, oldBytes, newBytes, saved)
		if saved < 1<<20 {
			t.Fatalf("sized read did not save the fixed 1MiB floor: %d", saved)
		}
	}
}

func BenchmarkCappedFetchMaterialization(b *testing.B) {
	for _, size := range []int{1024, 64 << 10, 1 << 20} {
		path := filepath.Join(b.TempDir(), "input")
		if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o600); err != nil {
			b.Fatal(err)
		}
		for _, item := range []struct {
			name string
			fn   func(context.Context, string, int64) ([]byte, error)
		}{{"legacy", legacyFetchCapped}, {"sized", fetchCapped}} {
			b.Run(item.name+"/"+benchmarkFetchSize(size), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					got, err := item.fn(context.Background(), path, MaxFetchSize)
					if err != nil || len(got) != size {
						b.Fatalf("workload changed: %d, %v", len(got), err)
					}
					fetchSizedSink = got
				}
				b.ReportMetric(float64(size), "delivered-B/op")
			})
		}
	}
}

func benchmarkFetchSize(size int) string {
	switch size {
	case 1024:
		return "1KiB"
	case 64 << 10:
		return "64KiB"
	default:
		return "1MiB"
	}
}
