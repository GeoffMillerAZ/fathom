// Package techpack discovers the tech packs a repository's own pins put on
// disk (docs/design/fathom-dir.md section 7.2: adoption is the binding).
//
// A tech pack is a `fathom-pack/` directory whose `pack.cue` carries a
// `pack: {name, version, about}` header and validates against
// schema/techpack's #Pack (through projectdir.ValidatePack, the one loader).
// Discovery is a closed, offline set: the modules go.mod requires and the
// modules cue.mod/module.cue depends on, each resolved where the toolchain
// would find it without a network. It never reads go.work (a workspace is
// the author's machine, not the pin), never runs the go command, and never
// fetches: a pinned module absent from disk is disclosed, not downloaded
// (I56).
package techpack

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cuemodfile "cuelang.org/go/mod/modfile"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// Source names where a pinned module's directory was found.
type Source string

const (
	// SourceReplace is a go.mod `replace` naming a local directory.
	SourceReplace Source = "replace"
	// SourceVendor is vendor/<path>, listed in vendor/modules.txt.
	SourceVendor Source = "vendor"
	// SourceModCache is the Go module cache at the required version.
	SourceModCache Source = "modcache"
	// SourceCUECache is the CUE module cache at the pinned version.
	SourceCUECache Source = "cuecache"
)

// PackDir is the directory a module ships its tech pack in, and PackFile
// the file inside it that carries the header.
const (
	PackDir  = "fathom-pack"
	PackFile = "pack.cue"
)

// Generated is one `generated` declaration: an output pattern, the sources
// it is produced from, and the command that regenerates it (rendered text;
// fathom never runs it).
type Generated struct {
	Output     string   `json:"output"`
	Source     []string `json:"source"`
	Regenerate string   `json:"regenerate"`
}

// Pack is one discovered tech pack.
type Pack struct {
	// Name, Version and About are the pack header's.
	Name    string
	Version int
	About   string
	// Module is the pinned module that carries the pack (a CUE module
	// without its major-version suffix), Pin the version the repository
	// requires, and Source where the module's directory was found.
	Module string
	Pin    string
	Source Source
	// Dir is the pack's fathom-pack/ directory.
	Dir string
	// Hash is the hex sha256 over every file of the pack, in sorted path
	// order: with Name and Version, the pack's identity.
	Hash string
	// Generated is the pack's `generated` declarations, in file order.
	Generated []Generated
}

// Tag is the pack's served identity, `name@version`.
func (p Pack) Tag() string { return fmt.Sprintf("%s@%d", p.Name, p.Version) }

// Missing is a pinned module that is not on disk: declared, never fetched.
type Missing struct {
	Ecosystem string // "go" or "cue"
	Module    string
	Version   string
}

func (m Missing) String() string { return m.Module + "@" + m.Version }

// Invalid is a fathom-pack/pack.cue that carries a header but is not a valid
// tech pack: named, never served.
type Invalid struct {
	Module string
	Dir    string
	Err    string
}

// Discovery is what Discover found, each list sorted.
type Discovery struct {
	Packs   []Pack
	Missing []Missing
	Invalid []Invalid
}

// pin is one module the repository pins, resolved to the directory it reads.
type pin struct {
	ecosystem string
	module    string
	version   string
	dir       string // "" when no directory names it
	source    Source
}

// Discover resolves root's Go and CUE pins offline and returns the tech
// packs they carry. No go.mod and no cue.mod/module.cue is an empty
// discovery. A pin file that exists but cannot be read or parsed is an
// error, never an empty discovery (I38).
func Discover(root string) (Discovery, error) {
	return discover(root, os.Getenv)
}

