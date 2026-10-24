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
	// The shape and measured effort U6 reads for a row's nearest
	// neighbours (WI-16): the zones the change touched, its effort, and
	// whether it was proven at the first attempt.
	zones               []string
	tokens, turns, wall float64
	firstAttempt        bool
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
		zs := make([]string, 0, len(c.Zones))
		for _, z := range c.Zones {
			zs = append(zs, z.Zone)
		}
		sort.Strings(zs)
		ix.rows[i] = indexedRow{
			commit: c.Commit, at: c.At, agent: c.Agent, vec: FeaturesOf(c), zones: zs,
			tokens: c.Tokens, turns: c.Turns, wall: c.WallSeconds, firstAttempt: c.FirstAttempt(),
		}
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

// RowQuery is what is known of a row before work starts, in the
// dimensions a proven change also has: the zones it touches, the
// claimed-unproven readings it already has, and its plan weight.
type RowQuery struct {
	Zones   []string
	Retries int
	Weight  int
	// ExcludeWeight omits the weight dimension when estimating that value.
	ExcludeWeight bool
}

// SimilarRow is one nearest proven change with its measured effort.
type SimilarRow struct {
	Commit       string  `json:"commit"`
	At           string  `json:"at"`
	Agent        string  `json:"agent"`
	Distance     float64 `json:"distance"`
	SharedZones  int     `json:"shared_zones"`
	Tokens       float64 `json:"tokens"`
	Turns        float64 `json:"turns"`
	WallSeconds  float64 `json:"wall_seconds"`
	FirstAttempt bool    `json:"first_attempt"`
}

// similarDims are the FeatureNames dimensions a row has before work: the
// zone count, the claimed-unproven retries and the row weight. The effort
// dimensions are what U6 predicts, so they are never part of the distance.
var similarDims = []int{3, 4, 5}

// Similar returns the k proven changes nearest q: Euclidean over the
// pre-work dimensions plus the share of the row's zones the change did not
// touch (a change in the row's own zone outranks an identical shape
// elsewhere). Ties go to the newest, then the commit — deterministic.
func (ix *RowFeatureIndex) Similar(q RowQuery, k int) []SimilarRow {
	qz := map[string]bool{}
	for _, z := range q.Zones {
		qz[z] = true
	}
	qv := make([]float64, len(FeatureNames))
	qv[3], qv[4], qv[5] = float64(len(qz)), float64(q.Retries), float64(q.Weight)
	out := make([]SimilarRow, 0, len(ix.rows))
	for _, r := range ix.rows {
		d := 0.0
		for _, i := range similarDims {
			if i == 5 && q.ExcludeWeight {
				continue
			}
			d += (r.vec[i] - qv[i]) * (r.vec[i] - qv[i])
		}
		shared := 0
		for _, z := range r.zones {
			if qz[z] {
				shared++
			}
		}
		union := len(qz) + len(r.zones) - shared
		if union > 0 {
			miss := 1 - float64(shared)/float64(union)
			d += miss * miss
		}
		out = append(out, SimilarRow{
			Commit: r.commit, At: r.at, Agent: r.agent, Distance: math.Round(math.Sqrt(d)*1e6) / 1e6, SharedZones: shared,
			Tokens: r.tokens, Turns: r.turns, WallSeconds: r.wall, FirstAttempt: r.firstAttempt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		switch {
		case out[i].Distance != out[j].Distance:
			return out[i].Distance < out[j].Distance
		case out[i].At != out[j].At:
			return out[i].At > out[j].At
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
