package semantic

// governingscope.go — VB-8 (docs/plans/verbs.md, ADR-035 decision 2): the
// obligations engine's PRIMARY ranking key becomes proximity, not glob
// length.
//
// Before this file, obligationsCore's match functions returned `len(g)` —
// the byte length of the longest matching scope glob — as the sole
// tie-breaker inside a role/freshness tier (ObligationsFor's inline
// closure). That heuristic happens to correlate with real proximity for
// most globs this repo's compiler emits ("internal/store/**" is both
// longer AND closer than "**/*.go"), but it is an accident of string
// length, not a rule: a short, package-scoped glob ("schema/**", 9 bytes)
// loses to a longer, root-reaching one ("**/schema.cue", 13 bytes) that
// merely happens to spell out more characters while governing everything.
//
// rankByGoverningScope replaces that accident with the four-tier proximity
// ADR-035 decision 2 and vision 29 §3 wave B name explicitly: an exact-file
// match beats a package (the file's own directory) beats a named ancestor
// directory (nearer wins) beats the repo root. The byte-length score
// survives as the KEYWORD tie-break inside a tier — two claims scoped to
// the same directory still need a deterministic order, and glob length is
// as good a proxy as any for "which of two co-equal globs says more".
import (
	"path"
	"strings"
)

// governingScope classifies how closely one of a claim's scope globs names
// the edited path — nearest first: an exact match on the file itself, its
// own package directory, a named ancestor directory (a deeper/nearer
// ancestor beats a shallower one), then the repo root. tier -1
// (noGoverningScope) means no glob in the claim's scope reaches path at
// all under this classification.
type governingScope struct {
	tier  int // 0 exact file, 1 package, 2 parent dir, 3 root; -1 no match
	depth int // matched anchor directory's path-segment count (tier 2 tie-break: deeper wins)
}

// noGoverningScope is the zero/absent classification — never closer than
// any real match.
var noGoverningScope = governingScope{tier: -1}

// closerThan reports whether a ranks nearer to the edited path than b:
// lower tier wins outright; within the parent-directory tier, the deeper
// (nearer) anchor wins. Two scopes with the SAME tier and depth are a true
// tie — the caller's keyword score (or final claim-id order) decides.
func (a governingScope) closerThan(b governingScope) bool {
	if a.tier != b.tier {
		return a.tier < b.tier
	}
	return a.depth > b.depth
}

// name renders the tier as the vocabulary VB-10's displaced-by-tier receipt
// uses: "exact_file", "package", "parent_dir", "root" — "unscoped" only for
// the defensive -1 case, which a claim that already passed the scope MATCH
// gate should never reach.
func (g governingScope) name() string {
	switch g.tier {
	case 0:
		return "exact_file"
	case 1:
		return "package"
	case 2:
		return "parent_dir"
	case 3:
		return "root"
	default:
		return "unscoped"
	}
}

// rankByGoverningScope returns the closest (best) proximity of any glob in
// scope to the edited path — the whole point of a claim's scope being a
// LIST of globs is that any one of them can be the reason it governs a
// file, so the claim earns its nearest, not its farthest, match.
// Deterministic: same scope and path, same tier and depth, on every call
// and in any input order.
func rankByGoverningScope(scope []string, path string) governingScope {
	best := noGoverningScope
	fileDir := dirOf(path)
	for _, g := range scope {
		gs := classifyGlob(g, fileDir, path)
		if gs.tier < 0 {
			continue
		}
		if best.tier < 0 || gs.closerThan(best) {
			best = gs
		}
	}
	return best
}

// crossTierCloser (wave O3, item 1; ADR-037 addendum) is closerThan's own
// caller-facing replacement in both rankers (obligationsCore's less() and
// RawFallbackRules' candidate sort): proximity is still the PRIMARY key —
// this exactly reproduces a.closerThan(b) whenever neither side qualifies
// for promotion — but a farther-tier candidate whose lexical relevance is
// STRONG (aStrong/bStrong — relevanceStrength's own verdict: a hit on a
// packageWeight-or-above token, i.e. the file's own package path, stem, or
// a caller-resolved symbol/behaviour name, never a mere shared ancestor or
// extension) is promoted ahead of a nearer-tier candidate that is NOT
// itself strong — "the nearer rule names the file" is judged by the exact
// same packageWeight-or-above bar, never by a bare hit of any weight.
//
// wave O4, item 1 (docs/bench readings, two more H1 misses: T02/T13)
// narrowed this from "the nearer candidate is a TOTAL lexical blank
// (relevance == 0)" to "the nearer candidate is not STRONG": an
// ancestor-directory token alone ("internal", "cmd", "docs" — every
// sibling file under that guide shares it, and a nested guide's own
// heading or prose mentions its own root directory constantly in passing)
// is not evidence that "the nearer rule names the file" — it is evidence
// the nearer rule names the whole subtree the file merely lives in. H1-T02
// (internal/core/model.go vs. internal/CLAUDE.md) was exactly this: every
// one of internal/CLAUDE.md's OTHER same-tier sections carried a weak,
// ancestor-only hit on the bare token "internal", which under the old
// relevance==0 test blocked ADR-021 (strong on core.Edge) from ever being
// promoted past them, even though none of them names model.go's own
// package ("core") or anything about it. Only a hit at packageWeight or
// above — the file's own package directory, stem, or a caller-resolved
// symbol/behaviour token, exactly what relevanceStrength's own "strong"
// bit already means — now blocks promotion; a bare ancestor or extension
// hit stays a within-tier tie-break ONLY (see the caller's own relevance
// sort immediately below this comparison), never a cross-tier blocker.
// This is still deliberately narrow: the moment the nearer candidate is
// ITSELF strong, proximity wins outright and no promotion happens — a
// nearer guide's own section that actually names the file's package is
// never leapfrogged by a farther, differently-specific rule. Ties (both
// sides strong, or neither farther side is strong) fall straight through
// to ordinary proximity — VB-8's tier/depth primacy is unchanged in every
// case this doesn't apply to. The relevance SCORES themselves are no
// longer read here at all (only the strong bit is) — a nearer candidate's
// magnitude of ancestor-only relevance never affects this decision, only
// the within-tier sort that runs when tiers are equal.
func crossTierCloser(a, b governingScope, aStrong, bStrong bool) bool {
	if a.tier != b.tier {
		switch {
		case aStrong && !bStrong:
			return true // a is farther but strong; b (nearer) has no more than an ancestor/extension hit
		case bStrong && !aStrong:
			return false // symmetric: b leapfrogs a
		}
	}
	return a.closerThan(b)
}

