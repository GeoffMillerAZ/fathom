package main

// ledgerroots_source.go — PE-23's source (docs/design/progress-and-economics.md
// §11, O121): the registered service roots every ledger reader asks come
// from the daemon's registry, the same list_services read every other
// registry consumer makes (inside serve-http it is the in-process registry,
// no HTTP). When the daemon does not answer, the roots are unread with the
// reason, and an unread registry excludes nothing (I38).

import (
	"net/http"

	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
)

// registeredLedgerRoots is the daemon's registered service roots, canonical.
func registeredLedgerRoots(hc *http.Client, baseURL string) ledgerroots.Roots {
	svcs, err := fetchServices(hc, baseURL)
	return ledgerRootsOf(svcs, err)
}

// ledgerRootsOf is the roots a fetchServices read gives the ledger readers:
// every service's recorded path, or unread with the reason when the read
// failed.
func ledgerRootsOf(svcs map[string]svcInfo, err error) ledgerroots.Roots {
	if err != nil {
		return ledgerroots.Unread("the daemon's service registry could not be read (" + err.Error() + ") — no ledger row was checked against the registered roots")
	}
	paths := make([]string, 0, len(svcs))
	for _, s := range svcs {
		if s.path != "" {
			paths = append(paths, s.path)
		}
	}
	return ledgerroots.RegisteredRoots(paths)
}
