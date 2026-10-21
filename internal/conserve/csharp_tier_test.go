package conserve

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestCSharpTierUsesActualEdgesWithoutPromotingUnresolved(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edges []core.Edge
		want  string
	}{
		{"none", nil, TierSyntax},
		{"direct SCIP", []core.Edge{{FromFile: "Game/Worker.cs", Kind: core.EdgeDirect}}, TierSCIP},
		{"file resolved", []core.Edge{{FromFile: "Game/Worker.cs", Kind: core.EdgeFileResolved}}, TierFileResolved},
		{"unresolved", []core.Edge{{FromFile: "Game/Worker.cs", Kind: core.EdgeUnresolved}}, TierSyntax},
		{"another language", []core.Edge{{FromFile: "Tools/generate.py", Kind: core.EdgeDirect}}, TierSyntax},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := store.NewMemoryStore()
			defer st.Close()
			for i := range tc.edges {
				tc.edges[i].FromService = "svc"
				tc.edges[i].FromSymbol = "svc::Game.Worker.Run"
				tc.edges[i].ToSymbol = "svc::Game.Worker.Accept"
			}
			if err := st.PutTypedEdges(context.Background(), "svc", "scip:unity.scip::Game", tc.edges); err != nil {
				t.Fatal(err)
			}
			got := TiersByLanguage(context.Background(), Input{Store: st, Service: "svc", ChangedFiles: []string{"Game/Worker.cs"}})
			if len(got) != 1 || got[0].Language != "C#" || got[0].Tier != tc.want {
				t.Fatalf("tier=%v,want C#/%s", got, tc.want)
			}
		})
	}
	got := TiersByLanguage(context.Background(), Input{Service: "svc", ChangedFiles: []string{"Game/Worker.cs"}})
	if len(got) != 1 || got[0].Tier != TierSyntax {
		t.Fatalf("missing store promoted: %v", got)
	}
}
