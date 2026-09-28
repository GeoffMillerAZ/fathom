package docindex

// depdocs.go — dependency doc units, version-exact (KR-11,
// docs/design/knowledge-retrieval.md §7). For a `pkg:` key the doc units are
// the package's own README and doc comments at the PINNED version, read from
// the directory the toolchain's own cache already holds: Go doc comments,
// `.d.ts` JSDoc, Python docstrings and Rust `///` comments. Nothing is ever
// fetched (I56): this package imports no network or process-spawning
// package (TestDepDocsNeverDial), and a package the cache does not hold is
// refused with the reason, never looked up.
//
// Where the directory comes from is a port, DepCacheLocator: the dependency
// plane's cache locator (docs/plans/system-graph.md SG-36, LocateCached) is
// its production implementation and its indexer (SG-37) the caller of
// IngestDepDocs. This file owns only "what a located directory says": pure
// reading, bounded (design system-graph §8.3: 8 MiB of source per package,
// 2 MiB per file), deterministic (internal/CLAUDE.md: sorted output, no
// clock), public surface only.
//
// The Maven and NuGet readers are not part of KR-11's list (Go, .d.ts,
// docstrings, Rust `///`, README); those ecosystems are refused by name
// (ErrDepEcosystemUnsupported), never answered as an empty package (I38).

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/syslink"
)

// Unit kinds recorded on doc_units.kind for a dependency's docs (beside
// split.go's KindSection and friends).
const (
	// KindDepDoc is one doc comment or docstring, keyed to the API symbol it
	// documents (heading path = the symbol).
	KindDepDoc = "dep-doc"
	// KindDepReadme is a section of the package's own README.
	KindDepReadme = "dep-readme"
)

var (
	// ErrNotCached is depDocUnits' refusal when the toolchain cache does not
	// hold the pinned version. Nothing is fetched (I56): the reason names the
	// one thing the user would run.
	ErrNotCached = errors.New("docindex: package is not in the local toolchain cache")
	// ErrDepEcosystemUnsupported names an ecosystem whose docs this reader
	// does not extract (I38: named, never an empty answer).
	ErrDepEcosystemUnsupported = errors.New("docindex: dependency docs are not read for this ecosystem")
	// ErrBadPkgKey is ParsePkgRef's refusal of a key that is not a
	// `pkg:<ecosystem>/<name>[@<version>]` coordinate.
	ErrBadPkgKey = errors.New("docindex: not a pkg: key")
)

// PkgRef is the coordinates of one package version: a `pkg:` contract key
// (internal/syslink's grammar) without its #symbol part.
type PkgRef struct {
	Ecosystem string // golang, npm, pypi, maven, cargo or nuget
	Name      string
	Version   string // as the lockfile pins it; empty only for an unpinned scope
}

// Key renders the canonical `pkg:` key, "pkg:<ecosystem>/<name>[@<version>]".
// It is also the service id the package's doc units are stored under: a doc
// unit is version-exact because the version is part of its scope.
func (r PkgRef) Key() string {
	k := "pkg:" + r.Ecosystem + "/" + r.Name
	if r.Version != "" {
		k += "@" + r.Version
	}
	return k
}

// ParsePkgRef parses a `pkg:` key (the prefix may be omitted) through
// syslink.ParseContractKey's own grammar, so the ecosystem list and its
// normalization stay one definition. A `#symbol` part is refused: a doc
// scope is a package version, and the symbol belongs in the query. The
// version is optional here so an unpinned scope can be named (never
// guessed) by the caller; a key the grammar rejects reports its reason.
func ParsePkgRef(s string) (PkgRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return PkgRef{}, fmt.Errorf("%w: empty key", ErrBadPkgKey)
	}
	if !strings.HasPrefix(s, "pkg:") {
		s = "pkg:" + s
	}
	ck, err := syslink.ParseContractKey(s)
	if err != nil {
		return PkgRef{}, fmt.Errorf("%w: %v", ErrBadPkgKey, err)
	}
	eco, rest, _ := strings.Cut(ck.Body, "/")
	if strings.Contains(rest, "#") {
		return PkgRef{}, fmt.Errorf("%w: %q carries a #symbol part; scope a docs search to ecosystem/name@version and put the symbol in the query", ErrBadPkgKey, s)
	}
	name, version := rest, ""
	if at := strings.LastIndexByte(rest, '@'); at > 0 {
		name, version = rest[:at], rest[at+1:]
	}
	return PkgRef{Ecosystem: eco, Name: name, Version: version}, nil
}

// DepCacheLocator resolves a pinned package to the directory the
// toolchain's own cache already holds, read-only. It returns ErrNotCached
// (wrapped or bare) when the package is not on disk; it must never fetch.
type DepCacheLocator func(ctx context.Context, ref PkgRef) (dir string, err error)

// depLimits are the read bounds (system-graph design §8.3).
type depLimits struct {
	totalBytes int64 // source bytes read per package
	fileBytes  int64 // one file
	entries    int   // directory entries visited
}

var defaultDepLimits = depLimits{totalBytes: 8 << 20, fileBytes: 2 << 20, entries: 50000}

