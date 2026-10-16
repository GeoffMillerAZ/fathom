// Package live tells a reader that a server-rendered region has newer data,
// without re-rendering or moving it.
//
// Script and Handler deliver live.js, a dependency-free L2 classic script with
// one global, window.AppKitLive. It reads an app-supplied server-sent event
// stream and matches each event to regions carrying data-live-source. A matched
// region keeps its content; only its reserved status line changes from the
// app's "as of" text to the same text plus "Updated — refresh this panel", and
// one polite live region announces the change. Refreshing is an ordinary page
// load that the app renders; the script never fetches or inserts region HTML.
//
// RenderRegion, RegionAttrs, RenderStatus and RenderPageStatus are L3b helpers:
// they emit semantic role utilities, so consumers scan appkit/ui/live/*.go.
// Without the script (or before it connects) the markup states the rendered
// freshness and "Live updates are off. Reload for newer data."; nothing claims
// live behavior that is not running. Neither marking a region nor any change of
// connection state resizes these helpers' markup, at any width: every changing
// text shares a grid cell with invisible copies of its alternatives.
//
// # Event contract
//
// The app owns event names, meaning and source keys. By default an event's data
// is JSON with "source" (a string) or "sources" (an array of strings); the
// source "*" marks every followed region, which a server can send when it can
// no longer replay a gap. AppKitLive.start({sources(message)}) maps any other
// payload shape. Only the configured event names are followed ("message" by
// default). Event data is never written into the document.
//
// # Connection contract
//
// The script uses fetch streaming, so every connection sends the last known id
// as the Last-Event-ID header and a server can replay what was missed. The
// header carries the id's UTF-8 bytes, as a browser's native SSE client sends
// them, so any id the stream sends round-trips. The first connection uses
// Page.LastEventID, the position the page was rendered at, so nothing published
// between rendering and subscribing is lost; an explicit start without
// lastEventId adopts it from the page status whose URL matches, and a start
// before the document is parsed connects once it is (regions and cursor exist).
//
// Failures reconnect with bounded exponential backoff and equal jitter (default
// 8 attempts, 30 s cap). The floor is baseDelay (1 s) or a larger server retry
// field; the first retry waits 1 to 1.5 times the floor, so clients that lost
// the same server spread out. A connection earns a fresh attempt budget only by
// staying open 10 s or by delivering an event that advances the cursor, so a
// server that accepts and then closes (even after a comment or retry preamble)
// still exhausts the attempts. A stream silent for idleTimeout (default 60 s)
// is treated as lost, so servers should send comment keepalives more often. A
// 204 response stops; other 4xx responses except 408 and 429 are unavailable
// without retrying. The stream pauses after the page is hidden for hiddenGrace
// (default 5 s) and resumes with Last-Event-ID when it is visible again.
// Credentials are same-origin; a cross-origin stream needs CORS that permits
// the Last-Event-ID request header.
//
// # Global and hooks
//
//	AppKitLive.start({url, events, sources, lastEventId, maxAttempts, baseDelay,
//	                  maxDelay, idleTimeout, hiddenGrace, announceDelay, maxEventBytes})
//	AppKitLive.start()    // resume after stop or unavailable, same configuration
//	AppKitLive.stop()     // manual stop: abort the stream, no reconnect
//	AppKitLive.state()    // {status, error, attempt, maxAttempts, url, lastEventId,
//	                      //  updated, ignored, unmatched, dropped}
//	AppKitLive.mark(src)  // mark regions following src as the stream would
//
// RenderPageStatus auto-starts the script from data-live-url, data-live-events
// and data-live-last-event-id when no explicit start ran first. Status is idle,
// connecting, live, reconnecting, paused, stopped or unavailable; window
// receives appkit:live-status on each change, including each retry attempt,
// and a region receives a bubbling appkit:live-updated when it is marked.
//
// # Keyboard contract
//
// The stop/resume control is one native button whose label follows the state,
// so focus stays on it; it is offered only while it can act (never after a
// configuration failure or in a browser without fetch streaming). The refresh
// link is a native link; when it targets the current document the script
// reloads it (a fragment-only navigation would not re-render) and, after the
// reload, focuses the region (tabindex -1) without scrolling, so the next Tab
// continues inside it and RenderRegion's focus-visible ring shows where focus
// is. Modified clicks keep native behavior. Nothing traps or moves focus
// otherwise.
package live

