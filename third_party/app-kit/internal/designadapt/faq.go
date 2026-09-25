// Package designadapt builds deliberately source-specific private adaptations.
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

const FAQArchive = "marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675"
const FAQMember = "html/sections/faq-sections/02-centered-accordion.html"
const FAQSourceSHA = "9114b0b981055638eb28fede17eff7bc49c91f40fc32ce758514bfdb7d19f937"
const faqSourceSize = 9173

//go:embed faq.go faq_model.go.txt faq.cue
var faqImplementation embed.FS

// FAQArtifact records source-specific generation, never target certification.
// Its generated material remains private like the purchased source.
type FAQArtifact struct {
	SchemaVersion int                       `json:"schema_version"`
	Adapter       string                    `json:"adapter"`
	Source        designconvert.Source      `json:"source"`
	Syntax        designconvert.Transformer `json:"syntax"`
	Runtime       struct {
		SourceReference designconvert.ElementsRuntime  `json:"source_reference"`
		NativeDetails   *designconvert.ElementsRuntime `json:"native_details"`
	} `json:"runtime"`
	RuleSHA256     string          `json:"rule_sha256"`
	SyntaxSHA256   string          `json:"syntax_sha256"`
	TemplSHA256    string          `json:"templ_sha256"`
	ModelSHA256    string          `json:"model_sha256"`
	DefaultsSHA256 string          `json:"defaults_sha256"`
	CSSSHA256      string          `json:"css_sha256"`
	Templ          string          `json:"templ"`
	Model          string          `json:"model"`
	CSS            string          `json:"css"`
	Defaults       json.RawMessage `json:"defaults"`
	Verification   string          `json:"verification"`
}

// AdaptFAQ accepts only the frozen P03 source. Archive access and membership
// remain an independent caller responsibility.
func AdaptFAQ(source designconvert.Source, body []byte) (FAQArtifact, error) {
	if source.ArchiveID != FAQArchive || source.Path != FAQMember || source.SHA256 != FAQSourceSHA || source.Size != faqSourceSize {
		return FAQArtifact{}, fmt.Errorf("faq adapter: unsupported source identity")
	}
	return adaptFAQ(source, body)
}

// VerifyFAQReplay proves deterministic generation only. It does not certify
// fidelity, behavior, accessibility, delivery, or consumer adoption.
func VerifyFAQReplay(artifact FAQArtifact, body []byte) error {
	return verifyFAQReplayWith(artifact, body, AdaptFAQ)
}

func verifyFAQReplayWith(artifact FAQArtifact, body []byte, generate func(designconvert.Source, []byte) (FAQArtifact, error)) error {
	if len(artifact.Templ) > 1<<20 || len(artifact.Model) > 65536 || len(artifact.Defaults) > 131072 || len(artifact.CSS) > 16384 {
		return fmt.Errorf("faq adapter: artifact exceeds replay bounds")
	}
	want, err := generate(artifact.Source, body)
	if err != nil {
		return err
	}
	gotDefaults, wantDefaults := artifact.Defaults, want.Defaults
	if artifact.SchemaVersion != want.SchemaVersion || artifact.Adapter != want.Adapter || artifact.Source != want.Source || artifact.Syntax != want.Syntax || artifact.Runtime.SourceReference != want.Runtime.SourceReference || artifact.Runtime.NativeDetails != nil || artifact.RuleSHA256 != want.RuleSHA256 || artifact.SyntaxSHA256 != want.SyntaxSHA256 || artifact.TemplSHA256 != want.TemplSHA256 || artifact.ModelSHA256 != want.ModelSHA256 || artifact.DefaultsSHA256 != want.DefaultsSHA256 || artifact.CSSSHA256 != want.CSSSHA256 || artifact.Templ != want.Templ || artifact.Model != want.Model || artifact.CSS != want.CSS || artifact.Verification != want.Verification || !bytes.Equal(gotDefaults, wantDefaults) {
		return fmt.Errorf("faq adapter: replay mismatch")
	}
	return nil
}

type faqSourceToken struct {
	token   html.Token
	mapping designconvert.Mapping
	chunk   string
}

