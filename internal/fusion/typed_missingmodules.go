package fusion

// typed_missingmodules.go — ADR-083 (internal/goenv): the typed pass runs
// the go command offline, so a dependency the module cache lacks is a
// refusal ("module lookup disabled by GOPROXY=off"), never a download. The
// refusal is counted like any failed package and named on the service with
// the module and its remedy (I38: a cold cache never reads as a smaller,
// healthy graph). The pass's partial state is recorded with the missing
// modules in its stamp (missingModulesStamp): the next ingest skips the
// pass like any partial one (RT-104) until a Go input changes or one of
// those modules appears in the module cache (`go mod download`), with no Go
// file changed.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/goenv"
)

// MissingGoModule is one dependency the typed pass could not load because
// the module cache lacks it: Module is "path@version" from the module's
// go.mod requirement (the package path when no requirement covers it), Dir
// the absolute directory of the module that requires it, where `go mod
// download` fills the cache.
type MissingGoModule struct {
	Module string
	Dir    string
}

// typedMissingPackages returns the import paths of root and its
// dependencies whose load error is the go command's offline refusal,
// sorted and deduplicated.
func typedMissingPackages(root *packages.Package) []string {
	seen := map[*packages.Package]bool{}
	found := map[string]bool{}
	stack := []*packages.Package{root}
	for len(stack) > 0 {
		pkg := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if pkg == nil || seen[pkg] {
			continue
		}
		seen[pkg] = true
		for _, err := range pkg.Errors {
			if strings.Contains(err.Msg, goenv.LookupRefused) {
				path := pkg.PkgPath
				if path == "" {
					path = pkg.ID
				}
				found[path] = true
			}
		}
		for _, imported := range pkg.Imports {
			stack = append(stack, imported)
		}
	}
	out := make([]string, 0, len(found))
	for p := range found {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// moduleRequirements reads dir/go.mod's require lines ("path" to
// "path@version"); nil when the file is unreadable or does not parse.
func moduleRequirements(dir string) map[string]string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod")) //nolint:gosec — the corpus's own go.mod
	if err != nil {
		return nil
	}
	f, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		return nil
	}
	out := make(map[string]string, len(f.Require))
	for _, r := range f.Require {
		out[r.Mod.Path] = r.Mod.Path + "@" + r.Mod.Version
	}
	return out
}

// requiredModuleFor names the requirement that provides importPath: the
// longest required module path that is importPath or a prefix of it at a
// slash. importPath itself when no requirement covers it.
func requiredModuleFor(reqs map[string]string, importPath string) string {
	best, bestLen := importPath, 0
	for path, mod := range reqs {
		if (importPath == path || strings.HasPrefix(importPath, path+"/")) && len(path) > bestLen {
			best, bestLen = mod, len(path)
		}
	}
	return best
}

// noteMissing records one module's missing dependencies on the run.
func (r *typedRun) noteMissing(dir string, pkgPaths []string) {
	if len(pkgPaths) == 0 {
		return
	}
	if r.requires == nil {
		r.requires = map[string]map[string]string{}
	}
	reqs, ok := r.requires[dir]
	if !ok {
		reqs = moduleRequirements(dir)
		r.requires[dir] = reqs
	}
	if r.missing == nil {
		r.missing = map[MissingGoModule]bool{}
	}
	for _, p := range pkgPaths {
		r.missing[MissingGoModule{Module: requiredModuleFor(reqs, p), Dir: dir}] = true
	}
}

// missingModules is the run's missing set, sorted by directory then module.
func (r *typedRun) missingModules() []MissingGoModule {
	out := make([]MissingGoModule, 0, len(r.missing))
	for m := range r.missing {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir < out[j].Dir
		}
		return out[i].Module < out[j].Module
	})
	return out
}

