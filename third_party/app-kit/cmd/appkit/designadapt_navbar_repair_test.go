package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designadapt"
)

func TestAssetsAdaptNavbarRepairSelectionRefusals(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	for _, flags := range [][]string{
		{"-repair"}, {"-runtime", ""}, {"-runtime", filepath.Join(root, "missing")},
		{"-repair=false", "-runtime", filepath.Join(root, "missing")},
		{"-repair", "-runtime=", "-out", out},
		{"-repair", "-runtime", "relative.js", "-out", out},
		{"-repair", "-runtime", filepath.Join(root, "missing"), "-out", out, "-check", out},
	} {
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-navbar", "-root", root}, flags...)...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatal("invalid runtime selection lost refusal/empty-output contract")
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("invalid selection created output")
	}
}

func TestAssetsAdaptNavbarRepairLive(t *testing.T) {
	kit := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kit == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kit) {
		t.Fatal("absolute private checkout required")
	}
	assets := filepath.Join(kit, "data/design-assets")
	runtime := filepath.Join(assets, "reference-assets/elements-1.0.22-pilot/elements.js")
	parent := t.TempDir()
	if requested := os.Getenv("APPKIT_PROBE_NAVBAR_DELIVERY_PARENT"); requested != "" {
		canonical, err := filepath.EvalSymlinks(requested)
		tmp, tmpErr := filepath.EvalSymlinks(os.TempDir())
		info, statErr := os.Lstat(requested)
		entries, listErr := os.ReadDir(requested)
		if !filepath.IsAbs(requested) || err != nil || tmpErr != nil || statErr != nil || listErr != nil || canonical != filepath.Clean(requested) || !strings.HasPrefix(canonical, tmp+string(filepath.Separator)) || info.Mode() != os.ModeDir|0700 || len(entries) != 0 {
			t.Fatal("retained output requires empty0700 real child of command TMPDIR")
		}
		parent = canonical
	}
	digest := func(body []byte) string {
		sum := sha256.Sum256(body)
		return hex.EncodeToString(sum[:])
	}
	before, err := directoryDeliveryFileSHA(runtime)
	if err != nil {
		t.Fatal("original runtime unavailable")
	}
	t.Cleanup(func() {
		if got, err := directoryDeliveryFileSHA(runtime); err != nil || got != before {
			t.Error("original runtime changed")
		}
	})
	baseJSON, err := captureAssetsCLI(t, "assets-adapt-navbar", "-root", assets)
	if err != nil || digest(baseJSON) != "5559fb79f3d06d312df0a6a7bafe216815b5f34499b28a804862291d3f8cb642" {
		t.Fatal("default JSON identity changed")
	}
	selection := []string{"assets-adapt-navbar", "-root", assets, "-repair", "-runtime", runtime}
	first, firstErr := captureAssetsCLI(t, selection...)
	second, secondErr := captureAssetsCLI(t, selection...)
	if firstErr != nil || secondErr != nil || len(first) == 0 || !bytes.Equal(first, second) {
		t.Fatal("repair JSON replay failed; private bodies withheld")
	}
	var a designadapt.NavbarRepairDeliveryArtifact
	if err := json.Unmarshal(first, &a); err != nil || a.Adapter != "p09-navbar-repair-delivery-v1" || a.Verification != "not-verified" {
		t.Fatal("repair JSON contract invalid")
	}
	if a.Runtime.RepairSHA256 != "77a5a32f909eeb1403be5eff43525d0f581d983d070e612ec6678b9b9532f581" || a.Runtime.RuntimeSHA256 != "7c7184f9ea5ba3e4842f8624f500cb80d81c3c8621570715cfbd69ecdf1d7179" {
		t.Fatal("paired browser-tested companions drifted")
	}
	invoke := func(want int, selected bool, flags ...string) {
		t.Helper()
		args := []string{"assets-adapt-navbar", "-root", assets}
		if selected {
			args = append(args, "-repair", "-runtime", runtime)
		}
		body, err := captureAssetsCLI(t, append(args, flags...)...)
		if command.Code(err) != want || len(body) != 0 {
			t.Fatalf("repair dispatch exit/output: got%d/%d, want%d/0", command.Code(err), len(body), want)
		}
	}
	baseDir, repairDir := filepath.Join(parent, "base"), filepath.Join(parent, "repair")
	invoke(0, false, "-out", baseDir)
	invoke(0, true, "-out", repairDir)
	invoke(0, true, "-check", repairDir)
	invoke(2, true, "-out", repairDir)
	invoke(0, true, "-check", repairDir)
	invoke(1, true, "-check", baseDir)
	invoke(1, false, "-check", repairDir)
	invoke(1, true, "-check", filepath.Join(parent, "missing"))
	invoke(2, true, "-runtime", filepath.Join(repairDir, "elements-navbar.js"), "-out", filepath.Join(parent, "refused"))
	if _, err := os.Lstat(filepath.Join(parent, "refused")); !os.IsNotExist(err) {
		t.Fatal("derivative-as-original refusal created output")
	}
	wantNames := []string{"USAGE.md", "artifact.json", "defaults.json", "elements-navbar.js", "navbar.go", "navbar.templ", "navbar_repair.css", "navbar_routes.go", "navbar_routes.templ"}
	entries, err := os.ReadDir(repairDir)
	if err != nil || len(entries) != len(wantNames) {
		t.Fatal("nine-file delivery inventory differs")
	}
	files := map[string]string{}
	for i, entry := range entries {
		info, err := entry.Info()
		if err != nil || entry.Name() != wantNames[i] || info.Mode() != 0600 {
			t.Fatal("private delivery name/mode differs")
		}
		body, err := os.ReadFile(filepath.Join(repairDir, entry.Name()))
		if err != nil {
			t.Fatal("delivered input unavailable")
		}
		files[entry.Name()] = digest(body)
	}
	if files["artifact.json"] != digest(first) || files["elements-navbar.js"] != a.Runtime.RuntimeSHA256 {
		t.Fatal("CLI JSON/runtime differs from delivery")
	}
	for _, name := range []string{"navbar.go", "navbar.templ", "defaults.json"} {
		base, err := os.ReadFile(filepath.Join(baseDir, name))
		if err != nil || digest(base) != files[name] {
			t.Fatal("base input changed in repair delivery")
		}
	}
	defaultsPath := filepath.Join(repairDir, "defaults.json")
	defaults, err := os.ReadFile(defaultsPath)
	if err != nil {
		t.Fatal(err)
	}
	if os.WriteFile(defaultsPath, append(bytes.Clone(defaults), '\n'), 0600) != nil {
		t.Fatal("seed drift failed")
	}
	invoke(1, true, "-check", repairDir)
	if os.WriteFile(defaultsPath, defaults, 0600) != nil {
		t.Fatal("restore drift failed")
	}
	invoke(0, true, "-check", repairDir)
	invoke(0, false, "-check", baseDir)
	after, err := captureAssetsCLI(t, "assets-adapt-navbar", "-root", assets)
	if err != nil || !bytes.Equal(after, baseJSON) {
		t.Fatal("default JSON changed after repair operations")
	}
	receipt := map[string]any{"kind": "private-p09-navbar-repair-cli-delivery", "artifact_sha256": digest(first), "base_artifact_sha256": digest(baseJSON), "paired_repair_sha256": a.Runtime.RepairSHA256, "runtime_sha256": a.Runtime.RuntimeSHA256, "files": files, "verification": "not-verified", "standalone": "not-tested", "consumer": "not-adopted"}
	body, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil || os.WriteFile(filepath.Join(parent, "delivery-receipt.json"), append(body, '\n'), 0600) != nil {
		t.Fatal("retained delivery receipt failed")
	}
	t.Logf("private CLI delivery retained at %s", parent)
}