// SkippedDepFile names one file the read did not use and why.
type SkippedDepFile struct {
	Path   string
	Reason string
}

// DepDocsReport is what a read did, so nothing it left out is silent (I38).
type DepDocsReport struct {
	Files     int   // files that produced at least one unit
	Units     int   // units produced
	BytesRead int64 // bytes of README and source read
	Skipped   []SkippedDepFile
	// Partial names a bound that stopped the read early ("" when it ran to
	// the end): the unit set is then a prefix in read order.
	Partial string
}

// depDocFile is one file's units, keyed by its slash-form path inside the
// package (README and source paths are relative to the package root).
type depDocFile struct {
	Path  string
	Units []store.DocUnit
}

// rawDepUnit is an extractor's output before ids and hashes are attached.
type rawDepUnit struct {
	Heading   string
	Kind      string
	Body      string
	LineStart int
	LineEnd   int
}

// depExtractor is one ecosystem's reader.
type depExtractor struct {
	skipDir func(name string) bool
	wants   func(dir, name string) bool // a source file this reader extracts
	extract func(rel string, src []byte) ([]rawDepUnit, error)
}

func depExtractorFor(ecosystem string) (depExtractor, bool) {
	switch ecosystem {
	case "golang":
		return depExtractor{skipDir: skipGoDepDir, wants: wantsGoFile, extract: extractGoDocs}, true
	case "npm":
		return depExtractor{skipDir: skipNPMDepDir, wants: wantsDTS, extract: extractDTSDocs}, true
	case "pypi":
		return depExtractor{skipDir: skipPyDepDir, wants: wantsPyFile, extract: extractPyDocs}, true
	case "cargo":
		return depExtractor{skipDir: skipRustDepDir, wants: wantsRustFile, extract: extractRustDocs}, true
	}
	return depExtractor{}, false
}

// depDocUnits reads the doc units of one pinned package from the toolchain
// cache loc points at (KR-11): the package README first, then source doc
// comments in path order. It never fetches; a package the cache does not
// hold answers ErrNotCached with the reason.
func depDocUnits(ctx context.Context, loc DepCacheLocator, ref PkgRef) ([]depDocFile, DepDocsReport, error) {
	return depDocUnitsLimited(ctx, loc, ref, defaultDepLimits)
}