func discover(root string, getenv func(string) string) (Discovery, error) {
	goPins, err := goPins(root, getenv)
	if err != nil {
		return Discovery{}, err
	}
	cuePins, err := cuePins(root, getenv)
	if err != nil {
		return Discovery{}, err
	}
	var d Discovery
	seen := map[string]bool{}
	for _, p := range append(goPins, cuePins...) {
		if p.dir == "" {
			d.Missing = append(d.Missing, Missing{Ecosystem: p.ecosystem, Module: p.module, Version: p.version})
			continue
		}
		if st, serr := os.Stat(p.dir); serr != nil || !st.IsDir() {
			d.Missing = append(d.Missing, Missing{Ecosystem: p.ecosystem, Module: p.module, Version: p.version})
			continue
		}
		dir := filepath.Join(p.dir, PackDir)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		pk, found, perr := readPack(dir)
		switch {
		case perr != nil:
			d.Invalid = append(d.Invalid, Invalid{Module: p.module, Dir: dir, Err: perr.Error()})
		case found:
			pk.Module, pk.Pin, pk.Source = p.module, p.version, p.source
			d.Packs = append(d.Packs, pk)
		}
	}
	sort.Slice(d.Packs, func(i, j int) bool {
		if d.Packs[i].Name != d.Packs[j].Name {
			return d.Packs[i].Name < d.Packs[j].Name
		}
		return d.Packs[i].Dir < d.Packs[j].Dir
	})
	sort.Slice(d.Missing, func(i, j int) bool {
		if d.Missing[i].Module != d.Missing[j].Module {
			return d.Missing[i].Module < d.Missing[j].Module
		}
		return d.Missing[i].Version < d.Missing[j].Version
	})
	sort.Slice(d.Invalid, func(i, j int) bool { return d.Invalid[i].Dir < d.Invalid[j].Dir })
	return d, nil
}

// goPins resolves every require of root/go.mod: a replace naming a local
// directory, else vendor/<path> when vendor/modules.txt lists the module,
// else the module cache at the required (or replaced) version.
func goPins(root string, getenv func(string) string) ([]pin, error) {
	gomod := filepath.Join(root, "go.mod")
	data, err := os.ReadFile(gomod)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("techpack: read %s: %w", gomod, err)
	}
	mf, err := modfile.Parse(gomod, data, nil)
	if err != nil {
		return nil, fmt.Errorf("techpack: parse %s: %w", gomod, err)
	}
	vendored, err := vendoredModules(root)
	if err != nil {
		return nil, err
	}
	cache := goModCache(getenv)
	var pins []pin
	for _, r := range mf.Require {
		p := pin{ecosystem: "go", module: r.Mod.Path, version: r.Mod.Version}
		target := r.Mod
		if rep := replacementFor(mf, r.Mod); rep != nil {
			if rep.New.Version == "" {
				dir := rep.New.Path
				if !filepath.IsAbs(dir) {
					dir = filepath.Join(root, filepath.FromSlash(dir))
				}
				p.dir, p.source = dir, SourceReplace
				pins = append(pins, p)
				continue
			}
			target = rep.New
		}
		switch {
		case vendored[r.Mod.Path]:
			p.dir, p.source = filepath.Join(root, "vendor", filepath.FromSlash(r.Mod.Path)), SourceVendor
		case cache != "":
			if dir, ok := cacheDir(cache, target.Path, target.Version); ok {
				p.dir, p.source = dir, SourceModCache
			}
		}
		pins = append(pins, p)
	}
	return pins, nil
}

// replacementFor returns the go.mod replace that applies to m: one naming
// m's exact version wins over one naming the path alone.
func replacementFor(mf *modfile.File, m module.Version) *modfile.Replace {
	var pathOnly *modfile.Replace
	for _, r := range mf.Replace {
		if r.Old.Path != m.Path {
			continue
		}
		if r.Old.Version == m.Version {
			return r
		}
		if r.Old.Version == "" {
			pathOnly = r
		}
	}
	return pathOnly
}

// vendoredModules lists the modules root/vendor/modules.txt records ("#
// <path> <version>" lines). No modules.txt is no vendor directory.
func vendoredModules(root string) (map[string]bool, error) {
	path := filepath.Join(root, "vendor", "modules.txt")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("techpack: read %s: %w", path, err)
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		rest, ok := strings.CutPrefix(line, "# ")
		if !ok {
			continue
		}
		if f := strings.Fields(rest); len(f) >= 2 {
			out[f[0]] = true
		}
	}
	return out, nil
}

