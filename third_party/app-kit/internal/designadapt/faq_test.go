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

func syntheticFAQ() string {
	var out strings.Builder
	out.WriteString("<!-- synthetic runtime note -->\n<!-- <script type=\"module\" src=\"/fixture.js\"></script> -->\n")
	out.WriteString(`<div class="fixture-surface"><div class="fixture-width"><div class="fixture-copy"><h2 class="fixture-heading"> Fixture questions </h2><dl class="fixture-list">`)
	for i := 0; i < 6; i++ {
		if i > 0 {
			out.WriteByte('\n')
		}
		hidden := ""
		if i > 0 {
			hidden = " hidden"
		}
		key := strconv.Itoa(i)
		out.WriteString(`<div class="fixture-item"><dt><button type="button" command="--toggle" commandfor="fixture-faq-` + key + `" class="group flex w-full items-start justify-between text-left text-gray-900"><span class="fixture-question">Fixture question ` + key + `?</span><span class="fixture-icons"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" data-slot="icon" aria-hidden="true" class="size-6 group-aria-expanded:hidden"><path d="M12 6v12M6 12h12" stroke-linecap="round" stroke-linejoin="round" /></svg><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" data-slot="icon" aria-hidden="true" class="hidden size-6 group-aria-expanded:block"><path d="M6 12h12" stroke-linecap="round" stroke-linejoin="round" /></svg></span></button></dt><el-disclosure id="fixture-faq-` + key + `"` + hidden + ` class="fixture-panel"><dd class="fixture-answer"><p class="fixture-answer-copy text-gray-600">Fixture answer ` + key + `.</p></dd></el-disclosure></div>`)
	}
	out.WriteString(`</dl></div></div></div>`)
	return out.String()
}

func faqSyntheticSource(body string) designconvert.Source {
	sum := sha256.Sum256([]byte(body))
	return designconvert.Source{ArchiveID: "marketing@" + hex.EncodeToString(sum[:]), Path: "html/sections/faq-sections/synthetic.html", SHA256: hex.EncodeToString(sum[:]), Size: len(body)}
}

