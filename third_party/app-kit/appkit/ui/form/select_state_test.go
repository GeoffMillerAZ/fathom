package form

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func selectElements(t *testing.T, body, name string) []*html.Node {
	t.Helper()
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
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

func selectAttr(n *html.Node, name string) (string, bool) {
	for _, attr := range n.Attr {
		if attr.Key == name {
			return attr.Val, true
		}
	}
	return "", false
}

func selectText(n *html.Node) string {
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

func TestRenderSelectUnmatchedValueUsesUnavailablePrompt(t *testing.T) {
	for _, tc := range []struct {
		name, placeholder, wantPrompt string
	}{
		{"default prompt", "", "Selection unavailable"},
		{"custom prompt is escaped", `Choose <safe> & retry`, `Choose <safe> & retry`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const unknown = `retired"><script>alert(1)</script>`
			body := RenderSelect(Select{
				ID: "region", Name: "region", Value: unknown, Placeholder: tc.placeholder,
				Options:  []Option{{Value: "phx", Label: "Phoenix"}, {Value: "lon", Label: "London"}},
				Required: true, Disabled: true,
			})
			selects := selectElements(t, body, "select")
			options := selectElements(t, body, "option")
			if len(selects) != 1 || len(options) != 3 {
				t.Fatalf("selects/options = %d/%d, want 1/3: %s", len(selects), len(options), body)
			}
			for _, attr := range []string{"required", "aria-required", "disabled"} {
				if _, ok := selectAttr(selects[0], attr); !ok {
					t.Errorf("select lost %s: %s", attr, body)
				}
			}
			if value, _ := selectAttr(selects[0], "aria-required"); value != "true" {
				t.Errorf("aria-required = %q, want true: %s", value, body)
			}
			for _, attr := range []string{"aria-invalid", "aria-describedby"} {
				if _, ok := selectAttr(selects[0], attr); ok {
					t.Errorf("unmatched value automatically added %s: %s", attr, body)
				}
			}
			if class, _ := selectAttr(selects[0], "class"); strings.Contains(class, "danger") {
				t.Errorf("unmatched value automatically changed styling: %s", body)
			}
			prompt := options[0]
			if value, _ := selectAttr(prompt, "value"); value != "" || selectText(prompt) != tc.wantPrompt {
				t.Errorf("unavailable prompt = value %q text %q", value, selectText(prompt))
			}
			if _, ok := selectAttr(prompt, "disabled"); !ok {
				t.Errorf("unavailable prompt is enabled: %s", body)
			}
			if _, ok := selectAttr(prompt, "selected"); !ok {
				t.Errorf("unavailable prompt is not selected: %s", body)
			}
			for _, option := range options[1:] {
				if _, ok := selectAttr(option, "selected"); ok {
					t.Errorf("valid option unexpectedly selected: %s", body)
				}
			}
			if strings.Contains(body, unknown) || strings.Contains(body, "<script") {
				t.Errorf("unknown value was reflected or prompt was not escaped: %s", body)
			}
		})
	}
}

func TestRenderSelectPlaceholderAloneOwnsEmptyValue(t *testing.T) {
	body := RenderSelect(Select{
		Placeholder: "Choose", Value: "",
		Options: []Option{{Value: "", Label: "None"}, {Value: "a", Label: "A"}},
	})
	options := selectElements(t, body, "option")
	if len(options) != 3 {
		t.Fatalf("options = %d, want 3: %s", len(options), body)
	}
	for i, option := range options {
		_, selected := selectAttr(option, "selected")
		if selected != (i == 0) {
			t.Errorf("option %d selected=%v, want %v: %s", i, selected, i == 0, body)
		}
	}
}

func TestRenderSelectUnaffectedBytesRemainStable(t *testing.T) {
	const class = "w-full rounded-md border bg-surface px-3 py-2 text-sm text-ink placeholder:text-ink-faint focus:outline-none focus:ring-2 border-line focus:ring-focus"
	for _, tc := range []struct {
		name string
		in   Select
		want string
	}{
		{"known disabled match", Select{ID: "s", Name: "s", Value: "b", Placeholder: "Pick", Options: []Option{{Value: "a", Label: "A"}, {Value: "b", Label: "B", Disabled: true}}, Required: true}, `<select class="` + class + `" id="s" name="s" required aria-required="true"><option value="" disabled>Pick</option><option value="a">A</option><option value="b" selected disabled>B</option></select>`},
		{"native first default", Select{Options: []Option{{Value: "a", Label: "A"}, {Value: "b", Label: "B"}}}, `<select class="` + class + `"><option value="a">A</option><option value="b">B</option></select>`},
		{"empty option without placeholder", Select{Options: []Option{{Value: "", Label: "None"}, {Value: "a", Label: "A"}}}, `<select class="` + class + `"><option value="" selected>None</option><option value="a">A</option></select>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RenderSelect(tc.in); got != tc.want {
				t.Fatalf("select bytes changed:\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestRenderSelectDoesNotMutateInput(t *testing.T) {
	in := Select{Value: "gone", Placeholder: "Choose", Options: []Option{{Value: "a", Label: "A"}, {Value: "b", Label: "B", Disabled: true}}}
	want := in
	want.Options = append([]Option(nil), in.Options...)
	_ = RenderSelect(in)
	if !reflect.DeepEqual(in, want) {
		t.Fatalf("RenderSelect mutated input:\n got %#v\nwant %#v", in, want)
	}
}
