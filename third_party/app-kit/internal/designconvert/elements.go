package designconvert

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	templparser "github.com/a-h/templ/parser/v2"
	"golang.org/x/net/html"
)

const (
	ElementsRuleID            = "html-elements-to-templ-v1"
	ElementsSyntaxKind        = "elements-syntax"
	ElementsRuntimePackage    = "@tailwindplus/elements"
	ElementsRuntimeVersion    = "1.0.22"
	ElementsRuntimeESMSHA256  = "120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013"
	ElementsRuntimeTGZSHA256  = "fc684f17d960fb60dadfdf9a0b49ba8fd295af4b8fd1715a26cd20b832ea245b"
	ElementsRuntimeDelivery   = "host-provided-module"
	maxElementsKindBytes      = 64
	maxElementsRuntimeBytes   = 256
	maxElementsReceiptStrByte = 1024
)

//go:embed elements.go elements_schema.cue
var elementsImplementation embed.FS

// ElementsRuntime identifies the host-supplied commercial runtime needed by
// the emitted custom elements. Translation neither embeds nor installs it.
type ElementsRuntime struct {
	Package   string `json:"package"`
	Version   string `json:"version"`
	ESMSHA256 string `json:"esm_sha256"`
	TGZSHA256 string `json:"tgz_sha256"`
	Delivery  string `json:"delivery"`
}

// ElementsTranslation is a syntax-only, source-bound receipt. In particular,
// its runtime pin is dependency metadata rather than behavior verification.
type ElementsTranslation struct {
	SchemaVersion      int             `json:"schema_version"`
	Kind               string          `json:"kind"`
	Source             Source          `json:"source"`
	Target             string          `json:"target"`
	Transformer        Transformer     `json:"transformer"`
	Runtime            ElementsRuntime `json:"runtime"`
	Status             string          `json:"status"`
	Artifact           *Artifact       `json:"artifact,omitempty"`
	Diagnostics        []Diagnostic    `json:"diagnostics"`
	OmittedDiagnostics int             `json:"omitted_diagnostics"`
	Verification       string          `json:"verification"`
}

type elementsCommand struct {
	command string
	ref     string
	offset  int
}

// TranslateElements translates only the named Tailwind Plus Elements family
// and the existing literal HTML/basic-SVG grammar. It performs no I/O, runtime
// installation, code execution, compilation, behavior mapping or verification.
func TranslateElements(source Source, body []byte) (ElementsTranslation, error) {
	if err := validateSource(source, body); err != nil {
		return ElementsTranslation{}, err
	}
	result, err := translateElements(source, body)
	if err != nil {
		return ElementsTranslation{}, err
	}
	if err := validateElementsShape(result); err != nil {
		return ElementsTranslation{}, err
	}
	return result, nil
}

func translateElements(source Source, body []byte) (ElementsTranslation, error) {
	result := ElementsTranslation{
		SchemaVersion: 1,
		Kind:          ElementsSyntaxKind,
		Source:        source,
		Target:        "go-templ",
		Transformer: Transformer{
			ID:            ElementsRuleID,
			SHA256:        elementsRuleDigest(),
			GoVersion:     runtime.Version(),
			HTMLParser:    HTMLParserVersion,
			RequiredTempl: RequiredTempl,
		},
		Runtime: ElementsRuntime{
			Package:   ElementsRuntimePackage,
			Version:   ElementsRuntimeVersion,
			ESMSHA256: ElementsRuntimeESMSHA256,
			TGZSHA256: ElementsRuntimeTGZSHA256,
			Delivery:  ElementsRuntimeDelivery,
		},
		Status:       "unsupported",
		Diagnostics:  []Diagnostic{},
		Verification: "not-verified",
	}
	if path.Ext(source.Path) != ".html" {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Code: "source-language", Offset: 0,
			Message: "This rule accepts official HTML fragments using the bounded Elements grammar only.",
		})
		return result, nil
	}
	converted, err := elementsHTMLToTempl(body)
	if err != nil {
		return ElementsTranslation{}, err
	}
	result.Diagnostics = append(result.Diagnostics, converted.Diagnostics...)
	result.OmittedDiagnostics = converted.OmittedDiagnostics
	if len(result.Diagnostics) != 0 || result.OmittedDiagnostics != 0 {
		if converted.Templ != "" || len(converted.Mappings) != 0 {
			return ElementsTranslation{}, fmt.Errorf("designconvert: unsupported Elements translation produced partial output")
		}
		return result, nil
	}
	if converted.Templ == "" {
		return ElementsTranslation{}, fmt.Errorf("designconvert: Elements translator produced no output or diagnostic")
	}
	result.Status = "translated"
	result.Artifact = &Artifact{
		Filename: "snippet.templ",
		SHA256:   digest([]byte(converted.Templ)),
		Content:  converted.Templ,
		Mappings: append([]Mapping{}, converted.Mappings...),
	}
	return result, nil
}

