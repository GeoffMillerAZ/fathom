package conserve

// coverageload.go — LoadConfiguredCoverage (design §3.5, CV-67/CV-70,
// I44): the one implementation of "find and load the coverage profile
// execution evidence draws from", now shared by every caller that builds
// an Input — `fathom conserve` (cmd/fathom/conserve.go), `fathom review
// --since` (cmd/fathom/reviewconservation.go) and the MCP review_diff
// tool (internal/mcp/review_tool.go). Before this file existed, only
// `fathom conserve` loaded coverage at all: the review conservation
// dimension silently stayed on static reach even with a usable profile
// sitting at [tests] results_dir, answering a DIFFERENT question about
// the identical base..head range (a defect design §3.4's "the review
// `conservation` dimension carries the counts" already promised against
// — CV-65).
import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// LoadConfiguredCoverage resolves and loads coverage.meta.json (and its
// bound profile) into in.Coverage/CoverageMeta/CoverageLoadError:
// metaPathOverride when non-empty (an explicit `--coverage-meta`), else
// `[tests] results_dir/coverage.meta.json` read from root's own
// .fathom/config.toml (project layer) merged with the user layer.
//
// Neither being configured, or the DEFAULT results_dir guess simply not
// existing there, is not itself a reportable failure — most runs have no
// profile at all, and in.CoverageLoadError stays "" (CoverageUnusable's
// own doc comment). An explicit metaPathOverride naming a path that does
// not exist IS reportable (I44: "the report names why" applies to a
// profile the caller asked for by name, never silently treated the same
// as "nothing configured"). A meta that reads but already fails CV-70's
// bound/dirty/passed bar skips the profile parse and repo-wide module
// walk entirely — Residue's own coverageUsableGlobally will name the
// same reason straight from CoverageMeta, so paying for LoadCoverage's
// full cost only to throw the result away unused would be pure waste on
// a large checkout. A profile that loads but maps NO block to any
// repository file (every key in LoadCoverage's own skipped list — a
// stale results_dir from another module, or a renamed module) is
// likewise named rather than silently accepted as an empty-but-usable
// profile. A profile that maps SOME blocks but skips other keys is usable,
// and the skipped keys go to in.CoverageSkipped — the report counts and
// samples them (ResidueReport.CoverageUnmapped) rather than letting a
// partial mapping pass for a complete one.
func LoadConfiguredCoverage(in *Input, root, metaPathOverride string) {
	explicit := metaPathOverride != ""
	metaPath := metaPathOverride
	if metaPath == "" {
		loaded, err := config.Load(config.DefaultUserPath(), filepath.Join(root, ".fathom", "config.toml"))
		if err != nil {
			in.CoverageLoadError = fmt.Sprintf("loading project config: %v", err)
			return
		}
		resultsDir := loaded.Config.Tests.ResultsDir
		if resultsDir == "" {
			return
		}
		// SEC-2: results_dir is a repo-path field — inside the repository,
		// never an absolute path, a .. escape or a symlink out.
		dir, perr := config.RepoPath(root, resultsDir)
		if perr != nil {
			in.CoverageLoadError = "[tests] results_dir " + perr.Error()
			return
		}
		metaPath = filepath.Join(dir, "coverage.meta.json")
	}

	meta, err := LoadCoverageMeta(metaPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if explicit {
				in.CoverageLoadError = fmt.Sprintf("--coverage-meta %s: no such file", metaPath)
			}
			return
		}
		in.CoverageLoadError = err.Error()
		return
	}
	in.CoverageMeta = meta

	if meta.Profile == "" {
		in.CoverageLoadError = fmt.Sprintf("%s names no profile", metaPath)
		return
	}
	if usable, _ := CoverageMetaUsable(meta); !usable {
		// coverageUsableGlobally (residue.go) will name the reason from
		// CoverageMeta directly; a non-nil-but-empty profile keeps that
		// path's existing "in.Coverage == nil means nothing was
		// configured at all" contract intact.
		in.Coverage = &CoverageProfile{Blocks: map[string][]CoverageBlock{}}
		return
	}
	profilePath := meta.Profile
	if !filepath.IsAbs(profilePath) {
		profilePath = filepath.Join(filepath.Dir(metaPath), profilePath)
	}
	prof, skipped, err := LoadCoverage(root, profilePath)
	if err != nil {
		in.CoverageLoadError = err.Error()
		return
	}
	if len(prof.Blocks) == 0 && len(skipped) > 0 {
		in.CoverageLoadError = fmt.Sprintf("%s: every block's file key matched no module under %s (%d skipped, e.g. %s)", profilePath, root, len(skipped), skipped[0])
		return
	}
	in.Coverage = &prof
	in.CoverageSkipped = skipped
}
