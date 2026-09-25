package designadapt

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Wholly synthetic structure and copy. No purchased classes, images or prose.
const syntheticHero = `<div><div><div><div>
<img src="/original-light.svg" alt="Synthetic company" />
<img src="/original-dark.svg" alt="Synthetic company" />
<div><div> Synthetic announcement <a href="#"><span aria-hidden="true"></span>Read update <span aria-hidden="true">&rarr;</span></a></div></div>
<h1>Synthetic heading</h1><p>Independent fixture introduction.</p>
<div><a href="#">Begin fixture</a><a href="#">Explore fixture <span aria-hidden="true">→</span></a></div>
</div></div><div><img src="/original-image.jpg" alt="" /></div></div></div>`

//go:embed hero_runtime_test.go.txt
var heroRuntimeTests string

func TestHeroSourceAndStructureRefusals(t *testing.T) {
	s := syntheticSource(syntheticHero)
	if a, err := AdaptHero(s, []byte(syntheticHero)); err == nil || a.Templ != "" {
		t.Fatal("arbitrary source accepted")
	}
	s.ArchiveID, s.Path, s.SHA256, s.Size = HeroArchive, HeroMember, HeroSourceSHA, heroSourceSize
	if _, err := AdaptHero(s, []byte(syntheticHero)); err == nil {
		t.Fatal("spoofed frozen identity accepted")
	}
	for name, body := range map[string]string{
		"missing-field":       strings.Replace(syntheticHero, "Synthetic heading", "", 1),
		"wrong-parent":        strings.ReplaceAll(syntheticHero, "h1", "h2"),
		"extra-copy":          syntheticHero + "Extra",
		"extra-wrapper":       "<div>" + syntheticHero + "</div>",
		"extra-link":          syntheticHero + `<a href="#">Extra</a>`,
		"missing-link":        strings.Replace(syntheticHero, `<a href="#">Begin fixture</a>`, "", 1),
		"non-placeholder":     strings.Replace(syntheticHero, `href="#"`, `href="/real"`, 1),
		"missing-destination": strings.Replace(syntheticHero, ` href="#"`, "", 1),
		"missing-image":       strings.Replace(syntheticHero, `<img src="/original-image.jpg" alt="" />`, "", 1),
		"extra-image":         syntheticHero + `<img src="/extra.png" alt="Extra" />`,
		"image-alt-absent":    strings.Replace(syntheticHero, ` alt=""`, "", 1),
		"image-alt-changed":   strings.Replace(syntheticHero, `alt=""`, `alt="Not decorative"`, 1),
		"image-src-absent":    strings.Replace(syntheticHero, ` src="/original-image.jpg"`, "", 1),
		"logo-alt-mismatch":   strings.Replace(syntheticHero, `alt="Synthetic company"`, `alt="Other name"`, 1),
		"logo-alt-empty":      strings.ReplaceAll(syntheticHero, `alt="Synthetic company"`, `alt=""`),
		"arrow-not-hidden":    strings.Replace(syntheticHero, `aria-hidden="true"`, `aria-hidden="false"`, 1),
		"arrow-changed":       strings.Replace(syntheticHero, "&rarr;", "→ Extra", 1),
		"comment":             "<!-- Extra -->" + syntheticHero,
		"behavior":            strings.Replace(syntheticHero, "<div>", `<div onclick="bad()">`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			a, err := adaptHero(syntheticSource(body), []byte(body))
			if err == nil || a.Templ != "" || a.Model != "" || a.Defaults != nil {
				t.Fatal("unsupported source produced partial output")
			}
		})
	}
}

func TestHeroSyntheticReplayAndCompiler(t *testing.T) {
	a, err := adaptHero(syntheticSource(syntheticHero), []byte(syntheticHero))
	if err != nil {
		t.Fatal(err)
	}
	b, err := adaptHero(syntheticSource(syntheticHero), []byte(syntheticHero))
	if err != nil || !reflect.DeepEqual(a, b) || verifyHeroReplayWith(a, []byte(syntheticHero), adaptHero) != nil {
		t.Fatal("non-deterministic generation")
	}
	if a.Verification != "not-verified" || a.Adapter != "p01-hero-v1" || a.SourceAssets != (HeroSourceAssets{"/original-light.svg", "/original-dark.svg", "/original-image.jpg"}) {
		t.Fatal("provenance boundary mismatch")
	}
	var defaults map[string]any
	if json.Unmarshal(a.Defaults, &defaults) != nil || len(defaults) != 15 || defaults["ImageDecorative"] != true || defaults["ImageAlt"] != "" {
		t.Fatal("defaults changed image semantics")
	}
	for _, key := range []string{"LogoLightURL", "LogoDarkURL", "ImageURL", "AnnouncementURL", "PrimaryURL", "SecondaryURL"} {
		if defaults[key] != "" {
			t.Fatal("defaults enabled an external request or invented a route")
		}
	}
	for _, field := range []string{"Adapter", "RuleSHA256", "SyntaxSHA256", "TemplSHA256", "ModelSHA256", "CSSSHA256", "DefaultsSHA256", "Templ", "Model", "CSS", "Verification"} {
		t.Run("reject-replay-"+field, func(t *testing.T) {
			changed := a
			v := reflect.ValueOf(&changed).Elem().FieldByName(field)
			v.SetString(v.String() + "x")
			if verifyHeroReplayWith(changed, []byte(syntheticHero), adaptHero) == nil {
				t.Fatal("changed artifact accepted")
			}
		})
	}
	changed := a
	changed.SourceAssets.ImageURL += "?changed"
	if verifyHeroReplayWith(changed, []byte(syntheticHero), adaptHero) == nil {
		t.Fatal("changed asset provenance accepted")
	}
	changed = a
	changed.Defaults = append(append(json.RawMessage(nil), a.Defaults...), ' ')
	if verifyHeroReplayWith(changed, []byte(syntheticHero), adaptHero) == nil {
		t.Fatal("changed default bytes accepted")
	}
	runHeroCompiler(t, a, syntheticHero, false, "")
}

