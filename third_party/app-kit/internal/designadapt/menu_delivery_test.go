package designadapt

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMenuPrivateDeliveryInventoryAndDrift(t *testing.T) {
	a, err := adaptMenu(syntheticSource(syntheticMenu), []byte(syntheticMenu))
	if err != nil {
		t.Fatal(err)
	}
	files, err := menuDeliveryFiles(a)
	if err != nil || len(files) != 6 || files[len(files)-1].name != "artifact.json" {
		t.Fatal("inventory contract changed")
	}
	parent := t.TempDir()
	if WriteMenuDelivery(filepath.Join(parent, "public-entry"), a, []byte(syntheticMenu)) == nil {
		t.Fatal("public entry accepted non-frozen source")
	}
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatal("rejected source wrote output")
	}
	for _, name := range []string{"menu.templ", "menu.go", "menu.css", "defaults.json", "USAGE.md", "artifact.json"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "delivery")
			if writePrivateDelivery(dir, files, "menu delivery") != nil || checkPrivateDelivery(dir, files, "menu delivery", ErrMenuDeliveryDrift) != nil {
				t.Fatal("private replay failed")
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte("synthetic drift"), 0600); err != nil {
				t.Fatal(err)
			}
			if !errors.Is(checkPrivateDelivery(dir, files, "menu delivery", ErrMenuDeliveryDrift), ErrMenuDeliveryDrift) {
				t.Fatal("drift not detected")
			}
		})
	}
}
