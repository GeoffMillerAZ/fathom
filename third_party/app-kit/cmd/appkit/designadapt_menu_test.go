package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

func TestAssetsAdaptMenuDispatch(t *testing.T) {
	if err := dataCmd(context.Background(), []string{"assets-adapt-menu", "-root", ""}); command.Code(err) != 2 {
		t.Fatal("menu subcommand not routed to strict invocation validation")
	}
}

func TestAssetsAdaptMenuRefusesBadFlagsAndUnavailableSourceWithoutOutput(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	for _, args := range [][]string{
		{"-root", ""},
		{"-root", root, "extra"},
		{"-root", root, "-out="},
		{"-root", root, "-check="},
		{"-root", root, "-out", out, "-check", filepath.Join(root, "check")},
		{"-root", root, "-unknown"},
		{"-root", root},
		{"-root", root, "-out", out},
		{"-root", root, "-check", filepath.Join(root, "missing-delivery")},
	} {
		for _, variant := range [][]string{nil, {"-repair"}} {
			body, err := captureAssetsCLI(t, append(append([]string{"assets-adapt-menu"}, variant...), args...)...)
			if command.Code(err) != 2 || len(body) != 0 {
				t.Fatalf("refusal lost exit/output boundary for %v %v", variant, args)
			}
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid invocation or unavailable source wrote filesystem state")
	}
}

func TestAssetsAdaptMenuLive(t *testing.T) {
	kitRoot := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kitRoot == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kitRoot) {
		t.Fatal("absolute private checkout required")
	}
	assets := filepath.Join(kitRoot, "data", "design-assets")
	_, archiveSHA, ok := strings.Cut(designadapt.MenuArchive, "@")
	if !ok {
		t.Fatal("menu archive identity invalid")
	}
	sourcePaths := []string{
		filepath.Join(assets, "catalog.json"),
		filepath.Join(assets, "archives", archiveSHA+".zip"),
	}
	sourceBefore := make(map[string][32]byte, len(sourcePaths))
	for _, path := range sourcePaths {
		digest, err := directoryDeliveryFileSHA(path)
		if err != nil {
			t.Fatal("private source identity unavailable")
		}
		sourceBefore[path] = digest
	}
	if got := sourceBefore[sourcePaths[1]]; got != mustDigest(t, archiveSHA) {
		t.Fatal("private archive digest differs from frozen identity")
	}
	t.Cleanup(func() {
		for path, want := range sourceBefore {
			got, err := directoryDeliveryFileSHA(path)
			if err != nil || got != want {
				t.Error("private archive/catalog changed or became unreadable")
			}
		}
	})

	first, firstErr := captureAssetsCLI(t, "assets-adapt-menu", "-root", assets)
	second, secondErr := captureAssetsCLI(t, "assets-adapt-menu", "-root", assets)
	if firstErr != nil || secondErr != nil || !bytes.Equal(first, second) || len(first) == 0 {
		t.Fatal("deterministic private JSON generation failed; bodies withheld")
	}
	var artifact designadapt.MenuArtifact
	if err := json.Unmarshal(first, &artifact); err != nil || artifact.Verification != "not-verified" || artifact.Adapter != "p07-menu-v1" {
		t.Fatal("private JSON envelope invalid; body withheld")
	}
	original, err := designassets.ReadFile(assets, designadapt.MenuArchive, designadapt.MenuMember)
	if err != nil || designadapt.VerifyMenuReplay(artifact, []byte(original.Content)) != nil {
		t.Fatal("frozen source replay failed; source details withheld")
	}

	parent := t.TempDir()
	out := filepath.Join(parent, "delivery")
	invoke := func(want int, flags ...string) {
		t.Helper()
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-menu", "-root", assets}, flags...)...)
		if command.Code(err) != want || len(body) != 0 {
			t.Fatalf("menu exit/output mismatch: got %d/%d bytes, want %d/0", command.Code(err), len(body), want)
		}
	}
	invoke(0, "-out", out)
	wantNames := []string{"USAGE.md", "artifact.json", "defaults.json", "menu.css", "menu.go", "menu.templ"}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != len(wantNames) {
		t.Fatal("private delivery inventory differs")
	}
	for i, entry := range entries {
		info, infoErr := entry.Info()
		if entry.Name() != wantNames[i] || infoErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			t.Fatal("private delivery name, type, or mode differs")
		}
	}
	if info, err := os.Stat(out); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatal("private delivery directory mode differs")
	}
	deliveredArtifact, err := os.ReadFile(filepath.Join(out, "artifact.json"))
	if err != nil || !bytes.Equal(deliveredArtifact, first) {
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
		t.Fatal("could not seed private drift canary")
	}
	invoke(1, "-check", out)
	if err := os.WriteFile(defaultsPath, defaults, 0o600); err != nil {
		t.Fatal("could not restore private drift canary")
	}
	invoke(0, "-check", out)
	t.Run("explicit-repair", func(t *testing.T) {
		repaired, err := captureAssetsCLI(t, "assets-adapt-menu", "-root", assets, "-repair")
		replayed, replayErr := captureAssetsCLI(t, "assets-adapt-menu", "-root", assets, "-repair")
		if err != nil || replayErr != nil || !bytes.Equal(repaired, replayed) {
			t.Fatal("repair JSON replay failed; private bodies withheld")
		}
		var enhanced designadapt.MenuRepairArtifact
		if err := json.Unmarshal(repaired, &enhanced); err != nil || enhanced.Adapter != "p07-menu-repair-v1" || enhanced.Verification != "not-verified" || enhanced.Mount.Gap != 8 || enhanced.Mount.Edge != 16 {
			t.Fatal("repair envelope or mount contract differs")
		}
		baseJSON, err := json.Marshal(enhanced.Base)
		if err != nil || !bytes.Equal(append(baseJSON, '\n'), first) || designadapt.VerifyMenuRepairReplay(enhanced, []byte(original.Content)) != nil {
			t.Fatal("repair lost original base byte identity or source replay")
		}
		selectedBase, err := captureAssetsCLI(t, "assets-adapt-menu", "-root", assets, "-repair=false")
		if err != nil || !bytes.Equal(selectedBase, first) {
			t.Fatal("explicit false changed default JSON")
		}
		repairOut := filepath.Join(parent, "repair-delivery")
		invoke(0, "-repair", "-out", repairOut)
		invoke(0, "-repair", "-check", repairOut)
		invoke(2, "-repair", "-out", repairOut)
		invoke(1, "-repair", "-check", out)
		invoke(1, "-check", repairOut)
		invoke(1, "-repair", "-check", filepath.Join(parent, "missing-repair"))
		names := []string{"USAGE.md", "artifact.json", "defaults.json", "menu.css", "menu.go", "menu.templ", "menu_repair.css", "menu_repair.mjs"}
		entries, err := os.ReadDir(repairOut)
		if err != nil || len(entries) != len(names) {
			t.Fatal("repair delivery inventory differs")
		}
		for i, entry := range entries {
			info, err := entry.Info()
			if entry.Name() != names[i] || err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
				t.Fatal("repair delivery name/type/mode differs")
			}
		}
		if info, err := os.Stat(repairOut); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
			t.Fatal("repair directory privacy differs")
		}
		for _, name := range []string{"menu.templ", "menu.go", "menu.css", "defaults.json"} {
			base, err := os.ReadFile(filepath.Join(out, name))
			copy, copyErr := os.ReadFile(filepath.Join(repairOut, name))
			if err != nil || copyErr != nil || !bytes.Equal(base, copy) {
				t.Fatalf("base source changed in repair delivery: %s", name)
			}
		}
		for name, want := range map[string][]byte{"artifact.json": repaired, "menu_repair.mjs": []byte(enhanced.Module), "menu_repair.css": []byte(enhanced.CSS)} {
			file := filepath.Join(repairOut, name)
			body, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(body, want) {
				t.Fatal("repair bytes differ from JSON")
			}
			if err := os.WriteFile(file, append(append([]byte(nil), body...), '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			invoke(1, "-repair", "-check", repairOut)
			if err := os.WriteFile(file, body, 0o600); err != nil {
				t.Fatal(err)
			}
			invoke(0, "-repair", "-check", repairOut)
		}
		invoke(0, "-check", out)
		t.Logf("P07 explicit repair JSON=%d bytes; eight private files and unchanged base proven", len(repaired))
	})
	t.Logf("P07 CLI JSON=%d bytes, delivery=%d files; paid bodies withheld and source preservation registered", len(first), len(entries))
}

func mustDigest(t *testing.T, encoded string) [32]byte {
	t.Helper()
	var digest [32]byte
	if len(encoded) != len(digest)*2 {
		t.Fatal("invalid digest length")
	}
	for i := range digest {
		high, low := fromHex(encoded[i*2]), fromHex(encoded[i*2+1])
		if high < 0 || low < 0 {
			t.Fatal("invalid digest encoding")
		}
		digest[i] = byte(high<<4 | low)
	}
	return digest
}

func fromHex(value byte) int {
	switch {
	case value >= '0' && value <= '9':
		return int(value - '0')
	case value >= 'a' && value <= 'f':
		return int(value-'a') + 10
	default:
		return -1
	}
}
