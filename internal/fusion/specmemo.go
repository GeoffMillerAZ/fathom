package fusion

// specmemo.go — the per-ingest file-spec memo (EW-52, O361). A file of a
// file-resolve language is parsed once per ingest: the extract stage reads its
// symbols and its FileSpecs from one parse (TSExtractor.ExtractWithFileSpecs)
// and leaves the specs here, keyed by path and the content hash the file is
// fingerprinted under; resolveLang takes them instead of parsing the file
// again. The memo lives in memory on the ingest's sibling index, so it is
// replaced by the next ingest and holds nothing across one; it is no table, no
// cache and no job. A file the extract stage did not see (stat-skipped, or
// unchanged) is simply absent, and resolveLang reads it as before.

import (
	"sync"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// specMemoOff is a test seam: with it set the extract stage keeps its two-call
// shape, so a test that faults extractImports reaches the resolve pass's parse.
var specMemoOff bool

type specMemoKey struct{ path, lang string }

type specMemoEntry struct {
	hash  string
	specs extractor.FileSpecs
}

// specMemo is one ingest's FileSpecs by file.
type specMemo struct {
	mu sync.Mutex
	m  map[specMemoKey]specMemoEntry
}

func newSpecMemo() *specMemo { return &specMemo{m: map[specMemoKey]specMemoEntry{}} }

// put records path's specs as read from the bytes that hash to hash.
func (s *specMemo) put(path, lang, hash string, sp extractor.FileSpecs) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m[specMemoKey{path, lang}] = specMemoEntry{hash, sp}
	s.mu.Unlock()
}

// take returns, and forgets, path's specs when they were read from the bytes
// that hash to hash under lang.
func (s *specMemo) take(path, lang, hash string) (extractor.FileSpecs, bool) {
	if s == nil {
		return extractor.FileSpecs{}, false
	}
	k := specMemoKey{path, lang}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[k]
	if !ok || e.hash != hash {
		return extractor.FileSpecs{}, false
	}
	delete(s.m, k)
	return e.specs, true
}

// drop forgets every entry of lang (the language's resolve pass is done).
func (s *specMemo) drop(lang string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	for k := range s.m {
		if k.lang == lang {
			delete(s.m, k)
		}
	}
	s.mu.Unlock()
}

// specMemoFor is service's current ingest's memo, nil before any ingest.
func (p *Pipeline) specMemoFor(serviceID string) *specMemo {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx := p.siblingIdx[serviceID]; idx != nil {
		return idx.specs
	}
	return nil
}

// oneParse is ext as the TSExtractor that reads path's symbols and file specs
// from one parse, ok only for a file the file-resolved pass will read under
// ext's language: its extension is one of the language's, or it is a .h the
// router sent to C or C++ (a header read as foreign keeps its own parse).
func (p *Pipeline) oneParse(ext extractor.Extractor, path string, content []byte) (*extractor.TSExtractor, bool) {
	if h, ok := ext.(*extractor.CHeaderExtractor); ok {
		ext = h.Route(content)
	}
	ts, ok := ext.(*extractor.TSExtractor)
	if !ok || p.cfg.DisableTypedEdges || specMemoOff {
		return nil, false
	}
	lang := ts.Name()
	if fileResolveLangFor(path) == lang {
		return ts, true
	}
	if isHeaderPath(path) {
		if c, cpp := headerLangs(); lang != "" && (lang == c || lang == cpp) {
			return ts, true
		}
	}
	return nil, false
}
