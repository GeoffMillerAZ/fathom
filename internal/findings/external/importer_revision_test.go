package external

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeCorpus(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// revisionEvent is validEventJSON anchored at internal/foo/bar.go lines
// 2-3, observed at rev ("" omits sourceRevision).
func revisionEvent(seed, rev string) string {
	e := validEventJSON(seed)
	e = strings.Replace(e, `"start": 10, "end": 12`, `"start": 2, "end": 3`, 1)
	if rev == "" {
		return strings.Replace(e, `"sourceRevision": "abc123",`, "", 1)
	}
	return strings.Replace(e, `"sourceRevision": "abc123"`, `"sourceRevision": "`+rev+`"`, 1)
}

// TestImportAnchorsHashAtSourceRevision (WAE-1): the importer hashes a
// finding's cited lines at the revision its producer observed, through
// git, never the working tree. A repository with two commits whose cited
// lines differ, its working tree at the second (plus an uncommitted edit):
// an event at the first commit gets the first commit's lines as its cites
// hash. An unknown, empty or unreadable revision writes no cites link and
// the receipt names the skew — never a fallback to the working tree — while
// the scopes and scope-fp links are written exactly as before.
func TestImportAnchorsHashAtSourceRevision(t *testing.T) {
	root := t.TempDir()
	gitIn(t, root, "init", "-q")
	firstBody := "package foo\nfunc Bar() {\n\treturn first\n}\n"
	writeCorpus(t, root, "internal/foo/bar.go", firstBody)
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "one")
	first := gitIn(t, root, "rev-parse", "HEAD")
	secondBody := "package foo\nfunc Bar(x int) {\n\treturn second\n}\n"
	writeCorpus(t, root, "internal/foo/bar.go", secondBody)
	gitIn(t, root, "commit", "-q", "-am", "two")
	second := gitIn(t, root, "rev-parse", "HEAD")
	writeCorpus(t, root, "internal/foo/bar.go", "package foo\nfunc Bar(y string) {\n\treturn dirty\n}\n")

	ctx := context.Background()
	wantFirst, ok := core.HashLines([]byte(firstBody), 2, 3)
	if !ok {
		t.Fatal("HashLines(first) failed")
	}
	wantSecond, _ := core.HashLines([]byte(secondBody), 2, 3)

	for _, c := range []struct {
		name, rev  string
		wantHash   string
		wantReason string
	}{
		{"at the first commit", first, wantFirst, ""},
		{"at the second commit", second, wantSecond, ""},
		{"unknown revision", strings.Repeat("cd", 20), "", "unknown-revision"},
		{"no revision", "", "", "no-source-revision"},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := store.NewMemoryStore()
			im := newTestImporter(t, st)
			im.ContentAt = GitContentAt(ctx, root)
			p := parseOne(t, revisionEvent("rev-"+c.name, c.rev))

			r, err := im.Import(ctx, testService, "", p, false)
			if err != nil {
				t.Fatalf("Import: %v", err)
			}
			if r.Delivered != 1 || len(r.Rejected) != 0 {
				t.Fatalf("receipt = %+v, want the finding delivered either way", r)
			}
			claimID := ClaimFromEvent(testService, p.Events[0], core.TierPreScan).ClaimID
			links, err := st.ClaimLinks(ctx, testService, []string{claimID})
			if err != nil {
				t.Fatalf("ClaimLinks: %v", err)
			}
			rels := map[string]core.ClaimLink{}
			for _, l := range links {
				rels[l.Rel] = l
			}
			if _, ok := rels[core.RelScopes]; !ok {
				t.Errorf("no scopes link: %+v", links)
			}
			if _, ok := rels[core.RelScopeFP]; !ok {
				t.Errorf("no scope-fp link: %+v", links)
			}

			cites, hasCites := rels[core.RelCites]
			if c.wantReason == "" {
				if !hasCites || cites.TargetHash != c.wantHash {
					t.Fatalf("cites = %+v (present %v), want hash %s of the lines at %s", cites, hasCites, c.wantHash, c.rev)
				}
				if !strings.Contains(cites.AnchorText, "Bar(") || strings.Contains(cites.AnchorText, "dirty") {
					t.Errorf("anchor text = %q, want the committed lines, never the working tree", cites.AnchorText)
				}
				if len(r.CitesSkew) != 0 {
					t.Errorf("CitesSkew = %+v, want none", r.CitesSkew)
				}
				return
			}
			if hasCites {
				t.Fatalf("cites link written (%+v) for a %s, want none (never the working tree)", cites, c.wantReason)
			}
			if len(r.CitesSkew) != 1 {
				t.Fatalf("CitesSkew = %+v, want one entry naming the skew", r.CitesSkew)
			}
			sk := r.CitesSkew[0]
			if sk.Reason != c.wantReason || sk.Revision != c.rev || sk.Path != "internal/foo/bar.go" || sk.Index != 0 || sk.FindingID != p.Events[0].FindingID {
				t.Errorf("CitesSkew[0] = %+v, want reason %q at revision %q", sk, c.wantReason, c.rev)
			}
		})
	}

	// Not a repository: the corpus root is a plain directory holding the
	// very file — it is still never read.
	plain := t.TempDir()
	writeCorpus(t, plain, "internal/foo/bar.go", firstBody)
	st := store.NewMemoryStore()
	im := newTestImporter(t, st)
	im.ContentAt = GitContentAt(ctx, plain)
	r, err := im.Import(ctx, testService, "", parseOne(t, revisionEvent("rev-plain", first)), true)
	if err != nil {
		t.Fatalf("Import (dry run): %v", err)
	}
	if len(r.CitesSkew) != 1 || r.CitesSkew[0].Reason != "not-a-repository" {
		t.Errorf("CitesSkew = %+v, want not-a-repository (a dry run computes the same receipt)", r.CitesSkew)
	}
}
