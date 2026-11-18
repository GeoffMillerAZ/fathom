package deps

import (
	"cmp"
	"maps"
	"slices"
)

// DialectInfo describes the dependency constructs a Reader implements for ID.
// Kinds is ordered by its wire values. Measured reflects the Reader's active
// resolver, independently of trust and any individual file's completeness.
type DialectInfo struct {
	ID       string
	Measured bool
	Kinds    []Kind
}

var builtinDependencyKinds = map[string][]Kind{
	"dockerfile": {KindBuildInput, KindConfigRef},
	"go":         {KindBuildInput, KindConfigRef, KindEmbed, KindImport},
	"javascript": {KindConfigRef, KindImport},
	"python":     {KindImport},
	"typescript": {KindConfigRef, KindImport},
}

// Dialects lists the built-in and registered pack dialects by ID. The returned
// metadata is owned by the caller.
func (r *Reader) Dialects() []DialectInfo {
	out := make([]DialectInfo, 0, len(builtinDependencyKinds)+len(r.packs))
	for id, kinds := range builtinDependencyKinds {
		out = append(out, DialectInfo{ID: id, Measured: r.activeAdmission(id, admissions[id]) == measured, Kinds: slices.Clone(kinds)})
	}
	for id, pack := range r.packs {
		kinds := slices.Collect(maps.Values(pack.kinds))
		slices.Sort(kinds)
		out = append(out, DialectInfo{ID: id, Kinds: slices.Compact(kinds)})
	}
	slices.SortFunc(out, func(a, b DialectInfo) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

func (r *Reader) activeAdmission(dialect string, state admission) admission {
	if state == measured && r.resolvers[dialect] != nil {
		return unmeasured
	}
	return state
}
