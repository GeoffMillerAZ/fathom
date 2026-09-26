package gallery

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/live"
)

// The live-freshness specimen is a tiny synthetic SSE fixture: a bounded replay
// window, bounded subscribers and explicit fault controls. It exists to exercise
// appkit/ui/live in a real browser; it is not a reusable event bus or a claim
// about any app's event names, retention or authorization.
const (
	liveMaxStreams   = 8
	liveReplayWindow = 32
	liveMaxOutage    = 8
	liveHeartbeat    = 15 * time.Second
	liveWriteTimeout = 5 * time.Second
	liveEventName    = "change"
	liveStreamPath   = "/live/events"
)

type liveSource struct {
	Key, Title, Label string
}

var liveSources = []liveSource{
	{Key: "plans:fixture", Title: "Plans", Label: "Plan status"},
	{Key: "index:fixture", Title: "Index", Label: "Index freshness"},
	{Key: "sessions:fixture", Title: "Sessions", Label: "Recent sessions"},
}

type liveEvent struct {
	ID      uint64
	Sources []string
}

type liveFixture struct {
	mu       sync.Mutex
	next     uint64
	events   []liveEvent
	revision map[string]int
	changed  chan struct{}
	dropped  chan struct{}
	streams  int
	failNext int
	now      func() time.Time
}

func newLiveFixture() *liveFixture {
	return &liveFixture{revision: map[string]int{}, changed: make(chan struct{}), dropped: make(chan struct{}), now: time.Now}
}

func registerLive(mux *http.ServeMux) {
	f := newLiveFixture()
	mux.HandleFunc("/live", f.servePage)
	mux.HandleFunc(liveStreamPath, f.serveEvents)
	mux.HandleFunc("/live/publish", f.servePublish)
	mux.HandleFunc("/live/drop", f.serveDrop)
	mux.HandleFunc("/live/outage", f.serveOutage)
	mux.HandleFunc("/page-heading", servePageHeading)
}

func liveKnownSource(key string) bool {
	for _, s := range liveSources {
		if s.Key == key {
			return true
		}
	}
	return false
}

// publish appends one event; "*" marks every source, as a gap would.
func (f *liveFixture) publish(key string) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	sources := []string{key}
	for _, s := range liveSources {
		if key == "*" || key == s.Key {
			f.revision[s.Key]++
		}
	}
	f.events = append(f.events, liveEvent{ID: f.next, Sources: sources})
	if len(f.events) > liveReplayWindow {
		f.events = append([]liveEvent(nil), f.events[len(f.events)-liveReplayWindow:]...)
	}
	close(f.changed)
	f.changed = make(chan struct{})
	return f.next
}

// Caller holds f.mu. A cursor outside the retained window cannot be replayed
// honestly, so the stream sends one wildcard event instead of guessing.
func (f *liveFixture) sinceLocked(cursor uint64) []liveEvent {
	if cursor > f.next || (len(f.events) > 0 && cursor+1 < f.events[0].ID) {
		return []liveEvent{{ID: f.next, Sources: []string{"*"}}}
	}
	var out []liveEvent
	for _, e := range f.events {
		if e.ID > cursor {
			out = append(out, e)
		}
	}
	return out
}

func liveFrame(e liveEvent) string {
	data, _ := json.Marshal(struct {
		Sources []string `json:"sources"`
	}{e.Sources})
	return fmt.Sprintf("id: %d\nevent: %s\ndata: %s\n\n", e.ID, liveEventName, data)
}

func parseLastEventID(values []string) (uint64, bool, bool) {
	if len(values) == 0 {
		return 0, false, true
	}
	if len(values) > 1 || len(values[0]) == 0 || len(values[0]) > 20 {
		return 0, false, false
	}
	for _, c := range values[0] {
		if c < '0' || c > '9' {
			return 0, false, false
		}
	}
	n, err := strconv.ParseUint(values[0], 10, 64)
	return n, true, err == nil
}

