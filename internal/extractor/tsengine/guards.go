package tsengine

import (
	"fmt"
	"regexp"
	"strings"
)

// Pattern guards (LB-F180, LB-M5).
//
// The guest runs every pattern of a query at every node, and a pattern that
// matches structurally but is dropped by a host-side text predicate still
// costs its matches: an HTML query whose `id`, form, field and custom-element
// patterns each match every attribute or tag of a page spends most of its time
// on matches the predicates then reject (a javadoc page of 3.6 MB of markup:
// 36 G instructions in the query against 13 G in the parse). A pattern may say
// what a source must contain for it to match at all:
//
//	(element (start_tag (tag_name) @name (#match? @name "-")) (#requires-text! "<[^\\s<>]*-")) @definition.custom_element
//
// `(#requires-text! "re")` is a necessary condition, read by the author of the
// query: the pattern can match a source only if re (RE2, unanchored) matches
// the source's bytes. Run executes a source the guard fails, in a variant of
// the query compiled once without that pattern: the surviving matches are the
// ones the full query gives, in the same order, with their original pattern
// indexes. A guard that is not necessary drops matches, so a dialect's tests
// run every guarded query over its testdata with and without the guards.

// maxQueryVariants bounds the pruned variants one query keeps compiled; past
// it a source runs the full query.
const maxQueryVariants = 16

// queryVariant is a query compiled from a subset of the patterns.
type queryVariant struct {
	ptr   uint32
	remap []uint16 // variant pattern index -> the full query's
}

// guarded returns the guest query to run over src and the map from its
// pattern indexes to the full query's (nil for the full query). none reports
// that no pattern can match src.
func (q *Query) guarded(src []byte) (ptr uint32, remap []uint16, none bool) {
	q.lastPatterns = len(q.patterns)
	if q.guardsOff {
		return q.ptr, nil, false
	}
	var kept []int
	pruned, any := false, false
	holds := map[*regexp.Regexp]bool{}
	for i := range q.patterns {
		ok := true
		for _, re := range q.patterns[i].requires {
			any = true
			h, seen := holds[re]
			if !seen {
				h = re.Match(src)
				holds[re] = h
			}
			if !h {
				ok = false
				break
			}
		}
		if ok {
			kept = append(kept, i)
		} else {
			pruned = true
		}
	}
	if !any || !pruned {
		return q.ptr, nil, false
	}
	if len(kept) == 0 {
		q.lastPatterns = 0
		return 0, nil, true
	}
	key := fmt.Sprint(kept)
	v, ok := q.variants[key]
	if !ok {
		if len(q.variants) >= maxQueryVariants {
			return q.ptr, nil, false
		}
		var err error
		if v, err = q.compileVariant(kept); err != nil {
			return q.ptr, nil, false // the full query answers
		}
		if q.variants == nil {
			q.variants = map[string]*queryVariant{}
		}
		q.variants[key] = v
	}
	q.lastPatterns = len(kept)
	return v.ptr, v.remap, false
}

// compileVariant compiles the patterns kept (indexes into q.patterns) alone.
func (q *Query) compileVariant(kept []int) (*queryVariant, error) {
	var b strings.Builder
	remap := make([]uint16, 0, len(kept))
	for _, i := range kept {
		m := q.patterns[i]
		b.WriteString(q.src[m.start:m.end])
		b.WriteString("\n\n")
		remap = append(remap, uint16(i))
	}
	ptr, err := q.p.compileRaw(b.String())
	if err != nil {
		return nil, err
	}
	return &queryVariant{ptr: ptr, remap: remap}, nil
}

// RunUnguarded is Run with the #requires-text! guards ignored: every pattern
// executes. A dialect's tests compare it with Run to prove its guards are
// necessary conditions.
func (q *Query) RunUnguarded(t *Tree) ([]Match, error) {
	q.guardsOff = true
	defer func() { q.guardsOff = false }()
	return q.Run(t)
}

// LastRunPatterns is how many patterns the guest query of q's last Run held:
// PatternCount, fewer when a guard pruned the query for that source.
func (q *Query) LastRunPatterns() int { return q.lastPatterns }
