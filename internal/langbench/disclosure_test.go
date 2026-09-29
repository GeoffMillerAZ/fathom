package langbench

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func connectorClaims(ext string) bool {
	return connector.LanguageForPath("x"+ext) != core.LangUnclassified
}

// TestUnsupportedTableMatchesQueue pins LB-F25: the connector's
// unsupported-language table is exactly the one rendered from the queue —
// every queued language not dispatched (unregistered, or refused by its
// plan row, §14.2) by its unambiguous extensions — so a language discloses
// until its dialect is dispatched and never after.
func TestUnsupportedTableMatchesQueue(t *testing.T) {
	in, err := TrackerInputFromRepo("../..")
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderDisclosure(in.Queue, in.Dispatched(), connectorClaims)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("../..", DisclosurePath))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s differs from the queue; regenerate it: go run ./cmd/indexbench lang-disclosure > %s", DisclosurePath, DisclosurePath)
	}
	table := DisclosureTable(in.Queue, in.Dispatched(), connectorClaims)
	for _, e := range in.Queue {
		if e.Kind != "language" {
			continue
		}
		_, registered := in.Dispatched()[e.ID]
		plain, _ := nameExtensions(e.Name)
		for _, x := range plain {
			lang, ok := connector.UnsupportedLanguageForPath("f" + x)
			switch {
			case registered && ok && lang == e.ID:
				t.Errorf("%s is registered but %s still discloses it", e.ID, x)
			case !registered && table[x] == e.ID && (!ok || lang != e.ID):
				t.Errorf("UnsupportedLanguageForPath(f%s) = %q,%v, want %s", x, lang, ok, e.ID)
			}
		}
	}
	for _, amb := range []string{".m", ".h", ".v", ".pl", ".fs", ".pp", ".inc", ".cls", ".t"} {
		if id, ok := table[amb]; ok {
			t.Errorf("ambiguous extension %s disclosed as %s (§5)", amb, id)
		}
	}
	if len(table) < 30 {
		t.Errorf("only %d extensions disclosed; the queue names many more unregistered languages", len(table))
	}
}

func TestNameExtensions(t *testing.T) {
	cases := []struct {
		name       string
		plain, all []string
	}{
		{"Perl (.pl .pm .t by sniff)", nil, []string{".pl", ".pm", ".t"}},
		{"F# (.fs .fsi .fsx, .fs by sniff)", []string{".fs", ".fsi", ".fsx"}, []string{".fs", ".fsi", ".fsx", ".fs"}},
		{"SQL (.sql; schema and migrations)", []string{".sql"}, []string{".sql"}},
		{"Terraform / OpenTofu (.tf .tfvars .tf.json)", []string{".tf", ".tfvars"}, []string{".tf", ".tfvars"}},
		{"Makefile / GNU make (Makefile, *.mk, GNUmakefile)", []string{".mk"}, []string{".mk"}},
		{"Mojo (.mojo, .🔥)", []string{".mojo"}, []string{".mojo"}},
		{"Kubernetes manifests (incl. CRDs)", nil, nil},
	}
	for _, c := range cases {
		plain, all := nameExtensions(c.name)
		if !slices.Equal(plain, c.plain) || !slices.Equal(all, c.all) {
			t.Errorf("nameExtensions(%q) = %v, %v; want %v, %v", c.name, plain, all, c.plain, c.all)
		}
	}
}
