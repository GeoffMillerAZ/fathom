package main

// console_wiring.go — the Wiring page at /wiring (vision 35 §3, System).
// F0 moves the per-repository Preview list onto the shell. The consent
// flow's mutations stay exactly where they were — POST
// /observatory/wiring/{preview,apply,undo} behind requireLocalMutation — so
// moving the page moved no gate. Wave E adds doctor, drift and receipts
// (OC-40).

import (
	"net/http"
)

func init() {
	declarePage(consolePage{
		ID: "wiring", Route: "/wiring", Href: "/wiring", Group: "System", Order: 10, Nav: true,
		Title:    "Wiring",
		Question: "What would fathom write into each repository, and can I undo it?",
		Live:     []string{"index"},
		Build:    buildWiringPage,
	})
}

// wiringModel is the wiring page's JSON projection (the same shape the
// legacy /observatory/wiring/?format=json answers).
type wiringModel struct {
	Services []registeredService `json:"services"`
}

func buildWiringPage(cr *consoleRequest) (consoleOut, error) {
	if cr.deps.c == nil {
		return consoleOut{}, &consoleError{Status: http.StatusServiceUnavailable, Title: "Store unreadable",
			Reason: "this daemon has no store open, so no repository can be listed for wiring", Remedy: "fathom serve-http --store <path>"}
	}
	all, err := clientWiringRegistry(cr.deps.c).list()
	if err != nil && wantsJSON(cr.r) {
		return consoleOut{}, &consoleError{Status: http.StatusServiceUnavailable, Title: "Store unreadable",
			Reason: "the registry could not be read: " + err.Error(), Remedy: "fathom doctor", RemedyHref: "/statusz", RemedyLabel: "Open the daemon status"}
	}
	m := cr.model()
	return consoleOut{
		Meta:  modelMeta(m),
		Model: wiringModel{Services: all},
		Body:  func() string { return obsContent("wiring", renderWiringIndex(all, err), m) },
	}, nil
}
