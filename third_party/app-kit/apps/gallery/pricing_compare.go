package gallery

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
)

// These are first-party workspace controls, never executable reference content.
// Their admission is deliberately separate from the prepared documents' CSP.
//
//go:embed pricing_compare.js
var pricingCompareJS []byte

//go:embed pricing_compare.css
var pricingCompareCSS []byte

func pricingCompareIntegrity() string {
	h := sha256.Sum256(pricingCompareJS)
	return "sha256-" + base64.StdEncoding.EncodeToString(h[:])
}

func pricingCompareCSP() string {
	return "default-src 'none'; script-src '" + pricingCompareIntegrity() + "'; style-src 'self'; frame-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"
}

func registerPricingCompareAssets(register func(string, bool, http.HandlerFunc)) {
	for _, asset := range []struct {
		path, mime string
		body       []byte
	}{
		{pricingReferenceRoute + "/workspace.js", "application/javascript; charset=utf-8", pricingCompareJS},
		{pricingReferenceRoute + "/workspace.css", "text/css; charset=utf-8", pricingCompareCSS},
	} {
		register(asset.path, false, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery != "" || r.URL.EscapedPath() != asset.path {
				conversionBadQuery(w, r, false)
				return
			}
			w.Header().Set("Content-Type", asset.mime)
			w.Header().Set("Content-Length", strconv.Itoa(len(asset.body)))
			_, _ = w.Write(asset.body)
		})
	}
}

func configurePricingCompare(v *pricingReferenceView, query url.Values) {
	v.Layout, v.Scale, v.Active = query.Get("layout"), query.Get("scale"), query.Get("active")
	if v.Layout == "" {
		v.Layout = "flip"
	}
	if v.Scale == "" {
		v.Scale = "fit"
	}
	if v.Active == "" {
		v.Active = "original"
	}
	v.ScriptIntegrity = pricingCompareIntegrity()
	q := url.Values{"layout": {v.Layout}, "scale": {v.Scale}, "viewport": {strconv.Itoa(v.Width)}, "preview_theme": {v.PreviewTheme}}
	if v.Theme != "" {
		q.Set("theme", v.Theme)
	}
	q.Set("active", "original")
	v.OriginalURL = referenceLink(pricingReferenceRoute, q)
	q.Set("active", "converted")
	v.ConvertedURL = referenceLink(pricingReferenceRoute, q)
}

const pricingReferenceHTML = `<!doctype html>
<html lang="en" {{if .Theme}}data-theme="{{.Theme}}"{{end}}><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>P02 pricing comparison · app-kit</title>
<link rel="stylesheet" href="/static/app.css"><link rel="stylesheet" href="/comparisons/p02-pricing/workspace.css">
<script src="/comparisons/p02-pricing/workspace.js" integrity="{{.ScriptIntegrity}}" defer></script>
</head><body><main id="comparison" data-layout="{{.Layout}}" data-active="{{.Active}}" data-scale="{{.Scale}}">
<header class="comparison-heading"><div><a href="{{.SourceURL}}">← Back to source</a><h1>P02 pricing <span>Original / templ</span></h1></div><div class="comparison-heading-actions"><span class="comparison-unverified">Unverified reference</span><button id="comparison-fullscreen" type="button" hidden>Enter fullscreen</button></div></header>
<form id="comparison-controls" action="/comparisons/p02-pricing" method="get" aria-label="Matched preview conditions">
{{if .Theme}}<input type="hidden" name="theme" value="{{.Theme}}">{{end}}<input type="hidden" name="active" value="{{.Active}}">
<label>Layout<select name="layout"><option value="flip" {{if eq .Layout "flip"}}selected{{end}}>Flip</option><option value="side" {{if eq .Layout "side"}}selected{{end}}>Side by side</option><option value="stack" {{if eq .Layout "stack"}}selected{{end}}>Above / below</option></select></label>
<label>Display size<select name="scale"><option value="fit" {{if eq .Scale "fit"}}selected{{end}}>Fit width</option><option value="actual" {{if eq .Scale "actual"}}selected{{end}}>Actual size · 100%</option></select></label>
<label>Viewport<select name="viewport"><option value="390" {{if eq .Width 390}}selected{{end}}>390 × 1000</option><option value="1440" {{if eq .Width 1440}}selected{{end}}>1440 × 1000</option></select></label>
<label>Preview theme<select name="preview_theme"><option value="light" {{if eq .PreviewTheme "light"}}selected{{end}}>Light</option><option value="dark" {{if eq .PreviewTheme "dark"}}selected{{end}}>Dark</option></select></label>
<button type="submit">Apply</button><output id="comparison-scale-label">{{if eq .Scale "actual"}}100%{{else}}100% · fit controls not initialized{{end}}</output>
</form>
<nav class="comparison-sides" aria-label="Active preview"><a data-select-side="original" href="{{.OriginalURL}}" {{if eq .Active "original"}}aria-current="true"{{end}}>Original</a><a data-select-side="converted" href="{{.ConvertedURL}}" {{if eq .Active "converted"}}aria-current="true"{{end}}>Converted</a><p>Same viewport and assets. Fit scales the view, not the layout.</p></nav>
<noscript><p class="comparison-note">Without JavaScript, use Apply or the Original/Converted links. Previews stay at actual size with scrolling; fit and browser fullscreen require workspace controls.</p></noscript>
<div class="comparison-panes">{{range .Panes}}<section class="comparison-pane" data-side="{{.Side}}" id="comparison-{{.Side}}" {{if and (eq $.Layout "flip") (ne $.Active .Side)}}hidden{{end}} aria-labelledby="comparison-label-{{.Side}}"><h2 id="comparison-label-{{.Side}}">{{.Label}}</h2><div class="comparison-window" tabindex="0" role="region" aria-label="Scrollable {{.Label}} preview"><div class="comparison-framebox"><iframe src="{{.URL}}" title="{{.Title}}" sandbox="" referrerpolicy="no-referrer" width="{{$.Width}}" height="1000"></iframe></div></div></section>{{end}}</div>
<p id="comparison-status" class="comparison-note" role="status" aria-live="polite">Scroll inside a preview for more content. Preview scroll positions are independent.</p>
<details class="comparison-evidence"><summary>Preparation, limitations and source identity</summary><div>
<p>Original HTML and compiled templ use the same chosen Tailwind 4.3.0 / InterVariable environment. This is not an exact portal baseline or verified target. At 100%, one specimen CSS pixel occupies one page CSS pixel; browser zoom remains separate.</p>
<p>These are static design specimens: example purchase links do not perform a purchase. Prepared documents allow no scripts, form submission or remote assets. First-party workspace controls only change the viewing conditions; they do not inspect frame contents, build, download dependencies or modify source.</p>
<p>Preparation checks source identity, compiled output, DOM equality and repeatable CSS. Browser containment, font loading, layout, keyboard behavior and fidelity still need independent review. Both sides remain unverified for adoption. The archive did not specify its original font or exact CSS patch.</p>
<dl><dt>Source member</dt><dd>{{.Descriptor.Receipt.Source.Path}}</dd><dt>Saved attempt</dt><dd>{{.Descriptor.Receipt.AttemptID}}</dd><dt>Prepared receipt SHA-256</dt><dd>{{.Descriptor.ReceiptSHA256}}</dd></dl>
</div></details></main></body></html>`