// VerifyElementsReplay binds the supplied original to the exact current rule,
// runtime metadata, diagnostics and output. It never upgrades not-verified.
func VerifyElementsReplay(result ElementsTranslation, body []byte) error {
	if err := validateElementsShape(result); err != nil {
		return err
	}
	if err := validateSource(result.Source, body); err != nil {
		return err
	}
	expected, err := translateElements(result.Source, body)
	if err != nil {
		return err
	}
	got, err := json.Marshal(result)
	if err != nil {
		return err
	}
	want, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("designconvert: Elements replay mismatch; source, rule, runtime, toolchain, diagnostics or artifact changed")
	}
	return nil
}

func elementsHTMLToTempl(source []byte) (HTMLResult, error) {
	r := HTMLResult{Diagnostics: []Diagnostic{}, Mappings: []Mapping{}}
	add := func(code, message string, offset int) {
		if len(r.Diagnostics) == MaxDiagnostics {
			r.OmittedDiagnostics++
			return
		}
		r.Diagnostics = append(r.Diagnostics, Diagnostic{Code: code, Message: message, Offset: offset})
	}
	if len(source) > MaxSourceBytes {
		add("source-limit", "HTML source exceeds the 256 KiB translation limit.", 0)
		return r, nil
	}
	if !utf8.Valid(source) || bytes.IndexByte(source, 0) >= 0 {
		add("source-encoding", "HTML source must be UTF-8 text without NUL bytes.", 0)
		return r, nil
	}
	if invalidControl(string(source), true) {
		add("source-control", "HTML source contains unsupported control characters.", 0)
		return r, nil
	}

	z := html.NewTokenizer(bytes.NewReader(source))
	z.SetMaxBuf(MaxSourceBytes)
	parts := make([]htmlPart, 0)
	stack := make([]htmlFrame, 0)
	ids := make(map[string][]string)
	commands := make([]elementsCommand, 0)
	offset, elementCount, elementsCount := 0, 0, 0
	for {
		tt := z.Next()
		raw := bytes.Clone(z.Raw())
		start := offset
		offset += len(raw)
		if tt == html.ErrorToken {
			if z.Err() != io.EOF || len(raw) != 0 {
				add("html-syntax", "HTML tokenization failed or ended inside a token.", start)
			}
			break
		}
		if len(parts) == maxTokens {
			add("token-limit", "HTML fragment exceeds the token limit.", start)
			break
		}
		t := z.Token()
		depth := len(stack)
		svg := len(stack) > 0 && stack[len(stack)-1].svg
		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			elementCount++
			if !validTagSyntax(raw, false) {
				add("html-syntax", "Tags require explicit well-formed syntax and quoted attribute values.", start)
			}
			if t.Data == "svg" {
				svg = true
			}
			isElements := elementsTags[t.Data] && !svg
			if isElements {
				elementsCount++
				if !exactElementsTag(raw, t.Data, false) {
					add("elements-syntax", "Elements custom tag names must use their exact lowercase spelling.", start)
				}
			}
			if !isElements && !allowedTag(t.Data, svg) {
				add("unsupported-element", "Element is outside the HTML/basic-SVG and admitted Elements grammar.", start)
			}
			if len(t.Attr) > maxAttributes {
				add("attribute-limit", "Element exceeds the attribute limit.", start)
			}
			seen := make(map[string]bool, len(t.Attr))
			attrs := make(map[string]string, len(t.Attr))
			for _, attr := range t.Attr {
				if seen[attr.Key] {
					add("duplicate-attribute", "Duplicate HTML attribute names are unsupported.", start)
				}
				seen[attr.Key] = true
				attrs[attr.Key] = attr.Val
				if !allowedElementsAttribute(t.Data, attr.Key, svg) {
					add("unsupported-attribute", "Attribute is outside the literal Elements grammar; styles, events and expression namespaces are unsupported.", start)
				}
				if invalidControl(attr.Val, false) {
					add("attribute-control", "Decoded attribute contains unsupported control characters or a carriage-return entity.", start)
				}
				if !safeElementsAttributeValue(t.Data, attr.Key, attr.Val, svg) {
					add("unsafe-attribute", "Attribute value is outside the safe literal or Elements placement grammar.", start)
				}
			}
			if id, ok := attrs["id"]; ok && id != "" {
				ids[id] = append(ids[id], t.Data)
				if len(ids[id]) > 1 {
					add("duplicate-id", "Non-empty IDs must be unique within the fragment.", start)
				}
			}
			command, hasCommand := attrs["command"]
			ref, hasRef := attrs["commandfor"]
			if hasCommand || hasRef {
				if !hasCommand || !hasRef || t.Data != "button" {
					add("command-contract", "Elements commands require command and commandfor together on a button.", start)
				} else {
					commands = append(commands, elementsCommand{command: command, ref: ref, offset: start})
				}
			}
			if tt == html.SelfClosingTagToken && !svg && !voidTags[t.Data] {
				add("html-self-closing", "Non-void HTML and Elements nodes need explicit closing tags.", start)
			}
			if !svg && invalidParent(stack, t.Data) {
				add("html-nesting", "Element nesting requires browser repair and is outside the explicit fragment grammar.", start)
			}
			if tt != html.SelfClosingTagToken && !(voidTags[t.Data] && !svg) {
				if len(stack) == maxDepth {
					add("depth-limit", "HTML fragment exceeds the nesting limit.", start)
					return r, nil
				}
				stack = append(stack, htmlFrame{name: t.Data, svg: svg})
			}
		case html.EndTagToken:
			if !validTagSyntax(raw, true) {
				add("html-syntax", "Closing tags may not carry attributes or slash modifiers.", start)
			}
			if elementsTags[t.Data] && !exactElementsTag(raw, t.Data, true) {
				add("elements-syntax", "Elements custom tag names must use their exact lowercase spelling.", start)
			}
			if len(stack) == 0 || stack[len(stack)-1].name != t.Data {
				add("html-nesting", "Closing tags must exactly match explicitly opened elements.", start)
			} else {
				stack = stack[:len(stack)-1]
				depth = len(stack)
			}
		case html.TextToken:
			if invalidControl(t.Data, false) {
				add("text-control", "Decoded text contains unsupported control characters or a carriage-return entity.", start)
			}
		case html.CommentToken:
			if !bytes.HasPrefix(raw, []byte("<!--")) || !bytes.HasSuffix(raw, []byte("-->")) ||
				strings.Contains(t.Data, "--") || strings.HasSuffix(t.Data, "-") || strings.HasPrefix(t.Data, ">") ||
				strings.HasPrefix(t.Data, "[if") || invalidControl(t.Data, false) {
				add("unsupported-comment", "Only ordinary, well-formed HTML comments are supported.", start)
			}
		default:
			add("unsupported-document", "Only HTML fragments are supported; document declarations require a separate adapter.", start)
		}
		parts = append(parts, htmlPart{token: t, start: start, end: offset, depth: depth})
	}
	if len(stack) != 0 {
		add("html-nesting", "HTML fragment ends with unclosed elements.", len(source))
	}
	if elementCount == 0 {
		add("html-fragment", "Translation requires an HTML element.", 0)
	}
	if elementsCount == 0 {
		add("elements-required", "This rule requires at least one admitted Elements custom element.", 0)
	}
	validateElementsCommands(ids, commands, add)
	if len(r.Diagnostics) != 0 {
		return r, nil
	}
	if !sameExplicitTree(source, parts) {
		add("html-tree-repair", "The HTML parser changes the explicit fragment tree; browser repair is unsupported.", 0)
		return r, nil
	}

	var out strings.Builder
	out.WriteString("package converted\n\ntempl Specimen() {\n")
	for _, p := range parts {
		begin := out.Len()
		indent := strings.Repeat("\t", p.depth+1)
		if p.token.Type != html.EndTagToken {
			fmt.Fprintf(&out, "%s// source bytes %d..%d\n", indent, p.start, p.end)
		}
		out.WriteString(indent)
		switch p.token.Type {
		case html.StartTagToken, html.SelfClosingTagToken:
			out.WriteByte('<')
			out.WriteString(p.token.Data)
			for _, attr := range p.token.Attr {
				fmt.Fprintf(&out, " %s={ %s }", attr.Key, strconv.Quote(attr.Val))
			}
			if p.token.Type == html.SelfClosingTagToken || voidTags[p.token.Data] {
				out.WriteString("/>")
			} else {
				out.WriteByte('>')
			}
		case html.EndTagToken:
			fmt.Fprintf(&out, "</%s>", p.token.Data)
		case html.TextToken:
			fmt.Fprintf(&out, "{ %s }", strconv.Quote(p.token.Data))
		case html.CommentToken:
			fmt.Fprintf(&out, "<!--%s-->", p.token.Data)
		}
		out.WriteByte('\n')
		r.Mappings = append(r.Mappings, Mapping{SourceStart: p.start, SourceEnd: p.end, OutputStart: begin, OutputEnd: out.Len()})
		if out.Len() > maxOutputBytes {
			add("output-limit", "Translated source exceeds the output limit.", p.start)
			r.Mappings = []Mapping{}
			return r, nil
		}
	}
	out.WriteString("}\n")
	if err := validateElementsTempl(out.String()); err != nil {
		return HTMLResult{}, err
	}
	r.Templ = out.String()
	return r, nil
}

