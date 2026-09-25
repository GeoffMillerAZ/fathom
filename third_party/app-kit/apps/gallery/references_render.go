package gallery

import "net/url"

// These labels describe the imported product, not compatibility or maturity.
func referenceTitle(family string) string {
	if name, ok := map[string]string{
		"application-ui": "Application UI", "catalyst": "Catalyst", "commit": "Commit",
		"compass": "Compass", "ecommerce": "Ecommerce", "keynote": "Keynote",
		"marketing": "Marketing", "oatmeal": "Oatmeal", "pocket": "Pocket",
		"primer": "Primer", "protocol": "Protocol", "radiant": "Radiant",
		"salient": "Salient", "spotlight": "Spotlight", "studio": "Studio",
		"syntax": "Syntax", "transmit": "Transmit",
	}[family]; ok {
		return name
	}
	return family
}

func referenceKind(kind string) string {
	switch kind {
	case "ui-kit":
		return "UI kit"
	case "ui-blocks":
		return "UI blocks"
	case "template":
		return "Template / kit"
	default:
		return kind
	}
}

func referenceURL(id, query string) string {
	return "/references/" + url.PathEscape(id) + query
}

type referenceInput struct{ Name, Value string }

// Carry only the catalog selection through file-filter forms, never a free-form
// return URL or current source path. New file filters reset their own cursor.
func referenceContext(back string) []referenceInput {
	u, err := url.Parse(back)
	if err != nil {
		return nil
	}
	var inputs []referenceInput
	for _, key := range []string{"q", "kind", "family", "limit", "cursor", "theme"} {
		if value := u.Query().Get(key); value != "" {
			inputs = append(inputs, referenceInput{key, value})
		}
	}
	return inputs
}

