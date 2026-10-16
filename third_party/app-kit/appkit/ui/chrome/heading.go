package chrome

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/internal/webref"
)

// PageHeading is the top of a page: an optional breadcrumb trail, an optional
// eyebrow, the page's single h1, an optional meta line and optional actions.
// Every text field is plain text and escaped; there is no trusted HTML slot.
type PageHeading struct {
	// ID optionally names the h1 (for aria-labelledby or a skip target).
	ID string
	// Breadcrumbs render through Breadcrumbs above the heading; empty omits them.
	// Each non-empty Href must be a relative or absolute HTTP(S) destination;
	// Breadcrumbs writes it as a link, so any other destination (javascript:,
	// data:, protocol-relative, padded) is refused rather than rendered.
	Breadcrumbs []Crumb
	// Eyebrow is a short context label shown above the title, e.g. "Project".
	Eyebrow string
	// Title is the h1 text. Required.
	Title string
	// Meta items form one wrapping line below the title, e.g. "12 plans".
	Meta []string
	// Actions render unchanged through the form package's RenderButton: an
	// Href makes a link (unsafe destinations become inert labels), otherwise a
	// native button of the given Type (default "submit", which only acts
	// inside an app-owned form; use Type "button" for script-owned actions).
	Actions []form.Button
}

// Bounds keep the heading one reviewable page element, not a layout engine.
const (
	MaxHeadingMeta    = 8
	MaxHeadingActions = 4
	maxHeadingRunes   = 300
)

var headingIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,127}$`)

// Validate reports the first invalid field. RenderPageHeading calls it and
// writes nothing on error.
func (h PageHeading) Validate() error {
	if h.ID != "" && !headingIDPattern.MatchString(h.ID) {
		return fmt.Errorf("chrome: heading ID %q must match %s", h.ID, headingIDPattern)
	}
	if err := headingText("title", h.Title, true); err != nil {
		return err
	}
	if err := headingText("eyebrow", h.Eyebrow, false); err != nil {
		return err
	}
	if len(h.Meta) > MaxHeadingMeta {
		return fmt.Errorf("chrome: at most %d heading meta items", MaxHeadingMeta)
	}
	for _, m := range h.Meta {
		if err := headingText("meta item", m, true); err != nil {
			return err
		}
	}
	if len(h.Actions) > MaxHeadingActions {
		return fmt.Errorf("chrome: at most %d heading actions", MaxHeadingActions)
	}
	for _, a := range h.Actions {
		if strings.TrimSpace(a.Label) == "" {
			return errors.New("chrome: heading action label is blank")
		}
	}
	for _, c := range h.Breadcrumbs {
		if strings.TrimSpace(c.Label) == "" {
			return errors.New("chrome: breadcrumb label is blank")
		}
		if c.Href != "" && !webref.Allowed(c.Href) {
			return fmt.Errorf("chrome: breadcrumb %q destination must be a relative or HTTP(S) URL", c.Label)
		}
	}
	return nil
}

func headingText(name, s string, required bool) error {
	if s == "" && !required {
		return nil
	}
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("chrome: heading %s is blank", name)
	}
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > maxHeadingRunes {
		return fmt.Errorf("chrome: heading %s must be valid UTF-8 of at most %d characters", name, maxHeadingRunes)
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return fmt.Errorf("chrome: heading %s contains a control character", name)
		}
	}
	return nil
}

// RenderPageHeading returns the heading as an HTML string. The title column
// shrinks and wraps long words; below the sm breakpoint the actions wrap onto
// their own row beneath the title, and from sm up they sit at the row's end.
// Action targets get a 44px minimum height. It emits role utilities, so scan
// appkit/ui/chrome/*.go and appkit/ui/form/*.go.
func RenderPageHeading(h PageHeading) (string, error) {
	if err := h.Validate(); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<header data-page-heading class="flex min-w-0 flex-col gap-3">`)
	b.WriteString(Breadcrumbs(h.Breadcrumbs))
	b.WriteString(`<div class="flex min-w-0 flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">`)
	b.WriteString(`<div class="min-w-0 flex-1">`)
	if h.Eyebrow != "" {
		fmt.Fprintf(&b, `<p class="text-sm font-medium text-accent">%s</p>`, esc(h.Eyebrow))
	}
	b.WriteString(`<h1`)
	if h.ID != "" {
		fmt.Fprintf(&b, ` id="%s"`, esc(h.ID))
	}
	b.WriteString(` class="text-2xl font-semibold text-ink [overflow-wrap:anywhere]`)
	if h.Eyebrow != "" {
		b.WriteString(` mt-1`)
	}
	fmt.Fprintf(&b, `">%s</h1>`, esc(h.Title))
	if len(h.Meta) > 0 {
		b.WriteString(`<ul role="list" class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-ink-muted">`)
		for _, m := range h.Meta {
			fmt.Fprintf(&b, `<li class="min-w-0 [overflow-wrap:anywhere]">%s</li>`, esc(m))
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`</div>`)
	if len(h.Actions) > 0 {
		b.WriteString(`<div class="flex flex-wrap items-center gap-2 sm:shrink-0 *:min-h-11">`)
		for _, a := range h.Actions {
			b.WriteString(form.RenderButton(a))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div></header>`)
	return b.String(), nil
}