// goModCache is the Go module cache directory without running the go
// command: $GOMODCACHE, else the first $GOPATH entry's pkg/mod, else
// ~/go/pkg/mod. "" when none can be named.
func goModCache(getenv func(string) string) string {
	if dir := getenv("GOMODCACHE"); dir != "" {
		return dir
	}
	if gp := getenv("GOPATH"); gp != "" {
		if first := filepath.SplitList(gp); len(first) > 0 && first[0] != "" {
			return filepath.Join(first[0], "pkg", "mod")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "go", "pkg", "mod")
}

// cacheDir is a module cache's directory for path@version, with the
// module cache's case escaping (golang.org/x/mod/module).
func cacheDir(cache, path, version string) (string, bool) {
	ep, err := module.EscapePath(path)
	if err != nil {
		return "", false
	}
	ev, err := module.EscapeVersion(version)
	if err != nil {
		return "", false
	}
	return filepath.Join(cache, filepath.FromSlash(ep)+"@"+ev), true
}

// cuePins resolves every dependency of root/cue.mod/module.cue through the
// CUE module cache ($CUE_CACHE_DIR, else the user cache directory's cue/),
// at mod/extract/<path>@<version>.
func cuePins(root string, getenv func(string) string) ([]pin, error) {
	path := filepath.Join(root, "cue.mod", "module.cue")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("techpack: read %s: %w", path, err)
	}
	if !strings.Contains(string(data), "deps") {
		return nil, nil // no dependency block: nothing to resolve, and no CUE compile on the hot path
	}
	mf, err := cuemodfile.ParseNonStrict(data, path)
	if err != nil {
		return nil, fmt.Errorf("techpack: parse %s: %w", path, err)
	}
	cache := getenv("CUE_CACHE_DIR")
	if cache == "" {
		if ucd, uerr := os.UserCacheDir(); uerr == nil && ucd != "" {
			cache = filepath.Join(ucd, "cue")
		}
	}
	names := make([]string, 0, len(mf.Deps))
	for name := range mf.Deps {
		names = append(names, name)
	}
	sort.Strings(names)
	var pins []pin
	for _, name := range names {
		base, _, _ := strings.Cut(name, "@")
		p := pin{ecosystem: "cue", module: base, version: mf.Deps[name].Version}
		if cache != "" {
			if dir, ok := cacheDir(filepath.Join(cache, "mod", "extract"), base, p.version); ok {
				p.dir, p.source = dir, SourceCUECache
			}
		}
		pins = append(pins, p)
	}
	return pins, nil
}

// readPack reads dir/pack.cue. found is false when the directory or the
// file is absent or the file carries no `pack` header (a fathom-pack/
// without a header is not a tech pack). A header that fails #Pack is an
// error.
func readPack(dir string) (Pack, bool, error) {
	file := filepath.Join(dir, PackFile)
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return Pack{}, false, nil
	}
	if err != nil {
		return Pack{}, false, err
	}
	v := cuecontext.New().CompileBytes(data, cue.Filename(file))
	if v.Err() != nil {
		return Pack{}, false, v.Err()
	}
	if !v.LookupPath(cue.ParsePath("pack")).Exists() {
		return Pack{}, false, nil
	}
	if verr := projectdir.ValidatePack(data, file); verr != nil {
		return Pack{}, false, verr
	}
	var doc struct {
		Pack struct {
			Name    string `json:"name"`
			Version int    `json:"version"`
			About   string `json:"about"`
		} `json:"pack"`
		Generated []Generated `json:"generated"`
	}
	if derr := v.Decode(&doc); derr != nil {
		return Pack{}, false, derr
	}
	hash, err := hashDir(dir)
	if err != nil {
		return Pack{}, false, err
	}
	return Pack{
		Name: doc.Pack.Name, Version: doc.Pack.Version, About: doc.Pack.About,
		Dir: dir, Hash: hash, Generated: doc.Generated,
	}, true, nil
}

// hashDir is the hex sha256 over every regular file under dir in sorted
// slash-path order: each file contributes its relative path, its length and
// its bytes.
func hashDir(dir string) (string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return rerr
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, rel := range files {
		data, rerr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if rerr != nil {
			return "", rerr
		}
		fmt.Fprintf(h, "%s\x00%d\x00", rel, len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
