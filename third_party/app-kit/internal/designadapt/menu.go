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

const MenuArchive = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"
const MenuMember = "html/elements/dropdowns/01-simple.html"
const MenuSourceSHA = "10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a"

//go:embed menu.go menu_model.go.txt menu.cue menu_styles.css
var menuImplementation embed.FS

// MenuArtifact is private generation evidence, never target certification.
// Templ includes an unchanged constant reference and an explicit action adapter.
type MenuArtifact struct {
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
	CSSSHA256      string                        `json:"css_sha256"`
	Templ          string                        `json:"templ"`
	Model          string                        `json:"model"`
	CSS            string                        `json:"css"`
	Defaults       json.RawMessage               `json:"defaults"`
	Verification   string                        `json:"verification"`
}

// AdaptMenu accepts only frozen P07. The caller independently checks archive
// access and membership with designassets.ReadFile. This function performs no I/O.
func AdaptMenu(source designconvert.Source, body []byte) (MenuArtifact, error) {
	if source.ArchiveID != MenuArchive || source.Path != MenuMember || source.SHA256 != MenuSourceSHA {
		return MenuArtifact{}, fmt.Errorf("menu adapter: unsupported source identity")
	}
	return adaptMenu(source, body)
}

func VerifyMenuReplay(artifact MenuArtifact, body []byte) error {
	return verifyMenuReplayWith(artifact, body, AdaptMenu)
}

func verifyMenuReplayWith(a MenuArtifact, body []byte, generate func(designconvert.Source, []byte) (MenuArtifact, error)) error {
	if len(a.Templ) > 1<<20 || len(a.Model) > 32768 || len(a.CSS) > 16384 || len(a.Defaults) > 32768 {
		return fmt.Errorf("menu adapter: artifact exceeds replay bounds")
	}
	w, err := generate(a.Source, body)
	if err != nil {
		return err
	}
	if a.SchemaVersion != w.SchemaVersion || a.Adapter != w.Adapter || a.Source != w.Source || a.Syntax != w.Syntax || a.Runtime != w.Runtime || a.RuleSHA256 != w.RuleSHA256 || a.SyntaxSHA256 != w.SyntaxSHA256 || a.TemplSHA256 != w.TemplSHA256 || a.ModelSHA256 != w.ModelSHA256 || a.DefaultsSHA256 != w.DefaultsSHA256 || a.CSSSHA256 != w.CSSSHA256 || a.Templ != w.Templ || a.Model != w.Model || a.CSS != w.CSS || a.Verification != w.Verification || !bytes.Equal(a.Defaults, w.Defaults) {
		return fmt.Errorf("menu adapter: replay mismatch")
	}
	return nil
}

// Structural signature only; no purchased classes, icon geometry or prose.
const menuShape = "el-dropdown button svg path /svg /button el-menu div a /a a /a a /a form button /button /form /div /el-menu /el-dropdown"

