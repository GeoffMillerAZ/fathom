package designadapt

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
	"golang.org/x/net/html"
)

const DrawerArchive = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"
const DrawerMember = "html/overlays/drawers/07-user-profile-example.html"
const DrawerSourceSHA = "1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415"

//go:embed drawer.go drawer_model.go.txt drawer.cue
var drawerImplementation embed.FS

// DrawerArtifact contains private source-specific generation evidence, not a
// certified component. Runtime installation and application authority are external.
type DrawerArtifact struct {
	SchemaVersion  int                           `json:"schema_version"`
	Adapter        string                        `json:"adapter"`
	Source         designconvert.Source          `json:"source"`
	Syntax         designconvert.Transformer     `json:"syntax"`
	Runtime        designconvert.ElementsRuntime `json:"runtime"`
	RuleSHA256     string                        `json:"rule_sha256"`
	SyntaxSHA256   string                        `json:"syntax_sha256"`
	TemplSHA256    string                        `json:"templ_sha256"`
	ModelSHA256    string                        `json:"model_sha256"`
	DefaultsSHA256 string                        `json:"defaults_sha256"`
	Templ          string                        `json:"templ"`
	Model          string                        `json:"model"`
	Defaults       json.RawMessage               `json:"defaults"`
	Verification   string                        `json:"verification"`
}

// AdaptDrawer accepts frozen P08 only. The caller separately verifies original
// archive access and membership. No network or filesystem operations occur here.
func AdaptDrawer(source designconvert.Source, body []byte) (DrawerArtifact, error) {
	if source.ArchiveID != DrawerArchive || source.Path != DrawerMember || source.SHA256 != DrawerSourceSHA {
		return DrawerArtifact{}, fmt.Errorf("drawer adapter: unsupported source identity")
	}
	return adaptDrawer(source, body)
}

func VerifyDrawerReplay(a DrawerArtifact, body []byte) error {
	return verifyDrawerReplayWith(a, body, AdaptDrawer)
}

func verifyDrawerReplayWith(a DrawerArtifact, body []byte, generate func(designconvert.Source, []byte) (DrawerArtifact, error)) error {
	if len(a.Templ) > 1<<20 || len(a.Model) > 32768 || len(a.Defaults) > 65536 {
		return fmt.Errorf("drawer adapter: artifact exceeds replay bounds")
	}
	w, err := generate(a.Source, body)
	if err != nil {
		return err
	}
	if a.SchemaVersion != w.SchemaVersion || a.Adapter != w.Adapter || a.Source != w.Source || a.Syntax != w.Syntax || a.Runtime != w.Runtime || a.RuleSHA256 != w.RuleSHA256 || a.SyntaxSHA256 != w.SyntaxSHA256 || a.TemplSHA256 != w.TemplSHA256 || a.ModelSHA256 != w.ModelSHA256 || a.DefaultsSHA256 != w.DefaultsSHA256 || a.Templ != w.Templ || a.Model != w.Model || a.Verification != w.Verification || !bytes.Equal(a.Defaults, w.Defaults) {
		return fmt.Errorf("drawer adapter: replay mismatch")
	}
	return nil
}

// Structural signature only, without purchased copy, classes or icon geometry.
const drawerShape = "button /button el-dialog dialog div el-dialog-panel div div div h2 /h2 div button span /span span /span svg path /svg /button /div /div /div div div div div img /div div div div div h3 /h3 span span /span /span /div p /p /div div button /button button /button div el-dropdown button span /span span /span svg path /svg /button el-menu div a /a a /a /div /el-menu /el-dropdown /div /div /div /div /div /div div dl div dt /dt dd p /p /dd /div div dt /dt dd /dd /div div dt /dt dd /dd /div div dt /dt dd time /time /dd /div /dl /div /div /div /el-dialog-panel /div /dialog /el-dialog"

