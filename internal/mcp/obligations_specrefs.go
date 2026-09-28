package mcp

// obligations_specrefs.go — SR-27 (surface reckoning F-35, design §4):
// arch_conformance, spec_coverage and spec_refs were unregistered (review_diff's
// engine already ran the first two in-process). spec_refs' one real consumer —
// the edit hook's spec-ID RPC (306 calls, cmd/fathom/edithook.go) — moves here,
// as an additive `spec_refs` section on the obligations answer the hook already
// fetches, so a second round trip is never needed.
import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/sdd"
)

// specRefLoc is one {file, line} location a spec ID is cited at — the
// same shape the retired spec_refs tool's "specs" map carried, so the
// edit hook's own decode (cmd/fathom/edithook.go's specRefLoc) is
// unchanged by this move.
type specRefLoc struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// obligationsSpecRefsMaxIDs caps how many distinct spec IDs one
// obligations call resolves locations for — mirrors the edit hook's own
// maxSpecIDsPerEdit cap (cmd/fathom/edithook.go) so this additive
// section can never grow unbounded on a file that cites many IDs.
const obligationsSpecRefsMaxIDs = 20

// obligationsSpecRefsSearchLimit bounds each per-ID location search.
const obligationsSpecRefsSearchLimit = 8

// obligationsSpecRefs resolves the spec IDs `file` cites into an
// additive spec_refs section for the obligations answer:
// {spec_id: [{file, line}]}, one entry per ID `file` cites, each mapping
// to that ID's OTHER citing locations elsewhere in the service — the
// citing file itself is excluded, mirroring the retired spec_refs tool's
// own "definition, not self" filter the edit hook always applied
// (fetchSpecRefs's `if loc.File == repoRelFile { continue }`).
//
// Two point-cost reads, never a whole-tree one: `file`'s own content
// comes from a single capped read (point cost, like read_source) to
// extract cited IDs, and each ID's other locations come from `search`'s
// own bounded, streaming grep (searchRun/resolveCorpusRoot) — never
// fetchSources' aggregated whole-tree Sources read, which RT-146's
// sourcesSlot governor exists to gate on the request path. Folding this
// into the hot obligations path therefore adds no new contender for
// RT-146's two concurrent whole-tree slots, and obligations' own
// declared cost class (tree, unchanged) already covers it.
//
// Returns nil on any degrade — an unreadable file, an unresolvable
// corpus root, an ignore-list load failure, a search error — never an
// error: this is enrichment on top of the governing rules obligations
// already promises, and it must never be the reason an obligations call
// fails.
func obligationsSpecRefs(ctx context.Context, backend QueryInterface, service, file string) map[string][]specRefLoc {
	if file == "" || backend == nil {
		return nil
	}
	root, err := resolveCorpusRoot(ctx, backend, service)
	if err != nil {
		return nil
	}
	_, full, err := validateRelPath(root, file)
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return nil
	}
	ids, err := sdd.ExtractRefsFromText(string(raw), "")
	if err != nil || len(ids) == 0 {
		return nil
	}
	if len(ids) > obligationsSpecRefsMaxIDs {
		ids = ids[:obligationsSpecRefsMaxIDs]
	}

	// Mirror search's own service-scoped exclusion discipline (readsource.go's
	// notIndexedReason precedent, born of the purged-credential incident): an
	// ignore-list that fails to load refuses the enrichment outright rather
	// than searching without it.
	ignore, ignorePatterns, _, ierr := connector.LoadIgnoreScopedForWithPatterns(root, service)
	if ierr != nil {
		return nil
	}

	out := make(map[string][]specRefLoc, len(ids))
	for _, id := range ids {
		pattern, caseSensitive := specRefSearchPattern(id)
		hits, _, _, serr := searchRun(ctx, root, searchOpts{
			pattern: pattern, caseSensitive: caseSensitive,
			limit: obligationsSpecRefsSearchLimit, ignore: ignore, ignorePatterns: ignorePatterns,
		})
		if serr != nil {
			continue
		}
		var locs []specRefLoc
		for _, h := range hits {
			if h.File == file {
				continue // the citing file itself, not another location
			}
			locs = append(locs, specRefLoc{File: h.File, Line: h.Line})
		}
		if len(locs) > 0 {
			out[id] = locs
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// specRefSearchPattern builds a regexp precisely matching id's own
// citation shape — sdd.DefaultSpecRefPattern's three families, reversed:
// a bracket ID (`US-042`) searches for its exact bracketed form; a
// milestone ID (`M18.2`) searches case-insensitively (the extractor
// accepts `m18.2` too) with word boundaries; a "doc N" ID searches with
// flexible internal whitespace, case-sensitively (the extractor's doc-
// reference family is lowercase-only: `\bdoc\s+\d+\b`, no `(?i)`) — so a
// hit is a genuine citation of id, never an incidental substring match.
func specRefSearchPattern(id string) (pattern string, caseSensitive bool) {
	if digits, ok := strings.CutPrefix(id, "doc "); ok {
		return `\bdoc\s+` + regexp.QuoteMeta(digits) + `\b`, true
	}
	if strings.ContainsRune(id, '.') {
		// Milestone form: "M18.2".
		return `\b` + regexp.QuoteMeta(id) + `\b`, false
	}
	// Bracket form: "US-042", "EP-013", "FR-7", "NFR-1".
	return `\[` + regexp.QuoteMeta(id) + `\]`, true
}
