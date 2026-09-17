package semantic

import "testing"

// TestRankByGoverningScope_Tiers pins the four proximity tiers VB-8 names:
// exact file > package > parent dir (deeper wins) > root.
func TestRankByGoverningScope_Tiers(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope []string
		path  string
		want  governingScope
	}{
		{
			"exact file",
			[]string{"internal/extractor/goextract.go"},
			"internal/extractor/goextract.go",
			governingScope{tier: 0, depth: 2},
		},
		{
			"package: glob anchored at the file's own directory",
			[]string{"internal/extractor/**"},
			"internal/extractor/goextract.go",
			governingScope{tier: 1, depth: 2},
		},
		{
			"parent dir: glob anchored above the file's directory",
			[]string{"internal/**"},
			"internal/extractor/goextract.go",
			governingScope{tier: 2, depth: 1},
		},
		{
			"root: bare double-star",
			[]string{"**"},
			"internal/extractor/goextract.go",
			governingScope{tier: 3, depth: 0},
		},
		{
			"root: doublestar-prefixed extension glob",
			[]string{"**/*.go"},
			"internal/extractor/goextract.go",
			governingScope{tier: 3, depth: 0},
		},
		{
			"root: doublestar-prefixed bare name",
			[]string{"**/schema.cue"},
			"schema/plan/schema.cue",
			governingScope{tier: 3, depth: 0},
		},
		{
			"parent dir: a named ancestor above the file's own directory",
			[]string{"internal/**"},
			"internal/mcp/semantic_tools.go",
			governingScope{tier: 2, depth: 1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := rankByGoverningScope(tc.scope, tc.path)
			if got != tc.want {
				t.Errorf("rankByGoverningScope(%v, %q) = %+v, want %+v", tc.scope, tc.path, got, tc.want)
			}
		})
	}
}

// TestRankByGoverningScope_BestOfMultipleGlobs: a claim's scope is a LIST
// of globs; the claim earns its NEAREST match, not its farthest.
func TestRankByGoverningScope_BestOfMultipleGlobs(t *testing.T) {
	scope := []string{"**", "internal/**", "internal/mcp/**"}
	got := rankByGoverningScope(scope, "internal/mcp/tools.go")
	want := governingScope{tier: 1, depth: 2} // internal/mcp/** IS the file's own package dir
	if got != want {
		t.Fatalf("rankByGoverningScope = %+v, want %+v (the package-level glob, not the root or ancestor one)", got, want)
	}
}

// TestGoverningScope_ClosestThan pins the ordering closerThan implements:
// lower tier always wins; a deeper ancestor wins inside tier 2.
func TestGoverningScope_ClosestThan(t *testing.T) {
	exact := governingScope{tier: 0, depth: 5}
	pkg := governingScope{tier: 1, depth: 5}
	deepAncestor := governingScope{tier: 2, depth: 3}
	shallowAncestor := governingScope{tier: 2, depth: 1}
	root := governingScope{tier: 3, depth: 0}

	if !exact.closerThan(pkg) {
		t.Error("exact file must rank closer than package")
	}
	if !pkg.closerThan(deepAncestor) {
		t.Error("package must rank closer than any ancestor directory")
	}
	if !deepAncestor.closerThan(shallowAncestor) {
		t.Error("a deeper (nearer) ancestor must rank closer than a shallower one")
	}
	if !shallowAncestor.closerThan(root) {
		t.Error("any named ancestor must rank closer than root")
	}
}

// TestRankByGoverningScope_LengthDoesNotDecideTier is VB-8's actual bug
// fix, pinned directly: a SHORT, package-scoped glob must outrank a
// LONGER, root-reaching one — the byte-length heuristic this replaces got
// this backwards.
func TestRankByGoverningScope_LengthDoesNotDecideTier(t *testing.T) {
	shortButClose := []string{"cmd/**"}   // 6 bytes
	longButRoot := []string{"**/main.go"} // 10 bytes — textually longer, root-reaching

	closeScope := rankByGoverningScope(shortButClose, "cmd/main.go")
	rootScope := rankByGoverningScope(longButRoot, "cmd/main.go")

	if len(shortButClose[0]) >= len(longButRoot[0]) {
		t.Fatalf("test setup: %q must be shorter than %q", shortButClose[0], longButRoot[0])
	}
	if !closeScope.closerThan(rootScope) {
		t.Errorf("the shorter, package-scoped glob must rank closer despite its shorter byte length: close=%+v root=%+v", closeScope, rootScope)
	}
}