var elementsTags = words(`el-disclosure el-dropdown el-menu el-dialog el-dialog-panel`)

var elementsAnchorPlacements = map[string]bool{
	"top": true, "top start": true, "top end": true,
	"bottom": true, "bottom start": true, "bottom end": true,
	"left": true, "left start": true, "left end": true,
	"right": true, "right start": true, "right end": true,
}

func allowedElementsAttribute(tag, name string, svg bool) bool {
	if name == "anchor" {
		return !svg && tag == "el-menu"
	}
	return allowedAttribute(name, svg)
}

func safeElementsAttributeValue(tag, name, value string, svg bool) bool {
	if name == "anchor" {
		return tag == "el-menu" && elementsAnchorPlacements[value]
	}
	return safeAttributeValue(tag, name, value, svg)
}

func exactElementsTag(raw []byte, name string, closing bool) bool {
	start := 1
	if closing {
		start = 2
	}
	return len(raw) >= start+len(name) && string(raw[start:start+len(name)]) == name
}

func validateElementsCommands(ids map[string][]string, commands []elementsCommand, add func(string, string, int)) {
	for _, c := range commands {
		if !validElementsIDReference(c.ref) {
			add("command-reference", "commandfor must be a plain ID reference within the fragment.", c.offset)
			continue
		}
		targets := ids[c.ref]
		if len(targets) != 1 {
			add("command-target", "commandfor must resolve to exactly one ID in the fragment.", c.offset)
			continue
		}
		allowed := c.command == "--toggle" && targets[0] == "el-disclosure" ||
			(c.command == "show-modal" || c.command == "close") && targets[0] == "dialog"
		if !allowed {
			add("command-action", "Only --toggle for el-disclosure and show-modal/close for dialog are admitted.", c.offset)
		}
	}
}

