package client

// A source window is an observed, bounded comparison of a sealed ingest
// projection with the current local tree. It holds hashes, never file bodies.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

const sourceWindowBytes = 64 << 20
const sourceWindowTime = 5 * time.Second

// SourceWindow keeps only one file's rolling hash and per-member hashes.
// Its owner serializes calls; a failed call invalidates the window.
type SourceWindow struct {
	Projection  store.SourceRefProjection
	Pass, Index int
	Offset      int64
	Bytes       int64
	Files       int
	Ready       bool
	Failed      bool
	first       []string
	hash        hash.Hash
	identity    os.FileInfo
	stats       []store.FileStat
	readChunk   func(context.Context, *os.File, []byte) (int, error) // test seam for a late filesystem read
}

// ExcludedHashes names the current bytes observed at both boundaries. These
// hashes attest only this window; the excluded ingest rows have no ref facts.
func (w *SourceWindow) ExcludedHashes() []map[string]string {
	if w == nil || !w.Ready || w.Failed {
		return nil
	}
	out := make([]map[string]string, 0)
	for i, row := range w.Projection.Rows {
		if row.Excluded != "" {
			out = append(out, map[string]string{"path": row.Path, "reason": row.Excluded, "sha256": w.first[i]})
		}
	}
	return out
}