func TestFAQSyntheticStructureDefaultsAndRefusals(t *testing.T) {
	body := syntheticFAQ()
	source := faqSyntheticSource(body)
	if _, err := AdaptFAQ(source, []byte(body)); err == nil {
		t.Fatal("synthetic source accepted by frozen entrypoint")
	}
	spoof := source
	spoof.ArchiveID, spoof.Path, spoof.SHA256, spoof.Size = FAQArchive, FAQMember, FAQSourceSHA, faqSourceSize
	if _, err := AdaptFAQ(spoof, []byte(body)); err == nil {
		t.Fatal("spoofed frozen identity accepted a different body")
	}

	first, err := adaptFAQ(source, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	second, err := adaptFAQ(source, []byte(body))
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("FAQ generation is nondeterministic")
	}
	if first.Adapter != "p03-faq-v1" || first.Verification != "not-verified" || first.Runtime.SourceReference.Package != designconvert.ElementsRuntimePackage || first.Runtime.SourceReference.Version != designconvert.ElementsRuntimeVersion || first.Runtime.SourceReference.ESMSHA256 != designconvert.ElementsRuntimeESMSHA256 || first.Runtime.SourceReference.TGZSHA256 != designconvert.ElementsRuntimeTGZSHA256 || first.Runtime.SourceReference.Delivery != designconvert.ElementsRuntimeDelivery || first.Runtime.NativeDetails != nil {
		t.Fatal("variant runtime or verification boundary changed")
	}
	if first.TemplSHA256 != faqDigest(first.Templ) || first.ModelSHA256 != faqDigest(first.Model) || first.DefaultsSHA256 != faqDigest(string(first.Defaults)) || first.CSSSHA256 != faqDigest(first.CSS) {
		t.Fatal("artifact digest mismatch")
	}
	var defaults struct {
		IDPrefix, Heading string
		Items             []struct {
			Key, Question, Answer string
			Open                  bool
		}
	}
	if err := json.Unmarshal(first.Defaults, &defaults); err != nil || defaults.IDPrefix != "fixture-faq" || defaults.Heading != " Fixture questions " || len(defaults.Items) != 6 || defaults.Items[0].Key != "0" || !defaults.Items[0].Open || defaults.Items[1].Open {
		t.Fatalf("source defaults mismatch: %#v, %v", defaults, err)
	}
	if !strings.Contains(first.CSS, `[data-appkit-faq-root] h2 { overflow-wrap: anywhere; }`) || !strings.Contains(first.CSS, `details[open]`) || !strings.Contains(first.CSS, "min-height: 44px") || strings.Contains(first.CSS, "el-disclosure") || strings.Contains(first.CSS, "aria-expanded") {
		t.Fatal("native CSS is unscoped, incomplete, or retains Elements state selectors")
	}

	mutations := map[string]string{
		"unknown-shape":        strings.Replace(body, "Fixture question 0?", "<em>Fixture question 0?</em>", 1),
		"unknown-attribute":    strings.Replace(body, `class="fixture-item"`, `class="fixture-item" data-new="shape"`, 1),
		"wrong-command":        strings.Replace(body, `command="--toggle"`, `command="--hide"`, 1),
		"wrong-target":         strings.Replace(body, `commandfor="fixture-faq-0"`, `commandfor="fixture-faq-5"`, 1),
		"duplicate-key":        strings.ReplaceAll(body, "fixture-faq-1", "fixture-faq-0"),
		"class-variant":        replaceFAQNth(body, `class="fixture-item"`, `class="fixture-item changed"`, 2),
		"geometry-variant":     replaceFAQNth(body, `d="M12 6v12M6 12h12"`, `d="M1 1h1"`, 2),
		"extra-default-open":   strings.Replace(body, `id="fixture-faq-1" hidden`, `id="fixture-faq-1"`, 1),
		"nonuniform-separator": strings.Replace(body, "</div>\n<div class=\"fixture-item\"", "</div>\n \n<div class=\"fixture-item\"", 1),
	}
	for name, changed := range mutations {
		t.Run(name, func(t *testing.T) {
			got, err := adaptFAQ(faqSyntheticSource(changed), []byte(changed))
			if err == nil || got.Templ != "" || got.Model != "" || got.CSS != "" {
				t.Fatal("unreviewed source emitted output")
			}
		})
	}
}

func TestFAQReplayRejectsEveryMaterialChange(t *testing.T) {
	body := syntheticFAQ()
	artifact, err := adaptFAQ(faqSyntheticSource(body), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyFAQReplayWith(artifact, []byte(body), adaptFAQ); err != nil {
		t.Fatalf("unchanged replay failed: %v", err)
	}
	mutations := map[string]func(*FAQArtifact){
		"adapter":     func(a *FAQArtifact) { a.Adapter += "-tampered" },
		"source":      func(a *FAQArtifact) { a.Source.Path += ".changed" },
		"syntax":      func(a *FAQArtifact) { a.Syntax.GoVersion += "-changed" },
		"runtime":     func(a *FAQArtifact) { a.Runtime.SourceReference.Version = "changed" },
		"native":      func(a *FAQArtifact) { r := a.Runtime.SourceReference; a.Runtime.NativeDetails = &r },
		"rule-hash":   func(a *FAQArtifact) { a.RuleSHA256 = strings.Repeat("0", 64) },
		"syntax-hash": func(a *FAQArtifact) { a.SyntaxSHA256 = strings.Repeat("0", 64) },
		"templ":       func(a *FAQArtifact) { a.Templ += " " },
		"model":       func(a *FAQArtifact) { a.Model += " " },
		"defaults":    func(a *FAQArtifact) { a.Defaults = append(append([]byte(nil), a.Defaults...), ' ') },
		"css":         func(a *FAQArtifact) { a.CSS += " " },
		"templ-hash":  func(a *FAQArtifact) { a.TemplSHA256 = strings.Repeat("0", 64) },
		"model-hash":  func(a *FAQArtifact) { a.ModelSHA256 = strings.Repeat("0", 64) },
		"default-hash": func(a *FAQArtifact) {
			a.DefaultsSHA256 = strings.Repeat("0", 64)
		},
		"css-hash":     func(a *FAQArtifact) { a.CSSSHA256 = strings.Repeat("0", 64) },
		"verification": func(a *FAQArtifact) { a.Verification = "certified" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := artifact
			changed.Defaults = append(json.RawMessage(nil), artifact.Defaults...)
			mutate(&changed)
			if verifyFAQReplayWith(changed, []byte(body), adaptFAQ) == nil {
				t.Fatal("tampered replay accepted")
			}
		})
	}
	oversized := artifact
	oversized.Templ = strings.Repeat("x", 1<<20+1)
	if verifyFAQReplayWith(oversized, []byte(body), adaptFAQ) == nil {
		t.Fatal("oversized artifact accepted")
	}
	if VerifyFAQReplay(artifact, []byte(body)) == nil {
		t.Fatal("public replay bypassed frozen identity")
	}
}

