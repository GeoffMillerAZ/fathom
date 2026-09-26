package console

// packs.go — the packs view (vision 35 §4.5 "the dialect matrix", design
// §3 "Packs", plan OC-31): dialect packs as a language × capability matrix
// — engine, typed-edge producer, fleet use, integrity support, inventory
// keys, and measured precision and recall where a reading exists — and
// layer packs with their source and the projects where each is active.
// `fathom report packs --json` prints exactly this; /packs renders it.
//
// A capability no reading measured says "unmeasured", never 0: the
// precision and recall columns are absent values naming why, and the
// readings that do exist carry the document they were published in.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// PacksView is the packs view.
type PacksView struct {
	Dialects []DialectPack `json:"dialects"`
	Layers   []LayerPack   `json:"layers"`
	// Readings are the published precision/recall readings the matrix
	// cites, one row per (language, tier) measured.
	Readings []DialectReading `json:"readings"`
	Notes    []string         `json:"notes,omitempty"`
}

// DialectPack is one language's row.
type DialectPack struct {
	Language   string   `json:"language"` // the registry's tag ("go", "ts", …)
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`
	Engine     string   `json:"engine"`
	// TypedEdges is where the language's typed call edges come from:
	// native (fathom resolves types), scip (an imported SCIP index), file
	// (the per-file resolver) or syntax (call names by syntax only).
	TypedEdges string `json:"typed_edges"`
	Producer   string `json:"producer,omitempty"`
	// Files and Projects are the fleet's indexed files in this language
	// and the projects holding any.
	Files    Value[int] `json:"files"`
	Projects Value[int] `json:"projects"`
	// Integrity is the conservation integrity plane's reach: "checked" for
	// Go; "unproven" for a language whose tests the plane names but cannot
	// check; "n/a" otherwise.
	Integrity string `json:"integrity"`
	// Inventory is the capability-inventory kinds extracted from this
	// language's files (api, cli, env, json, route, rpc).
	Inventory []string `json:"inventory"`
	// Precision and Recall are absent — "unmeasured" — unless a published
	// reading measured this language; Reading names it.
	Precision Value[float64] `json:"precision"`
	Recall    Value[float64] `json:"recall"`
	Reading   string         `json:"reading,omitempty"`
}

// LayerPack is one layer pack.
type LayerPack struct {
	Name    string `json:"name"`
	Source  string `json:"source"` // embedded | project | kit | orphan
	Origin  string `json:"origin"`
	Stratum string `json:"stratum"`
	Kind    string `json:"kind,omitempty"`
	Role    string `json:"role,omitempty"`
	Evaled  bool   `json:"evaled"`
	// ActiveIn names the projects with current claims from this layer;
	// Contributes is how many.
	ActiveIn    []string   `json:"active_in"`
	InstalledIn []string   `json:"installed_in,omitempty"`
	Contributes Value[int] `json:"contributes"`
	Note        string     `json:"note,omitempty"`
}

// DialectReading is one published precision/recall reading.
type DialectReading struct {
	Language  string  `json:"language"`
	Tier      string  `json:"tier"`
	Measure   string  `json:"measure"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	Source    string  `json:"source"`
	Date      string  `json:"date"`
}

// DialectReadings are the readings fathom has published per language.
// Each row restates a number from the document Source names; a language
// with no row is unmeasured. TestPackMatrix_MeasuredOrSaidUnmeasured holds
// every Source to a file in the repository.
var DialectReadings = []DialectReading{
	{Language: "go", Tier: "native", Measure: "find_callers against a go/types truth (L0 index oracle, 200 sampled queries)", Precision: 1.0, Recall: 1.0, Source: "docs/bench/README.md", Date: "2026-09-12"},
	{Language: "ts", Tier: "file", Measure: "file-resolved call edges against scip-typescript on zod (CV-M7)", Precision: 0.87, Recall: 0.03, Source: "docs/bench/readings/l0-ts-py-2026-09-24.md", Date: "2026-09-24"},
	{Language: "py", Tier: "file", Measure: "file-resolved call edges against scip-python on flask (CV-M7)", Precision: 0.95, Recall: 0.12, Source: "docs/bench/readings/l0-ts-py-2026-09-24.md", Date: "2026-09-24"},
}

// languageNames is the registry tags' display names (docs/languages.md).
var languageNames = map[string]string{
	"go": "Go", "cue": "CUE", "ts": "TypeScript", "js": "JavaScript", "py": "Python",
	"java": "Java", "kotlin": "Kotlin", "scala": "Scala", "rust": "Rust", "c": "C",
	"cpp": "C++", "ruby": "Ruby", "cs": "C#", "php": "PHP", "sh": "Bash", "lua": "Lua",
	"sql": "SQL", "terraform": "Terraform/HCL", "yaml": "YAML (Kubernetes, Helm, compose)",
	"md": "Markdown", "dockerfile": "Dockerfile", "cmake": "CMake", "proto": "Protocol Buffers",
	"graphql": "GraphQL", "toml": "TOML", "json": "JSON", "templ": "templ",
}

// LanguageName is a registry tag's display name (the tag itself when none).
func LanguageName(tag string) string {
	if n, ok := languageNames[tag]; ok {
		return n
	}
	return tag
}