import (
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/internal/webref"
)

//go:embed live.js
var liveJS string

// Script returns the classic browser script, without a wrapping script tag.
// Loading it twice is harmless; the first global wins.
func Script() string { return liveJS }

// Handler serves the script with revalidation at a stable consumer-owned URL,
// including GET, HEAD and conditional requests.
func Handler() http.Handler {
	etag := fmt.Sprintf(`"%x"`, sha256.Sum256([]byte(liveJS)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
		w.Header().Set("ETag", etag)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.ServeContent(w, r, "live.js", time.Time{}, strings.NewReader(liveJS))
	})
}

// Bounds for typed configuration. They keep attributes, fragments and selectors
// unambiguous; they are not a statement about any app's naming scheme.
const (
	MaxSources   = 16
	MaxEvents    = 16
	MaxTextRunes = 200
)

var (
	idPattern     = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,127}$`)
	sourcePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+-]{0,127}$`)
	eventPattern  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._:-]{0,63}$`)
)

// Region describes one server-rendered area that follows app-owned sources.
type Region struct {
	// ID is the document-unique element id; the refresh link targets #ID.
	ID string
	// Sources are the app-owned source keys this region follows, such as
	// "plans:fathom". "*" is reserved for events and refused here.
	Sources []string
	// Label names the region in announcements and the refresh link's
	// accessible name, e.g. "Plan status for fathom".
	Label string
	// AsOf states the freshness of the rendered content, e.g. "As of 12:03 UTC".
	// It is required: a live region always says what it shows.
	AsOf string
	// RefreshHref is the page that re-renders this region (relative or
	// absolute HTTP(S), without a fragment). Empty means the current page.
	RefreshHref string
	// Body is reviewed, app-owned HTML. RenderRegion never escapes or alters it.
	Body template.HTML
}

// Validate reports the first invalid field. Render functions call it and
// produce no partial markup on error.
func (r Region) Validate() error {
	if !idPattern.MatchString(r.ID) {
		return fmt.Errorf("live: region ID %q must match %s", r.ID, idPattern)
	}
	if len(r.Sources) == 0 || len(r.Sources) > MaxSources {
		return fmt.Errorf("live: region %s needs 1 to %d sources", r.ID, MaxSources)
	}
	seen := map[string]bool{}
	for _, s := range r.Sources {
		if !sourcePattern.MatchString(s) {
			return fmt.Errorf("live: region %s source %q must match %s", r.ID, s, sourcePattern)
		}
		if seen[s] {
			return fmt.Errorf("live: region %s repeats source %q", r.ID, s)
		}
		seen[s] = true
	}
	if err := plainText("label", r.Label); err != nil {
		return fmt.Errorf("live: region %s %w", r.ID, err)
	}
	if err := plainText("as-of text", r.AsOf); err != nil {
		return fmt.Errorf("live: region %s %w", r.ID, err)
	}
	if r.RefreshHref != "" && !webURL(r.RefreshHref) {
		return fmt.Errorf("live: region %s refresh destination must be a relative or HTTP(S) URL without a fragment", r.ID)
	}
	return nil
}

// webURL reports whether s is a relative or HTTP(S) destination without a
// fragment that a browser's URL parser also accepts: webref's shape plus a
// port within 0 to 65535, which Go's parser leaves unbounded. A stream URL the
// browser refuses would leave the script unable to start.
func webURL(s string) bool {
	if s == "" || strings.Contains(s, "#") || !webref.Allowed(s) {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err != nil || n > 65535 {
			return false
		}
	}
	return true
}

func plainText(name, s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s is blank", name)
	}
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > MaxTextRunes {
		return fmt.Errorf("%s must be valid UTF-8 of at most %d characters", name, MaxTextRunes)
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return fmt.Errorf("%s contains a control character", name)
		}
	}
	return nil
}

func esc(s string) string { return html.EscapeString(s) }

func regionAttrs(r Region) string {
	return fmt.Sprintf(`id="%s" tabindex="-1" data-live-source="%s" data-live-label="%s"`,
		esc(r.ID), esc(strings.Join(r.Sources, " ")), esc(r.Label))
}

// RegionAttrs returns the attributes that make an app-owned wrapper element a
// live region: id, tabindex="-1" and the data-live-* hooks. Put RenderStatus
// output inside that element. The app supplies the wrapper's accessible name
// (for example a heading) and its focus-visible styling.
func RegionAttrs(r Region) (template.HTMLAttr, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	return template.HTMLAttr(regionAttrs(r)), nil
}

// RenderStatus returns the region's reserved status line. The "as of" layer and
// the updated layer share one grid cell and both start with the as-of text, so
// revealing the affordance changes no layout and the rendered freshness stays in
// place; the updated layer stays invisible (and out of the accessibility tree
// and tab order) until the script marks the region. The refresh link's vertical
// padding gives a 44px target inside the line's own padding.
func RenderStatus(r Region) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	return renderStatus(r, ""), nil
}

func renderStatus(r Region, spacing string) string {
	href := r.RefreshHref + "#" + r.ID
	var b strings.Builder
	fmt.Fprintf(&b, `<p data-live-status class="%sgrid py-3.5 text-sm text-ink-muted">`, spacing)
	fmt.Fprintf(&b, `<span data-live-asof class="col-start-1 row-start-1">%s</span>`, esc(r.AsOf))
	fmt.Fprintf(&b, `<span data-live-updated class="invisible col-start-1 row-start-1 text-ink">%s · <strong class="font-semibold">Updated</strong> — `, esc(r.AsOf))
	fmt.Fprintf(&b, `<a data-live-refresh href="%s" class="rounded-sm py-3.5 font-medium text-accent underline underline-offset-2 hover:text-accent-hover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">refresh this panel<span class="sr-only"> (%s)</span></a></span>`,
		esc(href), esc(r.Label))
	b.WriteString(`</p>`)
	return b.String()
}

// RenderRegion returns a labelled section wrapping the app's Body followed by
// its status line. The section is programmatically focusable for focus return
// after a refresh and then shows a focus-visible ring. It carries no
// focus:outline-none: under Tailwind v4 that sets the outline style variable
// the focus-visible ring reads, so the pair would paint nothing. A mouse click
// inside the section focuses it without matching :focus-visible, so no ring.
func RenderRegion(r Region) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<section %s aria-label="%s" class="min-w-0 scroll-mt-4 rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">`, regionAttrs(r), esc(r.Label))
	b.WriteString(string(r.Body))
	b.WriteString(renderStatus(r, "mt-3 "))
	b.WriteString(`</section>`)
	return b.String(), nil
}

