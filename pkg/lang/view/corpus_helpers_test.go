package view_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// The LP-8 readings (HLP-1, HLP-2, HLP-3) share one corpus loader. Each reading is opt-in and skips unless
// FATHOM_LP8_CORPUS names the manifest.

// corpusSubjects reads the manifest named by FATHOM_LP8_CORPUS and returns its subjects sorted by dialect and
// subject. It skips the test when the variable is unset.
func corpusSubjects(t *testing.T) []corpusSubject {
	t.Helper()
	mp := os.Getenv("FATHOM_LP8_CORPUS")
	if mp == "" {
		t.Skip("FATHOM_LP8_CORPUS not set")
	}
	raw, err := os.ReadFile(mp)
	if err != nil {
		t.Fatal(err)
	}
	var subjects []corpusSubject
	if err := json.Unmarshal(raw, &subjects); err != nil {
		t.Fatal(err)
	}
	sort.Slice(subjects, func(i, j int) bool {
		if subjects[i].Dialect != subjects[j].Dialect {
			return subjects[i].Dialect < subjects[j].Dialect
		}
		return subjects[i].Subject < subjects[j].Subject
	})
	return subjects
}

// corpusFiles checks the subject's checkout against its pin (HEAD equals the commit, the tree is clean) and its
// selection (the frozen count and digest), and returns the selected paths, sorted.
func corpusFiles(t *testing.T, s corpusSubject) []string {
	t.Helper()
	if head := strings.TrimSpace(gitOut(t, s.Path, "rev-parse", "HEAD")); head != s.Commit {
		t.Fatalf("%s: HEAD %s is not the pin %s", s.Subject, head, s.Commit)
	}
	if st := gitOut(t, s.Path, "status", "--porcelain", "--untracked-files=all"); st != "" {
		t.Fatalf("%s: checkout is not clean", s.Subject)
	}
	return selectFiles(t, s)
}

// forEachMeasuredFile reads every selected file of s with rd and calls fn for each one whose read has at least
// one symbol: the measured files. It returns the number of selected files.
func forEachMeasuredFile(t *testing.T, rd *lang.Reader, s corpusSubject, fn func(rel string, src []byte, res lang.Result)) int {
	t.Helper()
	files := corpusFiles(t, s)
	for _, rel := range files {
		src, err := os.ReadFile(filepath.Join(s.Path, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		res, err := rd.Extract(context.Background(), lang.File{Path: rel, Src: src})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Symbols) == 0 {
			continue
		}
		fn(rel, src, res)
	}
	return len(files)
}