func validElementsIDReference(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_-.:", c)) {
			return false
		}
	}
	return true
}

// This guard admits the exact custom element names in addition to the legacy
// literal grammar. It permits no raw HTML, calls, imports or injected Go.
func validateElementsTempl(source string) error {
	tf, err := templparser.ParseString(source)
	if err != nil {
		return fmt.Errorf("designconvert: generated Elements templ failed parsing")
	}
	if strings.TrimSpace(tf.Package.Expression.Value) != "package converted" || len(tf.Header) != 0 || len(tf.Nodes) != 1 {
		return fmt.Errorf("designconvert: generated Elements templ has unexpected top-level nodes")
	}
	t, ok := tf.Nodes[0].(*templparser.HTMLTemplate)
	if !ok || t.Expression.Value != "Specimen()" {
		return fmt.Errorf("designconvert: generated Elements templ has an unexpected declaration")
	}
	return validateElementsTemplNodes(t.Children)
}

func validateElementsTemplNodes(nodes []templparser.Node) error {
	for _, node := range nodes {
		switch n := node.(type) {
		case *templparser.Element:
			if !htmlTags[n.Name] && !svgTags[n.Name] && !elementsTags[n.Name] {
				return fmt.Errorf("designconvert: generated Elements templ has an unexpected element")
			}
			for _, attr := range n.Attributes {
				a, ok := attr.(*templparser.ExpressionAttribute)
				if !ok {
					return fmt.Errorf("designconvert: generated Elements templ has a nonliteral attribute")
				}
				if _, ok := a.Key.(templparser.ConstantAttributeKey); !ok || !literalString(a.Expression.Value) {
					return fmt.Errorf("designconvert: generated Elements templ has a nonliteral attribute")
				}
			}
			if err := validateElementsTemplNodes(n.Children); err != nil {
				return err
			}
		case *templparser.StringExpression:
			if !literalString(n.Expression.Value) {
				return fmt.Errorf("designconvert: generated Elements templ has a nonliteral expression")
			}
		case *templparser.GoComment:
			var start, end int
			_, scanErr := fmt.Sscanf(strings.TrimSpace(n.Contents), "source bytes %d..%d", &start, &end)
			if n.Multiline || scanErr != nil ||
				start < 0 || end < start || strings.TrimSpace(n.Contents) != fmt.Sprintf("source bytes %d..%d", start, end) {
				return fmt.Errorf("designconvert: generated Elements templ has an unexpected comment")
			}
		case *templparser.HTMLComment, *templparser.Whitespace:
		default:
			return fmt.Errorf("designconvert: generated Elements templ has an unexpected executable node")
		}
	}
	return nil
}

