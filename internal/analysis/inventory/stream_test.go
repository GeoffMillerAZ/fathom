package inventory

import (
	"reflect"
	"testing"
)

func TestStreamMatchesExtractAcrossMountedRoutesAndPatterns(t *testing.T) {
	files := []File{
		{Path: "app.js", Content: []byte("const users = require('./users')\nfastify.register(users, { prefix: '/api' })\nconst a = process.env.API_KEY\n")},
		{Path: "users.js", Content: []byte("fastify.get('/users', listUsers)\nName: \"first\"\nName: \"same\"\n")},
		{Path: "other.go", Content: []byte("package other\nfunc Pub() {}\n// Name: \"same\"\n")},
		{Path: "other_test.go", Content: []byte("package other\n// Name: \"excluded\"\n")},
	}
	patterns := []Pattern{
		{Kind: "tag", Regex: `Name: "([a-z]+)"`},
		{Kind: "scoped", Glob: "users.js", Regex: `Name: "([a-z]+)"`},
	}
	want := Extract(files, patterns)
	a := NewAccumulator(patterns)
	for _, f := range files {
		a.Add(f)
	}
	if got := a.Items(); !reflect.DeepEqual(got, want) {
		t.Fatalf("stream differs from Extract\n got: %#v\nwant: %#v", got, want)
	}
	var mounted, env, pattern, scoped bool
	for _, it := range want {
		mounted = mounted || it.Key == "route:GET /api/users"
		env = env || it.Key == "env:API_KEY"
		pattern = pattern || it.Key == "tag:same" && it.File == "other.go"
		scoped = scoped || it.Key == "scoped:same" && it.File == "users.js"
	}
	if !mounted || !env || !pattern || !scoped {
		t.Fatalf("fixture did not exercise mount/env/dedup/glob: mounted=%v env=%v pattern=%v scoped=%v", mounted, env, pattern, scoped)
	}
}
