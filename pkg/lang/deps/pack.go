package deps

import (
	"context"
	"maps"
	"slices"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/pkg/lang"
)

type dependencyPack struct {
	reader *lang.Reader
	kinds  map[string]Kind
}

// WithPack registers one document dialect from a lang.Reader configured with
// lang.WithDialects. The kinds map translates the dialect's cross-file
// relation names to dependency kinds. Select the dialect in Input.Dialect. The caller owns the
// lang.Reader and keeps it open for dependency reads. A pack is unmeasured.
func WithPack(reader *lang.Reader, dialect string, kinds map[string]Kind) Option {
	kinds = maps.Clone(kinds)
	return func(o *readerOptions) error {
		if reader == nil || !validDialect(dialect) || len(kinds) == 0 ||
			builtinDependencyKinds[dialect] != nil || o.packs[dialect].reader != nil {
			return ErrInput
		}
		if !slices.ContainsFunc(reader.Dialects(), func(info lang.DialectInfo) bool {
			return info.ID == dialect && info.Engine == "doc" && info.Level == lang.LevelUnmeasured
		}) {
			return ErrInput
		}
		// lang.Dialects includes unmeasured built-in document dialects too.
		builtins, err := extractor.LoadDialects()
		if err != nil || slices.ContainsFunc(builtins, func(info extractor.Dialect) bool {
			return info.ID == dialect
		}) {
			return ErrInput
		}
		for relation, kind := range kinds {
			if relation == "" || !validKind(kind) {
				return ErrInput
			}
		}
		o.packs[dialect] = dependencyPack{reader: reader, kinds: kinds}
		return nil
	}
}

func readPackDependencies(ctx context.Context, repo repositoryFS, src []byte, pack dependencyPack, resolver Resolver, out *result) {
	got, err := pack.reader.Extract(ctx, lang.File{Path: out.Path, Src: src})
	switch {
	case err != nil || got.Skipped == lang.SkipBinary:
		out.addUnknown(Unknown{Code: UnknownParseError})
		return
	case got.Skipped == lang.SkipOversize || got.Skipped == lang.SkipBusy:
		out.addUnknown(Unknown{Code: UnknownReadLimit, DecidableBy: []DecidableBy{DecideObservation}})
		return
	case got.Incomplete:
		out.addUnknown(Unknown{Code: UnknownParseError})
	case got.Skipped != "" || got.Dialect != out.Dialect:
		out.addUnknown(Unknown{Code: UnknownUnsupportedConstruct})
		return
	}
	if got.Dialect != out.Dialect {
		return
	}
	for _, ref := range got.References {
		if ctx.Err() != nil {
			return
		}
		kind, classified := pack.kinds[ref.Edge]
		if !classified && ref.Strategy == "" {
			continue
		}
		raw := ref.Raw
		if raw == "" {
			raw = ref.To
		}
		if !classified || ref.Strategy == "" {
			out.addUnknown(Unknown{Code: UnknownUnsupportedConstruct, Raw: raw, Line: ref.Line})
			continue
		}
		if resolver != nil {
			out.resolveExtension(ctx, resolver, ResolveRequest{Repo: repo, From: out.Path, Raw: raw, Kind: kind, Line: ref.Line})
			continue
		}
		out.addPackTarget(repo, ref, raw, kind)
	}
	for _, u := range got.Unresolved {
		code := UnknownUnsupportedConstruct
		if _, classified := pack.kinds[u.Edge]; classified {
			code = UnknownUnresolvedAlias
		}
		out.addUnknown(Unknown{Code: code, Raw: u.Raw, Line: u.Line})
	}
}

func (f *result) addPackTarget(repo repositoryFS, ref lang.Reference, raw string, kind Kind) {
	if ref.Strategy != "repo-path" && ref.Strategy != "repo-root" {
		f.addUnknown(Unknown{Code: UnknownUnsupportedConstruct, Raw: raw, Line: ref.Line})
		return
	}
	info, err := repo.Stat(ref.To)
	if err != nil {
		f.addUnknown(Unknown{Code: UnknownUnresolvedAlias, Raw: raw, Line: ref.Line})
		return
	}
	resolution := ResolvedFile
	if info.IsDir() {
		resolution = ResolvedDirectory
	}
	f.addEdge(Edge{Kind: kind, Raw: raw, To: ref.To, Resolution: resolution, Line: ref.Line, Provenance: ProvenancePack})
}
