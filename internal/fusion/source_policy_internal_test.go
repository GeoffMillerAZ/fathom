package fusion

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestSourceMembershipStableRejectsConcurrentAdd(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "initial.txt"), []byte("[FIRST-1]"), 0644); err != nil {
		t.Fatal(err)
	}
	c := connector.NewLocalConnector()
	svc := core.ServiceRef{ID: "svc", Path: root}
	before, err := c.Discover(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "arrived.txt"), []byte("[LATE-2]"), 0644); err != nil {
		t.Fatal(err)
	}
	if sourceMembershipStable(context.Background(), c, svc, before) {
		t.Fatal("concurrent addition passed final membership check")
	}
}