func adaptDrawer(source designconvert.Source, body []byte) (DrawerArtifact, error) {
	fail := func(reason string) (DrawerArtifact, error) {
		return DrawerArtifact{}, fmt.Errorf("drawer adapter: %s", reason)
	}
	base, err := designconvert.TranslateElements(source, body)
	if err != nil || base.Status != "translated" || base.Artifact == nil {
		return fail("source did not pass Elements syntax translation")
	}
	var tokens []html.Token
	var chunks, shape []string
	var parents, ends, stack, texts []int
	tags := map[string][]int{}
	comments := 0
	z := html.NewTokenizer(bytes.NewReader(body))
	for i := 0; ; i++ {
		kind := z.Next()
		if kind == html.ErrorToken {
			if z.Err() != io.EOF {
				return fail("tokenization failed")
			}
			break
		}
		if i >= len(base.Artifact.Mappings) {
			return fail("source map mismatch")
		}
		t := z.Token()
		m := base.Artifact.Mappings[i]
		parent := -1
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		tokens = append(tokens, t)
		parents = append(parents, parent)
		ends = append(ends, i)
		chunks = append(chunks, base.Artifact.Content[m.OutputStart:m.OutputEnd])
		switch kind {
		case html.StartTagToken, html.SelfClosingTagToken:
			shape = append(shape, t.Data)
			tags[t.Data] = append(tags[t.Data], i)
			if kind == html.StartTagToken && t.Data != "img" {
				stack = append(stack, i)
			}
		case html.EndTagToken:
			shape = append(shape, "/"+t.Data)
			if len(stack) == 0 || tokens[stack[len(stack)-1]].Data != t.Data {
				return fail("unexpected nesting")
			}
			ends[stack[len(stack)-1]] = i
			stack = stack[:len(stack)-1]
		case html.TextToken:
			if strings.TrimSpace(t.Data) != "" {
				texts = append(texts, i)
			}
		case html.CommentToken:
			comments++
		default:
			return fail("unexpected token kind")
		}
	}
	if len(tokens) != len(base.Artifact.Mappings) || len(stack) != 0 || strings.Join(shape, " ") != drawerShape || comments != 3 || len(texts) != 19 {
		return fail("unreviewed drawer structure")
	}
	attr := func(i int, key string) string {
		for _, a := range tokens[i].Attr {
			if a.Key == key {
				return a.Val
			}
		}
		return ""
	}
	buttons, links, dt, dd := tags["button"], tags["a"], tags["dt"], tags["dd"]
	dialog, heading, picture := tags["dialog"][0], tags["h2"][0], tags["img"][0]
	textParents := []int{buttons[0], heading, tags["span"][1], tags["h3"][0], tags["span"][3], tags["p"][0], buttons[2], buttons[3], tags["span"][5], links[0], links[1], dt[0], tags["p"][1], dt[1], dd[1], dt[2], dd[2], dt[3], tags["time"][0]}
	for i, parent := range textParents {
		if parents[texts[i]] != parent || texts[i] != parent+1 {
			return fail("unreviewed text binding position")
		}
	}
	if attr(dialog, "id") != "drawer" || attr(heading, "id") != "slide-over-heading" || attr(dialog, "aria-labelledby") != "" || attr(buttons[0], "command") != "show-modal" || attr(buttons[0], "commandfor") != "drawer" || attr(buttons[1], "command") != "close" || attr(buttons[1], "commandfor") != "drawer" || attr(buttons[0], "type") != "" || attr(buttons[4], "type") != "" {
		return fail("unreviewed dialog commands")
	}
	for _, i := range buttons[1:4] {
		if attr(i, "type") != "button" {
			return fail("unreviewed action type")
		}
	}
	for _, i := range buttons[2:4] {
		for _, a := range tokens[i].Attr {
			if a.Key != "type" && a.Key != "class" {
				return fail("unreviewed button-to-link attribute")
			}
		}
	}
	for _, i := range links {
		if attr(i, "href") != "#" || ends[i] != i+2 {
			return fail("unreviewed menu links")
		}
	}
	if attr(picture, "src") == "" || attr(picture, "alt") != "" {
		return fail("unreviewed image semantics")
	}
	if attr(tags["time"][0], "datetime") == "" {
		return fail("missing source date")
	}
	if len(tokens[tags["p"][1]].Attr) != 0 || len(tokens[tags["time"][0]].Attr) != 1 {
		return fail("unreviewed detail content attributes")
	}
	// The repeated source terms/definitions must really use one reviewed style.
	sameAttrs := func(a, b int) bool { return fmt.Sprint(tokens[a].Attr) == fmt.Sprint(tokens[b].Attr) }
	for i := 1; i < 4; i++ {
		if !sameAttrs(dt[0], dt[i]) || !sameAttrs(dd[0], dd[i]) || !sameAttrs(parents[dt[0]], parents[dt[i]]) {
			return fail("nonuniform detail styles")
		}
	}
	if !sameAttrs(links[0], links[1]) {
		return fail("nonuniform menu styles")
	}
	bindAttr := func(i int, key, expr string) bool {
		old := key + "={ " + strconv.Quote(attr(i, key)) + " }"
		if strings.Count(chunks[i], old) != 1 {
			return false
		}
		chunks[i] = strings.Replace(chunks[i], old, key+"={ "+expr+" }", 1)
		return true
	}
	bindText := func(i int, expr string) bool {
		value := tokens[i].Data
		trimmed := strings.TrimSpace(value)
		at := strings.Index(value, trimmed)
		old := "{ " + strconv.Quote(value) + " }"
		if strings.Count(chunks[i], old) != 1 {
			return false
		}
		chunks[i] = strings.Replace(chunks[i], old, "{ "+strconv.Quote(value[:at])+" + "+expr+" + "+strconv.Quote(value[at+len(trimmed):])+" }", 1)
		return true
	}
	for _, b := range []struct {
		i         int
		key, expr string
	}{
		{dialog, "id", `data.IDPrefix + "-dialog"`}, {heading, "id", `data.IDPrefix + "-heading"`},
		{buttons[0], "commandfor", `data.IDPrefix + "-dialog"`}, {buttons[1], "commandfor", `data.IDPrefix + "-dialog"`},
		{picture, "src", "data.ImageURL"}, {picture, "alt", "data.ImageAlt"}, {links[0], "href", "item.URL"},
	} {
		if !bindAttr(b.i, b.key, b.expr) {
			return fail("attribute binding mismatch")
		}
	}
	for _, b := range []struct {
		i    int
		expr string
	}{{0, "data.OpenLabel"}, {1, "data.Heading"}, {2, "data.CloseLabel"}, {3, "data.Name"}, {4, "data.OnlineLabel"}, {5, "data.Handle"}, {6, "data.Primary.Label"}, {7, "data.Secondary.Label"}, {8, "data.MenuLabel"}, {9, "item.Label"}} {
		if !bindText(texts[b.i], b.expr) {
			return fail("text binding mismatch")
		}
	}
	chunks[dialog] = strings.Replace(chunks[dialog], "<dialog", `<dialog aria-labelledby={ data.IDPrefix + "-heading" }`, 1)
	for _, i := range []int{buttons[0], buttons[4]} {
		chunks[i] = strings.Replace(chunks[i], "<button", `<button type="button"`, 1)
	}
	for n, i := range buttons[2:4] {
		field := []string{"Primary", "Secondary"}[n]
		chunks[i] = strings.Replace(chunks[i], ` type={ "button" }`, "", 1)
		chunks[i] = strings.Replace(chunks[i], "<button", `<a href={ data.`+field+`.URL }`, 1)
		chunks[ends[i]] = strings.Replace(chunks[ends[i]], "</button>", "</a>", 1)
	}
	conditional := map[int]string{buttons[2]: "data.Primary != nil", buttons[3]: "data.Secondary != nil", tags["el-dropdown"][0]: "len(data.MenuLinks) > 0", parents[texts[5]]: `data.Handle != ""`, parents[parents[texts[4]]]: `data.OnlineLabel != ""`}
	closeCondition := map[int]bool{}
	for start := range conditional {
		closeCondition[ends[start]] = true
	}
	detailStart, detailEnd := parents[dt[0]], ends[parents[dt[3]]]
	var out strings.Builder
	if strings.Count(base.Artifact.Content, "templ Specimen()") != 1 {
		return fail("reference declaration mismatch")
	}
	out.WriteString(strings.Replace(base.Artifact.Content, "templ Specimen()", "templ DrawerReference()", 1))
	out.WriteString("\ntempl drawer(data Drawer) {\n")
	for i := 0; i < len(chunks); i++ {
		if expr, ok := conditional[i]; ok {
			out.WriteString("if " + expr + " {\n")
		}
		if i == links[0] {
			out.WriteString("for _, item := range data.MenuLinks {\n@drawerMenuLink(item)\n}\n")
			i = ends[links[1]]
			continue
		}
		if i == detailStart {
			out.WriteString("for _, item := range data.Details {\n@drawerDetail(item)\n}\n")
			i = detailEnd
			continue
		}
		out.WriteString(chunks[i])
		if closeCondition[i] {
			out.WriteString("}\n")
		}
	}
	out.WriteString("}\n\ntempl drawerMenuLink(item DrawerLink) {\n")
	for i := links[0]; i <= ends[links[0]]; i++ {
		out.WriteString(chunks[i])
	}
	out.WriteString("}\n\ntempl drawerDetail(item DrawerDetail) {\n")
	out.WriteString(chunks[detailStart] + chunks[dt[0]] + "{ item.Label }" + chunks[ends[dt[0]]] + chunks[dd[0]])
	out.WriteString("\nif item.Kind == \"paragraph\" {\n<p>{ item.Text }</p>\n} else if item.Kind == \"date\" {\n<time datetime={ item.Date }>{ item.Text }</time>\n} else {\n{ item.Text }\n}\n")
	out.WriteString(chunks[ends[dd[0]]] + chunks[ends[detailStart]] + "}\n")
	text := func(n int) string { return strings.TrimSpace(tokens[texts[n]].Data) }
	details := make([]map[string]string, 4)
	for i := range details {
		details[i] = map[string]string{"Label": text(11 + i*2), "Text": text(12 + i*2), "Kind": "text", "Date": ""}
	}
	details[0]["Kind"] = "paragraph"
	details[3]["Kind"] = "date"
	details[3]["Date"] = attr(tags["time"][0], "datetime")
	defaults, err := json.Marshal(map[string]any{"IDPrefix": "", "OpenLabel": text(0), "Heading": text(1), "CloseLabel": text(2), "Name": text(3), "Handle": text(5), "OnlineLabel": "", "MenuLabel": text(8), "ImageURL": "", "ImageAlt": "", "ImageDecorative": true, "Primary": nil, "Secondary": nil, "MenuLinks": []any{}, "Details": details})
	if err != nil {
		return fail("defaults encoding failed")
	}
	inputs := make([][]byte, 3)
	h := sha256.New()
	for i, name := range []string{"drawer.go", "drawer_model.go.txt", "drawer.cue"} {
		inputs[i], err = drawerImplementation.ReadFile(name)
		if err != nil {
			return fail("implementation unavailable")
		}
		fmt.Fprintf(h, "%d:", len(inputs[i]))
		_, _ = h.Write(inputs[i])
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	result := DrawerArtifact{SchemaVersion: 1, Adapter: "p08-profile-drawer-v1", Source: source, Syntax: base.Transformer, Runtime: base.Runtime, RuleSHA256: hex.EncodeToString(h.Sum(nil)), SyntaxSHA256: base.Transformer.SHA256, TemplSHA256: digest([]byte(out.String())), ModelSHA256: digest(inputs[1]), DefaultsSHA256: digest(defaults), Templ: out.String(), Model: string(inputs[1]), Defaults: defaults, Verification: "not-verified"}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fail("envelope encoding failed")
	}
	ctx := cuecontext.New()
	if err := ctx.CompileBytes(inputs[2]).LookupPath(cue.ParsePath("#DrawerArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return fail("generated envelope failed schema")
	}
	return result, nil
}
