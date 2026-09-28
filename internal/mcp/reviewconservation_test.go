package mcp

// reviewconservation_test.go — DimConservation's internal/mcp wiring
// (ADR-044 §1.6/§2, wave 2): review_diff's buildReviewInput runs
// internal/conserve's full plane set (conserve.Planes()) through the real
// *pkg/client.Client (which satisfies conserveBackend via its Store()
// accessor) once a base resolves, end to end against a throwaway git
// repo. White-box (package mcp, not mcp_test) because buildReviewInput is
// unexported — mirrors reviewdiff_since_test.go's own git-repo fixture
// pattern, extended with a real commit history (base/head) since that
// file's own fixture is a single commit with nothing to diff.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// conservationWireGitRepo is a throwaway git work tree with TWO commits:
// base declares an exported api:widgets.Old, head removes it — the same
// undeclared-surface-loss shape cmd/fathom/conserve_test.go's own fixture
// uses. Returns the repo root and the base commit's sha.
func conservationWireGitRepo(t *testing.T) (root, base string) {
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
	write("widgets/w.go", "package widgets\n\nfunc Old() {}\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base commit")
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base = string(bytes.TrimSpace(out))

	// Remove the exported func at head — an undeclared surface loss.
	write("widgets/w.go", "package widgets\n\nfunc New() {}\n")
	return root, base
}

// TestBuildReviewInput_ConservationDimensionWiredEndToEnd pins that
// review_diff's buildReviewInput, given a real *client.Client (which now
// satisfies conserveBackend) and a resolvable `since`, fills
// ConservationSupported/ConservationPlanes so DimConservation reports a
// checked dimension naming the removed api:widgets.Old — never
// checked:false, whatever --since names. Before this wiring,
// ConservationSupported/ConservationPlanes were never set by any caller
// in this package: every review_diff call read checked:false regardless
// of `since`.
func TestBuildReviewInput_ConservationDimensionWiredEndToEnd(t *testing.T) {
	root, base := conservationWireGitRepo(t)

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()
	if err := c.IngestServiceAt(ctx, "svc", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}

	in, _, rpcErr := buildReviewInput(ctx, c, "svc", []string{"widgets/w.go"}, "", base, nil)
	if rpcErr != nil {
		t.Fatalf("buildReviewInput: %+v", rpcErr)
	}
	if !in.ConservationSupported {
		t.Fatalf("ConservationSupported = false, want true (a real conserveBackend and a resolvable base)")
	}
	var sawSurfaceLoss bool
	for _, p := range in.ConservationPlanes {
		for _, f := range p.Findings {
			if f.Kind == "removed" && f.Key == "api:widgets.Old" {
				sawSurfaceLoss = true
			}
		}
	}
	if !sawSurfaceLoss {
		t.Errorf("expected a removed api:widgets.Old finding on the surface plane; planes=%+v", in.ConservationPlanes)
	}
}

// TestBuildReviewInput_ConservationDimensionNoSinceIsUnsupported pins the
// "no --since" degrade at the wiring level: ConservationSupported stays
// false when since is "" — DimConservation's own "needs a base ref" gate
// (internal/analysis/review) is the only thing a --since-less call ever
// reports.
func TestBuildReviewInput_ConservationDimensionNoSinceIsUnsupported(t *testing.T) {
	root, _ := conservationWireGitRepo(t)

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	ctx := context.Background()
	if err := c.IngestServiceAt(ctx, "svc", root); err != nil {
		t.Fatalf("IngestServiceAt: %v", err)
	}

	in, _, rpcErr := buildReviewInput(ctx, c, "svc", []string{"widgets/w.go"}, "", "", nil)
	if rpcErr != nil {
		t.Fatalf("buildReviewInput: %+v", rpcErr)
	}
	if in.ConservationSupported {
		t.Errorf("ConservationSupported = true, want false (no since given)")
	}
}
