package extractor

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
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
	// Preprocess is the dialect's pre-parse step (the manifest's preprocess), nil
	// when it names none: the texts a file that does not parse as written is read as.
	Preprocess func(src []byte) [][]byte
	// PreprocessLenient is the manifest's lenient pre-parse reading: a text
	// with fewer errors than the file as written is read too.
	PreprocessLenient bool
}

// ProbeParse parses src as the extractor reads it: as written, or, when that
// holds an error and the dialect names a pre-parse step, the first of the
// step's texts the extractor would take (tsPreprocessed). The caller closes
// the result's Tree.
func ProbeParse(p *tsengine.Parser, target ProbeTarget, src []byte) (ProbeParseResult, error) {
	tree, err := p.Parse(src)
	if err != nil {
		return ProbeParseResult{}, err
	}
	if target.Preprocess == nil {
		return ProbeParseResult{Tree: tree, Text: src}, nil
	}
	return tsPreprocessedAt(p, tree, src, target.Preprocess, target.PreprocessLenient)
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
	return ProbeTargetForSource(dialect, relPath, nil)
}

// ProbeTargetForSource is ProbeTargetFor for a file whose content may select
// the variant (the css dialect's Tailwind reading, DialectVar.Sniff).
func ProbeTargetForSource(dialect, relPath string, src []byte) (ProbeTarget, error) {
	if _, err := tsRegistry(); err != nil {
		return ProbeTarget{}, err
	}
	spec, ok := tsDialectMap[dialect]
	if !ok {
		return ProbeTarget{}, fmt.Errorf("unknown tags dialect %q (known: %v)", dialect, TagsDialects())
	}
	v := (&TSExtractor{spec: spec}).variantFor(relPath, src)
	return ProbeTarget{Dialect: dialect, Blob: v.blob, EntryPoint: v.entryPoint, QueryFile: v.rules.queryFile, Query: v.rules.query, Preprocess: v.rules.preprocess, PreprocessLenient: v.rules.preprocessLenient}, nil
}

// VariantSniff is the content sniff of the dialect's variant id (DialectVar.
// Sniff), compiled as the extractor compiles it: the test the harness uses to
// measure exactly the files the variant reads.
func VariantSniff(dialect, variant string) (*regexp.Regexp, bool) {
	if _, err := tsRegistry(); err != nil {
		return nil, false
	}
	ds, err := LoadDialects()
	if err != nil {
		return nil, false
	}
	for _, d := range ds {
		if d.ID != dialect {
			continue
		}
		for _, v := range d.Variants {
			if v.ID == variant && v.Sniff != "" {
				re, err := regexp.Compile("(?m)" + v.Sniff)
				return re, err == nil
			}
		}
	}
	return nil, false
}

// VariantSniffWindow is how much of a file the variant sniffs read.
const VariantSniffWindow = tsSniffWindow
