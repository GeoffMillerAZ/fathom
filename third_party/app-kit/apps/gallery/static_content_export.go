package gallery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const staticContentCSSLimit = 128 << 10

type staticExportManifest struct {
	SchemaVersion    int                        `json:"schema_version"`
	BasePath         string                     `json:"base_path"`
	Files            []staticExportManifestFile `json:"files"`
	ManifestExcludes []string                   `json:"manifest_excludes"`
}

type staticExportManifestFile struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type staticExportPage struct {
	name, title, theme string
	body               template.HTML
}

// ExportStaticContent writes the Gallery's experimental two-page static-content
// reference to a fresh absolute directory. css must be explicitly trusted compiled
// input; this function bounds and preserves it, but is not an HTML or CSS sanitizer.
func ExportStaticContent(out string, css []byte, basePath string) error {
	if len(css) == 0 || len(css) > staticContentCSSLimit || !utf8.Valid(css) {
		return errors.New("static content CSS must be nonempty valid UTF-8 within 128 KiB")
	}
	if err := validateStaticExportBasePath(basePath); err != nil {
		return err
	}
	root, err := freshStaticExportPath(out)
	if err != nil {
		return err
	}
	files, err := buildStaticExport(css, basePath)
	if err != nil {
		return err
	}
	if err := validateStaticExport(files, basePath); err != nil {
		return fmt.Errorf("validate static content export: %w", err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		return fmt.Errorf("create static content export: %w", err)
	}
	outputRoot, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open static content export: %w", err)
	}
	defer outputRoot.Close()
	names := removeStaticExportName(sortedStaticExportNames(files), "manifest.json")
	names = append(names, "manifest.json")
	for _, name := range names {
		file, err := outputRoot.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
		written, writeErr := file.Write(files[name])
		if writeErr == nil && written != len(files[name]) {
			writeErr = io.ErrShortWrite
		}
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

func validateStaticExportBasePath(basePath string) error {
	if basePath == "/" {
		return nil
	}
	if len(basePath) < 3 || basePath[0] != '/' || basePath[len(basePath)-1] != '/' || strings.Contains(basePath, "//") {
		return errors.New("static content base path must be a canonical absolute prefix ending in slash")
	}
	for _, segment := range strings.Split(strings.Trim(basePath, "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return errors.New("static content base path contains an unsafe segment")
		}
		for _, value := range []byte(segment) {
			if !(value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || strings.ContainsRune("-._~", rune(value))) {
				return errors.New("static content base path must contain only ASCII unreserved segments")
			}
		}
	}
	return nil
}

func freshStaticExportPath(out string) (string, error) {
	if !filepath.IsAbs(out) || filepath.Clean(out) != out {
		return "", errors.New("static content export path must be clean and absolute")
	}
	parent := filepath.Dir(out)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", fmt.Errorf("resolve static content export parent: %w", err)
	}
	if realParent != parent {
		return "", errors.New("static content export parent must not contain symlinks")
	}
	if info, err := os.Lstat(out); err == nil {
		return "", fmt.Errorf("static content export target already exists (%s)", info.Mode().Type())
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect static content export target: %w", err)
	}
	return out, nil
}

func buildStaticExport(css []byte, basePath string) (map[string][]byte, error) {
	files := map[string][]byte{"app.css": bytes.Clone(css)}
	for _, theme := range []string{"", "light", "dark"} {
		suffix := staticExportSuffix(theme)
		indexName, profileName, notFoundName := "index"+suffix+".html", "profile"+suffix+".html", "404"+suffix+".html"
		view, err := newStaticContentView(staticContentGuide(), staticContentRequest{Theme: theme, State: staticContentReady})
		if err != nil {
			return nil, err
		}
		view.Static = true
		view.Reset = indexName
		view.Destination = func(id string) string {
			fragment := map[string]string{"choosing-profile": "content-profile-title", "catalog": "content-runtime-fit", "quality": "content-delivery-evidence"}[id]
			return profileName + "#" + fragment
		}
		indexBody, err := htmlf("static-content-export", staticContentHTML, view)
		if err != nil {
			return nil, err
		}
		profileBody, err := htmlf("static-content-profile-export", staticContentArticleHTML, map[string]string{"Back": indexName + "#content-guide"})
		if err != nil {
			return nil, err
		}
		notFoundBody, err := htmlf("static-content-404-export", staticContentProblemHTML, map[string]string{"Message": "This static content destination does not exist.", "Back": basePath + indexName})
		if err != nil {
			return nil, err
		}
		pages := []staticExportPage{
			{name: indexName, title: "Choose an integration profile", theme: theme, body: indexBody},
			{name: profileName, title: "Choosing a profile", theme: theme, body: profileBody},
			{name: notFoundName, title: "Content destination unavailable", theme: theme, body: notFoundBody},
		}
		for _, page := range pages {
			body, err := renderStaticExportPage(page, basePath)
			if err != nil {
				return nil, err
			}
			files[page.name] = body
		}
	}
	manifest := staticExportManifest{SchemaVersion: 1, BasePath: basePath, ManifestExcludes: []string{"manifest.json"}}
	for _, name := range sortedStaticExportNames(files) {
		digest := sha256.Sum256(files[name])
		manifest.Files = append(manifest.Files, staticExportManifestFile{Path: name, Bytes: len(files[name]), SHA256: hex.EncodeToString(digest[:])})
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	files["manifest.json"] = append(body, '\n')
	return files, nil
}

func staticExportSuffix(theme string) string {
	if theme == "" {
		return ""
	}
	return "-" + theme
}

func renderStaticExportPage(page staticExportPage, basePath string) ([]byte, error) {
	t := template.Must(template.New("static-export-page").Parse(`<!doctype html>
<html lang="en"{{if .Theme}} data-theme="{{.Theme}}"{{end}}><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'"><title>{{.Title}} · app-kit static reference</title><link rel="stylesheet" href="{{.Stylesheet}}"></head>
<body class="bg-surface-sunken font-sans text-ink">
<a href="#static-reference-main" class="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:inline-flex focus:min-h-11 focus:items-center focus:rounded-md focus:bg-surface focus:px-4 focus:py-2 focus:text-ink focus:outline-2 focus:outline-focus">Skip to content</a>
<header class="border-b border-line bg-surface"><div class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-6 py-2">
<a href="{{.Index}}" class="inline-flex min-h-11 items-center rounded-md font-semibold text-ink focus-visible:outline-2 focus-visible:outline-focus">Static content reference</a>
<nav aria-label="Appearance" class="flex flex-wrap gap-x-3 gap-y-1 text-sm">
<a href="{{.System}}" class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-accent underline focus-visible:outline-2 focus-visible:outline-focus"{{if .SystemCurrent}} aria-current="page"{{end}}>System</a>
<a href="{{.Light}}" class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-accent underline focus-visible:outline-2 focus-visible:outline-focus"{{if .LightCurrent}} aria-current="page"{{end}}>Light</a>
<a href="{{.Dark}}" class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md text-accent underline focus-visible:outline-2 focus-visible:outline-focus"{{if .DarkCurrent}} aria-current="page"{{end}}>Dark</a>
</nav></div></header>
<main id="static-reference-main" tabindex="-1" class="mx-auto max-w-5xl px-6 py-10 focus:outline-2 focus:outline-focus">{{.Body}}</main>
</body></html>
`))
	stem := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(page.name, ".html"), "-light"), "-dark")
	name := func(theme string) string { return stem + staticExportSuffix(theme) + ".html" }
	link := func(name string) string { return name }
	stylesheet := "app.css"
	if strings.HasPrefix(page.name, "404") {
		link = func(name string) string { return basePath + name }
		stylesheet = basePath + "app.css"
	}
	data := struct {
		Title, Theme, Index, System, Light, Dark, Stylesheet string
		SystemCurrent, LightCurrent, DarkCurrent             bool
		Body                                                 template.HTML
	}{page.title, page.theme, link("index" + staticExportSuffix(page.theme) + ".html"), link(name("")), link(name("light")), link(name("dark")), stylesheet, page.theme == "", page.theme == "light", page.theme == "dark", page.body}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func validateStaticExport(files map[string][]byte, basePath string) error {
	ids := map[string]map[string]bool{}
	hrefs := map[string][]string{}
	for name, body := range files {
		if !strings.HasSuffix(name, ".html") {
			continue
		}
		doc, err := html.Parse(bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		ids[name] = map[string]bool{}
		var walk func(*html.Node) error
		walk = func(node *html.Node) error {
			if node.Type == html.ElementNode {
				switch node.Data {
				case "script", "style", "base", "form":
					return fmt.Errorf("%s contains forbidden %s", name, node.Data)
				}
				for _, attr := range node.Attr {
					if attr.Key == "id" {
						if ids[name][attr.Val] {
							return fmt.Errorf("%s duplicates id %s", name, attr.Val)
						}
						ids[name][attr.Val] = true
					}
					if attr.Key == "href" {
						hrefs[name] = append(hrefs[name], attr.Val)
					}
				}
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if err := walk(child); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(doc); err != nil {
			return err
		}
		if !bytes.Contains(body, []byte(`default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'`)) {
			return fmt.Errorf("%s lacks the static CSP", name)
		}
	}
	for source, links := range hrefs {
		is404 := strings.HasPrefix(source, "404") && strings.HasSuffix(source, ".html")
		for _, href := range links {
			u, err := url.Parse(href)
			if err != nil || u.IsAbs() || u.Host != "" || u.RawQuery != "" {
				return fmt.Errorf("%s has unsafe href %q", source, href)
			}
			target := u.Path
			if strings.HasPrefix(target, "/") {
				if !is404 || !strings.HasPrefix(target, basePath) {
					return fmt.Errorf("%s has unselected absolute href %q", source, href)
				}
				target = strings.TrimPrefix(target, basePath)
			} else if is404 && target != "" {
				return fmt.Errorf("%s has relative deep-404 href %q", source, href)
			}
			if target != "" && pathpkg.Base(target) != target {
				return fmt.Errorf("%s has unsafe or non-flat href %q", source, href)
			}
			if target == "" {
				target = source
			}
			if _, ok := files[target]; !ok {
				return fmt.Errorf("%s links to missing %s", source, target)
			}
			if u.Fragment != "" && !ids[target][u.Fragment] {
				return fmt.Errorf("%s links to missing fragment %s#%s", source, target, u.Fragment)
			}
		}
	}
	return nil
}

func sortedStaticExportNames(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func removeStaticExportName(names []string, remove string) []string {
	kept := names[:0]
	for _, name := range names {
		if name != remove {
			kept = append(kept, name)
		}
	}
	return kept
}
