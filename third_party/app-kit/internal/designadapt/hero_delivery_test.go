package designadapt

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func syntheticHeroDelivery(t *testing.T) (HeroArtifact, []heroDeliveryFile) {
	t.Helper()
	artifact := HeroArtifact{
		SchemaVersion: 1, Adapter: "synthetic-hero", Templ: "synthetic templ",
		Model: "synthetic model", CSS: "synthetic css", Defaults: json.RawMessage(`{"synthetic":true}`),
		Verification: "not-verified",
	}
	files, err := heroDeliveryFiles(artifact)
	if err != nil {
		t.Fatal(err)
	}
	return artifact, files
}

func TestHeroDeliveryFixedPrivateInventory(t *testing.T) {
	artifact, files := syntheticHeroDelivery(t)
	wantNames := []string{"hero.templ", "hero.go", "hero.css", "defaults.json", "USAGE.md", "artifact.json"}
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = file.name
	}
	if !reflect.DeepEqual(names, wantNames) || validatePrivateDeliveryFiles(files) != nil {
		t.Fatalf("hero delivery inventory differs: %v", names)
	}
	if !bytes.Equal(files[3].body, artifact.Defaults) {
		t.Fatal("defaults bytes changed")
	}
	for _, text := range []string{"private", "v0.3.1020", "Tailwind 4", "all six required URLs", "intentionally", "ImageDecorative", "NewHero preserves", "NewHeroEnhanced", "after source CSS", "does not download or bundle", "not a certified target"} {
		if !bytes.Contains(files[4].body, []byte(text)) {
			t.Fatalf("usage boundary missing %q", text)
		}
	}

	parent := t.TempDir()
	for _, leaf := range []string{"first", "second"} {
		dir := filepath.Join(parent, leaf)
		if err := writeHeroDelivery(dir, files); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(dir)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal("output directory not private")
		}
		for _, file := range files {
			body, err := os.ReadFile(filepath.Join(dir, file.name))
			if err != nil || !bytes.Equal(body, file.body) {
				t.Fatalf("delivered %s differs", file.name)
			}
			info, err := os.Stat(filepath.Join(dir, file.name))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("delivered %s is not private", file.name)
			}
		}
		if err := writeHeroDelivery(dir, files); err == nil {
			t.Fatal("existing delivery overwritten")
		}
	}
}

func TestHeroDeliveryRejectsInvalidArtifactBeforeIO(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "must-not-exist")
	if WriteHeroDelivery(dir, HeroArtifact{}, nil) == nil || CheckHeroDelivery(dir, HeroArtifact{}, nil) == nil {
		t.Fatal("invalid artifact passed source replay")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid artifact caused filesystem I/O")
	}
}

func TestHeroDeliveryRefusesExistingOrLinkedDestinations(t *testing.T) {
	_, files := syntheticHeroDelivery(t)
	parent := t.TempDir()
	real := filepath.Join(parent, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"", real, link, link + "/.", filepath.Join(link, "child"), filepath.Join(parent, "missing", "child"), parent, filepath.Join(parent, ".")} {
		if err := writeHeroDelivery(dir, files); err == nil {
			t.Fatalf("accepted invalid output %q", dir)
		}
	}
	entries, err := os.ReadDir(real)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid export changed existing destination")
	}
}

func TestHeroDeliveryDetectsFileAndRootDrift(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("secure reader supports macOS/Linux")
	}
	_, files := syntheticHeroDelivery(t)
	for _, file := range files {
		for _, variant := range []string{"missing", "same-size-edit", "larger", "directory", "symlink", "hardlink", "public-mode", "executable-mode", "setuid-mode", "sticky-mode"} {
			t.Run(file.name+"/"+variant, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "delivery")
				if err := writeHeroDelivery(dir, files); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, file.name)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				var err error
				switch variant {
				case "same-size-edit":
					body := bytes.Clone(file.body)
					body[0] ^= 1
					err = os.WriteFile(path, body, 0600)
				case "larger":
					err = os.WriteFile(path, append(bytes.Clone(file.body), 'x'), 0600)
				case "directory":
					err = os.Mkdir(path, 0700)
				case "symlink":
					err = os.Symlink("USAGE.md", path)
				case "hardlink":
					external := filepath.Join(t.TempDir(), "exact-copy")
					if err = os.WriteFile(external, file.body, 0600); err == nil {
						err = os.Link(external, path)
					}
				case "public-mode":
					err = os.WriteFile(path, file.body, 0644)
				case "executable-mode":
					err = os.WriteFile(path, file.body, 0700)
				case "setuid-mode", "sticky-mode":
					mode := os.ModeSetuid
					if variant == "sticky-mode" {
						mode = os.ModeSticky
					}
					if err = os.WriteFile(path, file.body, 0600); err == nil {
						err = os.Chmod(path, mode|0600)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := checkHeroDelivery(dir, files); !errors.Is(err, ErrHeroDeliveryDrift) || errors.Is(err, ErrPricingDeliveryDrift) {
					t.Fatalf("file drift classified incorrectly: %v", err)
				}
			})
		}
	}
	for _, variant := range []string{"missing-root", "root-link", "root-mode", "extra"} {
		t.Run(variant, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "delivery")
			if variant != "missing-root" {
				if err := writeHeroDelivery(dir, files); err != nil {
					t.Fatal(err)
				}
			}
			var err error
			switch variant {
			case "root-link":
				link := filepath.Join(parent, "link")
				err = os.Symlink(dir, link)
				dir = link + "/."
			case "root-mode":
				err = os.Chmod(dir, 0755)
			case "extra":
				err = os.WriteFile(filepath.Join(dir, ".extra"), []byte("x"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := checkHeroDelivery(dir, files); !errors.Is(err, ErrHeroDeliveryDrift) || errors.Is(err, ErrPricingDeliveryDrift) {
				t.Fatalf("root drift classified incorrectly: %v", err)
			}
		})
	}
}

func TestHeroDeliveryPartialWriteRetainsCollisionWithoutCompletion(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("secure reader supports macOS/Linux")
	}
	_, files := syntheticHeroDelivery(t)
	parent := t.TempDir()
	dir := filepath.Join(parent, "partial")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(dir, "hero.css")
	if err := os.WriteFile(collision, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := writeHeroFiles(root, "partial", files); err == nil {
		t.Fatal("file collision accepted")
	}
	body, err := os.ReadFile(collision)
	if err != nil || string(body) != "preserve" {
		t.Fatal("collision overwritten")
	}
	if _, err := os.Stat(filepath.Join(dir, "hero.templ")); err != nil {
		t.Fatal("partial output removed unexpectedly")
	}
	if _, err := os.Stat(filepath.Join(dir, "artifact.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completion marker created after failure")
	}
	if err := checkHeroDelivery(dir, files); !errors.Is(err, ErrHeroDeliveryDrift) {
		t.Fatal("partial output passed")
	}
}