type faqSourceItem struct {
	start, end, question, answer, button, disclosure int
	id, key                                          string
	open                                             bool
}

func adaptFAQ(source designconvert.Source, body []byte) (FAQArtifact, error) {
	fail := func(reason string) (FAQArtifact, error) {
		return FAQArtifact{}, fmt.Errorf("faq adapter: %s", reason)
	}
	base, err := designconvert.TranslateElements(source, body)
	if err != nil || base.Status != "translated" || base.Artifact == nil {
		return fail("source did not pass Elements syntax translation")
	}
	z := html.NewTokenizer(bytes.NewReader(body))
	var tokens []faqSourceToken
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
		tokens = append(tokens, faqSourceToken{z.Token(), m, base.Artifact.Content[m.OutputStart:m.OutputEnd]})
	}
	if len(tokens) != len(base.Artifact.Mappings) {
		return fail("incomplete source map")
	}

	shape, rootStart, dlStart, dlEnd, heading := []string{}, -1, -1, -1, -1
	comments, elements := 0, 0
	var stack []string
	var items []faqSourceItem
	for i := range tokens {
		t := tokens[i].token
		switch t.Type {
		case html.CommentToken:
			if elements != 0 {
				return fail("unexpected comment placement")
			}
			comments++
		case html.StartTagToken, html.SelfClosingTagToken:
			elements++
			if rootStart < 0 {
				rootStart = i
			}
			shape = append(shape, t.Data)
			parent := ""
			if len(stack) != 0 {
				parent = stack[len(stack)-1]
			}
			if t.Data == "dl" {
				dlStart = i
			}
			if t.Data == "div" && parent == "dl" {
				items = append(items, faqSourceItem{start: i, end: -1, question: -1, answer: -1, button: -1, disclosure: -1})
			}
			if len(items) != 0 && items[len(items)-1].end < 0 {
				item := &items[len(items)-1]
				switch t.Data {
				case "button":
					item.button = i
				case "el-disclosure":
					item.disclosure = i
				}
			}
			if t.Type == html.StartTagToken {
				stack = append(stack, t.Data)
			}
		case html.EndTagToken:
			shape = append(shape, "/"+t.Data)
			if len(stack) == 0 || stack[len(stack)-1] != t.Data {
				return fail("unexpected nesting")
			}
			if t.Data == "div" && len(stack) >= 2 && stack[len(stack)-2] == "dl" && len(items) != 0 {
				items[len(items)-1].end = i
			}
			if t.Data == "dl" {
				dlEnd = i
			}
			stack = stack[:len(stack)-1]
		case html.TextToken:
			if strings.TrimSpace(t.Data) == "" {
				continue
			}
			parent := ""
			if len(stack) != 0 {
				parent = stack[len(stack)-1]
			}
			if parent == "h2" && heading < 0 {
				heading = i
				continue
			}
			if len(items) == 0 || items[len(items)-1].end >= 0 {
				return fail("unexpected text")
			}
			item := &items[len(items)-1]
			if parent == "span" && containsFAQTag(stack, "button") && item.question < 0 {
				item.question = i
			} else if parent == "p" && containsFAQTag(stack, "el-disclosure") && item.answer < 0 {
				item.answer = i
			} else {
				return fail("unexpected item text")
			}
		}
	}
	itemShape := "div dt button span /span span svg path /svg svg path /svg /span /button /dt el-disclosure dd p /p /dd /el-disclosure /div"
	wantShape := "div div div h2 /h2 dl " + strings.TrimSpace(strings.Repeat(itemShape+" ", 6)) + " /dl /div /div /div"
	if comments != 2 || len(stack) != 0 || strings.Join(shape, " ") != wantShape || len(items) != 6 || rootStart < 0 || dlStart < 0 || dlEnd < dlStart || heading < 0 {
		return fail("unreviewed FAQ structure")
	}
	if err := validateFAQAttributes(tokens, items, dlStart); err != nil {
		return fail(err.Error())
	}

	prefix := ""
	seenKeys := map[string]bool{}
	for i := range items {
		item := &items[i]
		buttonAttrs := faqAttrs(tokens[item.button].token)
		disclosureAttrs := faqAttrs(tokens[item.disclosure].token)
		id := disclosureAttrs["id"]
		cut := strings.LastIndexByte(id, '-')
		if buttonAttrs["commandfor"] != id || cut <= 0 || cut == len(id)-1 {
			return fail("disclosure identity binding")
		}
		item.id, item.key = id, id[cut+1:]
		itemPrefix := id[:cut]
		if prefix == "" {
			prefix = itemPrefix
		}
		if itemPrefix != prefix || !faqASCIIIdentity(itemPrefix, true) || !faqASCIIIdentity(item.key, false) || seenKeys[item.key] {
			return fail("unreviewed disclosure identity")
		}
		seenKeys[item.key] = true
		_, hidden := disclosureAttrs["hidden"]
		item.open = !hidden
		if item.open != (i == 0) || item.question < 0 || item.answer < 0 || item.button < 0 || item.disclosure < 0 {
			return fail("unreviewed default state or fields")
		}
		if i > 0 && !sameFAQItem(tokens[items[0].start:items[0].end+1], tokens[item.start:item.end+1]) {
			return fail("FAQ item variants require review")
		}
	}
	separator := ""
	for i := 1; i < len(items); i++ {
		start := tokens[items[i-1].end].mapping.SourceEnd
		end := tokens[items[i].start].mapping.SourceStart
		candidate := string(body[start:end])
		if strings.TrimSpace(candidate) != "" || i > 1 && candidate != separator {
			return fail("nonuniform item separators")
		}
		separator = candidate
	}

	bindText := func(index int, expression string) (string, error) {
		t := tokens[index].token
		needle := "{ " + strconv.Quote(t.Data) + " }"
		if strings.Count(tokens[index].chunk, needle) != 1 {
			return "", fmt.Errorf("text mapping mismatch")
		}
		return strings.Replace(tokens[index].chunk, needle, "{ "+expression+" }", 1), nil
	}
	chunks := make([]string, len(tokens))
	for i := range tokens {
		chunks[i] = tokens[i].chunk
	}
	chunks[heading], err = bindText(heading, "data.Heading")
	if err != nil {
		return fail(err.Error())
	}
	first := items[0]
	chunks[first.question], err = bindText(first.question, "item.Question")
	if err != nil {
		return fail(err.Error())
	}
	chunks[first.answer], err = bindText(first.answer, "item.Answer")
	if err != nil {
		return fail(err.Error())
	}
	chunks[first.button], err = faqBindAttribute(chunks[first.button], "commandfor", first.id, "faqPanelID(data.IDPrefix, item.Key)")
	if err != nil {
		return fail(err.Error())
	}
	chunks[first.disclosure], err = faqBindAttribute(chunks[first.disclosure], "id", first.id, "faqPanelID(data.IDPrefix, item.Key)")
	if err != nil {
		return fail(err.Error())
	}
	chunks[first.disclosure], err = faqConditionalAttribute(chunks[first.disclosure], "hidden", "!item.Open")
	if err != nil {
		return fail(err.Error())
	}

	var reference strings.Builder
	reference.WriteString("package converted\n\ntempl faqReference(data FAQ) {\n")
	for i := 0; i < len(chunks); i++ {
		if i == first.start {
			reference.WriteString("@faqReferenceItems(data)\n")
			i = items[len(items)-1].end
		} else {
			reference.WriteString(chunks[i])
		}
	}
	reference.WriteString("}\n\ntempl faqReferenceItem(data FAQ, item FAQItem) {\n")
	for i := first.start; i <= first.end; i++ {
		reference.WriteString(chunks[i])
	}
	reference.WriteString("}\n")

	details, err := buildFAQDetails(tokens, items[0], rootStart, dlStart, dlEnd, heading)
	if err != nil {
		return fail(err.Error())
	}
	reference.WriteString(details)

	type defaultItem struct {
		Key, Question, Answer string
		Open                  bool
	}
	defaults := struct {
		IDPrefix, Heading string
		Items             []defaultItem
	}{IDPrefix: prefix, Heading: tokens[heading].token.Data}
	for _, item := range items {
		defaults.Items = append(defaults.Items, defaultItem{item.key, tokens[item.question].token.Data, tokens[item.answer].token.Data, item.open})
	}
	defaultsJSON, err := json.Marshal(defaults)
	if err != nil {
		return fail("defaults encoding")
	}
	model, err := faqImplementation.ReadFile("faq_model.go.txt")
	if err != nil {
		return fail("model input")
	}
	model = append(model, []byte("\nconst faqReferenceSeparator = "+strconv.Quote(separator)+"\n")...)
	rule, err := faqImplementation.ReadFile("faq.go")
	if err != nil {
		return fail("rule input")
	}
	schema, err := faqImplementation.ReadFile("faq.cue")
	if err != nil {
		return fail("schema input")
	}
	h := sha256.New()
	for _, input := range [][]byte{rule, model[:len(model)-len("\nconst faqReferenceSeparator = "+strconv.Quote(separator)+"\n")], schema} {
		fmt.Fprintf(h, "%d:", len(input))
		_, _ = h.Write(input)
	}
	css := faqNativeCSS
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	artifact := FAQArtifact{
		SchemaVersion: 1, Adapter: "p03-faq-v1", Source: source, Syntax: base.Transformer,
		RuleSHA256: hex.EncodeToString(h.Sum(nil)), SyntaxSHA256: base.Transformer.SHA256,
		TemplSHA256: digest([]byte(reference.String())), ModelSHA256: digest(model), DefaultsSHA256: digest(defaultsJSON), CSSSHA256: digest([]byte(css)),
		Templ: reference.String(), Model: string(model), CSS: css, Defaults: defaultsJSON, Verification: "not-verified",
	}
	artifact.Runtime.SourceReference = base.Runtime
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return fail("artifact encoding")
	}
	ctx := cuecontext.New()
	if err := ctx.CompileBytes(schema).LookupPath(cue.ParsePath("#FAQArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return fail("generated envelope failed schema")
	}
	return artifact, nil
}

