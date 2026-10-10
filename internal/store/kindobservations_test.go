package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestObservationsOfKinds (EW-57): both backends answer the service's rows
// of the named kinds only, every column, ordered by source, line, symbol;
// another service's rows and other kinds never answer.
func TestObservationsOfKinds(t *testing.T) {
	ctx := context.Background()
	sq, err := NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sq.Close() })
	at := time.Unix(1700000000, 0).UTC()
	obs := []core.Observation{
		{ServiceID: "a", SourceRef: "tf/main.tf", SourceKind: core.KindTerraformResource, EvidenceTier: core.TierVerified, Symbol: "aws_lambda_function.api", Package: "tf/main", Line: 9, EndLine: 14, IngestedAt: at},
		{ServiceID: "a", SourceRef: "infra/app.yaml", SourceKind: "CFNResource", EvidenceTier: core.TierVerified, Symbol: "Orders", Package: "infra/app", Claim: "AWS::Serverless::Function", Line: 3, IngestedAt: at},
		{ServiceID: "a", SourceRef: "infra/app.yaml", SourceKind: "CFNTemplate", EvidenceTier: core.TierVerified, Symbol: "app.yaml", Line: 1, IngestedAt: at},
		{ServiceID: "a", SourceRef: "tf/main.tf", SourceKind: core.KindTerraformModule, EvidenceTier: core.TierVerified, Symbol: "module.fn", Claim: `module "fn" { source = "./lambda" }`, Line: 2, IngestedAt: at},
		{ServiceID: "a", SourceRef: "src/app.py", SourceKind: core.KindPyFunc, EvidenceTier: core.TierVerified, Symbol: "handler", Line: 1, IngestedAt: at},
		{ServiceID: "b", SourceRef: "tf/main.tf", SourceKind: core.KindTerraformResource, EvidenceTier: core.TierVerified, Symbol: "aws_lambda_function.other", Line: 1, IngestedAt: at},
	}
	kinds := []core.SourceKind{"CFNResource", core.KindTerraformResource, core.KindTerraformModule}
	want := []core.Observation{obs[1], obs[3], obs[0]}
	for name, st := range map[string]interface {
		Store
		KindObservationsStore
	}{"sqlite": sq, "memory": NewMemoryStore()} {
		t.Run(name, func(t *testing.T) {
			if err := st.PutObservations(ctx, obs); err != nil {
				t.Fatal(err)
			}
			got, err := st.ObservationsOfKinds(ctx, "a", kinds)
			if err != nil {
				t.Fatal(err)
			}
			for i := range got {
				got[i].IngestedAt = got[i].IngestedAt.UTC()
				got[i].Calls = nil
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("ObservationsOfKinds\n got: %+v\nwant: %+v", got, want)
			}
			if none, err := st.ObservationsOfKinds(ctx, "a", nil); err != nil || len(none) != 0 {
				t.Fatalf("no kinds: %v %v", none, err)
			}
			if none, err := st.ObservationsOfKinds(ctx, "c", kinds); err != nil || len(none) != 0 {
				t.Fatalf("a service with no rows: %v %v", none, err)
			}
		})
	}
}
