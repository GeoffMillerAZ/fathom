package designadapt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func syntheticMenuRepair(t *testing.T) (MenuArtifact, MenuRepairArtifact, []byte) {
	t.Helper()
	body := []byte(syntheticMenu)
	base, err := adaptMenu(syntheticSource(syntheticMenu), body)
	if err != nil {
		t.Fatal(err)
	}
	return base, newMenuRepairArtifact(base), body
}

func TestMenuRepairArtifactDeterminismAndReplay(t *testing.T) {
	base, first, body := syntheticMenuRepair(t)
	second := newMenuRepairArtifact(base)
	if !reflect.DeepEqual(first, second) || verifyMenuRepairReplayWith(first, body, adaptMenu) != nil {
		t.Fatal("repair generation or replay differs")
	}
	if first.SchemaVersion != 1 || first.Adapter != "p07-menu-repair-v1" || first.Verification != "not-verified" || first.Mount.Gap != 8 || first.Mount.Edge != 16 {
		t.Fatal("repair envelope metadata differs")
	}
	if !reflect.DeepEqual(first.Base, base) {
		t.Fatal("nested base differs")
	}
	digest := func(value string) string {
		sum := sha256.Sum256([]byte(value))
		return hex.EncodeToString(sum[:])
	}
	if first.RuleSHA256 != digest(menuRepairRule) || first.ModuleSHA256 != digest(menuRepairModule) || first.CSSSHA256 != digest(menuRepairCSS) {
		t.Fatal("repair implementation identities differ")
	}
	before, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	_ = newMenuRepairArtifact(base)
	after, err := json.Marshal(base)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("repair generation mutated base")
	}
}

func TestMenuRepairReplayRefusesEveryMutation(t *testing.T) {
	_, artifact, body := syntheticMenuRepair(t)
	mutations := map[string]func(*MenuRepairArtifact){
		"schema":       func(a *MenuRepairArtifact) { a.SchemaVersion++ },
		"adapter":      func(a *MenuRepairArtifact) { a.Adapter += "-unknown" },
		"rule-hash":    func(a *MenuRepairArtifact) { a.RuleSHA256 = strings.Repeat("0", 64) },
		"module-hash":  func(a *MenuRepairArtifact) { a.ModuleSHA256 = strings.Repeat("0", 64) },
		"css-hash":     func(a *MenuRepairArtifact) { a.CSSSHA256 = strings.Repeat("0", 64) },
		"module":       func(a *MenuRepairArtifact) { a.Module += "\n// changed" },
		"css":          func(a *MenuRepairArtifact) { a.CSS += "\n/* changed */" },
		"gap":          func(a *MenuRepairArtifact) { a.Mount.Gap++ },
		"edge":         func(a *MenuRepairArtifact) { a.Mount.Edge++ },
		"verification": func(a *MenuRepairArtifact) { a.Verification = "verified" },
		"base":         func(a *MenuRepairArtifact) { a.Base.Adapter += "-changed" },
		"defaults":     func(a *MenuRepairArtifact) { a.Base.Defaults = append(bytes.Clone(a.Base.Defaults), ' ') },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := artifact
			changed.Base.Defaults = bytes.Clone(artifact.Base.Defaults)
			mutate(&changed)
			if verifyMenuRepairReplayWith(changed, body, adaptMenu) == nil {
				t.Fatal("mutated repair artifact passed replay")
			}
		})
	}
	for name, mutate := range map[string]func(*MenuRepairArtifact){
		"rehashed-module": func(a *MenuRepairArtifact) {
			a.Module += "\n// changed"
			sum := sha256.Sum256([]byte(a.Module))
			a.ModuleSHA256 = hex.EncodeToString(sum[:])
		},
		"rehashed-css": func(a *MenuRepairArtifact) {
			a.CSS += "\n/* changed */"
			sum := sha256.Sum256([]byte(a.CSS))
			a.CSSSHA256 = hex.EncodeToString(sum[:])
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := artifact
			mutate(&changed)
			if verifyMenuRepairReplayWith(changed, body, adaptMenu) == nil {
				t.Fatal("rehashed repair mutation passed replay")
			}
		})
	}
	oversized := artifact
	oversized.Module = strings.Repeat("x", (64<<10)+1)
	if verifyMenuRepairReplayWith(oversized, body, adaptMenu) == nil {
		t.Fatal("oversized module passed replay")
	}
	oversized = artifact
	oversized.CSS = strings.Repeat("x", (16<<10)+1)
	if verifyMenuRepairReplayWith(oversized, body, adaptMenu) == nil {
		t.Fatal("oversized CSS passed replay")
	}
}

