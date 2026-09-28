package langbench

// dialectdata.go — the dialect data directories and their grammar pins as
// the console's Support page shows them (support-matrix SM-22, design
// docs/design/support-matrix.md §5 panel 2): each directory's manifest
// identity and engine, and grammars.lock.json's pin, licence and embedded
// (gzip) size per blob. Only committed JSON is read; nothing is compiled.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// DialectData is one dialect data directory's manifest identity.
type DialectData struct {
	ID      string
	Display string
	Engine  string
	// Blob is the grammar blob the manifest names; "" when the engine
	// embeds none.
	Blob string
}

// GrammarPin is one grammars.lock.json row as the Support page names it.
type GrammarPin struct {
	Blob    string `json:"blob"`
	Repo    string `json:"repo"`
	Tag     string `json:"tag"`
	Licence string `json:"licence"`
	GzBytes int64  `json:"gz_bytes"`
}

// DialectDataFromRepo reads every internal/extractor/dialects/*/dialect.json
// under root, sorted by id. No directory is an empty list, not an error.
func DialectDataFromRepo(root string) ([]DialectData, error) {
	paths, err := filepath.Glob(filepath.Join(root, "internal", "extractor", "dialects", "*", "dialect.json"))
	if err != nil {
		return nil, err
	}
	var out []DialectData
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var m struct {
			ID      string `json:"id"`
			Display string `json:"display"`
			Engine  string `json:"engine"`
			Grammar *struct {
				Blob string `json:"blob"`
			} `json:"grammar"`
		}
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		d := DialectData{ID: m.ID, Display: m.Display, Engine: m.Engine}
		if m.Grammar != nil {
			d.Blob = m.Grammar.Blob
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// GrammarPinsFromRepo reads internal/extractor/tsengine/grammars/
// grammars.lock.json under root, keyed by blob. A missing lock is an empty
// map (a checkout with no embedded grammar), not an error.
func GrammarPinsFromRepo(root string) (map[string]GrammarPin, error) {
	b, err := os.ReadFile(filepath.Join(root, "internal", "extractor", "tsengine", "grammars", "grammars.lock.json"))
	if os.IsNotExist(err) {
		return map[string]GrammarPin{}, nil
	}
	if err != nil {
		return nil, err
	}
	var lock struct {
		Grammars []GrammarPin `json:"grammars"`
	}
	if err := json.Unmarshal(b, &lock); err != nil {
		return nil, fmt.Errorf("grammars.lock.json: %w", err)
	}
	out := make(map[string]GrammarPin, len(lock.Grammars))
	for _, g := range lock.Grammars {
		out[g.Blob] = g
	}
	return out, nil
}
