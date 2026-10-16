package designconvert

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

// Opt-in, read-only purchase probe. Only hashes/counts/diagnostic codes are logged;
// no vendor source/converted bodies are logged. Supported constant-only output
// compiles/renders in private temporary files; vendor JS/projects never execute.
func TestFrozenConversionPilotLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_CONVERSION_KIT_ROOT to the existing absolute private app-kit checkout")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("APPKIT_PROBE_CONVERSION_KIT_ROOT must be absolute")
	}
	const marketing = "marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675"
	const application = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"
	const catalyst = "catalyst@94acb41796a8a97fdef44bf5b2d18f42919e43300cbca90077775f6b36dfa529"
	cases := []struct{ id, archive, member, sha string }{
		{"P01-hero", marketing, "html/sections/heroes/09-split-with-image.html", "356c19f1095d82529cb54014e3b94a67b7531489bb26b96b5fba8686d94beba9"},
		{"P02-pricing", marketing, "html/sections/pricing/05-single-price-with-details.html", "f545f273689b9a4312e5d0eeb66c15dc21d7d318e102e37c39b40f494e017e0f"},
		{"P03-faq", marketing, "html/sections/faq-sections/02-centered-accordion.html", "9114b0b981055638eb28fede17eff7bc49c91f40fc32ce758514bfdb7d19f937"},
		{"P04-table", application, "html/lists/tables/07-with-stacked-columns-on-mobile.html", "5d00e2755dcf060cceafa4c2dba4f15452719e54f5cd40646ab3155c89dc8288"},
		{"P05-form", application, "html/forms/input-groups/03-input-with-validation-error.html", "c35ba209333b7a4897c93a8c696b7fe306f4797625bc42906090e6edf864cc19"},
		{"P06-tabs", application, "html/navigation/tabs/01-tabs-with-underline.html", "01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31"},
		{"P07-menu", application, "html/elements/dropdowns/01-simple.html", "10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a"},
		{"P08-nested-dialog-menu", application, "html/overlays/drawers/07-user-profile-example.html", "1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415"},
		{"P09-mobile-nav", application, "html/navigation/navbars/04-simple-with-menu-button-on-left.html", "79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7"},
		{"P10-catalyst-settings", catalyst, "catalyst-ui-kit/demo/typescript/src/app/(app)/settings/page.tsx", "1ca7f4e14738e917390815dc38d48047588eb324090d337f5c718b9d2ec6fa21"},
		{"P11-matdash-analytics", "", "data/matdash/packages/main/src/views/dashboard/Dashboard2.tsx", "36c37212b4e5571367618d4e0d8fc9d1dfa35c75266228170f8eac071112b277"},
		{"P12-client-editor", "", "data/matdash/packages/main/src/views/forms/from-tiptap/TiptapEdit.tsx", "569ffc17dd27859dfe0df9a3fbf85e673e6552a547d142d840f37a30447fc9ac"},
	}
	manifest, err := os.ReadFile(filepath.Join(root, "docs/assessments/2026-09-13-conversion-pilot.md"))
	if err != nil {
		t.Fatal("frozen pilot manifest unavailable")
	}
	translated, unsupported, localClientSources, compiledRendered := 0, 0, 0, 0
	for _, test := range cases {
		t.Run(test.id, func(t *testing.T) {
			if !frozenPilotMatches(string(manifest), test.id, test.archive, test.member, test.sha) {
				t.Fatal("probe case-to-source binding differs from the frozen source manifest")
			}
			if test.archive == "" {
				body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(test.member)))
				if err != nil || digest(body) != test.sha {
					t.Fatal("local client source missing or changed; review the frozen case, never replace it silently")
				}
				localClientSources++
				t.Log("source_language=tsx disposition=source-native-or-reviewed-adapter verification=not-verified; no fabricated archive identity")
				return
			}
			file, err := designassets.ReadFile(filepath.Join(root, "data/design-assets"), test.archive, test.member)
			if err != nil || file.SHA256 != test.sha {
				t.Fatal("original verification failed; source/error bodies omitted")
			}
			body := []byte(file.Content)
			result, err := Translate(Source{ArchiveID: file.ArchiveID, Path: file.Path, SHA256: file.SHA256, Size: len(body)}, body)
			if err != nil {
				t.Fatal("translation failed operationally; source/error bodies omitted")
			}
			if err := VerifyReplay(result, body); err != nil {
				t.Fatal("deterministic replay failed; source/error bodies omitted")
			}
			if result.Status == "translated" {
				translated++
				provePilotRenderedDOM(t, root, result.Artifact.Content, file.Content)
				compiledRendered++
				t.Logf("status=translated compiled_rendered=true input_bytes=%d output_bytes=%d output_sha256=%s verification=not-verified", len(body), len(result.Artifact.Content), result.Artifact.SHA256)
			} else {
				unsupported++
				codes := map[string]int{}
				for _, d := range result.Diagnostics {
					codes[d.Code]++
				}
				t.Logf("status=unsupported diagnostics=%v omitted=%d verification=not-verified", codes, result.OmittedDiagnostics)
			}
		})
	}
	if translated+unsupported+localClientSources != 12 {
		t.Error("pilot denominator incomplete")
	}
	t.Logf("frozen_cases=12 translated=%d compiled_rendered_dom_equal=%d unsupported_by_html_rule=%d local_client_sources=%d verified_targets=0 visual_baselines=0", translated, compiledRendered, unsupported, localClientSources)
}

