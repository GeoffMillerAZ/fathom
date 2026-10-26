package benchprep

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

func TestHopDistanceGoTypesCHA(t *testing.T) {
	root, err := filepath.Abs("testdata/hops")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, issue string
		distance    int
		bucket      string
	}{
		{"zero", "`Gold` fails", 0, ""},
		{"one", "`Middle` fails", 1, ""},
		{"two", "`Entry` fails", 2, ""},
		{"qualified", "hops.Entry fails", 2, ""},
		{"full package", "example.test/hops.Entry fails", 2, ""},
		{"shortest named source", "`Entry` and `Middle`", 1, ""},
		{"unreachable", "`Unreachable` fails", -1, "no-graph-path"},
		{"direction is forward", "leaf.Entry fails", -1, "no-graph-path"},
		{"unnamed source", "no symbol is named", -1, "no-graph-path"},
		{"cycle", "`Cycle` fails", 3, ""},
		{"interface dispatch", "`Dispatch` fails", 2, ""},
		{"receiver qualified", "Worker.Run fails", 1, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HopDistance(context.Background(), HopRequest{
				Root: root, Issue: tt.issue,
				GoldFunctions: []FunctionRef{{Path: "leaf/leaf.go", Name: "Gold"}},
			})
			if err != nil || got.Distance != tt.distance || got.Bucket != tt.bucket || got.Oracle != "go/types+cha" {
				t.Fatalf("hop = %+v, %v", got, err)
			}
		})
	}
}