// BeginSourceWindow refuses absent seals, non-local connectors and any
// membership, root, policy, exclusion or file_stats disagreement.
func (c *Client) BeginSourceWindow(ctx context.Context, service string) (*SourceWindow, error) {
	release, err := acquireSourcesSlot(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	ctx, cancel := cancelOnAbandon(ctx)
	defer cancel()
	p, found, err := c.SourceRefProjection(ctx, service)
	if err != nil {
		return nil, err
	}
	if !found || !store.ValidSourceRefProjection(p) || p.ServiceID != service {
		return nil, errors.New("current source window: sealed projection unavailable")
	}
	stats, err := c.sourceWindowBoundary(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &SourceWindow{Projection: p, first: make([]string, len(p.Rows)), stats: stats}, nil
}

func (c *Client) sourceWindowBoundary(ctx context.Context, p store.SourceRefProjection, baseline []store.FileStat) ([]store.FileStat, error) {
	meta, found, err := c.st.GetServiceMeta(ctx, p.ServiceID)
	if err != nil || !found || meta.Path != p.Root || store.WatermarkOf(meta) != p.Watermark {
		return nil, errors.New("current source window: root or watermark changed")
	}
	latest, found, err := c.SourceRefProjection(ctx, p.ServiceID)
	if err != nil || !found || latest.Root != p.Root || latest.Policy != p.Policy || latest.Watermark != p.Watermark || latest.MembershipHash != p.MembershipHash || latest.ExtractorVersion != p.ExtractorVersion || latest.ProducedAt != p.ProducedAt || latest.Complete != p.Complete || latest.Scope != p.Scope {
		return nil, errors.New("current source window: seal or discovery changed")
	}
	stats, ok, err := c.st.FileStats(ctx, p.ServiceID)
	if err != nil || !ok || len(stats) != len(p.Rows) {
		return nil, errors.New("current source window: file_stats unavailable or changed")
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Path < stats[j].Path })
	for i, row := range p.Rows {
		if stats[i].Path != row.Path || stats[i].Excluded != row.Excluded {
			return nil, fmt.Errorf("current source window: file_stats mismatch at %s", row.Path)
		}
		if baseline != nil && stats[i] != baseline[i] {
			return nil, fmt.Errorf("current source window: file_stats fields changed at %s", row.Path)
		}
		if row.Excluded != "" && row.Excluded != "oversize" {
			return nil, fmt.Errorf("current source window: unreadable or unreviewed exclusion %s: %s", row.Path, row.Excluded)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]store.FileStat(nil), stats...), nil
}

// AdvanceSourceWindow reads at most byteLimit bytes and observes a timeLimit
// deadline. A synchronous filesystem read can return after that deadline;
// such an overrun invalidates the window and cannot publish a verdict.
// Zero limits select the production 64 MiB / 5 s unit. The caller must not
// retain a SourcesScope across calls: each unit takes and releases its slot.
func (c *Client) AdvanceSourceWindow(ctx context.Context, w *SourceWindow, byteLimit int64, timeLimit time.Duration) (err error) {
	if w == nil || w.Failed || w.Ready {
		return errors.New("current source window: invalid continuation")
	}
	if byteLimit <= 0 || byteLimit > sourceWindowBytes {
		byteLimit = sourceWindowBytes
	}
	if timeLimit <= 0 || timeLimit > sourceWindowTime {
		timeLimit = sourceWindowTime
	}
	parentCtx := ctx
	ctx, deadlineCancel := context.WithTimeout(ctx, timeLimit)
	defer deadlineCancel()
	release, err := acquireSourcesSlot(ctx)
	if err != nil {
		w.Failed = true
		return err
	}
	defer release()
	ctx, cancel := cancelOnAbandon(ctx)
	defer cancel()
	deadline, _ := ctx.Deadline()
	spent := int64(0)
	buf := make([]byte, 256<<10)
	for spent < byteLimit && time.Now().Before(deadline) {
		if err = ctx.Err(); err != nil {
			if errors.Is(err, context.DeadlineExceeded) && parentCtx.Err() == nil {
				return nil
			}
			w.Failed = true
			return err
		}
		if w.Index == len(w.Projection.Rows) {
			if w.Pass == 0 {
				w.Pass = 1
				w.Index = 0
				w.Offset = 0
				w.hash = nil
				w.identity = nil
				return nil
			}
			_, err = c.sourceWindowBoundary(ctx, w.Projection, w.stats)
			if err != nil {
				w.Failed = true
				return err
			}
			if err = ctx.Err(); err != nil {
				w.Failed = true
				return err
			}
			w.Ready = true
			return nil
		}
		row := w.Projection.Rows[w.Index]
		path := filepath.Join(w.Projection.Root, filepath.FromSlash(row.Path))
		if !filepath.IsLocal(row.Path) {
			w.Failed = true
			return fmt.Errorf("current source window: nonlocal path %q", row.Path)
		}
		li, e := os.Lstat(path)
		if e != nil || !li.Mode().IsRegular() {
			w.Failed = true
			return fmt.Errorf("current source window: unreadable or nonregular %s: %v", row.Path, e)
		}
		if row.Excluded == "oversize" && li.Size() <= connector.MaxFetchSize || row.Excluded == "" && li.Size() > connector.MaxFetchSize {
			w.Failed = true
			return fmt.Errorf("current source window: eligibility changed at %s", row.Path)
		}
		if err = tenancy.ProducerAllowsCtx(ctx, tenancy.ClassTree); err != nil {
			w.Failed = true
			return fmt.Errorf("current source window: producer refusal: %w", err)
		}
		f, e := os.Open(path)
		if e != nil {
			w.Failed = true
			return fmt.Errorf("current source window: open %s: %w", row.Path, e)
		}
		fi, e := f.Stat()
		if e != nil || !os.SameFile(li, fi) || (w.identity != nil && (!os.SameFile(w.identity, fi) || w.identity.Size() != fi.Size() || !w.identity.ModTime().Equal(fi.ModTime()))) {
			f.Close()
			w.Failed = true
			return fmt.Errorf("current source window: file changed at %s", row.Path)
		}
		if w.hash == nil {
			w.hash = sha256.New()
			w.identity = fi
		}
		if _, e = f.Seek(w.Offset, io.SeekStart); e != nil {
			f.Close()
			w.Failed = true
			return e
		}
		remaining := byteLimit - spent
		if remaining > int64(len(buf)) {
			remaining = int64(len(buf))
		}
		var n int
		if w.readChunk != nil {
			n, e = w.readChunk(ctx, f, buf[:remaining])
		} else {
			n, e = f.Read(buf[:remaining])
		}
		post, statErr := f.Stat()
		f.Close()
		if err = ctx.Err(); err != nil {
			w.Failed = true
			return err
		}
		if !time.Now().Before(deadline) {
			w.Failed = true
			return context.DeadlineExceeded
		}
		if err = tenancy.ProducerAllowsCtx(ctx, tenancy.ClassTree); err != nil {
			w.Failed = true
			return fmt.Errorf("current source window: producer refusal: %w", err)
		}
		if statErr != nil || !os.SameFile(w.identity, post) || w.identity.Size() != post.Size() || !w.identity.ModTime().Equal(post.ModTime()) {
			w.Failed = true
			return fmt.Errorf("current source window: file mutated at %s", row.Path)
		}
		if e != nil && e != io.EOF {
			w.Failed = true
			return fmt.Errorf("current source window: read %s: %w", row.Path, e)
		}
		if n > 0 {
			w.hash.Write(buf[:n])
			w.Offset += int64(n)
			w.Bytes += int64(n)
			spent += int64(n)
		}
		if w.Offset < fi.Size() {
			if e == io.EOF || n == 0 {
				w.Failed = true
				return fmt.Errorf("current source window: short read at %s", row.Path)
			}
			continue
		}
		got := hex.EncodeToString(w.hash.Sum(nil))
		if row.Excluded == "" && got != row.SourceHash {
			w.Failed = true
			return fmt.Errorf("current source window: source hash changed at %s", row.Path)
		}
		if w.Pass == 0 {
			w.first[w.Index] = got
		} else if got != w.first[w.Index] {
			w.Failed = true
			return fmt.Errorf("current source window: bytes changed across window at %s", row.Path)
		}
		w.Files++
		w.Index++
		w.Offset = 0
		w.hash = nil
		w.identity = nil
	}
	if err := ctx.Err(); err != nil && (!errors.Is(err, context.DeadlineExceeded) || parentCtx.Err() != nil) {
		w.Failed = true
		return err
	}
	return nil
}