func (f *liveFixture) serveEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !referenceRequestAllowed(r) {
		http.Error(w, "The synthetic stream requires same-origin loopback requests.", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return
	}
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		http.Error(w, "Request text/event-stream to open this stream.", http.StatusNotAcceptable)
		return
	}
	last, resume, ok := parseLastEventID(r.Header.Values("Last-Event-ID"))
	if !ok || r.URL.RawQuery != "" {
		http.Error(w, "The stream accepts one decimal Last-Event-ID and no query.", http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	if f.failNext > 0 {
		f.failNext--
		f.mu.Unlock()
		http.Error(w, "Synthetic outage: this connection was refused on purpose.", http.StatusServiceUnavailable)
		return
	}
	if f.streams >= liveMaxStreams {
		f.mu.Unlock()
		w.Header().Set("Retry-After", "1")
		http.Error(w, "The bounded synthetic stream capacity is reached.", http.StatusTooManyRequests)
		return
	}
	f.streams++
	var backlog []liveEvent
	cursor := f.next
	if resume {
		backlog = f.sinceLocked(last)
	}
	changed, dropped := f.changed, f.dropped
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.streams--
		f.mu.Unlock()
	}()

	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(liveWriteTimeout)); err != nil {
		http.Error(w, "This host cannot bound stream writes.", http.StatusInternalServerError)
		return
	}
	defer controller.SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	write := func(s string) bool {
		if err := controller.SetWriteDeadline(time.Now().Add(liveWriteTimeout)); err != nil {
			return false
		}
		if _, err := io.WriteString(w, s); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	var first strings.Builder
	first.WriteString("retry: 1000\n: connected\n\n")
	for _, e := range backlog {
		first.WriteString(liveFrame(e))
	}
	if !write(first.String()) {
		return
	}
	heartbeat := time.NewTicker(liveHeartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-dropped:
			return
		case <-heartbeat.C:
			if !write(": keepalive\n\n") {
				return
			}
		case <-changed:
			f.mu.Lock()
			fresh := f.sinceLocked(cursor)
			cursor, changed = f.next, f.changed
			f.mu.Unlock()
			var b strings.Builder
			for _, e := range fresh {
				b.WriteString(liveFrame(e))
			}
			if b.Len() > 0 && !write(b.String()) {
				return
			}
		}
	}
}

// liveMutation guards the fixture's POST controls: loopback same-origin only,
// a small form body, and 303 back to the page for script-free use.
func liveMutation(w http.ResponseWriter, r *http.Request) (url.Values, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if !referenceRequestAllowed(r) {
		http.Error(w, "Fixture controls accept same-origin loopback requests only.", http.StatusForbidden)
		return nil, false
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := r.ParseForm(); err != nil || r.URL.RawQuery != "" {
		http.Error(w, "Send one small form body and no query.", http.StatusBadRequest)
		return nil, false
	}
	return r.PostForm, true
}

func liveDone(w http.ResponseWriter, r *http.Request, message string) {
	if r.Header.Get("X-Live-Fixture") == "fetch" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, message)
		return
	}
	http.Redirect(w, r, "/live", http.StatusSeeOther)
}

func (f *liveFixture) servePublish(w http.ResponseWriter, r *http.Request) {
	values, ok := liveMutation(w, r)
	if !ok {
		return
	}
	key := values.Get("source")
	if len(values["source"]) != 1 || (key != "*" && !liveKnownSource(key)) {
		http.Error(w, "Choose one fixture source or *.", http.StatusBadRequest)
		return
	}
	id := f.publish(key)
	liveDone(w, r, fmt.Sprintf("Published event %d for %s.", id, key))
}

func (f *liveFixture) serveDrop(w http.ResponseWriter, r *http.Request) {
	if _, ok := liveMutation(w, r); !ok {
		return
	}
	f.mu.Lock()
	close(f.dropped)
	f.dropped = make(chan struct{})
	f.mu.Unlock()
	liveDone(w, r, "Dropped every open stream; clients reconnect with Last-Event-ID.")
}

