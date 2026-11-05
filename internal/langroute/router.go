// Package langroute decides which extractor reads a file (ADR-092, the
// language library). A Router owns the extension, basename, tool-kind and
// content-sniff tables and the dispatch over them; the classifiers, the
// dialect gate and the binary gate live beside it. It imports only core,
// extractor, langclass and tsengine, never the connector, store or fusion, so
// a consumer that wants language understanding does not link the product.
package langroute

import (
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// FormatDispatch claims a file for a product-only format (fathom's own
// configuration and plan files). A Router built without one skips those files.
type FormatDispatch func(relPath string, content []byte) (extractor.Extractor, ToolKind, bool)

// Router is the dispatch table: which extractor reads a file, and the tool kind
// its observations carry. Build one with NewRouter. A Router is not safe to
// modify (RegisterExtractor) while another goroutine resolves a file.
type Router struct {
	exts      map[string]extractor.Extractor   // extension or basename -> extractor
	toolExts  map[ToolKind]extractor.Extractor // ADR-021 §2: yaml-family extractors keyed by classifier verdict
	sniffExts map[string]extractor.Extractor   // dialect id -> extractor, for a dialect that claims an extension only through match.sniff (LB-Q-matlab)
	sniffExt  map[string]bool                  // the lowercased extensions those sniffs settle (sniffedExtractor reads only these files)

	timoni  extractor.Extractor // the Timoni dialect's extractor; nil skips Timoni files
	formats FormatDispatch      // product-only formats; nil skips them
}

// Option configures NewRouter.
type Option func(*Router)

// WithCUE registers the CUE extractor for .cue files. A Router without it
// leaves .cue files unclaimed, which keeps cuelang.org/go out of the closure.
func WithCUE(e extractor.Extractor) Option {
	return func(r *Router) { r.exts[".cue"] = e }
}

// WithTimoni registers the Timoni dialect's extractor. A Router without it
// does not claim Timoni modules, bundles or runtimes.
func WithTimoni(e extractor.Extractor) Option {
	return func(r *Router) { r.timoni = e }
}

// WithFormats registers a product-only format dispatch (fathom's configuration
// and plan files). A Router without it skips those files.
func WithFormats(d FormatDispatch) Option {
	return func(r *Router) { r.formats = d }
}

// Exts is the extension and basename table. The caller may read it and write
// it (a registration); the Router does not copy it.
func (r *Router) Exts() map[string]extractor.Extractor { return r.exts }

// ToolExts is the tool-kind table.
func (r *Router) ToolExts() map[ToolKind]extractor.Extractor { return r.toolExts }

// SniffExts is the sniff-claimed dialect table.
func (r *Router) SniffExts() map[string]extractor.Extractor { return r.sniffExts }

// formatDispatch is the product-only format claim, or no claim when none was
// registered.
func (r *Router) formatDispatch(relPath string, content []byte) (extractor.Extractor, ToolKind, bool) {
	if r.formats == nil {
		return nil, "", false
	}
	return r.formats(relPath, content)
}

// Close releases the extractors' pooled modules.
func (r *Router) Close() {
	seen := map[*extractor.TSExtractor]bool{}
	for _, e := range r.exts {
		if ts, ok := e.(*extractor.TSExtractor); ok && !seen[ts] {
			seen[ts] = true
			ts.Close()
		}
	}
	for _, e := range r.exts {
		if td, ok := e.(*extractor.TagsDocExtractor); ok {
			td.Close() // the conan dialect owns one tags engine, shared by its four basenames
			break
		}
	}
	for _, e := range r.sniffExts {
		if ts, ok := e.(*extractor.TSExtractor); ok && !seen[ts] {
			seen[ts] = true
			ts.Close()
		}
		if td, ok := e.(*extractor.TagsDocExtractor); ok {
			td.Close() // the checkov-policies dialect's tags engine, reached by glob
		}
	}
}
