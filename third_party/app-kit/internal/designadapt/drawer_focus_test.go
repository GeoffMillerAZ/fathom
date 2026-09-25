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

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

func syntheticDrawerFocus(t *testing.T) (DrawerArtifact, DrawerFocusArtifact, []byte) {
	t.Helper()
	body := []byte(syntheticDrawer)
	base, err := adaptDrawer(syntheticSource(syntheticDrawer), body)
	if err != nil {
		t.Fatal(err)
	}
	a, err := newDrawerFocusArtifact(base)
	if err != nil {
		t.Fatal(err)
	}
	return base, a, body
}

func TestDrawerFocusDeterminismAndBaseIsolation(t *testing.T) {
	base, a, body := syntheticDrawerFocus(t)
	b, err := newDrawerFocusArtifact(base)
	if err != nil || !reflect.DeepEqual(a, b) || verifyDrawerFocusReplayWith(a, body, adaptDrawer) != nil {
		t.Fatal("focus artifact generation/replay differs")
	}
	if a.Adapter != "p08-drawer-focus-v1" || a.SchemaVersion != 1 || a.Verification != "not-verified" || a.WrapperAttribute != "data-appkit-drawer-focus" || !reflect.DeepEqual(a.Base, base) {
		t.Fatal("focus variant lost its explicit boundary")
	}
	digest := func(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
	if a.RuleSHA256 != digest(drawerFocusRule+"\x00"+drawerFocusSchema) || a.CSSSHA256 != digest(drawerFocusCSS) {
		t.Fatal("companion identity differs")
	}
	before, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = newDrawerFocusArtifact(base)
	after, err := json.Marshal(base)
	if err != nil || !bytes.Equal(before, after) || strings.Contains(base.Templ+base.Model, a.WrapperAttribute) {
		t.Fatal("repair changed base output or enabled its wrapper implicitly")
	}
	changed := base
	changed.Adapter = "unknown"
	if _, err := newDrawerFocusArtifact(changed); err == nil {
		t.Fatal("CUE contract accepted an unsupported nested base")
	}
}

func TestDrawerFocusReplayRejectsMutation(t *testing.T) {
	_, a, body := syntheticDrawerFocus(t)
	for _, field := range []string{"Adapter", "RuleSHA256", "CSSSHA256", "CSS", "WrapperAttribute", "Verification"} {
		t.Run(field, func(t *testing.T) {
			changed := a
			v := reflect.ValueOf(&changed).Elem().FieldByName(field)
			v.SetString(v.String() + "x")
			if verifyDrawerFocusReplayWith(changed, body, adaptDrawer) == nil {
				t.Fatal("mutated field accepted")
			}
		})
	}
	for name, mutate := range map[string]func(*DrawerFocusArtifact){
		"schema":   func(a *DrawerFocusArtifact) { a.SchemaVersion++ },
		"base":     func(a *DrawerFocusArtifact) { a.Base.Adapter += "-changed" },
		"defaults": func(a *DrawerFocusArtifact) { a.Base.Defaults = append(bytes.Clone(a.Base.Defaults), ' ') },
		"runtime":  func(a *DrawerFocusArtifact) { a.Base.Runtime.ESMSHA256 = strings.Repeat("0", 64) },
		"oversize": func(a *DrawerFocusArtifact) { a.CSS = strings.Repeat("x", (16<<10)+1) },
		"rehashed-css": func(a *DrawerFocusArtifact) {
			a.CSS += "\n/* changed */"
			sum := sha256.Sum256([]byte(a.CSS))
			a.CSSSHA256 = hex.EncodeToString(sum[:])
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := a
			mutate(&changed)
			if verifyDrawerFocusReplayWith(changed, body, adaptDrawer) == nil {
				t.Fatal("mutated artifact accepted")
			}
		})
	}
	if verifyDrawerFocusReplayWith(a, append(bytes.Clone(body), 'x'), adaptDrawer) == nil {
		t.Fatal("source byte drift accepted")
	}
}

func TestDrawerFocusCSSIsScopedPaintOnly(t *testing.T) {
	css := drawerFocusCSS
	// A bounded source assertion for this flat authored stylesheet, not a CSS parser.
	for strings.Contains(css, "/*") {
		start := strings.Index(css, "/*")
		end := strings.Index(css[start+2:], "*/")
		if end < 0 {
			t.Fatal("unterminated stylesheet comment")
		}
		css = css[:start] + css[start+2+end+2:]
	}
	flatCSS := strings.ReplaceAll(css, "@media (prefers-color-scheme: dark) {", "")
	for _, block := range strings.Split(flatCSS, "}") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		selector, _, ok := strings.Cut(block, "{")
		if !ok {
			t.Fatal("malformed stylesheet block")
		}
		for _, part := range strings.Split(selector, ",") {
			if !strings.HasPrefix(strings.TrimSpace(part), "[data-appkit-drawer-focus]") {
				t.Fatal("unscoped repair selector")
			}
		}
	}
	for _, want := range []string{"outline: 2px solid currentColor", "outline-offset: -2px", "min-block-size: 44px", "min-inline-size: 44px", "overflow-wrap: anywhere", "display: contents", "el-dropdown > button:focus-visible", "outline-color: var(--color-gray-900)", "outline-color: var(--color-gray-100)", "@media (prefers-color-scheme: dark)"} {
		if !strings.Contains(css, want) {
			t.Fatalf("paint contract lost %q", want)
		}
	}
	for _, forbidden := range []string{"!important", "@import", "url(", "position:", "z-index:", "overflow: hidden", "color: inherit", `command="close"`} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("repair changed ownership/closure: %q", forbidden)
		}
	}
}

