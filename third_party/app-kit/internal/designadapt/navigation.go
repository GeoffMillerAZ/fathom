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

const NavigationArchive = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"
const NavigationMember = "html/navigation/tabs/01-tabs-with-underline.html"
const NavigationSourceSHA = "01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31"

//go:embed navigation.go navigation_model.go.txt navigation.cue
var navigationImplementation embed.FS

// NavigationArtifact records source-specific generation, never target certification.
type NavigationArtifact struct {
	SchemaVersion  int                       `json:"schema_version"`
	Adapter        string                    `json:"adapter"`
	Source         designconvert.Source      `json:"source"`
	Syntax         designconvert.Transformer `json:"syntax"`
	RuleSHA256     string                    `json:"rule_sha256"`
	SyntaxSHA256   string                    `json:"syntax_sha256"`
	TemplSHA256    string                    `json:"templ_sha256"`
	ModelSHA256    string                    `json:"model_sha256"`
	DefaultsSHA256 string                    `json:"defaults_sha256"`
	Templ          string                    `json:"templ"`
	Model          string                    `json:"model"`
	Defaults       json.RawMessage           `json:"defaults"`
	Verification   string                    `json:"verification"`
}

// AdaptNavigation requires independently checked archive membership/access first.
func AdaptNavigation(source designconvert.Source, body []byte) (NavigationArtifact, error) {
	if source.ArchiveID != NavigationArchive || source.Path != NavigationMember || source.SHA256 != NavigationSourceSHA {
		return NavigationArtifact{}, fmt.Errorf("navigation adapter: unsupported source identity")
	}
	return adaptNavigation(source, body)
}

func VerifyNavigationReplay(artifact NavigationArtifact, body []byte) error {
	if len(artifact.Templ) > 1<<20 || len(artifact.Model) > 32768 || len(artifact.Defaults) > 32768 {
		return fmt.Errorf("navigation adapter: artifact exceeds replay bounds")
	}
	want, err := AdaptNavigation(artifact.Source, body)
	if err != nil {
		return err
	}
	a, err1 := json.Marshal(artifact)
	b, err2 := json.Marshal(want)
	if err1 != nil || err2 != nil || !bytes.Equal(a, b) {
		return fmt.Errorf("navigation adapter: replay mismatch")
	}
	return nil
}

