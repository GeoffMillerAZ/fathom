package extractor

// specextractor.go — the one-parse contract (EW-52, EW-53) on its own, with no
// CGO tree-sitter import (LB-F394): fusion's extract stage (specmemo.go,
// extract_sink.go) asks it of every extractor, the tags engine's TSExtractor
// answers it. The native-tree helpers a CGO incumbent used to answer it
// (nativespecs.go, EW-53) left with the last CGO walkers, rustextract.go and
// scalaextract.go, in wave lf3.

import "github.com/geoffmilleraz/fathom/internal/core"

// SpecExtractor is an Extractor that also reads a file's FileSpecs from the
// tree it parsed to extract (EW-52, EW-53): the extract stage asks it instead
// of Extract for a file the file-resolved pass will read, so the file is
// parsed once. SpecLang is the file-resolve language the specs belong to.
// ok is false when the specs could not be taken from this parse; the caller
// then leaves them to the resolve pass.
type SpecExtractor interface {
	Extractor
	SpecLang() string
	ExtractWithFileSpecs(service, relPath string, src []byte) (nodes []core.SymbolNode, specs FileSpecs, ok bool, err error)
}

// SpecLang is the file-resolve language this extractor's specs belong to.
func (e *TSExtractor) SpecLang() string { return e.spec.name }
