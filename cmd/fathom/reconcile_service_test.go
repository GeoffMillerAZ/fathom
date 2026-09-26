package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// `fleet reconcile --repo <root>` rendered the stanza with the directory's
// basename whenever the registry was not consulted — always, with --repo.
// A checkout at …/sextant registered as "fathom" read "CLAUDE.md behind the
// current template", and --apply (OWN-4's documented remedy) would have
// rewritten both stanzas to name a service that does not exist. The id now
// comes from the registry when reachable, else from the service the
// existing fathom region already names, and the basename only for a repo
// whose region names none.
func TestReconcileRepoFlag_KeepsTheServiceTheStanzaNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "sextant")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	regions, _ := stanzaRegionsFit(root, stanzaFiles, harness.ClaudeCode, "fathom", intentChannelWired(root))
	if _, err := wiring.ApplyWiring(root, wiring.Spec{Regions: regions}, wiring.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if !strings.Contains(string(before), `service "fathom"`) {
		t.Fatalf("fixture stanza does not name the service:\n%s", before)
	}

	var stdout, stderr strings.Builder
	unreachable := "http://127.0.0.1:1"
	if code := cmdFleetReconcile([]string{"--url", unreachable, "--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("dry-run exit = %d:\n%s%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "behind the current template") {
		t.Errorf("a current stanza read as stale under the directory's name:\n%s", stdout.String())
	}
	if code := cmdFleetReconcile([]string{"--url", unreachable, "--apply", "--repo", root}, &stdout, &stderr); code != 0 {
		t.Fatalf("apply exit = %d:\n%s%s", code, stdout.String(), stderr.String())
	}
	after, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if strings.Contains(string(after), `service "sextant"`) || !strings.Contains(string(after), `service "fathom"`) {
		t.Errorf("--apply renamed the service the stanza names:\n%s", after)
	}
}