// Page configures the page-level stream and status line.
type Page struct {
	// URL is the app-owned server-sent event endpoint (relative or absolute
	// HTTP(S), without a fragment).
	URL string
	// Events are the SSE event names to follow; empty means "message".
	Events []string
	// LastEventID is the stream position the page was rendered at. The first
	// connection sends it as Last-Event-ID (its UTF-8 bytes), so events
	// published between rendering and subscribing are replayed instead of lost.
	// Empty means the page has no position (it then only sees events after it
	// connects). It must not begin or end with a space or tab, which fetch
	// would strip from the header.
	LastEventID string
}

// Validate reports the first invalid field.
func (p Page) Validate() error {
	if !webURL(p.URL) {
		return errors.New("live: stream URL must be a relative or HTTP(S) URL without a fragment")
	}
	if len(p.LastEventID) > 256 || strings.ContainsAny(p.LastEventID, "\x00\r\n") || !utf8.ValidString(p.LastEventID) ||
		strings.Trim(p.LastEventID, " \t") != p.LastEventID {
		return errors.New("live: last event ID must be one valid unpadded line of at most 256 bytes")
	}
	if len(p.Events) > MaxEvents {
		return fmt.Errorf("live: at most %d event names", MaxEvents)
	}
	seen := map[string]bool{}
	for _, e := range p.Events {
		if !eventPattern.MatchString(e) {
			return fmt.Errorf("live: event name %q must match %s", e, eventPattern)
		}
		if seen[e] {
			return fmt.Errorf("live: event name %q repeats", e)
		}
		seen[e] = true
	}
	return nil
}

