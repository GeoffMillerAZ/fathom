package main

// reviewsince_test.go — CV-13 (ADR-044 §1.6): `fathom review --since`
// fills review.Input.Root (always, from ServiceMeta) and .Base (the
// resolved ref, via the injected gitRunner — never a bare `git` shell,
// so this is testable without a real repo).

import (
	"bytes"
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// rewriteGitRunner answers "rev-parse <ref>" with sha and anything else
// (e.g. "diff --name-only") with paths — enough to exercise both call
// shapes buildReviewInputFromClient and its caller make.
func rewriteGitRunner(paths []string, sha string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return []byte(sha + "\n"), nil
		}
		return []byte(joinLines(paths)), nil
	}
}

func joinLines(paths []string) string {
	out := ""
	for _, p := range paths {
		out += p + "\n"
	}
	return out
}

func TestBuildReviewInputFromClient_SinceResolvesBaseAndRoot(t *testing.T) {
	storePath, root := setupReviewModeFixture(t, "review-since-svc")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	in, _, err := buildReviewInputFromClient(context.Background(), c, "review-since-svc",
		[]string{"pkg/a.go"}, rewriteGitRunner([]string{"pkg/a.go"}, "deadbeef"), "HEAD~1")
	if err != nil {
		t.Fatalf("buildReviewInputFromClient: %v", err)
	}
	if in.Root != root {
		t.Errorf("Root = %q, want %q", in.Root, root)
	}
	if in.Base != "deadbeef" {
		t.Errorf("Base = %q, want deadbeef", in.Base)
	}
}

func TestBuildReviewInputFromClient_NoSinceLeavesBaseEmpty(t *testing.T) {
	storePath, root := setupReviewModeFixture(t, "review-nosince-svc")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	in, _, err := buildReviewInputFromClient(context.Background(), c, "review-nosince-svc",
		[]string{"pkg/a.go"}, rewriteGitRunner([]string{"pkg/a.go"}, "deadbeef"), "")
	if err != nil {
		t.Fatalf("buildReviewInputFromClient: %v", err)
	}
	if in.Root != root {
		t.Errorf("Root = %q, want %q", in.Root, root)
	}
	if in.Base != "" {
		t.Errorf("Base = %q, want empty (no --since given)", in.Base)
	}
}

func TestBuildReviewInputFromClient_UnresolvableSinceFailsOpen(t *testing.T) {
	storePath, _ := setupReviewModeFixture(t, "review-badsince-svc")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	failingRunner := func(args ...string) ([]byte, error) {
		return nil, bytes.ErrTooLarge // any error
	}
	in, _, err := buildReviewInputFromClient(context.Background(), c, "review-badsince-svc",
		[]string{"pkg/a.go"}, failingRunner, "not-a-real-ref")
	if err != nil {
		t.Fatalf("buildReviewInputFromClient must not error on an unresolvable --since: %v", err)
	}
	if in.Base != "" {
		t.Errorf("Base = %q, want empty (fail open)", in.Base)
	}
}
