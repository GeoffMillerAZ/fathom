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
	"sync/atomic"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/fsfault"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/techpack"
)

// LayerSource identifies where a resolved layer definition came from.
type LayerSource string

const (
	SourceEmbedded LayerSource = "embedded"
	// SourceTech is a tech pack's layers/ (FD-18): a fathom-pack/ the
	// repository's own pins put on disk (techpack.Discover).
	SourceTech    LayerSource = "tech"
	SourceProject LayerSource = "project"
)

// LayerInfo is one resolved layer with union provenance — the data source
// for `fathom layers list`/`show` (LC-7).
type LayerInfo struct {
	LayerDef
	Source LayerSource
	// Overrides is true when an earlier stratum also defines this name: the
	// union's precedence is embedded, then tech pack, then project, a later
	// stratum winning by layer id (presence = enabled — no other precedence
	// rule, no enabled/disabled bit to set).
	Overrides bool
	// TechPack is the `name@version` of the tech pack a SourceTech layer
	// came from, "" otherwise.
	TechPack string
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
const ProjectLayersDir = projectdir.Dir + "/" + string(projectdir.Layers)

// LoadLayerIn resolves the named layer from the union of embedded packs, the
// layers/ of the tech packs root's pins put on disk (FD-18,
// techpack.Discover), and root's project-local .fathom/layers/ (LC-3), in
// that precedence order. root == "" (or a root with no
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
//
// RT-83 (docs/design/responsible-tenant.md §4.3): the compiled union is
// memoized per (root, digest of root/.fathom/layers). The embedded packs
// are fixed for the life of the binary, so the digest need only cover the
// project directory: every entry's relative path, size, mtime and mode
// (unionDigest — a stat walk of a handful of small files, never a read).
// Editing, adding or removing a project pack moves the digest and costs
// exactly one rebuild; concurrent callers at the same key share one build.
// Errors are never memoized. The returned map is shared between callers
// and must be treated as read-only (every caller in this package copies
// out of it).
func loadUnion(root string) (map[string]LayerInfo, error) {
	packs := discoverTechPacks(root)
	digest := unionDigest(root) + techPackDigest(packs)

	unionMemo.mu.Lock()
	if e, ok := unionMemo.entries[root]; ok && e.digest == digest {
		unionMemo.mu.Unlock()
		return e.infos, nil
	}
	flightKey := root + "\x00" + digest
	if call, ok := unionMemo.flight[flightKey]; ok {
		unionMemo.mu.Unlock()
		<-call.done
		return call.infos, call.err
	}
	call := &unionCall{done: make(chan struct{})}
	unionMemo.flight[flightKey] = call
	unionMemo.mu.Unlock()

	call.infos, call.err = buildUnion(root, packs)

	unionMemo.mu.Lock()
	delete(unionMemo.flight, flightKey)
	if call.err == nil {
		if len(unionMemo.entries) >= unionMemoMaxRoots {
			// Bounded: one entry per root, and far more roots than a
			// fleet registers means something is minting them — start
			// over rather than grow.
			unionMemo.entries = map[string]unionEntry{}
		}
		unionMemo.entries[root] = unionEntry{digest: digest, infos: call.infos}
	}
	unionMemo.mu.Unlock()
	close(call.done)
	return call.infos, call.err
}

// unionMemoMaxRoots bounds the union memo (one small map per root).
const unionMemoMaxRoots = 256

type unionEntry struct {
	digest string
	infos  map[string]LayerInfo
}

type unionCall struct {
	done  chan struct{}
	infos map[string]LayerInfo
	err   error
}

var unionMemo = struct {
	mu      sync.Mutex
	entries map[string]unionEntry
	flight  map[string]*unionCall
}{entries: map[string]unionEntry{}, flight: map[string]*unionCall{}}

// unionDigest is the memo key's project half: a stat walk of
// root/.fathom/layers, through symlinks (relative path, size, mtime in ns,
// mode per entry's target,
// in WalkDir's lexical order). "" root and an absent directory digest to
// fixed markers; an unreadable directory digests to an error marker, which
// buildUnion then resolves to embedded-only exactly as before.
func unionDigest(root string) string {
	if root == "" {
		return "embedded-only"
	}
	dir := filepath.Join(root, filepath.FromSlash(ProjectLayersDir))
	var b strings.Builder
	// buildUnion reads through symlinks (os.ReadDir, os.DirFS), so the
	// digest must stat through them too: walk the resolved directory (a
	// kit's layers symlinked in) and os.Stat each entry (a symlinked
	// layer.cue). The resolved path is part of the key, so retargeting the
	// link moves it. WalkDir still never descends a symlinked subdirectory
	// — buildUnion does not treat one as a pack either.
	if resolved, rerr := filepath.EvalSymlinks(dir); rerr == nil && resolved != dir {
		fmt.Fprintf(&b, "@%s\n", resolved)
		dir = resolved
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(&b, "!%s:%v\n", path, err)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, ierr := os.Stat(path)
		if ierr != nil {
			fmt.Fprintf(&b, "!%s\n", path)
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		fmt.Fprintf(&b, "%s\x00%d\x00%d\x00%o\n", rel, info.Size(), info.ModTime().UnixNano(), info.Mode())
		return nil
	})
	if err != nil {
		fmt.Fprintf(&b, "!walk:%v", err)
	}
	return b.String()
}

// discoverTechPacks is the tech packs root's own pins put on disk. A
// discovery error is warned about once and resolves to none: the embedded
// and project strata keep serving (the resilience contract above).
func discoverTechPacks(root string) []techpack.Pack {
	if root == "" {
		return nil
	}
	d, err := techpack.Discover(root)
	if err != nil {
		warnBrokenPackOnce(root+"#techpacks", "tech packs under %s could not be discovered and are being ignored (other layers still serve): %v", root, err)
		return nil
	}
	return d.Packs
}

// techPackDigest is the memo key's tech pack half: each pack's directory
// and content hash, so editing, adding or removing a pack file moves it.
func techPackDigest(packs []techpack.Pack) string {
	var b strings.Builder
	for _, p := range packs {
		fmt.Fprintf(&b, "#tech\x00%s\x00%s\n", p.Dir, p.Hash)
	}
	return b.String()
}

// unionBuilds counts real union compiles (buildUnion runs) — the RT-83
// test's observable.
var unionBuilds atomic.Int64

func unionBuildCount() int64 { return unionBuilds.Load() }

func resetUnionMemoForTest() {
	unionMemo.mu.Lock()
	unionMemo.entries = map[string]unionEntry{}
	unionMemo.mu.Unlock()
}

// buildUnion is the uncached union compile loadUnion memoizes.
func buildUnion(root string, packs []techpack.Pack) (map[string]LayerInfo, error) {
	unionBuilds.Add(1)
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

	for _, p := range packs {
		addTechPackLayers(cctx, layerSchema, infos, p)
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

// addTechPackLayers decodes one tech pack's layers/<name>/layer.cue entries
// into infos, each overriding an earlier stratum's layer of the same name.
// A pack without layers/ adds nothing. A broken layer is skipped and
// recorded exactly as a broken project pack is (LoadError, warned once),
// and a declared `evaled: true` with no readable eval/RESULTS.md beside it
// is overridden to false: a tech pack's eval/ is on disk, as a project's.
func addTechPackLayers(cctx *cue.Context, layerSchema cue.Value, infos map[string]LayerInfo, p techpack.Pack) {
	dir := filepath.Join(p.Dir, "layers")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			warnBrokenPackOnce(dir, "tech pack %s layer directory %s is unreadable and is being ignored (other layers still serve): %v", p.Tag(), dir, err)
		}
		return
	}
	packFS := os.DirFS(dir)
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		def, err := decodePack(cctx, layerSchema, packFS, name+"/layer.cue", name)
		if err != nil {
			warnBrokenPackOnce(dir+"/"+name, "tech pack %s layer %q at %s failed to load and is being ignored (other packs still serve): %v", p.Tag(), name, dir, err)
			if prev, shadowed := infos[name]; shadowed {
				prev.LoadError = err.Error()
				infos[name] = prev
			} else {
				infos[name] = LayerInfo{LayerDef: LayerDef{Name: name}, Source: SourceTech, TechPack: p.Tag(), LoadError: err.Error()}
			}
			continue
		}
		if def.Evaled {
			if _, rerr := fs.ReadFile(packFS, name+"/eval/RESULTS.md"); rerr != nil {
				warnBrokenPackOnce(dir+"/"+name+"#evaled", "tech pack %s layer %q declares evaled: true but has no readable eval/RESULTS.md — treating it as NOT evaled", p.Tag(), name)
				def.Evaled = false
			}
		}
		_, overrides := infos[name]
		infos[name] = LayerInfo{LayerDef: def, Source: SourceTech, Overrides: overrides, TechPack: p.Tag()}
	}
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
		data, rerr := fsfault.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
