package designadapt

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func syntheticDirectoryDeliveries(t *testing.T) (DirectoryArtifact, DirectoryEnhancedArtifact, []byte) {
	t.Helper()
	body := []byte(syntheticDirectory(6))
	base, err := adaptDirectory(syntheticSource(string(body)), body)
	if err != nil {
		t.Fatal(err)
	}
	enhanced, err := enhanceDirectory(base)
	if err != nil {
		t.Fatal(err)
	}
	return base, enhanced, body
}

func directoryDeliveryInventory(t *testing.T) (DirectoryArtifact, DirectoryEnhancedArtifact, []privateDeliveryFile, []privateDeliveryFile) {
	t.Helper()
	base, enhanced, _ := syntheticDirectoryDeliveries(t)
	baseBefore, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	enhancedBefore, err := json.Marshal(enhanced)
	if err != nil {
		t.Fatal(err)
	}
	baseFiles, err := directoryDeliveryFiles(base)
	if err != nil {
		t.Fatal(err)
	}
	enhancedFiles, err := directoryEnhancedDeliveryFiles(enhanced)
	if err != nil {
		t.Fatal(err)
	}
	baseAfter, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	enhancedAfter, err := json.Marshal(enhanced)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(baseBefore, baseAfter) || !bytes.Equal(enhancedBefore, enhancedAfter) {
		t.Fatal("delivery inventory mutated an artifact")
	}
	return base, enhanced, baseFiles, enhancedFiles
}

func TestDirectoryDeliveryInventoriesAndPrivateDeterminism(t *testing.T) {
	base, enhanced, baseFiles, enhancedFiles := directoryDeliveryInventory(t)
	wantNames := [][]string{
		{"directory.templ", "directory.go", "defaults.json", "README.md", "artifact.json"},
		{"directory.templ", "directory.go", "directory_enhanced.templ", "directory_enhanced.go", "directory_enhanced.css", "defaults.json", "README.md", "artifact.json"},
	}
	for i, files := range [][]privateDeliveryFile{baseFiles, enhancedFiles} {
		got := make([]string, len(files))
		for j, item := range files {
			got[j] = item.name
		}
		if !reflect.DeepEqual(got, wantNames[i]) || files[len(files)-1].name != "artifact.json" {
			t.Fatalf("delivery inventory %d differs: %v", i, got)
		}
	}
	if !bytes.Equal(baseFiles[2].body, base.Defaults) || !bytes.Equal(enhancedFiles[5].body, enhanced.Base.Defaults) {
		t.Fatal("defaults bytes changed")
	}
	baseJSON, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	enhancedJSON, err := json.Marshal(enhanced)
	if err != nil {
		t.Fatal(err)
	}
	wantBodies := [][]byte{
		[]byte(base.Templ), []byte(base.Model), base.Defaults, []byte(directoryDeliveryReadme), append(baseJSON, '\n'),
		[]byte(enhanced.Base.Templ), []byte(enhanced.Base.Model), []byte(enhanced.Templ), []byte(enhanced.Model),
		[]byte(enhanced.CSS), enhanced.Base.Defaults, []byte(directoryEnhancedDeliveryReadme), append(enhancedJSON, '\n'),
	}
	for i, item := range append(append([]privateDeliveryFile{}, baseFiles...), enhancedFiles...) {
		if !bytes.Equal(item.body, wantBodies[i]) {
			t.Fatalf("inventory body differs: %s", item.name)
		}
	}

	baseBefore, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	enhancedBefore, err := json.Marshal(enhanced)
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	for kind, files := range map[string][]privateDeliveryFile{"base-a": baseFiles, "base-b": baseFiles, "enhanced-a": enhancedFiles, "enhanced-b": enhancedFiles} {
		dir := filepath.Join(parent, kind)
		if err := writePrivateDelivery(dir, files, "directory delivery"); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(dir)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatalf("%s directory is not private", kind)
		}
		for _, item := range files {
			got, err := os.ReadFile(filepath.Join(dir, item.name))
			if err != nil || !bytes.Equal(got, item.body) {
				t.Fatalf("%s/%s differs", kind, item.name)
			}
			info, err := os.Stat(filepath.Join(dir, item.name))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("%s/%s is not private", kind, item.name)
			}
		}
		if err := writePrivateDelivery(dir, files, "directory delivery"); err == nil {
			t.Fatalf("%s was overwritten", kind)
		}
	}
	baseAfter, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	enhancedAfter, err := json.Marshal(enhanced)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(baseBefore, baseAfter) || !bytes.Equal(enhancedBefore, enhancedAfter) {
		t.Fatal("delivery projection mutated an artifact")
	}
}

func TestDirectoryDeliveryRefusesExistingDestinations(t *testing.T) {
	_, _, files, _ := directoryDeliveryInventory(t)
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
		if err := writePrivateDelivery(dir, files, "directory delivery"); err == nil {
			t.Fatalf("accepted invalid output %q", dir)
		}
	}
	entries, err := os.ReadDir(real)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid exports changed existing destination")
	}
}

