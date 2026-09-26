package main

// unregistered.go — OC-H2's wiring (internal/console/registry.go): the
// registry a verb or the observatory model read decides which call-log rows
// name a service it does not list; those rows are left out of every
// per-service figure and named.

import (
	"github.com/geoffmilleraz/fathom/internal/console"
)

// registryOf is the registry a fetchServices read gives the projections:
// unread, with the reason, when the read failed — then no row is checked.
func registryOf(svcs map[string]svcInfo, err error) console.Registry {
	if err != nil {
		return console.RegistryUnread("the daemon's service registry could not be read (" + err.Error() + ") — no row was checked against it")
	}
	names := make([]string, 0, len(svcs))
	for n := range svcs {
		names = append(names, n)
	}
	return console.RegistryOf(names)
}

// unregisteredCalls folds the call-log rows under r.
func unregisteredCalls(entries []adoptionCallEntry, r console.Registry) console.Unregistered {
	services := make([]string, 0, len(entries))
	for _, e := range entries {
		services = append(services, e.Service)
	}
	return console.UnregisteredOf(services, r)
}
