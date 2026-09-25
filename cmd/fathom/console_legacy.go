package main

// console_legacy.go — OC-7 (docs/design/observatory-console.md §1): the old
// /observatory* routes keep answering. An HTML request is redirected (308,
// method-preserving) to the page's new home; a JSON request (?format=json or
// Accept: application/json) is answered by the old handler unchanged, because
// observatoryModel v1 and its per-page JSON are part of the compatibility
// surface (docs/intent/scope.md). The wiring mutation endpoints
// (/observatory/wiring/{preview,apply,undo}) are not redirected at all: they
// stay exactly where the local-mutation gate guards them.

import (
	"net/http"
	"net/url"
	"strings"
)

// legacyObservatoryRedirect wraps an old route. target maps the request to
// its new home ("" means "not a page render — let the old handler answer").
func legacyObservatoryRedirect(target func(r *http.Request) string, legacy http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && !wantsJSON(r) {
			if to := target(r); to != "" {
				http.Redirect(w, r, to, http.StatusPermanentRedirect)
				return
			}
		}
		legacy.ServeHTTP(w, r)
	})
}

// legacyQuery re-attaches the request's query minus the keys a new route
// carries in its path (id, and nothing else) — ?format=md survives the move.
func legacyQuery(r *http.Request, drop ...string) string {
	q := r.URL.Query()
	for _, k := range drop {
		q.Del(k)
	}
	if enc := q.Encode(); enc != "" {
		return "?" + enc
	}
	return ""
}

// mountLegacyObservatory registers every old route on mux.
func mountLegacyObservatory(mux *http.ServeMux, selfURL string, wiring http.Handler) {
	fixed := func(path string) func(*http.Request) string {
		return func(r *http.Request) string { return path + legacyQuery(r) }
	}
	mux.Handle("/observatory", legacyObservatoryRedirect(fixed("/fleet"), observatoryHandler(selfURL)))
	mux.Handle("/observatory/service", legacyObservatoryRedirect(func(r *http.Request) string {
		id := r.URL.Query().Get("id")
		if id == "" {
			return "/fleet"
		}
		return "/projects/" + url.PathEscape(id) + legacyQuery(r, "id")
	}, observatoryServicePage(selfURL)))
	mux.Handle("/observatory/impact", legacyObservatoryRedirect(fixed("/impact"), observatoryImpactPage(selfURL)))
	mux.Handle("/observatory/story", legacyObservatoryRedirect(fixed("/sessions"), observatoryStoryPage(selfURL)))
	mux.Handle("/observatory/session", legacyObservatoryRedirect(func(r *http.Request) string {
		id := r.URL.Query().Get("id")
		if id == "" {
			return "/sessions" + legacyQuery(r, "id")
		}
		return "/sessions/" + url.PathEscape(id) + legacyQuery(r, "id")
	}, observatorySessionPage(selfURL)))
	mux.Handle("/observatory/brief", legacyObservatoryRedirect(fixed("/brief"), observatoryBriefPage(selfURL)))
	// The wiring subtree: only the index page moves; the POST endpoints stay.
	mux.Handle("/observatory/wiring/", legacyObservatoryRedirect(func(r *http.Request) string {
		if strings.TrimSuffix(r.URL.Path, "/") == "/observatory/wiring" {
			return "/wiring" + legacyQuery(r)
		}
		return ""
	}, wiring))
}
