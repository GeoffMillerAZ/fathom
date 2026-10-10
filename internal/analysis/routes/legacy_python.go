package routes

// legacy_python.go — the Flask/FastAPI line-regex pattern set, moved
// unchanged out of routes.go into a registered reader at the route seam
// (LB-F321, wave c2). The shared regex machinery (routePattern,
// extractRegex) stays in routes.go.

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
	return Result{Routes: extractRegex(pythonRegexPatterns, path, src)}
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