const referenceIndexHTML = `
<div class="max-w-3xl">
  <p class="text-sm font-medium text-accent">PRIVATE DESIGN REFERENCES</p>
  <h1 class="mt-2 text-3xl font-semibold">Start with a complete design</h1>
  <p class="mt-3 text-ink-muted">Explore your purchased projects, inspect their dependencies, and read the original source. Choose a visual reference here, then a compatible renderer in the capability catalog.</p>
</div>
<nav aria-label="Private library" class="mt-6 flex flex-wrap gap-5 text-sm"><span aria-current="page" class="font-medium">Originals</span><a href="{{.ConversionsURL}}" class="text-accent underline underline-offset-4">Saved conversions</a><a href="{{.EvidenceURL}}" class="inline-flex min-h-11 items-center text-accent underline underline-offset-4">Target evidence</a></nav>
<form action="/references" method="get" role="search" aria-label="Design reference library" class="mt-8 grid gap-4 border-y border-line py-5 sm:grid-cols-3">
  {{if .Theme}}<input type="hidden" name="theme" value="{{.Theme}}">{{end}}
  {{if .Query.Family}}<input type="hidden" name="family" value="{{.Query.Family}}">{{end}}
  <div class="sm:col-span-2"><label for="reference-query" class="block text-sm font-medium">Project or file name</label><input id="reference-query" type="search" name="q" value="{{.Query.Text}}" placeholder="Try Catalyst, settings, or dashboard" class="mt-2 w-full rounded-md border border-line-strong bg-surface-raised p-2 text-ink focus-visible:outline-2 focus-visible:outline-focus"></div>
  <div><label for="reference-kind" class="block text-sm font-medium">Collection</label><select id="reference-kind" name="kind" class="mt-2 w-full rounded-md border border-line-strong bg-surface-raised p-2 text-ink focus-visible:outline-2 focus-visible:outline-focus"><option value="">All collections</option><option value="ui-kit"{{if eq .Query.Kind "ui-kit"}} selected{{end}}>UI kits</option><option value="template"{{if eq .Query.Kind "template"}} selected{{end}}>Templates / kits</option><option value="ui-blocks"{{if eq .Query.Kind "ui-blocks"}} selected{{end}}>UI blocks</option></select></div>
  <div class="flex flex-wrap items-center gap-4 sm:col-span-3"><button class="rounded-md bg-accent px-4 py-2 font-medium text-accent-ink hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Find references</button><a href="{{.Reset}}" class="text-sm text-ink-muted underline underline-offset-4">Reset filters</a>{{if .Query.Family}}<span class="text-sm text-ink-muted">Family: {{referenceTitle .Query.Family}}</span>{{end}}</div>
</form>
<section class="mt-8" aria-labelledby="reference-results">
  <div class="flex flex-wrap items-baseline justify-between gap-2"><h2 id="reference-results" class="text-lg font-semibold">{{.Page.Total}} matching archives</h2><p class="text-xs text-ink-muted">Inventory metadata. Opening a project verifies its ZIP.</p></div>
  <div class="mt-3 border-t border-line">{{range .Page.Items}}
    <article class="grid gap-3 border-b border-line py-5 sm:grid-cols-3">
      <div class="min-w-0 sm:col-span-2"><h3 class="text-lg font-semibold"><a href="{{referenceURL .ID $.DetailQuery}}" class="text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-focus">{{referenceTitle .Family}}</a></h3><p class="mt-1 break-all text-sm text-ink-muted">{{.Filename}}</p><p class="mt-2 text-xs text-ink-muted">Tailwind Plus · original licensed source</p></div>
      <div class="text-sm"><p class="font-medium">{{referenceKind .Kind}}</p><p class="mt-1 text-ink-muted">{{.FileCount}} files · {{.Projects}} project manifests</p><details class="mt-2 text-xs text-ink-muted"><summary class="cursor-pointer">Source revision</summary><p class="mt-2 break-all font-mono">{{.SHA256}}</p></details></div>
    </article>
  {{else}}<div class="py-10"><h3 class="font-semibold">No reference matches these filters</h3><p class="mt-2 max-w-2xl text-ink-muted">Search uses project metadata and file paths, not every word inside the source. Try a product name, a shorter file name, or another collection.</p><a href="{{.Reset}}" class="mt-4 inline-block text-accent underline">Clear filters and browse your library</a></div>{{end}}</div>
  {{if .Next}}<a href="{{.Next}}" class="mt-5 inline-block rounded-md border border-line-strong px-4 py-2 text-accent focus-visible:outline-2 focus-visible:outline-focus">Next page →</a>{{end}}
</section>
<aside class="mt-8 grid gap-5 border-t border-line pt-6 sm:grid-cols-2" aria-label="Using purchased references">
  <div><h2 class="font-semibold">Source is not a running preview</h2><p class="mt-2 text-sm text-ink-muted">Imports do not install dependencies or execute scripts. External fonts, media and services may still be required. This library does not claim offline rendering or runtime compatibility.</p></div>
  <div><h2 class="font-semibold">Preserve what makes the design work</h2><p class="mt-2 text-sm text-ink-muted">Inspect complete pages, spacing, typography and interaction states before adapting. Keep dark, hover and on-fill contrast distinctions. Compare the working result with its original reference.</p></div>
</aside>
<details class="mt-8 text-xs text-ink-muted"><summary class="cursor-pointer">Shared inventory revision and privacy</summary><p class="mt-2 break-all font-mono">{{.Revision}}</p><p class="mt-2">The CLI and MCP use this same inventory. Licensed source stays in your local, Git-ignored store and needs a separate backup. Do not publish this library or its contents.</p></details>
`

