package designadapt

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func syntheticDelivery(t *testing.T) []pricingDeliveryFile {
	t.Helper()
	artifact, err := adaptPricing(syntheticSource(syntheticPricing), []byte(syntheticPricing))
	if err != nil {
		t.Fatal(err)
	}
	files, err := pricingDeliveryFiles(artifact)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestPricingDeliveryDeterministicAndPrivate(t *testing.T) {
	files := syntheticDelivery(t)
	parent := t.TempDir()
	for _, leaf := range []string{"first", "second"} {
		dir := filepath.Join(parent, leaf)
		if err := writePricingDelivery(dir, files); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(dir)
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal("output directory not private")
		}
		for _, item := range files {
			body, err := os.ReadFile(filepath.Join(dir, item.name))
			if err != nil || !bytes.Equal(body, item.body) {
				t.Fatal("projection differs")
			}
			info, err := os.Stat(filepath.Join(dir, item.name))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("output file not private")
			}
		}
		if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
			if err := checkPricingDelivery(dir, files); err != nil {
				t.Fatal(err)
			}
		}
		if err := writePricingDelivery(dir, files); err == nil {
			t.Fatal("overwrote existing delivery")
		}
	}
}

func TestPricingDeliveryRefusesExistingOrLinkedDestinations(t *testing.T) {
	files := syntheticDelivery(t)
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
		if err := writePricingDelivery(dir, files); err == nil {
			t.Fatalf("accepted invalid output %q", dir)
		}
	}
	entries, err := os.ReadDir(real)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid export changed existing destination")
	}
}

func TestPricingDeliveryDetectsEveryFileAndEntryDrift(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("secure reader supports macOS/Linux")
	}
	files := syntheticDelivery(t)
	for _, item := range files {
		for _, variant := range []string{"missing", "same-size-edit", "larger", "directory", "symlink", "hardlink", "public-mode", "executable-mode", "setuid-mode", "sticky-mode"} {
			t.Run(item.name+"/"+variant, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), "delivery")
				if err := writePricingDelivery(dir, files); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, item.name)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				var err error
				switch variant {
				case "same-size-edit":
					body := bytes.Clone(item.body)
					body[0] ^= 1
					err = os.WriteFile(path, body, 0600)
				case "larger":
					err = os.WriteFile(path, append(bytes.Clone(item.body), 'x'), 0600)
				case "directory":
					err = os.Mkdir(path, 0700)
				case "symlink":
					err = os.Symlink("README.md", path)
				case "hardlink":
					external := filepath.Join(t.TempDir(), "exact-copy")
					if err = os.WriteFile(external, item.body, 0600); err == nil {
						err = os.Link(external, path)
					}
				case "public-mode":
					err = os.WriteFile(path, item.body, 0644)
				case "executable-mode":
					err = os.WriteFile(path, item.body, 0700)
				case "setuid-mode", "sticky-mode":
					mode := os.ModeSetuid
					if variant == "sticky-mode" {
						mode = os.ModeSticky
					}
					if err = os.WriteFile(path, item.body, 0600); err == nil {
						err = os.Chmod(path, mode|0600)
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := checkPricingDelivery(dir, files); !errors.Is(err, ErrPricingDeliveryDrift) {
					t.Fatalf("drift classified incorrectly: %v", err)
				}
			})
		}
	}
	for _, variant := range []string{"missing-root", "root-link", "root-mode", "extra"} {
		t.Run(variant, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "delivery")
			if variant != "missing-root" {
				if err := writePricingDelivery(dir, files); err != nil {
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
			if err := checkPricingDelivery(dir, files); !errors.Is(err, ErrPricingDeliveryDrift) {
				t.Fatalf("invalid root/entries passed: %v", err)
			}
		})
	}
}

func TestPricingDeliveryPartialWriteIsNotCompleteAndDoesNotOverwrite(t *testing.T) {
	files := syntheticDelivery(t)
	parent := t.TempDir()
	dir := filepath.Join(parent, "partial")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(dir, "pricing.css")
	if err := os.WriteFile(collision, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := writePricingFiles(root, "partial", files); err == nil {
		t.Fatal("file collision accepted")
	}
	body, err := os.ReadFile(collision)
	if err != nil || string(body) != "preserve" {
		t.Fatal("collision overwritten")
	}
	if _, err := os.Stat(filepath.Join(dir, "pricing.templ")); err != nil {
		t.Fatal("partial output removed unexpectedly")
	}
	if _, err := os.Stat(filepath.Join(dir, "artifact.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completion marker created after failure")
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if err := checkPricingDelivery(dir, files); !errors.Is(err, ErrPricingDeliveryDrift) {
			t.Fatal("partial output passed")
		}
	}
}

func TestPricingDeliveryRequiresFrozenReplayBeforeIO(t *testing.T) {
	artifact, err := adaptPricing(syntheticSource(syntheticPricing), []byte(syntheticPricing))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "uncreated")
	if err := WritePricingDelivery(dir, artifact, []byte(syntheticPricing)); err == nil {
		t.Fatal("export accepted non-frozen source")
	}
	if err := CheckPricingDelivery(dir, artifact, []byte(syntheticPricing)); err == nil || errors.Is(err, ErrPricingDeliveryDrift) {
		t.Fatal("check trusted unsupported source")
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("refused replay performed I/O")
	}
}
