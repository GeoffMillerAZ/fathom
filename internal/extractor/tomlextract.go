package extractor

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	toml "github.com/pelletier/go-toml/v2"
)

// TOMLExtractor implements Extractor for TOML configuration files (.toml).
// It uses github.com/pelletier/go-toml/v2 for parsing and emits:
//   - KindTOMLTable for top-level keys whose value is a map (a table)
//   - KindTOMLKey   for all other top-level keys
//
// Parsing is best-effort: on error the extractor returns whatever could be
// decoded, never a non-nil error.
type TOMLExtractor struct{}

// NewTOMLExtractor returns a new TOMLExtractor.
func NewTOMLExtractor() *TOMLExtractor { return &TOMLExtractor{} }

// Extract parses TOML source bytes and returns SymbolNodes for each top-level
// key. Output is sorted by Name for determinism (line is always 0 for TOML).
func (t *TOMLExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)

	var m map[string]any
	_ = toml.Unmarshal(src, &m) // best-effort; partial decode is fine

	if len(m) == 0 {
		return nil, nil
	}

	// Collect keys in sorted order for determinism.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	nodes := make([]core.SymbolNode, 0, len(keys))
	for _, k := range keys {
		v := m[k]
		kind := core.KindTOMLKey
		if _, isMap := v.(map[string]any); isMap {
			kind = core.KindTOMLTable
		}
		nodes = append(nodes, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     k,
			Kind:     kind,
			FilePath: relPath,
			Line:     0,
			Tier:     core.TierVerified,
		})
	}

	return nodes, nil
}