func validateElementsShape(result ElementsTranslation) error {
	if err := validateElementsReceiptBounds(result); err != nil {
		return err
	}
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	schemaBody, err := elementsImplementation.ReadFile("elements_schema.cue")
	if err != nil {
		return err
	}
	ctx := cuecontext.New()
	schema := ctx.CompileBytes(schemaBody)
	if err := schema.Err(); err != nil {
		return fmt.Errorf("designconvert: Elements schema compile: %w", err)
	}
	value := ctx.CompileBytes(body).Unify(schema.LookupPath(cue.ParsePath("#ElementsTranslation")))
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("designconvert: Elements translation schema: %w", err)
	}
	for _, d := range result.Diagnostics {
		if d.Offset > result.Source.Size {
			return fmt.Errorf("designconvert: Elements diagnostic exceeds original source")
		}
	}
	if artifact := result.Artifact; artifact != nil {
		if artifact.SHA256 != digest([]byte(artifact.Content)) {
			return fmt.Errorf("designconvert: Elements artifact hash mismatch")
		}
		for _, m := range artifact.Mappings {
			if m.SourceEnd > result.Source.Size || m.OutputEnd > len(artifact.Content) {
				return fmt.Errorf("designconvert: Elements mapping exceeds original or artifact")
			}
		}
	}
	return nil
}