// TestCrossTierCloser_PromotesStrongFartherOverBlankNearer (wave O3, item
// 1) pins the one case crossTierCloser adds over plain closerThan: a
// farther-tier candidate that is STRONG leapfrogs a nearer-tier candidate
// that is not itself strong (here, a total lexical blank).
func TestCrossTierCloser_PromotesStrongFartherOverBlankNearer(t *testing.T) {
	nearerBlank := governingScope{tier: 1, depth: 2} // package tier, zero relevance
	fartherStrong := governingScope{tier: 3, depth: 0}
	if crossTierCloser(nearerBlank, fartherStrong, false /*aStrong*/, true /*bStrong*/) {
		t.Error("the nearer-but-blank candidate must not sort ahead of a farther-but-strong one")
	}
	if !crossTierCloser(fartherStrong, nearerBlank, true, false) {
		t.Error("the farther-but-strong candidate must sort ahead of a nearer-but-blank one")
	}
}

// TestCrossTierCloser_AncestorOnlyHitOnNearerDoesNotBlockPromotion (wave
// O4, item 1; ADR-037 addendum) pins the narrowed bar: a nearer candidate
// whose ONLY relevance is an ancestor-directory (or extension) token —
// never classified "strong" by relevanceStrength — no longer keeps its
// tier against a farther, strong candidate. H1-T02 (internal/core/model.go
// vs. internal/CLAUDE.md's other sections, each carrying only a weak hit
// on the bare token "internal") is exactly this shape.
func TestCrossTierCloser_AncestorOnlyHitOnNearerDoesNotBlockPromotion(t *testing.T) {
	nearerWeakHit := governingScope{tier: 1, depth: 2} // ancestor-only hit, not strong
	fartherStrong := governingScope{tier: 3, depth: 0}
	if !crossTierCloser(fartherStrong, nearerWeakHit, true, false) {
		t.Error("a nearer candidate with only a weak (non-strong) hit must be leapfrogged by a farther, strong one")
	}
	// Order-independent: the promotion holds in the reverse comparison too.
	if crossTierCloser(nearerWeakHit, fartherStrong, false, true) {
		t.Error("the nearer-but-weak candidate must not sort ahead of the farther-but-strong one")
	}
}

// TestCrossTierCloser_StrongHitOnNearerBlocksPromotion: the bar
// crossTierCloser now uses — a nearer candidate that IS itself strong
// (a hit on its own package path, stem, or a caller-resolved
// symbol/behaviour token) keeps its tier; only an ancestor-only hit fails
// to block, never a real one.
func TestCrossTierCloser_StrongHitOnNearerBlocksPromotion(t *testing.T) {
	nearerStrong := governingScope{tier: 1, depth: 2}
	fartherStrong := governingScope{tier: 3, depth: 0}
	if crossTierCloser(fartherStrong, nearerStrong, true, true) {
		t.Error("a nearer candidate that is itself strong must keep its tier — no promotion")
	}
	if !crossTierCloser(nearerStrong, fartherStrong, true, true) {
		t.Error("the nearer, strong candidate must sort ahead of the farther one")
	}
}

// TestCrossTierCloser_SameTierFallsThroughToCloserThan: when both
// candidates share a tier (the usual case — most comparisons never
// involve two different tiers at all), crossTierCloser must behave
// identically to plain closerThan, regardless of strength.
func TestCrossTierCloser_SameTierFallsThroughToCloserThan(t *testing.T) {
	deeper := governingScope{tier: 2, depth: 3}
	shallower := governingScope{tier: 2, depth: 1}
	if !crossTierCloser(deeper, shallower, false, true) {
		t.Error("same-tier comparisons must ignore strength entirely and fall through to closerThan (deeper wins)")
	}
}

// TestCrossTierCloser_NeitherStrong_ProximityWins: promotion requires the
// FARTHER side to be strong; when neither side is, ordinary proximity
// decides regardless of any ancestor-only relevance either carries.
func TestCrossTierCloser_NeitherStrong_ProximityWins(t *testing.T) {
	nearer := governingScope{tier: 1, depth: 2}
	farther := governingScope{tier: 3, depth: 0}
	if !crossTierCloser(nearer, farther, false, false) {
		t.Error("nearer must win over a farther candidate that is not strong")
	}
}

func TestGoverningScope_Name(t *testing.T) {
	for _, tc := range []struct {
		g    governingScope
		want string
	}{
		{governingScope{tier: 0}, "exact_file"},
		{governingScope{tier: 1}, "package"},
		{governingScope{tier: 2}, "parent_dir"},
		{governingScope{tier: 3}, "root"},
		{noGoverningScope, "unscoped"},
	} {
		if got := tc.g.name(); got != tc.want {
			t.Errorf("governingScope{tier:%d}.name() = %q, want %q", tc.g.tier, got, tc.want)
		}
	}
}
