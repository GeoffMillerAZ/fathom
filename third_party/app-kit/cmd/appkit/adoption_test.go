package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/internal/adoption"
)

func adoptionCLIConsumer(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	s := adoption.Selection{SchemaVersion: 1, Consumer: "synthetic-cli-test", Profile: "go-web", Layers: []string{"L3b"}, Capabilities: []string{"shell.layout"}, CatalogRevision: c.Revision, Contract: "app-kit.md", Module: &adoption.ModuleRequirement{File: "go.mod", Version: "v0.0.0", AllowLocalReplace: true}, Artifacts: []adoption.Artifact{}, Agents: false}
	body, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for p, data := range map[string][]byte{
		adoption.DefaultManifest: body,
		"app-kit.md":             []byte("Synthetic adoption contract; no runtime proof."),
		"go.mod":                 []byte("module example.test/consumer\n\ngo 1.26.0\n\nrequire github.com/geoffmilleraz/app-kit v0.0.0\nreplace github.com/geoffmilleraz/app-kit => ../unresolved-app-kit\n"),
	} {
		if err := os.WriteFile(filepath.Join(root, p), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAdoptionCLIMachineExitAndOutput(t *testing.T) {
	good, empty := adoptionCLIConsumer(t), t.TempDir()
	for _, tc := range []struct {
		name    string
		args    []string
		code    int
		outcome string
	}{
		{"check matches", []string{"check", "--root", good, "--json"}, 0, "passed"},
		{"inspect declarations", []string{"inspect", "--root", good, "--json"}, 0, "observed"},
		{"unselected inventory", []string{"inspect", "--root", empty, "--json"}, 0, "observed"},
		{"missing required selection", []string{"check", "--root", empty, "--json"}, 1, "failed"},
		{"root unavailable", []string{"check", "--root", filepath.Join(empty, "missing"), "--json"}, 2, "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runAdoption(context.Background(), tc.args, &stdout, &stderr)
			if command.Code(err) != tc.code || stderr.Len() != 0 || bytes.Contains(stdout.Bytes(), []byte{0x1b}) {
				t.Fatalf("exit=%d want=%d; stderr=%q", command.Code(err), tc.code, stderr.String())
			}
			var r adoption.Report
			d := json.NewDecoder(&stdout)
			if d.Decode(&r) != nil || r.SchemaVersion != 1 || r.ExitCode != tc.code || r.Outcome != tc.outcome || len(r.Limitations) < 4 {
				t.Fatal("machine output is not one truthful versioned report")
			}
			if r.CheckerBuild.GoVersion == "" || r.CheckerBuild.GOOS == "" || r.CheckerBuild.GOARCH == "" {
				t.Fatal("machine report omitted the running checker's identity")
			}
			if d.Decode(new(any)) != io.EOF {
				t.Fatal("stdout has data after the JSON report")
			}
			if r.Module != nil && (r.Module.Resolution != "declarations-only" || r.Module.Replacement == nil || r.Module.Replacement.Path != "../unresolved-app-kit") {
				t.Fatal("local replacement was resolved or its limitation hidden")
			}
		})
	}
}

func TestAdoptionCLIUsageRouterAndHumanOutput(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"check", "extra"}, {"check", "--root", ""}, {"check", "--manifest", "../other.json"}, {"check", "--unknown"}, {"router", "--contract", "/absolute.md"}, {"router", "extra"}} {
		var out, stderr bytes.Buffer
		if err := runAdoption(context.Background(), args, &out, &stderr); command.Code(err) != 2 || out.Len() != 0 {
			t.Fatalf("invalid args %v: exit=%d stdout=%q", args, command.Code(err), out.String())
		}
	}
	var out, stderr bytes.Buffer
	if err := runAdoption(context.Background(), []string{"router", "--contract", "docs/ui-contract.md"}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	want, _ := adoption.Router("docs/ui-contract.md")
	if out.String() != want || stderr.Len() != 0 {
		t.Fatal("router output differs from the checker's canonical block")
	}
	out.Reset()
	t.Setenv("NO_COLOR", "1")
	if err := runAdoption(context.Background(), []string{"check", "--root", adoptionCLIConsumer(t)}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"static evidence only", "local replacement is allowed for development only", "Outcome: passed (static check only)", "No project commands", "not the effective build graph"} {
		if !strings.Contains(out.String(), phrase) {
			t.Errorf("human report omits %q", phrase)
		}
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("NO_COLOR human output contains ANSI")
	}
	for _, args := range [][]string{{"--help"}, {"check", "--help"}, {"router", "--help"}} {
		if err := runAdoption(context.Background(), args, &out, &stderr); err != nil {
			t.Fatalf("help failed: %v", err)
		}
	}
}

func TestAdoptionCLIPinsIsReadOnlyAndMachineComplete(t *testing.T) {
	root := adoptionCLIConsumer(t)
	manifest := filepath.Join(root, adoption.DefaultManifest)
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	err = runAdoption(context.Background(), []string{"pins", "--root", root, "--json"}, &out, &stderr)
	if command.Code(err) != 0 || stderr.Len() != 0 {
		t.Fatalf("pins exit=%d stderr=%q err=%v", command.Code(err), stderr.String(), err)
	}
	var report adoption.PinReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 1 || report.Outcome != "observed" || report.ExitCode != 0 || len(report.CapabilityRevisions) != 1 || len(report.Entries) != 1 {
		t.Fatalf("incomplete pins report: %+v", report)
	}
	entry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	wantEntry, ok := entry.Get("shell.layout")
	if !ok || report.CapabilityRevisions[0].ID != wantEntry.ID {
		t.Fatalf("wrong selected entry: %+v", report.CapabilityRevisions)
	}
	envelope := struct {
		Domain               string        `json:"domain"`
		Version              int           `json:"version"`
		CatalogSchemaVersion int           `json:"catalog_schema_version"`
		Entry                catalog.Entry `json:"entry"`
	}{"app-kit/catalog-entry", 1, entry.SchemaVersion, wantEntry}
	body, _ := json.Marshal(envelope)
	digest := sha256.Sum256(body)
	if got := report.CapabilityRevisions[0].SHA256; got != hex.EncodeToString(digest[:]) {
		t.Fatalf("pin digest=%s want independent envelope digest=%s", got, hex.EncodeToString(digest[:]))
	}
	after, _ := os.ReadFile(manifest)
	if !bytes.Equal(before, after) {
		t.Fatal("pins changed manifest or failed to emit JSON")
	}
	if strings.Contains(out.String(), "app-kit adoption pins:") {
		t.Fatal("machine pins output contains human decoration")
	}
	out.Reset()
	if err := runAdoption(context.Background(), []string{"pins", "--root", root}, &out, &stderr); err != nil || !strings.Contains(out.String(), "shell.layout") || !strings.Contains(out.String(), report.CapabilityRevisions[0].SHA256) {
		t.Fatalf("human pins output missing ID/hash: err=%v output=%q", err, out.String())
	}
}

func TestAdoptionCLIPinsUnknownAndUnavailable(t *testing.T) {
	root := adoptionCLIConsumer(t)
	var object map[string]any
	body, _ := os.ReadFile(filepath.Join(root, adoption.DefaultManifest))
	if err := json.Unmarshal(body, &object); err != nil {
		t.Fatal(err)
	}
	object["capabilities"] = []any{"missing.capability"}
	body, _ = json.Marshal(object)
	if err := os.WriteFile(filepath.Join(root, adoption.DefaultManifest), body, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if err := runAdoption(context.Background(), []string{"pins", "--root", root, "--json"}, &out, &stderr); command.Code(err) != 1 || stderr.Len() != 0 {
		t.Fatalf("unknown pin exit=%d stderr=%q", command.Code(err), stderr.String())
	}
	var failed adoption.PinReport
	if json.Unmarshal(out.Bytes(), &failed) != nil || failed.ExitCode != 1 || len(failed.CapabilityRevisions) != 0 {
		t.Fatal("unknown ID did not produce bounded failed pins report")
	}
	out.Reset()
	if err := runAdoption(context.Background(), []string{"pins", "--root", filepath.Join(root, "missing"), "--json"}, &out, &stderr); command.Code(err) != 2 {
		t.Fatalf("unavailable root exit=%d", command.Code(err))
	}
}

type adoptionBrokenWriter struct{}

func (adoptionBrokenWriter) Write([]byte) (int, error) {
	return 0, errors.New("synthetic write failure")
}

func TestAdoptionCLIReportFailureAndCancellation(t *testing.T) {
	root := adoptionCLIConsumer(t)
	for _, args := range [][]string{{"check", "--root", root}, {"check", "--root", root, "--json"}, {"pins", "--root", root}, {"pins", "--root", root, "--json"}, {"router"}, {"--help"}} {
		if err := runAdoption(context.Background(), args, adoptionBrokenWriter{}, io.Discard); command.Code(err) != 2 {
			t.Fatalf("writer failure exit=%d", command.Code(err))
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout bytes.Buffer
	if err := runAdoption(ctx, []string{"check", "--root", root, "--json"}, &stdout, io.Discard); command.Code(err) != 2 {
		t.Fatalf("cancellation exit=%d", command.Code(err))
	}
}
