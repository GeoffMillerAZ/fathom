// Package deps parses dependency lockfiles and manifests into a canonical
// SBOM-style slice, and provides drift detection between two snapshots.
// All parsers use only stdlib (regexp, encoding/json, strings, bufio, sort).
package deps

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Dependency is a single resolved dependency entry from a lockfile.
type Dependency struct {
	Name      string
	Version   string
	Direct    bool   // true when the dep appears as a direct (non-transitive) entry
	Ecosystem string // "go", "npm", "pypi", "gem", "cargo"
}

// DriftItem reports a version mismatch for a single package between two snapshots.
type DriftItem struct {
	Name     string
	Current  string
	Baseline string
}

// sortDeps sorts a []Dependency slice in-place by (Name, Version) for determinism.
func sortDeps(deps []Dependency) {
	sort.Slice(deps, func(i, j int) bool {
		if deps[i].Name != deps[j].Name {
			return deps[i].Name < deps[j].Name
		}
		return deps[i].Version < deps[j].Version
	})
}

// -----------------------------------------------------------------
// ParseGoSum parses a go.sum file.
// Each line: <module> <version>[/go.mod] <hash>
// We deduplicate by (name, bare-version) keeping the non-/go.mod entry first.
// All entries in go.sum are considered non-direct (go.sum does not distinguish).
// -----------------------------------------------------------------

var goSumLineRe = regexp.MustCompile(`^(\S+)\s+(\S+)\s+\S+$`)

// ParseGoSum parses go.sum content and returns deduplicated Dependency entries.
func ParseGoSum(content []byte) []Dependency {
	seen := make(map[string]bool)
	var out []Dependency

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		m := goSumLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := m[1]
		ver := m[2]
		// strip /go.mod suffix from version token
		ver = strings.TrimSuffix(ver, "/go.mod")
		key := name + "@" + ver
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Dependency{
			Name:      name,
			Version:   ver,
			Direct:    false,
			Ecosystem: "go",
		})
	}
	sortDeps(out)
	return out
}

// -----------------------------------------------------------------
// ParsePackageLock parses an npm package-lock.json (v2/v3).
// We use the top-level "packages" map. The root package "" is skipped.
// A dep is Direct when its "dev" field is absent or false AND its path has
// no nested "node_modules/" in the key beyond the first segment — or more
// simply: Direct = the package key is "node_modules/<name>" (one segment).
// -----------------------------------------------------------------

type packageLockFile struct {
	Packages map[string]packageLockEntry `json:"packages"`
}

type packageLockEntry struct {
	Version string `json:"version"`
	Dev     bool   `json:"dev"`
}

// ParsePackageLock parses an npm package-lock.json (format v2/v3) content.
func ParsePackageLock(content []byte) ([]Dependency, error) {
	var lf packageLockFile
	if err := json.Unmarshal(content, &lf); err != nil {
		return nil, fmt.Errorf("deps: parse package-lock.json: %w", err)
	}
	seen := make(map[string]bool)
	var out []Dependency
	for key, entry := range lf.Packages {
		if key == "" {
			continue // root pseudo-package
		}
		// Extract the logical package name from the key, e.g.:
		// "node_modules/foo"            -> "foo"          (direct)
		// "node_modules/foo/node_modules/bar" -> "foo/node_modules/bar" (transitive)
		// Scoped: "node_modules/@scope/pkg" -> "@scope/pkg"
		name := strings.TrimPrefix(key, "node_modules/")
		// direct = only one "node_modules/" segment in key
		direct := !strings.Contains(name, "node_modules/")
		dedupeKey := name + "@" + entry.Version
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true
		out = append(out, Dependency{
			Name:      name,
			Version:   entry.Version,
			Direct:    direct,
			Ecosystem: "npm",
		})
	}
	sortDeps(out)
	return out, nil
}

// -----------------------------------------------------------------
// ParseRequirementsTxt parses a Python requirements.txt / pip freeze output.
// Supports: pkg==ver, pkg>=ver, pkg~=ver, inline comments, blank lines.
// -----------------------------------------------------------------

var reqLineRe = regexp.MustCompile(`^([A-Za-z0-9_.\-\[\]]+)\s*([=~!<>]+)\s*([^\s;#]+)`)

// ParseRequirementsTxt parses a pip requirements.txt content.
func ParseRequirementsTxt(content []byte) []Dependency {
	var out []Dependency
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		// strip inline comments
		if idx := strings.Index(line, " #"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		m := reqLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out = append(out, Dependency{
			Name:      m[1],
			Version:   m[2] + m[3], // e.g. "==1.2.3"
			Direct:    true,
			Ecosystem: "pypi",
		})
	}
	sortDeps(out)
	return out
}

