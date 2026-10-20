package fusion

// resolve_regopkg.go — the rego-package import strategy (LB-F132, design
// §6's closed set; the Rego dialect, LB-Q-rego). A Rego import path
// `data.x.y` names the files of the universe declaring `package x.y`, else
// the files declaring `package x` that define a rule `y` — the reading OPA
// gives an import, which the tier-A oracle (calibration/oracles/opa-parse)
// writes the same way. `import input…`, `import rego.v1` and `import
// future…` name no file. The universe's packages are read once per
// resolution environment, through its reader, by the dialect's own reader
// (extractor.RegoDeclarations).
//
// A package may span files (regal's data.regal.ast is six), so a whole
// import of one binds the package's first file (wholeIsPackage) and every
// file re-exports its own package (the dialect's FileSpecs.Exports): a call
// through the bound name resolves to the rule in whichever of the package's
// files defines it.

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// regoPackageIndex is the universe's Rego packages: the files declaring
// each package path (sorted), and the rules each file defines.
type regoPackageIndex struct {
	files map[string][]string
	rules map[string]map[string]bool
}

// regoPackages reads the universe's .rego files once (each under the
// manifest read cap).
func (e *resolveEnv) regoPackages() *regoPackageIndex {
	if e.regoPkgs != nil {
		return e.regoPkgs
	}
	x := &regoPackageIndex{files: map[string][]string{}, rules: map[string]map[string]bool{}}
	for _, p := range e.paths {
		if !strings.HasSuffix(strings.ToLower(p), ".rego") {
			continue
		}
		b, ok := e.read(p)
		if !ok || len(b) > maxManifestBytes {
			continue
		}
		pkg, rules := extractor.RegoDeclarations(b)
		if pkg == "" {
			continue
		}
		x.files[pkg] = append(x.files[pkg], p)
		rs := make(map[string]bool, len(rules))
		for _, r := range rules {
			rs[r] = true
		}
		x.rules[p] = rs
	}
	for _, fs := range x.files {
		sort.Strings(fs)
	}
	e.regoPkgs = x
	return x
}

// resolveRegoPackage resolves one import path (`data.x.y`, as the dialect
// writes it) to the files it names.
func resolveRegoPackage(env *resolveEnv, module string) []string {
	segs := strings.Split(module, ".")
	if len(segs) < 2 || segs[0] != "data" {
		return nil
	}
	idx := env.regoPackages()
	if fs := idx.files[strings.Join(segs[1:], ".")]; len(fs) > 0 {
		return append([]string(nil), fs...)
	}
	if len(segs) < 3 {
		return nil
	}
	parent, leaf := strings.Join(segs[1:len(segs)-1], "."), segs[len(segs)-1]
	var out []string
	for _, f := range idx.files[parent] {
		if idx.rules[f][leaf] {
			out = append(out, f)
		}
	}
	return out
}

// wholeIsPackage: under rego-package and cue-module a whole-module import
// names a package that may span files, and binds one of them (its first; under
// cue-module one of its own directory's, wholeFile), whose re-export of its
// package reaches the rest (see the file comment, and resolve_cuemodule.go's).
func (r *importResolver) wholeIsPackage() bool {
	return r.strategy == "rego-package" || r.strategy == "cue-module"
}