const referenceDetailHTML = `
<a href="{{.Back}}" class="text-sm text-accent underline underline-offset-4">← Reference library</a>
<div class="mt-5 flex flex-wrap items-start justify-between gap-4">
  <div class="min-w-0"><p class="text-sm text-ink-muted">{{referenceKind .Asset.Kind}} · Tailwind Plus</p><h1 class="mt-2 text-3xl font-semibold">{{referenceTitle .Asset.Family}}</h1><p class="mt-3 break-all text-sm text-ink-muted">{{.Asset.Filename}}</p></div>
  <a href="{{.Asset.SourceURL}}" rel="noreferrer" class="rounded-md border border-line-strong px-4 py-2 text-sm text-accent focus-visible:outline-2 focus-visible:outline-focus">Open vendor page ↗</a>
</div>
<p class="mt-5 max-w-3xl text-sm text-ink-muted">Original ZIP verified against its recorded SHA-256 and inventory for this request. {{.Asset.FileCount}} files, {{.Asset.Projects}} project manifests. This is licensed source, not an installed or tested application.</p>
<a href="{{.ConversionsURL}}" class="mt-4 inline-block text-sm text-accent underline underline-offset-4">Saved conversions from this archive</a>
<nav aria-label="Reference sections" class="mt-6 flex flex-wrap gap-5 border-b border-line pb-4 text-sm"><a href="#reference-files" class="text-accent underline underline-offset-4">Browse files</a><a href="#reference-projects" class="text-accent underline underline-offset-4">Dependencies</a><a href="#reference-provenance" class="text-accent underline underline-offset-4">Provenance</a></nav>
<section id="reference-files" class="mt-8" aria-labelledby="reference-files-title">
  <h2 id="reference-files-title" class="text-xl font-semibold">Files and documentation</h2><p class="mt-2 text-sm text-ink-muted">Start with a README, license, page, or theme. Text is displayed without executing it; binary and sensitive files cannot be read here.</p>
  <form action="{{referenceURL .Asset.ID ""}}" method="get" aria-label="Filter reference files" class="mt-5 grid gap-4 sm:grid-cols-3">
    {{range referenceContext .Back}}<input type="hidden" name="{{.Name}}" value="{{.Value}}">{{end}}
    <div class="sm:col-span-2"><label for="reference-prefix" class="block text-sm font-medium">Path prefix</label><input id="reference-prefix" name="prefix" value="{{.FileQuery.Prefix}}" placeholder="Paste the beginning of a path below" class="mt-2 w-full rounded-md border border-line-strong bg-surface-raised p-2 text-ink focus-visible:outline-2 focus-visible:outline-focus"></div>
    <div><label for="reference-file-kind" class="block text-sm font-medium">File type</label><select id="reference-file-kind" name="file_kind" class="mt-2 w-full rounded-md border border-line-strong bg-surface-raised p-2 text-ink focus-visible:outline-2 focus-visible:outline-focus"><option value="">All files</option><option value="readme"{{if eq .FileQuery.Kind "readme"}} selected{{end}}>README</option><option value="license"{{if eq .FileQuery.Kind "license"}} selected{{end}}>License</option><option value="source"{{if eq .FileQuery.Kind "source"}} selected{{end}}>Source</option><option value="theme"{{if eq .FileQuery.Kind "theme"}} selected{{end}}>Styles</option><option value="manifest"{{if eq .FileQuery.Kind "manifest"}} selected{{end}}>Manifests</option><option value="config"{{if eq .FileQuery.Kind "config"}} selected{{end}}>Configuration</option><option value="documentation"{{if eq .FileQuery.Kind "documentation"}} selected{{end}}>Documentation</option><option value="asset"{{if eq .FileQuery.Kind "asset"}} selected{{end}}>Assets</option><option value="other"{{if eq .FileQuery.Kind "other"}} selected{{end}}>Other</option></select></div>
    <div class="flex flex-wrap items-center gap-4 sm:col-span-3"><button class="rounded-md bg-accent px-4 py-2 font-medium text-accent-ink hover:bg-accent-hover focus-visible:outline-2 focus-visible:outline-focus focus-visible:outline-offset-2">Filter files</button><a href="{{.ResetFiles}}" class="text-sm text-ink-muted underline underline-offset-4">Reset file filters</a></div>
  </form>
  <p class="mt-5 text-sm font-medium">{{.Files.Total}} matching files</p>
  <ul class="mt-2 divide-y divide-line border-y border-line">{{range .Files.Items}}<li class="flex min-w-0 flex-wrap items-baseline justify-between gap-2 py-3"><a href="{{index $.FileLinks .Path}}" class="min-w-0 break-all font-mono text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-focus">{{.Path}}</a><span class="text-xs text-ink-muted">{{.Kind}} · {{.Size}} bytes</span></li>{{else}}<li class="py-6 text-sm text-ink-muted">No files match. Clear the path prefix or choose another file type.</li>{{end}}</ul>
  {{if .Next}}<a href="{{.Next}}" class="mt-4 inline-block rounded-md border border-line-strong px-4 py-2 text-sm text-accent">Next files →</a>{{end}}
</section>
{{if or .Source .SourceProblem}}<section id="reference-source" tabindex="-1" class="mt-8 min-w-0 border-t border-line pt-6" aria-labelledby="reference-source-title">
  <div class="flex flex-wrap items-baseline justify-between gap-3"><h2 id="reference-source-title" class="text-xl font-semibold">Original source</h2><a href="{{.ClearFile}}" class="text-sm text-accent underline">Close source</a></div>
  {{if .SourceProblem}}<p role="status" class="mt-3 text-sm text-ink-muted">{{.SourceProblem}}</p>{{else}}
    <p class="mt-3 break-all font-mono text-sm">{{.Source.Path}}</p><p class="mt-2 text-xs text-ink-muted">{{.Source.Size}} bytes · verified original UTF-8 text · not instructions to execute</p>
    {{if .SourceTooLarge}}<p class="mt-4 max-w-2xl text-sm text-ink-muted">This file exceeds the 32 KiB inline display limit. No source was truncated. Read the complete verified text as JSON, or use <code>appkit data assets-get -id ARCHIVE_ID -path EXACT_MEMBER_PATH</code>.</p>{{else}}<pre tabindex="0" role="region" aria-label="Original source text" class="mt-4 max-h-[32rem] max-w-full overflow-auto rounded-md border border-line bg-surface-sunken p-4 font-mono text-xs leading-relaxed focus-visible:outline-2 focus-visible:outline-focus"><code>{{.Source.Content}}</code></pre>{{end}}
    <a href="{{.SourceURL}}" class="mt-4 inline-block text-sm text-accent underline">Read verified source as JSON</a>
    <a href="{{.SourceConversionsURL}}" class="mt-4 block text-sm text-accent underline">View saved conversions for this file</a>
  {{end}}
</section>{{end}}
<section id="reference-projects" class="mt-10 border-t border-line pt-6" aria-labelledby="reference-projects-title">
  <h2 id="reference-projects-title" class="text-xl font-semibold">Project dependencies</h2><p class="mt-2 max-w-3xl text-sm text-ink-muted">These are original manifest declarations, not installed or resolved versions. Read lockfiles and setup instructions before running a reference. Importing did not install anything.</p>
  {{range .Projects}}<details class="mt-5 rounded-md border border-line p-4"><summary class="cursor-pointer text-sm font-medium"><span class="break-all font-mono">{{.Path}}</span>{{if .Name}}<span class="mt-1 block text-ink-muted">{{.Name}}{{if .Version}} · {{.Version}}{{end}}</span>{{end}}</summary><div class="mt-5 grid min-w-0 gap-6 sm:grid-cols-2">
    <div><h3 class="text-sm font-semibold">Runtime dependencies</h3><dl class="mt-2 space-y-2 text-xs">{{range $name,$version := .Dependencies}}<div class="flex flex-wrap justify-between gap-2"><dt class="break-all font-mono">{{$name}}</dt><dd class="break-all font-mono text-ink-muted">{{$version}}</dd></div>{{else}}<div><dt class="sr-only">Runtime</dt><dd class="text-ink-muted">None declared</dd></div>{{end}}</dl></div>
    <div><h3 class="text-sm font-semibold">Development dependencies</h3><dl class="mt-2 space-y-2 text-xs">{{range $name,$version := .DevDependencies}}<div class="flex flex-wrap justify-between gap-2"><dt class="break-all font-mono">{{$name}}</dt><dd class="break-all font-mono text-ink-muted">{{$version}}</dd></div>{{else}}<div><dt class="sr-only">Development</dt><dd class="text-ink-muted">None declared</dd></div>{{end}}</dl></div>
    {{if .PeerDependencies}}<div><h3 class="text-sm font-semibold">Peer dependencies</h3><dl class="mt-2 space-y-2 text-xs">{{range $name,$version := .PeerDependencies}}<div><dt class="break-all font-mono">{{$name}}</dt><dd class="break-all font-mono text-ink-muted">{{$version}}</dd></div>{{end}}</dl></div>{{end}}
    {{if .OptionalDependencies}}<div><h3 class="text-sm font-semibold">Optional dependencies</h3><dl class="mt-2 space-y-2 text-xs">{{range $name,$version := .OptionalDependencies}}<div><dt class="break-all font-mono">{{$name}}</dt><dd class="break-all font-mono text-ink-muted">{{$version}}</dd></div>{{end}}</dl></div>{{end}}
  </div></details>{{else}}<p class="mt-4 text-sm text-ink-muted">No package.json project manifests in this archive. That does not prove the source has no runtime dependencies.</p>{{end}}
</section>
<section id="reference-provenance" class="mt-10 border-t border-line pt-6" aria-labelledby="reference-provenance-title"><h2 id="reference-provenance-title" class="text-xl font-semibold">Provenance and access</h2><dl class="mt-4 space-y-4 text-xs"><div><dt class="font-medium">Stable archive ID</dt><dd class="mt-1 break-all font-mono text-ink-muted">{{.Asset.ID}}</dd></div><div><dt class="font-medium">Archive SHA-256</dt><dd class="mt-1 break-all font-mono text-ink-muted">{{.Asset.SHA256}}</dd></div><div><dt class="font-medium">Shared inventory revision</dt><dd class="mt-1 break-all font-mono text-ink-muted">{{.Revision}}</dd></div></dl><p class="mt-5 max-w-3xl text-sm text-ink-muted">Private licensed reference. Preserve license files, source attribution and access restrictions. Modifying a template does not automatically grant redistribution rights. The ignored archive store needs a separate backup.</p></section>
`

