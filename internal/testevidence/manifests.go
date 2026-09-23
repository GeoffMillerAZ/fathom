package testevidence

// manifests.go — LoadManifests (TC-2): loads and schema-validates every
// tests/*.cue file under a service's root against the mirrored test-kit
// #TestManifest definition (schema/testkit/manifest.cue, embedded here
// byte-identical to that mirror — TestTestKitManifestSchemaMatchesSource
// proves it).

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed testkit_manifest.cue
var testKitManifestSchema []byte

// LoadManifests loads every tests/*.cue file under root (a service's repo
// root), validating each against the embedded #TestManifest definition.
// Absent root/tests (no manifests declared) returns (nil, nil, nil) — "no
// manifests found" is the CALLER's named skip (TC-4), never an error here.
//
// A manifest that fails to parse or does not satisfy #TestManifest is a
// WARNING naming the file, never a hard error: one malformed manifest must
// not block every other suite's evidence from loading. Manifests are
// compiled standalone (cuecontext.CompileBytes), not through cue/load's
// module resolution: test-kit's own manifests are plain data literals with
// no imports (component/suites fields only — see
// examples/runnable/tests/cart.cue), so there is nothing a real module
// load would resolve that CompileBytes cannot, and this way root need not
// itself carry a cue.mod — the common case for a service being onboarded
// into fathom's fleet.
func LoadManifests(root string) ([]Manifest, []string, error) {
	dir := filepath.Join(root, "tests")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".cue") {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, nil, nil
	}

	cctx := cuecontext.New()
	schema := cctx.CompileBytes(testKitManifestSchema)
	if schema.Err() != nil {
		return nil, nil, fmt.Errorf("compile embedded test-kit manifest schema: %w", schema.Err())
	}
	def := schema.LookupPath(cue.ParsePath("#TestManifest"))
	if def.Err() != nil {
		return nil, nil, fmt.Errorf("look up #TestManifest in embedded schema: %w", def.Err())
	}

	var manifests []Manifest
	var warnings []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", f, err)
		}
		doc := cctx.CompileBytes(data, cue.Filename(f))
		if doc.Err() != nil {
			warnings = append(warnings, fmt.Sprintf("%s: parse: %v", f, doc.Err()))
			continue
		}
		unified := def.Unify(doc)
		if verr := unified.Validate(cue.Concrete(true)); verr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: does not satisfy #TestManifest: %v", f, verr))
			continue
		}
		m, derr := decodeManifest(unified, f)
		if derr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", f, derr))
			continue
		}
		manifests = append(manifests, m)
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].File < manifests[j].File })
	return manifests, warnings, nil
}

// decodeManifest extracts the fields fathom's stage-0 facts need
// (component, and per suite: class/tier/port) from a validated,
// concrete #TestManifest value — not a full Decode into a matching Go
// struct, since most #Suite fields (inputs, runner, needs, runtime,
// flake) are executor concerns test-kit itself never inspects either
// (schema/testkit/manifest.cue's own doc comment on #Suite.inputs).
func decodeManifest(v cue.Value, file string) (Manifest, error) {
	component, err := v.LookupPath(cue.ParsePath("component")).String()
	if err != nil {
		return Manifest{}, fmt.Errorf("component: %w", err)
	}

	iter, err := v.LookupPath(cue.ParsePath("suites")).Fields()
	if err != nil {
		return Manifest{}, fmt.Errorf("suites: %w", err)
	}

	var suites []Suite
	for iter.Next() {
		su := Suite{Name: iter.Selector().Unquoted()}
		sv := iter.Value()
		if cls := sv.LookupPath(cue.ParsePath("class")); cls.Exists() {
			su.Class, _ = cls.String()
		}
		if tier := sv.LookupPath(cue.ParsePath("tier")); tier.Exists() {
			su.Tier, _ = tier.String()
		}
		if port := sv.LookupPath(cue.ParsePath("port")); port.Exists() {
			su.Port, _ = port.String()
		}
		suites = append(suites, su)
	}
	sort.Slice(suites, func(i, j int) bool { return suites[i].Name < suites[j].Name })

	return Manifest{File: file, Component: component, Suites: suites}, nil
}
