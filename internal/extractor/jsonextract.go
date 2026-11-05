package extractor

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// JSONExtractor implements Extractor for JSON files (.json).
// It uses encoding/json for a real parse and emits one KindJSONKey node per
// TOP-LEVEL key when the root is a JSON object.
//
// Design decisions:
//   - Only top-level keys are emitted; nested keys are intentionally skipped so
//     large files (e.g. package-lock.json) stay cheap to process.
//   - If the root is not a JSON object (array, string, number, …) the extractor
//     returns nil, nil (zero nodes, no error).
//   - On invalid JSON the extractor returns nil, nil (best-effort).
//   - Line is always 0 — encoding/json does not expose token positions without
//     additional over-engineering.
//   - Tier is TierVerified because the parse is real (not pattern-matched).
//   - Output is sorted by Name for determinism.
type JSONExtractor struct{}

// NewJSONExtractor returns a new JSONExtractor.
func NewJSONExtractor() *JSONExtractor { return &JSONExtractor{} }

// Extract parses JSON source bytes and returns one SymbolNode per top-level
// object key. Non-object roots and invalid JSON yield nil, nil.
func (j *JSONExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	nodes, _, err := j.ExtractReport(service, relPath, src)
	return nodes, err
}

// ExtractReport is Extract and, when the file could not be decoded, the reason
// (ReportingExtractor).
func (j *JSONExtractor) ExtractReport(service, relPath string, src []byte) ([]core.SymbolNode, string, error) {
	pkg := pkgFromPath(relPath)

	if textTooDeep(src, "[{", "]}", false) {
		return nil, errTooDeep("json decode"), nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(src, &root); err != nil {
		// Not a JSON object (array/scalar/invalid) — return nothing, no error.
		return nil, jsonReason(src, err), nil
	}

	if len(root) == 0 {
		return nil, "", nil
	}

	// Collect keys in sorted order for determinism.
	keys := make([]string, 0, len(root))
	for k := range root {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	nodes := make([]core.SymbolNode, 0, len(keys))
	for _, k := range keys {
		nodes = append(nodes, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     k,
			Kind:     core.KindJSONKey,
			FilePath: relPath,
			Line:     0,
			Tier:     core.TierVerified,
		})
	}

	return nodes, "", nil
}

// jsonReason is the reason for a JSON file the extractor could not decode: only
// text that is not JSON counts, not a valid document whose root is no object.
func jsonReason(src []byte, err error) string {
	if json.Valid(src) || len(bytes.TrimSpace(src)) == 0 {
		return ""
	}
	return incompleteReason("json decode", err)
}
