// Package corpus is the request path's one way to read corpus files
// (docs/design/responsible-tenant.md §4.1, invariant I48, ADR-047).
//
// A request reads only the files it was asked about, and it reads them
// through ReadFileCapped: stat first (an oversize or non-regular file is
// excluded by name without being opened), sniff the first 8 KiB for a
// binary, read the rest under the cap, and charge every byte to the call's
// admission meter. Whole-tree reads exist only as ScanBudgeted: one file
// at a time, stopping at a byte, file, wall-clock or hit bound or at the
// caller's ctx, and reporting what it covered as a lower bound (I38) —
// never a silent partial.
//
// It sits beside internal/connector (I20: it imports neither query nor
// mcp).
package corpus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// DefaultCap is the per-file read cap: fusion.MaxExtractSize (2 MiB), the
// same line ingest draws.
const DefaultCap int64 = 2 << 20

// SniffSize is how much of a file the binary sniff reads.
const SniffSize = 8 << 10

// Exclusion reasons.
const (
	ReasonOversize   = "oversize"
	ReasonBinary     = "binary"
	ReasonIgnored    = "ignored"
	ReasonUnreadable = "unreadable"
	ReasonNotRegular = "not-regular"
)

// Excluded names one file a read left out and why.
type Excluded struct {
	Path   string
	Reason string
	Size   int64
	// Err is the failure behind an unreadable exclusion (nil otherwise).
	Err error
}

// FileStat is a file's stat facts (file_stats rows, §6).
type FileStat struct {
	Path    string
	Size    int64
	MtimeNS int64
	Ino     uint64
	Mode    fs.FileMode
}

// ReadFileCapped reads path when it is a regular file of at most cap bytes
// (cap ≤ 0: DefaultCap) that does not sniff as binary. It stats before it
// opens: an oversize or non-regular file comes back as an *Excluded
// without being read. A binary is excluded after reading at most SniffSize
// bytes. Every byte read is charged to ctx's admission ticket; a charge
// over an enforcing ceiling returns its error and no content. A missing
// file is an error wrapping fs.ErrNotExist; a permission failure is an
// unreadable exclusion.
func ReadFileCapped(ctx context.Context, path string, cap int64) ([]byte, *Excluded, error) {
	data, _, ex, err := Read(ctx, path, cap, nil)
	return data, ex, err
}

// Read is ReadFileCapped that also returns the file's stat (nil when the
// stat failed) and tells onOpen (when non-nil) each path it actually
// opens — the callers' proof that an excluded file was never read.
func Read(ctx context.Context, path string, cap int64, onOpen func(path string)) ([]byte, fs.FileInfo, *Excluded, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassPoint); err != nil {
		return nil, nil, nil, err
	}
	if cap <= 0 {
		cap = DefaultCap
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil, err
		}
		return nil, nil, &Excluded{Path: path, Reason: ReasonUnreadable, Err: err}, nil
	}
	if !info.Mode().IsRegular() {
		return nil, info, &Excluded{Path: path, Reason: ReasonNotRegular, Size: info.Size()}, nil
	}
	if info.Size() > cap {
		return nil, info, &Excluded{Path: path, Reason: ReasonOversize, Size: info.Size()}, nil
	}
	if onOpen != nil {
		onOpen(path)
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, info, nil, err
		}
		return nil, info, &Excluded{Path: path, Reason: ReasonUnreadable, Size: info.Size(), Err: err}, nil
	}
	defer f.Close()

	// Sniff: at most SniffSize bytes decide binary before the rest is read.
	head := make([]byte, min(int64(SniffSize), info.Size()+1))
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, info, &Excluded{Path: path, Reason: ReasonUnreadable, Size: info.Size(), Err: err}, nil
	}
	head = head[:n]
	if cerr := tenancy.Charge(ctx, int64(n)); cerr != nil {
		return nil, info, nil, cerr
	}
	if connector.IsBinary(head) {
		return nil, info, &Excluded{Path: path, Reason: ReasonBinary, Size: info.Size()}, nil
	}
	if n < len(head) {
		// EOF inside the sniff: the whole file is in head.
		return head, info, nil, nil
	}
	// The rest, under the cap: a file that grew since the stat stops at
	// cap+1 and is named oversize.
	buf := make([]byte, n, max(info.Size(), int64(n))+1)
	copy(buf, head)
	rest, err := io.ReadAll(io.LimitReader(f, cap+1-int64(n)))
	if err != nil {
		return nil, info, &Excluded{Path: path, Reason: ReasonUnreadable, Size: info.Size(), Err: err}, nil
	}
	if cerr := tenancy.Charge(ctx, int64(len(rest))); cerr != nil {
		return nil, info, nil, cerr
	}
	buf = append(buf, rest...)
	if int64(len(buf)) > cap {
		return nil, info, &Excluded{Path: path, Reason: ReasonOversize, Size: int64(len(buf))}, nil
	}
	return buf, info, nil, nil
}

