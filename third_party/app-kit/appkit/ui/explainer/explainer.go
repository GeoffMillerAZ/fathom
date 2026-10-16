// Package explainer assembles a complete, self-contained visual explainer
// document — the "sell this concept on one page" artifact: a dark, technical,
// SVG-rich page a stakeholder or domain expert can open anywhere offline or
// export to a single PDF.
//
// It is the kit's one deliberately self-contained output: unlike the themed
// app components (which render fragments styled by the Tailwind token layer),
// explainer.Render emits an ENTIRE HTML document with its design system inlined,
// no build step, no external CSS/JS/fonts. That is exactly what a shareable
// design doc needs, and it is why the structure lived as hand-written HTML
// before this package. Now the structure is DRY Go: a Doc value of a few
// Sections becomes the page, and the recurring chrome (the token theme, the
// reveal/anchor styling, and the export toolbar from ui/export) comes for free.
//
// Pure Go, no Node (D2). The only JS is the opt-in export toolbar (ui/export),
// which is plain page JS — loopback-safe (D5). Plain-text fields are escaped;
// fields documented as "trusted HTML" (Lead, Section.Body, Foot) are emitted
// verbatim so callers can compose figures, tables, and links with the helpers.
//
// # Motion: why this package does not use appkit/ui/motion
//
// It is the one exception, stated so the kit does not end up with two motion
// idioms and no reason. appkit/ui/motion ships a shared stylesheet an app
// @imports or serves, and every kit component that animates composes it. This
// package cannot: its entire contract is a single file with no external CSS,
// so importing the layer would break the property the package exists for. Its
// animations are also a different genre — two decorative infinite loops (the
// live dot, the marching-ants dash), not an interpolation between two states.
//
// What is NOT exempt is the invariant the layer enforces: both states legible
// without motion, and a prefers-reduced-motion escape. baseCSS therefore
// carries its own escape for both loops and for scroll-behavior. Any new
// animation added here must extend that block. A component that renders a
// fragment into a themed app has no such excuse and composes appkit/ui/motion.
//
// The escape uses animation:none, not motion.css's 1ms. That divergence is
// deliberate: motion.css argues for 1ms because a 0s/none transition never
// fires transitionend, hanging any JS awaiting it. Both animations here are
// infinite decorative loops — the pulsing dot, the marching-ants dash — with
// nothing awaiting their end, which is the one case where none is safe and 1ms
// would only keep a pointless loop spinning. TestRender_ReducedMotionEscape
// pins both the escape and the static fallbacks it leaves legible.
package explainer

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/export"
)

// Doc is a whole explainer page.
type Doc struct {
	Title    string         // <title>; also the H1 when Codename is empty
	Codename string         // the large H1 (e.g. "canon")
	Eyebrow  string         // the uppercase tagline above the title (a pulsing live dot precedes it)
	Lead     string         // trusted HTML: the 1-2 sentence value statement
	Oneliner string         // plain text: a single monospace end-to-end flow line
	Accent   string         // #hex primary accent; default the kit cyan
	Accent2  string         // #hex secondary accent; default = Accent
	Roles    []Role         // the context strip placing the product in its suite
	Sections []Section      // the body, in order
	Foot     string         // trusted HTML: the footer (cross-links, sources)
	Export   export.Options // the export toolbar (zero value = both PDF + HTML buttons)
}

// Role is one card in the context strip.
type Role struct {
	Kind    string // the small mono label (e.g. "THE RUNNER")
	Name    string // the bold name
	Note    string // the trailing description
	Current bool   // highlight this card as the current product
}

// Section is one labeled block of the page.
type Section struct {
	Label string // the mono uppercase h2 label
	Title string // the section headline
	Intro string // optional trusted-HTML lead paragraph
	Body  string // trusted HTML: figures/grids/tables, built with the helpers below
}

const (
	defaultAccent  = "#4fd1c9"
	defaultAccent2 = "#5fe3d6"
)

func (d Doc) norm() Doc {
	if d.Accent == "" {
		d.Accent = defaultAccent
	}
	if d.Accent2 == "" {
		d.Accent2 = d.Accent
	}
	if d.Title == "" {
		d.Title = d.Codename
	}
	return d
}

func (d Doc) heading() string {
	if d.Codename != "" {
		return d.Codename
	}
	return d.Title
}

// Render returns the complete self-contained HTML document.
func Render(d Doc) string {
	d = d.norm()
	css := strings.ReplaceAll(baseCSS, "__ACCENT__", d.Accent)
	css = strings.ReplaceAll(css, "__ACCENT2__", d.Accent2)

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", esc(d.Title))
	b.WriteString("<style>\n" + css + "\n</style>\n</head>\n<body>\n<div class=\"wrap\">\n")

	if d.Eyebrow != "" {
		fmt.Fprintf(&b, "  <div class=\"eyebrow\"><span class=\"live\"></span> %s</div>\n", esc(d.Eyebrow))
	}
	fmt.Fprintf(&b, "  <h1>%s</h1>\n", esc(d.heading()))
	if d.Lead != "" {
		fmt.Fprintf(&b, "  <p class=\"lead\">%s</p>\n", d.Lead)
	}
	if d.Oneliner != "" {
		fmt.Fprintf(&b, "  <div class=\"oneliner\">%s</div>\n", esc(d.Oneliner))
	}
	if len(d.Roles) > 0 {
		b.WriteString(renderRoles(d.Roles))
	}
	for _, s := range d.Sections {
		b.WriteString(renderSection(s))
	}
	if d.Foot != "" {
		fmt.Fprintf(&b, "  <div class=\"foot\">%s</div>\n", d.Foot)
	}
	b.WriteString("</div>\n")
	b.WriteString(export.Toolbar(d.Export))
	b.WriteString("\n</body>\n</html>\n")
	return b.String()
}

