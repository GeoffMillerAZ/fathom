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

// TagsDialects returns the sorted ids of every dialect on the tags engine.
// Until LB-F1 builds the registry from manifests, these are TSExtractor's
// built-in languages.
func TagsDialects() []string {
	ids := make([]string, 0, len(tsRegistry))
	for id := range tsRegistry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// ProbeTargetFor resolves dialect and a file path to the variant the tags
// engine would parse that file with, and that variant's query source.
func ProbeTargetFor(dialect, relPath string) (ProbeTarget, error) {
	spec, ok := tsRegistry[dialect]
	if !ok {
		return ProbeTarget{}, fmt.Errorf("unknown tags dialect %q (known: %v)", dialect, TagsDialects())
	}
	v := spec.defaultVariant
	if ev, ok := spec.variantForExt[extOf(relPath)]; ok {
		v = ev
	}
	src, err := tsQueryFS.ReadFile("tsqueries/" + v.queryFile)
	if err != nil {
		return ProbeTarget{}, fmt.Errorf("read query %s: %w", v.queryFile, err)
	}
	return ProbeTarget{Dialect: dialect, Blob: v.blob, EntryPoint: v.entryPoint, QueryFile: v.queryFile, Query: string(src)}, nil
}
