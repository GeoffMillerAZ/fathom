package lang

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langlink"
)

// The reasons a pack or a Go extractor is refused, in PackError.Reason. The
// set is closed.
//
// The values are written as literals, not aliases of the internal constants,
// so a consumer reading the godoc sees them and a change to one is a visible
// edit here. TestPublicConstantValues holds them to the internal ones.
const (
	PackEmpty      = "empty"      // the pack holds no dialect
	PackManifest   = "manifest"   // a manifest is unreadable or breaks the dialect rules
	PackConflict   = "conflict"   // an id, source kind, extension or basename is already taken
	PackEngine     = "engine"     // the engine is not tags or doc
	PackInjections = "injections" // queries.injections is not accepted in a pack
	PackGrammar    = "grammar"    // the grammar is not one the library embeds
	PackQuery      = "query"      // a query does not compile against its grammar
	PackContainer  = "container"  // match.container is not accepted in a pack
	PackClaims     = "claims"     // the dialect claims no file, claims one by a shebang, or names a glob that is not a pattern
	PackRoute      = "route"      // the dialect claims what a built-in dialect owns outright
)

// PackError is why New refused a pack or a Go extractor. Nothing of it is
// loaded: a pack is accepted whole or not at all.
type PackError struct {
	// Reason is one of the Pack constants.
	Reason string
	// Dialect is the dialect that broke the rule, "" when the pack as a whole did.
	Dialect string
	// Detail says which rule and why.
	Detail string
}

func (e *PackError) Error() string {
	who := ""
	if e.Dialect != "" {
		who = " dialect " + e.Dialect + ":"
	}
	return fmt.Sprintf("lang: pack refused (%s):%s %s", e.Reason, who, e.Detail)
}

// publicPackError turns an internal refusal into the public type. Any other
// error passes through with the package prefix.
func publicPackError(err error) error {
	var pe *extractor.PackError
	if errors.As(err, &pe) {
		return &PackError{Reason: pe.Reason, Dialect: pe.Dialect, Detail: pe.Err.Error()}
	}
	return fmt.Errorf("lang: %w", err)
}

// WithDialects adds a pack: dialects read from fsys, laid out as
// dialects/<id>/dialect.json beside the queries and doc specs each names. A
// pack is data. It names a grammar the library embeds and runs no code of its
// own, and only the "tags" and "doc" engines are accepted.
//
// The pack is merged with the built-in dialects and held to the same rules
// they are, then compiled, and New returns a *PackError and no Reader when
// any dialect breaks one. A pack dialect claims files by basename, glob or
// sniff, or by an extension no built-in dialect owns. It is tried after the
// specific built-in dialects and before an extension's generic reader, in id
// order, so it cannot take a file a built-in dialect reads. Its queries run
// under the same budget as the built-ins'. A pack dialect's Level is
// LevelUnmeasured. queries.injections is not accepted in this version.
//
// pkg/lang/langtest.CheckPack runs the same checks, then the pack's goldens.
func WithDialects(fsys fs.FS) Option {
	return func(c *config) error {
		if fsys == nil {
			return errors.New("lang: WithDialects(nil)")
		}
		if c.packFS != nil {
			return errors.New("lang: WithDialects given twice")
		}
		c.packFS = fsys
		return nil
	}
}

// Sniff claims files of the listed extensions whose first kilobyte matches
// Pattern (a Go regular expression, matched across lines).
type Sniff struct {
	Extensions []string
	Pattern    string
}

// Match says which files a Go extractor reads and which kinds it emits. It is
// held to the rules a pack dialect is.
type Match struct {
	// ID names the dialect: lowercase letters, digits and hyphens, and not a
	// dialect the Reader already has.
	ID string
	// Kind is "language" or "tooling" ("language" when empty).
	Kind string
	// Extensions (with their dot), Basenames and Globs claim files, as in a pack.
	// A built-in dialect's extension or basename is refused.
	Extensions []string
	Basenames  []string
	Globs      []string
	// Sniff claims files by content.
	Sniff *Sniff
	// Kinds maps each Symbol.Kind the extractor emits to its Symbol.Canonical,
	// a value of the closed vocabulary. At least one is required.
	Kinds map[string]string
}

// Extracted is what an Extractor reads from one file. Symbol.Key,
// Symbol.Canonical and Reference.ToKey are derived by the Reader and ignored
// here. Reference.Edge is the extractor's own relation name and is passed
// through.
type Extracted struct {
	Symbols    []Symbol
	References []Reference
	Unresolved []Unresolved
	Facts      []Fact
}

