// Package form renders the token-themed, accessible form vocabulary every app
// needs — labeled fields with help/error text, text inputs, textareas, selects,
// checkboxes, radio groups, and buttons — as deterministic server-rendered HTML
// (D7-themed, escaped, no Node, loopback-safe). Controls carry the ARIA wiring a
// field needs (label association, aria-describedby for help/error, aria-invalid,
// aria-required) so the Field wrapper and a bare control stay consistent.
//
// HTML-string style (like appkit/ui/chrome): each control returns a string;
// Field composes a control (passed as template.HTML) with its label and messages.
package form

import (
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/internal/webref"
)

func esc(s string) string { return html.EscapeString(s) }

// attr renders ` name="value"` (escaped) when value is non-empty, else "".
func attr(name, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf(` %s="%s"`, name, esc(value))
}

// boolAttr renders ` name` when on, else "". name is emitted verbatim (callers
// pass static, safe attribute text).
func boolAttr(name string, on bool) string {
	if !on {
		return ""
	}
	return " " + name
}

// ── Field ─────────────────────────────────────────────────────────────────────

// Field wraps a labeled control with optional help text and an error message.
// It owns the accessible plumbing: the <label for> targets For; Help and Error
// render with stable ids the control references via aria-describedby (HelpID /
// ErrorID). When Error is set the field reads as invalid — set the control's
// Invalid to match the styling.
type Field struct {
	Label    string
	For      string        // id of the control this field labels
	Help     string        // optional help text (rendered below the control)
	Error    string        // optional error (rendered below; takes visual priority)
	Required bool          // appends a required marker to the label
	Control  template.HTML // the control HTML (Input/Select/Textarea/… or custom)
}

// HelpID returns the DOM id of a field's help text, for a control's
// aria-describedby. Empty when forID is empty.
func HelpID(forID string) string {
	if forID == "" {
		return ""
	}
	return forID + "-help"
}

// ErrorID returns the DOM id of a field's error text, for aria-describedby.
func ErrorID(forID string) string {
	if forID == "" {
		return ""
	}
	return forID + "-error"
}

// RenderField returns the labeled field as an HTML string.
func RenderField(f Field) string {
	var b strings.Builder
	b.WriteString(`<div class="flex flex-col gap-1">`)

	if f.Label != "" {
		fmt.Fprintf(&b, `<label class="text-sm font-medium text-ink"%s>%s`,
			attr("for", f.For), esc(f.Label))
		if f.Required {
			b.WriteString(`<span class="text-danger" aria-hidden="true"> *</span>`)
		}
		b.WriteString(`</label>`)
	}

	b.WriteString(string(f.Control))

	switch {
	case f.Error != "":
		fmt.Fprintf(&b, `<p class="text-sm text-danger"%s>%s</p>`,
			attr("id", ErrorID(f.For)), esc(f.Error))
	case f.Help != "":
		fmt.Fprintf(&b, `<p class="text-sm text-ink-muted"%s>%s</p>`,
			attr("id", HelpID(f.For)), esc(f.Help))
	}

	b.WriteString(`</div>`)
	return b.String()
}

// ── shared control helpers ────────────────────────────────────────────────────

// controlClass is the shared input/select/textarea surface. Invalid swaps the
// border + focus ring to danger.
func controlClass(invalid bool) string {
	base := "w-full rounded-md border bg-surface px-3 py-2 text-sm text-ink placeholder:text-ink-faint focus:outline-none focus:ring-2 "
	if invalid {
		return base + "border-danger focus:ring-danger"
	}
	return base + "border-line focus:ring-focus"
}

// ariaWiring writes the shared required/disabled/invalid/describedby attributes
// for a control with the given id.
func ariaWiring(b *strings.Builder, id string, required, disabled, invalid, hasHelp bool) {
	b.WriteString(boolAttr("required", required))
	b.WriteString(boolAttr(`aria-required="true"`, required))
	b.WriteString(boolAttr("disabled", disabled))
	b.WriteString(boolAttr(`aria-invalid="true"`, invalid))
	switch {
	case invalid:
		b.WriteString(attr("aria-describedby", ErrorID(id)))
	case hasHelp:
		b.WriteString(attr("aria-describedby", HelpID(id)))
	}
}

