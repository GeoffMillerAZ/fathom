package routes

// nextjs.go — the Next.js file-system route reader (LB-F329).

import (
	"bytes"
	"path"
	"strings"
)

func init() {
	Register(Reader{Framework: "nextjs", Langs: []string{"js", "ts"}, Read: readNext})
}

// nextRouteMethods are the HTTP methods an app/ route handler exports.
var nextRouteMethods = map[string]bool{
	"GET": true, "HEAD": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "OPTIONS": true,
}

// nextPageExts are Next's default pageExtensions.
var nextPageExts = map[string]bool{".js": true, ".jsx": true, ".ts": true, ".tsx": true}

// nextSkipDirs never hold a route Next serves from the source tree.
var nextSkipDirs = map[string]bool{
	"node_modules": true, ".next": true, "out": true, "dist": true, "build": true,
	"__tests__": true, "__mocks__": true, "cypress": true, "e2e": true,
}

// nextNotRoot are folders a Next project's pages/ or app/ never sits in.
var nextNotRoot = map[string]bool{
	"components": true, "lib": true, "utils": true, "helpers": true, "features": true, "modules": true,
	"views": true, "layouts": true, "containers": true, "ui": true, "shared": true, "common": true,
}

// nextSpecialPages are pages/ files Next renders but never routes by name.
var nextSpecialPages = map[string]bool{"_app": true, "_document": true, "_error": true, "404": true, "500": true, "_middleware": true}

// readNext reads Next.js routes from the file's path (LB-F329).
//
//   - app/**/page.* is a GET route at its folder path; app/**/route.* is
//     one route per exported HTTP-method function (GET, POST, …).
//   - pages/** is a GET page route and pages/api/** an ANY API route;
//     index names its folder; _app, _document, _error, 404 and 500 are
//     not routes.
//   - A route group (x) and a parallel-route slot @x add no segment; an
//     intercepting route ((.)x), a private folder (_x) and anything under
//     node_modules, build output or tests is not a route; [id], [...slug]
//     and [[...slug]] become {}.
//   - Next's path conventions gate the read: an app/ page or a pages/
//     file needs a default export (a non-API page also JSX or a react or
//     next import), an app/ route handler at least one method export.
//   - The handler is the default export's name (the innermost named
//     argument of a wrapper: withAuth(Page) is "Page"), "" when
//     anonymous; for a route handler, the exported method function.
func readNext(p string, src []byte) Result {
	ext := path.Ext(p)
	if !nextPageExts[ext] || strings.HasSuffix(p, ".d.ts") {
		return Result{}
	}
	base := strings.TrimSuffix(path.Base(p), ext)
	if strings.Contains(base, ".test") || strings.Contains(base, ".spec") || strings.Contains(base, ".stories") {
		return Result{}
	}
	segs := strings.Split(p, "/")
	for _, s := range segs[:len(segs)-1] {
		if nextSkipDirs[s] {
			return Result{}
		}
	}
	for i, s := range segs[:len(segs)-1] {
		if nextNotRoot[s] {
			return Result{} // a pages/ or app/ folder inside these is a component folder
		}
		switch s {
		case "app":
			if base == "page" || base == "route" {
				return readNextApp(p, src, segs[i+1:len(segs)-1], base)
			}
			return Result{}
		case "pages":
			return readNextPages(p, src, segs[i+1:len(segs)-1], base)
		}
	}
	return Result{}
}

// nextSegments turns app/ or pages/ folder names into a route path; ok is
// false when a folder makes the file no route.
func nextSegments(dirs []string, app bool) (string, bool) {
	var out []string
	for _, d := range dirs {
		switch {
		case app && strings.HasPrefix(d, "(."):
			return "", false // an intercepting route: (.)x, (..)x, (...)x
		case app && strings.HasPrefix(d, "(") && strings.HasSuffix(d, ")"):
			continue // a route group
		case app && strings.HasPrefix(d, "@"):
			continue // a parallel-route slot
		case app && strings.HasPrefix(d, "_"):
			return "", false // a private folder
		case strings.HasPrefix(d, "[") && strings.HasSuffix(d, "]"):
			out = append(out, "{}")
		default:
			out = append(out, d)
		}
	}
	return "/" + strings.Join(out, "/"), true
}

