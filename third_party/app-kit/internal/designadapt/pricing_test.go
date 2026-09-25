package designadapt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

// Entirely synthetic markup. No purchased body, class list, icon or copy.
const syntheticPricing = `<section><h2>Heading</h2><p>Introduction</p><h3>Plan</h3><p>Description</p><h4>Included</h4><ul>
<li> First </li>
<li> Second </li>
<li> Third </li>
<li> Fourth </li>
</ul><p>Payment</p><p><span>123</span><span>XYZ</span></p><a href="#">Action</a><p>Note</p></section>`

func syntheticSource(body string) designconvert.Source {
	h := sha256.Sum256([]byte(body))
	return designconvert.Source{ArchiveID: "synthetic@" + strings.Repeat("a", 64), Path: "html/synthetic.html", SHA256: hex.EncodeToString(h[:]), Size: len(body)}
}

func TestPricingSourceAndStructureRefusals(t *testing.T) {
	s := syntheticSource(syntheticPricing)
	if _, err := AdaptPricing(s, []byte(syntheticPricing)); err == nil {
		t.Fatal("arbitrary source accepted as frozen P02")
	}
	s.ArchiveID, s.Path, s.SHA256 = PricingArchive, PricingMember, PricingSourceSHA
	if _, err := AdaptPricing(s, []byte(syntheticPricing)); err == nil {
		t.Fatal("spoofed hash accepted")
	}
	for name, body := range map[string]string{
		"missing-field":   strings.Replace(syntheticPricing, "<h2>Heading</h2>", "", 1),
		"wrong-parent":    strings.ReplaceAll(syntheticPricing, "h2", "h1"),
		"extra-copy":      syntheticPricing + "<p>Extra</p>",
		"extra-link":      syntheticPricing + `<a href="#">Extra</a>`,
		"non-placeholder": strings.Replace(syntheticPricing, `href="#"`, `href="/elsewhere"`, 1),
		"variant-item":    strings.Replace(syntheticPricing, "<li> Second", `<li class="different"> Second`, 1),
		"variant-spacing": strings.Replace(syntheticPricing, " Third ", " Third  ", 1),
		"missing-item":    strings.Replace(syntheticPricing, "<li> Fourth </li>\n", "", 1),
		"behavior":        strings.Replace(syntheticPricing, "<section>", `<section onclick="bad()">`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			result, err := adaptPricing(syntheticSource(body), []byte(body))
			if err == nil || result.Templ != "" || result.Model != "" {
				t.Fatal("unsupported shape emitted partial output")
			}
		})
	}
}

func TestPricingSyntheticReplayAndCompiler(t *testing.T) {
	body := []byte(syntheticPricing)
	first, err := adaptPricing(syntheticSource(syntheticPricing), body)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adaptPricing(syntheticSource(syntheticPricing), body)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("non-deterministic adaptation")
	}
	if first.Verification != "not-verified" || len(first.RuleSHA256) != 64 || len(first.SyntaxSHA256) != 64 {
		t.Fatal("missing provenance boundary")
	}
	var defaults map[string]any
	if json.Unmarshal(first.Defaults, &defaults) != nil || defaults["URL"] != "" || len(defaults["Features"].([]any)) != 4 {
		t.Fatal("defaults invented an action")
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"pricing.templ": first.Templ, "pricing.go": first.Model, "pricing_test.go": pricingRuntimeTests + "\nconst sourceExpected = " + strconv.Quote(strings.Replace(syntheticPricing, `href="#"`, `href="/access"`, 1)) + "\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("synthetic compiler test: %v\n%s", err, out)
		}
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		if phase == "formatted" {
			run("tool", "templ", "fmt", filepath.Join(dir, "pricing.templ"))
		}
		run("tool", "templ", "generate", "-f", filepath.Join(dir, "pricing.templ"), "-include-version=false")
		run("test", "-count=1", filepath.Join(dir, "pricing.go"), filepath.Join(dir, "pricing_templ.go"), filepath.Join(dir, "pricing_test.go"))
	}
}

