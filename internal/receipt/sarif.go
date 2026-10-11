package receipt

// sarif.go — the run side of the shared SARIF emitter (EVX-2, EVX-3, EVX-5;
// vision 43 sections 6.2 and 6.6): every SARIF caller hands AnchorRun its
// results, then renders through verify.RenderSARIFRun with the provenance it
// returns. A result that cites a line of a file at the run's commit gains
// its anchor's excerpt digest as partialFingerprints["fathomExcerpt/v1"], so
// code scanning keeps one alert when lines move above it. The run reads
// clean only when no anchor read other bytes than the commit holds.

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// Run is what a SARIF producer knows about its run beyond its results.
type Run struct {
	// Dir is a directory inside the repository the results cite; "" or a
	// directory in no work tree gives a run with no commit and no anchors.
	Dir string
	// Base is the directory a relative result path is relative to; "" is
	// the work tree's top level (the producer writes repo-relative paths).
	Base string
	// Rev is the commit the producer read; "" is HEAD.
	Rev string
	// Service is the service whose suppressions withhold a file; "" reads
	// only the fleet-wide ones.
	Service string
	// RepositoryURI is the configured [receipts] repository_uri, passed
	// through to the provenance; "" writes no versionControlProvenance
	// unless RepositoryURIAt names one.
	RepositoryURI string
	// RepositoryURIAt, when set and RepositoryURI is "", reads the URI for
	// the resolved work tree's root (ConfiguredRepositoryURI).
	RepositoryURIAt func(root string) string
	// Stored is the index's stored fingerprint for a repo-relative path,
	// for a producer that answered from the index; nil (or a path it does
	// not hold) means the producer read the work tree's file.
	Stored func(path string) (string, bool)
	// Nodes is the service's definitions, when the producer holds them:
	// a file holding a secret-valued one is withheld (ADR-065).
	Nodes []core.SymbolNode
}

// producer is this binary, as `fathom review` stamps it.
func producer() verify.Producer {
	return verify.Producer{Name: "fathom", Version: buildinfo.Version(), Build: buildinfo.Read().BuiltAt}
}

// AnchorRun resolves run's commit, anchors each result that cites a file
// inside its work tree at it, and returns the run's provenance and a copy of
// results: a path inside the work tree is written repo-relative, and a
// result citing lines gets its anchor's excerpt digest as Fingerprint. A
// result with no line still anchors its file for the run's clean bit (a
// whole-file digest would move with every edit, so it is not a
// fingerprint). A result that cannot be anchored (outside the work tree, a
// directory, lines the file does not hold) keeps no fingerprint and leaves
// clean alone.
func AnchorRun(ctx context.Context, run Run, results []verify.SARIFResult) (verify.RunProvenance, []verify.SARIFResult) {
	prov := verify.RunProvenance{Producer: producer(), ExtractorEpoch: fusion.ExtractorEpoch, RepositoryURI: run.RepositoryURI}
	out := append([]verify.SARIFResult(nil), results...)
	if run.Dir == "" {
		// No directory is no repository, never the process's own cwd.
		return prov, out
	}
	c, err := ResolveCommit(ctx, run.Dir, run.Rev)
	if err != nil {
		return prov, out
	}
	prov.Commit, prov.Clean = c.ID, true
	if prov.RepositoryURI == "" && run.RepositoryURIAt != nil {
		prov.RepositoryURI = run.RepositoryURIAt(c.Root)
	}

	secret := SecretFiles(run.Nodes)
	set, now := scope.NewSource(scope.PathFromEnv(), nil).Snapshot()
	type key struct {
		path       string
		start, end int
	}
	anchors := map[key]*Anchor{}
	for i := range out {
		rel, ok := repoRelative(c.Root, run.Base, out[i].File)
		if !ok {
			continue
		}
		out[i].File = rel
		start, end := out[i].Line, out[i].EndLine
		if start <= 0 {
			start, end = 0, 0
		} else if end < start {
			end = start
		}
		k := key{rel, start, end}
		a, seen := anchors[k]
		if !seen {
			src := Source{Withheld: withheldReason(rel, secret, set, run.Service, now)}
			if run.Stored != nil {
				if fp, ok := run.Stored(rel); ok {
					src.Fingerprint = fp
				}
			}
			if got, aerr := AnchorAt(ctx, c, Span{Path: rel, StartLine: start, EndLine: end}, src); aerr == nil {
				a = &got
			}
			anchors[k] = a
		}
		if a == nil {
			continue
		}
		if !a.Clean {
			prov.Clean = false
		}
		if start > 0 {
			out[i].Fingerprint = a.ExcerptSHA256
		}
	}
	return prov, out
}

// repoRelative is file as a clean slash-separated path relative to root,
// and whether it lies inside root: an absolute path is made relative, a
// relative one is taken as relative to base ("" is root).
func repoRelative(root, base, file string) (string, bool) {
	if file == "" {
		return "", false
	}
	if base != "" && !filepath.IsAbs(file) {
		file = filepath.Join(base, file)
	}
	if filepath.IsAbs(file) {
		rel, err := filepath.Rel(root, file)
		if err != nil || strings.HasPrefix(filepath.ToSlash(rel), "../") {
			// git names the work tree by its real path: resolve a
			// symlinked prefix (macOS /var is /private/var) once.
			if real, rerr := filepath.EvalSymlinks(file); rerr == nil {
				rel, err = filepath.Rel(root, real)
			}
		}
		if err != nil {
			return "", false
		}
		file = rel
	}
	rel := filepath.ToSlash(filepath.Clean(file))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
		return "", false
	}
	return rel, true
}