// ── Input ─────────────────────────────────────────────────────────────────────

// Input is a single-line text-like control (text, email, number, date, …).
type Input struct {
	ID, Name     string
	Type         string // default "text"
	Value        string
	Placeholder  string
	Autocomplete string
	Required     bool
	Disabled     bool
	Invalid      bool // danger styling + aria-invalid
	HasHelp      bool // wire aria-describedby to the field's help id
}

// RenderInput returns the input control as an HTML string.
func RenderInput(in Input) string {
	typ := in.Type
	if typ == "" {
		typ = "text"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<input type="%s" class="%s"`, esc(typ), controlClass(in.Invalid))
	b.WriteString(attr("id", in.ID))
	b.WriteString(attr("name", in.Name))
	b.WriteString(attr("value", in.Value))
	b.WriteString(attr("placeholder", in.Placeholder))
	b.WriteString(attr("autocomplete", in.Autocomplete))
	ariaWiring(&b, in.ID, in.Required, in.Disabled, in.Invalid, in.HasHelp)
	b.WriteString(`/>`)
	return b.String()
}

// ── Textarea ──────────────────────────────────────────────────────────────────

// Textarea is a multi-line text control.
type Textarea struct {
	ID, Name    string
	Value       string
	Placeholder string
	Rows        int // default 4
	Required    bool
	Disabled    bool
	Invalid     bool
	HasHelp     bool
}

// RenderTextarea returns the textarea control as an HTML string.
func RenderTextarea(t Textarea) string {
	rows := t.Rows
	if rows <= 0 {
		rows = 4
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<textarea class="%s" rows="%d"`, controlClass(t.Invalid), rows)
	b.WriteString(attr("id", t.ID))
	b.WriteString(attr("name", t.Name))
	b.WriteString(attr("placeholder", t.Placeholder))
	ariaWiring(&b, t.ID, t.Required, t.Disabled, t.Invalid, t.HasHelp)
	fmt.Fprintf(&b, `>%s</textarea>`, esc(t.Value))
	return b.String()
}

// ── Select ────────────────────────────────────────────────────────────────────

// Option is one choice in a Select or RadioGroup.
type Option struct {
	Value, Label string
	Disabled     bool
}

// Select is a dropdown control. When Placeholder is set, a disabled empty-value
// option is rendered first and exclusively selected when Value is empty.
// An unavailable nonempty Value selects that prompt (or "Selection unavailable")
// rather than silently selecting a different option. Apps own validation messages.
type Select struct {
	ID, Name    string
	Value       string // the currently selected option value
	Placeholder string // optional empty-value prompt (e.g. "Select…")
	Options     []Option
	Required    bool
	Disabled    bool
	Invalid     bool
	HasHelp     bool
}

// RenderSelect returns the select control as an HTML string.
func RenderSelect(s Select) string {
	unavailable := s.Value != ""
	for _, o := range s.Options {
		if o.Value == s.Value {
			unavailable = false
			break
		}
	}
	prompt := s.Placeholder
	if unavailable && prompt == "" {
		prompt = "Selection unavailable"
	}
	selectPrompt := prompt != "" && (s.Value == "" || unavailable)
	var b strings.Builder
	fmt.Fprintf(&b, `<select class="%s"`, controlClass(s.Invalid))
	b.WriteString(attr("id", s.ID))
	b.WriteString(attr("name", s.Name))
	ariaWiring(&b, s.ID, s.Required, s.Disabled, s.Invalid, s.HasHelp)
	b.WriteString(`>`)
	if prompt != "" {
		sel := ""
		if selectPrompt {
			sel = " selected"
		}
		fmt.Fprintf(&b, `<option value="" disabled%s>%s</option>`, sel, esc(prompt))
	}
	for _, o := range s.Options {
		sel := ""
		if !selectPrompt && o.Value == s.Value {
			sel = " selected"
		}
		fmt.Fprintf(&b, `<option value="%s"%s%s>%s</option>`,
			esc(o.Value), sel, boolAttr("disabled", o.Disabled), esc(o.Label))
	}
	b.WriteString(`</select>`)
	return b.String()
}

