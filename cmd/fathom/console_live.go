package main

// console_live.go — OC-14 (docs/design/observatory-console.md §2, §4): live
// freshness on the kit's live helper (appkit/ui/live, catalog
// interaction.live-freshness — FATHOM-KIT-06, which fathom originated and
// returned to the kit).
//
// Every panel carries its source name: a renderPanel panel names the
// sources it follows in data-console-source (fathom's key, e.g.
// "plans:fathom"; several keys are space-separated) and the same keys in the
// kit's data-live-source, on an element the kit makes a live region with a
// reserved status line. A page whose body is not yet built from panels
// (the pages F0 moved onto the shell unchanged) is one region following the
// sources its model reads. The page's status line (live.RenderPageStatus)
// starts the kit script against GET /events from the event-stream position
// the page was rendered at, so nothing published between the render and
// the subscription is lost.
//
// When an event names a source a region follows (console.Event.Sources, or
// "*" after a gap the replay cannot cover), the region's status line gains
// "Updated — refresh this panel": the same line, whose layers already share
// one grid cell, so content never moves under the reader, and the kit
// announces it once through a polite live region. Refreshing is an ordinary
// page load; no event carries content and the script never inserts any.

import (
	"html"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/live"
	"github.com/geoffmilleraz/fathom/internal/console"
)

// consoleLiveScriptPath serves live.js (live.Handler).
const consoleLiveScriptPath = "/assets/live.js"

// consoleLiveURL is the event stream the pages follow (OC-13).
const consoleLiveURL = "/events"

// consoleLiveEvents are the SSE event names a page follows: the five kinds
// the bus publishes plus the gap notice (console.GapEvent).
var consoleLiveEvents = []string{
	string(console.EventIngest), string(console.EventCompile), string(console.EventVerify),
	string(console.EventSession), string(console.EventDaemon), console.GapEvent,
}

// consolePageRegionID is the id of a page-level region.
const consolePageRegionID = "page-live"

// consoleRegionClass is a live region's own focus treatment: the kit's
// script focuses the region after a refresh, so a keyboard reader sees
// where it landed (the kit leaves the wrapper's styling to the app).
const consoleRegionClass = "min-w-0 scroll-mt-4 rounded-lg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"

// liveSources splits a data-console-source value into its keys.
func liveSources(keys string) []string { return strings.Fields(keys) }

// consoleLiveRegion is a panel's or page's region: the kit's region
// attributes plus fathom's data-console-source, and the reserved status
// line that states the rendered freshness. ok is false when the kit refuses
// the region (a key outside its grammar, a missing id or as-of text), in
// which case the caller renders the content without the live affordance
// rather than a broken one.
func consoleLiveRegion(id, label, asOf, keys string) (attrs, status string, ok bool) {
	r := live.Region{ID: id, Sources: liveSources(keys), Label: label, AsOf: asOf}
	a, err := live.RegionAttrs(r)
	if err != nil {
		return "", "", false
	}
	st, err := live.RenderStatus(r)
	if err != nil {
		return "", "", false
	}
	return string(a) + ` data-console-source="` + html.EscapeString(keys) + `"`, st, true
}

// consoleLivePage wraps a page body that is not built from panels in one
// region following the sources its page declares: the status line leads the
// body, where a reader sees it, and states when the page was rendered.
func consoleLivePage(title string, keys []string, now time.Time, body string) (string, bool) {
	if len(keys) == 0 {
		return body, false
	}
	attrs, status, ok := consoleLiveRegion(consolePageRegionID, title+" page", "Rendered "+now.UTC().Format("2006-01-02 15:04:05 UTC"), strings.Join(keys, " "))
	if !ok {
		return body, false
	}
	return `<div ` + attrs + ` class="` + consoleRegionClass + `">` + status + body + `</div>`, true
}

// consoleLiveStatus is the page's connection line and the kit script's
// start: the stream URL, the followed events and the stream position the
// page was rendered at ("<epoch>-<n>", console.EventBus.Cursor).
func consoleLiveStatus(cursor string) string {
	out, err := live.RenderPageStatus(live.Page{URL: consoleLiveURL, Events: consoleLiveEvents, LastEventID: cursor})
	if err != nil {
		return ""
	}
	return `<div class="mb-3" data-console-live>` + out + `</div>`
}

// pageLiveSources fills a page's declared live sources for one request: a
// "{svc}" in a key is the request's project.
func pageLiveSources(p *consolePage, cr *consoleRequest) []string {
	if len(p.Live) == 0 {
		return nil
	}
	svc := cr.r.PathValue("svc")
	out := make([]string, 0, len(p.Live))
	for _, k := range p.Live {
		if strings.Contains(k, "{svc}") {
			if svc == "" || !console.IsNameOrID(svc) {
				continue
			}
			k = strings.ReplaceAll(k, "{svc}", svc)
		}
		out = append(out, k)
	}
	return out
}
