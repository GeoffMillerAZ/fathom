package conserve

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestTiersByLanguage pins CV-38 (ADR-044 §5): Go/CUE are always "typed";
// a scip-eligible language reports "scip" when the store holds a real
// typed edge from one of its files, "file-resolved" when it holds only
// core.EdgeFileResolved edges, "syntax" otherwise; every other language is
// unconditionally "syntax"; a language untouched by ChangedFiles is
// absent from the result entirely.
func TestTiersByLanguage(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	defer st.Close()

	const svc = "svc"
	if err := st.PutTypedEdges(ctx, svc, "widgets", []core.Edge{
		{FromService: svc, FromFile: "widgets/a.ts", FromSymbol: "svc::widgets.A", ToSymbol: "svc::widgets.B", Kind: core.EdgeDirect},
		{FromService: svc, FromFile: "widgets/b.py", FromSymbol: "svc::widgets.C", ToSymbol: "svc::widgets.D", Kind: core.EdgeFileResolved},
	}); err != nil {
		t.Fatalf("PutTypedEdges: %v", err)
	}

	in := Input{
		Service: svc,
		Store:   st,
		ChangedFiles: []string{
			"main.go",              // Go -> typed
			"schema/config.cue",    // CUE -> typed
			"widgets/a.ts",         // TypeScript -> has a real typed edge -> scip
			"widgets/b.py",         // Python -> only file-resolved edges -> file-resolved
			"widgets/untouched.js", // JavaScript -> no typed edges at all -> syntax
			"scripts/build.rs",     // Rust -> always syntax
			"README.md",            // no recognized language -> excluded entirely
		},
	}

	got := TiersByLanguage(ctx, in)
	byLang := map[string]string{}
	for _, ti := range got {
		byLang[ti.Language] = ti.Tier
	}

	want := map[string]string{
		"Go":         TierTyped,
		"CUE":        TierTyped,
		"TypeScript": TierSCIP,
		"Python":     TierFileResolved,
		"JavaScript": TierSyntax,
		"Rust":       TierSyntax,
	}
	for lang, tier := range want {
		if got := byLang[lang]; got != tier {
			t.Errorf("tier[%s] = %q, want %q (full: %+v)", lang, got, tier, got)
		}
	}
	if _, ok := byLang["Markdown"]; ok {
		t.Errorf("README.md should not have produced a language entry: %+v", got)
	}
	if len(got) != len(want) {
		t.Errorf("TiersByLanguage returned %d entries, want %d: %+v", len(got), len(want), got)
	}
}

// TestTiersByLanguage_NoStoreDegradesToSyntax pins the fail-open contract:
// a scip-eligible language with no store wired reports "syntax", never an
// error and never a false "scip"/"file-resolved".
func TestTiersByLanguage_NoStoreDegradesToSyntax(t *testing.T) {
	got := TiersByLanguage(context.Background(), Input{Service: "svc", ChangedFiles: []string{"widgets/a.ts"}})
	if len(got) != 1 || got[0].Language != "TypeScript" || got[0].Tier != TierSyntax {
		t.Errorf("TiersByLanguage with no store = %+v, want [{TypeScript syntax}]", got)
	}
}

// TestTiersByLanguage_NoChangedFilesIsEmpty pins the degenerate case.
func TestTiersByLanguage_NoChangedFilesIsEmpty(t *testing.T) {
	got := TiersByLanguage(context.Background(), Input{Service: "svc"})
	if len(got) != 0 {
		t.Errorf("TiersByLanguage with no changed files = %+v, want empty", got)
	}
}
