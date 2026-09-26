package certify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGaps_UnreadableSourceIsNamedNeverFewerTargets pins I38 for the
// target universe: incidents.md and a port's source file that exist but
// cannot be read are named in GapReport.Unread, never silently dropped —
// an unreadable incidents.md once made every incident target vanish, and
// the console's certifications page said "nothing declared" for a
// repository whose reads the operating system was refusing.
func TestGaps_UnreadableSourceIsNamedNeverFewerTargets(t *testing.T) {
	ctx, st, root := seedPortFixture(t)
	mustWrite(t, filepath.Join(root, "docs", "intent", "incidents.md"), "# Incidents\n\n### INC-1 — a thing happened\n\nOutcome: open\n")

	report, err := Gaps(ctx, st, testPortService, root, nil)
	if err != nil {
		t.Fatalf("Gaps: %v", err)
	}
	if len(report.Unread) != 0 {
		t.Fatalf("a readable corpus reports unread sources: %v", report.Unread)
	}
	kinds := map[string]int{}
	for _, tg := range report.Targets {
		kinds[tg.Kind]++
	}
	if kinds["incident"] != 1 || kinds["port"] != 2 {
		t.Fatalf("readable targets = %v, want 1 incident and 2 ports", kinds)
	}

	if os.Getuid() == 0 {
		t.Skip("root reads mode-000 files")
	}
	for _, rel := range []string{"docs/intent/incidents.md", "billing/coupon.go"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	}
	report, err = Gaps(ctx, st, testPortService, root, nil)
	if err != nil {
		t.Fatalf("Gaps: %v", err)
	}
	joined := strings.Join(report.Unread, "\n")
	for _, want := range []string{"docs/intent/incidents.md", "billing/coupon.go"} {
		if !strings.Contains(joined, want) {
			t.Errorf("unread = %v, want %s named", report.Unread, want)
		}
	}
}
