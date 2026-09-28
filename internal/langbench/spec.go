package langbench

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
)

// Manifest is the subset of a dialect manifest (schema/dialect, design
// §2.2) the harness reads. Unknown fields are ignored here; the CUE schema
// is what refuses them.
type Manifest struct {
	ID            string                       `json:"id"`
	Display       string                       `json:"display"`
	Kind          string                       `json:"kind"`
	LangTag       string                       `json:"lang_tag"`
	Match         ManifestMatch                `json:"match"`
	Kinds         map[string]KindSpec          `json:"kinds"`
	RequiredKinds []string                     `json:"required_kinds"`
	OracleKinds   map[string]map[string]string `json:"oracle_kinds"`
	TargetLevel   string                       `json:"target_level"`
	QueueID       string                       `json:"queue_id"`
}

// ManifestMatch is a manifest's classification keys.
type ManifestMatch struct {
	Extensions []string `json:"extensions"`
	Basenames  []string `json:"basenames"`
}

// KindSpec maps one of a dialect's kinds to a SourceKind and a canonical
// kind (§2.3).
type KindSpec struct {
	SourceKind string `json:"source_kind"`
	Canonical  string `json:"canonical"`
	Container  bool   `json:"container,omitempty"`
	Callable   bool   `json:"callable,omitempty"`
}

// CtagsSpec says how to run universal-ctags for a dialect.
type CtagsSpec struct {
	Languages []string `json:"languages"`
	Args      []string `json:"args,omitempty"`
}

// BenchSpec is what lang-oracle measures a dialect with: its manifest and
// its tier-B oracle's invocation.
type BenchSpec struct {
	Manifest Manifest  `json:"manifest"`
	Ctags    CtagsSpec `json:"ctags"`
	Notes    []string  `json:"notes,omitempty"`
}

// incumbentsJSON describes, in manifest shape, every language registered
// before LB-F1 moves extraction onto dialect manifests: the kinds today's
// extractors emit, mapped to canonical kinds, and the ctags mapping. It is
// the LB-M1 baseline's input; a dialect's own dialect.json supersedes its
// entry once it exists.
//
//go:embed incumbents.json
var incumbentsJSON []byte

// Incumbents returns the embedded incumbent specs, sorted by id.
func Incumbents() ([]BenchSpec, error) {
	var specs []BenchSpec
	if err := json.Unmarshal(incumbentsJSON, &specs); err != nil {
		return nil, fmt.Errorf("incumbents.json: %w", err)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Manifest.ID < specs[j].Manifest.ID })
	return specs, nil
}

// IncumbentSpec returns the embedded spec for id.
func IncumbentSpec(id string) (BenchSpec, bool) {
	specs, err := Incumbents()
	if err != nil {
		return BenchSpec{}, false
	}
	for _, s := range specs {
		if s.Manifest.ID == id {
			return s, true
		}
	}
	return BenchSpec{}, false
}

// ParseManifest decodes a dialect.json for the harness.
func ParseManifest(b []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, err
	}
	if !QueueIDPattern.MatchString(m.ID) {
		return Manifest{}, fmt.Errorf("manifest id %q must match %s", m.ID, QueueIDPattern)
	}
	return m, nil
}

// Owns reports whether relPath is one of this dialect's files by basename
// or extension (the classification keys; sniffing arrives with LB-F9).
func (m Manifest) Owns(relPath string) bool {
	base := path.Base(relPath)
	if slices.Contains(m.Match.Basenames, base) {
		return true
	}
	return slices.Contains(m.Match.Extensions, strings.ToLower(path.Ext(base)))
}

// CanonicalForSource maps a SourceKind the extractor emitted to its
// canonical kind.
func (m Manifest) CanonicalForSource(sourceKind string) (canonical string, callable, ok bool) {
	for _, k := range m.Kinds {
		if k.SourceKind == sourceKind {
			return k.Canonical, k.Callable, true
		}
	}
	return "", false, false
}

// OracleCanonical maps an oracle's kind to a canonical kind. A
// "<kind>@scoped" key, when present, maps an oracle entry that sits inside
// a scope (a ctags func with a receiver is a method).
func (m Manifest) OracleCanonical(oracle, kind string, scoped bool) (string, bool) {
	table := m.OracleKinds[oracle]
	if scoped {
		if c, ok := table[kind+"@scoped"]; ok {
			return c, true
		}
	}
	c, ok := table[kind]
	return c, ok
}

// Intersection is the sorted set of canonical kinds both the dialect and
// the oracle produce (§1.4).
func (m Manifest) Intersection(oracle string) []string {
	mine := map[string]bool{}
	for _, k := range m.Kinds {
		mine[k.Canonical] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range m.OracleKinds[oracle] {
		if mine[c] && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}