func TestFAQGeneratedPackageBeforeAndAfterFormatting(t *testing.T) {
	body := syntheticFAQ()
	artifact, err := adaptFAQ(faqSyntheticSource(body), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	files := map[string]string{
		"faq.templ":   artifact.Templ,
		"faq.go":      artifact.Model,
		"faq_test.go": faqRuntimeTests + "\nconst sourceExpected = " + strconv.Quote(body) + "\nconst sourceDefaults = " + strconv.Quote(string(artifact.Defaults)) + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generated package: %v\n%s", err, output)
		}
	}
	for _, phase := range []string{"raw", "formatted"} {
		if phase == "formatted" {
			run("tool", "templ", "fmt", filepath.Join(dir, "faq.templ"))
		}
		run("tool", "templ", "generate", "-f", filepath.Join(dir, "faq.templ"), "-include-version=false")
		run("test", "-count=1", filepath.Join(dir, "faq.go"), filepath.Join(dir, "faq_templ.go"), filepath.Join(dir, "faq_test.go"))
	}
}

func TestFAQFrozenPrivateSourceOptIn(t *testing.T) {
	if os.Getenv("APPKIT_FAQ_PRIVATE_CHECK") == "" {
		t.Skip("set APPKIT_FAQ_PRIVATE_CHECK=1 to verify the frozen paid source")
	}
	body, err := os.ReadFile(filepath.Join("..", "..", "data", "design-assets", "reference-assets", "elements-1.0.22-pilot", "p03-faq-original.html"))
	if err != nil {
		t.Fatal(err)
	}
	source := designconvert.Source{ArchiveID: FAQArchive, Path: FAQMember, SHA256: FAQSourceSHA, Size: len(body)}
	artifact, err := AdaptFAQ(source, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyFAQReplay(artifact, body); err != nil {
		t.Fatal(err)
	}
}

func faqDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func replaceFAQNth(s, old, replacement string, n int) string {
	index := -1
	for i := 0; i < n; i++ {
		next := strings.Index(s[index+1:], old)
		if next < 0 {
			return s
		}
		index += next + 1
	}
	return s[:index] + replacement + s[index+len(old):]
}

const faqRuntimeTests = `package converted

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "io"
 "reflect"
 "strings"
 "testing"
 "golang.org/x/net/html"
)

type failWriter struct{ err error }
func (w failWriter) Write([]byte) (int,error) { return 0,w.err }

func fixture(t *testing.T) FAQ { t.Helper(); var data FAQ; if err:=json.Unmarshal([]byte(sourceDefaults),&data);err!=nil{t.Fatal(err)};return data }
func render(t *testing.T,c interface{Render(context.Context,io.Writer)error})string{t.Helper();var out bytes.Buffer;if err:=c.Render(context.Background(),&out);err!=nil{t.Fatal(err)};return out.String()}
func parse(t *testing.T,s string)*html.Node{t.Helper();n,e:=html.Parse(strings.NewReader(s));if e!=nil{t.Fatal(e)};return n}
type canon struct{Type html.NodeType;Namespace,Data string;Attr []html.Attribute;Children []canon}
func canonical(n *html.Node)canon{c:=canon{n.Type,n.Namespace,n.Data,append([]html.Attribute(nil),n.Attr...),nil};for x:=n.FirstChild;x!=nil;x=x.NextSibling{c.Children=append(c.Children,canonical(x))};return c}
func walk(n *html.Node,fn func(*html.Node)){fn(n);for c:=n.FirstChild;c!=nil;c=c.NextSibling{walk(c,fn)}}
func elements(n *html.Node,name string)[]*html.Node{var out []*html.Node;walk(n,func(x *html.Node){if x.Type==html.ElementNode&&x.Namespace==""&&x.Data==name{out=append(out,x)}});return out}
func attr(n *html.Node,key string)(string,bool){for _,a:=range n.Attr{if a.Namespace==""&&a.Key==key{return a.Val,true}};return "",false}
func text(n *html.Node)string{var b strings.Builder;walk(n,func(x *html.Node){if x.Type==html.TextNode{b.WriteString(x.Data)}});return b.String()}
func requireInvalid(t *testing.T,d FAQ){t.Helper();for _,f:=range []func(FAQ)(interface{Render(context.Context,io.Writer)error},error){func(d FAQ)(interface{Render(context.Context,io.Writer)error},error){return NewFAQReference(d)},func(d FAQ)(interface{Render(context.Context,io.Writer)error},error){return NewFAQDetails(d)}}{if c,e:=f(d);e==nil||c!=nil{t.Fatal("invalid FAQ accepted")}}}

func TestReferenceExactNamespaceDOMTextWhitespaceAndAttributeOrder(t *testing.T){d:=fixture(t);c,e:=NewFAQReference(d);if e!=nil{t.Fatal(e)};got,want:=canonical(parse(t,render(t,c))),canonical(parse(t,sourceExpected));if !reflect.DeepEqual(got,want){t.Fatal("reference DOM differs from source")}}

func TestNativeDetailsSemanticsBindingsAndNoRuntime(t *testing.T){d:=fixture(t);d.IDPrefix="native-a";d.Items[1].Open=true;c,e:=NewFAQDetails(d);if e!=nil{t.Fatal(e)};out:=render(t,c);doc:=parse(t,out);ds,ss:=elements(doc,"details"),elements(doc,"summary");if len(ds)!=6||len(ss)!=6{t.Fatalf("details=%d summary=%d",len(ds),len(ss))};for _,bad:=range []string{"el-disclosure","commandfor=","command=","aria-expanded","<button","<script"}{if strings.Contains(out,bad){t.Fatalf("native retained %q",bad)}};for i,item:=range d.Items{id,_:=attr(elements(ds[i],"div")[0],"id");_,open:=attr(ds[i],"open");if id!="native-a-"+item.Key||open!=item.Open||!strings.Contains(text(ss[i]),item.Question){t.Fatalf("native binding %d",i)}};icons:=0;walk(doc,func(n *html.Node){if _,ok:=attr(n,"data-appkit-faq-icon");ok{icons++}});if icons!=12{t.Fatalf("icons=%d",icons)}}

func TestEscapingSnapshotWriterAndCancellation(t *testing.T){d:=fixture(t);d.Heading="<script>heading</script> & text";d.Items[0].Question="<img src=x> question";d.Items[0].Answer="line one\n\tline two & <b>plain</b>";c,e:=NewFAQDetails(d);if e!=nil{t.Fatal(e)};wantQ,wantA:=d.Items[0].Question,d.Items[0].Answer;d.Items[0].Question="mutated";d.Items[0].Answer="mutated";d.Items=append(d.Items,FAQItem{Key:"new",Question:"new",Answer:"new"});out:=render(t,c);doc:=parse(t,out);if len(elements(doc,"script"))!=0||len(elements(doc,"img"))!=0||len(elements(doc,"b"))!=0||!strings.Contains(text(doc),wantQ)||!strings.Contains(text(doc),wantA)||strings.Contains(text(doc),"mutated"){t.Fatal("escaping or snapshot failed")};sentinel:=errors.New("writer failure");if e:=c.Render(context.Background(),failWriter{sentinel});!errors.Is(e,sentinel){t.Fatalf("writer error=%v",e)};ctx,cancel:=context.WithCancel(context.Background());cancel();if e:=c.Render(ctx,&bytes.Buffer{});!errors.Is(e,context.Canceled){t.Fatalf("cancel error=%v",e)}}

func TestBoundsIdentitiesAndControls(t *testing.T){base:=fixture(t);for _,count:=range []int{0,25}{d:=base;d.Items=append([]FAQItem(nil),base.Items...);for len(d.Items)<count{d.Items=append(d.Items,FAQItem{Key:"x_"+strings.Repeat("x",len(d.Items)),Question:"Q",Answer:"A"})};d.Items=d.Items[:count];requireInvalid(t,d)};for _,count:=range []int{1,24}{d:=FAQ{IDPrefix:"A-",Heading:strings.Repeat("h",256)};for i:=0;i<count;i++{d.Items=append(d.Items,FAQItem{Key:"_"+strings.Repeat("x",i),Question:strings.Repeat("q",512),Answer:strings.Repeat("a",8192)})};if _,e:=NewFAQDetails(d);e!=nil{t.Fatalf("valid count %d: %v",count,e)}};for _,bad:=range []string{"","1prefix","bad prefix","bad/",strings.Repeat("p",65),string([]byte{255})}{d:=base;d.IDPrefix=bad;requireInvalid(t,d)};for _,bad:=range []string{"","has-hyphen","bad key","bad/",strings.Repeat("k",65),string([]byte{255})}{d:=base;d.Items=append([]FAQItem(nil),base.Items...);d.Items[0].Key=bad;requireInvalid(t,d)};d:=base;d.Items=append([]FAQItem(nil),base.Items...);d.Items[1].Key=d.Items[0].Key;requireInvalid(t,d);for _,mutate:=range []func(*FAQ){func(d *FAQ){d.Heading=" "},func(d *FAQ){d.Heading="bad\nheading"},func(d *FAQ){d.Heading=string([]byte{255})},func(d *FAQ){d.Heading=strings.Repeat("x",257)},func(d *FAQ){d.Items[0].Question="\t"},func(d *FAQ){d.Items[0].Question="bad\tquestion"},func(d *FAQ){d.Items[0].Question=string([]byte{255})},func(d *FAQ){d.Items[0].Question=strings.Repeat("x",513)},func(d *FAQ){d.Items[0].Answer="\n\t"},func(d *FAQ){d.Items[0].Answer="bad\ranswer"},func(d *FAQ){d.Items[0].Answer=strings.Repeat("x",8193)},func(d *FAQ){d.Items[0].Answer=string([]byte{255})}}{d:=base;d.Items=append([]FAQItem(nil),base.Items...);mutate(&d);requireInvalid(t,d)};d=base;d.IDPrefix="a-b";d.Items=append([]FAQItem(nil),base.Items...);d.Items[0].Key="c_";if _,e:=NewFAQDetails(d);e!=nil{t.Fatal(e)};d.IDPrefix="a-";d.Items[0].Key="_";if _,e:=NewFAQDetails(d);e!=nil{t.Fatal(e)};d.IDPrefix="a";d.Items[0].Key="b-c";requireInvalid(t,d)}
`
