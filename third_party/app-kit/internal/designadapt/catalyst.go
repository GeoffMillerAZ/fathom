package designadapt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/app-kit/internal/designconvert"
	"golang.org/x/net/html"
)

// CatalystFormSourceSHA256 and CatalystFormSourceBytes identify the prepared,
// private P10 DOM capture. They are an input guard, not a delivery mechanism.
const (
	CatalystFormSourceSHA256 = "5c79ace9bb6c337a1651d615754a1c5e9cd8e9784f4de25bee389cbc9976064d"
	CatalystFormSourceBytes  = 32710
)

// AdaptCatalystForm accepts only the selected private P10 capture. It performs no
// filesystem access and does not make the generated body distributable or certified.
func AdaptCatalystForm(body []byte) (string, error) {
	if len(body) != CatalystFormSourceBytes {
		return "", fmt.Errorf("catalyst adapter: unexpected private source size")
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != CatalystFormSourceSHA256 {
		return "", fmt.Errorf("catalyst adapter: unexpected private source digest")
	}
	return adaptCatalystForm(body)
}

// These tokens are deliberately private-transform implementation details. They must
// not be present in an input capture: a collision could turn source text into code.
var catalystMarkers = []string{
	"__APPKIT_P10_FORM_ACTION__", "__APPKIT_P10_FORM_ID__", "__APPKIT_P10_CSRF__",
	"__APPKIT_P10_NAME_VALUE__", "__APPKIT_P10_BIO_VALUE__", "__APPKIT_P10_EMAIL_VALUE__",
	"__APPKIT_P10_ADDRESS_VALUE__", "__APPKIT_P10_CITY_VALUE__", "__APPKIT_P10_POSTAL_CODE_VALUE__",
	"__APPKIT_P10_COUNTRY_OPTIONS__", "__APPKIT_P10_REGION_OPTIONS__", "__APPKIT_P10_CURRENCY_OPTIONS__",
	"__APPKIT_P10_BIO_COUNT__", "__APPKIT_P10_STATUS__", "__APPKIT_P10_BIO_ERROR__",
	"__APPKIT_P10_EMAIL_ERROR__", "__APPKIT_P10_COUNTRY_ERROR__", "__APPKIT_P10_REGION_ERROR__",
	"__APPKIT_P10_CURRENCY_ERROR__", "__APPKIT_P10_NAME_ID__", "__APPKIT_P10_BIO_ID__",
	"__APPKIT_P10_EMAIL_ID__", "__APPKIT_P10_ADDRESS_ID__", "__APPKIT_P10_CITY_ID__",
	"__APPKIT_P10_POSTAL_CODE_ID__", "__APPKIT_P10_COUNTRY_ID__", "__APPKIT_P10_REGION_ID__",
	"__APPKIT_P10_CURRENCY_ID__", "__APPKIT_P10_EMAIL_IS_PUBLIC_ID__",
	"__APPKIT_P10_CHECKED__", "__APPKIT_P10_RESET_ACTION__", "__APPKIT_P10_RESET_FORM__",
	"__APPKIT_P10_BIO_DESCRIBEDBY__", "__APPKIT_P10_EMAIL_DESCRIBEDBY__",
	"__APPKIT_P10_COUNTRY_DESCRIBEDBY__", "__APPKIT_P10_REGION_DESCRIBEDBY__", "__APPKIT_P10_CURRENCY_DESCRIBEDBY__",
	"__APPKIT_P10_BIO_COUNT_ID__", "__APPKIT_P10_STATUS_ID__",
	"__APPKIT_P10_BIO_INVALID__", "__APPKIT_P10_EMAIL_INVALID__", "__APPKIT_P10_COUNTRY_INVALID__",
	"__APPKIT_P10_REGION_INVALID__", "__APPKIT_P10_CURRENCY_INVALID__",
	"__APPKIT_P10_NAME_INVALID__", "__APPKIT_P10_ADDRESS_INVALID__", "__APPKIT_P10_CITY_INVALID__", "__APPKIT_P10_POSTAL_CODE_INVALID__",
}

var catalystFields = []string{"name", "bio", "email", "address", "city", "postal_code"}
var catalystInputFields = []string{"name", "email", "address", "city", "postal_code"}

func adaptCatalystForm(body []byte) (string, error) {
	fail := func(reason string) (string, error) { return "", fmt.Errorf("catalyst adapter: %s", reason) }
	if !utf8.Valid(body) || len(body) == 0 || len(body) > 1<<20 {
		return fail("invalid private source bytes")
	}
	for _, marker := range catalystMarkers {
		if bytes.Contains(body, []byte(marker)) {
			return fail("private marker collision")
		}
	}
	if !catalystUniqueAttributes(body) {
		return fail("duplicate source attributes")
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return fail("private source did not parse")
	}
	forms := catalystElements(doc, "form")
	if len(forms) != 1 {
		return fail("expected one form")
	}
	form := forms[0]
	if len(catalystElements(form, "fieldset")) != 1 || len(catalystElements(form, "section")) != 5 {
		return fail("expected complete five-section form")
	}
	if !catalystHasAttr(form, "novalidate") || catalystAttr(form, "class") == "" {
		return fail("form contract changed")
	}
	setCatalystAttr(form, "method", "post")
	setCatalystAttr(form, "action", catalystMarkers[0])
	setCatalystAttr(form, "id", catalystMarkers[1])
	setCatalystAttr(form, "data-p10-settings", "")
	catalystDeleteAttr(catalystElements(form, "fieldset")[0], "disabled")
	for _, n := range catalystElements(form, "noscript") {
		catalystRemove(n)
	}

	inputs := map[string]*html.Node{}
	for _, input := range catalystElements(form, "input") {
		name := catalystAttr(input, "name")
		if _, known := catalystKnownField(name); known {
			if _, duplicate := inputs[name]; duplicate {
				return fail("duplicate native field")
			}
			inputs[name] = input
		}
	}
	for _, field := range catalystInputFields {
		input := inputs[field]
		if input == nil || input.Data != "input" {
			return fail("missing native field")
		}
		setCatalystAttr(input, "id", catalystIDMarker(field))
		setCatalystAttr(input, "name", field)
		setCatalystAttr(input, "value", catalystValueMarker(field))
		setCatalystAttr(input, "aria-invalid", "__APPKIT_P10_"+strings.ToUpper(field)+"_INVALID__")
		if field == "email" {
			setCatalystAttr(input, "aria-describedby", "__APPKIT_P10_EMAIL_DESCRIBEDBY__")
		}
	}
	for _, input := range catalystElements(form, "input") {
		if field, known := catalystKnownField(catalystAttr(input, "name")); known {
			setCatalystAttr(input, catalystFocusAttribute(field), "")
		}
	}

	textareas := catalystElements(form, "textarea")
	if len(textareas) != 1 || catalystAttr(textareas[0], "name") != "bio" {
		return fail("missing biography")
	}
	bio := textareas[0]
	// Hard wrapping may inject line breaks into a submitted value that are not
	// present in textarea.value. This adapter counts the textarea API value, so
	// it accepts only the default/explicit soft source behavior.
	if catalystHasAttr(bio, "wrap") && !strings.EqualFold(catalystAttr(bio, "wrap"), "soft") {
		return fail("unsupported biography wrapping")
	}
	setCatalystAttr(bio, "id", "__APPKIT_P10_BIO_ID__")
	setCatalystAttr(bio, "name", "bio")
	setCatalystAttr(bio, "aria-invalid", "__APPKIT_P10_BIO_INVALID__")
	setCatalystAttr(bio, "aria-describedby", "__APPKIT_P10_BIO_DESCRIBEDBY__")
	setCatalystAttr(bio, catalystFocusAttribute("bio"), "")
	catalystReplaceChildren(bio, catalystText("__APPKIT_P10_BIO_VALUE__"))

	if err := catalystReplaceSelects(form); err != nil {
		return fail(err.Error())
	}
	if err := catalystReplaceCheckbox(form); err != nil {
		return fail(err.Error())
	}
	if err := catalystReplaceActions(form); err != nil {
		return fail(err.Error())
	}
	catalystRemoveHiddenAuthority(form)
	if err := catalystInsertHelpers(form); err != nil {
		return fail(err.Error())
	}
	csrf := &html.Node{Type: html.ElementNode, Data: "input", Attr: []html.Attribute{{Key: "type", Val: "hidden"}, {Key: "name", Val: "_csrf"}, {Key: "value", Val: "__APPKIT_P10_CSRF__"}}}
	form.InsertBefore(csrf, form.FirstChild)

	var normalized bytes.Buffer
	if err := html.Render(&normalized, form); err != nil {
		return fail("private source did not render")
	}
	source := designconvert.Source{ArchiveID: "catalyst@94acb41796a8a97fdef44bf5b2d18f42919e43300cbca90077775f6b36dfa529", Path: "reference-default.html", SHA256: catalystDigest(normalized.Bytes()), Size: normalized.Len()}
	translated, err := designconvert.Translate(source, normalized.Bytes())
	if err != nil {
		return fail("private DOM syntax translation failed: " + err.Error())
	}
	if translated.Status != "translated" || translated.Artifact == nil {
		return fail("private DOM did not pass narrow syntax translation: " + catalystDiagnostics(normalized.Bytes(), translated.Diagnostics))
	}
	output, ok := catalystTemplBody(translated.Artifact.Content)
	if !ok {
		return fail("narrow syntax translation envelope changed")
	}
	replacements := map[string]string{
		`action={ "__APPKIT_P10_FORM_ACTION__" }`:                    `action={ data.Action }`,
		`id={ "__APPKIT_P10_FORM_ID__" }`:                            `id={ data.FieldID("form") }`,
		`value={ "__APPKIT_P10_CSRF__" }`:                            `value={ data.CSRFToken }`,
		`formaction={ "__APPKIT_P10_RESET_ACTION__" }`:               `formaction={ data.ResetAction }`,
		`form={ "__APPKIT_P10_RESET_FORM__" }`:                       `form={ data.FieldID("reset-form") }`,
		`checked={ "__APPKIT_P10_CHECKED__" }`:                       `checked?={ data.Values.PublicEmail }`,
		`aria-invalid={ "__APPKIT_P10_BIO_INVALID__" }`:              `aria-invalid={ data.Invalid("bio") }`,
		`aria-invalid={ "__APPKIT_P10_NAME_INVALID__" }`:             `aria-invalid={ data.Invalid("name") }`,
		`aria-invalid={ "__APPKIT_P10_EMAIL_INVALID__" }`:            `aria-invalid={ data.Invalid("email") }`,
		`aria-invalid={ "__APPKIT_P10_ADDRESS_INVALID__" }`:          `aria-invalid={ data.Invalid("address") }`,
		`aria-invalid={ "__APPKIT_P10_CITY_INVALID__" }`:             `aria-invalid={ data.Invalid("city") }`,
		`aria-invalid={ "__APPKIT_P10_POSTAL_CODE_INVALID__" }`:      `aria-invalid={ data.Invalid("postal_code") }`,
		`aria-invalid={ "__APPKIT_P10_COUNTRY_INVALID__" }`:          `aria-invalid={ data.Invalid("country") }`,
		`aria-invalid={ "__APPKIT_P10_REGION_INVALID__" }`:           `aria-invalid={ data.Invalid("region") }`,
		`aria-invalid={ "__APPKIT_P10_CURRENCY_INVALID__" }`:         `aria-invalid={ data.Invalid("currency") }`,
		`aria-describedby={ "__APPKIT_P10_BIO_DESCRIBEDBY__" }`:      `aria-describedby={ data.FieldID("bio-count") + " " + data.FieldID("bio-error") }`,
		`aria-describedby={ "__APPKIT_P10_EMAIL_DESCRIBEDBY__" }`:    `aria-describedby={ data.FieldID("email-error") }`,
		`aria-describedby={ "__APPKIT_P10_COUNTRY_DESCRIBEDBY__" }`:  `aria-describedby={ data.FieldID("country-error") }`,
		`aria-describedby={ "__APPKIT_P10_REGION_DESCRIBEDBY__" }`:   `aria-describedby={ data.FieldID("region-error") }`,
		`aria-describedby={ "__APPKIT_P10_CURRENCY_DESCRIBEDBY__" }`: `aria-describedby={ data.FieldID("currency-error") }`,
		`id={ "__APPKIT_P10_BIO_COUNT_ID__" }`:                       `id={ data.FieldID("bio-count") }`,
		`id={ "__APPKIT_P10_STATUS_ID__" }`:                          `id={ data.FieldID("status") }`,
		`for={ "__APPKIT_P10_EMAIL_IS_PUBLIC_ID__" }`:                `for={ data.FieldID("email_is_public") }`,
	}
	for _, field := range append(append([]string(nil), catalystFields...), "country", "region", "currency", "email_is_public") {
		replacements[`id={ "`+catalystIDMarker(field)+`" }`] = `id={ data.FieldID("` + field + `") }`
	}
	for _, field := range catalystInputFields {
		replacements[`value={ "`+catalystValueMarker(field)+`" }`] = `value={ data.Value("` + field + `") }`
	}
	for _, field := range append(append([]string(nil), catalystFields...), "country", "region", "currency", "email_is_public", "reset", "status") {
		from := catalystFocusAttribute(field) + `={ "" }`
		replacements[from] = `if data.Focus == "` + field + `" { autofocus }`
	}
	replacements[`{ "__APPKIT_P10_BIO_VALUE__" }`] = `{ settingsBioText(data) }`
	replacements[`{ "__APPKIT_P10_COUNTRY_OPTIONS__" }`] = `@settingsOptions(data, "country")`
	replacements[`{ "__APPKIT_P10_REGION_OPTIONS__" }`] = `@settingsOptions(data, "region")`
	replacements[`{ "__APPKIT_P10_CURRENCY_OPTIONS__" }`] = `@settingsOptions(data, "currency")`
	replacements[`{ "__APPKIT_P10_BIO_COUNT__" }`] = `@settingsBioCount(data)`
	replacements[`{ "__APPKIT_P10_STATUS__" }`] = `@settingsStatus(data)`
	for _, field := range []string{"bio", "email", "country", "region", "currency"} {
		replacements[`{ "__APPKIT_P10_`+strings.ToUpper(field)+`_ERROR__" }`] = `@settingsError(data, "` + field + `")`
	}
	for from, to := range replacements {
		if strings.Count(output, from) != 1 {
			return fail("translation marker missing or ambiguous " + from)
		}
		output = strings.Replace(output, from, to, 1)
	}
	for _, marker := range catalystMarkers {
		if strings.Contains(output, marker) {
			return fail("unreplaced private marker " + marker)
		}
	}
	return "// Code generated from a private P10 capture; experimental, private, and native-not-faithful.\npackage converted\n\ntempl settingsForm(data SettingsView) {\n" + output + "\n@settingsResetForm(data)\n@settingsEnhancementConfig(data)\n}\n", nil
}

func catalystReplaceSelects(form *html.Node) error {
	buttons := map[string]*html.Node{}
	for _, button := range catalystElements(form, "button") {
		if label := catalystAttr(button, "aria-label"); label == "Country" || label == "Region" {
			if buttons[label] != nil {
				return fmt.Errorf("duplicate select control")
			}
			buttons[label] = button
		}
	}
	currencies := catalystElements(form, "select")
	if buttons["Country"] == nil || buttons["Region"] == nil || len(currencies) != 1 || catalystAttr(currencies[0], "name") != "currency" || catalystAttr(currencies[0], "aria-label") != "Currency" {
		return fmt.Errorf("missing select control")
	}
	currencyWrapper := catalystControlWrapper(currencies[0])
	if currencyWrapper == nil {
		return fmt.Errorf("missing currency wrapper")
	}
	for _, spec := range []struct{ label, name, marker, invalid string }{
		{"Country", "country[code]", "__APPKIT_P10_COUNTRY_OPTIONS__", "__APPKIT_P10_COUNTRY_INVALID__"},
		{"Region", "region", "__APPKIT_P10_REGION_OPTIONS__", "__APPKIT_P10_REGION_INVALID__"},
	} {
		original := catalystControlWrapper(buttons[spec.label])
		if original == nil {
			return fmt.Errorf("missing select wrapper")
		}
		clone := catalystClone(currencyWrapper)
		catalystKeepLayoutClass(clone, original)
		selects := catalystElements(clone, "select")
		if len(selects) != 1 {
			return fmt.Errorf("currency wrapper shape")
		}
		selectNode := selects[0]
		selectNode.Data = "select"
		selectNode.DataAtom = 0
		selectNode.Attr = catalystRetainClass(selectNode.Attr)
		selectNode.Attr = append(selectNode.Attr, html.Attribute{Key: "aria-label", Val: spec.label}, html.Attribute{Key: "name", Val: spec.name}, html.Attribute{Key: "id", Val: catalystIDMarker(strings.TrimSuffix(spec.name, "[code]"))}, html.Attribute{Key: "aria-invalid", Val: spec.invalid}, html.Attribute{Key: "aria-describedby", Val: "__APPKIT_P10_" + strings.ToUpper(strings.TrimSuffix(spec.name, "[code]")) + "_DESCRIBEDBY__"})
		selectNode.Attr = append(selectNode.Attr, html.Attribute{Key: "data-p10-focus-" + strings.TrimSuffix(spec.name, "[code]"), Val: ""})
		catalystReplaceChildren(selectNode, catalystText(spec.marker))
		catalystReplaceNode(original, clone)
	}
	currency := currencies[0]
	setCatalystAttr(currency, "id", "__APPKIT_P10_CURRENCY_ID__")
	setCatalystAttr(currency, "aria-invalid", "__APPKIT_P10_CURRENCY_INVALID__")
	setCatalystAttr(currency, "aria-describedby", "__APPKIT_P10_CURRENCY_DESCRIBEDBY__")
	setCatalystAttr(currency, catalystFocusAttribute("currency"), "")
	catalystReplaceChildren(currency, catalystText("__APPKIT_P10_CURRENCY_OPTIONS__"))
	return nil
}

func catalystReplaceCheckbox(form *html.Node) error {
	controllers := []*html.Node{}
	for _, span := range catalystElements(form, "span") {
		if catalystAttr(span, "role") == "checkbox" {
			controllers = append(controllers, span)
		}
	}
	if len(controllers) != 1 || controllers[0].Parent == nil {
		return fmt.Errorf("missing checkbox controller")
	}
	row := controllers[0].Parent
	labels := catalystElements(row, "label")
	if len(labels) != 1 {
		return fmt.Errorf("checkbox label contract")
	}
	label := labels[0]
	// The custom controller's generated label ID is no longer referenced. Native
	// association uses for; retaining that static ID would collide across instances.
	catalystDeleteAttr(label, "id")
	setCatalystAttr(label, "for", "__APPKIT_P10_EMAIL_IS_PUBLIC_ID__")
	setCatalystAttr(label, "data-p10-checkbox-label", "")
	checkbox := &html.Node{Type: html.ElementNode, Data: "input", Attr: []html.Attribute{{Key: "type", Val: "checkbox"}, {Key: "id", Val: "__APPKIT_P10_EMAIL_IS_PUBLIC_ID__"}, {Key: "name", Val: "email_is_public"}, {Key: "data-p10-checkbox", Val: ""}, {Key: catalystFocusAttribute("email_is_public"), Val: ""}, {Key: "checked", Val: "__APPKIT_P10_CHECKED__"}}}
	row.InsertBefore(checkbox, row.FirstChild)
	catalystRemove(controllers[0])
	return nil
}

func catalystReplaceActions(form *html.Node) error {
	var reset, submit *html.Node
	for _, n := range catalystElements(form, "button") {
		switch catalystAttr(n, "type") {
		case "reset":
			reset = n
		case "submit":
			submit = n
		}
	}
	if reset == nil || submit == nil {
		return fmt.Errorf("missing action controls")
	}
	setCatalystAttr(reset, "type", "submit")
	setCatalystAttr(reset, "name", "_action")
	setCatalystAttr(reset, "value", "reset")
	setCatalystAttr(reset, "formnovalidate", "")
	setCatalystAttr(reset, "formaction", "__APPKIT_P10_RESET_ACTION__")
	setCatalystAttr(reset, "form", "__APPKIT_P10_RESET_FORM__")
	setCatalystAttr(reset, catalystFocusAttribute("reset"), "")
	setCatalystAttr(submit, "name", "_action")
	setCatalystAttr(submit, "value", "validate")
	return nil
}

func catalystInsertHelpers(form *html.Node) error {
	if count := catalystMarkerReplaceText(form, "bio-count", "__APPKIT_P10_BIO_COUNT__", "__APPKIT_P10_BIO_COUNT_ID__"); count != 1 {
		return fmt.Errorf("biography count contract")
	}
	status := 0
	for _, p := range catalystElements(form, "p") {
		if catalystAttr(p, "role") == "status" {
			setCatalystAttr(p, "id", "__APPKIT_P10_STATUS_ID__")
			setCatalystAttr(p, "tabindex", "-1")
			setCatalystAttr(p, catalystFocusAttribute("status"), "")
			catalystReplaceChildren(p, catalystText("__APPKIT_P10_STATUS__"))
			status++
		}
	}
	if status != 1 {
		return fmt.Errorf("status contract")
	}
	for _, field := range []string{"bio", "email", "country", "region", "currency"} {
		target := catalystFindControl(form, field)
		anchor := catalystErrorAnchor(target)
		if anchor == nil || anchor.Parent == nil {
			return fmt.Errorf("error target contract")
		}
		anchor.Parent.InsertBefore(catalystText("__APPKIT_P10_"+strings.ToUpper(field)+"_ERROR__"), anchor.NextSibling)
	}
	return nil
}

func catalystErrorAnchor(target *html.Node) *html.Node {
	for node := target; node != nil; node = node.Parent {
		if catalystAttr(node, "data-slot") == "control" {
			return node
		}
	}
	return target
}

func catalystFindControl(form *html.Node, field string) *html.Node {
	for _, n := range catalystElements(form, "input") {
		if catalystAttr(n, "name") == field {
			return n
		}
	}
	for _, n := range append(catalystElements(form, "textarea"), catalystElements(form, "select")...) {
		if catalystAttr(n, "name") == field || field == "country" && catalystAttr(n, "name") == "country[code]" {
			return n
		}
	}
	return nil
}

func catalystRemoveHiddenAuthority(form *html.Node) {
	for _, input := range catalystElements(form, "input") {
		if catalystAttr(input, "type") == "hidden" || catalystHasAttr(input, "hidden") || catalystHasAttr(input, "readonly") {
			catalystRemove(input)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, span := range catalystElements(form, "span") {
			if catalystHasAttr(span, "hidden") && !catalystHasElementChild(span) {
				catalystRemove(span)
				changed = true
			}
		}
	}
}

func catalystKnownField(name string) (string, bool) {
	for _, field := range catalystInputFields {
		if name == field {
			return field, true
		}
	}
	return "", false
}

func catalystValueMarker(field string) string {
	return "__APPKIT_P10_" + strings.ToUpper(field) + "_VALUE__"
}
func catalystFocusAttribute(field string) string {
	return "data-p10-focus-" + strings.ReplaceAll(field, "_", "-")
}
func catalystIDMarker(field string) string { return "__APPKIT_P10_" + strings.ToUpper(field) + "_ID__" }
func catalystDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func catalystDiagnostics(source []byte, diagnostics []designconvert.Diagnostic) string {
	if len(diagnostics) == 0 {
		return "no diagnostic"
	}
	parts := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		parts = append(parts, fmt.Sprintf("%s@%d[%s]", diagnostic.Code, diagnostic.Offset, catalystDiagnosticAttributes(source, diagnostic.Offset)))
	}
	return strings.Join(parts, ",")
}

func catalystDiagnosticAttributes(source []byte, offset int) string {
	z := html.NewTokenizer(bytes.NewReader(source))
	position := 0
	for {
		tokenType := z.Next()
		raw := z.Raw()
		start, end := position, position+len(raw)
		position = end
		if tokenType == html.ErrorToken {
			return ""
		}
		if offset < start || offset >= end || tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}
		var names []string
		for _, attribute := range z.Token().Attr {
			names = append(names, attribute.Key)
		}
		return strings.Join(names, ",")
	}
}

