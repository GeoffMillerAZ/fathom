//go:build go1.27

package fusion_test

import (
	"os"
	"path/filepath"
	"testing"
)

// A Go 1.27 generic method must type-check in the typed pass, and its call
// must resolve. The module is written at test time: a 1.26 gofmt (the
// pre-commit check) cannot parse the syntax.
func TestTypedPassLoadsGenericMethod(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":  "module example.com/genericmethod\n\ngo 1.27\n",
		"main.go": genericMethodSrc,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rep, typed := ingestTypedDir(t, "genericmethod", dir)
	if rep.TypedPackagesFailed != 0 || rep.TypedFailed {
		t.Fatalf("typed pass: %d package(s) failed, %s", rep.TypedPackagesFailed, rep.TypedFailReason)
	}
	if len(typed) == 0 {
		t.Error("no typed edges from the generic-method fixture")
	}
}

const genericMethodSrc = `package main

type Set struct{ items []int }

func (s *Set) Map[T any](f func(int) T) []T {
	out := make([]T, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, f(it))
	}
	return out
}

func double(i int) int { return i * 2 }

func main() {
	s := &Set{items: []int{1, 2, 3}}
	_ = s.Map(double)
}
`
