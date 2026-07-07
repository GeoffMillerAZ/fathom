package connector_test

// github_test.go — tests for GitHubConnector.
// All network I/O is replaced by a fake runner func so tests are hermetic.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// fakeRunner builds a runner func that returns predefined responses keyed by
// the first argument token that uniquely identifies each call pattern.
// Calls not matching any key return an error.
func fakeRunner(responses map[string][]byte) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		// The key is the sub-sub-command or URL fragment unique to each call.
		// We join all args and scan for known substrings.
		joined := strings.Join(args, " ")
		for k, v := range responses {
			if strings.Contains(joined, k) {
				return v, nil
			}
		}
		return nil, errors.New("fakeRunner: no match for args: " + joined)
	}
}

// fakeRunnerErr always returns an error regardless of args.
func fakeRunnerErr(msg string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		return nil, errors.New(msg)
	}
}

// TestGitHubConnector_DiscoverAndFetch exercises a happy-path Discover+Fetch
// using a fake runner that returns a minimal JSON tree response.
func TestGitHubConnector_DiscoverAndFetch(t *testing.T) {
	// Minimal GitHub Trees API response listing two files.
	treeJSON := `{
		"tree": [
			{"path": "main.go",       "type": "blob", "sha": "abc"},
			{"path": "internal/foo.go", "type": "blob", "sha": "def"},
			{"path": "internal",      "type": "tree", "sha": "xyz"}
		]
	}`
	fooContent := []byte("package main\n")
	internalContent := []byte("package internal\n")

	runner := fakeRunner(map[string][]byte{
		"git/trees": []byte(treeJSON),
		"main.go":   fooContent,
		"foo.go":    internalContent,
	})

	gc := connector.NewGitHubConnector("org/myrepo", runner)
	svc := core.ServiceRef{ID: "myrepo", Repo: "org/myrepo"}

	ctx := context.Background()

	refs, err := gc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: unexpected error: %v", err)
	}

	if len(refs) == 0 {
		t.Fatal("Discover returned no SourceRefs")
	}

	// Only blobs should be returned; the tree entry should be filtered.
	for _, r := range refs {
		if r.Service != svc.ID {
			t.Errorf("SourceRef.Service = %q, want %q", r.Service, svc.ID)
		}
	}

	// Paths must be sorted deterministically.
	for i := 1; i < len(refs); i++ {
		if refs[i].Path < refs[i-1].Path {
			t.Errorf("paths not sorted: [%d]=%q < [%d]=%q",
				i, refs[i].Path, i-1, refs[i-1].Path)
		}
	}

	// The connector must not return the tree-type entry.
	for _, r := range refs {
		if r.Path == "internal" {
			t.Errorf("tree-type entry %q should not appear in Discover results", r.Path)
		}
	}

	// Fetch each discovered ref — the fake runner should handle them.
	for _, r := range refs {
		data, fetchErr := gc.Fetch(ctx, r)
		if fetchErr != nil {
			t.Errorf("Fetch(%q): unexpected error: %v", r.Path, fetchErr)
			continue
		}
		if len(data) == 0 {
			t.Errorf("Fetch(%q): returned empty bytes", r.Path)
		}
	}
}

// TestGitHubConnector_DiscoverError verifies that a runner error from the tree
// call is propagated as a Discover error.
func TestGitHubConnector_DiscoverError(t *testing.T) {
	gc := connector.NewGitHubConnector("org/myrepo", fakeRunnerErr("tree call failed"))
	svc := core.ServiceRef{ID: "myrepo", Repo: "org/myrepo"}

	_, err := gc.Discover(context.Background(), svc)
	if err == nil {
		t.Fatal("expected error from Discover, got nil")
	}
}

// TestGitHubConnector_FetchError verifies that a runner error from a content
// call is propagated as a Fetch error.
func TestGitHubConnector_FetchError(t *testing.T) {
	gc := connector.NewGitHubConnector("org/myrepo", fakeRunnerErr("fetch failed"))
	ref := core.SourceRef{Service: "myrepo", Path: "main.go"}

	_, err := gc.Fetch(context.Background(), ref)
	if err == nil {
		t.Fatal("expected error from Fetch, got nil")
	}
}

// TestGitHubConnector_KindAssignment checks that .go files get KindGoFunc.
func TestGitHubConnector_KindAssignment(t *testing.T) {
	treeJSON := `{"tree":[{"path":"handler.go","type":"blob","sha":"aaa"}]}`
	runner := fakeRunner(map[string][]byte{
		"git/trees":  []byte(treeJSON),
		"handler.go": []byte("package main\n"),
	})

	gc := connector.NewGitHubConnector("org/repo", runner)
	svc := core.ServiceRef{ID: "repo", Repo: "org/repo"}

	refs, err := gc.Discover(context.Background(), svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("no refs returned")
	}
	if refs[0].Kind != core.KindGoFunc {
		t.Errorf("Kind = %q, want KindGoFunc", refs[0].Kind)
	}
}

// TestGitHubConnector_ImplementsConnector is a compile-time interface check.
func TestGitHubConnector_ImplementsConnector(t *testing.T) {
	var _ connector.Connector = (*connector.GitHubConnector)(nil)
}
