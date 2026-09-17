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
// that is a total lexical blank (relevance == 0).
func TestCrossTierCloser_PromotesStrongFartherOverBlankNearer(t *testing.T) {
	nearerBlank := governingScope{tier: 1, depth: 2} // package tier, zero relevance
	fartherStrong := governingScope{tier: 3, depth: 0}
	if crossTierCloser(nearerBlank, fartherStrong, 0 /*aRel*/, 12.5 /*bRel*/, false /*aStrong*/, true /*bStrong*/) {
		t.Error("the nearer-but-blank candidate must not sort ahead of a farther-but-strong one")
	}
	if !crossTierCloser(fartherStrong, nearerBlank, 12.5, 0, true, false) {
		t.Error("the farther-but-strong candidate must sort ahead of a nearer-but-blank one")
	}
}

// TestCrossTierCloser_AnyHitOnNearerBlocksPromotion pins the cap: the
// moment the nearer candidate has ANY relevance at all — however weak
// (an ancestor-only hit, never classified as strong) — proximity wins
// outright and the farther, strong candidate does NOT get promoted.
func TestCrossTierCloser_AnyHitOnNearerBlocksPromotion(t *testing.T) {
	nearerWeakHit := governingScope{tier: 1, depth: 2}
	fartherStrong := governingScope{tier: 3, depth: 0}
	if crossTierCloser(fartherStrong, nearerWeakHit, 9.5, 1.5 /* nearer has a weak but nonzero hit */, true, false) {
		t.Error("a nearer candidate with any hit (even weak) must keep its tier — no promotion")
	}
	// Order-independent: proximity still wins in the reverse comparison too.
	if !crossTierCloser(nearerWeakHit, fartherStrong, 1.5, 9.5, false, true) {
		t.Error("the nearer candidate (with any hit) must sort ahead of the farther one")
	}
}

// TestCrossTierCloser_SameTierFallsThroughToCloserThan: when both
// candidates share a tier (the usual case — most comparisons never
// involve two different tiers at all), crossTierCloser must behave
// identically to plain closerThan, regardless of relevance/strength.
func TestCrossTierCloser_SameTierFallsThroughToCloserThan(t *testing.T) {
	deeper := governingScope{tier: 2, depth: 3}
	shallower := governingScope{tier: 2, depth: 1}
	if !crossTierCloser(deeper, shallower, 0, 99, false, true) {
		t.Error("same-tier comparisons must ignore relevance entirely and fall through to closerThan (deeper wins)")
	}
}

// TestCrossTierCloser_BothSidesHaveHits_ProximityWins: promotion requires
// the nearer side to be a TOTAL blank; when both sides carry relevance
// (even if the farther side is much stronger), ordinary proximity decides.
func TestCrossTierCloser_BothSidesHaveHits_ProximityWins(t *testing.T) {
	nearer := governingScope{tier: 1, depth: 2}
	farther := governingScope{tier: 3, depth: 0}
	if !crossTierCloser(nearer, farther, 1.0, 50.0, false, true) {
		t.Error("nearer with nonzero relevance must win over a farther, stronger candidate")
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
