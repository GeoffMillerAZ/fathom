package extractor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type azurePipelineGolden struct {
	Name, Kind string
	Line, End  int
	Calls      []string
}

func TestAzurePipelinesDialectGoldens(t *testing.T) {
	x, err := NewDocDialectExtractor("azure-pipelines")
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("dialects/azure-pipelines/testdata/*.yml")
	if err != nil || len(files) != 3 {
		t.Fatalf("fixtures %v %v", files, err)
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			src, _ := os.ReadFile(f)
			raw, _ := os.ReadFile(f + ".golden.json")
			var want []azurePipelineGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			r, err := x.ExtractDoc("svc", "azure-pipelines.yml", src)
			if err != nil {
				t.Fatal(err)
			}
			got := []azurePipelineGolden{}
			for _, n := range r.Nodes {
				got = append(got, azurePipelineGolden{n.Name, string(n.Kind), n.Line, n.EndLine, n.Calls})
			}
			sort.Slice(got, func(i, j int) bool { return got[i].Line < got[j].Line })
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v want %+v", got, want)
			}
			again, _ := x.ExtractDoc("svc", "azure-pipelines.yml", src)
			if !reflect.DeepEqual(r, again) {
				t.Fatal("nondeterministic")
			}
		})
	}
}