// inventoryKinds is the capability-inventory kinds
// internal/analysis/inventory.Extract draws from each language's files
// (api/json/cli from Go's parser, route/env from the shared analyzers for
// Go, TypeScript, JavaScript and Python, rpc from .proto); a configured
// [[surface.pattern]] adds kinds for any file.
func inventoryKinds(tag string) []string {
	switch tag {
	case "go":
		return []string{"api", "cli", "env", "json", "route"}
	case "ts", "js", "py":
		return []string{"env", "route"}
	case "proto":
		return []string{"rpc"}
	}
	return []string{}
}

// integrityOf is the integrity plane's reach (internal/conserve
// plane_integrity.go: Go only; a test file in another code language is
// named unproven, never silently skipped).
func integrityOf(tag string) string {
	switch tag {
	case "go":
		return "checked"
	case "py", "js", "ts", "java", "cs", "c", "cpp", "rust", "scala", "php":
		return "unproven"
	}
	return "n/a"
}

// engineOf names the extraction engine for a language's extensions.
func engineOf(tag string, exts []string) string {
	switch tag {
	case "go":
		return "go/parser + go/types"
	case "cue":
		return "cue/load"
	}
	tree := extractor.EngineProvenance()
	for _, e := range exts {
		if tree[e] != "" {
			return "tree-sitter (wasm)"
		}
	}
	return "Go extractor (deterministic, no grammar)"
}

// PackMatrix computes the packs view from the registry, the published
// readings and the store.
func PackMatrix(ctx context.Context, src KnowledgeSource) (PacksView, error) {
	k, err := readKnowledge(ctx, src)
	if err != nil {
		return PacksView{}, err
	}
	out := PacksView{Dialects: []DialectPack{}, Layers: []LayerPack{}, Readings: DialectReadings}
	for _, p := range k.PackErr {
		out.Notes = append(out.Notes, "pack catalog unreadable: "+p)
	}

	// Registered languages and their extensions.
	exts := map[string][]string{}
	for _, e := range connector.RegisteredExtensions() {
		if tag := connector.LanguageForPath(e); tag != "" {
			exts[tag] = append(exts[tag], e)
		}
	}
	// Fleet use: indexed files per language, from every project's
	// fingerprinted source refs.
	files := map[string]int{}
	projects := map[string]map[string]bool{}
	var unreadable []string
	for _, s := range k.Services {
		refs, rerr := src.Store.ListSourceRefs(ctx, s.Service)
		if rerr != nil {
			unreadable = append(unreadable, s.Service)
			continue
		}
		for _, ref := range refs {
			tag := connector.LanguageForPath(ref)
			if _, known := exts[tag]; !known {
				continue
			}
			files[tag]++
			if projects[tag] == nil {
				projects[tag] = map[string]bool{}
			}
			projects[tag][s.Service] = true
		}
	}
	readings := map[string]DialectReading{}
	for _, r := range DialectReadings {
		readings[r.Language] = r
	}
	const useSource = "each project's indexed source files (fingerprints)"
	for tag, es := range exts {
		sort.Strings(es)
		level, tool, _ := strings.Cut(connector.TypedEdgeProducer(tag), ":")
		row := DialectPack{
			Language: tag, Name: LanguageName(tag), Extensions: es, Engine: engineOf(tag, es),
			TypedEdges: level, Producer: tool, Integrity: integrityOf(tag), Inventory: inventoryKinds(tag),
		}
		if row.TypedEdges == "" {
			row.TypedEdges = "syntax"
		}
		denom := fmt.Sprintf("across %s", countNoun(len(k.Services), "project"))
		if len(unreadable) > 0 {
			reason := "lower bound; unreadable: " + strings.Join(unreadable, ", ")
			row.Files = Partial(files[tag], reason).WithSource(useSource)
			row.Projects = Partial(len(projects[tag]), reason).WithSource(useSource)
		} else {
			row.Files = Measured(files[tag], useSource).WithDenominator(denom)
			row.Projects = Measured(len(projects[tag]), useSource).WithDenominator(denom)
		}
		if r, ok := readings[tag]; ok {
			row.Precision = Present(r.Precision, r.Source).WithDenominator(r.Measure)
			row.Recall = Present(r.Recall, r.Source).WithDenominator(r.Measure)
			row.Reading = r.Tier + " tier: " + r.Measure + " — " + r.Source + " (" + r.Date + ")"
		} else {
			why := "unmeasured — no published precision/recall reading for " + row.Name
			row.Precision = Absent[float64](why)
			row.Recall = Absent[float64](why)
		}
		out.Dialects = append(out.Dialects, row)
	}
	// Most-used first, then by name: the matrix leads with what the fleet
	// actually indexes.
	sort.Slice(out.Dialects, func(i, j int) bool {
		a, b := out.Dialects[i], out.Dialects[j]
		if a.Files.V != b.Files.V {
			return a.Files.V > b.Files.V
		}
		return a.Name < b.Name
	})

	for _, name := range k.layerNames() {
		b := layerBlock(k, name)
		lp := LayerPack{
			Name: name, Source: b.Source, Origin: b.Origin, Kind: b.Kind, Role: b.Role, Evaled: b.Evaled,
			ActiveIn: b.Services, InstalledIn: b.InstalledIn, Contributes: b.Claims, Note: b.Note,
			Stratum: StratumConcern,
		}
		if p := k.Packs[name]; p != nil && p.Stratum != "" {
			lp.Stratum = p.Stratum
		}
		out.Layers = append(out.Layers, lp)
	}
	return out, nil
}
