package extractor

// tagsdoc.go — the tags+doc engine (design §2.2's `engine: "tags+doc"`,
// LB-Q-helm): one dialect whose files split across the two engines. A Helm
// chart is both — Chart.yaml and values.yaml are documents (the doc engine),
// templates/*.tpl and templates/*.yaml are Go templates (the tags engine over
// the go-template grammar) — and one manifest owns them all. The doc spec's
// `detect` (basenames, globs) says which files are documents; every other
// file the manifest matches goes to the tags engine.

import (
	"fmt"
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// DocResulter is what the cross-file pass needs of an extractor
// (fusion.enrichDocXrefEdges, indexbench's xref scoring): the doc engine's
// full result for one file. *DocExtractor and *TagsDocExtractor are both.
type DocResulter interface {
	ExtractDoc(service, relPath string, src []byte) (DocResult, error)
}

// TagsDocExtractor routes one tags+doc dialect's files to its two engines.
type TagsDocExtractor struct {
	id     string
	ts     *TSExtractor
	doc    *DocExtractor
	detect DocDetect
}

var (
	_ Extractor   = (*TagsDocExtractor)(nil)
	_ DocResulter = (*TagsDocExtractor)(nil)
)

// NewTagsDocExtractor builds the extractor for a tags+doc dialect id.
func NewTagsDocExtractor(id string) (*TagsDocExtractor, error) {
	ds, err := LoadDialects()
	if err != nil {
		return nil, err
	}
	for _, d := range ds {
		if d.ID != id {
			continue
		}
		if d.Engine != "tags+doc" {
			return nil, fmt.Errorf("dialect %q is engine %q, not tags+doc", id, d.Engine)
		}
		doc, err := LoadDocExtractor(d)
		if err != nil {
			return nil, err
		}
		det := doc.spec.Detect
		if len(det.Basenames) == 0 && len(det.Globs) == 0 {
			return nil, fmt.Errorf("dialect %q: a tags+doc doc.json must name detect.basenames or detect.globs, the files the doc engine reads", id)
		}
		ts, err := NewTSExtractor(id)
		if err != nil {
			return nil, err
		}
		return &TagsDocExtractor{id: id, ts: ts, doc: doc, detect: det}, nil
	}
	return nil, fmt.Errorf("dialect %q not found (tags+doc engine)", id)
}

// NewDialectExtractor is the one constructor a harness needs for any
// embedded dialect it measures through its own engine: tags, doc or tags+doc.
func NewDialectExtractor(id string) (Extractor, error) {
	ds, err := LoadDialects()
	if err != nil {
		return nil, err
	}
	for _, d := range ds {
		if d.ID != id {
			continue
		}
		switch d.Engine {
		case "tags":
			return NewTSExtractor(id)
		case "doc":
			return NewDocDialectExtractor(id)
		case "tags+doc":
			return NewTagsDocExtractor(id)
		}
		return nil, fmt.Errorf("dialect %q: engine %q has no extractor here", id, d.Engine)
	}
	return nil, fmt.Errorf("dialect %q not found", id)
}

// IsDocFile reports whether relPath is a document the doc engine reads: its
// basename is one of the doc spec's detect.basenames or it matches one of
// its detect.globs.
func (x *TagsDocExtractor) IsDocFile(relPath string) bool {
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	base := path.Base(relPath)
	for _, b := range x.detect.Basenames {
		if b == base {
			return true
		}
	}
	for _, g := range x.detect.Globs {
		if docGlobMatch(g, relPath) {
			return true
		}
	}
	return false
}

// Extract sends a document to the doc engine and every other file to the
// tags engine.
func (x *TagsDocExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	if x.IsDocFile(relPath) {
		return x.doc.Extract(service, relPath, src)
	}
	return x.ts.Extract(service, relPath, src)
}

// ExtractDoc is the doc engine's full result for a document; a file the tags
// engine owns has none.
func (x *TagsDocExtractor) ExtractDoc(service, relPath string, src []byte) (DocResult, error) {
	if !x.IsDocFile(relPath) {
		return DocResult{}, nil
	}
	return x.doc.ExtractDoc(service, relPath, src)
}

// TypeRefs, EngineFaults and Close are the tags engine's (the harness reads
// them off the extractor it measures).
func (x *TagsDocExtractor) TypeRefs() int64     { return x.ts.TypeRefs() }
func (x *TagsDocExtractor) EngineFaults() int64 { return x.ts.EngineFaults() }
func (x *TagsDocExtractor) Close()              { x.ts.Close() }

// docGlobMatch matches a slash path against a manifest glob: path.Match per
// segment, `**` matching zero or more segments (the connector's rule, §5).
func docGlobMatch(pattern, p string) bool {
	return docGlobSegs(strings.Split(pattern, "/"), strings.Split(p, "/"))
}

func docGlobSegs(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for i := 0; i <= len(segs); i++ {
				if docGlobSegs(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, err := path.Match(pat[0], segs[0]); err != nil || !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}
