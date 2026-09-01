package catalog

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// Doc is the indexed text for one node: what Search matches against.
type Doc struct {
	NodeID string
	Terms  []string // tokenized Title + Synopsis + facet values, lowercased
}

// tokenPattern splits indexed text into lowercase word tokens.
var tokenPattern = regexp.MustCompile(`[a-z0-9]+`)

func tokenize(s string) []string {
	return tokenPattern.FindAllString(strings.ToLower(s), -1)
}

// BuildDocs derives one Doc per node from Title, Synopsis, and every
// facet value (never facet axis names, which are structural, not
// content). Facet values are visited in sorted (axis, value) order, so
// BuildDocs never depends on map iteration order.
func BuildDocs(cat Catalog) []Doc {
	docs := make([]Doc, 0, len(cat.Nodes))
	for _, n := range cat.Nodes {
		var terms []string
		terms = append(terms, tokenize(n.Title)...)
		terms = append(terms, tokenize(n.Synopsis)...)

		axes := make([]string, 0, len(n.Facets))
		for axis := range n.Facets {
			axes = append(axes, axis)
		}
		sort.Strings(axes)
		for _, axis := range axes {
			vals := append([]string(nil), n.Facets[axis]...)
			sort.Strings(vals)
			for _, v := range vals {
				terms = append(terms, tokenize(v)...)
			}
		}
		docs = append(docs, Doc{NodeID: n.ID(), Terms: terms})
	}
	return docs
}

// Result is one scored match. Score is always exposed — design §5's "a
// caller that cannot see the score cannot judge the match."
type Result struct {
	NodeID string
	Score  float64
}

// Ranker scores a query against a document set. The kit ships BM25Ranker;
// a semantic/vector ranker is a consumer's own choice, injected here —
// this package never imports one (design §5, the kit's standing
// evidence-gated position on embeddings).
type Ranker interface {
	Rank(query string, docs []Doc) []Result
}

// BM25Ranker is the kit's default Ranker: standard Okapi BM25 with
// deterministic tie-breaking (score descending, then NodeID ascending),
// so two runs over the same node set always return results in the same
// order.
type BM25Ranker struct {
	K1 float64
	B  float64
}

// NewBM25Ranker returns a BM25Ranker with the conventional defaults
// (k1=1.2, b=0.75).
func NewBM25Ranker() BM25Ranker {
	return BM25Ranker{K1: 1.2, B: 0.75}
}

// Rank implements Ranker.
func (r BM25Ranker) Rank(query string, docs []Doc) []Result {
	qTerms := tokenize(query)
	if len(qTerms) == 0 || len(docs) == 0 {
		return nil
	}

	avgLen := 0.0
	for _, d := range docs {
		avgLen += float64(len(d.Terms))
	}
	avgLen /= float64(len(docs))

	df := map[string]int{}
	for _, d := range docs {
		seen := map[string]bool{}
		for _, t := range d.Terms {
			seen[t] = true
		}
		for t := range seen {
			df[t]++
		}
	}
	n := float64(len(docs))

	var results []Result
	for _, d := range docs {
		tf := map[string]int{}
		for _, t := range d.Terms {
			tf[t]++
		}
		score := 0.0
		for _, qt := range qTerms {
			f := float64(tf[qt])
			if f == 0 {
				continue
			}
			docDF := float64(df[qt])
			idf := math.Log(1 + (n-docDF+0.5)/(docDF+0.5))
			denom := f + r.K1*(1-r.B+r.B*float64(len(d.Terms))/avgLen)
			score += idf * (f * (r.K1 + 1)) / denom
		}
		if score > 0 {
			results = append(results, Result{NodeID: d.NodeID, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].NodeID < results[j].NodeID
	})
	return results
}

// Outcome is Search's result: the query that was run, and what it found.
// An empty Results is itself the answer — "no match" — never a silent
// nearest-neighbour (design §5's refusal doctrine, applied to search).
type Outcome struct {
	Query   string
	Results []Result
}

// NoMatch reports whether the search found nothing.
func (o Outcome) NoMatch() bool { return len(o.Results) == 0 }

// Search runs query against cat's nodes using ranker (BM25Ranker via
// NewBM25Ranker if the caller has no reason to inject another).
func Search(cat Catalog, query string, ranker Ranker) Outcome {
	docs := BuildDocs(cat)
	return Outcome{Query: query, Results: ranker.Rank(query, docs)}
}