// Budget bounds a scan. A zero field is no bound on that axis.
type Budget struct {
	Bytes int64         // total bytes read
	Files int           // files scanned (read and handed to fn)
	Wall  time.Duration // elapsed time
	Hits  int           // reserved for callers that count hits themselves; fn's stop is the hit bound
	// FileCap is the per-file cap (0: DefaultCap).
	FileCap int64
	// Now is the clock for Wall (nil: time.Now).
	Now func() time.Time
}

// Stop reasons (Coverage.Stopped).
const (
	StopBytes = "bytes"
	StopFiles = "files"
	StopTime  = "time"
	StopHits  = "hits"
	StopCtx   = "ctx"
	StopMeter = "meter"
)

// Coverage is what a scan covered: Scanned of Total files, the files it
// excluded by name, and why it stopped early ("" = it did not).
type Coverage struct {
	Scanned int
	Total   int
	// BytesRead is what the scan handed on (the sum of the files' sizes).
	BytesRead int64
	Excluded  []Excluded
	Stopped   string
}

// LowerBound reports whether the scan stopped early, so any count it
// produced is a lower bound (I38).
func (c Coverage) LowerBound() bool { return c.Stopped != "" }

// String renders the coverage for a partial answer: "scanned 812/2236
// files: budget bytes; excluded 3".
func (c Coverage) String() string {
	s := fmt.Sprintf("scanned %d/%d files", c.Scanned, c.Total)
	if c.Stopped != "" {
		s += ": budget " + c.Stopped
	}
	if len(c.Excluded) > 0 {
		s += fmt.Sprintf("; excluded %d", len(c.Excluded))
	}
	return s
}

// ScanBudgeted reads paths (relative ones under root) one file at a time
// through ReadFileCapped and hands each readable file to fn, releasing the
// buffer before the next read. It stops before a read that would pass
// b.Bytes, after b.Files files, once b.Wall has elapsed, when fn returns
// true (hits), when ctx ends (and returns ctx's error), or when the meter
// refuses a charge. Missing files are skipped; every other exclusion is
// named in Coverage.Excluded.
func ScanBudgeted(ctx context.Context, root string, paths []string, b Budget, fn func(path string, content []byte) (stop bool)) (Coverage, error) {
	cov := Coverage{Total: len(paths)}
	now := b.Now
	if now == nil {
		now = time.Now
	}
	start := now()
	fileCap := b.FileCap
	if fileCap <= 0 {
		fileCap = DefaultCap
	}
	var read int64
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			cov.Stopped = StopCtx
			return cov, err
		}
		if b.Files > 0 && cov.Scanned >= b.Files {
			cov.Stopped = StopFiles
			return cov, nil
		}
		if b.Wall > 0 && now().Sub(start) >= b.Wall {
			cov.Stopped = StopTime
			return cov, nil
		}
		full := p
		if root != "" && !filepath.IsAbs(p) {
			full = filepath.Join(root, p)
		}
		if b.Bytes > 0 {
			// Only a file that would be read spends the budget: an oversize
			// or non-regular one is excluded from its stat, unread.
			if info, err := os.Stat(full); err == nil && info.Mode().IsRegular() && info.Size() <= fileCap && read+info.Size() > b.Bytes {
				cov.Stopped = StopBytes
				return cov, nil
			}
		}
		data, ex, err := ReadFileCapped(ctx, full, b.FileCap)
		if err != nil {
			switch {
			case errors.Is(err, fs.ErrNotExist):
				continue
			case ctx.Err() != nil:
				cov.Stopped = StopCtx
				return cov, ctx.Err()
			default:
				cov.Stopped = StopMeter
				return cov, nil
			}
		}
		if ex != nil {
			ex.Path = p
			cov.Excluded = append(cov.Excluded, *ex)
			continue
		}
		read += int64(len(data))
		cov.BytesRead = read
		cov.Scanned++
		if fn(p, data) {
			cov.Stopped = StopHits
			return cov, nil
		}
	}
	return cov, nil
}

