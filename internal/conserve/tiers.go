package conserve

// tiers.go — TiersByLanguage (CV-38, ADR-044 §5): "every conservation
// report names the tier per language touched" — the edge-confidence floor
// a plane's absence/surface claims for that language actually stood on,
// so a Report never lets a heuristic-only language's findings read as
// trustworthy as Go's own native typed pass without saying so.
//
// Report.Tiers is left nil by Run itself (conserve.go's own doc comment);
// a caller (cmd/fathom's cmdConserve, CV-19) calls TiersByLanguage after
// Run and assigns the result to Report.Tiers.

import (
	"context"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// The four tier labels TiersByLanguage reports (design doc §5's own
// wording): Go/CUE's native typed pass, an imported SCIP index, the
// file-resolved (SU-11) per-file resolver, or nothing beyond syntax-only
// call names.
const (
	TierTyped        = "typed"
	TierSCIP         = "scip"
	TierFileResolved = "file-resolved"
	TierSyntax       = "syntax"
)

// scipEligible is the set of languages whose ONLY typed-edge sources are
// an imported SCIP index (SU-9) or the file-resolved fallback (SU-11) —
// internal/fusion's native typed pass (EnrichTypedEdges) is Go-only, so a
// non-file-resolved, non-empty Edge.Kind from one of these languages'
// files can only have come from a SCIP import.
var scipEligible = map[string]bool{"TypeScript": true, "JavaScript": true, "Python": true}

// TiersByLanguage reports the edge tier backing the store's typed-edge
// facts for every language among in.ChangedFiles (sorted by Language) —
// "typed" for Go/CUE unconditionally (their own native pass needs no
// store lookup to confirm); for TypeScript/JavaScript/Python, "scip" when
// the store holds at least one typed (non-empty, non-file-resolved) edge
// from that language's files, "file-resolved" when it holds only
// core.EdgeFileResolved edges from them, "syntax" when it holds neither;
// every other language is unconditionally "syntax" (docs/languages.md:
// no typed-edge source beyond heuristic call names exists for them yet).
// A nil/erroring Store degrades every scip-eligible language's tier to
// "syntax" rather than failing outright — TiersByLanguage never returns
// an error; a caller that cares whether the store was even consulted
// reads Input.Store itself.
func TiersByLanguage(ctx context.Context, in Input) []TierInfo {
	langs := languagesOf(in.ChangedFiles)
	if len(langs) == 0 {
		return nil
	}

	needsEdges := false
	for _, l := range langs {
		if scipEligible[l] {
			needsEdges = true
			break
		}
	}
	var edges []core.Edge
	if needsEdges && in.Store != nil {
		if all, err := in.Store.TypedEdges(ctx, in.Service); err == nil {
			edges = all
		}
	}

	out := make([]TierInfo, 0, len(langs))
	for _, l := range langs {
		out = append(out, TierInfo{Language: l, Tier: tierFor(l, edges)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Language < out[j].Language })
	return out
}

func tierFor(lang string, edges []core.Edge) string {
	switch lang {
	case "Go", "CUE":
		return TierTyped
	}
	if !scipEligible[lang] {
		return TierSyntax
	}
	sawTyped, sawFileResolved := false, false
	for _, e := range edges {
		if languageOfFile(e.FromFile) != lang {
			continue
		}
		switch {
		case e.Kind == core.EdgeFileResolved:
			sawFileResolved = true
		case e.Kind != "" && e.Kind != core.EdgeUnresolved:
			sawTyped = true
		}
	}
	switch {
	case sawTyped:
		return TierSCIP
	case sawFileResolved:
		return TierFileResolved
	default:
		return TierSyntax
	}
}

// languagesOf returns the sorted, deduplicated set of languages
// (docs/languages.md's own names) among files — a file whose extension
// names no known language contributes nothing (a Markdown doc, a JSON
// fixture, … is not "a language touched").
func languagesOf(files []string) []string {
	seen := map[string]bool{}
	for _, f := range files {
		if l := languageOfFile(f); l != "" {
			seen[l] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// languageOfFile maps path's extension to docs/languages.md's own
// language name, "" for anything not recognized.
func languageOfFile(path string) string {
	switch {
	case strings.HasSuffix(path, ".go"):
		return "Go"
	case strings.HasSuffix(path, ".cue"):
		return "CUE"
	case strings.HasSuffix(path, ".ts"), strings.HasSuffix(path, ".tsx"):
		return "TypeScript"
	case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".jsx"), strings.HasSuffix(path, ".mjs"), strings.HasSuffix(path, ".cjs"):
		return "JavaScript"
	case strings.HasSuffix(path, ".py"):
		return "Python"
	case strings.HasSuffix(path, ".java"):
		return "Java"
	case strings.HasSuffix(path, ".kt"), strings.HasSuffix(path, ".kts"):
		return "Kotlin"
	case strings.HasSuffix(path, ".scala"):
		return "Scala"
	case strings.HasSuffix(path, ".rs"):
		return "Rust"
	case strings.HasSuffix(path, ".c"), strings.HasSuffix(path, ".h"):
		return "C"
	case strings.HasSuffix(path, ".cpp"), strings.HasSuffix(path, ".cc"), strings.HasSuffix(path, ".hpp"):
		return "C++"
	case strings.HasSuffix(path, ".rb"):
		return "Ruby"
	case strings.HasSuffix(path, ".cs"):
		return "C#"
	case strings.HasSuffix(path, ".php"):
		return "PHP"
	case strings.HasSuffix(path, ".swift"):
		return "Swift"
	case strings.HasSuffix(path, ".lua"):
		return "Lua"
	case strings.HasSuffix(path, ".sh"), strings.HasSuffix(path, ".bash"):
		return "Bash"
	case strings.HasSuffix(path, ".sql"):
		return "SQL"
	case strings.HasSuffix(path, ".tf"), strings.HasSuffix(path, ".hcl"):
		return "Terraform/HCL"
	default:
		return ""
	}
}
