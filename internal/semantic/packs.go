package semantic

// packs.go — LC-3 (docs/plans/layer-catalog.md): the pack loader unions
// embedded (built-in) packs with a project's local .fathom/layers/ — presence
// = enabled (ADR-018 reconciler doctrine: no separate enabled/disabled state
// anywhere), a project pack overriding an embedded one of the same name. Also
// the pack validation/install logic `fathom layers install` (LC-7, cmd/fathom)
// drives: users never hand-author CUE, they copy a directory and this package
// validates it against #Layer before it's trusted.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue/cuecontext"
)

// LayerSource identifies where a resolved layer definition came from.
type LayerSource string

const (
	SourceEmbedded LayerSource = "embedded"
	SourceProject  LayerSource = "project"
)

// LayerInfo is one resolved layer with union provenance — the data source
// for `fathom layers list`/`show` (LC-7).
type LayerInfo struct {
	LayerDef
	Source LayerSource
	// Overrides is true when Source is project and an embedded pack of the
	// same name also exists: the project pack wins the union (presence =
	// enabled, project-over-embedded — no other precedence rule, no
	// enabled/disabled bit to set).
	Overrides bool
}

// ProjectLayersDir is the project-local pack root, relative to a project's
// repo root (docs/vision/02-shape.md §3, docs/plans/layer-catalog.md LC-3):
// "<root>/.fathom/layers/<name>/{layer.cue,prompt.md,eval/,KILL.md}".
const ProjectLayersDir = ".fathom/layers"

// LoadLayerIn resolves the named layer from the union of embedded packs and
// root's project-local .fathom/layers/ (LC-3). root == "" (or a root with no
// .fathom/layers/ directory at all) is exactly LoadLayer's embedded-only
// result. A project pack overrides an embedded pack of the same name.
func LoadLayerIn(root, name string) (LayerDef, error) {
	infos, err := loadUnion(root)
	if err != nil {
		return LayerDef{}, err
	}
	info, ok := infos[name]
	if !ok {
		return LayerDef{}, fmt.Errorf("semantic: unknown layer %q (known layers: %s)", name, strings.Join(sortedInfoNames(infos), ", "))
	}
	return info.LayerDef, nil
}

// ListLayersIn returns every resolved layer (embedded ∪ project) sorted by
// name, with source provenance.
func ListLayersIn(root string) ([]LayerInfo, error) {
	infos, err := loadUnion(root)
	if err != nil {
		return nil, err
	}
	names := sortedInfoNames(infos)
	out := make([]LayerInfo, 0, len(names))
	for _, n := range names {
		out = append(out, infos[n])
	}
	return out, nil
}

func sortedInfoNames(infos map[string]LayerInfo) []string {
	names := make([]string, 0, len(infos))
	for n := range infos {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// loadUnion compiles #Layer once and decodes every embedded pack, then every
// project-local pack under root, into a name-keyed map — a later (project)
// entry overwrites an earlier (embedded) one of the same name, which is the
// override rule LayerInfo.Overrides records.
func loadUnion(root string) (map[string]LayerInfo, error) {
	cctx := cuecontext.New()
	layerSchema, err := compileLayerSchema(cctx)
	if err != nil {
		return nil, err
	}

	embeddedNames, err := packNames(embeddedLayers, "layers")
	if err != nil {
		return nil, fmt.Errorf("semantic: list embedded layer packs: %w", err)
	}
	infos := map[string]LayerInfo{}
	for _, name := range embeddedNames {
		def, err := decodePack(cctx, layerSchema, embeddedLayers, "layers/"+name+"/layer.cue", name)
		if err != nil {
			return nil, err
		}
		infos[name] = LayerInfo{LayerDef: def, Source: SourceEmbedded}
	}

	if root == "" {
		return infos, nil
	}
	projectDir := filepath.Join(root, filepath.FromSlash(ProjectLayersDir))
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return infos, nil // presence = enabled; no .fathom/layers/ is fine
		}
		return nil, fmt.Errorf("semantic: list project layer packs (%s): %w", projectDir, err)
	}
	projFS := os.DirFS(projectDir)
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		def, err := decodePack(cctx, layerSchema, projFS, name+"/layer.cue", name)
		if err != nil {
			return nil, fmt.Errorf("semantic: project pack %q (%s): %w", name, projectDir, err)
		}
		_, overrides := infos[name]
		infos[name] = LayerInfo{LayerDef: def, Source: SourceProject, Overrides: overrides}
	}
	return infos, nil
}

// ValidatePack validates a candidate pack directory on disk against #Layer
// WITHOUT installing it: layer.cue must parse and satisfy #Layer. A missing
// prompt.md or empty/absent eval/ is reported as a WARNING, not a failure —
// docs/design/semantic-layers.md §4.6's precision/recall gate is what a pack
// without eval/ hasn't cleared yet, and `fathom layers install` still lets an
// operator install a work-in-progress pack; it just can't compile until
// prompt.md exists. Returns the decoded LayerDef and any warnings.
func ValidatePack(dir string) (LayerDef, []string, error) {
	cctx := cuecontext.New()
	layerSchema, err := compileLayerSchema(cctx)
	if err != nil {
		return LayerDef{}, nil, err
	}
	name := filepath.Base(filepath.Clean(dir))
	def, err := decodePack(cctx, layerSchema, os.DirFS(dir), "layer.cue", name)
	if err != nil {
		return LayerDef{}, nil, err
	}
	var warnings []string
	if def.PromptText == "" {
		warnings = append(warnings, fmt.Sprintf("pack %q: prompt.md is missing or unreadable (prompt_file: %q) — compiling this layer will fail until it's added", name, def.PromptFile))
	}
	evalEntries, everr := os.ReadDir(filepath.Join(dir, "eval"))
	if everr != nil || len(evalEntries) == 0 {
		warnings = append(warnings, fmt.Sprintf("pack %q: eval/ is missing or empty — this layer has not been precision/recall-gated (docs/design/semantic-layers.md §4.6)", name))
	}
	return def, warnings, nil
}

// InstallPack validates srcDir (see ValidatePack) then copies it verbatim
// into root's project-local pack directory (<root>/.fathom/layers/<name>/,
// where <name> is the pack's OWN declared name — not necessarily srcDir's
// basename, though ValidatePack already requires them to match). Users never
// hand-author CUE for this: install copies packs; the shipped packs are
// authored once, under the gate. Returns the validated LayerDef and any
// non-fatal warnings (e.g. missing eval/).
func InstallPack(srcDir, root string) (LayerDef, []string, error) {
	def, warnings, err := ValidatePack(srcDir)
	if err != nil {
		return LayerDef{}, nil, fmt.Errorf("semantic: pack at %s failed validation: %w", srcDir, err)
	}
	destDir := filepath.Join(root, filepath.FromSlash(ProjectLayersDir), def.Name)
	if err := copyPackDir(srcDir, destDir); err != nil {
		return LayerDef{}, nil, fmt.Errorf("semantic: install pack %q: %w", def.Name, err)
	}
	return def, warnings, nil
}

// copyPackDir recursively copies src's contents into dst (creating dst).
func copyPackDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