func (f *liveFixture) serveOutage(w http.ResponseWriter, r *http.Request) {
	values, ok := liveMutation(w, r)
	if !ok {
		return
	}
	n, err := strconv.Atoi(values.Get("count"))
	if len(values["count"]) != 1 || err != nil || n < 0 || n > liveMaxOutage {
		http.Error(w, fmt.Sprintf("Count must be 0 to %d.", liveMaxOutage), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.failNext = n
	f.mu.Unlock()
	liveDone(w, r, fmt.Sprintf("The next %d stream connections will be refused with 503.", n))
}

func (f *liveFixture) servePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/live" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
		return
	}
	content, err := f.build()
	if err != nil {
		http.Error(w, "The live-freshness specimen could not be rendered.", http.StatusInternalServerError)
		return
	}
	writeSpecimenPage(w, r, "live", "Live freshness", content, "")
}

type liveRegionView struct {
	Region template.HTML
}

func (f *liveFixture) build() (template.HTML, error) {
	f.mu.Lock()
	rendered := f.now().UTC().Format("15:04:05")
	last := f.next
	revisions := map[string]int{}
	for k, v := range f.revision {
		revisions[k] = v
	}
	f.mu.Unlock()

	heading, err := chrome.RenderPageHeading(chrome.PageHeading{
		Breadcrumbs: []chrome.Crumb{{Label: "Overview", Href: "/overview"}, {Label: "Live freshness"}},
		Eyebrow:     "Specimen · L2 script + L3b helpers",
		Title:       "Live freshness",
		Meta:        []string{"Synthetic SSE fixture", fmt.Sprintf("Latest event %d", last), "Rendered " + rendered + " UTC"},
		Actions: []form.Button{
			{Label: "Catalog entry", Href: "/catalog/interaction.live-freshness", Variant: form.ButtonGhost},
			{Label: "Page heading specimen", Href: "/page-heading", Variant: form.ButtonGhost},
		},
	})
	if err != nil {
		return "", err
	}
	status, err := live.RenderPageStatus(live.Page{URL: liveStreamPath, Events: []string{liveEventName}, LastEventID: strconv.FormatUint(last, 10)})
	if err != nil {
		return "", err
	}
	regions := make([]liveRegionView, 0, len(liveSources))
	for i, s := range liveSources {
		n := revisions[s.Key]
		body := fmt.Sprintf(`<h2 class="text-lg font-medium text-ink">%s</h2><p class="mt-1 text-sm text-ink-muted">Synthetic source <code class="font-mono">%s</code></p><p class="mt-3 text-3xl font-semibold text-ink">%d</p><p class="text-sm text-ink-muted">published changes rendered on this page</p>`,
			template.HTMLEscapeString(s.Title), template.HTMLEscapeString(s.Key), n)
		region, err := live.RenderRegion(live.Region{
			ID:      "live-" + strconv.Itoa(i+1),
			Sources: []string{s.Key},
			Label:   s.Label,
			AsOf:    fmt.Sprintf("As of %s UTC · revision %d", rendered, n),
			Body:    template.HTML(body),
		})
		if err != nil {
			return "", err
		}
		regions = append(regions, liveRegionView{Region: template.HTML(region)})
	}
	return htmlf("live", liveHTML, struct {
		Heading, Status template.HTML
		Regions         []liveRegionView
		Sources         []liveSource
		MaxOutage       int
		Script          template.JS
	}{template.HTML(heading), template.HTML(status), regions, liveSources, liveMaxOutage, template.JS(live.Script() + liveFixtureScript)})
}