// offlineLoadError names a whole-module load the offline go command refused
// (ADR-083) with the module it names and the remedy, and records it on the
// run; any other error is returned as it was.
func (r *typedRun) offlineLoadError(err error, dir string) error {
	mod, ok := goenv.MissingModule(err.Error())
	if !ok {
		return err
	}
	if mod == "" {
		mod = "a required module"
	} else {
		if r.missing == nil {
			r.missing = map[MissingGoModule]bool{}
		}
		r.missing[MissingGoModule{Module: mod, Dir: dir}] = true
	}
	return fmt.Errorf("%w: %s is not in the module cache and fathom runs go offline (GOPROXY=off, ADR-083): %s", err, mod, goenv.Remedy(dir))
}

// formatMissingModules renders the missing set for a Reason or a stderr
// line: each directory's modules, then its remedy. "" when none.
func formatMissingModules(missing []MissingGoModule) string {
	if len(missing) == 0 {
		return ""
	}
	var parts []string
	for i := 0; i < len(missing); {
		j := i
		var mods []string
		for j < len(missing) && missing[j].Dir == missing[i].Dir {
			mods = append(mods, missing[j].Module)
			j++
		}
		parts = append(parts, strings.Join(mods, ", ")+": "+goenv.Remedy(missing[i].Dir))
		i = j
	}
	return fmt.Sprintf(": %d module(s) not in the module cache, and fathom runs go offline (GOPROXY=off, ADR-083): %s", len(missing), strings.Join(parts, "; "))
}

// missingStampPrefix starts the part of a partial pass's state row that
// names the modules the module cache lacked.
const missingStampPrefix = "@missing="

// missingModulesStamp is what a partial typed pass's state row appends for
// the modules the module cache lacked: "" for none, else "@missing=" and,
// sorted and comma-separated, each module with the cache files it had when
// the pass ended ("path@version:" then "m" for its .mod and "z" for its
// .zip under modCache/cache/download).
func missingModulesStamp(modCache string, missing []MissingGoModule) string {
	if len(missing) == 0 {
		return ""
	}
	seen := map[string]bool{}
	var entries []string
	for _, m := range missing {
		if seen[m.Module] {
			continue
		}
		seen[m.Module] = true
		entries = append(entries, m.Module+":"+cachedModuleFiles(modCache, m.Module))
	}
	sort.Strings(entries)
	return missingStampPrefix + strings.Join(entries, ",")
}

// missingModulesUnchanged reports whether every module stamp (a
// missingModulesStamp) names still has exactly the cache files it had: true
// keeps the partial pass standing, false (a module appeared, or modCache is
// unknown) runs the pass again.
func missingModulesUnchanged(modCache, stamp string) bool {
	list, ok := strings.CutPrefix(stamp, missingStampPrefix)
	if !ok || list == "" || modCache == "" {
		return false
	}
	for _, entry := range strings.Split(list, ",") {
		i := strings.LastIndexByte(entry, ':')
		if i < 0 || cachedModuleFiles(modCache, entry[:i]) != entry[i+1:] {
			return false
		}
	}
	return true
}

// cachedModuleFiles names which of mod's (path@version) download files
// modCache holds: "m" for the .mod, "z" for the .zip, "" for neither or
// for a module without a version.
func cachedModuleFiles(modCache, mod string) string {
	path, version, ok := strings.Cut(mod, "@")
	if !ok || modCache == "" {
		return ""
	}
	escPath, err := module.EscapePath(path)
	if err != nil {
		return ""
	}
	escVersion, err := module.EscapeVersion(version)
	if err != nil {
		return ""
	}
	base := filepath.Join(modCache, "cache", "download", filepath.FromSlash(escPath), "@v", escVersion)
	out := ""
	if _, err := os.Stat(base + ".mod"); err == nil {
		out += "m"
	}
	if _, err := os.Stat(base + ".zip"); err == nil {
		out += "z"
	}
	return out
}

// typedModCache is the GOMODCACHE TypedEdgeEnv hands the go command.
func typedModCache() string {
	v := ""
	for _, kv := range repairedGoEnv() {
		if c, ok := strings.CutPrefix(kv, "GOMODCACHE="); ok {
			v = c
		}
	}
	return v
}