func readNextApp(p string, src []byte, dirs []string, base string) Result {
	route, ok := nextSegments(dirs, true)
	if !ok || !bytes.Contains(src, []byte("export")) {
		return Result{}
	}
	f := newJSFile(p, src)
	if base == "page" {
		h, line, ok := nextDefaultExport(f)
		if !ok {
			return Result{}
		}
		return Result{Routes: []Route{{Method: "GET", Path: route, Handler: h, File: p, Line: line}}}
	}
	var out Result
	for _, e := range nextNamedExports(f) {
		if nextRouteMethods[e.exported] {
			out.Routes = append(out.Routes, Route{Method: e.exported, Path: route, Handler: e.local, File: p, Line: e.line})
		}
	}
	sortRoutes(out.Routes)
	return out
}

func readNextPages(p string, src []byte, dirs []string, base string) Result {
	if len(dirs) == 0 && nextSpecialPages[base] {
		return Result{}
	}
	if !bytes.Contains(src, []byte("export")) {
		return Result{}
	}
	api := len(dirs) > 0 && dirs[0] == "api"
	if !api && !nextPageSignals(src) {
		return Result{}
	}
	if base != "index" {
		dirs = append(append([]string(nil), dirs...), base)
	}
	route, ok := nextSegments(dirs, false)
	if !ok {
		return Result{}
	}
	f := newJSFile(p, src)
	h, line, ok := nextDefaultExport(f)
	if !ok {
		return Result{}
	}
	method := "GET"
	if api {
		method = "ANY"
	}
	return Result{Routes: []Route{{Method: method, Path: route, Handler: h, File: p, Line: line}}}
}

// nextPageSignals: a page renders React — JSX, or a react or next import.
func nextPageSignals(src []byte) bool {
	for _, s := range [][]byte{[]byte("/>"), []byte("</"), []byte("'react'"), []byte(`"react"`), []byte("'next/"), []byte(`"next/`), []byte("'next'"), []byte(`"next"`)} {
		if bytes.Contains(src, s) {
			return true
		}
	}
	return false
}

// nextDefaultExport finds the default export and names it.
func nextDefaultExport(f *jsFile) (string, int, bool) {
	for i, t := range f.toks {
		if !t.is(tkIdent, "export") {
			continue
		}
		if f.tok(i+1).is(tkIdent, "default") {
			j := i + 2
			if f.tok(j).is(tkIdent, "async") {
				j++
			}
			switch {
			case f.tok(j).is(tkIdent, "function") || f.tok(j).is(tkIdent, "class"):
				k := j + 1
				if f.tok(k).punct("*") {
					k++
				}
				if n := f.tok(k); n.kind == tkIdent && n.val != "extends" {
					return n.val, t.line, true
				}
				return "", t.line, true
			default:
				end := f.exprEnd(j)
				return jsHandlerName(f, j, end), t.line, true
			}
		}
		// export { Page as default }
		if f.tok(i + 1).punct("{") {
			for _, a := range f.args(i + 1) {
				if a[1]-a[0] == 3 && f.toks[a[0]+1].is(tkIdent, "as") && f.toks[a[0]+2].is(tkIdent, "default") {
					return f.toks[a[0]].val, t.line, true
				}
			}
		}
	}
	return "", 0, false
}

type nextExport struct {
	exported, local string
	line            int
}

// nextNamedExports lists `export [async] function X`, `export const X =`
// and `export { a as X, Y }`.
func nextNamedExports(f *jsFile) []nextExport {
	var out []nextExport
	for i, t := range f.toks {
		if !t.is(tkIdent, "export") {
			continue
		}
		j := i + 1
		if f.tok(j).is(tkIdent, "async") {
			j++
		}
		switch {
		case f.tok(j).is(tkIdent, "function") && f.tok(j+1).kind == tkIdent:
			out = append(out, nextExport{f.toks[j+1].val, f.toks[j+1].val, t.line})
		case (f.tok(j).is(tkIdent, "const") || f.tok(j).is(tkIdent, "let") || f.tok(j).is(tkIdent, "var")) && f.tok(j+1).kind == tkIdent:
			out = append(out, nextExport{f.toks[j+1].val, f.toks[j+1].val, t.line})
		case f.tok(j).punct("{"):
			for _, a := range f.args(j) {
				switch {
				case a[1]-a[0] == 1 && f.toks[a[0]].kind == tkIdent:
					out = append(out, nextExport{f.toks[a[0]].val, f.toks[a[0]].val, t.line})
				case a[1]-a[0] == 3 && f.toks[a[0]+1].is(tkIdent, "as"):
					out = append(out, nextExport{f.toks[a[0]+2].val, f.toks[a[0]].val, t.line})
				}
			}
		}
	}
	return out
}