func TestDrawerFocusDeliveryInventoryAndDrift(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("secure reader supports macOS/Linux")
	}
	base, a, body := syntheticDrawerFocus(t)
	files, err := drawerFocusDeliveryFiles(a)
	if err != nil {
		t.Fatal(err)
	}
	baseFiles, err := drawerDeliveryFiles(base)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"drawer.templ", "drawer.go", "defaults.json", "drawer_focus.css", "USAGE.md", "artifact.json"}
	if len(files) != len(want) {
		t.Fatal("repair inventory differs")
	}
	for i, f := range files {
		if f.name != want[i] || i < 3 && !bytes.Equal(f.body, baseFiles[i].body) {
			t.Fatal("repair reordered inventory or changed base component/defaults")
		}
	}
	if !bytes.Equal(files[3].body, []byte(a.CSS)) {
		t.Fatal("delivered CSS differs from envelope")
	}
	for _, required := range []string{"load drawer_focus.css after source CSS", "<div data-appkit-drawer-focus>", "Do not wrap", "DrawerReference()", "No new constructor", "runtime loading fails", "Elements continues to own", "five-file export", "six-file inventory", "never overwritten", "artifact.json is written last"} {
		if !strings.Contains(string(files[4].body), required) {
			t.Fatalf("usage omits %q", required)
		}
	}
	dir := filepath.Join(t.TempDir(), "repair")
	if err := writeDrawerFocusDeliveryWith(dir, a, body, adaptDrawer); err != nil {
		t.Fatal(err)
	}
	check := func() error { return checkDrawerFocusDeliveryWith(dir, a, body, adaptDrawer) }
	if check() != nil || writeDrawerFocusDeliveryWith(dir, a, body, adaptDrawer) == nil {
		t.Fatal("repair replay/no-overwrite boundary failed")
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("repair parent is not private")
	}
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatalf("repair member is not private: %s", f.name)
		}
		if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(check(), ErrDrawerFocusDeliveryDrift) {
			t.Fatalf("member drift not classified: %s", f.name)
		}
		if err := os.WriteFile(path, f.body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(checkPrivateDelivery(dir, baseFiles, "drawer delivery", ErrDrawerDeliveryDrift), ErrDrawerDeliveryDrift) {
		t.Fatal("repair silently passed a base check")
	}
	baseDir := filepath.Join(t.TempDir(), "base")
	if err := writePrivateDelivery(baseDir, baseFiles, "drawer delivery"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(checkDrawerFocusDeliveryWith(baseDir, a, body, adaptDrawer), ErrDrawerFocusDeliveryDrift) {
		t.Fatal("base silently passed a repair check")
	}
	if err := os.Chmod(filepath.Join(dir, "drawer_focus.css"), 0640); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(check(), ErrDrawerFocusDeliveryDrift) {
		t.Fatal("mode drift not classified")
	}
}

func TestDrawerFocusReplayBeforeIO(t *testing.T) {
	_, a, body := syntheticDrawerFocus(t)
	dir := filepath.Join(t.TempDir(), "must-not-exist")
	if WriteDrawerFocusDelivery(dir, a, body) == nil || CheckDrawerFocusDelivery(dir, a, body) == nil {
		t.Fatal("public API accepted synthetic source")
	}
	a.Verification = "verified"
	if writeDrawerFocusDeliveryWith(dir, a, body, adaptDrawer) == nil || checkDrawerFocusDeliveryWith(dir, a, body, adaptDrawer) == nil {
		t.Fatal("unsupported certification accepted")
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("replay refusal touched filesystem")
	}
}

func TestDrawerFocusLiveReplay(t *testing.T) {
	kit := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kit == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kit) {
		t.Fatal("absolute private checkout required")
	}
	original, err := designassets.ReadFile(filepath.Join(kit, "data", "design-assets"), DrawerArchive, DrawerMember)
	if err != nil {
		t.Fatal("authorized original unavailable")
	}
	body := []byte(original.Content)
	source := designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(body)}
	a, err := AdaptDrawerFocus(source, body)
	if err != nil || VerifyDrawerFocusReplay(a, body) != nil {
		t.Fatal("private original replay failed; source withheld")
	}
	base, err := AdaptDrawer(source, body)
	if err != nil || !reflect.DeepEqual(base, a.Base) {
		t.Fatal("repair nested base differs from independent source replay")
	}
	parent := t.TempDir()
	baseDir, repairDir := filepath.Join(parent, "base"), filepath.Join(parent, "repair")
	if WriteDrawerDelivery(baseDir, base, body) != nil || WriteDrawerFocusDelivery(repairDir, a, body) != nil {
		t.Fatal("private delivery failed")
	}
	if CheckDrawerDelivery(baseDir, base, body) != nil || CheckDrawerFocusDelivery(repairDir, a, body) != nil {
		t.Fatal("private exact checks failed")
	}
	if !errors.Is(CheckDrawerDelivery(repairDir, base, body), ErrDrawerDeliveryDrift) || !errors.Is(CheckDrawerFocusDelivery(baseDir, a, body), ErrDrawerFocusDeliveryDrift) {
		t.Fatal("private variants cross-checked")
	}
	for _, name := range []string{"drawer.templ", "drawer.go", "defaults.json"} {
		baseBytes, err := os.ReadFile(filepath.Join(baseDir, name))
		if err != nil {
			t.Fatal("private base output unavailable")
		}
		repairBytes, err := os.ReadFile(filepath.Join(repairDir, name))
		if err != nil || !bytes.Equal(baseBytes, repairBytes) {
			t.Fatal("private repair changed typed source/defaults")
		}
	}
}
