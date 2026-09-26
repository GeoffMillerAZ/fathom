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
// readings that do exist carry the document they were published in and the
// tier they measured. The typed-edge column says what the fleet actually
// holds, not only what could produce it: its share of resolver-confirmed
// call edges is read from the store's typed_edges rows, so a language whose
// SCIP delegate was never run reads "no SCIP index imported", never as if
// its edges were typed.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// PacksView is the packs view.
type PacksView struct {
	Dialects []DialectPack `json:"dialects"`
	Layers   []LayerPack   `json:"layers"`
	// Readings are the published precision/recall readings the matrix
	// cites, one row per (language, tier) measured.
	Readings []DialectReading `json:"readings"`
	// Claims is the fleet's current governed claims (MetricClaims), the sum
	// the layer packs contribute.
	Claims Value[int] `json:"claims"`
	// LayerCount is how many layers are listed — a lower bound when a pack
	// catalog could not be read.
	LayerCount Value[int] `json:"layer_count"`
	Notes      []string   `json:"notes,omitempty"`
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
	// EdgeKinds counts the fleet's typed_edges rows whose calling file is
	// in this language, by kind: direct, iface-dispatch and func-value come
	// from a resolver (fathom's type checker, or an imported SCIP index);
	// file-resolved and unresolved are heuristic.
	EdgeKinds map[string]int `json:"edge_kinds"`
	// TypedShare is the share of those rows a resolver confirmed — absent
	// when the language's files carry no typed-edge row (call edges by
	// syntax only), a measured 0 when every row is heuristic.
	TypedShare Value[float64] `json:"typed_share"`
	// Precision and Recall are absent — "unmeasured" — unless a published
	// reading measured this language; Reading names it and ReadingTier is
	// the edge tier it measured (native, file), which may differ from the
	// language's best producer.
	Precision   Value[float64] `json:"precision"`
	Recall      Value[float64] `json:"recall"`
	Reading     string         `json:"reading,omitempty"`
	ReadingTier string         `json:"reading_tier,omitempty"`
}

// ResolvedEdges is how many of the language's typed-edge rows a resolver
// confirmed.
func (d DialectPack) ResolvedEdges() int {
	n := 0
	for kind, c := range d.EdgeKinds {
		if core.EdgeConfidenceOf(core.Edge{Kind: core.EdgeKind(kind)}) == core.EdgeConfidenceResolved {
			n += c
		}
	}
	return n
}

// EdgeTally counts one service's typed_edges rows by the language of the
// calling file (a registry tag) and by edge kind.
type EdgeTally map[string]map[string]int

// TallyTypedEdges reads svc's typed call edges and tallies them.
func TallyTypedEdges(ctx context.Context, st store.Store, svc string) (EdgeTally, error) {
	edges, err := st.TypedEdges(ctx, svc)
	if err != nil {
		return nil, err
	}
	out := EdgeTally{}
	for _, e := range edges {
		tag := connector.LanguageForPath(e.FromFile)
		if tag == "" {
			continue
		}
		if out[tag] == nil {
			out[tag] = map[string]int{}
		}
		kind := string(e.Kind)
		if kind == "" {
			kind = "syntax"
		}
		out[tag][kind]++
	}
	return out, nil
}

// LayerPack is one layer pack.
type LayerPack struct {
	Name   string `json:"name"`
	Source string `json:"source"` // embedded | project | kit | orphan | unknown
	Origin string `json:"origin"`
	// Stratum is empty when it is not known: a project pack that failed to
	// load, or a layer whose pack catalog could not be read.
	Stratum string `json:"stratum,omitempty"`
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
	out := PacksView{Dialects: []DialectPack{}, Layers: []LayerPack{}, Readings: DialectReadings, Claims: currentClaims(k)}
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
	edgeKinds := map[string]map[string]int{}
	var unreadable, edgesUnreadable []string
	for _, s := range k.Services {
		if tally, terr := src.edgeTally(ctx, s.Service); terr != nil {
			edgesUnreadable = append(edgesUnreadable, s.Service)
		} else {
			for tag, kinds := range tally {
				if edgeKinds[tag] == nil {
					edgeKinds[tag] = map[string]int{}
				}
				for kind, n := range kinds {
					edgeKinds[tag][kind] += n
				}
			}
		}
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
		row.EdgeKinds = map[string]int{}
		for kind, n := range edgeKinds[tag] {
			row.EdgeKinds[kind] = n
		}
		row.TypedShare = typedShare(row, edgesUnreadable)
		if r, ok := readings[tag]; ok {
			row.Precision = Present(r.Precision, r.Source).WithDenominator(r.Measure)
			row.Recall = Present(r.Recall, r.Source).WithDenominator(r.Measure)
			row.Reading = r.Tier + " tier: " + r.Measure + " — " + r.Source + " (" + r.Date + ")"
			row.ReadingTier = r.Tier
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
		}
		// The stratum is the pack's own; a layer with no pack (an orphan) is
		// the concern catalog's by origin. A broken pack or an unreadable
		// catalog leaves it unknown — never a default.
		p := k.Packs[name]
		switch {
		case p.broken() || b.Source == PackUnknown:
		case p != nil && p.Stratum != "":
			lp.Stratum = p.Stratum
		default:
			lp.Stratum = StratumConcern
		}
		out.Layers = append(out.Layers, lp)
	}
	out.LayerCount = layerCount(k, len(out.Layers))
	return out, nil
}

// typedShare is the share of a language's typed-edge rows a resolver
// confirmed.
func typedShare(d DialectPack, unreadable []string) Value[float64] {
	const src = "the store's typed_edges rows, by the calling file's language"
	total := 0
	for _, n := range d.EdgeKinds {
		total += n
	}
	resolved := d.ResolvedEdges()
	share := 0.0
	if total > 0 {
		share = float64(int64(float64(resolved)/float64(total)*1000+0.5)) / 1000
	}
	if len(unreadable) > 0 {
		return Partial(share, "typed edges unreadable for: "+strings.Join(unreadable, ", ")).WithSource(src)
	}
	if total == 0 {
		why := "no typed-edge row for " + d.Name + " files — call edges by syntax only"
		if d.Files.V == 0 {
			why = "no " + d.Name + " file indexed in this fleet"
		}
		return Absent[float64](why).WithSource(src)
	}
	denom := fmt.Sprintf("%s of %s call edges confirmed by a resolver", thousands(resolved), thousands(total))
	if resolved == 0 {
		denom = fmt.Sprintf("0 of %s call edges confirmed by a resolver — every one heuristic", thousands(total))
	}
	return Measured(share, src).WithDenominator(denom)
}
