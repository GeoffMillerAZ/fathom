package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestPushCheckScript runs scripts/push-check_test.sh (SU-26, invariant
// I40): a push from any clone refuses when the lefthook pre-push hook is
// absent. The shell test builds its own temp repositories; this wrapper
// only makes the script part of `go test` so the plan row is anchored to
// a symbol the index can see.
func TestPushCheckScript(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "scripts", "push-check_test.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("missing %s: %v", script, err)
	}
	cmd := exec.Command("bash", script)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("push-check_test.sh failed: %v\n%s", err, out)
	}
}

// TestPushHTTPScript runs scripts/push-http_test.sh (INC-22, invariant
// I41): `task push` sends the pack over smart HTTP, derived from the push
// remote, so no transport session is held open across the pre-push gate.
// The shell test only derives URLs (--print-url) and never touches the
// network.
func TestPushHTTPScript(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "scripts", "push-http_test.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("missing %s: %v", script, err)
	}
	cmd := exec.Command("bash", script)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("push-http_test.sh failed: %v\n%s", err, out)
	}
}
