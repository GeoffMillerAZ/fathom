//go:build darwin || linux

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
	"strings"
	"syscall"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

func TestNavbarRepairDeliveryRuntimeOriginalRefusals(t *testing.T) {
	parent := t.TempDir()
	validUnknown := filepath.Join(parent, "unknown.js")
	if err := os.WriteFile(validUnknown, []byte("export {};\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadNavbarRuntimeOriginal("relative.js"); err == nil {
		t.Fatal("relative runtime path accepted")
	}

	tests := map[string]func() string{
		"missing": func() string { return filepath.Join(parent, "missing.js") },
		"directory": func() string {
			path := filepath.Join(parent, "directory")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			return path
		},
		"symlink": func() string {
			path := filepath.Join(parent, "symlink.js")
			if err := os.Symlink(validUnknown, path); err != nil {
				t.Fatal(err)
			}
			return path
		},
		"hardlink": func() string {
			source := filepath.Join(parent, "hardlink-source.js")
			if err := os.WriteFile(source, []byte("export {};\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(parent, "hardlink.js")
			if err := os.Link(source, path); err != nil {
				t.Fatal(err)
			}
			return path
		},
		"fifo": func() string {
			path := filepath.Join(parent, "runtime.fifo")
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		},
		"oversize": func() string {
			path := filepath.Join(parent, "oversize.js")
			if err := os.WriteFile(path, bytes.Repeat([]byte("x"), (256<<10)+1), 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		},
		"unknown-identity": func() string { return validUnknown },
	}
	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			if body, err := ReadNavbarRuntimeOriginal(setup()); err == nil || body != nil {
				t.Fatal("untrusted runtime input accepted")
			}
		})
	}
}

func TestNavbarRepairDeliveryRejectsInvalidReplayBeforeIO(t *testing.T) {
	parent := t.TempDir()
	artifact := NavbarRepairDeliveryArtifact{}
	for name, operation := range map[string]func(string) error{
		"write": func(dir string) error { return WriteNavbarRepairDelivery(dir, artifact, nil, nil) },
		"check": func(dir string) error { return CheckNavbarRepairDelivery(dir, artifact, nil, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(filepath.Join(parent, "must-not-exist-"+name)); err == nil || errors.Is(err, ErrNavbarRepairDeliveryDrift) {
				t.Fatalf("invalid replay classified as delivery state: %v", err)
			}
		})
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatal("invalid replay reached output filesystem")
	}
}

func TestNavbarRepairDeliveryLive(t *testing.T) {
	kit := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kit == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kit) {
		t.Fatal("absolute private checkout required")
	}
	designRoot := filepath.Join(kit, "data", "design-assets")
	original, err := designassets.ReadFile(designRoot, NavbarArchive, NavbarMember)
	if err != nil {
		t.Fatal("verified original unavailable")
	}
	body := []byte(original.Content)
	source := designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(body)}
	runtimePath := filepath.Join(designRoot, "reference-assets", "elements-1.0.22-pilot", "elements.js")
	runtimeBody, err := ReadNavbarRuntimeOriginal(runtimePath)
	if err != nil {
		t.Fatal("exact pinned runtime unavailable")
	}
	originalRuntime := bytes.Clone(runtimeBody)
	t.Cleanup(func() {
		got, err := os.ReadFile(runtimePath)
		if err != nil || !bytes.Equal(got, originalRuntime) {
			t.Error("original runtime changed")
		}
	})

	first, err := AdaptNavbarRepairDelivery(source, body, runtimeBody)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AdaptNavbarRepairDelivery(source, body, runtimeBody)
	if err != nil || !reflect.DeepEqual(first, second) || VerifyNavbarRepairDeliveryReplay(first, body, runtimeBody) != nil {
		t.Fatal("delivery replay is not deterministic")
	}
	digestJSON := func(v any) string {
		encoded, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(append(encoded, '\n'))
		return hex.EncodeToString(sum[:])
	}
	if digestJSON(first.Repair.Base) != "5559fb79f3d06d312df0a6a7bafe216815b5f34499b28a804862291d3f8cb642" ||
		digestJSON(first.Repair) != "77a5a32f909eeb1403be5eff43525d0f581d983d070e612ec6678b9b9532f581" ||
		first.Runtime.RuntimeSHA256 != "7c7184f9ea5ba3e4842f8624f500cb80d81c3c8621570715cfbd69ecdf1d7179" {
		t.Fatal("frozen base, repair or runtime identity drifted")
	}

	for name, mutate := range map[string]func(*NavbarRepairDeliveryArtifact){
		"envelope": func(a *NavbarRepairDeliveryArtifact) { a.RuleSHA256 = strings.Repeat("0", 64) },
		"repair-companion": func(a *NavbarRepairDeliveryArtifact) {
			a.Repair.CSS += "\n/* changed */"
			a.Repair.CSSSHA256 = navbarRuntimeDigest([]byte(a.Repair.CSS))
		},
		"runtime-companion": func(a *NavbarRepairDeliveryArtifact) {
			a.Runtime.Runtime += "\n/* changed */"
			a.Runtime.RuntimeSHA256 = navbarRuntimeDigest([]byte(a.Runtime.Runtime))
		},
		"nested-base": func(a *NavbarRepairDeliveryArtifact) { a.Repair.Base.Adapter += "-changed" },
	} {
		t.Run("replay-refuses-"+name, func(t *testing.T) {
			changed := first
			mutate(&changed)
			if VerifyNavbarRepairDeliveryReplay(changed, body, runtimeBody) == nil {
				t.Fatal("mutated or self-rehashed envelope accepted")
			}
		})
	}

	files, err := navbarRepairDeliveryFiles(first)
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{"navbar.templ", "navbar.go", "defaults.json", "navbar_repair.css", "navbar_routes.go", "navbar_routes.templ", "elements-navbar.js", "USAGE.md", "artifact.json"}
	if len(files) != len(wantNames) {
		t.Fatal("delivery inventory size differs")
	}
	for i, want := range wantNames {
		if files[i].name != want {
			t.Fatalf("delivery inventory differs at %d", i)
		}
	}

	parent := t.TempDir()
	dir := filepath.Join(parent, "repair")
	if err := WriteNavbarRepairDelivery(dir, first, body, runtimeBody); err != nil {
		t.Fatal(err)
	}
	if err := CheckNavbarRepairDelivery(dir, first, body, runtimeBody); err != nil {
		t.Fatal(err)
	}
	if err := WriteNavbarRepairDelivery(dir, first, body, runtimeBody); err == nil {
		t.Fatal("existing delivery overwritten")
	}
	if info, err := os.Stat(dir); err != nil || info.Mode() != os.ModeDir|0o700 {
		t.Fatal("delivery directory mode differs")
	}
	for _, file := range files {
		info, err := os.Stat(filepath.Join(dir, file.name))
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			t.Fatalf("delivery member mode differs: %s", file.name)
		}
	}

	for name, change := range map[string]func(string) error{
		"missing": func(path string) error { return os.Remove(filepath.Join(path, "navbar.go")) },
		"extra":   func(path string) error { return os.WriteFile(filepath.Join(path, "extra"), []byte("x"), 0o600) },
		"drift": func(path string) error {
			return os.WriteFile(filepath.Join(path, "USAGE.md"), []byte("changed"), 0o600)
		},
		"mode": func(path string) error { return os.Chmod(filepath.Join(path, "elements-navbar.js"), 0o640) },
	} {
		t.Run("delivery-"+name, func(t *testing.T) {
			changedDir := filepath.Join(t.TempDir(), "delivery")
			if err := WriteNavbarRepairDelivery(changedDir, first, body, runtimeBody); err != nil {
				t.Fatal(err)
			}
			if err := change(changedDir); err != nil {
				t.Fatal(err)
			}
			if err := CheckNavbarRepairDelivery(changedDir, first, body, runtimeBody); !errors.Is(err, ErrNavbarRepairDeliveryDrift) || errors.Is(err, ErrNavbarDeliveryDrift) {
				t.Fatalf("delivery drift classified incorrectly: %v", err)
			}
		})
	}

	baseFiles, err := navbarDeliveryFiles(first.Repair.Base)
	if err != nil {
		t.Fatal(err)
	}
	baseDir := filepath.Join(parent, "base")
	if err := writePrivateDelivery(baseDir, baseFiles, "navbar delivery"); err != nil {
		t.Fatal(err)
	}
	if err := CheckNavbarDelivery(dir, first.Repair.Base, body); !errors.Is(err, ErrNavbarDeliveryDrift) {
		t.Fatal("repair delivery silently passed base check")
	}
	if err := CheckNavbarRepairDelivery(baseDir, first, body, runtimeBody); !errors.Is(err, ErrNavbarRepairDeliveryDrift) {
		t.Fatal("base delivery silently passed repair check")
	}
}
