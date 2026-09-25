package form

import (
	"html/template"
	"regexp"
	"strings"
	"testing"
)

// hexColorRE matches CSS hex color literals in fill/stroke/color/background.
var hexColorRE = regexp.MustCompile(`(?i)(fill|stroke|color|background)[=:]["']?\s*#[0-9a-f]{3,8}\b`)

func TestField_LabelHelpRequired(t *testing.T) {
	got := RenderField(Field{
		Label: "Case name", For: "name", Help: "As it appears on the caption.",
		Required: true, Control: template.HTML(`<input id="name">`),
	})
	for _, want := range []string{
		`<label class="text-sm font-medium text-ink" for="name">Case name`,
		`text-danger`, // required marker
		`<input id="name">`,
		`id="name-help"`, "As it appears on the caption.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("field missing %q in:\n%s", want, got)
		}
	}
}

func TestField_ErrorTakesPriorityOverHelp(t *testing.T) {
	got := RenderField(Field{
		Label: "Email", For: "email", Help: "We never share it.",
		Error: "Enter a valid email.", Control: template.HTML(`<input id="email">`),
	})
	if !strings.Contains(got, `id="email-error"`) || !strings.Contains(got, "Enter a valid email.") {
		t.Errorf("expected error message, got:\n%s", got)
	}
	if strings.Contains(got, "We never share it.") {
		t.Errorf("help should be suppressed when error is present:\n%s", got)
	}
}

func TestInput_TypeAndAria(t *testing.T) {
	got := RenderInput(Input{ID: "amt", Name: "amount", Type: "number", Required: true, Invalid: true})
	for _, want := range []string{
		`type="number"`, `id="amt"`, `name="amount"`, ` required`, `aria-required="true"`,
		`aria-invalid="true"`, `aria-describedby="amt-error"`, "border-danger",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("input missing %q in:\n%s", want, got)
		}
	}
}

func TestInput_DefaultsTextAndHelpDescribedBy(t *testing.T) {
	got := RenderInput(Input{ID: "x", HasHelp: true})
	if !strings.Contains(got, `type="text"`) {
		t.Errorf("expected default type text: %s", got)
	}
	if !strings.Contains(got, `aria-describedby="x-help"`) {
		t.Errorf("expected help describedby: %s", got)
	}
	if strings.Contains(got, "aria-invalid") {
		t.Errorf("non-invalid input should not be aria-invalid: %s", got)
	}
}

func TestTextarea(t *testing.T) {
	got := RenderTextarea(Textarea{ID: "notes", Name: "notes", Value: "hi", Rows: 6})
	if !strings.Contains(got, `rows="6"`) || !strings.Contains(got, `>hi</textarea>`) {
		t.Errorf("textarea wrong: %s", got)
	}
	if !strings.Contains(RenderTextarea(Textarea{}), `rows="4"`) {
		t.Error("textarea default rows should be 4")
	}
}

func TestSelect_PlaceholderAndSelected(t *testing.T) {
	got := RenderSelect(Select{
		ID: "tier", Name: "tier", Placeholder: "Select tier…", Value: "doc",
		Options: []Option{{Value: "doc", Label: "Documentary"}, {Value: "test", Label: "Testimonial"}},
	})
	if !strings.Contains(got, `<option value="" disabled>Select tier…</option>`) {
		t.Errorf("expected disabled placeholder (not selected when value set): %s", got)
	}
	if !strings.Contains(got, `<option value="doc" selected>Documentary</option>`) {
		t.Errorf("expected selected option: %s", got)
	}
}

func TestSelect_PlaceholderSelectedWhenEmpty(t *testing.T) {
	got := RenderSelect(Select{ID: "k", Placeholder: "Pick…", Options: []Option{{Value: "a", Label: "A"}}})
	if !strings.Contains(got, `<option value="" disabled selected>Pick…</option>`) {
		t.Errorf("placeholder should be selected when no value: %s", got)
	}
}