const pricingRuntimeTests = `package converted
import("bytes"; "context"; "errors"; "reflect"; "strings"; "testing"; "golang.org/x/net/html"; "github.com/a-h/templ")
type failWriter struct { err error }
func (w failWriter) Write(b []byte) (int,error) { return 0,w.err }
func TestEnhancedSharesValidationSnapshotAndErrors(t *testing.T) {
 for _,newComponent:=range []func(Pricing)(templ.Component,error){NewPricing,NewPricingEnhanced} {
  invalid:=good();invalid.URL="javascript:alert(1)";if c,e:=newComponent(invalid); e==nil||c!=nil {t.Fatal("constructor skipped validation")}
  d:=good();c,e:=newComponent(d);if e!=nil {t.Fatal(e)};d.Features[0]="mutation";var b bytes.Buffer;if e=c.Render(context.Background(),&b);e!=nil||strings.Contains(b.String(),"mutation") {t.Fatal("lost snapshot")}
  sentinel:=errors.New("writer failure");if e=c.Render(context.Background(),failWriter{sentinel});!errors.Is(e,sentinel) {t.Fatal("writer error lost")}
  ctx,cancel:=context.WithCancel(context.Background());cancel();if e=c.Render(ctx,&b);!errors.Is(e,context.Canceled) {t.Fatal("cancellation lost")}
 }
 c,e:=NewPricingEnhanced(good());if e!=nil {t.Fatal(e)};var b bytes.Buffer;_ = c.Render(context.Background(),&b);if !strings.HasPrefix(b.String(),"<div data-appkit-pricing")||strings.Count(b.String(),"data-appkit-pricing")!=1 {t.Fatal("enhanced wrapper")}
}
func TestRuntimeExactSourceDOM(t *testing.T) {
 d:=Pricing{Heading:"Heading",Introduction:"Introduction",Plan:"Plan",Description:"Description",Included:"Included",Features:[]string{"First","Second","Third","Fourth"},Payment:"Payment",Price:"123",Currency:"XYZ",Action:"Action",Note:"Note",URL:"/access"}
 actual:=render(t,d);got,e1:=html.Parse(strings.NewReader(actual));want,e2:=html.Parse(strings.NewReader(sourceExpected)); if e1!=nil||e2!=nil||!reflect.DeepEqual(got,want) {t.Fatalf("synthetic source DOM mismatch: got=%q want=%q",actual,sourceExpected)}
}
func good() Pricing { return Pricing{Heading:"Heading", Introduction:"Intro", Plan:"Plan", Description:"Description", Included:"Included", Features:[]string{"One", "Two"}, Payment:"Once", Price:"123", Currency:"XYZ", Action:"Continue", Note:"Not billing", URL:"/access?a=1&b=2"} }
func render(t *testing.T, d Pricing) string { t.Helper(); c,e:=NewPricing(d); if e!=nil {t.Fatal(e)}; var b bytes.Buffer; if e=c.Render(context.Background(), &b);e!=nil {t.Fatal(e)}; return b.String() }
func TestRuntimeEscapingSnapshotAndCounts(t *testing.T) {
 d:=good(); d.Heading="<script>not code</script> & \"text\""; d.Price="<img src=x>"; d.Features=[]string{"<b>not bold</b>", "日本語 café"}
 c,e:=NewPricing(d); if e!=nil {t.Fatal(e)}; d.Features[0]="mutation"; var b bytes.Buffer; if e=c.Render(context.Background(),&b);e!=nil {t.Fatal(e)}
 doc,e:=html.Parse(strings.NewReader(b.String())); if e!=nil {t.Fatal(e)}; anchors,items:=0,0; texts:=""
 var walk func(*html.Node); walk=func(n *html.Node){ if n.Type==html.ElementNode {switch n.Data {case "a": anchors++; for _,a:=range n.Attr {if a.Key=="href" && a.Val!=d.URL {t.Fatal("destination changed")}}; case "li": items++; case "script","img","b","form","button": t.Fatal("injection or invented behavior")}}; if n.Type==html.TextNode {texts+=n.Data}; for child:=n.FirstChild;child!=nil;child=child.NextSibling {walk(child)} }; walk(doc)
 if anchors!=1 || items!=2 || !strings.Contains(texts,d.Heading) || !strings.Contains(texts,d.Price) || !strings.Contains(texts,"<b>not bold</b>") || strings.Contains(texts,"mutation") {t.Fatal("escaping/snapshot/structure mismatch")}
 for _, count:=range []int{1,4,12} { d=good();d.Features=make([]string,count);for i:=range d.Features {d.Features[i]=strings.Repeat("Long label ",20)}; if strings.Count(render(t,d),"<li>")!=count {t.Fatal("feature count")}}
}
func TestRuntimeInvalidData(t *testing.T) {
 for _,name:=range []string{"Heading","Introduction","Plan","Description","Included","Payment","Price","Currency","Action","Note"} {
  for _,bad:=range []string{"", "   ", "bad\ncontrol", string([]byte{255}), strings.Repeat("x",2049)} {
   d:=good();reflect.ValueOf(&d).Elem().FieldByName(name).SetString(bad);c,e:=NewPricing(d);if e==nil||c!=nil {t.Fatalf("accepted %s",name)}
  }
 }
 for _,features:=range [][]string{nil,{}, {""},{"bad\tcontrol"},{strings.Repeat("x",241)}, make([]string,13)} {d:=good();d.Features=features;if c,e:=NewPricing(d);e==nil||c!=nil {t.Fatal("invalid feature accepted")}}
 for _,bad:=range []string{"", "#", "relative", "//evil.example/path", "/\\evil", "/%2fevil", "/%5cevil", "/%0aevil", "/x?y=%0D", "javascript:alert(1)", "data:text/html,x", "http://example.com", "https://", "https://user:pass@example.com", "https://example.com:bad", "https://example.com:65536", "https://example.com:0", "https://example.com:", " https://example.com", "https://example.com/\n", "https://example.com/%00", "https://example.com/%", "https://example.com\\@evil.com"} {
  d:=good();d.URL=bad;if c,e:=NewPricing(d);e==nil||c!=nil {t.Fatalf("invalid URL accepted: %q",bad)}
 }
 for _,valid:=range []string{"/access", "/access?q=a%20b&other=two#details", "https://example.com/access?q=one&b=two", "https://example.com:443/path"} { d:=good();d.URL=valid;_ = render(t,d) }
}
`