// ── Checkbox ──────────────────────────────────────────────────────────────────

// Checkbox is a single boolean control with an inline label.
type Checkbox struct {
	ID, Name string
	Value    string // submitted value when checked (default "true")
	Label    string
	Checked  bool
	Required bool
	Disabled bool
	Invalid  bool // danger styling + aria-invalid; describes ErrorID(ID)
	HasHelp  bool // describes HelpID(ID) when not invalid
}

// RenderCheckbox returns the checkbox + inline label as an HTML string.
func RenderCheckbox(c Checkbox) string {
	val := c.Value
	if val == "" {
		val = "true"
	}
	class := "rounded border-line text-accent focus:ring-2 focus:ring-focus"
	if c.Invalid {
		class = "rounded border-danger text-accent focus:ring-2 focus:ring-danger"
	}
	var b strings.Builder
	b.WriteString(`<label class="flex items-center gap-2 text-sm text-ink">`)
	fmt.Fprintf(&b, `<input type="checkbox" class="%s"`, class)
	b.WriteString(attr("id", c.ID))
	b.WriteString(attr("name", c.Name))
	b.WriteString(attr("value", val))
	b.WriteString(boolAttr("checked", c.Checked))
	ariaWiring(&b, c.ID, c.Required, c.Disabled, c.Invalid, c.HasHelp)
	b.WriteString(`/>`)
	b.WriteString(esc(c.Label))
	b.WriteString(`</label>`)
	return b.String()
}

// ── RadioGroup ────────────────────────────────────────────────────────────────

// RadioGroup is a set of mutually-exclusive options rendered as a fieldset with
// a legend (the accessible group label) — use this instead of Field for radios.
type RadioGroup struct {
	Name     string // shared control name (required for grouping)
	Legend   string // the group's accessible label
	Value    string // the selected option value
	Options  []Option
	Required bool
	Disabled bool
	Help     string // optional help text below the options
	Error    string // optional error below the options
	Inline   bool   // lay options out in a row instead of stacked
}

// RenderRadioGroup returns the fieldset of radios as an HTML string.
// A nonempty Name also identifies the active help/error message. Callers must
// keep group names and the resulting control/message IDs unique in the document.
func RenderRadioGroup(g RadioGroup) string {
	var b strings.Builder
	feedbackID, feedbackAttr := "", ""
	if g.Name != "" {
		switch {
		case g.Error != "":
			feedbackID = fmt.Sprintf("ak-radio-%x-error", g.Name)
		case g.Help != "":
			feedbackID = fmt.Sprintf("ak-radio-%x-help", g.Name)
		}
	}
	if feedbackID != "" {
		feedbackAttr = ` aria-describedby="` + feedbackID + `"`
	}
	classes := "border-line text-accent focus:ring-2 focus:ring-focus"
	if g.Error != "" {
		feedbackAttr += ` aria-invalid="true"`
		classes = "border-danger text-accent focus:ring-2 focus:ring-danger"
	}
	b.WriteString(`<fieldset class="m-0 flex min-w-0 flex-col gap-2 border-0 p-0"`)
	b.WriteString(boolAttr("disabled", g.Disabled))
	b.WriteString(`>`)
	if g.Legend != "" {
		b.WriteString(`<legend class="text-sm font-medium text-ink">`)
		b.WriteString(esc(g.Legend))
		if g.Required {
			b.WriteString(`<span class="text-danger" aria-hidden="true"> *</span>`)
		}
		b.WriteString(`</legend>`)
	}
	wrap := "flex flex-col gap-2"
	if g.Inline {
		wrap = "flex flex-wrap gap-4"
	}
	fmt.Fprintf(&b, `<div class="%s">`, wrap)
	for i, o := range g.Options {
		id := fmt.Sprintf("%s-%d", g.Name, i)
		sel := ""
		if o.Value == g.Value {
			sel = " checked"
		}
		fmt.Fprintf(&b, `<label class="flex items-center gap-2 text-sm text-ink"><input type="radio" class="%s" id="%s" name="%s" value="%s"%s%s%s%s/>%s</label>`,
			classes, esc(id), esc(g.Name), esc(o.Value), sel,
			boolAttr("required", g.Required && i == 0), boolAttr("disabled", o.Disabled), feedbackAttr, esc(o.Label))
	}
	b.WriteString(`</div>`)
	messageAttr := ""
	if feedbackID != "" {
		messageAttr = ` id="` + feedbackID + `"`
	}
	switch {
	case g.Error != "":
		fmt.Fprintf(&b, `<p class="text-sm text-danger"%s>%s</p>`, messageAttr, esc(g.Error))
	case g.Help != "":
		fmt.Fprintf(&b, `<p class="text-sm text-ink-muted"%s>%s</p>`, messageAttr, esc(g.Help))
	}
	b.WriteString(`</fieldset>`)
	return b.String()
}