func containsFAQTag(stack []string, want string) bool {
	for _, tag := range stack {
		if tag == want {
			return true
		}
	}
	return false
}

func faqAttrs(t html.Token) map[string]string {
	attrs := make(map[string]string, len(t.Attr))
	for _, attr := range t.Attr {
		attrs[attr.Key] = attr.Val
	}
	return attrs
}

func faqAttrKeys(t html.Token) string {
	keys := make([]string, len(t.Attr))
	for i, attr := range t.Attr {
		keys[i] = attr.Key
	}
	return strings.Join(keys, " ")
}

func validateFAQAttributes(tokens []faqSourceToken, items []faqSourceItem, dlStart int) error {
	for i := range tokens {
		t := tokens[i].token
		if t.Type != html.StartTagToken && t.Type != html.SelfClosingTagToken {
			continue
		}
		want := ""
		switch t.Data {
		case "div", "h2", "dl", "span", "dd", "p":
			want = "class"
		case "dt":
			want = ""
		case "button":
			want = "type command commandfor class"
		case "el-disclosure":
			want = "id class"
			if _, ok := faqAttrs(t)["hidden"]; ok {
				want = "id hidden class"
			}
		case "svg":
			want = "viewbox fill stroke stroke-width data-slot aria-hidden class"
		case "path":
			want = "d stroke-linecap stroke-linejoin"
		default:
			return fmt.Errorf("unsupported FAQ element")
		}
		if faqAttrKeys(t) != want {
			return fmt.Errorf("unreviewed %s attributes", t.Data)
		}
		attrs := faqAttrs(t)
		if t.Data != "dt" && t.Data != "path" && t.Data != "svg" && attrs["class"] == "" {
			return fmt.Errorf("missing reviewed class")
		}
		if t.Data == "button" && (attrs["type"] != "button" || attrs["command"] != "--toggle") {
			return fmt.Errorf("unreviewed disclosure command")
		}
		if t.Data == "svg" && attrs["aria-hidden"] != "true" {
			return fmt.Errorf("icon accessibility")
		}
	}
	if tokens[dlStart].token.Data != "dl" {
		return fmt.Errorf("missing FAQ list")
	}
	return nil
}

