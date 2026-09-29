package extractor_test

// helmdialect_incumbent_test.go — design §13.2 step 15 for LB-Q-helm: the
// incumbents' own fixtures (helmextract_test.go) run through the helm dialect,
// which must emit every (SourceKind, Name) the incumbent does. Where the two
// differ on purpose the test says so: a dependency's signature is its version
// (the incumbent appended the repository), a template's line is its real line
// (the incumbent recorded 1), and the dialect signs a named template with its
// header.

import (
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func kindNames(nodes []core.SymbolNode) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, string(n.Kind)+" "+n.Name)
	}
	sort.Strings(out)
	return out
}

func TestHelmDialectMatchesIncumbents(t *testing.T) {
	d, err := extractor.NewTagsDocExtractor("helm")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, tc := range []struct {
		rel, src string
		inc      extractor.Extractor
	}{
		{"Chart.yaml", chartYAMLFixture, extractor.NewHelmChartExtractor()},
		{"values.yaml", valuesYAMLFixture, extractor.NewHelmValuesExtractor()},
		{"templates/_helpers.tpl", helpersTplFixture, extractor.NewHelmTemplateExtractor()},
	} {
		t.Run(tc.rel, func(t *testing.T) {
			inc, err := tc.inc.Extract("svc", tc.rel, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			got, err := d.Extract("svc", tc.rel, []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			// The dialect also emits what the incumbent does not (a
			// .Values accessor inside a template is a HelmValueRef, a
			// template call a HelmTemplateRef): compare only the kinds
			// the incumbent emits from this file type.
			incKinds := map[core.SourceKind]bool{}
			for _, n := range inc {
				incKinds[n.Kind] = true
			}
			var same []core.SymbolNode
			for _, n := range got {
				if incKinds[n.Kind] {
					same = append(same, n)
				}
			}
			if a, b := kindNames(inc), kindNames(same); !reflect.DeepEqual(a, b) {
				t.Fatalf("dialect emits %q, incumbent %q", b, a)
			}
			if tc.rel == "Chart.yaml" {
				for _, n := range same {
					if n.Kind == core.KindHelmChart && n.Signature != "1.2.3" {
						t.Errorf("chart signature %q, want the version", n.Signature)
					}
				}
			}
		})
	}
}

// TestHelmDialectPastSyntaxError: where the incumbent's text/template parse
// fails and it falls back to a regex at TierPreScan, the dialect's tree
// survives the same error and keeps its verified tier.
func TestHelmDialectPastSyntaxError(t *testing.T) {
	d, err := extractor.NewTagsDocExtractor("helm")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	broken := "{{- define \"a.ok\" -}}\nx\n{{- end }}\n{{- define \"a.open\" -}}\n{{ if .Values.x }}\n"
	inc, _ := extractor.NewHelmTemplateExtractor().Extract("svc", "templates/broken.tpl", []byte(broken))
	got, err := d.Extract("svc", "templates/broken.tpl", []byte(broken))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, n := range got {
		if n.Kind == core.KindHelmTemplate {
			have[n.Name] = true
			if n.Tier != core.TierVerified {
				t.Errorf("%s: tier %v, want verified", n.Name, n.Tier)
			}
		}
	}
	for _, n := range inc {
		if !have[n.Name] {
			t.Errorf("the incumbent's regex fallback found %q, the dialect did not", n.Name)
		}
	}
	if !have["a.ok"] {
		t.Errorf("the definition before the error must survive: %v", have)
	}
}