func renderRoles(roles []Role) string {
	var b strings.Builder
	b.WriteString("  <div class=\"roles\">\n")
	for _, r := range roles {
		cls := "role"
		if r.Current {
			cls = "role cur"
		}
		fmt.Fprintf(&b, "    <div class=\"%s\"><span class=\"k\">%s</span><br><b>%s</b> <span class=\"sub\">%s</span></div>\n",
			cls, esc(r.Kind), esc(r.Name), esc(r.Note))
	}
	b.WriteString("  </div>\n")
	return b.String()
}

func renderSection(s Section) string {
	var b strings.Builder
	if s.Label != "" {
		fmt.Fprintf(&b, "  <h2>%s</h2>\n", esc(s.Label))
	}
	if s.Title != "" {
		fmt.Fprintf(&b, "  <div class=\"st\">%s</div>\n", esc(s.Title))
	}
	if s.Intro != "" {
		fmt.Fprintf(&b, "  <p class=\"sub\">%s</p>\n", s.Intro)
	}
	if s.Body != "" {
		b.WriteString("  " + s.Body + "\n")
	}
	return b.String()
}

// --- body helpers (compose these into Section.Body) -----------------------------------------------

// SVG wraps inner markup in the standard responsive explainer canvas (viewBox
// 0 0 1080 height). Provide your own <defs>/markers/nodes as inner.
func SVG(height int, inner string) string {
	return `<svg viewBox="0 0 1080 ` + strconv.Itoa(height) + `" width="100%" height="` + strconv.Itoa(height) + `">` + inner + `</svg>`
}

// Figure wraps an SVG (or any block) with a centered caption. caption is trusted HTML.
func Figure(body, caption string) string {
	return `<figure>` + body + `<figcaption>` + caption + `</figcaption></figure>`
}

// Grid lays cards out in the responsive auto-fit grid.
func Grid(cards ...string) string {
	return `<div class="grid">` + strings.Join(cards, "") + `</div>`
}

// Card renders a titled card with bullet points and an optional value callout.
// title and bullets are plain text (escaped); bullets may contain <b>…</b> via
// the trusted Bullets variant if needed — keep these plain for safety.
func Card(title string, bullets []string, value string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="card"><h4>%s</h4><ul>`, esc(title))
	for _, li := range bullets {
		fmt.Fprintf(&b, `<li>%s</li>`, esc(li))
	}
	b.WriteString(`</ul>`)
	if value != "" {
		b.WriteString(Value(value))
	}
	b.WriteString(`</div>`)
	return b.String()
}

// Value renders a "✓ …" payoff callout. text is plain (escaped).
func Value(text string) string {
	return `<span class="val">✓ ` + esc(text) + `</span>`
}

// Table renders a bordered table. Cells are plain text (escaped).
func Table(headers []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString(`<table><tr>`)
	for _, h := range headers {
		fmt.Fprintf(&b, `<th>%s</th>`, esc(h))
	}
	b.WriteString(`</tr>`)
	for _, row := range rows {
		b.WriteString(`<tr>`)
		for _, c := range row {
			fmt.Fprintf(&b, `<td>%s</td>`, esc(c))
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</table>`)
	return b.String()
}

func esc(s string) string { return html.EscapeString(s) }