func validateElementsReceiptBounds(result ElementsTranslation) error {
	check := func(name, value string, limit int) error {
		if len(value) > limit {
			return fmt.Errorf("designconvert: Elements receipt limit: %s exceeds %d bytes", name, limit)
		}
		if !utf8.ValidString(value) {
			return fmt.Errorf("designconvert: Elements receipt encoding: %s must be UTF-8", name)
		}
		return nil
	}
	fields := []struct {
		name, value string
		limit       int
	}{
		{"kind", result.Kind, maxElementsKindBytes},
		{"source.archive_id", result.Source.ArchiveID, maxArchiveIDBytes},
		{"source.path", result.Source.Path, maxSourcePathBytes},
		{"source.sha256", result.Source.SHA256, 64},
		{"target", result.Target, 128},
		{"transformer.id", result.Transformer.ID, 128},
		{"transformer.sha256", result.Transformer.SHA256, 64},
		{"transformer.go_version", result.Transformer.GoVersion, maxElementsRuntimeBytes},
		{"transformer.html_parser", result.Transformer.HTMLParser, maxElementsRuntimeBytes},
		{"transformer.required_templ", result.Transformer.RequiredTempl, maxElementsRuntimeBytes},
		{"runtime.package", result.Runtime.Package, maxElementsRuntimeBytes},
		{"runtime.version", result.Runtime.Version, maxElementsRuntimeBytes},
		{"runtime.esm_sha256", result.Runtime.ESMSHA256, 64},
		{"runtime.tgz_sha256", result.Runtime.TGZSHA256, 64},
		{"runtime.delivery", result.Runtime.Delivery, maxElementsRuntimeBytes},
		{"status", result.Status, 32},
		{"verification", result.Verification, 32},
	}
	for _, field := range fields {
		if err := check(field.name, field.value, field.limit); err != nil {
			return err
		}
	}
	if len(result.Diagnostics) > MaxDiagnostics {
		return fmt.Errorf("designconvert: Elements receipt limit: diagnostics exceeds %d entries", MaxDiagnostics)
	}
	for _, diagnostic := range result.Diagnostics {
		if err := check("diagnostics.code", diagnostic.Code, 128); err != nil {
			return err
		}
		if err := check("diagnostics.message", diagnostic.Message, maxElementsReceiptStrByte); err != nil {
			return err
		}
	}
	if artifact := result.Artifact; artifact != nil {
		if len(artifact.Mappings) > maxTokens {
			return fmt.Errorf("designconvert: Elements receipt limit: artifact.mappings exceeds %d entries", maxTokens)
		}
		if err := check("artifact.filename", artifact.Filename, 128); err != nil {
			return err
		}
		if err := check("artifact.sha256", artifact.SHA256, 64); err != nil {
			return err
		}
		if err := check("artifact.content", artifact.Content, maxOutputBytes); err != nil {
			return err
		}
	}
	return nil
}

func elementsRuleDigest() string {
	h := sha256.New()
	for _, name := range []string{"elements.go", "elements_schema.cue"} {
		body, _ := elementsImplementation.ReadFile(name)
		fmt.Fprintf(h, "%s\x00%d\x00", name, len(body))
		h.Write(body)
	}
	fmt.Fprintf(h, "legacy-rule-digest\x00%s\x00", ruleDigest())
	return fmt.Sprintf("%x", h.Sum(nil))
}
