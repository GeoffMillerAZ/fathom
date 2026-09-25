package designconvert

import (
	"bytes"
	"context"
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

func TestHTMLToTemplDeterministicLiteralMappings(t *testing.T) {
	source := []byte(`<div class="a  b&#10;a" data-example="&quot; }; import &quot;os&quot;; {"><span>A</span><span>B</span>{ @Thing() } &amp; &lt;</div>`)
	before := bytes.Clone(source)
	a, err := HTMLToTempl(source)
	if err != nil {
		t.Fatal(err)
	}
	if a.Templ == "" || len(a.Diagnostics) != 0 || len(a.Mappings) == 0 {
		t.Fatalf("missing complete translation: %+v", a)
	}
	b, err := HTMLToTempl(source)
	if err != nil || !reflect.DeepEqual(a, b) || !bytes.Equal(source, before) {
		t.Fatal("translation is nondeterministic or mutated input", err)
	}
	if !strings.Contains(a.Templ, `class={ "a  b\na" }`) || strings.Contains(a.Templ, "templ.Raw") {
		t.Fatal("literal class fidelity/raw-HTML contract changed")
	}
	end, outEnd := 0, 0
	for _, m := range a.Mappings {
		if m.SourceStart != end || m.SourceEnd <= m.SourceStart || m.SourceEnd > len(source) || m.OutputStart < outEnd || m.OutputEnd <= m.OutputStart || m.OutputEnd > len(a.Templ) {
			t.Fatalf("bad source mapping: %+v", m)
		}
		end, outEnd = m.SourceEnd, m.OutputEnd
	}
	if end != len(source) {
		t.Fatal("mappings omit source bytes")
	}
	encoded, err := json.Marshal(a)
	if err != nil || !bytes.Contains(encoded, []byte(`"source_start"`)) || !bytes.Contains(encoded, []byte(`"omitted_diagnostics"`)) {
		t.Fatal("missing stable JSON fields", err)
	}
}

func TestHTMLToTemplRefusesUnsupportedWithoutPartialOutput(t *testing.T) {
	cases := []struct{ name, source, code string }{
		{"elements runtime", `<div>before</div><el-dialog open><button>Close</button></el-dialog>`, "unsupported-element"},
		{"script", `<script type="module">import x from 'x';</script><p>after</p>`, "unsupported-element"},
		{"style element", `<style>body{color:red}</style>`, "unsupported-element"},
		{"style attribute", `<p style="color:red">x</p>`, "unsupported-attribute"},
		{"inline handler", `<button OnClick="alert(1)">x</button>`, "unsupported-attribute"},
		{"alpine", `<div x-data="{}"></div>`, "unsupported-attribute"},
		{"vue", `<div v-if="true"></div>`, "unsupported-attribute"},
		{"vue shorthand", `<div :class="foo"></div>`, "unsupported-attribute"},
		{"datastar", `<div data-signals="{}"></div>`, "unsupported-attribute"},
		{"datastar modifier", `<div data-on:click="@get('/x')"></div>`, "unsupported-attribute"},
		{"unknown custom", `<widget-box></widget-box>`, "unsupported-element"},
		{"jsx props", `<div className="x">x</div>`, "unsupported-attribute"},
		{"source language", `export default function App() { return null }`, "html-fragment"},
		{"duplicate", `<p class="one" CLASS="two">x</p>`, "duplicate-attribute"},
		{"unsafe href", `<a href="javascript:alert(1)">x</a>`, "unsafe-resource"},
		{"unsafe image", `<img src="javascript:alert(1)">`, "unsafe-resource"},
		{"unsafe encoded scheme", `<a href="jav&#x61;script:alert(1)">x</a>`, "unsafe-resource"},
		{"unsafe encoded controls", `<a href="java&#10;script:alert(1)">x</a>`, "unsafe-resource"},
		{"unsafe form", `<form action="data:text/html,x"></form>`, "unsafe-resource"},
		{"unsafe form button", `<button formaction="javascript:x">x</button>`, "unsafe-resource"},
		{"unsafe form input", `<input formaction="javascript:x">`, "unsafe-resource"},
		{"unsafe svg href", `<svg><use href="javascript:x"/></svg>`, "unsafe-resource"},
		{"unsafe svg xlink", `<svg><use xlink:href="data:text/html,x"/></svg>`, "unsafe-resource"},
		{"unsafe svg fill", `<svg><path fill="url(https://example.test/image.svg)"/></svg>`, "unsafe-resource"},
		{"unsafe svg expression", `<svg><path fill="expression(alert(1))"/></svg>`, "unsafe-resource"},
		{"unsafe protocol relative", `<img src="//example.test/x">`, "unsafe-resource"},
		{"unsafe credentials", `<img src="https://user:password@example.test/x">`, "unsafe-resource"},
		{"unsafe backslash", `<a href="https:\example.test">x</a>`, "unsafe-resource"},
		{"templ would rewrite fragment", `<a href="#item:details">x</a>`, "unsafe-resource"},
		{"templ would rewrite relative query", `<form action="?next=https://example.test"></form>`, "unsafe-resource"},
		{"unsupported srcset", `<img srcset="/small.jpg 1x, /large.jpg 2x">`, "unsupported-attribute"},
		{"unsafe srcdoc", `<div srcdoc="<script>x</script>"></div>`, "unsupported-attribute"},
		{"unknown namespace", `<svg xmlns="https://example.test/namespace"></svg>`, "unsafe-resource"},
		{"foreign object", `<svg><foreignObject><div>x</div></foreignObject></svg>`, "unsupported-element"},
		{"svg animation", `<svg><animate attributeName="x"/></svg>`, "unsupported-element"},
		{"svg text path", `<svg><text><textPath href="#curve">x</textPath></text></svg>`, "unsupported-element"},
		{"document", `<!DOCTYPE html><html><body>x</body></html>`, "unsupported-document"},
		{"cdata", `<svg><![CDATA[x]]></svg>`, "unsupported-comment"},
		{"conditional comment", `<!--[if IE]>x<![endif]--><p>x</p>`, "unsupported-comment"},
		{"malformed comment", `<!--a--b--><p>x</p>`, "unsupported-comment"},
		{"unclosed comment", `<p>x</p><!--oops`, "unsupported-comment"},
		{"nonvoid slash", `<span/>`, "html-self-closing"},
		{"unclosed", `<p>x`, "html-nesting"},
		{"closing mismatch", `<div><span>x</div></span>`, "html-nesting"},
		{"stray close", `</p><div>x</div>`, "html-nesting"},
		{"implicit p repair", `<p><div>x</div></p>`, "html-nesting"},
		{"nested li repair", `<ul><li>one<li>two</li></li></ul>`, "html-tree-repair"},
		{"table text reparenting", `<table>outside<tbody><tr><td>inside</td></tr></tbody></table>`, "html-tree-repair"},
		{"heading repair", `<h1>one<h2>two</h2></h1>`, "html-tree-repair"},
		{"closing slash", `<p>x</p/>`, "html-syntax"},
		{"closing attributes", `<p>x</p extra="x">`, "html-syntax"},
		{"attribute missing separator", `<p title="one"class="two">x</p>`, "html-syntax"},
		{"unquoted attribute", `<p title=unquoted>x</p>`, "html-syntax"},
		{"malformed unquoted attribute", `<p title=one<two>x</p>`, "html-syntax"},
		{"implicit table", `<table><tr><td>x</td></tr></table>`, "html-nesting"},
		{"table foster parent", `<table><div>x</div></table>`, "html-nesting"},
		{"nested anchors", `<a><a>x</a></a>`, "html-nesting"},
		{"nested forms", `<form><form></form></form>`, "html-nesting"},
		{"select discarded node", `<select><div>x</div></select>`, "html-nesting"},
		{"text entity CR", `<p>&#13;</p>`, "text-control"},
		{"attribute entity CR", `<p title="&#13;">x</p>`, "attribute-control"},
		{"invalid utf8", "<p>\xff</p>", "source-encoding"},
		{"NUL", "<p>\x00</p>", "source-encoding"},
		{"control", "<p>\x01</p>", "source-control"},
		{"empty", "", "html-fragment"},
		{"truncated tag", `<div title="unterminated`, "html-syntax"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := HTMLToTempl([]byte(tc.source))
			if err != nil {
				t.Fatal("unsupported source returned internal error", err)
			}
			if r.Templ != "" || len(r.Mappings) != 0 {
				t.Fatal("unsupported source emitted partial output")
			}
			found := false
			for _, d := range r.Diagnostics {
				if d.Code == tc.code {
					found = true
				}
				if d.Offset < 0 || d.Offset > len(tc.source) {
					t.Fatal("offset is outside original bytes")
				}
				if strings.Contains(d.Message, "alert(1)") || strings.Contains(d.Message, "password") {
					t.Fatal("diagnostic disclosed source excerpt")
				}
			}
			if !found {
				t.Fatalf("missing %s: %+v", tc.code, r.Diagnostics)
			}
		})
	}
}