// baseCSS is the inlined design system (the dark, technical "aerospace" theme).
// __ACCENT__ / __ACCENT2__ are substituted with the doc's accents by Render. It
// uses ReplaceAll (not fmt) precisely because the CSS is full of % literals.
//
// The last rule is the reduced-motion escape (see the package doc for why this
// file cannot just use appkit/ui/motion). Both animations here are decorative
// loops on top of an already-legible mark — the live dot is a green dot without
// its pulse, the dash is a dashed stroke without its march — so cancelling them
// removes ornament, never meaning. Keep it that way: anything animated here
// must read correctly with the animation switched off.
const baseCSS = `:root{
  --bg:#06100f; --panel:#0c1a1a; --panel2:#0f2120; --ink:#e3f3f0; --mut:#92b8b3; --dim:#5f8580;
  --line:#1f3a37; --faint:#132a27;
  --cyan:__ACCENT__; --teal:__ACCENT2__; --green:#6ee7a8; --amber:#f4b860; --red:#ff7a8a; --violet:#a594ff; --blue:#7fb8ff;
  --mono:"SFMono-Regular",ui-monospace,Menlo,Consolas,monospace;
  --sans:-apple-system,BlinkMacSystemFont,"Segoe UI",Inter,system-ui,sans-serif;
}
*{box-sizing:border-box} html{scroll-behavior:smooth}
body{margin:0;background:var(--bg);color:var(--ink);font-family:var(--sans);line-height:1.55;
  background-image:radial-gradient(1100px 650px at 75% -10%, rgba(79,209,201,.10), transparent 60%),
    linear-gradient(rgba(31,58,55,.16) 1px,transparent 1px),linear-gradient(90deg,rgba(31,58,55,.16) 1px,transparent 1px);
  background-size:auto,48px 48px,48px 48px;}
.wrap{max-width:1120px;margin:0 auto;padding:30px 26px 90px}
.eyebrow{font-family:var(--mono);font-size:12px;color:var(--cyan);letter-spacing:.14em;text-transform:uppercase;display:flex;align-items:center;gap:9px}
.live{width:8px;height:8px;border-radius:50%;background:var(--green);animation:pulse 2.4s infinite}
@keyframes pulse{0%{box-shadow:0 0 0 0 rgba(110,231,168,.5)}70%{box-shadow:0 0 0 7px rgba(110,231,168,0)}100%{box-shadow:0 0 0 0 rgba(110,231,168,0)}}
h1{font-size:clamp(2.3rem,5.5vw,3.8rem);line-height:1.02;letter-spacing:-.02em;margin:14px 0 0;font-weight:680}
.lead{color:var(--mut);font-size:clamp(1.02rem,1.9vw,1.28rem);max-width:50rem;margin:18px 0 0}
.lead b{color:var(--ink)}
.oneliner{margin:22px 0 0;background:var(--panel);border:1px solid var(--line);border-left:3px solid var(--cyan);border-radius:12px;padding:13px 16px;font-family:var(--mono);font-size:12px;color:var(--teal);overflow-x:auto;white-space:pre-wrap;overflow-wrap:break-word}
h2{font-size:12.5px;font-family:var(--mono);text-transform:uppercase;letter-spacing:.16em;color:var(--dim);margin:60px 0 4px;padding-top:16px;border-top:1px solid var(--line)}
.st{font-size:clamp(1.35rem,2.7vw,1.85rem);font-weight:650;margin:2px 0 4px;letter-spacing:-.01em}
p{margin:9px 0} .sub{color:var(--mut);font-size:14px}
figure{margin:18px 0 0;background:linear-gradient(180deg,var(--panel),#081413);border:1px solid var(--line);border-radius:16px;padding:14px;overflow:hidden}
figcaption{color:var(--mut);font-size:12.5px;margin-top:10px;text-align:center;line-height:1.5}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:13px;margin:16px 0 0}
.grid2{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin:16px 0 0}
@media(max-width:760px){.grid2{grid-template-columns:1fr}}
.card{background:var(--panel2);border:1px solid var(--line);border-radius:13px;padding:15px 17px}
.card h4{margin:0 0 7px;font-size:13.5px;font-family:var(--mono);color:var(--teal)}
.card ul{margin:7px 0 0;padding-left:16px} .card li{font-size:12.8px;color:var(--mut);margin:4px 0} .card li b{color:var(--ink)}
.val{display:inline-block;margin-top:10px;font-size:12px;color:var(--green);border:1px solid var(--faint);background:#0c1f1c;border-radius:7px;padding:5px 9px}
.cy{color:var(--cyan)} .gr{color:var(--green)} .am{color:var(--amber)} .rd{color:var(--red)} .vi{color:var(--violet)} .mut{color:var(--mut)} .te{color:var(--teal)} .bl{color:var(--blue)}
.mono{font-family:var(--mono)}
table{border-collapse:collapse;width:100%;margin:14px 0 0;font-size:12.5px}
th,td{border:1px solid var(--line);padding:8px 11px;text-align:left;vertical-align:top}
th{background:#0c1f1c;color:var(--teal);font-family:var(--mono);font-size:11px;text-transform:uppercase;letter-spacing:.05em}
td b{color:var(--ink)}
svg text{font-family:var(--mono)}
.dash{stroke-dasharray:6 6;animation:flow 1.1s linear infinite}@keyframes flow{to{stroke-dashoffset:-24}}
.roles{display:flex;gap:10px;flex-wrap:wrap;margin:20px 0 0}
.role{flex:1;min-width:200px;background:var(--panel);border:1px solid var(--line);border-radius:11px;padding:11px 14px}
.role.cur{border-color:var(--cyan)} .role .k{font-family:var(--mono);font-size:10.5px;letter-spacing:.08em;color:var(--dim)}
.role b{color:var(--ink)}
.foot{margin-top:54px;border-top:1px solid var(--line);padding-top:20px;color:var(--dim);font-size:12px;font-family:var(--mono)}
.foot a{color:var(--mut);text-decoration:none} .foot a:hover{color:var(--cyan)}
@media(prefers-reduced-motion:reduce){html{scroll-behavior:auto}.live{animation:none}.dash{animation:none}}`