func TestDirectoryDeliveryDetectsEveryInventoryDrift(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("secure reader supports macOS/Linux")
	}
	_, _, baseFiles, enhancedFiles := directoryDeliveryInventory(t)
	for kind, files := range map[string][]privateDeliveryFile{"base": baseFiles, "enhanced": enhancedFiles} {
		for _, item := range files {
			for _, variant := range []string{"missing", "byte", "directory", "symlink", "hardlink", "mode"} {
				t.Run(kind+"/"+item.name+"/"+variant, func(t *testing.T) {
					dir := filepath.Join(t.TempDir(), "delivery")
					if err := writePrivateDelivery(dir, files, "directory delivery"); err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(dir, item.name)
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					var err error
					switch variant {
					case "byte":
						body := bytes.Clone(item.body)
						body[0] ^= 1
						err = os.WriteFile(path, body, 0600)
					case "directory":
						err = os.Mkdir(path, 0700)
					case "symlink":
						err = os.Symlink("README.md", path)
					case "hardlink":
						external := filepath.Join(t.TempDir(), "exact-copy")
						if err = os.WriteFile(external, item.body, 0600); err == nil {
							err = os.Link(external, path)
						}
					case "mode":
						err = os.WriteFile(path, item.body, 0644)
					}
					if err != nil {
						t.Fatal(err)
					}
					err = checkPrivateDelivery(dir, files, "directory delivery", ErrDirectoryDeliveryDrift)
					if !errors.Is(err, ErrDirectoryDeliveryDrift) || errors.Is(err, ErrPricingDeliveryDrift) {
						t.Fatalf("drift classified incorrectly: %v", err)
					}
				})
			}
		}
	}
	for _, variant := range []string{"missing-root", "root-link", "root-mode", "extra"} {
		t.Run(variant, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "delivery")
			if variant != "missing-root" {
				if err := writePrivateDelivery(dir, baseFiles, "directory delivery"); err != nil {
					t.Fatal(err)
				}
			}
			var err error
			switch variant {
			case "root-link":
				link := filepath.Join(t.TempDir(), "link")
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
			err = checkPrivateDelivery(dir, baseFiles, "directory delivery", ErrDirectoryDeliveryDrift)
			if !errors.Is(err, ErrDirectoryDeliveryDrift) || errors.Is(err, ErrPricingDeliveryDrift) {
				t.Fatalf("root drift classified incorrectly: %v", err)
			}
		})
	}
}

func TestDirectoryDeliveryRefusesWrongInventory(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("secure reader supports macOS/Linux")
	}
	_, _, baseFiles, enhancedFiles := directoryDeliveryInventory(t)
	baseDir := filepath.Join(t.TempDir(), "base")
	enhancedDir := filepath.Join(t.TempDir(), "enhanced")
	if err := writePrivateDelivery(baseDir, baseFiles, "directory delivery"); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateDelivery(enhancedDir, enhancedFiles, "directory delivery"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dir   string
		files []privateDeliveryFile
	}{{baseDir, enhancedFiles}, {enhancedDir, baseFiles}} {
		err := checkPrivateDelivery(tc.dir, tc.files, "directory delivery", ErrDirectoryDeliveryDrift)
		if !errors.Is(err, ErrDirectoryDeliveryDrift) || errors.Is(err, ErrPricingDeliveryDrift) {
			t.Fatalf("wrong inventory classified incorrectly: %v", err)
		}
	}
}

func TestDirectoryDeliveryPartialWriteHasNoCompletionMarker(t *testing.T) {
	_, _, baseFiles, enhancedFiles := directoryDeliveryInventory(t)
	for kind, files := range map[string][]privateDeliveryFile{"base": baseFiles, "enhanced": enhancedFiles} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "partial")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			collision := filepath.Join(dir, files[1].name)
			if err := os.WriteFile(collision, []byte("preserve"), 0600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(parent)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if err := writePrivateFiles(root, "partial", files); err == nil {
				t.Fatal("file collision accepted")
			}
			body, err := os.ReadFile(collision)
			if err != nil || string(body) != "preserve" {
				t.Fatal("collision overwritten")
			}
			if _, err := os.Stat(filepath.Join(dir, files[0].name)); err != nil {
				t.Fatal("partial output removed unexpectedly")
			}
			if _, err := os.Stat(filepath.Join(dir, "artifact.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("completion marker created after partial write")
			}
			if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
				err := checkPrivateDelivery(dir, files, "directory delivery", ErrDirectoryDeliveryDrift)
				if !errors.Is(err, ErrDirectoryDeliveryDrift) {
					t.Fatalf("partial output passed inspection: %v", err)
				}
			}
		})
	}
}

func TestDirectoryDeliveryRequiresFrozenReplayBeforeIO(t *testing.T) {
	base, enhanced, source := syntheticDirectoryDeliveries(t)
	sourceBefore := bytes.Clone(source)
	baseBefore, _ := json.Marshal(base)
	enhancedBefore, _ := json.Marshal(enhanced)
	for _, tc := range []struct {
		name string
		call func(string) error
	}{
		{"write-base", func(dir string) error { return WriteDirectoryDelivery(dir, base, source) }},
		{"check-base", func(dir string) error { return CheckDirectoryDelivery(dir, base, source) }},
		{"write-enhanced", func(dir string) error { return WriteDirectoryEnhancedDelivery(dir, enhanced, source) }},
		{"check-enhanced", func(dir string) error { return CheckDirectoryEnhancedDelivery(dir, enhanced, source) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "must-not-exist")
			err := tc.call(dir)
			if err == nil || errors.Is(err, ErrDirectoryDeliveryDrift) || !strings.Contains(err.Error(), "unsupported source identity") {
				t.Fatalf("unsupported replay classified incorrectly: %v", err)
			}
			if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refused replay performed destination I/O")
			}
		})
	}
	baseAfter, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	enhancedAfter, err := json.Marshal(enhanced)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(baseBefore, baseAfter) || !bytes.Equal(enhancedBefore, enhancedAfter) {
		t.Fatal("refused replay mutated an artifact")
	}
	if !bytes.Equal(sourceBefore, source) {
		t.Fatal("refused replay mutated source bytes")
	}
}
