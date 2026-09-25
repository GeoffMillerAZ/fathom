package gallery

import (
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/geoffmilleraz/app-kit/internal/qualityplan"
)

type qualityRequest struct {
	Scope                   string
	CapabilityID            string
	ExpectedRevision        string
	ExpectedCatalogRevision string
	Theme                   string
}

type qualityListView struct {
	Result      qualityplan.ListResult
	Request     qualityRequest
	DetailQuery string
	Reset       string
}

type qualityDetailView struct {
	Result  qualityplan.DetailResult
	Request qualityRequest
	Back    string
}

// registerQuality projects the embedded, CUE-authored work queue without reading
// source references or executing gates. It intentionally shares one immutable
// snapshot between Gallery's HTML and JSON routes.
func registerQuality(mux *http.ServeMux) {
	plan, loadErr := qualityplan.Default()
	ready := func(w http.ResponseWriter) bool {
		if loadErr != nil {
			http.Error(w, "quality work unavailable: "+loadErr.Error(), http.StatusInternalServerError)
			return false
		}
		return true
	}
	list := func(w http.ResponseWriter, r *http.Request, api bool) {
		if !ready(w) {
			return
		}
		in, err := parseQualityURL(r, false)
		if err != nil {
			writeQualityProblem(w, r, api, http.StatusBadRequest, err, plan)
			return
		}
		result, err := plan.ListForCatalog(in.CapabilityID, in.Scope, in.ExpectedCatalogRevision)
		if err != nil {
			writeQualityProblem(w, r, api, qualityProblemStatus(err), err, plan)
			return
		}
		if api {
			writeCatalogJSON(w, r, result, "")
			return
		}
		in.ExpectedCatalogRevision = result.CatalogRevision
		content, err := htmlf("quality-list", qualityListHTML, qualityListView{
			Result: result, Request: in, DetailQuery: "?" + qualityValues(in, true, result.Revision).Encode(), Reset: qualityResetLink(in.Theme, result.CatalogRevision),
		})
		writeQualityStatus(w, r, "Quality work", content, err, http.StatusOK)
	}
	detail := func(w http.ResponseWriter, r *http.Request, api bool) {
		if !ready(w) {
			return
		}
		in, err := parseQualityURL(r, true)
		if err != nil {
			writeQualityProblem(w, r, api, http.StatusBadRequest, err, plan)
			return
		}
		// Detail links retain list context. Validate a supplied capability even
		// though it does not change the exact work-item representation.
		if _, err := plan.ListForCatalog(in.CapabilityID, in.Scope, in.ExpectedCatalogRevision); err != nil {
			writeQualityProblem(w, r, api, qualityProblemStatus(err), err, plan)
			return
		}
		result, err := plan.GetForCatalog(r.PathValue("id"), in.ExpectedRevision, in.ExpectedCatalogRevision)
		if err != nil {
			status := qualityProblemStatus(err)
			if strings.Contains(err.Error(), "unknown item") {
				status = http.StatusNotFound
			}
			writeQualityProblem(w, r, api, status, err, plan)
			return
		}
		if api {
			writeCatalogJSON(w, r, result, "")
			return
		}
		in.ExpectedCatalogRevision = result.CatalogRevision
		content, err := htmlf("quality-detail", qualityDetailHTML, qualityDetailView{Result: result, Request: in, Back: qualityLink("/quality", qualityValues(in, false))})
		writeQualityStatus(w, r, result.Item.ID, content, err, http.StatusOK)
	}
	mux.HandleFunc("GET /quality", func(w http.ResponseWriter, r *http.Request) { list(w, r, false) })
	mux.HandleFunc("GET /quality/{id}", func(w http.ResponseWriter, r *http.Request) { detail(w, r, false) })
	mux.HandleFunc("GET /api/quality", func(w http.ResponseWriter, r *http.Request) { list(w, r, true) })
	mux.HandleFunc("GET /api/quality/{id}", func(w http.ResponseWriter, r *http.Request) { detail(w, r, true) })
	// Keep method errors read-only and non-cacheable too. GET patterns also
	// accept HEAD and take precedence over these exact-path fallbacks.
	for _, path := range []string{"/quality", "/quality/{id}", "/api/quality", "/api/quality/{id}"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		})
	}
}

