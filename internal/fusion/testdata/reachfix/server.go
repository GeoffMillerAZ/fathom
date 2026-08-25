// Package reachfix is the VS-10 (docs/plans/verify-spine.md) reachability
// fixture: a small, self-contained Go module exercising every reference
// shape TestImpact must resolve — handlers registered as VALUES (a bare
// selector arg, a keyed route-table literal, a type-conversion arg, a
// middleware arg, a method value), a call reachable only through a t.Run
// closure, and two types sharing a method name.
package reachfix

import "net/http"

// route pairs a path with a handler stored as a struct field — a keyed
// composite literal, so its "h" key must never be mistaken for a value
// reference to a function literally named "h".
type route struct {
	path string
	h    http.HandlerFunc
}

type server struct {
	mux *http.ServeMux
}

// newServer registers every handler by VALUE, never by direct call: a bare
// selector argument, a route-table literal, a type-conversion argument, a
// middleware-wrapped argument, and a method value.
func newServer() *server {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", handleA)

	routes := []route{{path: "/b", h: handleB}}
	for _, r := range routes {
		mux.Handle(r.path, http.HandlerFunc(r.h))
	}

	mux.Handle("/c", http.HandlerFunc(handleC))
	mux.HandleFunc("/d", wrap(handleD))

	s := &server{mux: mux}
	mux.HandleFunc("/e", s.handleE)

	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func handleA(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
func handleB(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
func handleC(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
func handleD(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

// handleE is a method value (s.handleE), never called directly.
func (s *server) handleE(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

// wrap is genuine middleware: a real call to wrap, and a value reference to
// h — h itself is called only through the local parameter inside the
// returned closure, which is the fixture's other middle-of-the-graph shape.
func wrap(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r)
	}
}