func depDocUnitsLimited(ctx context.Context, loc DepCacheLocator, ref PkgRef, lim depLimits) ([]depDocFile, DepDocsReport, error) {
	var rep DepDocsReport
	if ref.Version == "" {
		return nil, rep, fmt.Errorf("depDocUnits: %s has no version; dependency docs are version-exact and read only for a pinned version", ref.Key())
	}
	ex, ok := depExtractorFor(ref.Ecosystem)
	if !ok {
		return nil, rep, fmt.Errorf("%w: %s (%s docs are not read; only golang, npm, pypi and cargo)", ErrDepEcosystemUnsupported, ref.Key(), ref.Ecosystem)
	}
	if loc == nil {
		return nil, rep, fmt.Errorf("depDocUnits: no cache locator for %s: %w", ref.Key(), ErrNotCached)
	}
	dir, err := loc(ctx, ref)
	if err != nil {
		if errors.Is(err, ErrNotCached) {
			return nil, rep, fmt.Errorf("%s is not on disk; run your package manager to populate its cache (fathom never fetches): %w", ref.Key(), err)
		}
		return nil, rep, fmt.Errorf("depDocUnits: locate %s: %w", ref.Key(), err)
	}
	// The cache entry itself may be a symlink (pnpm's store); resolve that
	// one hop-chain, and follow no link inside it.
	root, err := filepath.EvalSymlinks(dir)
	if err == nil {
		var st os.FileInfo
		st, err = os.Stat(root)
		if err == nil && !st.IsDir() {
			err = fmt.Errorf("%s is not a directory", root)
		}
	}
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, rep, fmt.Errorf("%s is not on disk (the cache entry %s is gone); run your package manager to repopulate it: %w", ref.Key(), dir, ErrNotCached)
		}
		return nil, rep, fmt.Errorf("depDocUnits: %s: %w", ref.Key(), err)
	}

	// The README is read first and stays first; source files follow in path
	// order, so a cap never cuts the package's own overview.
	var readmeFiles, srcFiles []depDocFile
	add := func(dst *[]depDocFile, rel string, raws []rawDepUnit) {
		if len(raws) == 0 {
			return
		}
		f := depDocFile{Path: rel}
		for _, u := range uniqueDepHeadings(raws) {
			f.Units = append(f.Units, store.DocUnit{
				UnitID:      unitID(rel, u.Heading, u.Kind),
				Path:        rel,
				LineStart:   u.LineStart,
				LineEnd:     u.LineEnd,
				HeadingPath: u.Heading,
				Kind:        u.Kind,
				ContentHash: sha256Hex([]byte(u.Body)),
				Body:        u.Body,
			})
		}
		*dst = append(*dst, f)
		rep.Files++
		rep.Units += len(f.Units)
	}

	// A read takes `size` of the byte budget if it fits both bounds. It
	// returns false (and records why) when the file must not be read.
	take := func(rel string, size int64) (bool, bool) {
		if size > lim.fileBytes {
			rep.Skipped = append(rep.Skipped, SkippedDepFile{Path: rel, Reason: fmt.Sprintf("%d bytes is over the per-file cap of %d", size, lim.fileBytes)})
			return false, false
		}
		if rep.BytesRead+size > lim.totalBytes {
			rep.Partial = fmt.Sprintf("read stopped at %s: the %d-byte source cap for one package was reached", rel, lim.totalBytes)
			return false, true
		}
		rep.BytesRead += size
		return true, false
	}

	stopped := false
	if rel, ok := findReadme(root); ok {
		info, err := os.Stat(filepath.Join(root, rel))
		if err != nil {
			return nil, rep, fmt.Errorf("depDocUnits: %s: %w", rel, err)
		}
		if okRead, stop := take(rel, info.Size()); okRead {
			raw, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				return nil, rep, fmt.Errorf("depDocUnits: read %s: %w", rel, err)
			}
			add(&readmeFiles, rel, readmeUnits(rel, string(raw)))
		} else if stop {
			stopped = true
		}
	}

	visited := 0
	if !stopped {
		walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			rel, relErr := filepath.Rel(root, p)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if rel == "." {
				return nil
			}
			visited++
			if visited > lim.entries {
				rep.Partial = fmt.Sprintf("walk stopped after %d entries (the per-package entry cap)", lim.entries)
				return fs.SkipAll
			}
			if d.Type()&fs.ModeSymlink != 0 {
				return nil
			}
			if d.IsDir() {
				if ex.skipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !ex.wants(path.Dir(rel), d.Name()) {
				return nil
			}
			// A Python module next to a same-stem stub: the stub is the
			// declared surface, the module is skipped.
			if strings.HasSuffix(rel, ".py") {
				if _, err := os.Stat(p + "i"); err == nil {
					return nil
				}
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			okRead, stop := take(rel, info.Size())
			if stop {
				return fs.SkipAll
			}
			if !okRead {
				return nil
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("read %s: %w", rel, err)
			}
			raws, xerr := ex.extract(rel, raw)
			if xerr != nil {
				rep.Skipped = append(rep.Skipped, SkippedDepFile{Path: rel, Reason: "unparseable: " + xerr.Error()})
				return nil
			}
			add(&srcFiles, rel, raws)
			return nil
		})
		if walkErr != nil {
			return nil, rep, fmt.Errorf("depDocUnits: %s: %w", ref.Key(), walkErr)
		}
	}

	// WalkDir visits a directory's entries lexically; sort anyway so the
	// order never rests on it (internal/CLAUDE.md).
	sort.Slice(srcFiles, func(i, j int) bool { return srcFiles[i].Path < srcFiles[j].Path })
	sort.Slice(rep.Skipped, func(i, j int) bool { return rep.Skipped[i].Path < rep.Skipped[j].Path })
	return append(readmeFiles, srcFiles...), rep, nil
}

// uniqueDepHeadings makes each unit's heading unique within its file, in
// file order ("pad", "pad (2)", "pad (3)"): a doc_units row is keyed by its
// heading, and overloads must not collide.
func uniqueDepHeadings(in []rawDepUnit) []rawDepUnit {
	seen := make(map[string]int, len(in))
	out := make([]rawDepUnit, len(in))
	for i, u := range in {
		seen[u.Heading]++
		if n := seen[u.Heading]; n > 1 {
			u.Heading = fmt.Sprintf("%s (%d)", u.Heading, n)
		}
		out[i] = u
	}
	return out
}

func isReadmeName(name string) bool {
	up := strings.ToUpper(name)
	return up == "README" || strings.HasPrefix(up, "README.")
}

// findReadme returns the package's own README at its root: the
// lexically first README* file (case-insensitive), preferring Markdown.
func findReadme(root string) (string, bool) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false
	}
	best := ""
	for _, e := range entries {
		if e.IsDir() || e.Type()&fs.ModeSymlink != 0 || !isReadmeName(e.Name()) {
			continue
		}
		if best == "" || readmeRank(e.Name()) < readmeRank(best) {
			best = e.Name()
		}
	}
	return best, best != ""
}

func readmeRank(name string) int {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md":
		return 0
	case ".markdown", ".mdx":
		return 1
	}
	return 2
}

// readmeUnits splits a README into units: Markdown by heading section (the
// same splitter the repository's own docs use), anything else as one unit.
func readmeUnits(rel, text string) []rawDepUnit {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".md", ".markdown", ".mdx":
		var out []rawDepUnit
		for _, u := range SplitDocUnits(DocKindOther, text) {
			out = append(out, rawDepUnit{Heading: u.HeadingPath, Kind: KindDepReadme, Body: u.Body, LineStart: u.LineStart, LineEnd: u.LineEnd})
		}
		if len(out) > 0 {
			return out
		}
	}
	return []rawDepUnit{{Heading: "README", Kind: KindDepReadme, Body: strings.TrimSpace(text), LineStart: 1, LineEnd: strings.Count(text, "\n") + 1}}
}
