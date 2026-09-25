package form

import (
	"strings"
	"testing"
	"unicode"

	"golang.org/x/net/html"
)

func radioNodes(t *testing.T, body string) (fieldset *html.Node, radios []*html.Node, messages []*html.Node) {
	t.Helper()
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "fieldset":
				fieldset = n
			case "input":
				if attrValue(n, "type") == "radio" {
					radios = append(radios, n)
				}
			case "p":
				messages = append(messages, n)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return
}

func attrValue(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func assertFeedbackRefs(t *testing.T, radios, messages []*html.Node) {
	t.Helper()
	ids := map[string]bool{}
	for _, message := range messages {
		if id := attrValue(message, "id"); id != "" {
			ids[id] = true
		}
	}
	for _, radio := range radios {
		if ref := attrValue(radio, "aria-describedby"); ref != "" && !ids[ref] {
			t.Fatalf("radio has dangling aria-describedby=%q", ref)
		}
	}
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return b.String()
}

func firstRadioRef(t *testing.T, body string) string {
	t.Helper()
	_, radios, _ := radioNodes(t, body)
	if len(radios) != 1 {
		t.Fatalf("want one radio, got %d", len(radios))
	}
	return attrValue(radios[0], "aria-describedby")
}

func TestRadioGroupFeedbackAssociationsAndNativeAttributes(t *testing.T) {
	name := `billing "plan" / Ü`
	body := RenderRadioGroup(RadioGroup{
		Name: name, Legend: `Choose <plan>`, Value: "pro", Required: true,
		Options: []Option{{Value: "pro", Label: `Pro & <fast>`}, {Value: "free", Label: "Free", Disabled: true}},
		Help:    `Help & "details"`, Error: `Error <fix>`, Disabled: true,
	})
	fieldset, radios, messages := radioNodes(t, body)
	if fieldset == nil || !hasAttr(fieldset, "disabled") || len(radios) != 2 || len(messages) != 1 {
		t.Fatalf("unexpected radio structure: fieldset=%v radios=%d messages=%d", fieldset != nil, len(radios), len(messages))
	}
	ref := attrValue(radios[0], "aria-describedby")
	if ref == "" || ref != attrValue(radios[1], "aria-describedby") || attrValue(radios[0], "aria-invalid") != "true" || attrValue(radios[1], "aria-invalid") != "true" {
		t.Fatalf("error feedback not shared by radios: %q %q", ref, attrValue(radios[1], "aria-describedby"))
	}
	if strings.IndexFunc(ref, unicode.IsSpace) >= 0 || !strings.Contains(attrValue(radios[0], "class"), "border-danger") || !strings.Contains(attrValue(radios[0], "class"), "focus:ring-danger") {
		t.Fatalf("unstable/unsafe feedback or error styling: ref=%q class=%q", ref, attrValue(radios[0], "class"))
	}
	if attrValue(radios[0], "id") != name+"-0" || attrValue(radios[0], "name") != name || attrValue(radios[0], "value") != "pro" || !hasAttr(radios[0], "required") || !hasAttr(radios[0], "checked") || hasAttr(radios[0], "disabled") {
		t.Fatalf("native radio attributes changed: %+v", radios[0].Attr)
	}
	if !hasAttr(radios[1], "disabled") || attrValue(messages[0], "id") != ref || nodeText(messages[0]) != `Error <fix>` || !strings.Contains(attrValue(messages[0], "class"), "text-danger") || strings.Contains(body, "Help &amp;") {
		t.Fatalf("error priority or escaping failed: %s", body)
	}
	if !strings.Contains(body, "Pro &amp; &lt;fast&gt;") || !strings.Contains(body, "Choose &lt;plan&gt;") {
		t.Fatalf("hostile labels were not escaped: %s", body)
	}
	assertFeedbackRefs(t, radios, messages)
	if again := RenderRadioGroup(RadioGroup{Name: name, Error: `Error <fix>`, Options: []Option{{Value: "pro", Label: "Pro"}}}); firstRadioRef(t, again) != ref {
		t.Fatal("feedback id changed across equivalent renders")
	}

	second := RenderRadioGroup(RadioGroup{Name: "other", Options: []Option{{Value: "x", Label: "X"}}, Help: "Other help"})
	_, secondRadios, secondMessages := radioNodes(t, second)
	secondRef := attrValue(secondRadios[0], "aria-describedby")
	if secondRef == "" || secondRef == ref || attrValue(secondMessages[0], "id") != secondRef || hasAttr(secondRadios[0], "aria-invalid") || strings.Contains(attrValue(secondRadios[0], "class"), "danger") || !strings.Contains(attrValue(secondMessages[0], "class"), "text-ink-muted") {
		t.Fatal("distinct groups reused feedback association")
	}
	assertFeedbackRefs(t, secondRadios, secondMessages)
}

func TestRadioGroupEmptyNameAndNoMessageCompatibility(t *testing.T) {
	without := RenderRadioGroup(RadioGroup{Name: "choice", Options: []Option{{Value: "a", Label: "A"}}})
	want := `<fieldset class="m-0 flex min-w-0 flex-col gap-2 border-0 p-0"><div class="flex flex-col gap-2"><label class="flex items-center gap-2 text-sm text-ink"><input type="radio" class="border-line text-accent focus:ring-2 focus:ring-focus" id="choice-0" name="choice" value="a"/>A</label></div></fieldset>`
	if without != want {
		t.Fatalf("no-message output changed:\n%s", without)
	}
	_, plainRadios, plainMessages := radioNodes(t, without)
	if len(plainMessages) != 0 || hasAttr(plainRadios[0], "aria-describedby") || hasAttr(plainRadios[0], "aria-invalid") {
		t.Fatal("no-message output gained feedback attributes")
	}
	body := RenderRadioGroup(RadioGroup{Name: "", Error: `Bad "choice"`, Options: []Option{{Value: "a", Label: "A"}}})
	_, radios, messages := radioNodes(t, body)
	if len(radios) != 1 || hasAttr(radios[0], "aria-describedby") || attrValue(radios[0], "aria-invalid") != "true" || len(messages) != 1 || hasAttr(messages[0], "id") {
		t.Fatalf("empty-name compatibility or invalid state failed: %s", body)
	}
}
