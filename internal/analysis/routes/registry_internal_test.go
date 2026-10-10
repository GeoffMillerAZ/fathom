package routes

import (
	"reflect"
	"testing"
)

// swapReaders gives a test its own copy of the registry and restores the
// process's on cleanup, so a fake reader never outlives its test.
func swapReaders(t *testing.T) {
	t.Helper()
	readersMu.Lock()
	saved := readers
	readers = append([]Reader(nil), saved...)
	readersMu.Unlock()
	t.Cleanup(func() {
		readersMu.Lock()
		readers = saved
		readersMu.Unlock()
	})
}

func fixedReader(framework string, langs []string, res Result) Reader {
	return Reader{Framework: framework, Langs: langs, Read: func(string, []byte) Result { return res }}
}

func TestRegister_DuplicateFrameworkPanics(t *testing.T) {
	swapReaders(t)
	Register(fixedReader("zz-dup", []string{"zz"}, Result{}))
	defer func() {
		if recover() == nil {
			t.Fatal("a second Register of framework zz-dup did not panic")
		}
	}()
	Register(fixedReader("zz-dup", []string{"zz"}, Result{}))
}

func TestRegister_RefusesIncompleteReader(t *testing.T) {
	swapReaders(t)
	for name, r := range map[string]Reader{
		"no framework": {Langs: []string{"zz"}, Read: func(string, []byte) Result { return Result{} }},
		"no langs":     {Framework: "zz-nolang", Read: func(string, []byte) Result { return Result{} }},
		"no read":      {Framework: "zz-noread", Langs: []string{"zz"}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: Register did not panic", name)
				}
			}()
			Register(r)
		}()
	}
}

// Readers run in Framework order whatever order init registered them in,
// and their routes are merged, sorted and deduplicated on (Method, Path,
// Handler, Line).
func TestExtractFile_MergesReadersInFrameworkOrder(t *testing.T) {
	swapReaders(t)
	var order []string
	mk := func(fw string, routes ...Route) Reader {
		return Reader{Framework: fw, Langs: []string{"zz"}, Read: func(path string, _ []byte) Result {
			order = append(order, fw)
			return Result{Routes: routes}
		}}
	}
	Register(mk("zz-b", Route{Method: "GET", Path: "/b", Handler: "B.b", File: "f.zz", Line: 2}, Route{Method: "GET", Path: "/a", Handler: "A.a", File: "f.zz", Line: 1}))
	Register(mk("zz-a", Route{Method: "GET", Path: "/a", Handler: "A.a", File: "f.zz", Line: 1}))
	got := ExtractFile("zz", "f.zz", nil)
	if !reflect.DeepEqual(order, []string{"zz-a", "zz-b"}) {
		t.Fatalf("readers ran in %v, want Framework order [zz-a zz-b]", order)
	}
	want := []Route{
		{Method: "GET", Path: "/a", Handler: "A.a", File: "f.zz", Line: 1},
		{Method: "GET", Path: "/b", Handler: "B.b", File: "f.zz", Line: 2},
	}
	if !reflect.DeepEqual(got.Routes, want) {
		t.Fatalf("routes = %+v, want %+v (sorted, the duplicate dropped)", got.Routes, want)
	}
	if ex := Extract("zz", "f.zz", nil); !reflect.DeepEqual(ex, want) {
		t.Fatalf("Extract = %+v, want ExtractFile's routes", ex)
	}
}

// A reader's mounts come back sorted, and a route or mount with no File
// takes the path being read.
func TestExtractFile_CarriesMounts(t *testing.T) {
	swapReaders(t)
	Register(fixedReader("zz-m", []string{"zz"}, Result{
		Routes: []Route{{Method: "GET", Path: "/x", Handler: "h", Line: 3}},
		Mounts: []Mount{{Prefix: "/v2", Target: "./b", Line: 9}, {Prefix: "/v1", Target: "./a", Line: 4}},
	}))
	got := ExtractFile("zz", "app/main.zz", nil)
	wantM := []Mount{{Prefix: "/v1", Target: "./a", File: "app/main.zz", Line: 4}, {Prefix: "/v2", Target: "./b", File: "app/main.zz", Line: 9}}
	if !reflect.DeepEqual(got.Mounts, wantM) {
		t.Fatalf("mounts = %+v, want %+v", got.Mounts, wantM)
	}
	if len(got.Routes) != 1 || got.Routes[0].File != "app/main.zz" {
		t.Fatalf("routes = %+v, want one route filed under app/main.zz", got.Routes)
	}
	if ex := Extract("zz", "app/main.zz", nil); len(ex) != 1 {
		t.Fatalf("Extract = %+v, want the routes alone", ex)
	}
}

// HasReader answers for the Go AST reader, every registered reader's
// languages, and the long spellings Extract always accepted.
func TestHasReader(t *testing.T) {
	for lang, want := range map[string]bool{
		"go": true, "js": true, "ts": true, "py": true,
		"javascript": true, "typescript": true, "python": true,
		"cobol": false, "": false, "yaml": false,
	} {
		if got := HasReader(lang); got != want {
			t.Errorf("HasReader(%q) = %v, want %v", lang, got, want)
		}
	}
}

// The legacy regex pattern sets are registered readers, under their
// framework names, for exactly the languages they read before the seam.
func TestLegacyReadersRegistered(t *testing.T) {
	want := map[string][]string{"express-regex": {"js", "ts"}, "python-regex": {"py"}}
	readersMu.RLock()
	defer readersMu.RUnlock()
	for fw, langs := range want {
		found := false
		for _, r := range readers {
			if r.Framework == fw {
				found = true
				if !reflect.DeepEqual(r.Langs, langs) {
					t.Errorf("%s langs = %v, want %v", fw, r.Langs, langs)
				}
			}
		}
		if !found {
			t.Errorf("reader %s is not registered", fw)
		}
	}
}

// The legacy readers called directly read what Extract read before the
// seam (reachable_from for the moved pattern sets).
func TestLegacyReaders_ReadDirectly(t *testing.T) {
	js := readExpressRegex("app.js", []byte("app.get('/users', listUsers)\n"))
	if len(js.Routes) != 1 || js.Routes[0].Path != "/users" || js.Routes[0].Handler != "listUsers" {
		t.Fatalf("express-regex = %+v", js.Routes)
	}
	py := readPythonRegex("app.py", []byte("@app.route('/items', methods=['POST', 'GET'])\ndef items(): pass\n"))
	if len(py.Routes) != 1 || py.Routes[0].Method != "GET,POST" {
		t.Fatalf("python-regex = %+v", py.Routes)
	}
}
