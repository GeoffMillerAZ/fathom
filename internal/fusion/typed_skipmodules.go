package fusion

// typed_skipmodules.go — EW-68 (docs/plans/efficiency-wave.md; owner ruling
// C1 on wave h5): a repository names, in its own .fathom/config.toml, the
// nested Go modules the typed pass does not load:
//
//	[typed]
//	skip_modules = ["calibration/oracles/**"]
//
// Wave h5's own-module rule (EW-62) made fathom's own calibration oracles
// whose module path is fathom's plus their directory (prometheus,
// alertmanager, grafana, telemetry-census) the repository's own, and on a
// cold build cache their third-party trees took stranger-check's init from
// 255 s to 1218-1413 s. Their path says "own"; only the repository can say
// "measurement only". The globs are repo-relative directory globs over a
// nested module's directory (internal/pathglob, the semantics [zones] and
// the router use); the root module is never skipped. Like `[connector]
// include` the key is a property of the corpus, read from the file at the
// root the pass loads (a stat, a small parse, once per ingest), never from
// the user layer (internal/config refuses it there). An absolute or escaping
// glob is refused by config.RepoPath and skips nothing; a file that does not
// parse skips nothing (`fathom config` is the loud surface for a broken
// file). The pass's state row carries the applied set's stamp
// (skipModulesStamp), so a changed list re-runs the pass at the next ingest
// and its reconcile purges a newly skipped module's typed rows.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/pathglob"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// skipConfigured is why a nested module the repository's skip_modules
// matched is not loaded, as the skipped line names it.
const skipConfigured = "skip_modules"

// skipModulesKnob names where a module is skipped, in the pass's line.
const skipModulesKnob = "[typed] skip_modules in .fathom/config.toml"

// typedConfigName is the corpus-local config file skip_modules is read
// from.
var typedConfigName = projectdir.Config.Path("")

// typedSkipModules returns the `[typed] skip_modules` globs of the corpus
// at repoPath: each cleaned to a slash-separated repo-relative glob, sorted,
// deduplicated; nil when the file is absent, over 1 MiB, not TOML, or sets
// none. An entry config.RepoPath refuses (absolute, a `..` escape, a
// symlink out of the corpus) is dropped.
func typedSkipModules(repoPath string) []string {
	p := filepath.Join(repoPath, typedConfigName)
	fi, err := os.Stat(p)
	if err != nil || fi.Size() > 1<<20 {
		return nil
	}
	data, err := os.ReadFile(p) //nolint:gosec — the corpus's own config file
	if err != nil {
		return nil
	}
	var doc struct {
		Typed struct {
			SkipModules []string `toml:"skip_modules"`
		} `toml:"typed"`
	}
	if toml.Unmarshal(data, &doc) != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, g := range doc.Typed.SkipModules {
		if _, err := config.RepoPath(repoPath, g); err != nil {
			continue
		}
		g = path.Clean(filepath.ToSlash(g))
		if g == "." || seen[g] {
			continue
		}
		seen[g] = true
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// skippedByConfig reports whether the nested module at rel (repo-relative,
// slash-separated) matches one of the skip_modules globs.
func skippedByConfig(rel string, globs []string) bool {
	return pathglob.MatchAny(globs, rel)
}

// skipModulesStamp is what the typed pass's state row appends for the
// skip_modules set it ran under: "" for none (a row written before EW-68
// still reads as completed), else "@skip-" and a short digest of the sorted
// set, so a changed set reads as no completed pass.
func skipModulesStamp(globs []string) string {
	if len(globs) == 0 {
		return ""
	}
	s := append([]string(nil), globs...)
	sort.Strings(s)
	h := sha256.Sum256([]byte(strings.Join(s, "\n")))
	return "@skip-" + hex.EncodeToString(h[:6])
}
