package docindex

// pkgsearch.go — `search kind: docs` scoped to a `pkg:` key (KR-12,
// docs/design/knowledge-retrieval.md §7): the answer comes from THAT
// version's docs, read from the FTS index IngestDepDocs filled from the
// local toolchain cache, and from no other version's. The scope is the
// package's own service id (PkgRef.Key), so version-exactness is not a
// filter that could be forgotten: a query cannot see another version's rows.
//
// It is a request-path read: one FTS query with a LIMIT through RankDocs,
// nothing read from the cache directory and nothing fetched. A version the
// index does not hold is answered by name — "not indexed", with the
// versions that are — never from a neighbouring version, never as an empty
// result that looks like a real no-match (I38). An unpinned scope is
// refused the same way: without the lockfile's pin there is no version to be
// exact about, and guessing one would be the failure this feature exists to
// prevent.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// PkgDocsState is how a scoped docs search resolved.
type PkgDocsState string

const (
	// PkgDocsAnswered: the version is indexed; Hits is its ranked answer
	// (possibly none — a real no-match).
	PkgDocsAnswered PkgDocsState = "answered"
	// PkgDocsNotIndexed: no docs are indexed for this exact version.
	PkgDocsNotIndexed PkgDocsState = "not_indexed"
	// PkgDocsUnpinned: the scope names no version.
	PkgDocsUnpinned PkgDocsState = "unpinned"
)

// PkgDocsResult is one scoped docs search's answer.
type PkgDocsResult struct {
	Key   string // the canonical pkg: key asked about
	State PkgDocsState
	// Reason says why the search was not answered (empty when it was).
	Reason string
	// IndexedVersions lists this package's versions that do have indexed
	// docs, sorted — set when the scope was not answered, so the caller can
	// choose one deliberately.
	IndexedVersions []string
	Hits            []RankedDoc
	// Partial names what indexing cut from this version (elided units,
	// skipped files, a read bound), empty when it cut nothing.
	Partial string
}

// searchPkgDocs answers one scoped query against sq (KR-12).
func searchPkgDocs(ctx context.Context, sq *store.SQLiteStore, ref PkgRef, query string, limit int) (PkgDocsResult, error) {
	res := PkgDocsResult{Key: ref.Key()}
	prefix := "pkg:" + ref.Ecosystem + "/" + ref.Name + "@"
	ids, err := sq.DocIndexedServiceIDs(ctx, prefix)
	if err != nil {
		return res, fmt.Errorf("searchPkgDocs: %w", err)
	}
	versions := make([]string, 0, len(ids))
	for _, id := range ids {
		versions = append(versions, strings.TrimPrefix(id, prefix))
	}

	if ref.Version == "" {
		res.State = PkgDocsUnpinned
		res.IndexedVersions = versions
		res.Reason = fmt.Sprintf("%s has no version: dependency docs are version-exact, so pin it as your lockfile does (pkg:%s/%s@<version>). Indexed versions: %s",
			res.Key, ref.Ecosystem, ref.Name, listOrNone(versions))
		return res, nil
	}

	raw, reported, err := sq.Fingerprint(ctx, res.Key, depReportRef)
	if err != nil {
		return res, fmt.Errorf("searchPkgDocs: %w", err)
	}
	hasUnits := false
	for _, v := range versions {
		if v == ref.Version {
			hasUnits = true
		}
	}
	if !reported && !hasUnits {
		res.State = PkgDocsNotIndexed
		res.IndexedVersions = versions
		res.Reason = fmt.Sprintf("%s is not indexed: dependency docs are read from the local toolchain cache by the dependency plane and never fetched (I56), so run your package manager to put this exact version on disk. Indexed versions of this package: %s",
			res.Key, listOrNone(versions))
		return res, nil
	}

	hits, err := RankDocs(ctx, sq, res.Key, query, nil, limit)
	if err != nil {
		return res, fmt.Errorf("searchPkgDocs: %w", err)
	}
	res.State = PkgDocsAnswered
	res.Hits = hits
	if reported {
		var rep depIndexReport
		if json.Unmarshal([]byte(raw), &rep) == nil {
			res.Partial = rep.describeCut()
		}
	}
	return res, nil
}

// SearchPkgDocs is searchPkgDocs' entry point for callers holding a key
// string ("pkg:<ecosystem>/<name>@<version>", the prefix optional) and the
// generic store. It refuses a key that is not a pkg: coordinate
// (ErrBadPkgKey) and a store with no doc index (ErrDocIndexUnavailable), the
// same refusal RankDocs makes.
func SearchPkgDocs(ctx context.Context, st store.Store, key, query string, limit int) (PkgDocsResult, error) {
	ref, err := ParsePkgRef(key)
	if err != nil {
		return PkgDocsResult{}, err
	}
	sq, ok := st.(*store.SQLiteStore)
	if !ok {
		return PkgDocsResult{}, ErrDocIndexUnavailable
	}
	return searchPkgDocs(ctx, sq, ref, query, limit)
}

func listOrNone(vs []string) string {
	if len(vs) == 0 {
		return "none"
	}
	return strings.Join(vs, ", ")
}

// describeCut renders what indexing cut as one sentence fragment, "" when
// nothing was cut.
func (r depIndexReport) describeCut() string {
	var parts []string
	if r.Elided > 0 {
		parts = append(parts, fmt.Sprintf("%d doc units elided at index time (over the package's byte budget)", r.Elided))
	}
	if r.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d files skipped (over the per-file cap or unparseable)", r.Skipped))
	}
	if r.Partial != "" {
		parts = append(parts, r.Partial)
	}
	return strings.Join(parts, "; ")
}
