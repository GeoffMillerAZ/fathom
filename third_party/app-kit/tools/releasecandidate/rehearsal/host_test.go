package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

func TestHostModuleRemovesOnlyKnownDevelopmentReplacement(t *testing.T) {
	version := "v0.0.0-rehearsal.h" + strings.Repeat("a", 64)
	original := "module " + hostModule + "\n\ngo 1.26.2\nrequire " + candidateModule + " v0.0.0\nreplace " + candidateModule + " => ../..\n"
	body, err := prepareHostModule([]byte(original), version)
	if err != nil {
		t.Fatal(err)
	}
	mod, err := modfile.Parse("go.mod", body, nil)
	if err != nil || len(mod.Replace) != 0 || len(mod.Require) != 1 || mod.Require[0].Mod.Version != version {
		t.Fatalf("not an exact standalone selection: %s; %v", body, err)
	}
	for name, source := range map[string]string{
		"other module":        strings.Replace(original, hostModule, "example.org/other", 1),
		"other source":        strings.Replace(original, "=> ../..", "=> /tmp/other", 1),
		"version replacement": strings.Replace(original, "=> ../..", "=> example.org/other v1.0.0", 1),
		"extra replacement":   original + "replace example.org/dep => /tmp/dep\n",
		"absent replacement":  strings.Split(original, "replace ")[0],
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := prepareHostModule([]byte(source), version); err == nil {
				t.Fatal("unsafe module accepted")
			}
		})
	}
	for _, version := range []string{"", "latest", "v1.0.0", "v0.0.0-rehearsal.h" + strings.Repeat("A", 64)} {
		if _, err := prepareHostModule([]byte(original), version); err == nil {
			t.Fatalf("invalid version accepted: %q", version)
		}
	}
}

func TestHostSnapshotSelectionIsBounded(t *testing.T) {
	for _, name := range []string{"main.go", "go.mod", "go.sum", "Info.plist", "cmd/prepare-wails/main.go", "internal/hostwire/wire_test.go", "patches/example.patch"} {
		if !hostSourceName(name) {
			t.Fatalf("expected source refused: %s", name)
		}
	}
	for _, name := range []string{"../main.go", "/main.go", "data/paid.go", "nested/go.mod", ".env", ".private.go", "gallery-host", "cmd/prepare-wails/nested/main.go", "a\\b.go"} {
		if hostSourceName(name) {
			t.Fatalf("unexpected source admitted: %s", name)
		}
	}
}

func TestHostBundleSourceDirectoriesAreExact(t *testing.T) {
	for _, name := range []string{
		"cmd/bundle-launcher/main.go",
		"cmd/bundle-launcher/exec_darwin.go",
		"cmd/prepare-bundle/main.go",
		"internal/macbundle/bundle.go",
	} {
		if !hostSourceName(name) {
			t.Fatalf("reviewed bundle source refused: %s", name)
		}
	}
	for _, name := range []string{
		"cmd/bundle-launchers/main.go",
		"cmd/bundle-launcher/nested/main.go",
		"cmd/bundle-launcher/.hidden.go",
		"cmd/prepare-bundlex/main.go",
		"internal/macbundlex/bundle.go",
		"internal/macbundle/nested/bundle.go",
		"data/paid.go",
		"paid/raw.go",
		"cmd/unknown/main.go",
	} {
		if hostSourceName(name) {
			t.Fatalf("neighboring, nested, hidden, paid, or unknown source admitted: %s", name)
		}
	}
}

func TestHostBundleSourceSnapshotRetainsExactGitVisibleBytes(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                    "module example.org/host\n",
		"go.sum":                    "sum\n",
		"main.go":                   "package main\n",
		"host.go":                   "package main\n",
		"host_test.go":              "package main\n",
		"native.go":                 "package main\n",
		"cmd/prepare-wails/main.go": "package main\n",
		"patches/wails-v3.0.0-beta.21-http-only-origin.patch": "origin\n",
		"patches/wails-v3.0.0-beta.21-media-denial.patch":     "media\n",
		"cmd/bundle-launcher/main.go":                         "package main // launcher\n",
		"cmd/bundle-launcher/exec_darwin.go":                  "package main // exec\n",
		"cmd/prepare-bundle/main.go":                          "package main // bundle\n",
		"internal/macbundle/bundle.go":                        "package macbundle\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	got, records, err := readHostSource(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(files) || len(records) != len(files) {
		t.Fatalf("snapshot sizes = %d/%d, want %d", len(got), len(records), len(files))
	}
	for name, body := range files {
		if string(got[name]) != body {
			t.Fatalf("snapshot changed %s", name)
		}
		found := false
		for _, record := range records {
			if record.Path == name {
				found = record.Bytes == int64(len(body)) && record.SHA256 == hashBytes([]byte(body))
			}
		}
		if !found {
			t.Fatalf("missing exact inventory identity for %s", name)
		}
	}
	unexpected := filepath.Join(root, "cmd", "bundle-launcher", "nested", "paid.go")
	if err := os.MkdirAll(filepath.Dir(unexpected), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unexpected, []byte("package paid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readHostSource(root); err == nil || !strings.Contains(err.Error(), `unexpected host source path "cmd/bundle-launcher/nested/paid.go"`) {
		t.Fatalf("unexpected path did not fail closed: %v", err)
	}
}

func TestHostEvidenceWriteDoesNotClobber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	if err := hostWrite(path, []byte("original")); err != nil {
		t.Fatal(err)
	}
	if err := hostWrite(path, []byte("replacement")); err == nil {
		t.Fatal("overwrote retained evidence")
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "original" {
		t.Fatalf("original changed: %q %v", body, err)
	}
}

func TestNativeModuleUsesStablePathWithoutErasingOriginalSelection(t *testing.T) {
	const wails = "github.com/wailsapp/wails/v3"
	const originalPath = "/private/run/random-stage/wails"
	source := "module " + hostModule + "\nrequire " + candidateModule + " v0.0.0-rehearsal.h" + strings.Repeat("a", 64) + "\nreplace " + wails + " => " + originalPath + "\n"
	body, err := hostNativeModule([]byte(source), originalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "=> ../patched-wails") || !strings.Contains(string(body), "v0.0.0-rehearsal.h"+strings.Repeat("a", 64)) {
		t.Fatalf("lost stable path or exact kit: %s", body)
	}
	for _, bad := range []string{
		strings.Replace(source, originalPath, "/unexpected/path", 1),
		source + "replace example.org/dep => /tmp/source\n",
		strings.Replace(source, wails, candidateModule, 1),
	} {
		if _, err := hostNativeModule([]byte(bad), originalPath); err == nil {
			t.Fatal("unverified replacement admitted")
		}
	}
}

func TestNativeMinimumOSChecksLinkedBinaryNotRequestedFlags(t *testing.T) {
	good := "Load command 7\ncmd LC_BUILD_VERSION\ncmdsize 32\nplatform 1\nminos 14.0\nsdk 27.0\n"
	if err := checkNativeMinimumOS(good); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "MACOSX_DEPLOYMENT_TARGET=14.0", strings.Replace(good, "minos 14.0", "minos 11.0", 1), strings.Replace(good, "minos 14.0", "sdk 14.0", 1), good + good} {
		if err := checkNativeMinimumOS(bad); err == nil {
			t.Fatalf("invalid linked minimum passed: %q", bad)
		}
	}
}
