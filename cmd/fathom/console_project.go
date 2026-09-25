package main

// console_project.go — the project workspace at /projects/{svc} (vision 35
// §3). F0 moves the service detail onto the shell with a trail back to the
// fleet; an unknown project renders on the shell with the remedy instead of
// a bare text/plain 404 (OC-5). Wave E adds the workspace tabs (OC-35..40).

import (
	"net/http"
	"sort"
	"strings"
)

func init() {
	declarePage(consolePage{
		ID: "project", Route: "/projects/{svc}", Group: "Fleet", Order: 10,
		Title: "Project", NavParent: "fleet", Aliases: []string{"service"},
		Question: "Everything about one repository: wiring, freshness, configuration and activity.",
		Build:    buildProjectPage,
	})
}

func buildProjectPage(cr *consoleRequest) (consoleOut, error) {
	id := cr.r.PathValue("svc")
	m := cr.model()
	body, ok := renderObsService(m, id)
	if !ok {
		return consoleOut{}, unknownProjectError(id, m)
	}
	var row obsServiceRow
	for _, s := range m.Services {
		if s.Service == id {
			row = s
		}
	}
	meta := append([]string{row.Path}, modelMeta(m)...)
	return consoleOut{
		Title: id,
		Meta:  meta,
		Model: row,
		Body:  func() string { return obsContent("service", body, m) },
	}, nil
}

// unknownProjectError is OC-5's unknown-service page: the name that missed,
// the closest registered names, and the command that indexes a repository.
func unknownProjectError(id string, m observatoryModel) *consoleError {
	if m.DegradedNote != "" {
		return &consoleError{
			Status: http.StatusServiceUnavailable, Title: "Store unreadable",
			Reason:      "The service registry could not be read, so project " + id + " cannot be looked up: " + m.DegradedNote,
			Remedy:      "fathom doctor",
			RemedyHref:  "/statusz",
			RemedyLabel: "Open the daemon status",
		}
	}
	var near []string
	for _, s := range m.Services {
		if strings.Contains(strings.ToLower(s.Service), strings.ToLower(id)) || strings.Contains(strings.ToLower(id), strings.ToLower(s.Service)) {
			near = append(near, s.Service)
		}
	}
	sort.Strings(near)
	reason := "No indexed project is named " + id + "."
	if len(near) > 0 {
		reason += " Did you mean: " + strings.Join(near, ", ") + "?"
	}
	return &consoleError{
		Status: http.StatusNotFound, Title: "Unknown project",
		Reason:     reason,
		Remedy:     "cd <repo> && fathom ingest --service " + id,
		RemedyHref: "/fleet",
	}
}