func TestHopDistanceMissingOracleAndTruth(t *testing.T) {
	for _, tt := range []struct {
		name    string
		req     HopRequest
		wantErr bool
	}{
		{"no oracle", HopRequest{}, false},
		{"bad Go root", HopRequest{Root: "testdata/absent", GoldFunctions: []FunctionRef{{Path: "leaf.go", Name: "Gold"}}}, true},
		{"no gold", HopRequest{Root: "testdata/hops"}, true},
		{"unknown gold", HopRequest{Root: "testdata/hops", GoldFunctions: []FunctionRef{{Path: "leaf/leaf.go", Name: "Absent"}}}, true},
		{"empty SCIP", HopRequest{SCIP: &scippb.Index{}, GoldFunctions: []FunctionRef{{Path: "leaf.go", Name: "Gold"}}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HopDistance(context.Background(), tt.req)
			if (err != nil) != tt.wantErr || got.Bucket != "no-oracle" || got.Distance != -1 {
				t.Fatalf("hop = %+v, %v", got, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := HopDistance(ctx, HopRequest{Root: "testdata/hops", GoldFunctions: []FunctionRef{{Path: "leaf/leaf.go", Name: "Gold"}}})
	if err == nil || got.Bucket != "no-oracle" {
		t.Fatalf("cancellation = %+v, %v", got, err)
	}
}

func TestHopDistanceSCIPReferences(t *testing.T) {
	index := hopSCIPFixture()
	for _, tt := range []struct {
		issue    string
		distance int
		bucket   string
	}{
		{"`Gold`", 0, ""}, {"`Middle`", 1, ""}, {"`Entry`", 2, ""},
		{"`Entry` `Middle`", 1, ""}, {"`Unreachable`", -1, "no-graph-path"},
		{"`Boundary`", -1, "no-graph-path"}, {"Type.Entry", 2, ""},
	} {
		t.Run(tt.issue, func(t *testing.T) {
			var prior HopResult
			for range 2 {
				got, err := HopDistance(context.Background(), HopRequest{
					Root: "testdata/absent", SCIP: index, Issue: tt.issue,
					GoldFunctions: []FunctionRef{{Path: "subject.ts", Name: "Gold"}},
				})
				if err != nil || got.Distance != tt.distance || got.Bucket != tt.bucket || got.Oracle != "scip-references" {
					t.Fatalf("SCIP hop = %+v, %v", got, err)
				}
				if prior.Oracle != "" && !reflect.DeepEqual(got, prior) {
					t.Fatal("SCIP hop is nondeterministic")
				}
				prior = got
			}
		})
	}
	// An edge in the enclosing class cannot shorten Entry -> Middle -> Gold.
	// References exactly at a half-open body end cannot connect Boundary.
	// DisplayName is absent, as it is in many real SCIP producers.
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	before := string(data)
	_, _ = HopDistance(context.Background(), HopRequest{SCIP: index, Issue: "`Entry`", GoldFunctions: []FunctionRef{{Path: "subject.ts", Name: "Gold"}}})
	data, _ = json.Marshal(index)
	if string(data) != before || strings.Contains(before, "display_name") {
		t.Fatal("SCIP input mutated or fixture has display names")
	}
}

func TestHopDistanceSCIPRefusesMissingRanges(t *testing.T) {
	index := hopSCIPFixture()
	index.Documents[0].Occurrences[1].EnclosingRange = nil
	got, err := HopDistance(context.Background(), HopRequest{
		SCIP: index, Issue: "`Entry`", GoldFunctions: []FunctionRef{{Path: "subject.ts", Name: "Gold"}},
	})
	if err == nil || got.Bucket != "no-oracle" {
		t.Fatalf("incomplete SCIP = %+v, %v", got, err)
	}
}

func TestHopDistanceNamesResolvedAgainstTruth(t *testing.T) {
	// Unlike the stratum's code-like query, graph seeds include any exact
	// symbol named in prose. The oracle disambiguates them, not casing.
	got, err := HopDistance(context.Background(), HopRequest{
		Root: "testdata/hops", Issue: "Middle fails",
		GoldFunctions: []FunctionRef{{Path: "leaf/leaf.go", Name: "Gold"}},
	})
	if err != nil || got.Distance != 1 || got.Bucket != "" {
		t.Fatalf("unquoted named symbol = %+v, %v", got, err)
	}
}

func TestHopDistanceGoIgnoresExternalPackageDriver(t *testing.T) {
	t.Setenv("GOPACKAGESDRIVER", "benchprep-driver-must-not-run")
	got, err := HopDistance(context.Background(), HopRequest{
		Root: "testdata/hops", Issue: "`Entry`",
		GoldFunctions: []FunctionRef{{Path: "leaf/leaf.go", Name: "Gold"}},
	})
	if err != nil || got.Distance != 2 || got.Bucket != "" {
		t.Fatalf("oracle must use the local Go toolchain = %+v, %v", got, err)
	}
}

func TestHopDistanceSCIPSkipsParameterDefinitions(t *testing.T) {
	index := hopSCIPFixture()
	index.Documents[0].Occurrences = append(index.Documents[0].Occurrences,
		&scippb.Occurrence{Symbol: "scip-typescript npm fixture 1.0.0 subject/Middle().(arg)", SymbolRoles: int32(scippb.SymbolRole_Definition), Range: []int32{6, 6, 9}},
	)
	got, err := HopDistance(context.Background(), HopRequest{
		SCIP: index, Issue: "`Entry`", GoldFunctions: []FunctionRef{{Path: "subject.ts", Name: "Gold"}},
	})
	if err != nil || got.Distance != 2 || got.Bucket != "" {
		t.Fatalf("SCIP parameter without a body = %+v, %v", got, err)
	}
}

func hopSCIPFixture() *scippb.Index {
	const prefix = "scip-typescript npm fixture 1.0.0 subject/"
	def := func(name string, line, end int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: prefix + name, SymbolRoles: int32(scippb.SymbolRole_Definition), Range: []int32{line, 0, 5}, EnclosingRange: []int32{line, 0, end, 0}}
	}
	ref := func(name string, line int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: prefix + name, Range: []int32{line, 1, 4}}
	}
	return &scippb.Index{Documents: []*scippb.Document{{RelativePath: "subject.ts", Occurrences: []*scippb.Occurrence{
		def("Type#", 0, 20),
		def("Type#Entry().", 1, 5), ref("Middle().", 2),
		def("Middle().", 6, 9), ref("Gold().", 7),
		def("Gold().", 10, 12),
		def("Unreachable().", 13, 15),
		def("Boundary().", 16, 18), ref("Gold().", 18),
		ref("Gold().", 19), // class-owned, not Entry-owned
	}}}}
}
