package semantic

// packs.go — LC-3 (docs/plans/layer-catalog.md): the pack loader unions
// embedded (built-in) packs with a project's local .fathom/layers/ — presence
// = enabled for COMPILATION (ADR-018 reconciler doctrine: no separate
// enabled/disabled state anywhere), a project pack overriding an embedded one
// of the same name. SERVING derives from the STORE, not from pack presence —
// see cmd/fathom/layers.go's doc comment and docs/layers.md for the honest
// uninstall story (LC-13). Also the pack validation/install logic
// `fathom layers install` (LC-7, cmd/fathom) drives: users never hand-author
// CUE, they copy a directory and this package validates it against #Layer
// before it's trusted.
//
// Resilience contract (critic finding 1 on the LC slice): one broken project
// pack must never take down the whole union. A pack directory whose layer.cue
// is missing/undecodable is SKIPPED from resolution (embedded packs — and an
// embedded pack the broken dir would have overridden — keep serving), warned
// about once per process on stderr, and surfaced in ListLayersIn via
// LayerInfo.LoadError so `fathom layers list` shows the breakage instead of
// hiding it. Only asking for the broken pack BY NAME errors loudly.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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
	// LoadError is non-empty when a project pack directory of this name
	// exists but failed to load/validate. When an embedded pack of the same
	// name also exists, the embedded definition stays live (Source stays
	// "embedded" — a broken override must not take the built-in down with
	// it) and this field records why the project copy was ignored. When the
	// broken pack is project-only, the entry carries no usable definition
	// and LoadLayerIn rejects it by name, loudly.
	LoadError string
}

// ProjectLayersDir is the project-local pack root, relative to a project's
// repo root (docs/vision/02-shape.md §3, docs/plans/layer-catalog.md LC-3):
// "<root>/.fathom/layers/<name>/{layer.cue,prompt.md,eval/,KILL.md}".
const ProjectLayersDir = ".fathom/layers"

// LoadLayerIn resolves the named layer from the union of embedded packs and
// root's project-local .fathom/layers/ (LC-3). root == "" (or a root with no
// .fathom/layers/ directory at all) is exactly LoadLayer's embedded-only
// result. A project pack overrides an embedded pack of the same name. A
// BROKEN project pack (see LayerInfo.LoadError) never breaks other layers'
// resolution: requesting it by name errors; requesting anything else works,
// falling back to the embedded definition when the broken pack shadowed one.
func LoadLayerIn(root, name string) (LayerDef, error) {
	infos, err := loadUnion(root)
	if err != nil {
		return LayerDef{}, err
	}
	info, ok := infos[name]
	if !ok {
		return LayerDef{}, fmt.Errorf("semantic: unknown layer %q (known layers: %s)", name, strings.Join(sortedInfoNames(infos), ", "))
	}
	if info.LoadError != "" && info.Kind == "" {
		// Project-only broken pack: there is no usable definition anywhere.
		return LayerDef{}, fmt.Errorf("semantic: layer %q: project pack failed to load: %s", name, info.LoadError)
	}
	return info.LayerDef, nil
}

// ListLayersIn returns every resolved layer (embedded ∪ project) sorted by
// name, with source provenance. Broken project packs appear with LoadError
// set (see LayerInfo) — visible, never silently dropped.
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

// brokenPackWarned dedupes the once-per-process stderr warnings loadUnion
// emits for broken project packs — loud enough to be seen, quiet enough not
// to spam a serve-time hot path that resolves the union on every call.
var (
	brokenPackWarnMu sync.Mutex
	brokenPackWarned = map[string]bool{}
)

func warnBrokenPackOnce(key, format string, args ...any) {
	brokenPackWarnMu.Lock()
	defer brokenPackWarnMu.Unlock()
	if brokenPackWarned[key] {
		return
	}
	brokenPackWarned[key] = true
	fmt.Fprintf(os.Stderr, "fathom: semantic: "+format+"\n", args...)
}