func runHeroCompiler(t *testing.T, a HeroArtifact, source string, private bool, outputDir string) {
	t.Helper()
	dir := t.TempDir()
	fixture := heroDOMTests + "\nconst heroSource = " + strconv.Quote(source) + "\nconst heroDefaults = " + strconv.Quote(string(a.Defaults)) + "\nconst heroOutput = " + strconv.Quote(outputDir) + "\n"
	for name, content := range map[string]string{"hero.templ": a.Templ, "hero.go": a.Model, "hero_test.go": heroRuntimeTests, "hero_dom_test.go": fixture} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal("hero compiler fixture write failed")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
		if private {
			if err := cmd.Run(); err != nil {
				t.Fatal("private hero compiler proof failed; source diagnostics withheld")
			}
		} else if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("synthetic hero compiler proof: %v\n%s", err, out)
		}
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		t.Run(phase, func(t *testing.T) {
			if phase == "formatted" {
				run("tool", "templ", "fmt", filepath.Join(dir, "hero.templ"))
			}
			run("tool", "templ", "generate", "-f", filepath.Join(dir, "hero.templ"), "-include-version=false")
			run("test", "-count=1", filepath.Join(dir, "hero.go"), filepath.Join(dir, "hero_templ.go"), filepath.Join(dir, "hero_test.go"), filepath.Join(dir, "hero_dom_test.go"))
		})
	}
}

const heroDOMTests = `package converted
import("bytes"; "encoding/json"; "os"; "path/filepath"; "reflect"; "strings"; "testing"; "golang.org/x/net/html")
func TestHeroExactSourceDOM(t *testing.T) {
 var d Hero
 if json.Unmarshal([]byte(heroDefaults),&d)!=nil {t.Fatal("defaults decode")}
 if c,e:=NewHero(d);e==nil||c!=nil {t.Fatal("source defaults enabled requests")}
 d.LogoLightURL="/assets/logo-light.svg";d.LogoDarkURL="/assets/logo-dark.svg";d.ImageURL="/assets/hero.jpg"
 d.AnnouncementURL="/announcement";d.PrimaryURL="/start";d.SecondaryURL="/learn"
 c,e:=NewHero(d);if e!=nil {t.Fatal(e)}
 actual:=renderHero(t,c)
 want,e:=html.Parse(strings.NewReader(heroSource));if e!=nil {t.Fatal(e)}
 got,e:=html.Parse(strings.NewReader(actual));if e!=nil {t.Fatal(e)}
 links,images:=0,0
 var walk func(*html.Node)
 walk=func(n *html.Node){if n.Type==html.ElementNode {for i,a:=range n.Attr {
  if n.Data=="a"&&a.Key=="href" {if links>=3 {t.Fatal("extra link")};n.Attr[i].Val=[]string{d.AnnouncementURL,d.PrimaryURL,d.SecondaryURL}[links];links++}
  if n.Data=="img"&&a.Key=="src" {if images>=3 {t.Fatal("extra image")};n.Attr[i].Val=[]string{d.LogoLightURL,d.LogoDarkURL,d.ImageURL}[images];images++}
 }};for child:=n.FirstChild;child!=nil;child=child.NextSibling {walk(child)}}
 walk(want)
 if links!=3||images!=3||!reflect.DeepEqual(got,want) {t.Fatal("source DOM differs beyond the six declared URL bindings")}
 enhanced,e:=NewHeroEnhanced(d);if e!=nil {t.Fatal(e)}
 enhancedHTML:=renderHero(t,enhanced)
 // Compare full parsed tree: the only enhanced structural change is one wrapper.
 enclosed,e:=html.Parse(strings.NewReader("<div data-appkit-hero>"+actual+"</div>"));if e!=nil {t.Fatal(e)}
 enhancedDOM,e:=html.Parse(strings.NewReader(enhancedHTML));if e!=nil||!reflect.DeepEqual(enhancedDOM,enclosed) {t.Fatal("enhanced content drift")}
 if heroOutput!="" {
  var original bytes.Buffer;if e:=html.Render(&original,want);e!=nil {t.Fatal(e)}
  for name,body:=range map[string]string{"reference.html":original.String(),"typed.html":actual,"enhanced.html":enhancedHTML} {if e:=os.WriteFile(filepath.Join(heroOutput,name),[]byte(body),0600);e!=nil {t.Fatal(e)}}
  d.Heading=strings.Repeat("W",160);d.Introduction=strings.Repeat("LongCopy",150)
  d.Announcement=strings.Repeat("Announcement",50);d.AnnouncementLabel=strings.Repeat("A",100)
  d.PrimaryLabel=strings.Repeat("P",100);d.SecondaryLabel=strings.Repeat("S",100)
  long,e:=NewHero(d);if e!=nil {t.Fatal(e)};longEnhanced,e:=NewHeroEnhanced(d);if e!=nil {t.Fatal(e)}
  for name,body:=range map[string]string{"typed-long.html":renderHero(t,long),"enhanced-long.html":renderHero(t,longEnhanced)} {if e:=os.WriteFile(filepath.Join(heroOutput,name),[]byte(body),0600);e!=nil {t.Fatal(e)}}
 }
}
`
