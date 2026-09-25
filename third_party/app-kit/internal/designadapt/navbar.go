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
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
	"golang.org/x/net/html"
)

const NavbarArchive = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"
const NavbarMember = "html/navigation/navbars/04-simple-with-menu-button-on-left.html"
const NavbarSourceSHA = "79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7"

//go:embed navbar.go navbar_model.go.txt navbar.cue
var navbarImplementation embed.FS

// NavbarArtifact is private source-specific evidence, not a certified target.
type NavbarArtifact struct {
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

// AdaptNavbar admits frozen P09 only. Callers verify archive access/membership
// separately; generation itself has no filesystem or network effects.
func AdaptNavbar(source designconvert.Source, body []byte) (NavbarArtifact, error) {
	if source.ArchiveID != NavbarArchive || source.Path != NavbarMember || source.SHA256 != NavbarSourceSHA {
		return NavbarArtifact{}, fmt.Errorf("navbar adapter: unsupported source identity")
	}
	return adaptNavbar(source, body)
}

func VerifyNavbarReplay(a NavbarArtifact, body []byte) error {
	return verifyNavbarReplayWith(a, body, AdaptNavbar)
}

func verifyNavbarReplayWith(a NavbarArtifact, body []byte, generate func(designconvert.Source, []byte) (NavbarArtifact, error)) error {
	if len(a.Templ) > 1<<20 || len(a.Model) > 32768 || len(a.Defaults) > 65536 {
		return fmt.Errorf("navbar adapter: artifact exceeds replay bounds")
	}
	w, err := generate(a.Source, body)
	if err != nil {
		return err
	}
	if a.SchemaVersion != w.SchemaVersion || a.Adapter != w.Adapter || a.Source != w.Source || a.Syntax != w.Syntax || a.Runtime != w.Runtime || a.RuleSHA256 != w.RuleSHA256 || a.SyntaxSHA256 != w.SyntaxSHA256 || a.TemplSHA256 != w.TemplSHA256 || a.ModelSHA256 != w.ModelSHA256 || a.DefaultsSHA256 != w.DefaultsSHA256 || a.Templ != w.Templ || a.Model != w.Model || a.Verification != w.Verification || !bytes.Equal(a.Defaults, w.Defaults) {
		return fmt.Errorf("navbar adapter: replay mismatch")
	}
	return nil
}

// Structure only: no purchased classes, copy or icon geometry.
const navbarShape = "nav div div div button span /span span /span svg path /svg svg path /svg /button /div div div img img /div div a /a a /a a /a a /a /div /div div button span /span span /span svg path /svg /button el-dropdown button span /span span /span img /button el-menu a /a a /a a /a /el-menu /el-dropdown /div /div /div el-disclosure div a /a a /a a /a a /a /div /el-disclosure /nav"

func adaptNavbar(source designconvert.Source, body []byte) (NavbarArtifact, error) {
	fail := func(reason string) (NavbarArtifact, error) {
		return NavbarArtifact{}, fmt.Errorf("navbar adapter: %s", reason)
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
	if len(tokens) != len(base.Artifact.Mappings) || len(stack) != 0 || strings.Join(shape, " ") != navbarShape || comments != 6 || len(texts) != 14 {
		return fail("unreviewed navbar structure")
	}
	attr := func(i int, key string) string {
		for _, a := range tokens[i].Attr {
			if a.Key == key {
				return a.Val
			}
		}
		return ""
	}
	buttons, links, spans, pictures := tags["button"], tags["a"], tags["span"], tags["img"]
	nav, disclosure, menu := tags["nav"][0], tags["el-disclosure"][0], tags["el-menu"][0]
	textParents := []int{spans[1], links[0], links[1], links[2], links[3], spans[3], spans[5], links[4], links[5], links[6], links[7], links[8], links[9], links[10]}
	for i, parent := range textParents {
		if parents[texts[i]] != parent || texts[i] != parent+1 {
			return fail("unreviewed text binding position")
		}
	}
	text := func(n int) string { return strings.TrimSpace(tokens[texts[n]].Data) }
	for i := 0; i < 4; i++ {
		if text(1+i) != text(10+i) {
			return fail("desktop/mobile source labels differ")
		}
	}
	if attr(disclosure, "id") != "mobile-menu" || attr(buttons[0], "type") != "button" || attr(buttons[0], "command") != "--toggle" || attr(buttons[0], "commandfor") != "mobile-menu" || attr(buttons[1], "type") != "button" || attr(buttons[2], "type") != "" || attr(menu, "anchor") != "bottom end" {
		return fail("unreviewed disclosure/menu contract")
	}
	for _, i := range []int{nav, buttons[1], buttons[2]} {
		for _, a := range tokens[i].Attr {
			if a.Key != "class" && !(i == buttons[1] && a.Key == "type") {
				return fail("unreviewed navigation/control attribute")
			}
		}
	}
	for _, i := range links {
		if attr(i, "href") != "#" || ends[i] != i+2 || attr(i, "aria-current") != "" {
			return fail("unreviewed source link")
		}
	}
	sameAttrs := func(a, b int) bool { return fmt.Sprint(tokens[a].Attr) == fmt.Sprint(tokens[b].Attr) }
	for _, pair := range [][2]int{{1, 2}, {1, 3}, {4, 5}, {4, 6}, {8, 9}, {8, 10}} {
		if !sameAttrs(links[pair[0]], links[pair[1]]) {
			return fail("nonuniform repeated link styles")
		}
	}
	if attr(pictures[0], "src") == "" || attr(pictures[1], "src") == "" || attr(pictures[2], "src") == "" || attr(pictures[0], "alt") == "" || attr(pictures[0], "alt") != attr(pictures[1], "alt") || attr(pictures[2], "alt") != "" {
		return fail("unreviewed image semantics")
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
		{disclosure, "id", `data.IDPrefix + "-mobile"`}, {buttons[0], "commandfor", `data.IDPrefix + "-mobile"`},
		{pictures[0], "src", "data.Brand.LightURL"}, {pictures[1], "src", "data.Brand.DarkURL"},
		{pictures[0], "alt", "data.Brand.Alt"}, {pictures[1], "alt", "data.Brand.Alt"}, {pictures[2], "src", "data.Account.ImageURL"},
		{links[0], "href", "item.URL"}, {links[1], "href", "item.URL"}, {links[4], "href", "item.URL"}, {links[7], "href", "item.URL"}, {links[8], "href", "item.URL"},
	} {
		if !bindAttr(b.i, b.key, b.expr) {
			return fail("attribute binding mismatch")
		}
	}
	for _, b := range []struct {
		i    int
		expr string
	}{{0, "data.MobileLabel"}, {1, "item.Label"}, {2, "item.Label"}, {5, "data.Notifications.Label"}, {6, "data.Account.Label"}, {7, "item.Label"}, {10, "item.Label"}, {11, "item.Label"}} {
		if !bindText(texts[b.i], b.expr) {
			return fail("text binding mismatch")
		}
	}
	chunks[nav] = strings.Replace(chunks[nav], "<nav", `<nav aria-label={ data.Label }`, 1)
	chunks[buttons[0]] = strings.Replace(chunks[buttons[0]], "<button", `<button aria-controls={ data.IDPrefix + "-mobile" }`, 1)
	chunks[buttons[2]] = strings.Replace(chunks[buttons[2]], "<button", `<button type="button"`, 1)
	chunks[buttons[1]] = strings.Replace(chunks[buttons[1]], ` type={ "button" }`, "", 1)
	chunks[buttons[1]] = strings.Replace(chunks[buttons[1]], "<button", `<a href={ data.Notifications.URL }`, 1)
	chunks[ends[buttons[1]]] = strings.Replace(chunks[ends[buttons[1]]], "</button>", "</a>", 1)
	for _, i := range []int{links[0], links[7]} {
		chunks[i] = strings.Replace(chunks[i], "<a", `<a aria-current="page"`, 1)
	}
	conditional := map[int]string{buttons[1]: "data.Notifications != nil", tags["el-dropdown"][0]: "data.Account != nil"}
	closeCondition := map[int]bool{}
	for start := range conditional {
		closeCondition[ends[start]] = true
	}
	var out strings.Builder
	if strings.Count(base.Artifact.Content, "templ Specimen()") != 1 {
		return fail("reference declaration mismatch")
	}
	out.WriteString(strings.Replace(base.Artifact.Content, "templ Specimen()", "templ NavbarReference()", 1))
	out.WriteString("\ntempl navbar(data Navbar) {\n")
	for i := 0; i < len(chunks); i++ {
		if expr, ok := conditional[i]; ok {
			out.WriteString("if " + expr + " {\n")
		}
		switch i {
		case links[0], links[7]:
			mobile := i == links[7]
			out.WriteString("for _, item := range data.Links {\n@navbarPrimary(item, item.Key == data.CurrentKey, " + strconv.FormatBool(mobile) + ")\n}\n")
			if mobile {
				i = ends[links[10]]
			} else {
				i = ends[links[3]]
			}
			continue
		case links[4]:
			out.WriteString("for _, item := range data.Account.Links {\n@navbarAccountLink(item)\n}\n")
			i = ends[links[6]]
			continue
		}
		out.WriteString(chunks[i])
		if closeCondition[i] {
			out.WriteString("}\n")
		}
	}
	out.WriteString("}\n\ntempl navbarPrimary(item NavbarLink, current bool, mobile bool) {\nif mobile {\nif current {\n")
	writeLink := func(i int) {
		for j := i; j <= ends[i]; j++ {
			out.WriteString(chunks[j])
		}
	}
	writeLink(links[7])
	out.WriteString("\n} else {\n")
	writeLink(links[8])
	out.WriteString("\n}\n} else {\nif current {\n")
	writeLink(links[0])
	out.WriteString("\n} else {\n")
	writeLink(links[1])
	out.WriteString("\n}\n}\n}\n\ntempl navbarAccountLink(item NavbarDestination) {\n")
	writeLink(links[4])
	out.WriteString("\n}\n")
	defaultsLinks := make([]map[string]string, 4)
	for i := range defaultsLinks {
		defaultsLinks[i] = map[string]string{"Key": "", "Label": text(1 + i), "URL": ""}
	}
	defaults, err := json.Marshal(map[string]any{"IDPrefix": "", "Label": "", "MobileLabel": text(0), "Brand": map[string]string{"LightURL": "", "DarkURL": "", "Alt": attr(pictures[0], "alt")}, "Links": defaultsLinks, "CurrentKey": "", "Notifications": nil, "Account": nil})
	if err != nil {
		return fail("defaults encoding failed")
	}
	inputs := make([][]byte, 3)
	h := sha256.New()
	for i, name := range []string{"navbar.go", "navbar_model.go.txt", "navbar.cue"} {
		inputs[i], err = navbarImplementation.ReadFile(name)
		if err != nil {
			return fail("implementation unavailable")
		}
		fmt.Fprintf(h, "%d:", len(inputs[i]))
		_, _ = h.Write(inputs[i])
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	result := NavbarArtifact{SchemaVersion: 1, Adapter: "p09-responsive-navbar-v1", Source: source, Syntax: base.Transformer, Runtime: base.Runtime, RuleSHA256: hex.EncodeToString(h.Sum(nil)), SyntaxSHA256: base.Transformer.SHA256, TemplSHA256: digest([]byte(out.String())), ModelSHA256: digest(inputs[1]), DefaultsSHA256: digest(defaults), Templ: out.String(), Model: string(inputs[1]), Defaults: defaults, Verification: "not-verified"}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fail("envelope encoding failed")
	}
	ctx := cuecontext.New()
	if err := ctx.CompileBytes(inputs[2]).LookupPath(cue.ParsePath("#NavbarArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return fail("generated envelope failed schema")
	}
	return result, nil
}
