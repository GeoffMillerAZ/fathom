// Package adoption checks explicit, consumer-owned declarations. It never
// executes a project, resolves its build graph, or claims runtime adoption.
package adoption

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"golang.org/x/mod/semver"
)

const DefaultManifest = "appkit.adoption.json"
const MaxManifestBytes = 64 << 10
const MaxFileBytes = 8 << 20
const MaxTotalBytes = 64 << 20

//go:embed schema.cue
var schema []byte

type ModuleRequirement struct {
	File              string `json:"file"`
	Version           string `json:"version"`
	AllowLocalReplace bool   `json:"allow_local_replace"`
}

type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Selection struct {
	SchemaVersion       int                   `json:"schema_version"`
	Consumer            string                `json:"consumer"`
	Profile             string                `json:"profile"`
	Layers              []string              `json:"layers"`
	Capabilities        []string              `json:"capabilities"`
	CatalogRevision     string                `json:"catalog_revision,omitempty"`
	CapabilityRevisions *[]CapabilityRevision `json:"capability_revisions,omitempty"`
	Contract            string                `json:"contract"`
	Module              *ModuleRequirement    `json:"module,omitempty"`
	Artifacts           []Artifact            `json:"artifacts"`
	Agents              bool                  `json:"agents"`
}

// Decode rejects duplicate/unknown/case-aliased fields and executable CUE input.
// CUE authors the schema; consumer files are bounded, inert JSON only.
func Decode(body []byte) (Selection, error) {
	var s Selection
	invalid := errors.New("adoption: invalid version-1 or version-2 selection JSON or schema")
	if len(body) > MaxManifestBytes || !utf8.Valid(body) || !json.Valid(body) {
		return s, invalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	if !uniqueJSON(d, 0) {
		return s, invalid
	}
	if _, err := d.Token(); err != io.EOF {
		return s, invalid
	}
	ctx := cuecontext.New()
	v := ctx.CompileBytes(schema).LookupPath(cue.ParsePath("#Selection")).Unify(ctx.CompileBytes(body))
	if v.Validate(cue.Concrete(true)) != nil || v.Decode(&s) != nil {
		return Selection{}, invalid
	}
	if !SafePath(s.Contract) || len(s.Consumer) > 80 || len(s.Profile) > 40 || len(s.Layers) > 6 || len(s.Capabilities) > 64 || len(s.Artifacts) > 64 {
		return Selection{}, invalid
	}
	if s.Module != nil && (!SafePath(s.Module.File) || !semver.IsValid(s.Module.Version) || semver.Canonical(s.Module.Version) != s.Module.Version) {
		return Selection{}, invalid
	}
	if len(s.Capabilities) == 0 && len(s.Artifacts) == 0 && s.Module == nil && !s.Agents {
		return Selection{}, errors.New("adoption: selection has no capability, artifact, module or agent routing requirement")
	}
	for _, values := range [][]string{s.Layers, s.Capabilities} {
		seen := map[string]bool{}
		for _, value := range values {
			if seen[value] || len(value) > 120 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
				return Selection{}, invalid
			}
			seen[value] = true
		}
		slices.Sort(values)
	}
	if s.SchemaVersion == 2 {
		if s.CapabilityRevisions == nil || len(*s.CapabilityRevisions) != len(s.Capabilities) {
			return Selection{}, invalid
		}
		pins := *s.CapabilityRevisions
		slices.SortFunc(pins, func(a, b CapabilityRevision) int { return strings.Compare(a.ID, b.ID) })
		for i, pin := range pins {
			if pin.ID != s.Capabilities[i] {
				return Selection{}, invalid
			}
		}
	}
	seen := map[string]bool{}
	for _, artifact := range s.Artifacts {
		if !SafePath(artifact.Path) || seen[artifact.Path] {
			return Selection{}, invalid
		}
		seen[artifact.Path] = true
	}
	slices.SortFunc(s.Artifacts, func(a, b Artifact) int { return strings.Compare(a.Path, b.Path) })
	return s, nil
}

func uniqueJSON(d *json.Decoder, depth int) bool {
	if depth > 12 {
		return false
	}
	token, err := d.Token()
	if err != nil {
		return false
	}
	switch token {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return false
			}
			seen[name] = true
			if !uniqueJSON(d, depth+1) {
				return false
			}
		}
		last, err := d.Token()
		return err == nil && last == json.Delim('}')
	case json.Delim('['):
		for d.More() {
			if !uniqueJSON(d, depth+1) {
				return false
			}
		}
		last, err := d.Token()
		return err == nil && last == json.Delim(']')
	}
	return true
}

// SafePath names one canonical portable relative file, never a root or traversal.
func SafePath(p string) bool {
	return utf8.ValidString(p) && p != "" && p != "." && p != ".." && len(p) <= 512 && path.Clean(p) == p && !path.IsAbs(p) && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\\:") && strings.IndexFunc(p, unicode.IsControl) < 0
}

func hash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// Router prints a short consumer-owned handoff, not kit-wide policy inheritance.
func Router(contract string) (string, error) {
	if !SafePath(contract) || strings.ContainsAny(contract, "`<>[]") {
		return "", errors.New("adoption: contract must be a safe relative documentation path")
	}
	return "<!-- appkit:begin -->\n## App-kit\n\nApp-kit adoption lives at " + contract + ": selected layers/profile, design context,\nrevision, proof commands and divergences. Before material UI/CLI work, consult it and\nthe kit's intent router through appkit MCP or the pinned local docs. Select a fitting\ncapability/recipe; verify the actual task and states in the target UI. Record reusable\nimprovements or a reasoned divergence and return applicable fixes upstream.\n<!-- appkit:end -->\n", nil
}
