package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/internal/adoption"
)

func draftCLIArgs(root, revision string) []string {
	return []string{"draft", "--root", root, "--consumer", "cli-draft", "--profile", "portable", "--contract", "docs/app-kit.md",
		"--expected-catalog-revision", revision, "--layer", "L1", "--layer", "L0",
		"--capability", "foundation.tokens", "--capability", "foundation.theme.geoff-synthwave",
		"--artifact", "web/z.css", "--artifact", "web/a.css"}
}

func draftCLIRoot(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string][]byte{"web/z.css": []byte("z\n"), "web/a.css": []byte("a\n")} {
		name := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, c.Revision
}

func TestAdoptionCLIDraftEmitsOnlyStableIndentedV2JSON(t *testing.T) {
	root, revision := draftCLIRoot(t)
	args := draftCLIArgs(root, revision)
	var first, stderr bytes.Buffer
	if err := runAdoption(context.Background(), args, &first, &stderr); err != nil || stderr.Len() != 0 {
		t.Fatalf("draft failed: %v, stderr=%q", err, stderr.String())
	}
	var selection adoption.Selection
	decoder := json.NewDecoder(bytes.NewReader(first.Bytes()))
	if err := decoder.Decode(&selection); err != nil || decoder.Decode(new(any)) != io.EOF {
		t.Fatalf("stdout is not exactly one JSON manifest: %v", err)
	}
	if selection.SchemaVersion != 2 || selection.CatalogRevision != "" || selection.CapabilityRevisions == nil || selection.Artifacts[0].Path != "web/a.css" {
		t.Fatalf("wrong draft selection: %+v", selection)
	}
	if !bytes.Contains(first.Bytes(), []byte("\n  \"consumer\"")) || bytes.Contains(first.Bytes(), []byte("static evidence only")) {
		t.Fatalf("draft output is not undecorated indented JSON: %q", first.String())
	}
	var second bytes.Buffer
	reversed := draftCLIArgs(root, revision)
	// Reverse only the values of each repeated family; canonical bytes must survive.
	reversed[12], reversed[14] = reversed[14], reversed[12]
	reversed[16], reversed[18] = reversed[18], reversed[16]
	reversed[20], reversed[22] = reversed[22], reversed[20]
	if err := runAdoption(context.Background(), reversed, &second, io.Discard); err != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("flag order changed canonical output: %v\nfirst=%s\nsecond=%s", err, first.Bytes(), second.Bytes())
	}
	if _, err := os.Stat(filepath.Join(root, adoption.DefaultManifest)); !os.IsNotExist(err) {
		t.Fatal("draft wrote a manifest")
	}
}

func TestAdoptionCLIDraftArtifactOnlyCLIUsesEmptyArrays(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "artifact.txt"), []byte("selected\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := runAdoption(context.Background(), []string{"draft", "--root", root, "--consumer", "artifact-only", "--profile", "cli", "--contract", "docs/app-kit.md", "--artifact", "artifact.txt"}, &stdout, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"layers": []`, `"capabilities": []`, `"capability_revisions": []`} {
		if !strings.Contains(stdout.String(), field) {
			t.Fatalf("artifact-only JSON omitted required empty %s: %s", field, stdout.String())
		}
	}
}

func TestAdoptionCLIDraftForwardsExplicitModuleAndAgents(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	args := []string{"draft", "--root", root, "--consumer", "cli-only", "--profile", "cli", "--contract", "missing.md",
		"--capability", "cli.command", "--module-file", "missing/go.mod", "--module-version", "v1.2.3", "--allow-local-replace", "--agents"}
	if err := runAdoption(context.Background(), args, &stdout, io.Discard); err != nil {
		t.Fatal(err)
	}
	var got adoption.Selection
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || got.Module == nil || !got.Module.AllowLocalReplace || !got.Agents {
		t.Fatalf("explicit optional flags lost: %+v, %v", got, err)
	}
}

func TestAdoptionCLIDraftErrorsEmitNoJSONAndExitTwo(t *testing.T) {
	root, revision := draftCLIRoot(t)
	valid := draftCLIArgs(root, revision)
	missing := filepath.Join(root, "missing-root")
	cases := [][]string{
		{"draft"},
		{"draft", "--root", "", "--consumer", "x", "--profile", "cli", "--contract", "x.md", "--artifact", "x"},
		{"draft", "--root", missing, "--consumer", "x", "--profile", "cli", "--contract", "x.md", "--artifact", "x"},
		{"draft", "--root", root, "--consumer", "x", "--profile", "cli", "--contract", "x.md", "--module-file", "go.mod"},
		{"draft", "--root", root, "--consumer", "x", "--profile", "cli", "--contract", "x.md", "--module-version", "v1.2.3"},
		{"draft", "--root", root, "--consumer", "x", "--profile", "cli", "--contract", "x.md", "--artifact", "web/a.css", "--allow-local-replace"},
		append(append([]string(nil), valid...), "--json"),
	}
	for _, args := range cases {
		var stdout bytes.Buffer
		if err := runAdoption(context.Background(), args, &stdout, io.Discard); command.Code(err) != 2 || stdout.Len() != 0 {
			t.Fatalf("args=%v exit=%d stdout=%q", args, command.Code(err), stdout.String())
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout bytes.Buffer
	if err := runAdoption(canceled, valid, &stdout, io.Discard); command.Code(err) != 2 || stdout.Len() != 0 {
		t.Fatalf("canceled draft exit=%d stdout=%q", command.Code(err), stdout.String())
	}
	if err := runAdoption(context.Background(), valid, adoptionBrokenWriter{}, io.Discard); command.Code(err) != 2 {
		t.Fatalf("writer failure exit=%d", command.Code(err))
	}
	if err := runAdoption(context.Background(), valid, adoptionDraftShortWriter{}, io.Discard); command.Code(err) != 2 {
		t.Fatalf("short writer exit=%d", command.Code(err))
	}
}

type adoptionDraftShortWriter struct{}

func (adoptionDraftShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestAdoptionCLIDraftHelpStatesBoundary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := runAdoption(context.Background(), []string{"draft", "--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("help failed: %v", err)
	}
	text := strings.ToLower(stdout.String() + stderr.String())
	for _, phrase := range []string{"draft", "does not prove", "no writes", "upgrade"} {
		if !strings.Contains(text, phrase) {
			t.Errorf("draft help omits %q: %q", phrase, text)
		}
	}
}

func TestAdoptionCLIDraftDoesNotMutateArgumentSlices(t *testing.T) {
	root, revision := draftCLIRoot(t)
	args := draftCLIArgs(root, revision)
	want := append([]string(nil), args...)
	var stdout bytes.Buffer
	if err := runAdoption(context.Background(), args, &stdout, io.Discard); err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("runAdoption mutated args: %v", err)
	}
}
