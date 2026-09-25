package styleguide

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
)

//go:embed styleguide.css
var guideCSS string

type guideField struct {
	Path, Value, Kind, Edit, Error string
	Inherit                        bool
}
type guideFont struct{ ID, Stack string }
type guideView struct {
	Base, Title, SourceLabel, SourceSHA, KitRevision                 string
	Specimen, Scheme, Viewport, Version, Error, Reviewed, PreviewURL string
	Specimens                                                        []Specimen
	Fonts                                                            []guideFont
	Colors, Typography, Radii                                        []guideField
	Candidate                                                        Candidate
	ColorsOpen, TypographyOpen, RadiiOpen                            bool
	InvalidPath                                                      string
}

func (h *Handler) page(w http.ResponseWriter, r *http.Request, edits map[string]string, reviewed, specimen, scheme, viewport, version, message string, status int) {
	v := guideView{Base: h.base, Title: h.title, SourceLabel: h.engine.SourceLabel(), SourceSHA: h.engine.SourceSHA256(), KitRevision: h.engine.KitRevision(), Specimen: specimen, Scheme: scheme, Viewport: viewport, Version: version, Error: message, Reviewed: reviewed, Specimens: h.specimens}
	candidate, err := h.engine.Prepare(edits)
	var invalid *fieldError
	errors.As(err, &invalid)
	if err != nil {
		candidate, err = h.engine.Prepare(nil)
		if err != nil {
			http.Error(w, "Baseline could not be prepared", 500)
			return
		}
		v.Version = "baseline"
	}
	v.Candidate = candidate
	fonts := h.engine.Fonts()
	ids := make([]string, 0, len(fonts))
	for id := range fonts {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		v.Fonts = append(v.Fonts, guideFont{id, fonts[id]})
	}
	for _, f := range h.engine.Fields() {
		edit, present := edits[f.Path]
		field := guideField{Path: f.Path, Value: f.Value, Kind: f.Kind, Edit: edit, Inherit: present && edit == "" && f.Path == "fonts.display"}
		if invalid != nil && invalid.path == f.Path {
			field.Error, v.InvalidPath = invalid.cause.Error(), f.Path
		}
		switch f.Kind {
		case "color":
			v.Colors = append(v.Colors, field)
			v.ColorsOpen = v.ColorsOpen || present
		case "font":
			v.Typography = append(v.Typography, field)
			v.TypographyOpen = v.TypographyOpen || present
		case "radius":
			v.Radii = append(v.Radii, field)
			v.RadiiOpen = v.RadiiOpen || present
		}
	}
	q := url.Values{"specimen": {specimen}, "scheme": {scheme}}
	if v.Version == "candidate" && len(edits) > 0 {
		raw, _ := json.Marshal(edits)
		if len(raw) > 12<<10 {
			v.Version, v.Reviewed = "baseline", ""
			v.Error = "Draft exceeds the 12KiB preview budget. Your edits remain in the form; reduce the number of edits and preview again. Source baseline is shown."
			status = http.StatusUnprocessableEntity
		} else {
			q.Set("draft", base64.RawURLEncoding.EncodeToString(raw))
		}
	}
	v.PreviewURL = h.base + "/preview?" + q.Encode()
	body, err := renderGuide("guide", guideHTML, v)
	if err != nil {
		http.Error(w, "Style guide render failed", 500)
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; style-src 'self'; frame-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
	// Chrome uses the form document's referrer policy when generating POST Origin.
	// no-referrer turns native same-origin submissions into Origin:null. Keep the
	// strict origin guard; only form-bearing pages need this narrower disclosure.
	// Preview documents/resources retain no-referrer and the opaque sandbox.
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

const guideHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}} · theme review</title><link rel="stylesheet" href="{{.Base}}/style.css"></head><body><main class="sg">
<header class="sg-heading"><div><p class="sg-eyebrow">LOCAL DEVELOPMENT · EXPERIMENTAL</p><h1>{{.Title}}</h1><p>Try a theme change. Inspect your app. Export a candidate, never overwrite the source.</p></div><a href="{{.Base}}">Reset to source</a></header>
<nav class="sg-sections" aria-label="Theme review sections"><a href="#sg-editor">Edit tokens</a><a href="#sg-review">Review preview</a><a href="#sg-export">Export candidate</a></nav>
{{if .Error}}<section class="sg-error" role="alert" tabindex="-1" autofocus aria-labelledby="sg-error-title"><h2 id="sg-error-title">Candidate needs attention</h2><p>{{.Error}}</p><p>Your source is unchanged. Correct the fields and preview again.</p><p>{{if .InvalidPath}}<a href="#sg-field-{{.InvalidPath}}">Fix {{.InvalidPath}}</a>{{else}}<a href="#sg-editor">Return to token fields</a>{{end}}</p></section>{{end}}
<form action="{{.Base}}" method="post" class="sg-layout">
<input type="hidden" name="review_digest" value="{{.Reviewed}}">
<aside id="sg-editor" class="sg-editor" aria-label="Candidate token overrides" tabindex="-1"><p class="sg-note">Blank fields keep the source value. Edits live in this form, not saved preferences.</p>
<details {{if .ColorsOpen}}open{{end}}><summary>Color roles</summary><p class="sg-note">Use opaque hex colors, for example #f92aad. Light and dark branches are independent.</p>{{range .Colors}}<div class="sg-field"><label>{{.Path}}<input id="sg-field-{{.Path}}" name="{{.Path}}" value="{{.Edit}}" placeholder="{{.Value}}" maxlength="512" spellcheck="false" autocomplete="off" {{if .Error}}aria-invalid="true" aria-describedby="sg-field-{{.Path}}-error"{{end}}></label><small>Source: <code>{{.Value}}</code></small>{{if .Error}}<p class="sg-field-error" id="sg-field-{{.Path}}-error">{{.Error}}</p>{{end}}</div>{{end}}</details>
<details {{if .TypographyOpen}}open{{end}}><summary>Typography</summary><p class="sg-note">Approved system/fallback stacks only. No font assets are loaded by this preview.</p>{{range .Typography}}{{$field:=.}}<div class="sg-field"><label>{{.Path}}<select id="sg-field-{{.Path}}" name="{{.Path}}" {{if .Error}}aria-invalid="true" aria-describedby="sg-field-{{.Path}}-error"{{end}}><option value="">Keep source</option>{{if eq .Path "fonts.display"}}<option value="__inherit__" {{if .Inherit}}selected{{end}}>Inherit sans</option>{{end}}{{range $.Fonts}}<option value="{{.ID}}" {{if eq $field.Edit .ID}}selected{{end}}>{{.ID}}</option>{{end}}</select></label><small>Source: <code>{{if .Value}}{{.Value}}{{else}}inherits sans{{end}}</code></small>{{if .Error}}<p class="sg-field-error" id="sg-field-{{.Path}}-error">{{.Error}}</p>{{end}}</div>{{end}}</details>
<details {{if .RadiiOpen}}open{{end}}><summary>Corner radii</summary><p class="sg-note">Nonnegative px or rem values. Full-round source values remain available.</p>{{range .Radii}}<div class="sg-field"><label>{{.Path}}<input id="sg-field-{{.Path}}" name="{{.Path}}" value="{{.Edit}}" placeholder="{{.Value}}" maxlength="512" autocomplete="off" {{if .Error}}aria-invalid="true" aria-describedby="sg-field-{{.Path}}-error"{{end}}></label><small>Source: <code>{{.Value}}</code></small>{{if .Error}}<p class="sg-field-error" id="sg-field-{{.Path}}-error">{{.Error}}</p>{{end}}</div>{{end}}</details>
</aside>
<div id="sg-review" class="sg-review" role="region" aria-label="Candidate preview and checks" tabindex="-1"><div class="sg-controls">
<label>App specimen<select name="specimen">{{range .Specimens}}<option value="{{.ID}}" {{if eq $.Specimen .ID}}selected{{end}}>{{.Name}}</option>{{end}}</select></label>
<label>Scheme<select name="scheme"><option value="light" {{if eq .Scheme "light"}}selected{{end}}>Light</option><option value="dark" {{if eq .Scheme "dark"}}selected{{end}}>Dark</option></select></label>
<label>Preview width<select name="viewport"><option value="wide" {{if eq .Viewport "wide"}}selected{{end}}>Available width</option><option value="narrow" {{if eq .Viewport "narrow"}}selected{{end}}>Narrow · up to 390px</option></select></label>
<button class="sg-primary" name="action" value="preview">Preview candidate</button><button name="action" value="baseline">Show source</button></div>
<div class="sg-preview-heading"><h2>{{if eq .Version "candidate"}}Candidate{{else}}Source baseline{{end}}</h2><a href="{{.PreviewURL}}" target="_blank" rel="noopener noreferrer">Open preview in new tab ↗</a></div>
<iframe class="sg-preview sg-preview--{{.Viewport}}" src="{{.PreviewURL}}" title="{{.Title}} {{.Version}} specimen in {{.Scheme}} scheme" sandbox="" referrerpolicy="no-referrer" width="100%" height="600"></iframe>
<p class="sg-note">Static app-owned specimen. Forms/scripts are disabled; declared fonts may use system fallbacks. Higher-specificity app styles can override tokens. Inspect the rendered result before adoption.</p>
<section aria-labelledby="sg-diff"><h2 id="sg-diff">Token diff <span>{{len .Candidate.Changes}} changes</span></h2>{{if .Candidate.Changes}}<div class="sg-table-scroll"><table><caption>Resolved candidate compared with immutable source</caption><thead><tr><th>Token</th><th>Before</th><th>After</th></tr></thead><tbody>{{range .Candidate.Changes}}<tr><th scope="row">{{.Path}}</th><td><code>{{.Before}}</code></td><td><code>{{.After}}</code></td></tr>{{end}}</tbody></table></div>{{else}}<p class="sg-note">No resolved token changes.</p>{{end}}</section>
<section aria-labelledby="sg-checks"><h2 id="sg-checks">App-selected contrast checks</h2><p class="sg-note">These checks measure selected token pairs, not whole-screen accessibility or font fidelity.</p><div class="sg-table-scroll"><table><caption>Contrast against the app's configured minimums</caption><thead><tr><th>Check</th><th>Scheme</th><th>Ratio / minimum</th><th>Result</th></tr></thead><tbody>{{range .Candidate.Checks}}<tr><th scope="row">{{.ID}}</th><td>{{.Scheme}}</td><td>{{if .Error}}{{.Error}}{{else}}{{printf "%.2f" .Ratio}} / {{printf "%.2f" .Minimum}}{{end}}</td><td>{{if .Passed}}Pass{{else}}Needs review{{end}}</td></tr>{{end}}</tbody></table></div></section>
<section id="sg-export" class="sg-export" aria-labelledby="sg-export-title" tabindex="-1"><h2 id="sg-export-title">Reviewable export</h2><p>Includes the exact original CUE, a separately labeled concrete candidate, generated CSS, token diff and identity receipt. The candidate does not preserve authored CUE comments or expressions. Nothing is applied automatically.</p><button name="action" value="export" {{if or (not .Reviewed) (not .Candidate.Exportable)}}disabled{{end}}>Export acknowledged candidate ZIP</button><p class="sg-note">Preview first, then inspect this diff. Editing after preview invalidates the acknowledgement. This is not authenticated human-approval proof.</p></section>
</div></form>
<details class="sg-identity"><summary>Source and kit identity</summary><dl><dt>Source label</dt><dd>{{.SourceLabel}}</dd><dt>Original source SHA-256</dt><dd>{{.SourceSHA}}</dd><dt>Selected kit identity</dt><dd>{{.KitRevision}}</dd><dt>Candidate CUE SHA-256</dt><dd>{{.Candidate.SHA256}}</dd></dl><p class="sg-note">Local experimental tool, no consumer migration or runtime-preference write. Keep authentication around all mounted routes when your app requires it. The original source file is included whole in the export.</p></details>
</main></body></html>`

const previewCSS = `
html { background: var(--ak-color-surface); color: var(--ak-color-ink); }
body { margin: 0; padding: 1rem; font-family: var(--ak-font-sans); }
`

const previewHTML = `<!doctype html><html lang="en" data-theme="{{.Scheme}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Name}} · isolated specimen</title>{{range .Styles}}<link rel="stylesheet" href="{{.}}">{{end}}<style>{{.CSS}}</style></head><body><main>{{.Fragment}}</main></body></html>`
