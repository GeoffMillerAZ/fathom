package langtest_test

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/langtest"
)

const samplePack = "testdata/samplepack"

// copyPack is the sample pack as a mutable in-memory filesystem, goldens included.
func copyPack(t *testing.T) fstest.MapFS {
	t.Helper()
	out := fstest.MapFS{}
	root := os.DirFS(samplePack)
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(root, p)
		out[p] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCheckPackPassesTheSamplePack(t *testing.T) {
	if err := langtest.CheckPack(os.DirFS(samplePack)); err != nil {
		t.Fatal(err)
	}
}

func TestCheckPackFailures(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(fstest.MapFS)
		want   string
	}{
		{"a dialect with no golden", func(f fstest.MapFS) {
			delete(f, "dialects/thrift-lite/testdata/shop.thr.golden.json")
		}, "dialect thrift-lite: no *.golden.json"},
		{"a golden that differs", func(f fstest.MapFS) {
			g := f["dialects/thrift-lite/testdata/shop.thr.golden.json"]
			g.Data = []byte(strings.Replace(string(g.Data), `"name": "Order"`, `"name": "Orders"`, 1))
		}, "differs from the Result at line"},
		{"a golden that is not a Result", func(f fstest.MapFS) {
			f["dialects/thrift-lite/testdata/shop.thr.golden.json"].Data = []byte("[1]")
		}, "not a lang.Result"},
		{"a file its dialect does not read", func(f fstest.MapFS) {
			f["dialects/widget-spec/testdata/plain.json"] = &fstest.MapFile{Data: []byte(`{"x":1}`)}
			f["dialects/widget-spec/testdata/plain.json.golden.json"] = &fstest.MapFile{Data: []byte(`{}`)}
		}, "is read by"},
		{"a required kind no golden shows", func(f fstest.MapFS) {
			// The thrift-lite fixture has no function once its service is gone.
			f["dialects/thrift-lite/testdata/shop.thr"].Data = []byte("struct Order {\n  1: i32 id\n}\n")
			g := f["dialects/thrift-lite/testdata/shop.thr.golden.json"]
			r, err := lang.New(lang.WithDialects(f))
			if err != nil {
				panic(err)
			}
			defer r.Close()
			res, err := r.Extract(t.Context(), lang.File{Path: "shop.thr", Src: f["dialects/thrift-lite/testdata/shop.thr"].Data})
			if err != nil {
				panic(err)
			}
			if g.Data, err = langtest.Golden(res); err != nil {
				panic(err)
			}
		}, `required kind "service" appears in no golden`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := copyPack(t)
			tc.mutate(f)
			err := langtest.CheckPack(f)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("CheckPack = %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestCheckPackReturnsTheRefusal(t *testing.T) {
	f := copyPack(t)
	f["dialects/thrift-lite/tags.scm"].Data = []byte("(nope) @name\n")
	err := langtest.CheckPack(f)
	var pe *lang.PackError
	if !errors.As(err, &pe) || pe.Reason != lang.PackQuery {
		t.Fatalf("CheckPack = %v, want a %s refusal", err, lang.PackQuery)
	}
}

func TestGoldenIsStableBytes(t *testing.T) {
	r, err := lang.New(lang.WithDialects(os.DirFS(samplePack)))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	src, _ := os.ReadFile(samplePack + "/dialects/thrift-lite/testdata/shop.thr")
	res, err := r.Extract(t.Context(), lang.File{Path: "shop.thr", Src: src})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := langtest.Golden(res)
	b, _ := langtest.Golden(res)
	if string(a) != string(b) || !strings.HasSuffix(string(a), "}\n") {
		t.Fatal("Golden is not stable bytes ending in a newline")
	}
}