func catalystTemplBody(output string) (string, bool) {
	const prefix = "package converted\n\ntempl Specimen() {\n"
	const suffix = "\n}\n"
	if !strings.HasPrefix(output, prefix) || !strings.HasSuffix(output, suffix) {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(output, prefix), suffix), true
}

func catalystElements(root *html.Node, name string) []*html.Node {
	var found []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == name {
			found = append(found, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func catalystUniqueAttributes(body []byte) bool {
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		tokenType := z.Next()
		if tokenType == html.ErrorToken {
			return z.Err() == io.EOF
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}
		seen := map[string]bool{}
		for _, attribute := range z.Token().Attr {
			if seen[attribute.Key] {
				return false
			}
			seen[attribute.Key] = true
		}
	}
}

func catalystAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func catalystHasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}
func setCatalystAttr(n *html.Node, key, value string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: value})
}

func catalystDeleteAttr(n *html.Node, key string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
			return
		}
	}
}
func catalystText(value string) *html.Node { return &html.Node{Type: html.TextNode, Data: value} }
func catalystReplaceChildren(n *html.Node, children ...*html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		c = next
	}
	for _, c := range children {
		n.AppendChild(c)
	}
}
func catalystRemove(n *html.Node) {
	if n.Parent != nil {
		n.Parent.RemoveChild(n)
	}
}
func catalystReplaceNode(old, replacement *html.Node) {
	if old.Parent != nil {
		old.Parent.InsertBefore(replacement, old)
		old.Parent.RemoveChild(old)
	}
}
func catalystControlWrapper(n *html.Node) *html.Node {
	for p := n; p != nil; p = p.Parent {
		if catalystAttr(p, "data-slot") == "control" {
			return p
		}
	}
	return nil
}
func catalystHasElementChild(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return true
		}
	}
	return false
}
func catalystRetainClass(attrs []html.Attribute) []html.Attribute {
	for _, a := range attrs {
		if a.Key == "class" {
			return []html.Attribute{{Key: "class", Val: a.Val}}
		}
	}
	return nil
}

func catalystKeepLayoutClass(clone, original *html.Node) {
	originalClass := catalystAttr(original, "class")
	if originalClass == "" {
		return
	}
	var layout []string
	for _, token := range strings.Fields(originalClass) {
		if strings.HasPrefix(token, "col-") || strings.HasPrefix(token, "row-") || strings.HasPrefix(token, "self-") {
			layout = append(layout, token)
		}
	}
	if len(layout) != 0 {
		setCatalystAttr(clone, "class", catalystAttr(clone, "class")+" "+strings.Join(layout, " "))
	}
}
func catalystClone(n *html.Node) *html.Node {
	clone := &html.Node{Type: n.Type, DataAtom: n.DataAtom, Data: n.Data, Namespace: n.Namespace, Attr: append([]html.Attribute(nil), n.Attr...)}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		clone.AppendChild(catalystClone(c))
	}
	return clone
}
func catalystMarkerReplaceText(form *html.Node, id, marker, replacementID string) int {
	count := 0
	for _, n := range catalystElements(form, "p") {
		if catalystAttr(n, "id") == id {
			setCatalystAttr(n, "id", replacementID)
			catalystReplaceChildren(n, catalystText(marker))
			count++
		}
	}
	return count
}
