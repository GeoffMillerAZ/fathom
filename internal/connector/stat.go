package connector

// stat.go — RT-102 (docs/design/responsible-tenant.md §7.2): pass 1 of an
// ingest asks each discovered file's (size, mtime, inode) — a stat, never a
// read — so the pipeline reads only the files whose stat moved since the
// last ingest.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// FileStat is one file's identity as pass 1 compares it.
type FileStat struct {
	Size    int64
	MtimeNS int64
	Ino     uint64
}

// Stater is the optional connector capability behind the stat gate: a
// connector that can stat a discovered ref without reading it.
type Stater interface {
	Stat(ctx context.Context, ref core.SourceRef) (FileStat, error)
}

// Stat implements Stater: an lstat-free os.Stat of the ref resolved exactly
// as Fetch resolves it.
func (c *LocalConnector) Stat(ctx context.Context, ref core.SourceRef) (FileStat, error) {
	if err := ctx.Err(); err != nil {
		return FileStat{}, err
	}
	p := ref.Path
	if !filepath.IsAbs(p) {
		root, ok := c.roots.Load(ref.Service)
		if !ok {
			return FileStat{}, fmt.Errorf("connector: no root recorded for service %q (Discover must precede Stat)", ref.Service)
		}
		p = filepath.Join(root.(string), filepath.FromSlash(p))
	}
	fi, err := os.Stat(p)
	if err != nil {
		return FileStat{}, err
	}
	return FileStat{Size: fi.Size(), MtimeNS: fi.ModTime().UnixNano(), Ino: inoOf(fi)}, nil
}
