package console

// rowindex.go — WI-15 (docs/design/work-intelligence.md §4): the
// nearest-neighbour index U6's preflight reads, over at most the last
// MaxFeatureRows proven changes' feature vectors. It is built through a
// memo group keyed by the caller's watermark (I51): a view never rebuilds
// it inline twice, the tenancy ladder can drop it, and a new watermark
// (a new verify reading or ingest) rebuilds it on the next read.

import (
	"context"
	"math"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/memo"
)

// MaxFeatureRows bounds the index (design §4).
const MaxFeatureRows = 5000

// FeatureNames names each dimension of a feature vector, in order.
var FeatureNames = []string{"log tokens", "log turns", "log wall seconds", "zones", "claimed-unproven retries", "row weight"}

// FeaturesOf is a proven change's feature vector.
func FeaturesOf(c ProvenChange) []float64 {
	return []float64{math.Log1p(c.Tokens), math.Log1p(c.Turns), math.Log1p(c.WallSeconds), float64(len(c.Zones)), float64(c.Retries), float64(c.Weight)}
}

type indexedRow struct {
	commit, at, agent string
	vec               []float64
}

// RowFeatureIndex is the bounded index.
type RowFeatureIndex struct {
	rows []indexedRow
	// Dropped counts measured changes older than the newest
	// MaxFeatureRows; Unmeasured counts changes with no effort row.
	Dropped    int
	Unmeasured int
}

// Len is the number of indexed rows.
func (ix *RowFeatureIndex) Len() int { return len(ix.rows) }

// Neighbor is one nearest row.
type Neighbor struct {
	Commit   string  `json:"commit"`
	At       string  `json:"at"`
	Agent    string  `json:"agent"`
	Distance float64 `json:"distance"`
}

// rowFeatureIndex builds the index over the newest MaxFeatureRows
// measured changes.
func rowFeatureIndex(changes []ProvenChange) *RowFeatureIndex {
	ix := &RowFeatureIndex{}
	measured := make([]ProvenChange, 0, len(changes))
	for _, c := range changes {
		if !c.Known {
			ix.Unmeasured++
			continue
		}
		measured = append(measured, c)
	}
	sort.SliceStable(measured, func(i, j int) bool {
		if measured[i].At != measured[j].At {
			return measured[i].At > measured[j].At
		}
		return measured[i].Commit < measured[j].Commit
	})
	if len(measured) > MaxFeatureRows {
		ix.Dropped = len(measured) - MaxFeatureRows
		measured = measured[:MaxFeatureRows]
	}
	ix.rows = make([]indexedRow, len(measured))
	for i, c := range measured {
		ix.rows[i] = indexedRow{commit: c.Commit, at: c.At, agent: c.Agent, vec: FeaturesOf(c)}
	}
	return ix
}

// Nearest returns the k rows nearest v (Euclidean), nearest first, ties
// by commit.
func (ix *RowFeatureIndex) Nearest(v []float64, k int) []Neighbor {
	out := make([]Neighbor, 0, len(ix.rows))
	for _, r := range ix.rows {
		d := 0.0
		for i := range r.vec {
			if i < len(v) {
				d += (r.vec[i] - v[i]) * (r.vec[i] - v[i])
			}
		}
		out = append(out, Neighbor{Commit: r.commit, At: r.at, Agent: r.agent, Distance: math.Sqrt(d)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Distance != out[j].Distance {
			return out[i].Distance < out[j].Distance
		}
		return out[i].Commit < out[j].Commit
	})
	if k >= 0 && len(out) > k {
		out = out[:k]
	}
	return out
}

type rowIndexKey struct{ watermark string }

var rowIndexGroup = memo.NewGroup[rowIndexKey, *RowFeatureIndex]("console.row-feature-index", memo.Options{MaxEntries: 2})

// RowFeatureIndexFor serves the index for watermark from its memo group,
// building it from load's changes once per watermark.
func RowFeatureIndexFor(ctx context.Context, watermark string, load func() []ProvenChange) (*RowFeatureIndex, error) {
	return rowIndexGroup.Get(ctx, rowIndexKey{watermark}, func(context.Context) (*RowFeatureIndex, int64, error) {
		ix := rowFeatureIndex(load())
		return ix, int64(ix.Len()) * 96, nil
	})
}
