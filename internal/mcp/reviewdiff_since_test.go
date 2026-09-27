package mcp

// reviewdiff_since_test.go — CV-13 (ADR-044 §1.6): review_diff's optional
// "since" argument resolves a base ref in the service's own root and fills
// review.Input.Base/Root — the wiring DimIntegrity (a later row) will read.
// White-box (package mcp, not mcp_test) because reviewDiffWithBase is
// unexported, mirroring this package's existing selfskip.go test pattern.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
)

func reviewDiffSinceGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out.String())
		}
	}
	run("init", "-q")
	run("config", "commit.gpgsign", "false")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "initial")
	return root
}

func TestReviewDiff_SinceArgument(t *testing.T) {
	root := reviewDiffSinceGitRepo(t)
	headSHA := func() string {
		out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(bytes.TrimSpace(out))
	}()

	var in review.Input
	reviewDiffWithBase(context.Background(), &in, root, "HEAD")
	if in.Root != root {
		t.Errorf("Root = %q, want %q", in.Root, root)
	}
	if in.Base != headSHA {
		t.Errorf("Base = %q, want the resolved sha %q", in.Base, headSHA)
	}
}

// TestReviewDiff_SinceArgument_Empty: no since argument means Root is still
// filled (needed for arch rules etc.) but Base stays empty — the
// review_diff wire contract's existing "checked:false, reason: needs a
// base ref" shape for a base-aware dimension, unchanged by this row.
func TestReviewDiff_SinceArgument_Empty(t *testing.T) {
	root := reviewDiffSinceGitRepo(t)
	var in review.Input
	reviewDiffWithBase(context.Background(), &in, root, "")
	if in.Root != root {
		t.Errorf("Root = %q, want %q", in.Root, root)
	}
	if in.Base != "" {
		t.Errorf("Base = %q, want empty (no since argument given)", in.Base)
	}
}

// TestReviewDiff_SinceArgument_UnresolvableRefFailsOpen: a since value that
// does not resolve (bad ref, corpusRoot not a git repo, git unavailable)
// never errors review_diff — every OTHER dimension must still run. Base
// stays empty, exactly as if since had not been given.
func TestReviewDiff_SinceArgument_UnresolvableRefFailsOpen(t *testing.T) {
	root := reviewDiffSinceGitRepo(t)
	var in review.Input
	reviewDiffWithBase(context.Background(), &in, root, "not-a-real-ref")
	if in.Base != "" {
		t.Errorf("Base = %q, want empty (ref does not resolve — fail open)", in.Base)
	}
	if in.Root != root {
		t.Errorf("Root = %q, want %q even when since fails to resolve", in.Root, root)
	}
}

// TestReviewDiff_SinceArgument_EmptyRootNeverShellsOut: an empty corpusRoot
// (service has no known path — ServiceMeta found nothing) is a no-op, not
// an attempt to run git with "-C ”" against the process cwd.
func TestReviewDiff_SinceArgument_EmptyRootNeverShellsOut(t *testing.T) {
	var in review.Input
	reviewDiffWithBase(context.Background(), &in, "", "HEAD")
	if in.Base != "" || in.Root != "" {
		t.Errorf("in = %+v, want a complete no-op with an empty root", in)
	}
}