// offMessage is the connection line before, or without, the script.
const offMessage = "Live updates are off. Reload for newer data."

// connectionMessages is every text the connection line can show: offMessage
// plus live.js's MESSAGES with {attempt} and {max} at their widest (the
// maxAttempts bound, in tabular digits). RenderPageStatus stacks them
// invisibly in the line's grid cell, so the line always has the height of its
// tallest message and no state change moves the content below it. A test
// keeps this list and live.js identical.
var connectionMessages = []string{
	offMessage,
	"Connecting to live updates…",
	"Live updates on.",
	"Reconnecting (attempt 50 of 50)…",
	"Paused while this page is hidden.",
	"Live updates stopped. Reload for newer data.",
	"Live updates unavailable. Reload to retry.",
	"Live updates unavailable. Reload for newer data.",
}

// RenderPageStatus returns the page's connection line, the stop/resume control
// and the polite announcer. Render it once per page; the script auto-starts from
// its data-live-url. Its size never changes after rendering: the connection text
// shares a grid cell with an invisible copy of every message the script can
// write, the control stays invisible (keeping its box, out of the tab order and
// accessibility tree) until the script can use it and reserves its wider label,
// and the control wraps below the text from a fixed 10rem text basis, so the
// layout depends only on the container width. The control carries no display
// utility, so a hidden attribute an app adds still holds.
func RenderPageStatus(p Page) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<div data-live-page data-live-url="%s"`, esc(p.URL))
	if len(p.Events) > 0 {
		fmt.Fprintf(&b, ` data-live-events="%s"`, esc(strings.Join(p.Events, " ")))
	}
	if p.LastEventID != "" {
		fmt.Fprintf(&b, ` data-live-last-event-id="%s"`, esc(p.LastEventID))
	}
	b.WriteString(` class="flex min-h-11 flex-wrap items-center gap-x-3 gap-y-2 text-sm text-ink-muted">`)
	b.WriteString(`<div class="grid min-w-0 grow basis-40 tabular-nums">`)
	for _, m := range connectionMessages {
		fmt.Fprintf(&b, `<span aria-hidden="true" class="invisible col-start-1 row-start-1">%s</span>`, esc(m))
	}
	fmt.Fprintf(&b, `<p data-live-connection class="col-start-1 row-start-1 self-center">%s</p></div>`, esc(offMessage))
	b.WriteString(`<button type="button" data-live-toggle class="invisible min-h-11 shrink-0 rounded-md border border-line bg-surface px-3 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">`)
	b.WriteString(`<span class="grid"><span aria-hidden="true" class="invisible col-start-1 row-start-1">Resume live updates</span><span data-live-toggle-label class="col-start-1 row-start-1">Stop live updates</span></span></button>`)
	b.WriteString(`<div data-live-announcer role="status" aria-live="polite" aria-atomic="true" class="sr-only"></div>`)
	b.WriteString(`</div>`)
	return b.String(), nil
}