func TestMenuRepairDeliveryInventoryAndUsage(t *testing.T) {
	_, artifact, _ := syntheticMenuRepair(t)
	files, err := menuRepairDeliveryFiles(artifact)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"menu.templ", "menu.go", "menu.css", "defaults.json", "menu_repair.mjs", "menu_repair.css", "USAGE.md", "artifact.json"}
	got := make([]string, len(files))
	for i, file := range files {
		got[i] = file.name
	}
	if !reflect.DeepEqual(got, want) || files[len(files)-1].name != "artifact.json" {
		t.Fatalf("repair inventory differs: %v", got)
	}
	if !bytes.Equal(files[0].body, []byte(artifact.Base.Templ)) || !bytes.Equal(files[1].body, []byte(artifact.Base.Model)) || !bytes.Equal(files[2].body, []byte(artifact.Base.CSS)) || !bytes.Equal(files[3].body, artifact.Base.Defaults) || !bytes.Equal(files[4].body, []byte(artifact.Module)) || !bytes.Equal(files[5].body, []byte(artifact.CSS)) {
		t.Fatal("repair inventory body differs")
	}
	usage := string(files[6].body)
	for _, required := range []string{"scans both menu.templ and menu.go", "menu_repair.css last", `customElements.whenDefined("el-menu")`, "mountMenuRepair(root, { gap: 8, edge: 16 })", "disposeMenuRepair()", "no automatic bootstrap", "global installation or test handle", "NewMenuEnhanced renders the selected [data-appkit-menu]", "NewMenu remains the unmounted", "MenuReference remains the unmounted", "Serve MenuReference only", "form submission denied", "Do not nest a POST-bearing menu", "app-owned fallback", "authentication", "CSRF", "private", "All eight files"} {
		if !strings.Contains(usage, required) {
			t.Fatalf("usage omits %q", required)
		}
	}
	if strings.Contains(usage, "globalThis.") || strings.Contains(usage, "window.mountMenuRepair") {
		t.Fatal("usage installs a global mount")
	}
}

func TestMenuRepairDeliveryPrivateNoClobberDriftAndCrossVariant(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("secure reader supports macOS/Linux")
	}
	base, artifact, body := syntheticMenuRepair(t)
	repairFiles, err := menuRepairDeliveryFiles(artifact)
	if err != nil {
		t.Fatal(err)
	}
	baseFiles, err := menuDeliveryFiles(base)
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	repairDir := filepath.Join(parent, "repair")
	if err := writeMenuRepairDeliveryWith(repairDir, artifact, body, adaptMenu); err != nil {
		t.Fatal(err)
	}
	if err := writeMenuRepairDeliveryWith(repairDir, artifact, body, adaptMenu); err == nil {
		t.Fatal("existing repair delivery was overwritten")
	}
	if err := checkMenuRepairDeliveryWith(repairDir, artifact, body, adaptMenu); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(repairDir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatal("repair directory mode differs")
	}
	for _, file := range repairFiles {
		info, err := os.Stat(filepath.Join(repairDir, file.name))
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			t.Fatalf("repair file mode differs: %s", file.name)
		}
	}
	baseDir := filepath.Join(parent, "base")
	if err := writePrivateDelivery(baseDir, baseFiles, "menu delivery"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(checkPrivateDelivery(repairDir, baseFiles, "menu delivery", ErrMenuDeliveryDrift), ErrMenuDeliveryDrift) || !errors.Is(checkPrivateDelivery(baseDir, repairFiles, "menu repair delivery", ErrMenuRepairDeliveryDrift), ErrMenuRepairDeliveryDrift) {
		t.Fatal("base and repair inventories cross-checked")
	}
	for _, file := range repairFiles {
		t.Run("drift/"+file.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "delivery")
			if err := writePrivateDelivery(dir, repairFiles, "menu repair delivery"); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, file.name), []byte("changed"), 0o600); err != nil {
				t.Fatal(err)
			}
			if !errors.Is(checkPrivateDelivery(dir, repairFiles, "menu repair delivery", ErrMenuRepairDeliveryDrift), ErrMenuRepairDeliveryDrift) {
				t.Fatal("repair drift not classified")
			}
		})
	}
	modeDir := filepath.Join(t.TempDir(), "delivery")
	if err := writePrivateDelivery(modeDir, repairFiles, "menu repair delivery"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(modeDir, "menu_repair.mjs"), 0o640); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(checkPrivateDelivery(modeDir, repairFiles, "menu repair delivery", ErrMenuRepairDeliveryDrift), ErrMenuRepairDeliveryDrift) {
		t.Fatal("repair mode drift not classified")
	}
}

func TestMenuRepairReplayPrecedesFilesystemAccess(t *testing.T) {
	_, artifact, body := syntheticMenuRepair(t)
	changed := artifact
	changed.Adapter = "unknown"
	parent := t.TempDir()
	for _, operation := range []func(string) error{
		func(dir string) error { return writeMenuRepairDeliveryWith(dir, changed, body, adaptMenu) },
		func(dir string) error { return checkMenuRepairDeliveryWith(dir, changed, body, adaptMenu) },
	} {
		if err := operation(filepath.Join(parent, "must-not-exist")); err == nil {
			t.Fatal("invalid artifact accepted")
		}
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 0 {
			t.Fatal("invalid artifact reached filesystem")
		}
	}
	publicParent := t.TempDir()
	if err := WriteMenuRepairDelivery(filepath.Join(publicParent, "delivery"), artifact, body); err == nil {
		t.Fatal("public API accepted synthetic source")
	}
	if entries, err := os.ReadDir(publicParent); err != nil || len(entries) != 0 {
		t.Fatal("rejected public replay wrote output")
	}
}