// ── Button ────────────────────────────────────────────────────────────────────

// ButtonVariant selects a button's visual weight.
type ButtonVariant string

const (
	ButtonPrimary ButtonVariant = "primary" // accent fill — the main action
	ButtonGhost   ButtonVariant = "ghost"   // bordered, low-emphasis
	ButtonDanger  ButtonVariant = "danger"  // destructive action
)

// Button is an action control. A nonempty Href selects link mode: enabled safe
// destinations render an anchor, otherwise a noninteractive disabled link label.
// An empty Href renders a <button> of the given Type (default "submit").
type Button struct {
	Label    string
	Variant  ButtonVariant
	Type     string // button type when not a link; default "submit"
	Href     string // relative URL or absolute HTTP(S); route authorization stays app-owned
	Name     string // optional form field name (e.g. formaction multi-submit)
	Value    string
	Disabled bool
	Full     bool // full-width
}

func buttonClass(v ButtonVariant, full bool) string {
	base := "inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium focus:outline-none focus:ring-2 focus:ring-focus disabled:opacity-50 "
	switch v {
	case ButtonGhost:
		base += "border border-line bg-surface text-ink hover:bg-surface-sunken"
	case ButtonDanger:
		base += "bg-danger text-ink-inverse hover:opacity-90"
	default: // primary
		base += "bg-accent text-accent-ink hover:bg-accent-hover"
	}
	if full {
		base += " w-full"
	}
	return base
}

// RenderButton returns the button (or link styled as a button) as an HTML string.
func RenderButton(btn Button) string {
	cls := buttonClass(btn.Variant, btn.Full)
	if btn.Href != "" {
		if btn.Disabled || !webref.Allowed(btn.Href) {
			// A refused link must not become an implicit form submit. Do not keep
			// href or tabindex: neither pointer nor keyboard can activate it.
			classes := strings.Fields(cls)
			inert := classes[:0]
			for _, c := range classes {
				if !strings.HasPrefix(c, "hover:") && !strings.HasPrefix(c, "focus:") && !strings.HasPrefix(c, "disabled:") {
					inert = append(inert, c)
				}
			}
			return fmt.Sprintf(`<span role="link" aria-disabled="true" class="%s opacity-50 cursor-not-allowed">%s</span>`, strings.Join(inert, " "), esc(btn.Label))
		}
		return fmt.Sprintf(`<a class="%s"%s>%s</a>`, cls, attr("href", btn.Href), esc(btn.Label))
	}
	typ := btn.Type
	if typ == "" {
		typ = "submit"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<button type="%s" class="%s"`, esc(typ), cls)
	b.WriteString(attr("name", btn.Name))
	b.WriteString(attr("value", btn.Value))
	b.WriteString(boolAttr("disabled", btn.Disabled))
	fmt.Fprintf(&b, `>%s</button>`, esc(btn.Label))
	return b.String()
}
