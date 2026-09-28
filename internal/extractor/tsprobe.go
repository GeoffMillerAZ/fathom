package extractor

import (
	"fmt"
	"sort"
)

// ProbeTarget is what `indexbench lang-probe` (LB-F20) needs to show a
// worker a file's tree and its dialect's captures: the grammar blob, the
// entry point the file's extension selects, and the query source.
type ProbeTarget struct {
	Dialect    string
	Blob       string
	EntryPoint string
	QueryFile  string
	Query      string
}

// TagsDialects returns the sorted ids of every dialect on the tags engine —
// every embedded manifest with engine "tags" (LB-F1).
func TagsDialects() []string {
	if _, err := tsRegistry(); err != nil {
		return nil
	}
	ids := make([]string, 0, len(tsDialectMap))
	for id := range tsDialectMap {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ProbeTargetFor resolves dialect and a file path to the variant the tags
// engine would parse that file with, and that variant's query source (the
// tags query, then the imports query).
func ProbeTargetFor(dialect, relPath string) (ProbeTarget, error) {
	if _, err := tsRegistry(); err != nil {
		return ProbeTarget{}, err
	}
	spec, ok := tsDialectMap[dialect]
	if !ok {
		return ProbeTarget{}, fmt.Errorf("unknown tags dialect %q (known: %v)", dialect, TagsDialects())
	}
	v := spec.defaultVariant
	if ev, ok := spec.variantForExt[extOf(relPath)]; ok {
		v = ev
	}
	return ProbeTarget{Dialect: dialect, Blob: v.blob, EntryPoint: v.entryPoint, QueryFile: v.rules.queryFile, Query: v.rules.query}, nil
}