// ScanFetch is ScanBudgeted over a fetch function — a connector's Fetch —
// for the request path's tree reads (RT-71, §4.6): one file at a time,
// handed to fn and released before the next, stopping at b's byte, file and
// wall bounds, at fn's hit stop, at ctx, or at a meter refusal. A file the
// byte budget cannot hold stops the scan (it was fetched under the per-file
// cap but never handed on). A missing file is skipped; an oversize one
// (*connector.OversizeError) is named in Coverage.Excluded; any other fetch
// failure is named unreadable.
func ScanFetch(ctx context.Context, paths []string, b Budget, fetch func(ctx context.Context, path string) ([]byte, error), fn func(path string, content []byte) (stop bool)) (Coverage, error) {
	cov := Coverage{Total: len(paths)}
	now := b.Now
	if now == nil {
		now = time.Now
	}
	start := now()
	var read int64
	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			cov.Stopped = StopCtx
			return cov, err
		}
		if b.Files > 0 && cov.Scanned >= b.Files {
			cov.Stopped = StopFiles
			return cov, nil
		}
		if b.Wall > 0 && now().Sub(start) >= b.Wall {
			cov.Stopped = StopTime
			return cov, nil
		}
		data, err := fetch(ctx, p)
		if err != nil {
			var over *connector.OversizeError
			switch {
			case ctx.Err() != nil:
				cov.Stopped = StopCtx
				return cov, ctx.Err()
			case errors.Is(err, fs.ErrNotExist):
				continue
			case errors.As(err, &over):
				cov.Excluded = append(cov.Excluded, Excluded{Path: p, Reason: ReasonOversize, Size: over.Size})
				continue
			case errors.Is(err, tenancy.ErrOverBudget), errors.Is(err, tenancy.ErrAbandonedByGovernor), errors.Is(err, tenancy.ErrHaltedOverBudget):
				cov.Stopped = StopMeter
				return cov, nil
			default:
				cov.Excluded = append(cov.Excluded, Excluded{Path: p, Reason: ReasonUnreadable, Err: err})
				continue
			}
		}
		if b.Bytes > 0 && read+int64(len(data)) > b.Bytes {
			cov.Stopped = StopBytes
			return cov, nil
		}
		read += int64(len(data))
		cov.BytesRead = read
		cov.Scanned++
		if fn(p, data) {
			cov.Stopped = StopHits
			return cov, nil
		}
	}
	return cov, nil
}

// ScanSourceFiles is ScanFetch over files already in memory — the shape an
// in-memory or test backend answers a tree scan with, under the same budget
// and hit semantics as a connector-backed one.
func ScanSourceFiles(ctx context.Context, files []core.SourceFile, b Budget, fn func(core.SourceFile) (stop bool)) (Coverage, error) {
	byPath := make(map[string]core.SourceFile, len(files))
	paths := make([]string, 0, len(files))
	for _, f := range files {
		if _, dup := byPath[f.Path]; !dup {
			paths = append(paths, f.Path)
		}
		byPath[f.Path] = f
	}
	return ScanFetch(ctx, paths, b, func(_ context.Context, p string) ([]byte, error) {
		return byPath[p].Content, nil
	}, func(p string, _ []byte) bool { return fn(byPath[p]) })
}
