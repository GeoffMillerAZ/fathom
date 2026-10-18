package langbench

// Unity assembly calibration compares declarations to an independently retained
// Editor receipt. It neither compiles C# nor invents typed/runtime call edges.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const unityAssemblyOracle = "UnityEditor.Compilation.CompilationPipeline.GetAssemblies"

type UnityAssemblyOracle struct {
	SchemaVersion          int             `json:"schemaVersion"`
	Oracle                 string          `json:"oracle"`
	UnityVersion           string          `json:"unityVersion"`
	BuildTarget            string          `json:"buildTarget"`
	AssembliesType         string          `json:"assembliesType"`
	Scope                  string          `json:"scope"`
	AvailableAssemblyNames []string        `json:"availableAssemblyNames"`
	Assemblies             []UnityAssembly `json:"assemblies"`
}

type UnityAssembly struct {
	Name                          string            `json:"name"`
	AsmdefPath                    string            `json:"asmdefPath"`
	Flags                         string            `json:"flags"`
	OutputPath                    string            `json:"outputPath"`
	DefinitionSHA256              string            `json:"definitionSha256"`
	NoEngineReferencesDeclared    bool              `json:"noEngineReferencesDeclared"`
	OverrideReferencesDeclared    bool              `json:"overrideReferencesDeclared"`
	PrecompiledReferencesDeclared []string          `json:"precompiledReferencesDeclared"`
	DeclaredReferences            []string          `json:"declaredReferences"`
	AssemblyReferences            []string          `json:"assemblyReferences"`
	EmittedReferencesAvailable    bool              `json:"emittedReferencesAvailable"`
	EmittedAssemblyReferences     []string          `json:"emittedAssemblyReferences"`
	CompiledAssemblyReferences    []string          `json:"compiledAssemblyReferences"`
	SourceFiles                   []string          `json:"sourceFiles"`
	SourceFacts                   []UnitySourceFact `json:"sourceFacts"`
}

