// Package router embeds the appkit view-router: a framework-agnostic,
// dependency-free History-API client router for static-client surfaces such
// as GitHub Pages SPAs. It requires no build step and no Node toolchain,
// consistent with the kit's Go + CUE only policy (ADR-0002).
//
// The router is a vendored JS artifact (`router.js`) embedded at compile time.
// Consumers serve it to the browser via [Handler] or inline the source via
// [Script]. It registers `window.AppKitRouter` with a single factory:
//
//	const router = AppKitRouter.create({
//	    routes: [
//	        { id: "home",   path: "/",       title: "Home" },
//	        { id: "resume", path: "/resume", title: "Résumé" },
//	    ],
//	    notFound: "home",
//	    onChange: (state) => render(state),
//	});
//	router.start();
//
// See [docs/design/view-router.md] for the full API reference and the static
// SPA on GitHub Pages recipe.
//
// # When to use this package
//
// Use this only for fully static client apps (no server router available —
// e.g. GitHub Pages). Kit apps that have a real HTTP server already get
// routing for free from real server URLs and the hypermedia shape (ADR-0003).
// Adding a client router on top of a Datastar app is an anti-pattern; see
// ADR-0016 for the decision boundary.
package router

import (
	_ "embed"
	"net/http"
)

//go:embed router.js
var routerJS string

// Script returns the router.js source as a string. Use this to inline the
// script into a page template (e.g. inside a <script> tag).
func Script() string {
	return routerJS
}

// Handler returns an [http.Handler] that serves router.js with an appropriate
// Content-Type and a long-lived cache header. Mount it at a stable path:
//
//	mux.Handle("/assets/router.js", router.Handler())
//
// The file is served from memory (embedded at build time); no disk reads occur
// at request time.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(routerJS))
	})
}