func sameFAQItem(a, b []faqSourceToken) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i].token, b[i].token
		if x.Type != y.Type || x.Type != html.TextToken && x.Data != y.Data || len(x.Attr) != len(y.Attr) && x.Data != "el-disclosure" {
			return false
		}
		if x.Type == html.TextToken {
			if strings.TrimSpace(x.Data) == "" && x.Data != y.Data || strings.TrimSpace(x.Data) != "" && strings.TrimSpace(y.Data) == "" {
				return false
			}
			continue
		}
		normalize := func(t html.Token) string {
			var out strings.Builder
			for _, attr := range t.Attr {
				if attr.Key == "hidden" {
					continue
				}
				value := attr.Val
				if attr.Key == "id" || attr.Key == "commandfor" {
					value = "<id>"
				}
				out.WriteString(attr.Namespace + "\x00" + attr.Key + "\x00" + value + "\x00")
			}
			return out.String()
		}
		if normalize(x) != normalize(y) {
			return false
		}
	}
	return true
}

func faqASCIIIdentity(s string, prefix bool) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i, c := range []byte(s) {
		letter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		digit := c >= '0' && c <= '9'
		if prefix && i == 0 && !letter || !letter && !digit && c != '_' && !(prefix && c == '-') {
			return false
		}
	}
	return true
}