type UnitySourceFact struct {
	Path   string `json:"path"`
	Owner  string `json:"owner"`
	SHA256 string `json:"sha256"`
}
type UnityAsmdefProblem struct {
	Assembly string `json:"assembly"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Detail   string `json:"detail"`
}
type UnityAsmdefReading struct {
	SchemaVersion          int                  `json:"schema_version"`
	Evidence               string               `json:"evidence"`
	NoEnginePolicy         string               `json:"no_engine_policy"`
	OracleSHA256           string               `json:"oracle_sha256"`
	UnityVersion           string               `json:"unity_version"`
	BuildTarget            string               `json:"build_target"`
	AssembliesType         string               `json:"assemblies_type"`
	Passed                 bool                 `json:"passed"`
	Complete               bool                 `json:"complete"`
	Assemblies             int                  `json:"assemblies"`
	DefinitionMatches      int                  `json:"definition_matches"`
	SourceFiles            int                  `json:"source_files"`
	OwnerMatches           int                  `json:"owner_matches"`
	NamedReferences        int                  `json:"named_references"`
	ReferenceMatches       int                  `json:"reference_matches"`
	NoEngineAssertions     int                  `json:"no_engine_assertions"`
	NoEngineMatches        int                  `json:"no_engine_matches"`
	EmittedReferenceChecks int                  `json:"emitted_reference_checks"`
	EmittedNoUnityMatches  int                  `json:"emitted_no_unity_matches"`
	Notices                []UnityAsmdefProblem `json:"notices"`
	Problems               []UnityAsmdefProblem `json:"problems"`
	Unjudged               []UnityAsmdefProblem `json:"unjudged"`
}

type unityDefinition struct {
	name        string
	refs        []string
	noEngine    bool
	override    bool
	precompiled []string
}

// CalibrateUnityAsmdefs reads only Assets assembly declarations and the exact
// source files named by a receipt. All receipt paths must be valid relative FS
// paths. The caller supplies a snapshot FS, with no symlinks outside its root.
// A successful comparison is compiler calibration of this receipt's observed
// assembly configuration, not a language support grade or a call-graph oracle.
func CalibrateUnityAsmdefs(files fs.FS, raw []byte) (UnityAsmdefReading, error) {
	var oracle UnityAssemblyOracle
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&oracle); err != nil {
		return UnityAsmdefReading{}, fmt.Errorf("Unity assembly receipt: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return UnityAsmdefReading{}, fmt.Errorf("Unity assembly receipt: trailing JSON data")
	}
	if oracle.SchemaVersion != 1 || oracle.Oracle != unityAssemblyOracle || oracle.UnityVersion == "" || oracle.BuildTarget == "" || (oracle.AssembliesType != "Editor" && oracle.AssembliesType != "Player") || len(oracle.Assemblies) == 0 {
		return UnityAsmdefReading{}, fmt.Errorf("Unity assembly receipt: unsupported or incomplete header")
	}
	r := UnityAsmdefReading{SchemaVersion: 1, Evidence: "declared-assembly-facts; compiler-calibrated; no typed or runtime call edges", NoEnginePolicy: "compiler core: no UnityEngine* or exact UnityEditor/UnityEditor.CoreModule; other available Unity SDK/plugin names disclosed; emitted metadata: no Unity* dependencies when observed",
		OracleSHA256: unityHash(raw), UnityVersion: oracle.UnityVersion, BuildTarget: oracle.BuildTarget, AssembliesType: oracle.AssembliesType, Problems: []UnityAsmdefProblem{}, Unjudged: []UnityAsmdefProblem{}, Notices: []UnityAsmdefProblem{}}
	problem := func(a, p, k, d string) { r.Problems = append(r.Problems, UnityAsmdefProblem{a, p, k, d}) }
	unjudged := func(a, p, k, d string) { r.Unjudged = append(r.Unjudged, UnityAsmdefProblem{a, p, k, d}) }
	x, err := extractor.NewDocDialectExtractor("unity-asmdef")
	if err != nil {
		return r, err
	}
	defs := map[string]unityDefinition{}
	dirs := map[string][]string{}
	asmrefs := map[string]bool{}
	err = fs.WalkDir(files, "Assets", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("Unity calibration refuses symlink %q", p)
		}
		if d.IsDir() {
			return nil
		}
		if path.Ext(p) == ".asmref" {
			asmrefs[path.Dir(p)] = true
			return nil
		}
		if path.Ext(p) != ".asmdef" {
			return nil
		}
		b, err := fs.ReadFile(files, p)
		if err != nil {
			return err
		}
		result, err := x.ExtractDoc("unity-calibration", p, b)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		def := unityDefinition{}
		for _, n := range result.Nodes {
			if string(n.Kind) == "UnityAssemblyDefinition" {
				if def.name != "" {
					return fmt.Errorf("%s: multiple definitions", p)
				}
				def.name = n.Name
			}
		}
		if def.name == "" {
			return fmt.Errorf("%s: no named assembly definition", p)
		}
		for _, f := range result.Facts {
			switch f.Key {
			case "reference":
				def.refs = append(def.refs, f.Value)
			case "noEngineReferences":
				def.noEngine = f.Value == "true"
			case "overrideReferences":
				def.override = f.Value == "true"
			case "precompiledReference":
				def.precompiled = append(def.precompiled, f.Value)
			}
		}
		defs[p] = def
		dirs[path.Dir(p)] = append(dirs[path.Dir(p)], p)
		return nil
	})
	if err != nil {
		return r, fmt.Errorf("Unity project snapshot: %w", err)
	}
	available := unitySet(oracle.AvailableAssemblyNames)
	seenAssemblies := map[string]bool{}
	for _, a := range oracle.Assemblies {
		r.Assemblies++
		if seenAssemblies[a.Name] {
			problem(a.Name, a.AsmdefPath, "duplicate-assembly", "oracle repeats an assembly")
			continue
		}
		seenAssemblies[a.Name] = true
		if !unityPath(a.AsmdefPath) || path.Ext(a.AsmdefPath) != ".asmdef" {
			problem(a.Name, a.AsmdefPath, "invalid-definition-path", "expected a project-relative Assets .asmdef path")
			continue
		}
		b, readErr := fs.ReadFile(files, a.AsmdefPath)
		def, found := defs[a.AsmdefPath]
		if readErr != nil || !found {
			problem(a.Name, a.AsmdefPath, "definition-unreadable", "definition is missing from snapshot")
			continue
		}
		if a.DefinitionSHA256 == "" || unityHash(b) != a.DefinitionSHA256 {
			problem(a.Name, a.AsmdefPath, "definition-digest-mismatch", "receipt does not bind current declaration")
			continue
		}
		if def.name != a.Name {
			problem(a.Name, a.AsmdefPath, "assembly-name-mismatch", def.name)
			continue
		}
		r.DefinitionMatches++
		if !reflect.DeepEqual(unitySorted(def.refs), unitySorted(a.DeclaredReferences)) {
			problem(a.Name, a.AsmdefPath, "declaration-reference-mismatch", "extracted declarations differ from receipt")
		}
		if def.noEngine != a.NoEngineReferencesDeclared {
			problem(a.Name, a.AsmdefPath, "no-engine-declaration-mismatch", "extracted declaration differs from receipt")
		}
		if def.override != a.OverrideReferencesDeclared {
			problem(a.Name, a.AsmdefPath, "override-declaration-mismatch", "extracted declaration differs from receipt")
		}
		if !reflect.DeepEqual(unitySorted(def.precompiled), unitySorted(a.PrecompiledReferencesDeclared)) {
			problem(a.Name, a.AsmdefPath, "precompiled-declaration-mismatch", "extracted declaration differs from receipt")
		}
		compiled := unitySet(a.AssemblyReferences)
		for _, ref := range def.refs {
			if strings.HasPrefix(ref, "GUID:") {
				unjudged(a.Name, a.AsmdefPath, "guid-reference", ref)
				continue
			}
			r.NamedReferences++
			if !available[ref] {
				problem(a.Name, a.AsmdefPath, "reference-not-available", ref)
			} else if !compiled[ref] {
				problem(a.Name, a.AsmdefPath, "reference-not-compiled", ref)
			} else {
				r.ReferenceMatches++
			}
		}
		if def.noEngine {
			r.NoEngineAssertions++
			if a.CompiledAssemblyReferences == nil {
				problem(a.Name, a.AsmdefPath, "compiled-references-missing", "engine-free check requires an observed compiled reference list")
				continue
			}
			engine := false
			for _, ref := range a.CompiledAssemblyReferences {
				base := strings.TrimSuffix(path.Base(strings.ReplaceAll(ref, "\\", "/")), ".dll")
				if base == "UnityEngine" || strings.HasPrefix(base, "UnityEngine.") || base == "UnityEditor" || base == "UnityEditor.CoreModule" {
					engine = true
					problem(a.Name, a.AsmdefPath, "engine-reference-present", base)
				} else if strings.HasPrefix(base, "Unity") {
					r.Notices = append(r.Notices, UnityAsmdefProblem{a.Name, a.AsmdefPath, "other-unity-prefixed-available-reference", base + ": compiler availability is not emitted use; may be an injected SDK or plugin"})
				}
			}
			if !engine {
				r.NoEngineMatches++
			}
			if !a.EmittedReferencesAvailable || a.EmittedAssemblyReferences == nil {
				unjudged(a.Name, a.AsmdefPath, "emitted-references-unavailable", "loaded assembly reference metadata was not observed")
			} else {
				r.EmittedReferenceChecks++
				emittedUnity := false
				for _, ref := range a.EmittedAssemblyReferences {
					base := strings.TrimSuffix(path.Base(strings.ReplaceAll(ref, "\\", "/")), ".dll")
					if strings.HasPrefix(base, "Unity") {
						emittedUnity = true
						problem(a.Name, a.AsmdefPath, "emitted-unity-reference", base)
					}
				}
				if !emittedUnity {
					r.EmittedNoUnityMatches++
				}
			}
		}
		facts := map[string]UnitySourceFact{}
		for _, f := range a.SourceFacts {
			if _, dup := facts[f.Path]; dup {
				problem(a.Name, f.Path, "duplicate-source-fact", "receipt repeats source")
			}
			facts[f.Path] = f
		}
		sources := unitySet(a.SourceFiles)
		for p := range facts {
			if !sources[p] {
				problem(a.Name, p, "unexpected-source-fact", "not in compiler sourceFiles")
			}
		}
		for _, p := range unitySorted(a.SourceFiles) {
			r.SourceFiles++
			if !unityPath(p) || path.Ext(p) != ".cs" {
				problem(a.Name, p, "invalid-source-path", "expected a project-relative Assets .cs path")
				continue
			}
			f, ok := facts[p]
			if !ok {
				problem(a.Name, p, "source-fact-missing", "source lacks owner and digest")
				continue
			}
			b, err := fs.ReadFile(files, p)
			if err != nil {
				problem(a.Name, p, "source-unreadable", "compiler source missing from snapshot")
				continue
			}
			if f.SHA256 == "" || unityHash(b) != f.SHA256 {
				problem(a.Name, p, "source-digest-mismatch", "receipt does not bind current source")
				continue
			}
			owner := ""
			unsupported := false
			for d := path.Dir(p); d != "."; d = path.Dir(d) {
				if asmrefs[d] {
					unsupported = true
					unjudged(a.Name, p, "asmref-owner", "assembly reference directories are unsupported")
					break
				}
				if ds := dirs[d]; len(ds) > 0 {
					if len(ds) != 1 {
						problem(a.Name, p, "ambiguous-owner", "multiple asmdefs in nearest directory")
					} else {
						owner = defs[ds[0]].name
					}
					break
				}
			}
			if unsupported {
				continue
			}
			if owner != a.Name || strings.TrimSuffix(f.Owner, ".dll") != a.Name {
				problem(a.Name, p, "owner-mismatch", fmt.Sprintf("nearest=%s oracle=%s assembly=%s", owner, f.Owner, a.Name))
			} else {
				r.OwnerMatches++
			}
		}
	}
	sortProblems := func(ps []UnityAsmdefProblem) {
		sort.Slice(ps, func(i, j int) bool {
			a, b := ps[i], ps[j]
			return a.Assembly+"\x00"+a.Path+"\x00"+a.Kind+"\x00"+a.Detail < b.Assembly+"\x00"+b.Path+"\x00"+b.Kind+"\x00"+b.Detail
		})
	}
	sortProblems(r.Problems)
	sortProblems(r.Unjudged)
	sortProblems(r.Notices)
	r.Passed = len(r.Problems) == 0
	r.Complete = len(r.Unjudged) == 0
	return r, nil
}

func unityHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func unityPath(p string) bool {
	return fs.ValidPath(p) && strings.HasPrefix(p, "Assets/") && !strings.Contains(p, "\\")
}
func unitySet(ss []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range ss {
		m[s] = true
	}
	return m
}
func unitySorted(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