func adaptNavigation(source designconvert.Source, body []byte) (NavigationArtifact, error) {
	fail := func(reason string) (NavigationArtifact, error) {
		return NavigationArtifact{}, fmt.Errorf("navigation adapter: %s", reason)
	}
	base, err := designconvert.Translate(source, body)
	if err != nil || base.Status != "translated" || base.Artifact == nil {
		return fail("source did not pass syntax translation")
	}
	type span struct {
		start, end  int
		text, class string
		current     bool
	}
	var options, links []span
	var chunks, shape []string
	var selectLabel, navLabel, selectClass, navClass string
	mobileStart, mobileEnd, divDepth := -1, -1, 0
	currentTag := ""
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
		m := base.Artifact.Mappings[i]
		chunk, t := base.Artifact.Content[m.OutputStart:m.OutputEnd], z.Token()
		attrs := map[string]string{}
		for _, a := range t.Attr {
			attrs[a.Key] = a.Val
		}
		bind := func(key, expression string) bool {
			v, ok := attrs[key]
			needle := key + "={ " + strconv.Quote(v) + " }"
			if !ok || strings.Count(chunk, needle) != 1 {
				return false
			}
			chunk = strings.Replace(chunk, needle, key+"={ "+expression+" }", 1)
			return true
		}
		switch kind {
		case html.StartTagToken, html.SelfClosingTagToken:
			shape = append(shape, t.Data)
			currentTag = t.Data
			switch t.Data {
			case "div":
				divDepth++
				if divDepth == 2 && mobileStart < 0 {
					mobileStart = i
				}
			case "select":
				if len(attrs) != 2 || selectLabel != "" {
					return fail("select contract")
				}
				selectLabel, selectClass = attrs["aria-label"], attrs["class"]
				if !bind("aria-label", "data.SelectLabel") || !bind("class", "navigationSelectStyle(data.Mode)") {
					return fail("select binding")
				}
				chunk = strings.Replace(chunk, ">", ` if data.Mode == "get" { name={ data.Form.FieldName } }>`, 1)
			case "option":
				_, selected := attrs["selected"]
				if len(attrs) != 0 && !(selected && len(attrs) == 1 && attrs["selected"] == "") {
					return fail("option attributes")
				}
				options = append(options, span{start: i, current: selected})
				chunk = `<option selected?={ current } if data.Mode == "get" { value={ item.Key } }>` + "\n"
			case "nav":
				if len(attrs) != 2 || navLabel != "" {
					return fail("nav contract")
				}
				navLabel, navClass = attrs["aria-label"], attrs["class"]
				if !bind("aria-label", "data.NavLabel") || !bind("class", "navigationNavStyle(data.Mode)") {
					return fail("nav binding")
				}
			case "a":
				current := attrs["aria-current"] == "page"
				if attrs["href"] != "#" || len(attrs) != 2 && !(current && len(attrs) == 3) {
					return fail("link attributes")
				}
				links = append(links, span{start: i, current: current, class: attrs["class"]})
				if !bind("href", "templ.SafeURL(item.URL)") || !bind("class", "navigationLinkClass(current, mode)") {
					return fail("link binding")
				}
				if current {
					chunk = strings.Replace(chunk, ` aria-current={ "page" }`, "", 1)
				}
				chunk = strings.Replace(chunk, ">", ` if current { aria-current={ "page" } }>`, 1)
			case "svg":
				if attrs["aria-hidden"] != "true" {
					return fail("icon accessibility")
				}
			}
		case html.EndTagToken:
			shape = append(shape, "/"+t.Data)
			currentTag = ""
			switch t.Data {
			case "div":
				if divDepth == 2 && mobileEnd < 0 {
					mobileEnd = i
				}
				divDepth--
			case "option":
				if len(options) == 0 {
					return fail("option range")
				}
				options[len(options)-1].end = i
			case "a":
				if len(links) == 0 {
					return fail("link range")
				}
				links[len(links)-1].end = i
			}
		case html.TextToken:
			if strings.TrimSpace(t.Data) != "" {
				var target *span
				if currentTag == "option" && len(options) > 0 {
					target = &options[len(options)-1]
				}
				if currentTag == "a" && len(links) > 0 {
					target = &links[len(links)-1]
				}
				if target == nil || target.text != "" {
					return fail("unexpected text")
				}
				target.text = t.Data
				chunk = "{ item.Label }\n"
			}
		}
		chunks = append(chunks, chunk)
	}
	wantShape := "div div select option /option option /option option /option option /option /select svg path /svg /div div div nav a /a a /a a /a a /a /nav /div /div /div"
	if len(chunks) != len(base.Artifact.Mappings) || strings.Join(shape, " ") != wantShape || len(options) != 4 || len(links) != 4 || mobileStart < 0 || mobileEnd < mobileStart || selectLabel == "" || navLabel == "" {
		return fail("unreviewed navigation structure")
	}
	type item struct{ Key, Label, URL string }
	defaults := struct {
		Items                             []item
		CurrentKey, NavLabel, SelectLabel string
	}{NavLabel: navLabel, SelectLabel: selectLabel}
	defaultClass, currentClass := "", ""
	for i := range options {
		if options[i].text == "" || options[i].text != links[i].text || options[i].current != links[i].current {
			return fail("select/link divergence")
		}
		key := fmt.Sprintf("page-%d", i+1)
		defaults.Items = append(defaults.Items, item{key, options[i].text, "#"})
		if links[i].current {
			if defaults.CurrentKey != "" {
				return fail("multiple current items")
			}
			defaults.CurrentKey, currentClass = key, links[i].class
		} else {
			if defaultClass != "" && defaultClass != links[i].class {
				return fail("inconsistent default classes")
			}
			defaultClass = links[i].class
		}
	}
	if defaults.CurrentKey == "" || currentClass == "" || currentClass == defaultClass {
		return fail("missing current styling")
	}
	for _, c := range []string{defaultClass, currentClass} {
		if strings.Count(" "+c+" ", " whitespace-nowrap ") != 1 {
			return fail("unreviewed link wrapping class")
		}
	}
	// Whitespace-only inter-item separators are source data, not normalized away.
	separator := func(parts []span) (string, bool) {
		var first string
		for i := 1; i < len(parts); i++ {
			start := base.Artifact.Mappings[parts[i-1].end].SourceEnd
			end := base.Artifact.Mappings[parts[i].start].SourceStart
			s := string(body[start:end])
			if strings.TrimSpace(s) != "" || i > 1 && s != first {
				return "", false
			}
			first = s
		}
		return first, true
	}
	optionSep, ok1 := separator(options)
	linkSep, ok2 := separator(links)
	if !ok1 || !ok2 {
		return fail("nonuniform item separators")
	}
	section := func(start, end int, optionsMode bool) string {
		parts, call := links, "@navigationItems(data, false)\n"
		if optionsMode {
			parts, call = options, "@navigationItems(data, true)\n"
		}
		var b strings.Builder
		for i := start; i <= end; i++ {
			if i == parts[0].start {
				b.WriteString(call)
				i = parts[len(parts)-1].end
			} else {
				b.WriteString(chunks[i])
			}
		}
		return b.String()
	}
	output := "package converted\n\ntempl pageNavigation(data navigationView) {\n" + strings.Join(chunks[:mobileStart], "") + "@navigationNarrow(data)\n" + section(mobileEnd+1, len(chunks)-1, false) + "}\n"
	output += "\ntempl navigationMobile(data navigationView) {\n" + section(mobileStart, mobileEnd, true) + "}\n"
	output += "\ntempl navigationOption(item NavigationItem, current bool, data navigationView) {\n" + strings.Join(chunks[options[0].start:options[0].end+1], "") + "}\n"
	output += "\ntempl navigationLink(item NavigationItem, current bool, mode string) {\n" + strings.Join(chunks[links[0].start:links[0].end+1], "") + "}\n"
	output += `
templ navigationGETForm(data navigationView) {
 <form action={ templ.SafeURL(data.Form.Action) } method="get" class="sm:hidden">
  @navigationMobile(data)
  <button type="submit" class="mt-2 min-h-11 min-w-11 rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white [overflow-wrap:anywhere] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600 dark:bg-indigo-500 dark:focus-visible:outline-indigo-400">{ data.Form.SubmitLabel }</button>
 </form>
}
templ navigationMobileLinks(data navigationView) {
 <nav aria-label={ data.NavLabel } class="flex flex-wrap gap-2 sm:hidden">
  @navigationItems(data, false)
 </nav>
}
`
	model, err := navigationImplementation.ReadFile("navigation_model.go.txt")
	if err != nil {
		return fail("model input")
	}
	rule, err := navigationImplementation.ReadFile("navigation.go")
	if err != nil {
		return fail("rule input")
	}
	schema, err := navigationImplementation.ReadFile("navigation.cue")
	if err != nil {
		return fail("schema input")
	}
	h := sha256.New()
	for _, input := range [][]byte{rule, model, schema} {
		fmt.Fprintf(h, "%d:", len(input))
		_, _ = h.Write(input)
	}
	for _, p := range [][2]string{{"DefaultClass", defaultClass}, {"CurrentClass", currentClass}, {"SelectClass", selectClass}, {"NavClass", navClass}, {"OptionSeparator", optionSep}, {"LinkSeparator", linkSep}} {
		model = append(model, []byte("\nconst navigation"+p[0]+" = "+strconv.Quote(p[1])+"\n")...)
	}
	defaultsJSON, err := json.Marshal(defaults)
	if err != nil {
		return fail("defaults encoding")
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	artifact := NavigationArtifact{SchemaVersion: 1, Adapter: "p06-page-navigation-v1", Source: source, Syntax: base.Transformer, RuleSHA256: hex.EncodeToString(h.Sum(nil)), SyntaxSHA256: base.Transformer.SHA256, TemplSHA256: digest([]byte(output)), ModelSHA256: digest(model), DefaultsSHA256: digest(defaultsJSON), Templ: output, Model: string(model), Defaults: defaultsJSON, Verification: "not-verified"}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return fail("artifact encoding")
	}
	ctx := cuecontext.New()
	if err := ctx.CompileBytes(schema).LookupPath(cue.ParsePath("#NavigationArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return fail("generated envelope failed schema")
	}
	return artifact, nil
}