func faqBindAttribute(chunk, key, value, expression string) (string, error) {
	needle := " " + key + "={ " + strconv.Quote(value) + " }"
	if strings.Count(chunk, needle) != 1 {
		return "", fmt.Errorf("%s binding mismatch", key)
	}
	return strings.Replace(chunk, needle, " "+key+"={ "+expression+" }", 1), nil
}

func faqConditionalAttribute(chunk, key, expression string) (string, error) {
	needle := " " + key + "={ \"\" }"
	if strings.Contains(chunk, needle) {
		return strings.Replace(chunk, needle, " "+key+"?={ "+expression+" }", 1), nil
	}
	at := strings.Index(chunk, " class=")
	if at < 0 {
		at = strings.LastIndex(chunk, ">")
	}
	if at < 0 {
		return "", fmt.Errorf("%s insertion mismatch", key)
	}
	return chunk[:at] + " " + key + "?={ " + expression + " }" + chunk[at:], nil
}

func faqTemplAttrs(attrs []html.Attribute, classFilter bool) string {
	var out strings.Builder
	for _, attr := range attrs {
		value := attr.Val
		if classFilter && attr.Key == "class" {
			var kept []string
			for _, field := range strings.Fields(value) {
				if field != "hidden" && !strings.Contains(field, "aria-expanded") {
					kept = append(kept, field)
				}
			}
			value = strings.Join(kept, " ")
		}
		fmt.Fprintf(&out, " %s={ %s }", attr.Key, strconv.Quote(value))
	}
	return out.String()
}