func TestHTMLToTemplBounds(t *testing.T) {
	cases := []struct{ name, input, code string }{
		{"source", strings.Repeat("x", MaxSourceBytes+1), "source-limit"},
		{"depth", strings.Repeat("<div>", maxDepth+1), "depth-limit"},
		{"tokens", "<div>" + strings.Repeat("<br>", maxTokens) + "</div>", "token-limit"},
		{"attributes", "<p" + strings.Repeat(` title="x"`, maxAttributes+1) + ">x</p>", "attribute-limit"},
		{"output", strings.Repeat("<div>", maxDepth-2) + strings.Repeat("<span>x</span>", 6000) + strings.Repeat("</div>", maxDepth-2), "output-limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := HTMLToTempl([]byte(tc.input))
			if err != nil || r.Templ != "" || len(r.Mappings) != 0 {
				t.Fatal("bound did not refuse cleanly", err)
			}
			found := false
			for _, d := range r.Diagnostics {
				if d.Code == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s diagnostic: %+v", tc.code, r.Diagnostics)
			}
		})
	}
	r, err := HTMLToTempl([]byte(strings.Repeat(`<p onclick="x">x</p>`, MaxDiagnostics+7)))
	if err != nil || len(r.Diagnostics) != MaxDiagnostics || r.OmittedDiagnostics != 7 || r.Templ != "" || len(r.Mappings) != 0 {
		t.Fatalf("diagnostic bound: %+v, %v", r, err)
	}
}

