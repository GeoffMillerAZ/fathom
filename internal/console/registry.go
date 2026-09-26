package console

// registry.go — OC-H2: a row that names a service the registry does not
// list is not the fleet's. 205 `fathom review` rows for the test fixture
// service "svc" reached the operator's call log; the catches timeline
// counted their 55 conservation findings as "conserve losses" this week.
// Deleting them is the owner's call; counting them is not honest. So every
// projection that folds rows by service checks each row's service against
// the daemon's registry: a row naming an unlisted service is left out of
// every figure and named instead — how many rows, and which service ids.
// A row naming no service is unattributed, a different thing, and is
// never excluded by this rule.

import (
	"fmt"
	"sort"
	"strings"
)

// Registry is the daemon's service registry as a projection over rows sees
// it. Read is false when the registry could not be read: no row can then be
// checked against it, so none is excluded, and Note says why.
type Registry struct {
	Read  bool
	Note  string
	names map[string]bool
}

// RegistryOf is a registry that was read and lists names (case-insensitive).
func RegistryOf(names []string) Registry {
	r := Registry{Read: true, names: make(map[string]bool, len(names))}
	for _, n := range names {
		r.names[strings.ToLower(strings.TrimSpace(n))] = true
	}
	return r
}

// RegistryUnread is a registry that could not be read, and why.
func RegistryUnread(note string) Registry { return Registry{Note: note} }

// Lists reports whether the registry was read and lists service.
func (r Registry) Lists(service string) bool {
	return r.Read && r.names[strings.ToLower(strings.TrimSpace(service))]
}

// Excludes reports whether a row naming service is left out: the registry
// was read, the row names a service, and the registry does not list it.
func (r Registry) Excludes(service string) bool {
	return r.Read && strings.TrimSpace(service) != "" && !r.Lists(service)
}

// Unregistered is what a projection left out under the registry rule.
type Unregistered struct {
	// Rows is how many rows named a service the registry does not list.
	Rows int `json:"rows"`
	// Services are the ids those rows name, lower-cased and sorted.
	Services []string `json:"services,omitempty"`
	// RegistryNote is set when the registry could not be read: no row was
	// checked against it, so none was excluded.
	RegistryNote string `json:"registry_note,omitempty"`
}

// Add counts one excluded row naming service.
func (u *Unregistered) Add(service string) {
	u.Rows++
	u.addService(service)
}

// Merge adds o's rows and services to u.
func (u *Unregistered) Merge(o Unregistered) {
	u.Rows += o.Rows
	for _, s := range o.Services {
		u.addService(s)
	}
	if u.RegistryNote == "" {
		u.RegistryNote = o.RegistryNote
	}
}

// addService keeps Services sorted and distinct.
func (u *Unregistered) addService(service string) {
	s := strings.ToLower(strings.TrimSpace(service))
	i := sort.SearchStrings(u.Services, s)
	if i < len(u.Services) && u.Services[i] == s {
		return
	}
	u.Services = append(u.Services, "")
	copy(u.Services[i+1:], u.Services[i:])
	u.Services[i] = s
}

// UnregisteredOf folds one service per row under r: the rows r excludes,
// or, when r could not be read, the note saying none was checked.
func UnregisteredOf(services []string, r Registry) Unregistered {
	u := Unregistered{}
	if !r.Read {
		u.RegistryNote = r.Note
		return u
	}
	for _, s := range services {
		if r.Excludes(s) {
			u.Add(s)
		}
	}
	return u
}

// Phrase names the exclusion for a denominator or a reason: "3 review rows
// naming a service the registry does not list (svc) excluded". It is ""
// when nothing was excluded.
func (u Unregistered) Phrase(noun string) string {
	if u.Rows == 0 {
		return ""
	}
	what := "a service"
	if len(u.Services) > 1 {
		what = fmt.Sprintf("%d services", len(u.Services))
	}
	return fmt.Sprintf("%s naming %s the registry does not list (%s) excluded", thousandsNoun(u.Rows, noun), what, shortList(u.Services, 3))
}