func buildFAQDetails(tokens []faqSourceToken, item faqSourceItem, rootStart, dlStart, dlEnd, heading int) (string, error) {
	bindHeading := tokens[heading].chunk
	needle := "{ " + strconv.Quote(tokens[heading].token.Data) + " }"
	if strings.Count(bindHeading, needle) != 1 {
		return "", fmt.Errorf("native heading mapping mismatch")
	}
	bindHeading = strings.Replace(bindHeading, needle, "{ data.Heading }", 1)
	var out strings.Builder
	out.WriteString("\ntempl faqDetails(data FAQ) {\n")
	for i := rootStart; i < dlStart; i++ {
		if i == rootStart {
			root := tokens[i].token
			out.WriteString("<" + root.Data + " data-appkit-faq-root={ \"\" }" + faqTemplAttrs(root.Attr, false) + ">\n")
		} else if i == heading {
			out.WriteString(bindHeading)
		} else {
			out.WriteString(tokens[i].chunk)
		}
	}
	dl := tokens[dlStart].token
	out.WriteString("<div data-appkit-faq-details={ \"\" }" + faqTemplAttrs(dl.Attr, false) + ">\n@faqDetailsItems(data)\n</div>\n")
	for i := dlEnd + 1; i < len(tokens); i++ {
		out.WriteString(tokens[i].chunk)
	}
	out.WriteString("}\n\ntempl faqDetailsItem(data FAQ, item FAQItem) {\n")

	var starts []int
	for i := item.start; i <= item.end; i++ {
		t := tokens[i].token
		if t.Type == html.StartTagToken || t.Type == html.SelfClosingTagToken {
			starts = append(starts, i)
		}
	}
	if len(starts) != 12 {
		return "", fmt.Errorf("native item source map mismatch")
	}
	itemDiv, button := tokens[starts[0]].token, tokens[starts[2]].token
	question, iconSpan := tokens[starts[3]].token, tokens[starts[4]].token
	closedSVG, closedPath := tokens[starts[5]].token, tokens[starts[6]].token
	openSVG, openPath := tokens[starts[7]].token, tokens[starts[8]].token
	disclosure, dd, p := tokens[starts[9]].token, tokens[starts[10]].token, tokens[starts[11]].token
	out.WriteString("<details open?={ item.Open }" + faqTemplAttrs(itemDiv.Attr, false) + ">\n")
	buttonAttrs := make([]html.Attribute, 0, len(button.Attr))
	for _, attr := range button.Attr {
		if attr.Key == "class" {
			buttonAttrs = append(buttonAttrs, attr)
		}
	}
	out.WriteString("<summary" + faqTemplAttrs(buttonAttrs, false) + ">\n")
	out.WriteString("<span" + faqTemplAttrs(question.Attr, false) + ">{ item.Question }</span>\n")
	out.WriteString("<span" + faqTemplAttrs(iconSpan.Attr, false) + ">\n")
	out.WriteString("<svg" + faqTemplAttrs(closedSVG.Attr, true) + " data-appkit-faq-icon={ \"closed\" }><path" + faqTemplAttrs(closedPath.Attr, false) + "/></svg>\n")
	out.WriteString("<svg" + faqTemplAttrs(openSVG.Attr, true) + " data-appkit-faq-icon={ \"open\" }><path" + faqTemplAttrs(openPath.Attr, false) + "/></svg>\n")
	out.WriteString("</span>\n</summary>\n")
	disclosureAttrs := make([]html.Attribute, 0, len(disclosure.Attr))
	for _, attr := range disclosure.Attr {
		if attr.Key == "class" {
			disclosureAttrs = append(disclosureAttrs, attr)
		}
	}
	out.WriteString("<div id={ faqPanelID(data.IDPrefix, item.Key) }" + faqTemplAttrs(disclosureAttrs, false) + ">\n")
	out.WriteString("<div" + faqTemplAttrs(dd.Attr, false) + "><p" + faqTemplAttrs(p.Attr, false) + ">{ item.Answer }</p></div>\n")
	out.WriteString("</div>\n</details>\n}\n")
	return out.String(), nil
}

const faqNativeCSS = `[data-appkit-faq-root] h2 { overflow-wrap: anywhere; }
[data-appkit-faq-details] > details > summary {
  min-height: 44px;
  cursor: pointer;
  overflow-wrap: anywhere;
}
[data-appkit-faq-details] > details > summary::-webkit-details-marker { display: none; }
[data-appkit-faq-details] > details > summary::marker { content: ""; }
[data-appkit-faq-details] > details > summary:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: 2px;
  border-radius: 0.25rem;
}
[data-appkit-faq-details] > details > summary > span:first-child,
[data-appkit-faq-details] > details > div { min-width: 0; overflow-wrap: anywhere; }
[data-appkit-faq-details] [data-appkit-faq-icon] { flex: none; }
[data-appkit-faq-details] [data-appkit-faq-icon="open"] { display: none; }
[data-appkit-faq-details] > details[open] [data-appkit-faq-icon="closed"] { display: none; }
[data-appkit-faq-details] > details[open] [data-appkit-faq-icon="open"] { display: block; }
`
