package receipt

// export.go — the producer side of an exported receipt (EVX-7): an
// Exporter resolves the commit once, cites each span at it with the bytes
// its excerpt covers, and gives an absence fact its tree and read set.
// `fathom export receipt` hands what it gathers to BuildReceipt.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

// Exporter anchors one export's spans at one commit.
type Exporter struct {
	c        Commit
	base     string
	sourceOf func(rel string) Source
}

// NewExporter resolves run's commit (run.Dir, run.Rev). run.Base is the
// directory a span's relative path is relative to ("" is the work tree's
// top level); run.Stored, run.Nodes and run.Service decide what the
// producer read and what is withheld, as for AnchorRun.
func NewExporter(ctx context.Context, run Run) (*Exporter, error) {
	if run.Dir == "" {
		return nil, errors.New("receipt: no repository to export from")
	}
	c, err := ResolveCommit(ctx, run.Dir, run.Rev)
	if err != nil {
		return nil, err
	}
	return &Exporter{c: c, base: run.Base, sourceOf: run.sources()}, nil
}

// Commit is the commit every anchor of the export names.
func (e *Exporter) Commit() Commit { return e.c }

// Cite anchors s, its path made repo-relative, with its excerpt's bytes.
func (e *Exporter) Cite(ctx context.Context, s Span) (CitedAnchor, error) {
	rel, ok := repoRelative(e.c.Root, e.base, s.Path)
	if !ok {
		return CitedAnchor{}, fmt.Errorf("receipt: %s is outside the work tree", s.Path)
	}
	s.Path = rel
	a, ex, err := AnchorExcerpt(ctx, e.c, s, e.sourceOf(rel))
	if err != nil {
		return CitedAnchor{}, err
	}
	return CitedAnchor{Anchor: a, Excerpt: ex}, nil
}

// Tree is the commit's root tree id: the snapshot an absence fact's read
// set names.
func (e *Exporter) Tree(ctx context.Context) (string, error) {
	out, err := gitro.Output(ctx, e.c.Root, "rev-parse", "--verify", "--quiet", "--end-of-options", e.c.ID+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("receipt: the tree of %s: %w", short(e.c.ID), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// outsideWorkTree stands for every indexed source outside the work tree:
// its path never leaves.
const outsideWorkTree = "(outside the work tree)"

var (
	errNoFingerprint = errors.New("the index holds no fingerprint for it")
	errOutside       = errors.New("an indexed source outside the work tree")
)

// IndexReadSet is the read set of an answer from the index: each of refs
// (the index's source refs, relative to the exporter's base) is read when
// the index holds its fingerprint, and missing otherwise (I38).
func (e *Exporter) IndexReadSet(ctx context.Context, refs []string, stored func(ctx context.Context, ref string) (string, bool, error)) readmanifest.Manifest {
	set := readmanifest.NewSet()
	for _, ref := range refs {
		rel, ok := repoRelative(e.c.Root, e.base, ref)
		if !ok {
			set.Record(outsideWorkTree, readmanifest.Unreadable(errOutside))
			continue
		}
		_, found, err := stored(ctx, ref)
		switch {
		case err != nil:
			set.Record(rel, readmanifest.Unreadable(err))
		case !found:
			set.Record(rel, readmanifest.Unreadable(errNoFingerprint))
		default:
			set.Record(rel, readmanifest.Read(0))
		}
	}
	return set.Manifest()
}
