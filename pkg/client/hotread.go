package client

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"
)

// HotPathReadCap is the largest file a request-path read in this package
// loads (docs/design/responsible-tenant.md §4.2–§4.3, invariant I38): a
// file over it is stat'ed, never read, and the skip is named to the caller
// as a partial answer, never passed off as absent or fresh.
const HotPathReadCap int64 = 2 << 20

// readObserver, when set, is told every path readFileCapped actually
// opens for reading — the tests' proof that a skipped file was never read
// and that a whole-tree walk read no contents. nil in production.
var readObserver atomic.Pointer[func(path string)]

// readFileCapped stats path first and reads it only when it is a regular
// file of at most max bytes. skipped names why a file was not read (and
// data is nil); err is a stat or read failure. A file that grew past max
// between the stat and the read is skipped too, never truncated.
func readFileCapped(path string, max int64) (data []byte, info os.FileInfo, skipped string, err error) {
	info, err = os.Stat(path)
	if err != nil {
		return nil, nil, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, info, "not a regular file", nil
	}
	if info.Size() > max {
		return nil, info, oversizeNote(info.Size(), max), nil
	}
	if obs := readObserver.Load(); obs != nil {
		(*obs)(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, info, "", err
	}
	defer f.Close()
	data, err = io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, info, "", err
	}
	if int64(len(data)) > max {
		return nil, info, oversizeNote(int64(len(data)), max), nil
	}
	return data, info, "", nil
}

func oversizeNote(size, max int64) string {
	return fmt.Sprintf("file is %s, over the %s read cap", humanBytes(size), humanBytes(max))
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
