// Package langtest is the conformance check for a dialect pack (ADR-092,
// design §6). A pack author calls CheckPack from a test:
//
//	func TestPack(t *testing.T) {
//		if err := langtest.CheckPack(os.DirFS("pack")); err != nil {
//			t.Fatal(err)
//		}
//	}
//
// It runs what lang.New runs (the manifest rules, the cross-dialect rules
// against the built-in dialects, query compilation), then holds the pack to
// its own goldens and to its required kinds.
//
// A golden is dialects/<id>/testdata/<name>.golden.json beside the file
// dialects/<id>/testdata/<name> it describes. It holds the lang.Result of
// reading that file, with the path relative to the testdata directory, as
// Golden prints it. fathom-lang extract prints the same Result.
package langtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// manifest is the part of dialect.json CheckPack reads.
type manifest struct {
	ID    string `json:"id"`
	Kinds map[string]struct {
		SourceKind string `json:"source_kind"`
		Canonical  string `json:"canonical"`
	} `json:"kinds"`
	RequiredKinds []string `json:"required_kinds"`
}

// Golden is the bytes a golden file holds for res.
func Golden(res lang.Result) ([]byte, error) {
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// CheckPack holds the pack at fsys to the rules of lang.WithDialects, then
// to its goldens and its required kinds. Every dialect needs at least one
// golden, and every golden must read as it says: the file is read by its own
// dialect and the Result equals the golden. Every required kind of a dialect
// must appear among its goldens' symbols, so a kind the manifest promises is
// a kind a fixture shows. The error lists every failure, or is nil. A pack
// refused by lang.New returns that *lang.PackError alone.
func CheckPack(fsys fs.FS) error {
	r, err := lang.New(lang.WithDialects(fsys))
	if err != nil {
		return err
	}
	defer r.Close()
	manifests, err := fs.Glob(fsys, "dialects/*/dialect.json")
	if err != nil {
		return err
	}
	sort.Strings(manifests)
	var failures []error
	for _, mp := range manifests {
		b, err := fs.ReadFile(fsys, mp)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		var m manifest
		if err := json.Unmarshal(b, &m); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", mp, err))
			continue
		}
		failures = append(failures, checkDialect(r, fsys, m)...)
	}
	return errors.Join(failures...)
}

// checkDialect runs one dialect's goldens and required kinds.
func checkDialect(r *lang.Reader, fsys fs.FS, m manifest) []error {
	dir := path.Join("dialects", m.ID, "testdata")
	var goldens []string
	_ = fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".golden.json") {
			goldens = append(goldens, p)
		}
		return nil
	})
	sort.Strings(goldens)
	if len(goldens) == 0 {
		return []error{fmt.Errorf("dialect %s: no *.golden.json under %s", m.ID, dir)}
	}
	var failures []error
	seenKind := map[string]bool{}
	seenCanonical := map[string]bool{}
	for _, gp := range goldens {
		input := strings.TrimSuffix(gp, ".golden.json")
		rel := strings.TrimPrefix(input, dir+"/")
		src, err := fs.ReadFile(fsys, input)
		if err != nil {
			failures = append(failures, fmt.Errorf("dialect %s: golden %s: input: %w", m.ID, gp, err))
			continue
		}
		res, err := r.Extract(context.Background(), lang.File{Path: rel, Src: src})
		if err != nil {
			failures = append(failures, fmt.Errorf("dialect %s: %s: %w", m.ID, rel, err))
			continue
		}
		if res.Dialect != m.ID {
			failures = append(failures, fmt.Errorf("dialect %s: %s is read by %q (skipped %q), not by the dialect", m.ID, rel, res.Dialect, res.Skipped))
			continue
		}
		for _, s := range res.Symbols {
			seenKind[s.Kind] = true
			seenCanonical[s.Canonical] = true
		}
		want, err := fs.ReadFile(fsys, gp)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if err := compareGolden(res, want); err != nil {
			failures = append(failures, fmt.Errorf("dialect %s: %s: %w", m.ID, gp, err))
		}
	}
	for _, rk := range m.RequiredKinds {
		shown := false
		for key, k := range m.Kinds {
			if (rk == key && seenKind[k.SourceKind]) || (rk == k.Canonical && seenCanonical[k.Canonical]) {
				shown = true
			}
		}
		if !shown {
			failures = append(failures, fmt.Errorf("dialect %s: required kind %q appears in no golden's symbols", m.ID, rk))
		}
	}
	return failures
}

// compareGolden is nil when res is what the golden holds. The golden is
// compared as a Result, so its whitespace does not matter.
func compareGolden(res lang.Result, want []byte) error {
	var wantRes lang.Result
	if err := json.Unmarshal(want, &wantRes); err != nil {
		return fmt.Errorf("not a lang.Result: %w", err)
	}
	gotB, err := Golden(res)
	if err != nil {
		return err
	}
	wantB, err := Golden(wantRes)
	if err != nil {
		return err
	}
	if string(gotB) == string(wantB) {
		return nil
	}
	gl, wl := strings.Split(string(gotB), "\n"), strings.Split(string(wantB), "\n")
	for i := 0; i < len(gl) || i < len(wl); i++ {
		var g, w string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g != w {
			return fmt.Errorf("differs from the Result at line %d: got %q, golden %q (langtest.Golden prints the bytes to keep)", i+1, strings.TrimSpace(g), strings.TrimSpace(w))
		}
	}
	return errors.New("differs from the Result")
}
