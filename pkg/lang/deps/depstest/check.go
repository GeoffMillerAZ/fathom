// Package depstest checks dependency extensions against caller-supplied direct
// evidence. It runs no native oracle and grants no dialect admission.
package depstest

import (
	"context"
	"io/fs"
	"slices"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

// TruthEdge identifies an independently observed construct and its target.
// Use Line 0 when the oracle has no line.
type TruthEdge = depbench.TruthEdge

// TruthFile records a file the oracle covered. An empty Edges list means the
// oracle found no direct dependencies for the file.
type TruthFile = depbench.TruthFile

// Reading gives the B1, B2 and B3 outcomes and the counts behind them. B1 and
// B2 are false when no complete file has oracle coverage, or any complete file
// lacks unique oracle coverage. UnknownByCode accounts for incomplete evidence.
type Reading = depbench.Reading

// Check reads the same inputs twice in repo (reversing their order the second
// time) and once in relocated, which the caller prepares at a different root.
// It compares the reader's serialized output and scores its bindings against
// truth. To grade an unmeasured extension, the caller explicitly trusts it in
// the candidate Reader. Apply dialect admission population minimums separately
// from these fixture results. Check leaves the Reader's admission untouched.
func Check(ctx context.Context, reader *deps.Reader, repo, relocated fs.FS, inputs []deps.Input, truth []TruthFile) (Reading, error) {
	if reader == nil {
		return Reading{}, deps.ErrInput
	}
	files, err := reader.ReadFiles(ctx, repo, inputs)
	if err != nil {
		return Reading{}, err
	}
	reversed := slices.Clone(inputs)
	slices.Reverse(reversed)
	repeat, err := reader.ReadFiles(ctx, repo, reversed)
	if err != nil {
		return Reading{}, err
	}
	moved, err := reader.ReadFiles(ctx, relocated, inputs)
	if err != nil {
		return Reading{}, err
	}
	return depbench.Score(depbench.Observation{Files: files, Repeat: repeat, Relocated: moved, Truth: truth}), nil
}