// dirOf is path.Dir normalised so a repo-root file's "directory" is "" —
// the empty anchor every root-reaching glob and every exact-match test
// below compares against, rather than Go's own "." sentinel.
func dirOf(p string) string {
	d := path.Dir(p)
	if d == "." {
		return ""
	}
	return d
}

// classifyGlob places ONE scope glob into a tier relative to fileDir/file.
// It does not itself decide whether the glob MATCHES file at all — the
// caller (obligationsCore) already gated matched claims through
// filter.Match; this only classifies proximity for a glob known to be
// relevant. A glob shape this repo's compiler never emits still gets a
// safe, defensible answer (never tier 0/1 by accident): the default is
// tier 3 (root), never a false promotion.
func classifyGlob(g, fileDir, file string) governingScope {
	anchor, exact := globAnchor(g)
	switch {
	case exact && g == file:
		return governingScope{tier: 0, depth: dirDepth(fileDir)}
	case exact:
		// A literal, wildcard-free glob that is NOT the edited file: it can
		// only have matched (per filter.Match's gitignore semantics) as a
		// bare component-anywhere name, which reaches arbitrarily deep —
		// treat it as root-tier rather than guessing a depth for it.
		return governingScope{tier: 3, depth: 0}
	case anchor == "":
		return governingScope{tier: 3, depth: 0}
	case anchor == fileDir:
		return governingScope{tier: 1, depth: dirDepth(anchor)}
	case fileDir == anchor || strings.HasPrefix(fileDir+"/", anchor+"/"):
		return governingScope{tier: 2, depth: dirDepth(anchor)}
	default:
		// The glob's literal anchor is not an ancestor of fileDir at all
		// (e.g. it matched via some other gitignore-style rule this
		// classifier does not model) — defensive root-tier, never a wrong
		// promotion.
		return governingScope{tier: 3, depth: 0}
	}
}

// dirDepth counts path segments ("" -> 0, "internal" -> 1,
// "internal/store" -> 2). Used only to compare two ancestor-tier anchors:
// a deeper (longer) anchor is a nearer ancestor of the edited file.
func dirDepth(dir string) int {
	if dir == "" {
		return 0
	}
	return strings.Count(dir, "/") + 1
}

// globAnchor extracts the literal directory a scope glob is anchored to
// (dir, false) or reports it as an exact/unanchored literal (dir, true) —
// the caller resolves "exact" against the edited file itself. "" as the
// anchor with exact=false means the glob is unanchored and reaches the
// repo root by construction (a bare "*", "**", "**/*.ext", or "**/name").
//
// The shapes this covers are exactly what this codebase's compiler emits
// (BP-32's scopeFloor, ContractScope's package/symbol contracts):
// "<dir>/**" and "<dir>/**/*.ext" (package/ancestor, anchored at <dir>),
// a bare literal path with no wildcard (an exact file, or — if it never
// equals the edited file — an unanchored bare name), and "**/…" or a bare
// "*"/"**" (root-reaching).
func globAnchor(g string) (dir string, exact bool) {
	g = strings.TrimSpace(g)
	g = strings.TrimPrefix(g, "./")
	if g == "*" || g == "**" || strings.HasPrefix(g, "**/") {
		return "", false
	}
	if i := strings.Index(g, "/**"); i >= 0 {
		return strings.TrimSuffix(g[:i], "/"), false
	}
	if !strings.ContainsAny(g, "*?[{") {
		if !strings.Contains(g, "/") {
			return "", false // bare name: gitignore-style, matches anywhere
		}
		return g, true // literal path — caller checks equality to the file
	}
	if strings.Contains(g, "/") {
		return path.Dir(g), false // e.g. "internal/foo/*.go" -> anchored at internal/foo
	}
	return "", false
}
