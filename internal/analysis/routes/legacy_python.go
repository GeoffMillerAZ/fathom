package routes

// legacy_python.go — the Flask/FastAPI line-regex pattern set, moved
// unchanged out of routes.go into a registered reader at the route seam
// (LB-F321, wave c2). The shared regex machinery (routePattern,
// extractRegex) stays in routes.go.
//
// Wave c2 item phppy: the Flask and FastAPI readers (flask.go, fastapi.go)
// replace it where they read: on a file either of them reads a route or a
// mount from, the regex reader reads nothing (it would double each route
// they bind to its handler with an unbound one at the decorator's local
// path, and spell a multi-method rule as one "GET,POST" row). It stays the
// fallback for a Python file they do not read (a Quart or Sanic
// application's @app.route). Before (python-regex alone) and after, on
// ctfd 3.8.8 and full-stack-fastapi-template 0.12.0:
// docs/bench/readings/routes/python.md.

import (
	"regexp"
	"sort"
	"strings"
)

func init() {
	Register(Reader{Framework: "python-regex", Langs: []string{"py"}, Read: readPythonRegex})
}

// pythonRegexPatterns is compiled once; a compiled regexp is safe for
// concurrent use.
var pythonRegexPatterns = pythonPatterns()

// readPythonRegex is the python-regex reader.
func readPythonRegex(path string, src []byte) Result {
	rs := extractRegex(pythonRegexPatterns, path, src)
	if len(rs) == 0 {
		return Result{}
	}
	for _, read := range []func(string, []byte) Result{readFlask, readFastAPI} {
		if x := read(path, src); len(x.Routes) > 0 || len(x.Mounts) > 0 {
			return Result{}
		}
	}
	return Result{Routes: rs}
}

// -----------------------------------------------------------------
// Python patterns: Flask, FastAPI decorators
// -----------------------------------------------------------------

func pythonPatterns() []*routePattern {
	// @app.route('/path', methods=['GET', 'POST'])
	// @app.route('/path') — defaults to GET
	flaskRoute := regexp.MustCompile(
		`(?m)@[\w.]+\.route\(\s*['"](?P<path>[^'"]+)['"]` +
			`(?:[^)]*methods\s*=\s*\[(?P<methods>[^\]]+)\])?`,
	)
	// @app.get('/path') @app.post('/path') etc. (FastAPI / Flask 2.x)
	fastAPIMethod := regexp.MustCompile(
		`(?m)@[\w.]+\.(?P<method>get|post|put|delete|patch|head|options)\(\s*` +
			`['"](?P<path>[^'"]+)['"]`,
	)
	return []*routePattern{
		{re: flaskRoute, method: "", pathGrp: "path", hndGrp: "", methodsFromMatch: flaskMethods},
		{re: fastAPIMethod, pathGrp: "path", hndGrp: ""},
	}
}

// flaskMethods extracts methods from a flask @app.route match string.
// Returns comma-joined uppercased methods, or "GET" if absent.
func flaskMethods(matchText string) string {
	methodsRe := regexp.MustCompile(`methods\s*=\s*\[([^\]]+)\]`)
	m := methodsRe.FindStringSubmatch(matchText)
	if m == nil {
		return "GET"
	}
	raw := m[1]
	var methods []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.Trim(strings.TrimSpace(part), `'"`)
		if part != "" {
			methods = append(methods, strings.ToUpper(part))
		}
	}
	sort.Strings(methods)
	return strings.Join(methods, ",")
}
