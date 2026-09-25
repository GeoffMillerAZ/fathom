package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/app-kit/internal/designassets"
)

func TestAssetsAdaptNavbarDispatch(t *testing.T) {
	if err := dataCmd(context.Background(), []string{"assets-adapt-navbar", "-root", ""}); command.Code(err) != 2 {
		t.Fatal("navbar subcommand not routed to strict invocation validation")
	}
}

func TestAssetsAdaptNavbarRefusals(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	for _, args := range [][]string{
		{"-root", ""}, {"-root", root, "extra"}, {"-root", root, "-out="},
		{"-root", root, "-check="}, {"-root", root, "-out", out, "-check", out},
		{"-root", root, "-unknown"}, {"-root", root}, {"-root", root, "-out", out},
		{"-root", root, "-check", out},
	} {
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-navbar"}, args...)...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatalf("refusal lost exit/output boundary for %v", args)
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("invalid invocation or unavailable source wrote filesystem state")
	}
}

func TestAssetsAdaptNavbarLive(t *testing.T) {
	kitRoot := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kitRoot == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kitRoot) {
		t.Fatal("absolute private checkout required")
	}
	assets := filepath.Join(kitRoot, "data", "design-assets")
	_, archiveSHA, ok := strings.Cut(designadapt.NavbarArchive, "@")
	if !ok {
		t.Fatal("navbar archive identity invalid")
	}
	sourcePaths := []string{filepath.Join(assets, "catalog.json"), filepath.Join(assets, "archives", archiveSHA+".zip")}
	sourceBefore := make(map[string][32]byte, len(sourcePaths))
	for _, path := range sourcePaths {
		digest, err := directoryDeliveryFileSHA(path)
		if err != nil {
			t.Fatal("private source identity unavailable")
		}
		sourceBefore[path] = digest
	}
	if sourceBefore[sourcePaths[1]] != mustDigest(t, archiveSHA) {
		t.Fatal("private archive digest differs from frozen identity")
	}
	t.Cleanup(func() {
		for path, want := range sourceBefore {
			if got, err := directoryDeliveryFileSHA(path); err != nil || got != want {
				t.Error("private archive/catalog changed or became unreadable")
			}
		}
	})

	first, firstErr := captureAssetsCLI(t, "assets-adapt-navbar", "-root", assets)
	second, secondErr := captureAssetsCLI(t, "assets-adapt-navbar", "-root", assets)
	if firstErr != nil || secondErr != nil || len(first) == 0 || !bytes.Equal(first, second) {
		t.Fatal("deterministic private JSON failed; bodies withheld")
	}
	var artifact designadapt.NavbarArtifact
	if err := json.Unmarshal(first, &artifact); err != nil || artifact.Verification != "not-verified" || artifact.Adapter != "p09-responsive-navbar-v1" {
		t.Fatal("private JSON envelope invalid; body withheld")
	}
	original, err := designassets.ReadFile(assets, designadapt.NavbarArchive, designadapt.NavbarMember)
	if err != nil || designadapt.VerifyNavbarReplay(artifact, []byte(original.Content)) != nil {
		t.Fatal("frozen source replay failed; source details withheld")
	}

	parent := t.TempDir()
	out := filepath.Join(parent, "delivery")
	invoke := func(want int, flags ...string) {
		t.Helper()
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-navbar", "-root", assets}, flags...)...)
		if command.Code(err) != want || len(body) != 0 {
			t.Fatalf("navbar exit/output mismatch: got %d/%d bytes, want %d/0", command.Code(err), len(body), want)
		}
	}
	invoke(0, "-out", out)
	wantNames := []string{"USAGE.md", "artifact.json", "defaults.json", "navbar.go", "navbar.templ"}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != len(wantNames) {
		t.Fatal("private delivery inventory differs")
	}
	for i, entry := range entries {
		info, err := entry.Info()
		if err != nil || entry.Name() != wantNames[i] || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			t.Fatal("private delivery name, type, or mode differs")
		}
	}
	if info, err := os.Stat(out); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatal("private delivery directory mode differs")
	}
	if delivered, err := os.ReadFile(filepath.Join(out, "artifact.json")); err != nil || !bytes.Equal(delivered, first) {
		t.Fatal("delivered artifact differs from default JSON")
	}
	invoke(0, "-check", out)
	invoke(2, "-out", out)
	invoke(0, "-check", out)
	invoke(1, "-check", filepath.Join(parent, "missing"))
	invoke(2, "-out", filepath.Join(parent, "other"), "-check", out)
	link := filepath.Join(parent, "delivery-link")
	if err := os.Symlink(out, link); err != nil {
		t.Fatal(err)
	}
	invoke(1, "-check", link)

	defaultsPath := filepath.Join(out, "defaults.json")
	defaults, err := os.ReadFile(defaultsPath)
	if err != nil {
		t.Fatal("private defaults unavailable")
	}
	if err := os.WriteFile(defaultsPath, append(append([]byte(nil), defaults...), '\n'), 0o600); err != nil {
		t.Fatal("could not seed private drift")
	}
	invoke(1, "-check", out)
	if err := os.WriteFile(defaultsPath, defaults, 0o600); err != nil {
		t.Fatal("could not restore private drift")
	}
	invoke(0, "-check", out)
}
