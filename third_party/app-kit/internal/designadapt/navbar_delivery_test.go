package designadapt

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNavbarPrivateDeliveryInventoryAndDrift(t *testing.T) {
	a, err := adaptNavbar(syntheticSource(syntheticNavbar), []byte(syntheticNavbar))
	if err != nil {
		t.Fatal(err)
	}
	files, err := navbarDeliveryFiles(a)
	if err != nil || len(files) != 5 {
		t.Fatal("wrong private inventory")
	}
	for i, name := range []string{"navbar.templ", "navbar.go", "defaults.json", "USAGE.md", "artifact.json"} {
		if files[i].name != name {
			t.Fatal("inventory ordering changed")
		}
	}
	if !bytes.Equal(files[2].body, a.Defaults) {
		t.Fatal("defaults bytes changed")
	}
	for _, text := range []string{"NavbarReference()", "NewNavbar(Navbar)", "Elements 1.0.22", "CurrentKey", "none active", "unique identifier Key", "decorative avatar", "No authentication", "not target certification"} {
		if !bytes.Contains(files[3].body, []byte(text)) {
			t.Fatalf("usage boundary missing %q", text)
		}
	}
	dir := filepath.Join(t.TempDir(), "delivery")
	if err := writePrivateDelivery(dir, files, "navbar fixture"); err != nil {
		t.Fatal(err)
	}
	if err := checkPrivateDelivery(dir, files, "navbar fixture", ErrNavbarDeliveryDrift); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateDelivery(dir, files, "navbar fixture"); err == nil {
		t.Fatal("existing output overwritten")
	}
	for _, file := range files {
		path := filepath.Join(dir, file.name)
		if err := os.WriteFile(path, []byte("drift"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := checkPrivateDelivery(dir, files, "navbar fixture", ErrNavbarDeliveryDrift); !errors.Is(err, ErrNavbarDeliveryDrift) || errors.Is(err, ErrDrawerDeliveryDrift) {
			t.Fatalf("changed member classified incorrectly: %v", err)
		}
		if err := os.WriteFile(path, file.body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "extra"), []byte("extra"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkPrivateDelivery(dir, files, "navbar fixture", ErrNavbarDeliveryDrift); !errors.Is(err, ErrNavbarDeliveryDrift) {
		t.Fatal("extra member accepted")
	}
}

func TestNavbarDeliveryVerifiesSourceBeforeIO(t *testing.T) {
	a, err := adaptNavbar(syntheticSource(syntheticNavbar), []byte(syntheticNavbar))
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	dir := filepath.Join(parent, "absent")
	if WriteNavbarDelivery(dir, a, []byte(syntheticNavbar)) == nil || CheckNavbarDelivery(dir, a, []byte(syntheticNavbar)) == nil {
		t.Fatal("unreviewed source accepted")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("replay refusal touched output")
	}
}
