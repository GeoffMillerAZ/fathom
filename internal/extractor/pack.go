package extractor

// pack.go -- dialect packs for the language library (ADR-092, design §6). A
// pack is a directory of dialect manifests, doc specs and queries laid out as
// internal/extractor/dialects is, read from any fs.FS. It is data: it names a
// grammar fathom already embeds and runs no code of its own. A pack is merged
// with the built-in dialects, held to checkDialect and the cross-dialect rules
// against them, and refused whole with a reason when any dialect breaks one.
// The product loads no pack.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// The ids the language library gives the generic readers (JSONExtractor,
// TOMLExtractor): readers of an extension, not dialects with a manifest.
const (
	GenericJSONDialect = "generic-json"
	GenericTOMLDialect = "generic-toml"
)

// The reasons a pack is refused. The set is closed: a caller may switch on it.
const (
	// PackEmpty: the pack holds no dialect.
	PackEmpty = "empty"
	// PackManifest: a manifest is unreadable or breaks checkDialect.
	PackManifest = "manifest"
	// PackConflict: an id, a source kind, an extension or a basename is already
	// a built-in's, or another pack dialect's.
	PackConflict = "conflict"
	// PackEngine: the engine is not tags or doc.
	PackEngine = "engine"
	// PackInjections: queries.injections, refused in this version.
	PackInjections = "injections"
	// PackGrammar: the grammar is not one fathom embeds, or its entry point is
	// not in it.
	PackGrammar = "grammar"
	// PackQuery: a query does not compile against the grammar.
	PackQuery = "query"
	// PackContainer: match.container, refused in a pack.
	PackContainer = "container"
	// PackClaims: the dialect claims no file, or claims one by a shebang.
	PackClaims = "claims"
	// PackRoute: the dialect claims an extension or basename the router owns
	// outright (set by the router, which knows its own table).
	PackRoute = "route"
)

// PackError is why a pack was refused: a reason from the closed set above, the
// dialect that broke it ("" when the pack as a whole did) and the detail.
type PackError struct {
	Reason  string
	Dialect string
	Err     error
}

func (e *PackError) Error() string {
	who := ""
	if e.Dialect != "" {
		who = " dialect " + e.Dialect + ":"
	}
	return fmt.Sprintf("pack refused (%s):%s %v", e.Reason, who, e.Err)
}

func (e *PackError) Unwrap() error { return e.Err }

func packErr(reason, dialect string, format string, args ...any) *PackError {
	return &PackError{Reason: reason, Dialect: dialect, Err: fmt.Errorf(format, args...)}
}

// Pack is a validated set of extra dialects: those read from a pack
// filesystem and those a Go extractor declares (AddNative).
type Pack struct {
	fsys      fs.FS
	claims    *dialectClaims
	dialects  []Dialect
	native    map[string]bool
	canonical map[string]string
}

// NewPack reads the pack at fsys (its dialects/<id>/dialect.json files),
// merges it with the built-in dialects and validates the result. A nil fsys
// gives an empty pack that AddNative can fill. The error is a *PackError.
func NewPack(fsys fs.FS) (*Pack, error) {
	builtins, err := LoadDialects()
	if err != nil {
		return nil, &PackError{Reason: PackManifest, Err: err}
	}
	p := &Pack{fsys: fsys, claims: newDialectClaims(), native: map[string]bool{}, canonical: map[string]string{}}
	for _, d := range builtins {
		if err := p.claims.admit("built-in "+d.ID, d); err != nil {
			return nil, &PackError{Reason: PackManifest, Err: err}
		}
	}
	// The native Go extractor and the generic JSON and TOML readers are dialects
	// no manifest declares; a pack may not take their ids.
	for _, id := range []string{"go", GenericJSONDialect, GenericTOMLDialect} {
		p.claims.ids[id] = true
	}
	if fsys == nil {
		return p, nil
	}
	byDir := map[string]Dialect{}
	manifests, err := fs.Glob(fsys, "dialects/*/dialect.json")
	if err != nil {
		return nil, &PackError{Reason: PackManifest, Err: err}
	}
	if len(manifests) == 0 {
		return nil, packErr(PackEmpty, "", "no dialects/<id>/dialect.json in the pack")
	}
	sort.Strings(manifests)
	for _, mp := range manifests {
		dir := path.Base(path.Dir(mp))
		b, err := fs.ReadFile(fsys, mp)
		if err != nil {
			return nil, &PackError{Reason: PackManifest, Dialect: dir, Err: err}
		}
		var d Dialect
		if err := json.Unmarshal(b, &d); err != nil {
			return nil, &PackError{Reason: PackManifest, Dialect: dir, Err: fmt.Errorf("%s: %w", mp, err)}
		}
		if err := p.checkPackDialect(fsys, dir, d); err != nil {
			return nil, err
		}
		if err := p.claims.admit(mp, d); err != nil {
			return nil, &PackError{Reason: PackConflict, Dialect: d.ID, Err: err}
		}
		p.add(d)
		if d.Engine == "doc" {
			if _, err := loadDocExtractor(fsys, d); err != nil {
				return nil, &PackError{Reason: PackManifest, Dialect: d.ID, Err: err}
			}
		}
		byDir[dir] = d
	}
	if err := auditPackFiles(fsys, byDir); err != nil {
		return nil, err
	}
	return p, nil
}

