package reviewbench

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

// fakeGit records every call it receives (dir + args) and returns a
// caller-scripted response, so EnsureRepo/FetchCommit/CheckoutCommit/
// ChangedFilesSince can be pinned without a real git binary or network.
type fakeGit struct {
	calls   [][]string
	diffOut string
	diffErr error
}

func (f *fakeGit) run(dir string, args ...string) ([]byte, error) {
	call := append([]string{dir}, args...)
	f.calls = append(f.calls, call)
	if len(args) > 0 && args[0] == "diff" {
		return []byte(f.diffOut), f.diffErr
	}
	return nil, nil
}

func TestChangedFilesSince_ParsesAndTrims(t *testing.T) {
	fg := &fakeGit{diffOut: "a.go\n\nb/c.go\n  \nd.go\n"}
	got, err := ChangedFilesSince(fg.run, "/repo", "deadbeef")
	if err != nil {
		t.Fatalf("ChangedFilesSince: %v", err)
	}
	want := []string{"a.go", "b/c.go", "d.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	wantCall := []string{"/repo", "diff", "--name-only", "deadbeef"}
	if len(fg.calls) != 1 || !reflect.DeepEqual(fg.calls[0], wantCall) {
		t.Errorf("git call = %v, want %v", fg.calls, wantCall)
	}
}

func TestChangedFilesSince_PropagatesError(t *testing.T) {
	fg := &fakeGit{diffErr: fmt.Errorf("boom")}
	if _, err := ChangedFilesSince(fg.run, "/repo", "deadbeef"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestFetchCommit_AndCheckoutCommit_ShellOutExactly(t *testing.T) {
	fg := &fakeGit{}
	if err := FetchCommit(fg.run, "/repo", "abc123"); err != nil {
		t.Fatalf("FetchCommit: %v", err)
	}
	if err := CheckoutCommit(fg.run, "/repo", "abc123"); err != nil {
		t.Fatalf("CheckoutCommit: %v", err)
	}
	want := [][]string{
		{"/repo", "fetch", "--depth", "1", "origin", "abc123"},
		{"/repo", "checkout", "-f", "abc123"},
	}
	if !reflect.DeepEqual(fg.calls, want) {
		t.Errorf("calls = %v, want %v", fg.calls, want)
	}
}

func TestEnsureRepo_InitsAndAddsRemoteOnlyOnce(t *testing.T) {
	dir := t.TempDir() + "/repo"
	fg := &fakeGit{}
	if err := EnsureRepo(fg.run, dir, "owner/name"); err != nil {
		t.Fatalf("EnsureRepo: %v", err)
	}
	want := [][]string{
		{dir, "init", "-q"},
		{dir, "remote", "add", "origin", "https://github.com/owner/name.git"},
	}
	if !reflect.DeepEqual(fg.calls, want) {
		t.Errorf("first call calls = %v, want %v", fg.calls, want)
	}

	// A .git subdirectory now exists on disk (created by the real test —
	// EnsureRepo's own os.MkdirAll only made the parent; fake git never
	// actually creates .git, so simulate what a real `git init` leaves
	// behind before checking the second call is a no-op).
	if err := os.MkdirAll(dir+"/.git", 0o755); err != nil {
		t.Fatal(err)
	}
	fg.calls = nil
	if err := EnsureRepo(fg.run, dir, "owner/name"); err != nil {
		t.Fatalf("EnsureRepo (second call): %v", err)
	}
	if len(fg.calls) != 0 {
		t.Errorf("second EnsureRepo call ran %v, want no git calls (already a repo)", fg.calls)
	}
}