func parseQualityURL(r *http.Request, detail bool) (qualityRequest, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return qualityRequest{}, errors.New("invalid quality query encoding; review the current capability catalog")
	}
	return parseQualityRequest(values, detail)
}

func qualityProblemStatus(err error) int {
	if strings.HasPrefix(err.Error(), "qualityplan: catalog revision mismatch:") {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

func parseQualityRequest(values url.Values, detail bool) (qualityRequest, error) {
	in := qualityRequest{Scope: "core"}
	allowed := map[string]bool{"scope": true, "capability_id": true, "theme": true, "expected_catalog_revision": true}
	if detail {
		allowed["expected_revision"] = true
	}
	for key, values := range values {
		if !allowed[key] || len(values) != 1 || !utf8.ValidString(values[0]) || strings.ContainsRune(values[0], 0) {
			return in, errors.New("invalid quality query")
		}
	}
	in.Scope, in.CapabilityID, in.ExpectedRevision, in.Theme = values.Get("scope"), values.Get("capability_id"), values.Get("expected_revision"), values.Get("theme")
	in.ExpectedCatalogRevision = values.Get("expected_catalog_revision")
	if in.Scope == "" {
		in.Scope = "core"
	}
	if in.Theme != "" && sanitizeTheme(in.Theme) == "" {
		return in, errors.New("invalid theme")
	}
	if in.Scope != "core" && in.Scope != "on-demand" && in.Scope != "all" {
		return in, errors.New("invalid scope")
	}
	return in, nil
}

func qualityValues(in qualityRequest, revision bool, revisions ...string) url.Values {
	v := url.Values{"scope": {in.Scope}}
	if in.CapabilityID != "" {
		v.Set("capability_id", in.CapabilityID)
	}
	if in.Theme != "" {
		v.Set("theme", in.Theme)
	}
	if in.ExpectedCatalogRevision != "" {
		v.Set("expected_catalog_revision", in.ExpectedCatalogRevision)
	}
	if revision && len(revisions) == 1 && revisions[0] != "" {
		v.Set("expected_revision", revisions[0])
	}
	return v
}

func qualityLink(path string, values url.Values) string {
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}

func qualityResetLink(theme, catalogRevision string) string {
	values := url.Values{}
	if theme != "" {
		values.Set("theme", theme)
	}
	if catalogRevision != "" {
		values.Set("expected_catalog_revision", catalogRevision)
	}
	return qualityLink("/quality", values)
}

func qualityCapabilityLink(capabilityID, theme, catalogRevision string) string {
	values := url.Values{"capability_id": {capabilityID}, "expected_catalog_revision": {catalogRevision}}
	if theme != "" {
		values.Set("theme", theme)
	}
	return qualityLink("/quality", values)
}

func writeQualityProblem(w http.ResponseWriter, r *http.Request, api bool, status int, problem error, plan *qualityplan.Snapshot) {
	w.Header().Set("Cache-Control", "no-store")
	if api {
		http.Error(w, problem.Error(), status)
		return
	}
	query, queryErr := url.ParseQuery(r.URL.RawQuery)
	theme := sanitizeTheme(query.Get("theme"))
	expected := query.Get("expected_catalog_revision")
	_, guardErr := plan.ListForCatalog("", "core", expected)
	reset, label := qualityResetLink(theme, expected), "Clear filters and return to quality work"
	review := queryErr != nil || len(query["expected_catalog_revision"]) > 1 || guardErr != nil
	if review {
		values := url.Values{}
		if theme != "" {
			values.Set("theme", theme)
		}
		if id := query.Get("capability_id"); len(id) > 0 && len(id) <= 96 && id[0] >= 'a' && id[0] <= 'z' && strings.Trim(id, "abcdefghijklmnopqrstuvwxyz0123456789._-") == "" {
			values.Set("q", id)
		}
		reset, label = qualityLink("/catalog", values), "Review current capability catalog"
	}
	content, err := htmlf("quality-problem", `<h1 class="text-2xl font-semibold">This quality-work request needs a reset</h1><p class="mt-3 max-w-3xl break-words text-ink-muted">{{.Problem}}</p>{{if .Review}}<p class="mt-3 max-w-3xl text-ink-muted">Your application and adoption pins have not changed. Review the current capability contract before continuing to quality work.</p>{{end}}<a href="{{.Reset}}" class="mt-5 inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">{{.Label}}</a>`, struct {
		Problem, Reset, Label string
		Review                bool
	}{problem.Error(), reset, label, review})
	writeQualityStatus(w, r, "Reset quality-work request", content, err, status)
}

func writeQualityStatus(w http.ResponseWriter, r *http.Request, title string, content template.HTML, err error, status int) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	theme := sanitizeTheme(r.URL.Query().Get("theme"))
	links := append([]navLink(nil), nav...)
	for i := range links {
		links[i].Active = links[i].Key == "quality"
		if theme != "" {
			links[i].Href += "?theme=" + theme
		}
	}
	withTheme := func(value string) string {
		q := r.URL.Query()
		q.Del("theme")
		if value != "" {
			q.Set("theme", value)
		}
		return r.URL.Path + "?" + q.Encode()
	}
	body, err := render(pageData{Title: title, Nav: links, Theme: theme, Page: "quality", Content: content, ThemeLight: withTheme("light"), ThemeDark: withTheme("dark"), ThemeSystem: withTheme("")})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

const qualityListHTML = `
<div class="max-w-3xl"><p class="text-sm font-medium text-accent">AUTHORED WORK QUEUE</p><h1 class="mt-2 text-3xl font-semibold">Quality work</h1><p class="mt-3 text-ink-muted">This is authored planning, not certification or a live evidence check. It records bounded starter work and explicitly selected on-demand work.</p></div>
<form action="/quality" method="get" class="mt-8 grid gap-4 border-y border-line py-5 sm:grid-cols-2" aria-label="Quality work filters">
{{if .Request.Theme}}<input type="hidden" name="theme" value="{{.Request.Theme}}">{{end}}
<input type="hidden" name="expected_catalog_revision" value="{{.Result.CatalogRevision}}">
<div><label for="quality-scope" class="block text-sm font-medium">Work scope</label><select id="quality-scope" name="scope" class="mt-1 min-h-11 w-full rounded-md border border-line bg-surface p-2 text-ink focus-visible:outline-2 focus-visible:outline-focus"><option value="core"{{if eq .Request.Scope "core"}} selected{{end}}>Core starter work</option><option value="on-demand"{{if eq .Request.Scope "on-demand"}} selected{{end}}>On-demand work</option><option value="all"{{if eq .Request.Scope "all"}} selected{{end}}>All recorded work</option></select></div>
<div><label for="quality-capability" class="block text-sm font-medium">Exact capability ID (optional)</label><input id="quality-capability" name="capability_id" value="{{.Request.CapabilityID}}" class="mt-1 min-h-11 w-full rounded-md border border-line bg-surface p-2 text-ink focus-visible:outline-2 focus-visible:outline-focus"></div>
<div class="flex flex-wrap items-center gap-4 sm:col-span-2"><button class="inline-flex min-h-11 items-center rounded-md bg-accent px-4 py-2 font-medium text-accent-ink focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Filter work</button><a href="{{.Reset}}" class="inline-flex min-h-11 items-center text-ink-muted underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Reset filters</a></div>
</form>
<section class="mt-8" aria-labelledby="quality-results"><div class="border-b border-line pb-3"><h2 id="quality-results" class="text-lg font-semibold">{{len .Result.Items}} recorded work items</h2><p class="mt-1 text-sm text-ink-muted">{{.Result.ClaimBoundary}}. No score or readiness count is implied.</p></div>
{{range .Result.Items}}<article class="grid min-w-0 gap-3 border-b border-line py-5 sm:grid-cols-3"><div class="min-w-0 sm:col-span-2"><h3 class="font-semibold"><a href="/quality/{{.ID}}{{$.DetailQuery}}" class="inline-flex min-h-11 max-w-full min-w-0 items-center whitespace-normal break-all text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">{{.ID}}</a></h3><p class="mt-1 break-words text-sm text-ink-muted">{{.Task}}</p><p class="mt-2 break-all font-mono text-xs text-ink-muted">{{range .CapabilityIDs}}{{.}} {{end}}</p></div><div class="min-w-0 text-sm"><p>State: {{.State}}</p><p class="mt-1 text-ink-muted">Profile: {{.Profile}}</p><p class="mt-1 text-ink-muted">{{if .Core}}Core{{else}}On-demand{{end}} · {{.Kind}}</p></div></article>{{else}}<div class="py-10"><h3 class="font-semibold">No recorded work matches this selection</h3><p class="mt-2 max-w-3xl text-ink-muted">That does not establish adoption, rejection or readiness. Reset filters or select a known capability with recorded work.</p><a href="{{.Reset}}" class="mt-4 inline-flex min-h-11 items-center text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Clear filters and view core work</a></div>{{end}}</section>
<p class="mt-8 break-all font-mono text-xs text-ink-muted">Inventory revision: {{.Result.Revision}} · Catalog revision: {{.Result.CatalogRevision}}</p>`

const qualityDetailHTML = `
<a href="{{.Back}}" class="inline-flex min-h-11 items-center text-sm text-accent underline focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">← Quality work</a>
<div class="mt-5 max-w-4xl"><p class="text-sm text-ink-muted">{{if .Result.Item.Core}}Core starter work{{else}}On-demand work{{end}} · {{.Result.Item.Profile}} · {{.Result.Item.State}}</p><h1 class="mt-2 break-words text-3xl font-semibold">{{.Result.Item.ID}}</h1><p class="mt-3 text-ink-muted">{{.Result.Item.Task}}</p><p class="mt-3 text-sm text-ink-muted">Authored planning, not certification. Listed references are plain records only, no source body is served and no gate is executed here.</p></div>
<section class="mt-8 border-t border-line pt-6" aria-labelledby="quality-standards"><h2 id="quality-standards" class="text-lg font-semibold">Seven enhancement standards</h2><div class="mt-4 space-y-6">{{range $key, $status := .Result.Item.Standards}}{{with index $.Result.Standards $key}}<article class="min-w-0 border-b border-line pb-5"><h3 class="font-medium">{{.Name}}: {{$status.Status}}</h3><p class="mt-2 text-sm text-ink-muted">{{.Requirement}}</p>{{if $status.Reason}}<p class="mt-2 break-words text-sm">Reason: {{$status.Reason}}</p>{{end}}{{if $status.NextStep}}<p class="mt-2 break-words text-sm text-ink-muted">Next step: {{$status.NextStep}}</p>{{end}}{{if or $status.Evidence $status.Review}}<details class="mt-3 min-w-0 text-sm text-ink-muted"><summary class="min-h-11 cursor-pointer py-3 font-medium text-accent">Evidence and review metadata</summary>{{range $status.Evidence}}<p class="mt-2 break-all font-mono text-xs">{{.Path}}: {{.Note}}</p>{{end}}{{with $status.Review}}<p class="mt-2 break-all font-mono text-xs">Review: {{.By}}: {{.Ref}}</p>{{end}}</details>{{end}}</article>{{end}}{{end}}</div></section>
<section class="mt-8 border-t border-line pt-6" aria-labelledby="quality-gates"><h2 id="quality-gates" class="text-lg font-semibold">Nine recorded gates</h2><div class="mt-4 space-y-5">{{range $name, $gate := .Result.Item.Gates}}<article class="min-w-0 border-b border-line pb-5"><h3 class="font-medium">{{$name}}: {{$gate.Status}}</h3>{{if $gate.Reason}}<p class="mt-2 break-words text-sm">Reason: {{$gate.Reason}}</p>{{end}}{{if $gate.NextStep}}<p class="mt-2 break-words text-sm text-ink-muted">Next step: {{$gate.NextStep}}</p>{{end}}{{if or $gate.Evidence $gate.Receipt $gate.Review $gate.SourceRevision $gate.ArtifactRevision}}<details class="mt-3 min-w-0 text-sm text-ink-muted"><summary class="min-h-11 cursor-pointer py-3 font-medium text-accent">Evidence and gate metadata</summary>{{range $gate.Evidence}}<p class="mt-2 break-all font-mono text-xs">{{.Path}}: {{.Note}}</p>{{end}}{{with $gate.Receipt}}<p class="mt-2 break-all font-mono text-xs">Receipt: {{.Path}}: {{.SHA256}}</p>{{end}}{{if $gate.SourceRevision}}<p class="mt-2 break-all font-mono text-xs">Source revision: {{$gate.SourceRevision}}</p>{{end}}{{if $gate.ArtifactRevision}}<p class="mt-2 break-all font-mono text-xs">Artifact revision: {{$gate.ArtifactRevision}}</p>{{end}}{{with $gate.Review}}<p class="mt-2 break-all font-mono text-xs">Review: {{.By}}: {{.Ref}}</p>{{end}}</details>{{end}}</article>{{end}}</div></section>
<section class="mt-8 border-t border-line pt-6" aria-labelledby="quality-references"><h2 id="quality-references" class="text-lg font-semibold">Authoring references</h2>{{if .Result.Item.SourceRefs}}<details class="mt-3 min-w-0 text-sm text-ink-muted"><summary class="min-h-11 cursor-pointer py-3 font-medium text-accent">Source references</summary>{{range .Result.Item.SourceRefs}}<p class="mt-2 break-all font-mono text-xs">{{.Path}}: {{.Note}}</p>{{end}}</details>{{end}}</section>
{{if .Result.Item.Selection}}<section class="mt-8 border-t border-line pt-6" aria-labelledby="quality-selection"><h2 id="quality-selection" class="text-lg font-semibold">Selected conversion boundary</h2><p class="mt-2 break-words text-sm text-ink-muted">{{.Result.Item.Selection.Workload}}</p>{{range .Result.Item.Selection.Budgets}}<p class="mt-2 text-sm">{{.Name}}: {{.Limit}}</p>{{end}}<details class="mt-3 min-w-0 text-sm text-ink-muted"><summary class="min-h-11 cursor-pointer py-3 font-medium text-accent">Selected IDs and acceptance contract</summary><p class="mt-2 break-all font-mono text-xs">Original: {{.Result.Item.Selection.Original.Kind}}: {{.Result.Item.Selection.Original.ID}}: {{.Result.Item.Selection.Original.RevisionSHA256}}</p><p class="mt-2 break-all font-mono text-xs">Target: {{.Result.Item.Selection.Target.ID}}: {{.Result.Item.Selection.Target.RevisionSHA256}}</p><p class="mt-2 break-all font-mono text-xs">Acceptance contract: {{.Result.Item.Selection.AcceptanceContract.Path}}: {{.Result.Item.Selection.AcceptanceContract.SHA256}}</p></details></section>{{end}}
<section class="mt-8 border-t border-line pt-6" aria-labelledby="quality-consumers"><h2 id="quality-consumers" class="text-lg font-semibold">Recorded consumers</h2>{{range .Result.Item.Consumers}}<article class="mt-3 min-w-0"><p class="font-medium">{{.App}}: {{.IntegrationStatus}}</p><p class="mt-1 break-all font-mono text-xs text-ink-muted">{{.Revision.Kind}}: {{.Revision.SHA256}}</p>{{range .Evidence}}<p class="mt-2 break-all font-mono text-xs text-ink-muted">{{.Path}}: {{.Note}}</p>{{end}}{{if or .Receipt .Review}}<details class="mt-3 min-w-0 text-sm text-ink-muted"><summary class="min-h-11 cursor-pointer py-3 font-medium text-accent">Receipt and review metadata</summary>{{with .Receipt}}<p class="mt-2 break-all font-mono text-xs">Receipt: {{.Path}}: {{.SHA256}}</p>{{end}}{{with .Review}}<p class="mt-2 break-all font-mono text-xs">Review: {{.By}}: {{.Ref}}</p>{{end}}</details>{{end}}</article>{{else}}<p class="mt-2 text-sm text-ink-muted">No recorded consumer assertion. This is not a claim that no consumer exists.</p>{{end}}</section>
<section class="mt-8 border-t border-line pt-6" aria-labelledby="quality-policy"><h2 id="quality-policy" class="text-lg font-semibold">On-demand policy</h2><p class="mt-2 text-sm text-ink-muted">{{.Result.BacklogPolicy.UnselectedClaim}}</p></section>
<p class="mt-8 break-all font-mono text-xs text-ink-muted">Inventory revision: {{.Result.Revision}} · Catalog revision: {{.Result.CatalogRevision}} · Standard: {{.Result.EnhancementStandardRevision}}</p>`