// loadUnion compiles #Layer once and decodes every embedded pack, then every
// project-local pack under root, into a name-keyed map — a later (project)
// entry overwrites an earlier (embedded) one of the same name, which is the
// override rule LayerInfo.Overrides records. Per the resilience contract in
// the package doc: a project pack that fails to decode is skipped (recorded
// via LoadError, warned once on stderr), and an unreadable .fathom/layers/
// directory falls back to embedded-only resolution — neither ever fails the
// union, because DriftedFindings/ObligationsFor resolve through here on the
// serve hot path and one half-authored pack must not silently disable
// code_drifted (or anything else) for every OTHER layer.
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
		warnBrokenPackOnce(projectDir, "project layer directory %s is unreadable and is being ignored (embedded packs still serve): %v", projectDir, err)
		return infos, nil
	}
	projFS := os.DirFS(projectDir)
	var names []string
	for _, e := range entries {
		// Hidden directories (".install-*" staging dirs, editor debris) are
		// never packs.
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		def, err := decodePack(cctx, layerSchema, projFS, name+"/layer.cue", name)
		if err != nil {
			warnBrokenPackOnce(projectDir+"/"+name, "project layer pack %q at %s failed to load and is being ignored (other packs still serve): %v", name, projectDir, err)
			if prev, shadowed := infos[name]; shadowed {
				// Keep the embedded definition live; record the breakage.
				prev.LoadError = err.Error()
				infos[name] = prev
			} else {
				infos[name] = LayerInfo{LayerDef: LayerDef{Name: name}, Source: SourceProject, LoadError: err.Error()}
			}
			continue
		}
		// Eval-gate honesty for PROJECT packs (D3-1 bounce finding 2): the
		// self-declared `evaled: true` manifest field is a necessity only
		// for EMBEDDED packs, whose eval/ is deliberately not compiled into
		// the binary. A project pack's eval/ IS right here on disk — so a
		// declaration with no readable eval/RESULTS.md behind it is
		// overridden to false (with a once-per-process warning), or a
		// one-line lie in layer.cue would walk straight through the compile
		// eval-gate with a clean, empty escape_note.
		if def.Evaled {
			if _, rerr := fs.ReadFile(projFS, name+"/eval/RESULTS.md"); rerr != nil {
				warnBrokenPackOnce(projectDir+"/"+name+"#evaled", "project layer pack %q at %s declares evaled: true but has no readable eval/RESULTS.md — treating it as NOT evaled (fathom compile will refuse it without --unsafe-skip-eval-gate)", name, projectDir)
				def.Evaled = false
			}
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

// InstallPack validates srcDir (see ValidatePack) then installs it into
// root's project-local pack directory (<root>/.fathom/layers/<name>/, where
// <name> is the pack's OWN declared name). Users never hand-author CUE for
// this: install copies packs; the shipped packs are authored once, under the
// gate. Returns the validated LayerDef and any non-fatal warnings (e.g.
// missing eval/).
//
// Install hygiene (critic finding 5 on the LC slice): the copy is staged
// into a hidden temp sibling directory and atomically renamed into place, so
// a mid-copy failure can never leave a half-written, layer.cue-less pack dir
// that the union loader would then flag as broken on every list/compile.
// An already-installed pack of the same name is REFUSED unless force is
// true; force replaces the whole directory (never merges — stale files from
// a previous version do not survive a reinstall).
func InstallPack(srcDir, root string, force bool) (LayerDef, []string, error) {
	def, warnings, err := ValidatePack(srcDir)
	if err != nil {
		return LayerDef{}, nil, fmt.Errorf("semantic: pack at %s failed validation: %w", srcDir, err)
	}

	layersDir := filepath.Join(root, filepath.FromSlash(ProjectLayersDir))
	destDir := filepath.Join(layersDir, def.Name)
	if _, statErr := os.Stat(destDir); statErr == nil {
		if !force {
			return LayerDef{}, nil, fmt.Errorf("semantic: pack %q is already installed at %s — pass force (CLI: --force) to replace it", def.Name, destDir)
		}
	} else if !os.IsNotExist(statErr) {
		return LayerDef{}, nil, fmt.Errorf("semantic: install pack %q: stat %s: %w", def.Name, destDir, statErr)
	}

	if err := os.MkdirAll(layersDir, 0o755); err != nil {
		return LayerDef{}, nil, fmt.Errorf("semantic: install pack %q: %w", def.Name, err)
	}
	// Stage into a hidden sibling (same filesystem, so the final rename is
	// atomic; hidden, so loadUnion never mistakes it for a pack).
	tmpDir, err := os.MkdirTemp(layersDir, ".install-"+def.Name+"-*")
	if err != nil {
		return LayerDef{}, nil, fmt.Errorf("semantic: install pack %q: stage: %w", def.Name, err)
	}
	if err := copyPackDir(srcDir, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return LayerDef{}, nil, fmt.Errorf("semantic: install pack %q: copy: %w", def.Name, err)
	}
	if force {
		if err := os.RemoveAll(destDir); err != nil {
			os.RemoveAll(tmpDir)
			return LayerDef{}, nil, fmt.Errorf("semantic: install pack %q: replace existing: %w", def.Name, err)
		}
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		os.RemoveAll(tmpDir)
		return LayerDef{}, nil, fmt.Errorf("semantic: install pack %q: finalize: %w", def.Name, err)
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