// checkGlobs refuses a glob that is not a valid pattern. The router matches a
// glob segment by segment with path.Match, which treats a bad pattern as a
// match of nothing, so a dialect with one would silently claim no file.
func checkGlobs(id string, globs []string) error {
	for _, g := range globs {
		for _, seg := range strings.Split(g, "/") {
			if seg == "**" {
				continue
			}
			if _, err := path.Match(seg, ""); err != nil {
				return packErr(PackClaims, id, "glob %q is not a valid pattern: %v", g, err)
			}
		}
	}
	return nil
}

// queryDefinition matches a @definition.<kind> capture in a tags query.
var queryDefinition = regexp.MustCompile(`@definition\.([A-Za-z0-9_.\-]*[A-Za-z0-9_])`)

// stripQueryComments drops the comments of a tree-sitter query, leaving string
// literals alone.
func stripQueryComments(src string) string {
	var b strings.Builder
	inStr := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inStr:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				i++
				b.WriteByte(src[i])
			} else if c == '"' {
				inStr = false
			}
		case c == '"':
			inStr = true
			b.WriteByte(c)
		case c == ';':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			b.WriteByte('\n')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// checkPackTags holds a tags dialect's queries to the rules a compile cannot:
// a query with no pattern reads nothing, a @definition.<kind> capture the
// manifest does not declare reads nothing, and a variant that names a tags
// query of its own needs an id, because the engine keys its parser and compiled
// query by dialect, grammar, entry point and variant id, so two variants of one
// entry point without ids would share the first one compiled.
func checkPackTags(fsys fs.FS, dir, id string, d Dialect) error {
	check := func(file string) error {
		b, err := fs.ReadFile(fsys, path.Join("dialects", dir, file))
		if err != nil {
			return &PackError{Reason: PackQuery, Dialect: id, Err: err}
		}
		q := stripQueryComments(string(b))
		if strings.TrimSpace(q) == "" {
			return packErr(PackQuery, id, "tags query %s holds no pattern", file)
		}
		for _, m := range queryDefinition.FindAllStringSubmatch(q, -1) {
			if _, ok := d.Kinds[m[1]]; !ok {
				return packErr(PackQuery, id, "tags query %s captures @definition.%s, which the manifest declares no kind for (kinds: %s)", file, m[1], strings.Join(sortedKindKeys(d), ", "))
			}
		}
		return nil
	}
	if d.Queries.Tags != "" {
		if err := check(d.Queries.Tags); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, v := range d.Variants {
		if v.Queries == nil || v.Queries.Tags == "" {
			continue
		}
		if v.ID == "" || seen[v.ID] {
			return packErr(PackManifest, id, "a variant with its own tags query needs a unique id (variant %q)", v.ID)
		}
		seen[v.ID] = true
		if err := check(v.Queries.Tags); err != nil {
			return err
		}
	}
	return nil
}

// auditPackFiles refuses what a pack's dialects directory holds that no
// manifest names: a directory without a dialect.json (a manifest misspelled
// dialect.jsn leaves its dialect out silently), a file beside the dialects, and
// a file in a dialect's directory that its manifest does not name. A testdata
// directory (the goldens langtest.CheckPack reads) is the one exception.
func auditPackFiles(fsys fs.FS, byDir map[string]Dialect) error {
	entries, err := fs.ReadDir(fsys, "dialects")
	if err != nil {
		return &PackError{Reason: PackManifest, Err: err}
	}
	for _, e := range entries {
		d, ok := byDir[e.Name()]
		switch {
		case !e.IsDir():
			return packErr(PackManifest, "", "dialects/%s is a file; a dialect is a directory holding dialect.json", e.Name())
		case !ok:
			return packErr(PackManifest, e.Name(), "dialects/%s holds no dialect.json", e.Name())
		}
		named := map[string]bool{"dialect.json": true}
		for _, f := range []string{d.Queries.Tags, d.Queries.Imports, d.Queries.Injections, d.Queries.References, d.Queries.Locals} {
			named[f] = true
		}
		if d.Engine == "doc" {
			if d.Doc != "" {
				named[d.Doc] = true
			} else {
				named["doc.json"] = true
			}
		}
		for _, v := range d.Variants {
			if v.Queries != nil {
				named[v.Queries.Tags] = true
			}
		}
		files, err := fs.ReadDir(fsys, path.Join("dialects", e.Name()))
		if err != nil {
			return &PackError{Reason: PackManifest, Dialect: e.Name(), Err: err}
		}
		for _, f := range files {
			if named[f.Name()] || (f.IsDir() && f.Name() == "testdata") {
				continue
			}
			return packErr(PackManifest, d.ID, "dialects/%s/%s is not a file the manifest names (a misspelled dialect.json or query name?)", e.Name(), f.Name())
		}
	}
	return nil
}

// checkPackDialect holds one pack manifest to the pack-only rules, then to
// checkDialect. The pack-only rules come first, so a refusal names the
// narrowest reason.
func (p *Pack) checkPackDialect(fsys fs.FS, dir string, d Dialect) error {
	id := d.ID
	if id == "" {
		id = dir
	}
	if d.Kind != "language" && d.Kind != "tooling" {
		return packErr(PackManifest, id, "kind %q is not language or tooling", d.Kind)
	}
	if d.Engine != "tags" && d.Engine != "doc" {
		return packErr(PackEngine, id, "engine %q: a pack reads the tags and doc engines only", d.Engine)
	}
	if d.Queries.Injections != "" {
		return packErr(PackInjections, id, "queries.injections is not accepted in a pack")
	}
	if d.InjectionHost {
		return packErr(PackInjections, id, "injection_host is not accepted in a pack")
	}
	if d.Match.Container != nil {
		return packErr(PackContainer, id, "match.container is not accepted in a pack")
	}
	if len(d.Match.Shebangs) > 0 {
		return packErr(PackClaims, id, "match.shebangs is not accepted in a pack")
	}
	if !dialectClaimsAFile(d) {
		return packErr(PackClaims, id, "claims no extension, basename, glob or sniff")
	}
	if err := checkGlobs(id, d.Match.Globs); err != nil {
		return err
	}
	if err := checkDialect(fsys, dir, d); err != nil {
		return &PackError{Reason: PackManifest, Dialect: id, Err: err}
	}
	if d.Engine == "tags" {
		if err := checkPackTags(fsys, dir, id, d); err != nil {
			return err
		}
	}
	if d.Engine == "tags" && !embeddedGrammar(d.Grammar.Blob) {
		return packErr(PackGrammar, id, "grammar %q is not one fathom embeds", d.Grammar.Blob)
	}
	return nil
}

func dialectClaimsAFile(d Dialect) bool {
	if len(d.Match.Extensions) > 0 || len(d.Match.Basenames) > 0 || len(d.Match.Globs) > 0 || d.Match.Sniff != nil {
		return true
	}
	for _, v := range d.Variants {
		if len(v.Extensions) > 0 {
			return true
		}
	}
	return false
}

func embeddedGrammar(blob string) bool {
	for _, g := range tsengine.Grammars() {
		if g == blob {
			return true
		}
	}
	return false
}

func (p *Pack) add(d Dialect) {
	p.dialects = append(p.dialects, d)
	sort.Slice(p.dialects, func(i, j int) bool { return p.dialects[i].ID < p.dialects[j].ID })
	for _, key := range sortedKindKeys(d) {
		k := d.Kinds[key]
		if _, dup := p.canonical[k.SourceKind]; !dup {
			p.canonical[k.SourceKind] = k.Canonical
		}
	}
}

// AddNative declares a dialect read by a Go extractor the caller supplies. It
// is held to the rules a pack manifest is: the id, the kinds, the claims and
// the cross-dialect rules against the built-ins and the rest of the pack. d
// needs ID, Kind, Match and Kinds; the engine is "native".
func (p *Pack) AddNative(d Dialect) error {
	d.Engine = "native"
	d.QueueID = d.ID
	if d.Kind == "" {
		d.Kind = "language"
	}
	if d.Kind != "language" && d.Kind != "tooling" {
		return packErr(PackManifest, d.ID, "kind %q is not language or tooling", d.Kind)
	}
	if !dialectClaimsAFile(d) {
		return packErr(PackClaims, d.ID, "claims no extension, basename, glob or sniff")
	}
	if len(d.Match.Shebangs) > 0 || d.Match.Container != nil {
		return packErr(PackClaims, d.ID, "match.shebangs and match.container are not accepted")
	}
	if err := checkGlobs(d.ID, d.Match.Globs); err != nil {
		return err
	}
	if err := checkDialect(emptyFS{}, d.ID, d); err != nil {
		return &PackError{Reason: PackManifest, Dialect: d.ID, Err: err}
	}
	if err := p.claims.admit("extractor "+d.ID, d); err != nil {
		return &PackError{Reason: PackConflict, Dialect: d.ID, Err: err}
	}
	p.native[d.ID] = true
	p.add(d)
	return nil
}

// emptyFS is a filesystem with no file, for checking a dialect that names none.
type emptyFS struct{}

func (emptyFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// Dialects is the pack's dialects, sorted by id.
func (p *Pack) Dialects() []Dialect { return append([]Dialect(nil), p.dialects...) }

// CanonicalKind is the package function over the built-in dialects, then the
// pack's: ok is false for a source kind neither declares.
func (p *Pack) CanonicalKind(sk string) (string, bool) {
	if c, ok := CanonicalKind(sk); ok {
		return c, true
	}
	c, ok := p.canonical[sk]
	return c, ok
}

// Build constructs the extractor of every pack dialect read from the pack's
// filesystem, and compiles their queries against their grammars, so a query
// that does not compile refuses the pack here and not at the first file. On an
// error it releases what it built. The error is a *PackError.
func (p *Pack) Build(ctx context.Context) (map[string]Extractor, error) {
	out := map[string]Extractor{}
	closeAll := func() {
		for _, e := range out {
			if ts, ok := e.(*TSExtractor); ok {
				ts.Close()
			}
		}
	}
	for _, d := range p.dialects {
		if p.native[d.ID] {
			continue
		}
		switch d.Engine {
		case "doc":
			x, err := loadDocExtractor(p.fsys, d)
			if err != nil {
				closeAll()
				return nil, &PackError{Reason: PackManifest, Dialect: d.ID, Err: err}
			}
			out[d.ID] = x
		case "tags":
			spec, err := tagsSpec(p.fsys, d)
			if err != nil {
				closeAll()
				return nil, &PackError{Reason: PackQuery, Dialect: d.ID, Err: err}
			}
			e := newTSExtractorFromSpec(spec)
			out[d.ID] = e
			if err := e.precompile(ctx); err != nil {
				closeAll()
				var pe *PackError
				if errors.As(err, &pe) {
					pe.Dialect = d.ID
					return nil, pe
				}
				return nil, &PackError{Reason: PackQuery, Dialect: d.ID, Err: err}
			}
		}
	}
	return out, nil
}

// precompile leases each variant's parser and compiles the dialect's queries
// on it, then returns the parser to the pool. A grammar that does not load is
// PackGrammar, a query that does not compile PackQuery.
func (e *TSExtractor) precompile(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	seen := map[string]bool{}
	var variants []tsVariant
	add := func(v tsVariant) {
		if k := v.key(); !seen[k] {
			seen[k] = true
			variants = append(variants, v)
		}
	}
	add(e.spec.defaultVariant)
	exts := make([]string, 0, len(e.spec.variantForExt))
	for x := range e.spec.variantForExt {
		exts = append(exts, x)
	}
	sort.Strings(exts)
	for _, x := range exts {
		add(e.spec.variantForExt[x])
	}
	for _, sv := range e.spec.sniffVariants {
		add(sv.variant)
	}
	for _, v := range variants {
		l, err := tsengine.AcquireParser(ctx, e.owner+v.key(), v.blob, v.entryPoint)
		if err != nil {
			return &PackError{Reason: PackGrammar, Err: err}
		}
		p := l.Parser()
		if _, ok := p.Cached("tags").(*tsengine.Query); !ok {
			q, err := p.CompileQuery(v.rules.query)
			if err != nil {
				l.Discard()
				return &PackError{Reason: PackQuery, Err: fmt.Errorf("compiling %s: %w", v.rules.queryFile, err)}
			}
			p.SetCached("tags", q)
		}
		if v.rules.refsQuery != "" {
			if _, err := p.CompileQuery(v.rules.refsQuery); err != nil {
				l.Discard()
				return &PackError{Reason: PackQuery, Err: fmt.Errorf("compiling the references query: %w", err)}
			}
		}
		l.Release()
	}
	return nil
}

// ValidNativeID reports whether id is a spelling a dialect id may take.
func ValidNativeID(id string) bool { return dialectIDRe.MatchString(id) }
