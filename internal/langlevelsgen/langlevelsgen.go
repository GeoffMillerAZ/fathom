// Package langlevelsgen builds the measured-level table the language library
// embeds (ADR-092, LL-12) from docs/bench/readings/lang/*.json. It lives
// outside pkg/lang so the library's closure never carries the generator; the
// library's drift test and its go:generate line both call Generate.
package langlevelsgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Row is one dialect's measured level: the reading's level_met and the grade
// that level was given.
type Row struct {
	ID    string `json:"id"`
	Level string `json:"level"`
	Grade string `json:"grade,omitempty"`
}

type reading struct {
	Dialect  string            `json:"dialect"`
	LevelMet string            `json:"level_met"`
	Grade    map[string]string `json:"grade"`
}

// Generate reads every *.json reading under dir and returns the table as
// indented JSON: rows sorted by dialect id, a trailing newline, and the same
// bytes for the same readings.
func Generate(dir string) ([]byte, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	rows := make([]Row, 0, len(paths))
	seen := map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var r reading
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(p), err)
		}
		if r.Dialect == "" || r.LevelMet == "" {
			return nil, fmt.Errorf("%s: a reading names its dialect and the level met", filepath.Base(p))
		}
		if want := strings.TrimSuffix(filepath.Base(p), ".json"); r.Dialect != want {
			return nil, fmt.Errorf("%s: dialect %q does not match the file name", filepath.Base(p), r.Dialect)
		}
		if prev, dup := seen[r.Dialect]; dup {
			return nil, fmt.Errorf("%s: dialect %q already read by %s", filepath.Base(p), r.Dialect, prev)
		}
		seen[r.Dialect] = filepath.Base(p)
		rows = append(rows, Row{ID: r.Dialect, Level: r.LevelMet, Grade: r.Grade[r.LevelMet]})
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
