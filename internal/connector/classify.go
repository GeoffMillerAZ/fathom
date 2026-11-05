package connector

import (
	"io"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langclass"
)

// The file classifiers live in the leaf internal/langclass (ADR-092), so a
// caller that only classifies a path does not link the extractors. This file
// forwards them, so connector's importers do not change.

// DialectRule is langclass.DialectRule.
type DialectRule = langclass.DialectRule

// DialectSniffRule is langclass.DialectSniffRule.
type DialectSniffRule = langclass.DialectSniffRule

// DialectVerdict is langclass.DialectVerdict.
type DialectVerdict = langclass.DialectVerdict

// How a DialectVerdict was reached.
const (
	ClassifiedByBasename  = langclass.ClassifiedByBasename
	ClassifiedByGlob      = langclass.ClassifiedByGlob
	ClassifiedByExtension = langclass.ClassifiedByExtension
	ClassifiedByShebang   = langclass.ClassifiedByShebang
	ClassifiedBySniff     = langclass.ClassifiedBySniff
	ClassifiedAmbiguous   = langclass.ClassifiedAmbiguous
	ClassifiedNone        = langclass.ClassifiedNone
)

// classification is langclass.Classification.
type classification = langclass.Classification

// ClassifyDialect forwards to langclass.ClassifyDialect.
func ClassifyDialect(rules []DialectRule, relPath string, head []byte) DialectVerdict {
	return langclass.ClassifyDialect(rules, relPath, head)
}

// IsBinary forwards to langclass.IsBinary.
func IsBinary(data []byte) bool { return langclass.IsBinary(data) }

// IsGenerated forwards to langclass.IsGenerated.
func IsGenerated(path string, content []byte) bool { return langclass.IsGenerated(path, content) }

// IsGeneratedReader forwards to langclass.IsGeneratedReader.
func IsGeneratedReader(path string, r io.Reader) (bool, error) {
	return langclass.IsGeneratedReader(path, r)
}

// IsGeneratedOrMinified forwards to langclass.IsGeneratedOrMinified.
func IsGeneratedOrMinified(path string, content []byte) bool {
	return langclass.IsGeneratedOrMinified(path, content)
}

// LanguageForPath forwards to langclass.LanguageForPath.
func LanguageForPath(path string) string { return langclass.LanguageForPath(path) }

// RegisteredExtensions forwards to langclass.RegisteredExtensions.
func RegisteredExtensions() []string { return langclass.RegisteredExtensions() }

// classify forwards to langclass.Classify.
func classify(base, ext string) (classification, bool) { return langclass.Classify(base, ext) }

// kindForFile forwards to langclass.KindForFile.
func kindForFile(base, ext string) core.SourceKind { return langclass.KindForFile(base, ext) }

// containsFold reports whether list holds the already-lowercased ext
// (case-insensitive on the list side), as modelcontainer.go needs it.
func containsFold(list []string, ext string) bool {
	for _, e := range list {
		if strings.ToLower(e) == ext {
			return true
		}
	}
	return false
}

// ClaimedModelContainer forwards to langclass.ClaimedModelContainer.
func ClaimedModelContainer(rules []DialectRule, relPath string, data []byte) bool {
	return langclass.ClaimedModelContainer(rules, relPath, data)
}
