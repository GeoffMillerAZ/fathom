package main

// docsphrases_test.go — RP-20 (docs/plans/reach-parity.md): two sentences
// went stale in the use-case docs and stayed stale for three releases —
// "not yet listed by `fathom help`" (every verb renders from the dispatch
// table since 0.6.0) and "emits nothing" (the session hook prints an
// explicit not-indexed line, the O2 fix). A doc that says either is wrong
// about the binary, so the phrases are forbidden under docs/.
//
// Scope: every Markdown file under docs/ EXCEPT docs/plans/ (a plan row
// quotes the phrase it forbids — RP-20 itself) and docs/changelog/ (the
// historical narrative, never normative). The custodian's
// `forbidden_pattern` kind is the repo's own instrument for CLAUDE.md /
// AGENTS.md; this test is the same rule over the docs tree.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var staleHelpPhrases = []string{
	"not yet listed by `fathom help`",
	"absent from `fathom help`",
	"not listed in `fathom help`",
	"emits nothing",
}

func TestNoStaleHelpPhrasesInDocs(t *testing.T) {
	root := filepath.Join("..", "..", "docs")
	var hits []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if rel == "plans" || rel == "changelog" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(raw), "\n") {
			for _, p := range staleHelpPhrases {
				if strings.Contains(line, p) {
					hits = append(hits, "docs/"+filepath.ToSlash(rel)+":"+strconv.Itoa(i+1)+": "+p)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) > 0 {
		t.Errorf("stale phrases under docs/ (every verb is listed by `fathom help`; the session hook prints a not-indexed line):\n  %s", strings.Join(hits, "\n  "))
	}
}