// Gallery-only enhancement: submit fixture controls without reloading, so the
// published event reaches the still-open page through the stream.
const liveFixtureScript = `
;(function () {
  var status = document.querySelector('[data-live-fixture-status]');
  document.addEventListener('submit', function (e) {
    var form = e.target.closest('form[data-live-fixture]');
    if (!form) return;
    e.preventDefault();
    fetch(form.action, { method: 'POST', body: new URLSearchParams(new FormData(form)), headers: { 'X-Live-Fixture': 'fetch' } })
      .then(function (r) { return r.text().then(function (t) { status.textContent = r.ok ? t : 'Fixture refused: ' + t; }); })
      .catch(function () { status.textContent = 'Fixture request failed; the page is unchanged.'; });
  });
})();`

const liveHTML = `
{{.Heading}}
<p class="mt-4 max-w-2xl text-ink-muted">Each panel follows a synthetic source. A published event never changes a panel's content: its status line gains <strong class="text-ink">Updated — refresh this panel</strong> in reserved space and one polite announcement is made. Refreshing reloads this page at that panel. Without JavaScript the page states its freshness and asks you to reload.</p>
<div class="mt-6 rounded-lg border border-line bg-surface-raised px-4 py-2">{{.Status}}</div>
<div class="mt-6 grid gap-4 md:grid-cols-3">
{{range .Regions}}<div class="min-w-0 rounded-lg border border-line bg-surface-raised p-4">{{.Region}}</div>
{{end}}</div>
<section aria-labelledby="live-fixture-title" class="mt-10">
<h2 id="live-fixture-title" class="text-lg font-medium text-ink">Drive the fixture</h2>
<p class="mt-1 max-w-2xl text-sm text-ink-muted">These loopback-only controls publish, drop or refuse synthetic stream events. With JavaScript they post in the background; without it they post and return here.</p>
<div class="mt-4 flex flex-wrap gap-2">
{{range .Sources}}<form method="post" action="/live/publish" data-live-fixture><input type="hidden" name="source" value="{{.Key}}"><button type="submit" class="min-h-11 rounded-md border border-line bg-surface px-3 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Publish {{.Title}} change</button></form>
{{end}}<form method="post" action="/live/publish" data-live-fixture><input type="hidden" name="source" value="*"><button type="submit" class="min-h-11 rounded-md border border-line bg-surface px-3 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Publish a gap (all panels)</button></form>
<form method="post" action="/live/drop" data-live-fixture><button type="submit" class="min-h-11 rounded-md border border-line bg-surface px-3 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Drop open streams</button></form>
<form method="post" action="/live/outage" data-live-fixture><input type="hidden" name="count" value="3"><button type="submit" class="min-h-11 rounded-md border border-line bg-surface px-3 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Refuse next 3 connections</button></form>
<form method="post" action="/live/outage" data-live-fixture><input type="hidden" name="count" value="{{.MaxOutage}}"><button type="submit" class="min-h-11 rounded-md border border-line bg-surface px-3 py-2 text-sm font-medium text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Refuse next {{.MaxOutage}} connections</button></form>
</div>
<p data-live-fixture-status role="status" class="mt-3 min-h-6 text-sm text-ink-muted"></p>
</section>
<script>{{.Script}}</script>`

// writeSpecimenPage renders a Gallery page that is not in the top navigation,
// keeping theme links on the same path and query.
func writeSpecimenPage(w http.ResponseWriter, r *http.Request, key, title string, content template.HTML, query string) {
	theme := sanitizeTheme(r.URL.Query().Get("theme"))
	links := make([]navLink, len(nav))
	copy(links, nav)
	for i := range links {
		if theme != "" {
			links[i].Href += "?theme=" + theme
		}
	}
	with := func(value string) string {
		values, _ := url.ParseQuery(query)
		if value != "" {
			values.Set("theme", value)
		}
		if len(values) == 0 {
			return r.URL.Path
		}
		return r.URL.Path + "?" + values.Encode()
	}
	body, err := render(pageData{Title: title, Nav: links, Theme: theme, Page: key, Content: content,
		ThemeLight: with("light"), ThemeDark: with("dark"), ThemeSystem: with("")})
	if err != nil {
		http.Error(w, "The specimen could not be rendered.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body)
}