func adaptMenu(source designconvert.Source, body []byte) (MenuArtifact, error) {
	fail := func(reason string) (MenuArtifact, error) {
		return MenuArtifact{}, fmt.Errorf("menu adapter: %s", reason)
	}
	base, err := designconvert.TranslateElements(source, body)
	if err != nil || base.Status != "translated" || base.Artifact == nil {
		return fail("source did not pass Elements syntax translation")
	}
	var tokens []html.Token
	var chunks, shape, stack []string
	var links, linkEnds, textIndexes, buttons []int
	formStart, formEnd, comments := -1, -1, 0
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
		tokens = append(tokens, t)
		chunks = append(chunks, base.Artifact.Content[m.OutputStart:m.OutputEnd])
		switch kind {
		case html.StartTagToken, html.SelfClosingTagToken:
			shape = append(shape, t.Data)
			if kind == html.StartTagToken {
				stack = append(stack, t.Data)
			}
			switch t.Data {
			case "button":
				buttons = append(buttons, i)
			case "a":
				links = append(links, i)
			case "form":
				formStart = i
			}
		case html.EndTagToken:
			shape = append(shape, "/"+t.Data)
			if len(stack) == 0 || stack[len(stack)-1] != t.Data {
				return fail("unexpected nesting")
			}
			stack = stack[:len(stack)-1]
			if t.Data == "a" {
				linkEnds = append(linkEnds, i)
			}
			if t.Data == "form" {
				formEnd = i
			}
		case html.TextToken:
			if strings.TrimSpace(t.Data) != "" {
				textIndexes = append(textIndexes, i)
			}
		case html.CommentToken:
			if len(shape) != 0 {
				return fail("unexpected comment placement")
			}
			comments++
		default:
			return fail("unexpected token kind")
		}
	}
	if len(tokens) != len(base.Artifact.Mappings) || len(stack) != 0 || comments != 2 || strings.Join(shape, " ") != menuShape || len(links) != 3 || len(linkEnds) != 3 || len(buttons) != 2 || len(textIndexes) != 5 || formStart < 0 || formEnd < formStart {
		return fail("unreviewed menu structure")
	}
	attr := func(i int, key string) (string, bool) {
		for _, a := range tokens[i].Attr {
			if a.Key == key {
				return a.Val, true
			}
		}
		return "", false
	}
	if _, exists := attr(buttons[0], "type"); exists {
		return fail("unreviewed trigger type")
	}
	if method, _ := attr(formStart, "method"); method != "POST" {
		return fail("unreviewed form method")
	}
	if action, _ := attr(formStart, "action"); action != "#" {
		return fail("unreviewed form action")
	}
	if kind, _ := attr(buttons[1], "type"); kind != "submit" {
		return fail("unreviewed submit type")
	}
	for i, start := range links {
		if href, _ := attr(start, "href"); href != "#" || linkEnds[i] != start+2 || textIndexes[i+1] != start+1 {
			return fail("unreviewed link structure")
		}
		if len(tokens[start].Attr) != len(tokens[links[0]].Attr) {
			return fail("nonuniform link attributes")
		}
		for j, a := range tokens[start].Attr {
			if a != tokens[links[0]].Attr[j] {
				return fail("nonuniform link attributes")
			}
		}
	}
	if textIndexes[0] != buttons[0]+1 || textIndexes[4] != buttons[1]+1 {
		return fail("unreviewed button text")
	}
	separator := string(body[base.Artifact.Mappings[linkEnds[0]].SourceEnd:base.Artifact.Mappings[links[1]].SourceStart])
	if strings.TrimSpace(separator) != "" || separator != string(body[base.Artifact.Mappings[linkEnds[1]].SourceEnd:base.Artifact.Mappings[links[2]].SourceStart]) {
		return fail("nonuniform link separators")
	}
	bindAttr := func(i int, key, value, expr string) error {
		needle := key + "={ " + strconv.Quote(value) + " }"
		if strings.Count(chunks[i], needle) != 1 {
			return fmt.Errorf("attribute binding mismatch")
		}
		chunks[i] = strings.Replace(chunks[i], needle, key+"={ "+expr+" }", 1)
		return nil
	}
	bindText := func(i int, expr string) error {
		value := tokens[i].Data
		trimmed := strings.TrimSpace(value)
		at := strings.Index(value, trimmed)
		needle := "{ " + strconv.Quote(value) + " }"
		if strings.Count(chunks[i], needle) != 1 {
			return fmt.Errorf("text binding mismatch")
		}
		chunks[i] = strings.Replace(chunks[i], needle, "{ "+strconv.Quote(value[:at])+" + "+expr+" + "+strconv.Quote(value[at+len(trimmed):])+" }", 1)
		return nil
	}
	if bindAttr(links[0], "href", "#", "item.URL") != nil || bindAttr(formStart, "action", "#", "data.Submit.Action") != nil || bindText(textIndexes[0], "data.Label") != nil || bindText(textIndexes[1], "item.Label") != nil || bindText(textIndexes[4], "data.Submit.Label") != nil {
		return fail("typed binding mismatch")
	}
	// Explicit app-workflow changes, not modifications of the constant reference.
	chunks[buttons[0]] = strings.Replace(chunks[buttons[0]], "<button", `<button type="button"`, 1)
	chunks[buttons[1]] = strings.Replace(chunks[buttons[1]], "<button", `<button disabled?={ data.Submit.Disabled }`, 1)
	chunks[formStart] += "<input type=\"hidden\" name={ data.Submit.CSRFName } value={ data.Submit.CSRFToken }/>\n"
	var out strings.Builder
	if strings.Count(base.Artifact.Content, "templ Specimen()") != 1 {
		return fail("reference declaration mismatch")
	}
	out.WriteString(strings.Replace(base.Artifact.Content, "templ Specimen()", "templ MenuReference()", 1))
	out.WriteString("\ntempl menu(data Menu) {\n")
	for i := 0; i < len(chunks); i++ {
		if i == links[0] {
			out.WriteString("for i, item := range data.Links {\nif i > 0 {\n{ " + strconv.Quote(separator) + " }\n}\n@menuLink(item)\n}\n")
			i = linkEnds[2]
			continue
		}
		if i == formStart {
			out.WriteString("if data.Submit != nil {\n")
		}
		out.WriteString(chunks[i])
		if i == formEnd {
			out.WriteString("}\n")
		}
	}
	out.WriteString("}\n\ntempl menuLink(item MenuLink) {\n")
	for i := links[0]; i <= linkEnds[0]; i++ {
		out.WriteString(chunks[i])
	}
	out.WriteString("}\n\ntempl menuEnhanced(content templ.Component) {\n<div data-appkit-menu>\n@content\n</div>\n}\n")
	defaultLinks := make([]map[string]string, 3)
	for i := range defaultLinks {
		defaultLinks[i] = map[string]string{"Label": strings.TrimSpace(tokens[textIndexes[i+1]].Data), "URL": ""}
	}
	defaults, err := json.Marshal(map[string]any{"Label": strings.TrimSpace(tokens[textIndexes[0]].Data), "Links": defaultLinks,
		"Submit": map[string]any{"Label": strings.TrimSpace(tokens[textIndexes[4]].Data), "Action": "", "CSRFName": "", "CSRFToken": "", "Disabled": false}})
	if err != nil {
		return fail("defaults encoding failed")
	}
	inputs := make([][]byte, 4)
	h := sha256.New()
	for i, name := range []string{"menu.go", "menu_model.go.txt", "menu.cue", "menu_styles.css"} {
		inputs[i], err = menuImplementation.ReadFile(name)
		if err != nil {
			return fail("implementation unavailable")
		}
		fmt.Fprintf(h, "%d:", len(inputs[i]))
		_, _ = h.Write(inputs[i])
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	result := MenuArtifact{SchemaVersion: 1, Adapter: "p07-menu-v1", Source: source, Syntax: base.Transformer, Runtime: base.Runtime,
		RuleSHA256: hex.EncodeToString(h.Sum(nil)), SyntaxSHA256: base.Transformer.SHA256, TemplSHA256: digest([]byte(out.String())), ModelSHA256: digest(inputs[1]), DefaultsSHA256: digest(defaults), CSSSHA256: digest(inputs[3]),
		Templ: out.String(), Model: string(inputs[1]), CSS: string(inputs[3]), Defaults: defaults, Verification: "not-verified"}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fail("envelope encoding failed")
	}
	ctx := cuecontext.New()
	if err := ctx.CompileBytes(inputs[2]).LookupPath(cue.ParsePath("#MenuArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return fail("generated envelope failed schema")
	}
	return result, nil
}