// Bind the complete source row, not the independent presence of its substrings.
// Archive aliases must resolve to the exact immutable ID in the same manifest.
func frozenPilotMatches(manifest, id, archive, member, sha string) bool {
	aliases := map[string]string{}
	caseRows := [][]string{}
	for _, line := range strings.Split(manifest, "\n") {
		parts := strings.Split(strings.TrimSpace(line), "|")
		for i := range parts {
			parts[i] = strings.Trim(strings.TrimSpace(parts[i]), "`")
		}
		if len(parts) == 5 && strings.Contains(parts[2], "@") {
			if _, exists := aliases[parts[1]]; exists {
				return false
			}
			aliases[parts[1]] = parts[2]
		}
		if len(parts) == 7 && parts[1] == id {
			caseRows = append(caseRows, parts)
		}
	}
	if len(caseRows) != 1 {
		return false
	}
	r := caseRows[0]
	if r[4] != member || r[5] != sha {
		return false
	}
	if r[3] == "MD" {
		return archive == ""
	}
	resolved, exists := aliases[r[3]]
	return exists && resolved == archive && archive != ""
}

func TestFrozenPilotBindingCanaries(t *testing.T) {
	const fixture = "| M | `marketing@aaa` | `marketing.zip` |\n" +
		"| A | `application@bbb` | `application.zip` |\n" +
		"| P01 | Hero | M | `html/hero.html` | `hash-one` |\n" +
		"| P02 | Table | A | `html/table.html` | `hash-two` |\n" +
		"| P03 | Editor | MD | `data/editor.tsx` | `hash-three` |\n"
	check := func(s string) bool {
		return frozenPilotMatches(s, "P01", "marketing@aaa", "html/hero.html", "hash-one")
	}
	if !check(fixture) || !frozenPilotMatches(fixture, "P03", "", "data/editor.tsx", "hash-three") {
		t.Fatal("valid source bindings refused")
	}
	for _, changed := range []string{
		strings.NewReplacer("P01", "P02", "P02", "P01").Replace(fixture),
		strings.NewReplacer("marketing@aaa", "application@bbb", "application@bbb", "marketing@aaa").Replace(fixture),
		strings.Replace(fixture, "| Hero | M |", "| Hero | A |", 1),
		fixture + "| P01 | Hero | M | `html/hero.html` | `hash-one` |\n",
		fixture + "| M | `marketing@aaa` | `marketing.zip` |\n",
	} {
		if check(changed) {
			t.Fatal("changed/ambiguous case binding passed manifest oracle")
		}
	}
}

func provePilotRenderedDOM(t *testing.T, root, converted, original string) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "snippet.templ")
	if err := os.WriteFile(file, []byte(converted), 0600); err != nil {
		t.Fatal("private generated fixture write failed")
	}
	testFile := filepath.Join(dir, "snippet_test.go")
	fixture := strings.Replace(pilotDOMFixture, "ORIGINAL_LITERAL", strconv.Quote(original), 1)
	if err := os.WriteFile(testFile, []byte(fixture), 0600); err != nil {
		t.Fatal("private original fixture write failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	run := func(step string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir, cmd.Env = root, offlineGoEnv()
		// Compiler output can quote licensed source. Keep even errors private.
		if err := cmd.Run(); err != nil {
			t.Fatalf("private compile/DOM proof failed at %s; source/output deliberately omitted", step)
		}
	}
	for _, formatted := range []bool{false, true} {
		if formatted {
			run("format", "tool", "templ", "fmt", file)
		}
		run("generate", "tool", "templ", "generate", "-f", file, "-include-version=false")
		run("compile-render-compare", "test", "-count=1", filepath.Join(dir, "snippet_templ.go"), testFile)
	}
}

const pilotDOMFixture = `package converted
import (
 "bytes"
 "context"
 "reflect"
 "strings"
 "testing"
 "golang.org/x/net/html"
 "golang.org/x/net/html/atom"
)
type nodeShape struct { Type html.NodeType; Data, Namespace string; Attr []html.Attribute; Children []nodeShape }
func shape(n *html.Node) nodeShape {
 s := nodeShape{Type:n.Type, Data:n.Data, Namespace:n.Namespace, Attr:n.Attr}
 for c := n.FirstChild; c != nil; c = c.NextSibling { s.Children = append(s.Children, shape(c)) }
 return s
}
func fragment(t *testing.T, s string) []nodeShape {
 t.Helper()
 nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{Type:html.ElementNode, Data:"div", DataAtom:atom.Div})
 if err != nil { t.Fatal("parse failed") }
 result := []nodeShape{}
 for _, n := range nodes { result = append(result, shape(n)) }
 return result
}
func TestOriginalRenderedDOM(t *testing.T) {
 const source = ORIGINAL_LITERAL
 var out bytes.Buffer
 if err := Specimen().Render(context.Background(), &out); err != nil { t.Fatal("render failed") }
 if !reflect.DeepEqual(fragment(t, source), fragment(t, out.String())) { t.Fatal("namespace-aware DOM/text/attribute/comment mismatch") }
}
`
