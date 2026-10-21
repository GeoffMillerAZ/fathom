package connector

// fetchcap.go — RT-69 (docs/design/responsible-tenant.md §4.2, I48): the
// local connector stats before it reads and never reads a file over
// MaxFetchSize. Before this, pass 2 of an ingest read every discovered
// file whole — app-kit's 3.5 GiB of files over 2 MiB were held in memory
// only for extraction to discard them as oversize.

import (
	"context"
	"fmt"
	"io"
	"os"
)

// MaxFetchSize is the per-file read cap: 2 MiB, equal to
// fusion.MaxExtractSize (a file the extractor would skip is never read).
const MaxFetchSize = 2 << 20

// OversizeError is Fetch's answer for a file over the cap: it was stat'ed,
// never opened. It is neither a read failure nor fatal — callers count and
// name it (I25/I38), never treat it as absent.
type OversizeError struct {
	Path string
	Size int64
	Cap  int64
}

func (e *OversizeError) Error() string {
	return fmt.Sprintf("connector: %s is %d bytes, over the %d-byte read cap (not read)", e.Path, e.Size, e.Cap)
}

// fetchCapped reads path when it is at most limit bytes, checking ctx first
// and deciding from a stat alone when it is not. A file that grows between
// the stat and the read is still bounded: at most limit+1 bytes are read,
// and one byte over reports the file as oversize.
func fetchCapped(ctx context.Context, path string, limit int64) ([]byte, error) {
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
	data, err := readFetchSized(ctx, f, fi.Size(), limit)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &OversizeError{Path: path, Size: int64(len(data)), Cap: limit}
	}
	return data, nil
}

// readFetchSized uses the observed size plus one growth-probe byte for the
// ordinary read. A file that grew can require one additional bounded buffer;
// no file is read beyond limit+1. The returned bytes are owned by this call.
func readFetchSized(ctx context.Context, r io.Reader, size, limit int64) ([]byte, error) {
	data := make([]byte, min(size+1, limit+1))
	n, err := fillFetchBuffer(ctx, r, data)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if err == io.EOF || n < len(data) || int64(n) > limit {
		return data[:n], nil
	}
	// The stat underreported the file. Preserve the probe and read the rest
	// under the original cap, including its one-byte oversize sentinel.
	grown := make([]byte, limit+1)
	copy(grown, data[:n])
	m, err := fillFetchBuffer(ctx, r, grown[n:])
	if err != nil && err != io.EOF {
		return nil, err
	}
	return grown[:n+m], nil
}

func fillFetchBuffer(ctx context.Context, r io.Reader, data []byte) (int, error) {
	used := 0
	for used < len(data) {
		// Check between reads, without a goroutine, changed producer class,
		// or a late check that would discard a successfully completed read.
		if err := ctx.Err(); err != nil {
			return used, err
		}
		n, err := r.Read(data[used:])
		used += n
		if err != nil {
			return used, err
		}
	}
	return used, nil
}