func TestHTMLToTemplSafeLiteralGrammar(t *testing.T) {
	for _, source := range []string{
		`<a href="https://example.test/a?q=x&amp;b=y" target="_blank" rel="noopener">x</a>`,
		`<a href="mailto:person@example.test">x</a><a href="tel:+123456789">call</a>`,
		`<form action="/submit" method="post"><input disabled="false" checked required aria-disabled="false"><button formaction="./submit">x</button></form>`,
		`<div hidden="until-found" popover data-is-selected data-state="open"></div>`,
		`<table><caption>x</caption><colgroup><col></colgroup><thead><tr><th scope="col">x</th></tr></thead><tbody><tr><td>x</td></tr></tbody></table>`,
		`<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><defs><linearGradient id="g" gradientUnits="userSpaceOnUse"><stop offset="0%" stop-color="red"/></linearGradient><clipPath id="clip"><rect width="10" height="10"/></clipPath></defs><path d="M0 0h10v10z" fill="url(#g)" clip-path="url(#clip)"/><use xlink:href="#g"/></svg>`,
		"<pre>\r\nA\t  B</pre><textarea>\r\nC&amp;&lt;</textarea>",
		"<!-- @Thing() { \"quoted\" } \\ backtick ` and </script> --><p>{ \" }; func bad() {} @Thing()</p>",
		`<!----><p title = 'single quote' checked>empty comment</p>`,
	} {
		r, err := HTMLToTempl([]byte(source))
		if err != nil || r.Templ == "" || len(r.Diagnostics) != 0 {
			t.Fatalf("supported synthetic source rejected: %v, %+v", err, r.Diagnostics)
		}
		if err := validateTempl(r.Templ); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTemplASTGateRejectsExecutableChanges(t *testing.T) {
	for _, source := range []string{
		"package converted\nimport \"os\"\ntempl Specimen() {<div/>}",
		"package other\ntempl Specimen() {<div/>}",
		"package converted\ntempl Other() {<div/>}",
		"package converted\ntempl Specimen() {<div>{ dangerous() }</div>}",
		"package converted\ntempl Specimen() {<div title={ dangerous() }/>}",
		"package converted\ntempl Specimen() {<div title=\"literal\"/>}",
		"package converted\ntempl Specimen() {@Other()}",
		"package converted\ntempl Specimen() {if true {<div/>}}",
		"package converted\ntempl Specimen() {\n// source bytes 0..4 arbitrary\n<div/>\n}",
	} {
		if validateTempl(source) == nil {
			t.Fatal("AST gate accepted unexpected grammar")
		}
	}
}

// This is deliberately not an opt-in test: compiler fidelity is a required
// synthetic regression, not a skipped vendor/visual compatibility claim.
func TestHTMLToTemplPinnedFormatGenerateCompileRender(t *testing.T) {
	source := "<div class=\"a  b&#10;a\" data-example=\"&quot; }; import &quot;os&quot;; {\"><span>A</span><span>B</span>C&amp;D\n  <span>&lt;literal&gt; { @Thing() }</span>\t<span>Z</span><!-- @Thing() { \"quoted\" } \\ backtick ` --></div>" +
		"<pre>\nA\t  B</pre><textarea>\nC&amp;&lt;</textarea>" +
		`<p title="a&#10;b&#9;c" data-empty="" aria-label="&quot;&amp;&lt;&gt;">x</p>` +
		"<!--\n  α &amp;\t\n--><!-- \t\n --><!----><p>x</p>" +
		"<pre></pre><pre>\n</pre><pre>\n\nX</pre><textarea></textarea><textarea>\n</textarea><textarea>\n\n&amp;&lt;</textarea>" +
		`<form action="/submit"><input disabled="false" checked required aria-disabled="false"><button formaction="./submit">x</button></form>` +
		`<div hidden="until-found" popover data-is-selected></div>` +
		`<p><span>A</span><br><span>B</span><!----><span>C</span></p>` +
		`<label for="choice">Pick</label><select id="choice"><option value="" disabled selected>None</option><optgroup label="Group"><option value="a">A &amp; B</option></optgroup></select>` +
		`<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><defs><linearGradient id="g" gradientUnits="userSpaceOnUse"><stop offset="0%" stop-color="red"/></linearGradient><clipPath id="clip"><rect width="10" height="10"/></clipPath></defs><path d="M0 0h10v10z" fill="url(#g)" clip-path="url(#clip)"/><use xlink:href="#g"/></svg>`
	r, err := HTMLToTempl([]byte(source))
	if err != nil || r.Templ == "" {
		t.Fatalf("translation: %v %+v", err, r.Diagnostics)
	}
	dir := t.TempDir()
	templPath := filepath.Join(dir, "specimen.templ")
	if err := os.WriteFile(templPath, []byte(r.Templ), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureTest := strings.ReplaceAll(renderFixtureTest, "SOURCE_LITERAL", strconv.Quote(source))
	fixturePath := filepath.Join(dir, "specimen_test.go")
	if err := os.WriteFile(fixturePath, []byte(fixtureTest), 0600); err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = repo
		cmd.Env = offlineGoEnv()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("pinned synthetic compiler check %v: %v\n%s", args, err, out)
		}
	}
	for _, formatted := range []bool{false, true} {
		if formatted {
			run("tool", "templ", "fmt", templPath)
		}
		run("tool", "templ", "generate", "-f", templPath, "-include-version=false")
		run("test", "-count=1", filepath.Join(dir, "specimen_templ.go"), fixturePath)
	}
}

func FuzzHTMLToTempl(f *testing.F) {
	for _, s := range []string{`<p>x</p>`, `<svg><path d="M0 0"/></svg>`, `<p title="&quot;&lt;">{literal}</p>`, `<!-- comment --><p>x</p>`, `<select><option selected>x</option></select>`, `<p title=bad>x</p>`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, source []byte) {
		r, err := HTMLToTempl(source)
		if err != nil {
			t.Fatal("bounded input exposed an emitter/AST mismatch", err)
		}
		if len(r.Diagnostics) > MaxDiagnostics || r.OmittedDiagnostics < 0 {
			t.Fatal("diagnostic bound violated")
		}
		if len(r.Diagnostics) != 0 {
			if r.Templ != "" || len(r.Mappings) != 0 {
				t.Fatal("unsupported source emitted partial output")
			}
		} else if r.Templ == "" || len(r.Templ) > maxOutputBytes {
			t.Fatal("supported source has missing or oversized output")
		}
	})
}

func offlineGoEnv() []string {
	env := []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "GOPROXY" && key != "GOSUMDB" && key != "GOTOOLCHAIN" && key != "GOFLAGS" && key != "GOWORK" {
			env = append(env, item)
		}
	}
	return append(env, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
}

const renderFixtureTest = `package converted
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
 if err != nil { t.Fatal(err) }
 result := []nodeShape{}
 for _, n := range nodes { result = append(result, shape(n)) }
 return result
}
func TestRenderedSyntheticSource(t *testing.T) {
 const source = SOURCE_LITERAL
 var out bytes.Buffer
 if err := Specimen().Render(context.Background(), &out); err != nil { t.Fatal(err) }
 if !strings.Contains(out.String(), "<span>A</span><span>B</span>C&amp;D") { t.Fatal("formatting added semantic whitespace between inline siblings") }
 if !reflect.DeepEqual(fragment(t, source), fragment(t, out.String())) { t.Fatalf("namespace-aware DOM/text/attribute/comment mismatch\nsource: %s\nrender: %s", source, out.String()) }
}
`