// -----------------------------------------------------------------
// ParseGemfileLock parses a Ruby Gemfile.lock.
// The GEM/PATH/GIT sections list "    name (version)" lines.
// The DEPENDENCIES section lists direct gems (indented 4 spaces, no deeper).
// -----------------------------------------------------------------

var (
	gemDepLineRe = regexp.MustCompile(`^\s{4}([A-Za-z0-9_.\-]+)\s+\(([^)]+)\)$`)
	// DEPENDENCIES section uses 2-space indent: "  gemname (~> version)" or "  gemname"
	gemDirectRe = regexp.MustCompile(`^\s{2,}([A-Za-z0-9_.\-]+)`)
)

// ParseGemfileLock parses Gemfile.lock content.
func ParseGemfileLock(content []byte) []Dependency {
	lines := strings.Split(string(content), "\n")

	// First pass: collect all resolved (name, version) pairs from spec sections.
	resolved := make(map[string]string)
	inSpecs := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == "  specs:" {
			inSpecs = true
			continue
		}
		// A blank line or a non-indented section header ends specs.
		if inSpecs {
			if trimmed == "" || (!strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "\t")) {
				inSpecs = false
				continue
			}
			m := gemDepLineRe.FindStringSubmatch(line)
			if m != nil {
				resolved[m[1]] = m[2]
			}
		}
	}

	// Second pass: collect direct gem names from DEPENDENCIES section.
	directs := make(map[string]bool)
	inDeps := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == "DEPENDENCIES" {
			inDeps = true
			continue
		}
		if inDeps {
			if trimmed == "" || (!strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "\t")) {
				inDeps = false
				continue
			}
			m := gemDirectRe.FindStringSubmatch(line)
			if m != nil {
				directs[m[1]] = true
			}
		}
	}

	var out []Dependency
	for name, ver := range resolved {
		out = append(out, Dependency{
			Name:      name,
			Version:   ver,
			Direct:    directs[name],
			Ecosystem: "gem",
		})
	}
	sortDeps(out)
	return out
}

// -----------------------------------------------------------------
// ParseCargoLock parses a Rust Cargo.lock (TOML-ish line parse, no lib).
// Format: [[package]] sections with name = "..." and version = "..." keys.
// -----------------------------------------------------------------

var (
	cargoNameRe = regexp.MustCompile(`^name\s*=\s*"([^"]+)"`)
	cargoVerRe  = regexp.MustCompile(`^version\s*=\s*"([^"]+)"`)
)

// ParseCargoLock parses Cargo.lock content using line-by-line TOML parsing.
// All packages are treated as non-direct (Cargo.lock doesn't record directness).
func ParseCargoLock(content []byte) []Dependency {
	var out []Dependency
	var curName, curVer string

	flush := func() {
		if curName != "" && curVer != "" {
			out = append(out, Dependency{
				Name:      curName,
				Version:   curVer,
				Direct:    false,
				Ecosystem: "cargo",
			})
		}
		curName = ""
		curVer = ""
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[[package]]" {
			flush()
			continue
		}
		if m := cargoNameRe.FindStringSubmatch(line); m != nil {
			curName = m[1]
			continue
		}
		if m := cargoVerRe.FindStringSubmatch(line); m != nil {
			curVer = m[1]
			continue
		}
	}
	flush()
	sortDeps(out)
	return out
}

// -----------------------------------------------------------------
// Detect dispatches to the correct parser based on filename.
// -----------------------------------------------------------------

// Detect infers the lockfile type from filename and parses content.
func Detect(filename string, content []byte) ([]Dependency, error) {
	base := filename
	// strip any directory prefix for matching
	if idx := strings.LastIndex(filename, "/"); idx != -1 {
		base = filename[idx+1:]
	}
	switch base {
	case "go.sum":
		return ParseGoSum(content), nil
	case "package-lock.json":
		return ParsePackageLock(content)
	case "requirements.txt":
		return ParseRequirementsTxt(content), nil
	case "Gemfile.lock":
		return ParseGemfileLock(content), nil
	case "Cargo.lock":
		return ParseCargoLock(content), nil
	default:
		return nil, fmt.Errorf("deps: unrecognised lockfile: %q", filename)
	}
}

// -----------------------------------------------------------------
// Drift compares current against baseline returning version mismatches.
// -----------------------------------------------------------------

// Drift returns DriftItems where current and baseline have the same Name but
// different Versions. Missing-only entries (added/removed) are not reported.
func Drift(current, baseline []Dependency) []DriftItem {
	base := make(map[string]string, len(baseline))
	for _, d := range baseline {
		base[d.Name] = d.Version
	}
	var out []DriftItem
	for _, d := range current {
		if bv, ok := base[d.Name]; ok && bv != d.Version {
			out = append(out, DriftItem{
				Name:     d.Name,
				Current:  d.Version,
				Baseline: bv,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
