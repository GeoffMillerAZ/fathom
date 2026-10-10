package routes

// legacy_express.go — the Express (JS/TS) line-regex pattern set, moved
// unchanged out of routes.go into a registered reader at the route seam
// (LB-F321, wave c2). The shared regex machinery (routePattern,
// extractRegex) stays in routes.go.

import "regexp"

func init() {
	Register(Reader{Framework: "express-regex", Langs: []string{"js", "ts"}, Read: readExpressRegex})
}

// expressRegexPatterns is compiled once; a compiled regexp is safe for
// concurrent use.
var expressRegexPatterns = expressPatterns()

// readExpressRegex is the express-regex reader.
func readExpressRegex(path string, src []byte) Result {
	return Result{Routes: extractRegex(expressRegexPatterns, path, src)}
}

// -----------------------------------------------------------------
// Express (JS/TS) patterns
// -----------------------------------------------------------------

func expressPatterns() []*routePattern {
	// app.get('/path', handler) or router.post('/path', handler) etc.
	// Supports single or double quoted paths.
	appMethod := regexp.MustCompile(
		`(?m)[\w.]+\.(?P<method>get|post|put|delete|patch|head|options|all|use)\(\s*` +
			`['"](?P<path>[^'"]+)['"]` +
			`(?:\s*,\s*(?P<handler>[A-Za-z_$][\w.$]*))?`,
	)
	return []*routePattern{
		{re: appMethod, pathGrp: "path", hndGrp: "handler"},
	}
}