const referenceProblemHTML = `
<div class="max-w-3xl"><p class="text-sm font-medium text-accent">PRIVATE DESIGN REFERENCES</p><h1 class="mt-2 text-3xl font-semibold">{{.Title}}</h1><p class="mt-4 text-ink-muted">{{.Message}}</p>
{{if .Setup}}<h2 class="mt-8 text-lg font-semibold">Connect your local library</h2><p class="mt-3 text-sm text-ink-muted">From the app-kit checkout, import the official ZIPs you downloaded. This preserves originals and does not execute their code.</p><pre tabindex="0" class="mt-4 overflow-x-auto rounded-md bg-surface-sunken p-4 text-xs"><code>go run ./cmd/appkit data assets-import -src /path/to/downloaded-zips
task gallery:run</code></pre><p class="mt-3 text-sm text-ink-muted">For another store, launch Gallery with <code>-design-assets /path/to/private-store</code>. Native capabilities work without purchased data.</p>{{end}}
<div class="mt-6 flex flex-wrap gap-5"><a href="{{.Back}}" class="text-accent underline underline-offset-4">Return to the reference library</a><a href="{{referenceLibraryURL .Theme}}" class="text-accent underline underline-offset-4">Retry connection</a><a href="/catalog" class="text-accent underline underline-offset-4">Browse native capabilities</a></div></div>
`
