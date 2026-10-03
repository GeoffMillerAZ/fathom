package extractor_test

// cuedoc_oracle_test.go — LB-Q-cue: the CUE dialect against the cue binary's
// own answer for the fixture module (dialects/cue/testdata/module.oracle.json,
// written by calibration/oracles/cue: `cue lsp` document symbols and
// definitions, the module file's fields by `cue export`). The harness's §1.4
// comparison, done in miniature: definitions by (file, bare name, line), the
// kinds the dialect maps (definition, field), and call pairs by (file, bare
// caller, bare callee).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

func TestCUEDialectAgreesWithTheCueBinary(t *testing.T) {
	dir := filepath.Join("dialects", "cue", "testdata")
	raw, err := os.ReadFile(filepath.Join(dir, "module.oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Tool, Version string
		Definitions   []struct {
			Path, Name, Kind string
			Line, End        int
		}
		Calls []struct{ Path, From, To string }
	}
	if err := json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Version != "v0.16.1" {
		t.Fatalf("oracle version %q: the fixture's answer is cue v0.16.1's", oracle.Version)
	}
	files := map[string]bool{}
	want := map[string]bool{}
	for _, d := range oracle.Definitions {
		files[d.Path] = true
		if d.Kind == "definition" || d.Kind == "field" {
			want[d.Path+"|"+d.Name+"|"+itoa(d.Line)+"-"+itoa(d.End)+"|"+d.Kind] = true
		}
	}
	wantCalls := map[string]bool{}
	for _, c := range oracle.Calls {
		wantCalls[c.Path+"|"+langbench.BareName(c.From)+"|"+langbench.BareName(c.To)] = true
	}

	x := extractor.NewCUEExtractor()
	got := map[string]bool{}
	gotCalls := map[string]bool{}
	for f := range files {
		src, err := os.ReadFile(filepath.Join(dir, "module", filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		nodes, err := x.Extract("svc", f, src)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range nodes {
			kind := map[string]string{"CUEDefinition": "definition", "CUEField": "field"}[string(n.Kind)]
			if kind == "" {
				continue
			}
			got[f+"|"+langbench.BareName(n.Name)+"|"+itoa(n.Line)+"-"+itoa(n.EndLine)+"|"+kind] = true
			for _, c := range n.Calls {
				gotCalls[f+"|"+langbench.BareName(n.Name)+"|"+langbench.BareName(c)] = true
			}
		}
	}
	diff := func(what string, a, b map[string]bool) {
		var only []string
		for k := range a {
			if !b[k] {
				only = append(only, k)
			}
		}
		sort.Strings(only)
		if len(only) > 0 {
			t.Errorf("%s:\n  %s", what, strings.Join(only, "\n  "))
		}
	}
	diff("definitions the cue binary declares and the dialect misses", want, got)
	diff("definitions the dialect emits and the cue binary does not declare", got, want)
	diff("references the cue binary resolves and the dialect misses", wantCalls, gotCalls)
	diff("references the dialect records and the cue binary does not resolve", gotCalls, wantCalls)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
