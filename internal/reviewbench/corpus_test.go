package reviewbench

import (
	"os"
	"path/filepath"
	"testing"
)

const fixtureCorpus = `{
  "pairs": [
    {
      "repo": "example/repo",
      "language": "go",
      "pr_a": {
        "number": 100,
        "merge_commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "base_commit": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "merged_at": "2020-01-01T00:00:00Z",
        "files_changed": ["a.go"]
      },
      "pr_b": {
        "number": 101,
        "merge_commit": "cccccccccccccccccccccccccccccccccccccccc",
        "base_commit": "dddddddddddddddddddddddddddddddddddddddd",
        "merged_at": "2020-02-01T00:00:00Z",
        "files_changed": [
          {"filename": "a.go", "patch": "@@ -1,1 +1,1 @@ func Foo() {\n"}
        ]
      }
    }
  ]
}`

func TestLoadCorpus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.json")
	if err := os.WriteFile(path, []byte(fixtureCorpus), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCorpus(path)
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if len(c.Pairs) != 1 {
		t.Fatalf("got %d pairs, want 1", len(c.Pairs))
	}
	p := c.Pairs[0]
	if p.Repo != "example/repo" || p.PRA.Number != 100 || p.PRB.Number != 101 {
		t.Errorf("unexpected pair: %+v", p)
	}
	if got, want := p.Key(), "example-repo-100-101"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
	a, b := p.MergedAtTimes()
	if !a.Before(b) {
		t.Errorf("MergedAtTimes: pr_a (%v) should be before pr_b (%v)", a, b)
	}
}

func TestLoadCorpus_MissingFile(t *testing.T) {
	if _, err := LoadCorpus("/nonexistent/path/corpus.json"); err == nil {
		t.Fatal("expected an error for a missing corpus file")
	}
}