// An Extractor reads the files a Match claims. It must be safe for concurrent
// use and deterministic. A panic in it is returned by Reader.Extract as an
// error.
type Extractor interface {
	Extract(ctx context.Context, f File) (Extracted, error)
}

// WithExtractor registers a Go extractor under the same routing rules as a
// pack dialect: it is merged with the built-ins, tried in id order after the
// specific built-ins and before an extension's generic reader, and refused
// whole, as a *PackError, when it breaks a rule. Its dialect appears in
// Dialects with Engine "native" and Level LevelUnmeasured.
func WithExtractor(m Match, e Extractor) Option {
	return func(c *config) error {
		if e == nil {
			return fmt.Errorf("lang: WithExtractor(%q): nil extractor", m.ID)
		}
		c.natives = append(c.natives, nativeSpec{m: m, e: e})
		return nil
	}
}

// Link adds an extractor family a sibling package registers by import, by its
// name. It is the hook pkg/lang/cue's Option calls, and cue is the only family
// there is: write cue.Option(), not Link("cue"). New returns an error when the
// family's package was not imported.
func Link(family string) Option {
	return func(c *config) error {
		if _, known := langlink.FamilyDialects[family]; !known {
			return fmt.Errorf("lang: unknown family %q (known: %s)", family, strings.Join(langlink.Names(), ", "))
		}
		c.families = append(c.families, family)
		return nil
	}
}

type nativeSpec struct {
	m Match
	e Extractor
}

// dialect is the manifest form of m, the shape the dialect rules check.
func (n nativeSpec) dialect() extractor.Dialect {
	d := extractor.Dialect{ID: n.m.ID, Display: n.m.ID, Kind: n.m.Kind, Kinds: map[string]extractor.DialectKind{}}
	d.Match.Extensions = append([]string{}, n.m.Extensions...)
	d.Match.Basenames = append([]string{}, n.m.Basenames...)
	d.Match.Globs = append([]string{}, n.m.Globs...)
	if s := n.m.Sniff; s != nil {
		d.Match.Sniff = &extractor.DialectSniff{Extensions: append([]string{}, s.Extensions...), Pattern: s.Pattern}
	}
	for kind, canonical := range n.m.Kinds {
		d.Kinds[kind] = extractor.DialectKind{SourceKind: kind, Canonical: canonical}
	}
	return d
}

// nativeStub stands in for a Go extractor in the router, which only needs
// to know which dialect claimed the file. The Reader calls the real
// extractor itself, with the caller's context and File.
type nativeStub struct{}

func (nativeStub) Extract(string, string, []byte) ([]core.SymbolNode, error) { return nil, nil }

// extractNative runs a Go extractor and builds the Result from what it read.
func (r *Reader) extractNative(ctx context.Context, id string, f File, res Result) (out Result, err error) {
	defer func() {
		if p := recover(); p != nil {
			out, err = Result{}, fmt.Errorf("lang: extractor %s panicked on %s: %v", id, f.Path, p)
		}
	}()
	got, err := r.natives[id].Extract(ctx, f)
	if err != nil {
		res.Notes = append(res.Notes, "extractor error: "+err.Error())
		res.Incomplete = true
	}
	res.Dialect = id
	res.Level = LevelUnmeasured
	for _, s := range got.Symbols {
		s.Key = symbolKey(s.Package, s.Name)
		s.Canonical, _ = r.pack.CanonicalKind(s.Kind)
		res.Symbols = append(res.Symbols, s)
	}
	res.References = append(res.References, got.References...)
	res.Unresolved = append(res.Unresolved, got.Unresolved...)
	res.Facts = append(res.Facts, got.Facts...)
	res.resolveTargets()
	res.addSiblingNote(f.Path, f.Siblings)
	res.sort()
	return res, nil
}

// packDialectInfos describes the pack's dialects for Dialects.
func packDialectInfos(levels map[string]levelRow, p *extractor.Pack) []DialectInfo {
	var out []DialectInfo
	for _, d := range p.Dialects() {
		set := map[string]bool{}
		for _, e := range d.Match.Extensions {
			set[strings.ToLower(e)] = true
		}
		for _, v := range d.Variants {
			for _, e := range v.Extensions {
				set[strings.ToLower(e)] = true
			}
		}
		for _, b := range d.Match.Basenames {
			set[b] = true
		}
		for _, g := range d.Match.Globs {
			set[g] = true
		}
		if d.Match.Sniff != nil {
			for _, e := range d.Match.Sniff.Extensions {
				set[strings.ToLower(e)] = true
			}
		}
		info := newDialectInfo(levels, d.ID, d.Kind, d.Engine, set)
		info.Level, info.Grade = LevelUnmeasured, "" // a pack ships no reading
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
