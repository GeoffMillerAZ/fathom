package mcp

// review_integrity_test.go — DimIntegrity's internal/mcp wiring
// (ADR-044 §1.6, CV-12, refactor-conservation gap-closing wave 3 item 1):
// review_diff's buildReviewInput runs the "integrity" conserve plane through
// the real *pkg/client.Client (which satisfies conserveBackend via its
// Store() accessor) once a base resolves, end to end against a throwaway
// git repo whose head deletes a guarded (// certifies:-marked) test —
// mirrors reviewconservation_test.go's own git-repo fixture pattern
// exactly, adapted to internal/conserve's own integrity-plane fixture shape
// (internal/conserve/plane_integrity_test.go's
// TestIntegrityPlane_DeletedGuardedTestIsAFinding).

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// integrityWireGitRepo is a throwaway git work tree with TWO commits: base
// declares a guarded test (a `// certifies:` marker), head deletes it — the
// same undeclared-loss shape internal/conserve's own plane test uses.
// Returns the repo root and the base commit's sha.
func integrityWireGitRepo(t *testing.T) (root, base string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root = t.TempDir()
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
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	run("config", "commit.gpgsign", "false")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.com")
	write("pkg/x_test.go", "package pkg\n\nimport \"testing\"\n\n// certifies: invariant I1 (asserts)\nfunc TestGuarded(t *testing.T) {\n\tif 1 != 2 {\n\t\tt.Fatalf(\"boom\")\n\t}\n}\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base commit")
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base = string(bytes.TrimSpace(out))

	// Delete the guarded test at head — an undeclared integrity loss.
	write("pkg/x_test.go", "package pkg\n\nimport \"testing\"\n")
	return root, base
}

// TestBuildReviewInput_IntegrityDimensionWiredEndToEnd pins that
// review_diff's buildReviewInput, given a real *client.Client (which
// satisfies conserveBackend) and a resolvable `since`, fills
// IntegrityFindings with the deleted guarded test — never empty, whatever
// --since names. Before this wiring, review_diff never called the
// "integrity" plane at all: every review_diff call with a resolvable since
// rendered DimIntegrity as checked:true with ZERO findings, indistinguishable
// from a genuinely clean range.
func TestBuildReviewInput_IntegrityDimensionWiredEndToEnd(t *testing.T) {
	root, base := integrityWireGitRepo(t)

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()
	if err := c.IngestServiceAt(ctx, "svc", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}

	in, _, rpcErr := buildReviewInput(ctx, c, "svc", []string{"pkg/x_test.go"}, "", base, nil)
	if rpcErr != nil {
		t.Fatalf("buildReviewInput: %+v", rpcErr)
	}
	var sawDeleted bool
	for _, f := range in.IntegrityFindings {
		if f.Kind == "deleted" && f.Test == "TestGuarded" {
			sawDeleted = true
		}
	}
	if !sawDeleted {
		t.Errorf("expected a deleted TestGuarded integrity finding; findings=%+v", in.IntegrityFindings)
	}
}

// TestBuildReviewInput_IntegrityDimensionNoSinceIsEmpty pins the "no
// --since" degrade at the wiring level: IntegrityFindings stays empty when
// since is "" — DimIntegrity's own "needs a base ref" gate (internal/
// analysis/review) is the only thing a --since-less call ever reports.
func TestBuildReviewInput_IntegrityDimensionNoSinceIsEmpty(t *testing.T) {
	root, _ := integrityWireGitRepo(t)

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()
	if err := c.IngestServiceAt(ctx, "svc", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}

	in, _, rpcErr := buildReviewInput(ctx, c, "svc", []string{"pkg/x_test.go"}, "", "", nil)
	if rpcErr != nil {
		t.Fatalf("buildReviewInput: %+v", rpcErr)
	}
	if len(in.IntegrityFindings) != 0 {
		t.Errorf("IntegrityFindings = %+v, want empty (no since given)", in.IntegrityFindings)
	}
}

// TestReviewDiff_IntegrityDescriptionMentionsIt: the tool's own Description
// must not undersell what since actually feeds — a caller reading it
// should know integrity/conservation exist before deciding whether to pass
// since (the exact discipline TestReviewDiff_SinceDescriptionNamesItsConsumers
// already pins for the since parameter's own sub-description).
func TestReviewDiff_IntegrityDescriptionMentionsIt(t *testing.T) {
	s := NewServer(nil)
	te, ok := s.findTool("review_diff")
	if !ok {
		t.Fatal("review_diff not registered")
	}
	desc := te.desc.Description
	for _, want := range []string{"integrity", "conservation"} {
		if !strings.Contains(desc, want) {
			t.Errorf("review_diff Description does not mention %q: %q", want, desc)
		}
	}
}
