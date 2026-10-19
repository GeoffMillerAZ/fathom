package langbench

import (
	"encoding/json"
	"testing"
)

func TestParityRefusalRetainsOriginalFloorReadings(t *testing.T) {
	inc := baseReading("language")
	inc.Metrics = l1Metrics(1)
	inc.MetricTiers = tiers("A", inc.Metrics)
	inc.Finalize()
	dial := inc
	parseErrors := 1
	dial.Tenancy = Tenancy{ThroughputMBs: 0.5, ThroughputBasis: ThroughputBasisCPU,
		WallThroughputMBs: 0.25, MaxFileMS: 2041, PeakModulePages: 1100,
		Faults: 1, FaultingFiles: []string{"src/b.lang"}, FilesWithError: &parseErrors}
	dial.Subjects = []Subject{
		{Name: "a", Files: 2, Bytes: 4096, Tenancy: &SubjectTenancy{ExtractMS: 9, ExtractCPUMS: 7.75}},
		{Name: "b", Files: 1, Bytes: 1024, Tenancy: &SubjectTenancy{ExtractMS: 3, ExtractCPUMS: 2.25}},
	}
	dial.Finalize()
	p := ComparePortParity(inc, dial, "old", "L4", nil, nil, nil)
	if p.Merge || p.DialectLevel != LevelNone {
		t.Fatalf("failed original floors must still refuse: %+v", p)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Incumbent *struct {
			Tenancy Tenancy `json:"tenancy"`
		} `json:"incumbent_floor"`
		Dialect *struct {
			Tenancy    Tenancy `json:"tenancy"`
			Files      int     `json:"files"`
			Bytes      int64   `json:"bytes"`
			ExtractMS  float64 `json:"extract_ms"`
			ExtractCPU float64 `json:"extract_cpu_ms"`
		} `json:"dialect_floor"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Incumbent == nil || got.Dialect == nil {
		t.Fatalf("refusal omitted the measured floor inputs: %s", data)
	}
	x := got.Dialect
	if x.Tenancy.ThroughputMBs != 0.5 || x.Tenancy.ThroughputBasis != ThroughputBasisCPU ||
		x.Tenancy.MaxFileMS != 2041 || x.Tenancy.PeakModulePages != 1100 ||
		x.Tenancy.Faults != 1 || x.Files != 3 || x.Bytes != 5120 ||
		x.ExtractMS != 12 || x.ExtractCPU != 10 || got.Incumbent.Tenancy.ThroughputMBs != 4 {
		t.Fatalf("report changed or omitted original readings: %s", data)
	}
	// A retained report must not change if a caller later edits its input.
	dial.Tenancy.FaultingFiles[0] = "changed.lang"
	parseErrors = 2
	after, err := json.Marshal(p)
	if err != nil || string(after) != string(data) {
		t.Fatalf("floor snapshot aliases mutable input: %s (%v)", after, err)
	}
	// Below-target compatibility is still allowed; diagnostics cannot promote a
	// level or turn target reporting into a new gate.
	if pass := ComparePortParity(inc, inc, "old", "L4", nil, nil, nil); !pass.Merge || pass.DialectLevel != "L1" {
		t.Fatalf("diagnostics changed dispatch-at-parity: %+v", pass)
	}
}
