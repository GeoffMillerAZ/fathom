package depbench

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

// COVBatch separates actual product output from hard input refusals. Rejected
// identities are campaign-private memory, never a serialized product finding.
type COVBatch struct {
	Files    []deps.FileDeps
	Rejected []deps.Input `json:"-"`
}

// COVReadIsolated bisects only an ErrInput batch. Valid partitions are read
// once; non-input failures remain failures. The product and population do not
// change. ESCALATE-19 authorizes this test-instrument behavior only.
func COVReadIsolated(ctx context.Context, r *deps.Reader, repo fs.FS, inputs []deps.Input) (COVBatch, error) {
	files, err := r.ReadFiles(ctx, repo, inputs)
	if err == nil {
		return COVBatch{Files: files}, nil
	}
	if !errors.Is(err, deps.ErrInput) {
		return COVBatch{}, err
	}
	if len(inputs) == 1 {
		return COVBatch{Rejected: slices.Clone(inputs)}, nil
	}
	if len(inputs) == 0 {
		return COVBatch{}, err
	}
	mid := len(inputs) / 2
	a, err := COVReadIsolated(ctx, r, repo, inputs[:mid])
	if err != nil {
		return COVBatch{}, err
	}
	b, err := COVReadIsolated(ctx, r, repo, inputs[mid:])
	if err != nil {
		return COVBatch{}, err
	}
	a.Files = append(a.Files, b.Files...)
	a.Rejected = append(a.Rejected, b.Rejected...)
	slices.SortFunc(a.Files, func(a, b deps.FileDeps) int { return strings.Compare(a.Path, b.Path) })
	slices.SortFunc(a.Rejected, func(a, b deps.Input) int { return strings.Compare(a.Path, b.Path) })
	return a, nil
}

// COVIncludeRejected forms an in-memory scoring population. Empty incomplete
// placeholders are harness accounting only: no product edge or unknown code
// is invented, and these placeholders must never be persisted as product rows.
func COVIncludeRejected(batch COVBatch) []deps.FileDeps {
	files := slices.Clone(batch.Files)
	for _, in := range batch.Rejected {
		files = append(files, deps.FileDeps{Path: in.Path, Dialect: in.Dialect, Edges: []deps.Edge{}, Unknowns: []deps.Unknown{}})
	}
	slices.SortFunc(files, func(a, b deps.FileDeps) int { return strings.Compare(a.Path, b.Path) })
	return files
}

func COVSameRejected(first, repeat, relocated []deps.Input) bool {
	normalize := func(inputs []deps.Input) []deps.Input {
		out := slices.Clone(inputs)
		slices.SortFunc(out, func(a, b deps.Input) int {
			if a.Path != b.Path {
				return strings.Compare(a.Path, b.Path)
			}
			return strings.Compare(a.Dialect, b.Dialect)
		})
		return out
	}
	return slices.Equal(normalize(first), normalize(repeat)) && slices.Equal(normalize(first), normalize(relocated))
}
