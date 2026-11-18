package dependencyboundaries

// TestDependencyBoundarySchema validates each commented JSON example.
// case: {"schema_version":2,"rules":[]}
// case: {"schema_version":1,"rules":[]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"reason":"Missing mode."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":["go:net/http"],"allow_only":["go:fmt"],"reason":"Conflicting modes."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":[],"reason":"Empty targets."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["/absolute/*.go"],"deny":["go:net/http"],"reason":"Absolute source."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["pkg/../*.go"],"deny":["go:net/http"],"reason":"Parent path."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["pkg/**bad/*.go"],"deny":["go:net/http"],"reason":"Partial globstar."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["pkg/[ab.go"],"deny":["go:net/http"],"reason":"Malformed class."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":["repo:lib"],"reason":"Reserved namespace."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":["go:/absolute"],"reason":"Absolute external operand."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":["go:net/http"],"except":[{"to":["go:net/http"]}],"reason":"Incomplete exception."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":["go:net/http"],"except":null,"reason":"Null exception."}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":["go:net/http"],"reason":" "}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":["go:net/http"],"reason":"one\ntwo"}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":["go:net/http"],"reason":"Extra field.","override":true}]}
// case: {"schema_version":1,"rules":[{"id":"all","from":["*.go"],"deny":["go:net/http"],"reason":"First."},{"id":"all","from":["*.go"],"deny":["go:fmt"],"reason":"Second."}]}
