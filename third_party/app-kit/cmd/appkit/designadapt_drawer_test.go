package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

func TestAssetsAdaptDrawerDispatch(t *testing.T) {
	if err := dataCmd(context.Background(), []string{"assets-adapt-drawer", "-root", ""}); command.Code(err) != 2 {
		t.Fatal("drawer subcommand not routed to strict invocation validation")
	}
}

func TestAssetsAdaptDrawerRefusals(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	for _, args := range [][]string{
		{"-root", ""}, {"-root", root, "extra"}, {"-root", root, "-out="},
		{"-root", root, "-check="}, {"-root", root, "-out", out, "-check", out},
		{"-root", root, "-unknown"}, {"-root", root, "-repair"}, {"-root", root},
		{"-root", root, "-out", out}, {"-root", root, "-check", out},
		{"-root", root, "-repair=invalid"}, {"-root", root, "-repair", "-out", out},
		{"-root", root, "-repair", "-out", out, "-check", out},
	} {
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-drawer"}, args...)...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatalf("refusal lost exit/output boundary for %v", args)
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("invalid invocation or unavailable source wrote filesystem state")
	}
}

func TestAssetsAdaptDrawerRepairLive(t *testing.T) {
	kit := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kit == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kit) {
		t.Fatal("absolute private checkout required")
	}
	assets := filepath.Join(kit, "data", "design-assets")
	args := []string{"assets-adapt-drawer", "-root", assets}
	baseBytes, baseErr := captureAssetsCLI(t, args...)
	disabledBytes, disabledErr := captureAssetsCLI(t, append(args, "-repair=false")...)
	first, firstErr := captureAssetsCLI(t, append(args, "-repair")...)
	second, secondErr := captureAssetsCLI(t, append(args, "-repair")...)
	if baseErr != nil || disabledErr != nil || firstErr != nil || secondErr != nil || !bytes.Equal(baseBytes, disabledBytes) || !bytes.Equal(first, second) {
		t.Fatal("explicit repair determinism/default compatibility failed; output withheld")
	}
	var base designadapt.DrawerArtifact
	var repair designadapt.DrawerFocusArtifact
	if json.Unmarshal(baseBytes, &base) != nil || json.Unmarshal(first, &repair) != nil || !reflect.DeepEqual(base, repair.Base) || repair.Adapter != "p08-drawer-focus-v1" || repair.Verification != "not-verified" {
		t.Fatal("repair envelope changed base or availability")
	}
	original, err := designassets.ReadFile(assets, designadapt.DrawerArchive, designadapt.DrawerMember)
	if err != nil || designadapt.VerifyDrawerFocusReplay(repair, []byte(original.Content)) != nil {
		t.Fatal("private repair source replay failed")
	}
	parent := t.TempDir()
	baseDir, repairDir := filepath.Join(parent, "base"), filepath.Join(parent, "repair")
	invoke := func(want int, flags ...string) {
		t.Helper()
		output, err := captureAssetsCLI(t, append(args, flags...)...)
		if command.Code(err) != want || len(output) != 0 {
			t.Fatalf("exit/output mismatch: got %d/%d bytes, want %d/0", command.Code(err), len(output), want)
		}
	}
	invoke(0, "-out", baseDir)
	invoke(0, "-repair", "-out", repairDir)
	invoke(0, "-repair", "-check", repairDir)
	invoke(1, "-check", repairDir)
	invoke(1, "-repair", "-check", baseDir)
	invoke(1, "-repair", "-check", filepath.Join(parent, "missing"))
	invoke(2, "-repair", "-out", repairDir)
	invoke(2, "-repair", "-out", filepath.Join(parent, "other"), "-check", repairDir)
	if got, err := os.ReadFile(filepath.Join(repairDir, "artifact.json")); err != nil || !bytes.Equal(got, first) {
		t.Fatal("repair delivery differs from JSON output")
	}
	files, err := os.ReadDir(repairDir)
	if err != nil || len(files) != 6 {
		t.Fatal("repair inventory differs")
	}
	css := filepath.Join(repairDir, "drawer_focus.css")
	if err := os.WriteFile(css, []byte(repair.CSS+"\n/* changed */"), 0600); err != nil {
		t.Fatal(err)
	}
	invoke(1, "-repair", "-check", repairDir)
	if err := os.WriteFile(css, []byte(repair.CSS), 0600); err != nil {
		t.Fatal(err)
	}
	invoke(0, "-repair", "-check", repairDir)
	invoke(0, "-check", baseDir)
}

func TestAssetsAdaptDrawerLive(t *testing.T) {
	kitRoot := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kitRoot == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kitRoot) {
		t.Fatal("absolute private checkout required")
	}
	assets := filepath.Join(kitRoot, "data", "design-assets")
	_, archiveSHA, ok := strings.Cut(designadapt.DrawerArchive, "@")
	if !ok {
		t.Fatal("drawer archive identity invalid")
	}
	sourcePaths := []string{filepath.Join(assets, "catalog.json"), filepath.Join(assets, "archives", archiveSHA+".zip")}
	sourceBefore := make(map[string][32]byte)
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
	first, firstErr := captureAssetsCLI(t, "assets-adapt-drawer", "-root", assets)
	second, secondErr := captureAssetsCLI(t, "assets-adapt-drawer", "-root", assets)
	if firstErr != nil || secondErr != nil || len(first) == 0 || !bytes.Equal(first, second) {
		t.Fatal("deterministic private JSON failed; bodies withheld")
	}
	var artifact designadapt.DrawerArtifact
	if err := json.Unmarshal(first, &artifact); err != nil || artifact.Verification != "not-verified" || artifact.Adapter != "p08-profile-drawer-v1" {
		t.Fatal("private JSON envelope invalid; body withheld")
	}
	original, err := designassets.ReadFile(assets, designadapt.DrawerArchive, designadapt.DrawerMember)
	if err != nil || designadapt.VerifyDrawerReplay(artifact, []byte(original.Content)) != nil {
		t.Fatal("frozen source replay failed; source details withheld")
	}
	parent := t.TempDir()
	out := filepath.Join(parent, "delivery")
	invoke := func(want int, flags ...string) {
		t.Helper()
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-drawer", "-root", assets}, flags...)...)
		if command.Code(err) != want || len(body) != 0 {
			t.Fatalf("drawer exit/output mismatch: got %d/%d bytes, want %d/0", command.Code(err), len(body), want)
		}
	}
	invoke(0, "-out", out)
	wantNames := []string{"USAGE.md", "artifact.json", "defaults.json", "drawer.go", "drawer.templ"}
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
