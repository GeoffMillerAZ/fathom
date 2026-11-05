package cue_test

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/cue"
)

const cueSrc = "package app\n\nname: \"web\"\nport: 8080\n"

func newReader(t *testing.T, opts ...lang.Option) *lang.Reader {
	t.Helper()
	r, err := lang.New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func extract(t *testing.T, r *lang.Reader, path, src string, siblings ...string) lang.Result {
	t.Helper()
	res, err := r.Extract(context.Background(), lang.File{Path: path, Src: []byte(src), Siblings: siblings})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestOptionLinksCUEAndTimoni(t *testing.T) {
	r := newReader(t, cue.Option())
	res := extract(t, r, "app/app.cue", cueSrc, "app/app.cue")
	if res.Dialect != "cue" || res.Skipped != "" || len(res.Symbols) == 0 {
		t.Fatalf("cue: dialect %q skipped %q symbols %d", res.Dialect, res.Skipped, len(res.Symbols))
	}
	tim := extract(t, r, "mod/timoni.cue", "package main\n\nvalues: {}\n", "mod/timoni.cue", "mod/values.cue")
	if tim.Dialect != "timoni" || tim.Skipped != "" {
		t.Fatalf("timoni: dialect %q skipped %q", tim.Dialect, tim.Skipped)
	}
	ids := map[string]bool{}
	for _, d := range r.Dialects() {
		ids[d.ID] = true
	}
	if !ids["cue"] || !ids["timoni"] {
		t.Errorf("a linked Reader lists cue %v, timoni %v", ids["cue"], ids["timoni"])
	}
}

func TestWithoutOptionCUEIsNotLinked(t *testing.T) {
	// This test binary imports the package, so the family is registered, but
	// a Reader built without the option does not link it.
	r := newReader(t)
	for _, p := range []string{"app/app.cue", "mod/timoni.cue"} {
		if res := extract(t, r, p, cueSrc); res.Skipped != lang.SkipNotLinked {
			t.Errorf("%s: skipped %q, want %q", p, res.Skipped, lang.SkipNotLinked)
		}
	}
	for _, d := range r.Dialects() {
		if d.ID == "cue" || d.ID == "timoni" {
			t.Errorf("Dialects lists %s, which this Reader cannot read", d.ID)
		}
	}
}

func TestLinkTwiceIsHarmless(t *testing.T) {
	r := newReader(t, cue.Option(), cue.Option())
	if res := extract(t, r, "app/app.cue", cueSrc); res.Dialect != "cue" {
		t.Fatalf("dialect %q", res.Dialect)
	}
}

func TestOptionIsDeterministic(t *testing.T) {
	a := extract(t, newReader(t, cue.Option()), "app/app.cue", cueSrc)
	b := extract(t, newReader(t, cue.Option()), "app/app.cue", cueSrc)
	if strings.Join(symbolNames(a), ",") != strings.Join(symbolNames(b), ",") {
		t.Fatalf("%v vs %v", symbolNames(a), symbolNames(b))
	}
}

func symbolNames(r lang.Result) []string {
	var out []string
	for _, s := range r.Symbols {
		out = append(out, s.Key)
	}
	return out
}
