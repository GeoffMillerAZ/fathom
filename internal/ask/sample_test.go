package ask

import (
	"fmt"
	"reflect"
	"testing"
)

func poolOf(repo string, n int) []Computed {
	out := make([]Computed, n)
	for i := range out {
		out[i] = Computed{Span: Span{Repo: repo, File: fmt.Sprintf("f%02d.go", i), Name: "F", Start: 1}}
	}
	return out
}

func keys(cs []Computed) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Span.Repo + "/" + c.Span.File
	}
	return out
}

// TestSampleIsSeededDeterministicAndRefillsShortfalls pins the sampler:
// the same pool and seed draw the same spans in the same order whatever
// the pool's input order, a repository short of its quota is refilled
// from the others in quota order, and no span is drawn twice.
func TestSampleIsSeededDeterministicAndRefillsShortfalls(t *testing.T) {
	tmpl, _ := TemplateByID("errcheck")
	pool := append(poolOf("a", 20), poolOf("b", 1)...)
	quotas := []Quota{{Repo: "a", N: 5}, {Repo: "b", N: 3}}
	got := Sample(tmpl, pool, quotas, SampleSeed)
	if len(got) != 8 {
		t.Fatalf("drew %d, want 8 (b's shortfall of 2 refilled from a)", len(got))
	}
	seen := map[string]bool{}
	byRepo := map[string]int{}
	for _, k := range keys(got) {
		if seen[k] {
			t.Fatalf("%s drawn twice", k)
		}
		seen[k] = true
		byRepo[k[:1]]++
	}
	if byRepo["a"] != 7 || byRepo["b"] != 1 {
		t.Errorf("per repo %v, want a:7 b:1", byRepo)
	}
	reversed := make([]Computed, len(pool))
	for i, c := range pool {
		reversed[len(pool)-1-i] = c
	}
	if again := Sample(tmpl, reversed, quotas, SampleSeed); !reflect.DeepEqual(keys(again), keys(got)) {
		t.Errorf("input order changed the draw:\n%v\n%v", keys(got), keys(again))
	}
	if other := Sample(tmpl, pool, quotas, SampleSeed+1); reflect.DeepEqual(keys(other), keys(got)) {
		t.Error("a different seed drew the identical sample")
	}
	lock, _ := TemplateByID("lock")
	if other := Sample(lock, pool, quotas, SampleSeed); reflect.DeepEqual(keys(other), keys(got)) {
		t.Error("each template must draw from its own stream")
	}
	// The generator is written out: its first outputs are fixed forever.
	g := splitmix64(0)
	if v := g.next(); v != 0xe220a8397b1dcdaf {
		t.Errorf("splitmix64(0).next() = %#x, want 0xe220a8397b1dcdaf", v)
	}
}
