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
		Verbs:    []string{"fathom fleet realization --service {svc} --json"},
		Live:     []string{"index:{svc}", "layers:{svc}", "sessions:{svc}", "plans:{svc}"},
		Build:    buildProjectPage,
	})
}

func buildProjectPage(cr *consoleRequest) (consoleOut, error) {
	id := cr.r.PathValue("svc")
	m := cr.model()
	body, ok := renderObsService(m, id)
	if !ok {
		return consoleOut{}, unknownProjectError(id, m, cr.deps.selfURL)
	}
	var row obsServiceRow
	for _, s := range m.Services {
		if s.Service == id {
			row = s
		}
	}
	meta := append([]string{row.Path}, modelMeta(m)...)
	out := consoleOut{
		Title: id,
		Meta:  meta,
		Model: row,
		Body:  func() string { return obsContent("service", body, m) },
		Verbs: []string{},
	}
	// OC-25 (RG-5): the realization ladder leads the overview.
	if l, ok := modelLadder(m, id); ok {
		verb := realizationVerb(id)
		out.Verbs, out.Sections = []string{verb}, map[string]any{verb: l}
		// The ladder is a live panel, so the router no longer wraps the
		// page in its region: the rest of the page keeps its own.
		rest, _ := consoleLivePage(id, pageLiveSources(cr.page, cr), cr.now, body)
		out.Body = func() string { return obsContent("service", ladderPanel(l)+rest, m) }
	}
	return out, nil
}

// unknownProjectError is OC-5's unknown-service page: the name that missed,
// the closest registered names, and the command that indexes a repository
// through THIS daemon (selfURL). The remedy is `fathom init`, doctor's own
// remedy for a repo missing from the index: it ingests through the daemon's
// ingest tool. A shell `fathom ingest` would write <repo>/workspace/db —
// a store this daemon never reads — so the project would stay unknown, and
// against the shared store it is the forbidden shell ingest (INC-3, SB-7).
func unknownProjectError(id string, m observatoryModel, selfURL string) *consoleError {
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
		Remedy:     "fathom init --path <repo> --service " + id + " --url " + selfURL,
		RemedyHref: "/fleet",
	}
}