func TestCheckbox(t *testing.T) {
	got := RenderCheckbox(Checkbox{ID: "tos", Name: "tos", Label: "I agree", Checked: true})
	for _, want := range []string{`type="checkbox"`, `id="tos"`, `value="true"`, ` checked`, "I agree"} {
		if !strings.Contains(got, want) {
			t.Errorf("checkbox missing %q: %s", want, got)
		}
	}
}

// Recorded before Invalid/HasHelp were added: opting out must preserve the
// complete existing markup, including attribute and utility ordering.
func TestCheckboxLegacyBytes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Checkbox
		want string
	}{
		{"zero", Checkbox{}, `<label class="flex items-center gap-2 text-sm text-ink"><input type="checkbox" class="rounded border-line text-accent focus:ring-2 focus:ring-focus" value="true"/></label>`},
		{"existing fixture", Checkbox{ID: "tos", Name: "tos", Label: "I agree", Checked: true}, `<label class="flex items-center gap-2 text-sm text-ink"><input type="checkbox" class="rounded border-line text-accent focus:ring-2 focus:ring-focus" id="tos" name="tos" value="true" checked/>I agree</label>`},
		{"escaped flags", Checkbox{ID: `a"b`, Name: "x&y", Value: `yes"now`, Label: "R&D <yes>", Checked: true, Required: true, Disabled: true}, `<label class="flex items-center gap-2 text-sm text-ink"><input type="checkbox" class="rounded border-line text-accent focus:ring-2 focus:ring-focus" id="a&#34;b" name="x&amp;y" value="yes&#34;now" checked required aria-required="true" disabled/>R&amp;D &lt;yes&gt;</label>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RenderCheckbox(tc.in); got != tc.want {
				t.Fatalf("legacy checkbox output changed:\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

func TestCheckboxHelpAndInvalidWiring(t *testing.T) {
	for _, tc := range []struct {
		name, describedBy string
		invalid, hasHelp  bool
	}{
		{"help", "digest-help", false, true},
		{"error", "digest-error", true, false},
		{"error takes precedence", "digest-error", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderCheckbox(Checkbox{ID: "digest", Name: "digest", Label: "Weekly digest", Invalid: tc.invalid, HasHelp: tc.hasHelp})
			if !strings.Contains(got, `aria-describedby="`+tc.describedBy+`"`) || strings.Count(got, "aria-describedby=") != 1 {
				t.Fatalf("wrong description target: %s", got)
			}
			if strings.Contains(got, `aria-invalid="true"`) != tc.invalid {
				t.Fatalf("wrong invalid semantics: %s", got)
			}
			if tc.invalid {
				if !strings.Contains(got, "border-danger") || !strings.Contains(got, "focus:ring-danger") || strings.Contains(got, "border-line") || strings.Contains(got, "focus:ring-focus") || strings.Contains(got, "digest-help") {
					t.Fatalf("error styling/precedence incorrect: %s", got)
				}
			} else if strings.Contains(got, "danger") {
				t.Fatalf("help alone received invalid styling: %s", got)
			}
			if hexColorRE.MatchString(got) {
				t.Fatalf("checkbox uses hardcoded colors: %s", got)
			}
		})
	}
}

func TestCheckboxInvalidFlagsAndEscapedIDs(t *testing.T) {
	const hostileID = `digest"><svg/onload=fixture()>`
	for _, invalid := range []bool{false, true} {
		got := RenderCheckbox(Checkbox{ID: hostileID, Label: "R&D <weekly>", Checked: true, Required: true, Disabled: true, Invalid: invalid, HasHelp: true})
		suffix := "-help"
		if invalid {
			suffix = "-error"
		}
		for _, want := range []string{
			`id="digest&#34;&gt;&lt;svg/onload=fixture()&gt;"`,
			`aria-describedby="digest&#34;&gt;&lt;svg/onload=fixture()&gt;` + suffix + `"`,
			`value="true" checked required aria-required="true" disabled`,
			`R&amp;D &lt;weekly&gt;</label>`,
		} {
			if !strings.Contains(got, want) {
				t.Errorf("missing escaped target/retained flag %q in %s", want, got)
			}
		}
		if strings.Contains(got, `<svg`) {
			t.Fatal("checkbox ID became active markup")
		}
	}
	for _, invalid := range []bool{false, true} {
		got := RenderCheckbox(Checkbox{Invalid: invalid, HasHelp: true})
		if strings.Contains(got, "aria-describedby") {
			t.Fatalf("empty ID created a dangling description: %s", got)
		}
		if strings.Contains(got, `aria-invalid="true"`) != invalid {
			t.Fatalf("empty ID lost invalid semantics: %s", got)
		}
	}
}

func TestRadioGroup_FieldsetLegendAndChecked(t *testing.T) {
	got := RenderRadioGroup(RadioGroup{
		Name: "ev", Legend: "Do you have evidence?", Value: "yes", Required: true,
		Options: []Option{{Value: "yes", Label: "Yes"}, {Value: "no", Label: "No"}},
	})
	for _, want := range []string{
		`<fieldset`, `<legend`, "Do you have evidence?",
		`name="ev"`, `value="yes" checked`, `id="ev-0"`, `id="ev-1"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("radiogroup missing %q: %s", want, got)
		}
	}
	// required only on the first radio (grouped requiredness).
	if strings.Count(got, " required") != 1 {
		t.Errorf("expected required on exactly one radio: %s", got)
	}
}

func TestButton_Variants(t *testing.T) {
	if !strings.Contains(RenderButton(Button{Label: "Save"}), `type="submit"`) {
		t.Error("button should default to submit")
	}
	if !strings.Contains(RenderButton(Button{Label: "Save"}), "bg-accent") {
		t.Error("default variant should be primary (accent)")
	}
	if !strings.Contains(RenderButton(Button{Label: "Delete", Variant: ButtonDanger}), "bg-danger") {
		t.Error("danger variant missing")
	}
	link := RenderButton(Button{Label: "Cancel", Variant: ButtonGhost, Href: "/back"})
	if !strings.Contains(link, `<a class=`) || !strings.Contains(link, `href="/back"`) {
		t.Errorf("href button should render as anchor: %s", link)
	}
}

func TestForm_EscapingAndTokensOnly(t *testing.T) {
	xss := `"><script>alert(1)</script>`
	all := strings.Join([]string{
		RenderField(Field{Label: xss, For: xss, Help: xss, Control: template.HTML("<input>")}),
		RenderInput(Input{ID: xss, Value: xss, Placeholder: xss}),
		RenderTextarea(Textarea{Value: xss}),
		RenderSelect(Select{Placeholder: xss, Options: []Option{{Value: xss, Label: xss}}}),
		RenderCheckbox(Checkbox{Label: xss}),
		RenderRadioGroup(RadioGroup{Name: "g", Legend: xss, Options: []Option{{Value: xss, Label: xss}}}),
		RenderButton(Button{Label: xss}),
	}, "")
	if strings.Contains(all, "<script>alert(1)</script>") {
		t.Errorf("unescaped script injected:\n%s", all)
	}
	if hexColorRE.MatchString(all) {
		t.Errorf("hardcoded hex color found — must use token utilities:\n%s", all)
	}
}

func TestForm_Deterministic(t *testing.T) {
	f := func() string {
		return RenderField(Field{Label: "L", For: "i", Control: template.HTML(RenderInput(Input{ID: "i"}))}) +
			RenderSelect(Select{ID: "s", Options: []Option{{Value: "a", Label: "A"}}}) +
			RenderButton(Button{Label: "Go"})
	}
	if f() != f() {
		t.Error("form rendering is non-deterministic")
	}
}
