package designadapt

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDrawerPrivateDeliveryInventoryAndDrift(t *testing.T) {
	a, err := adaptDrawer(syntheticSource(syntheticDrawer), []byte(syntheticDrawer))
	if err != nil {
		t.Fatal(err)
	}
	files, err := drawerDeliveryFiles(a)
	if err != nil || len(files) != 5 {
		t.Fatal("wrong private inventory")
	}
	for i, name := range []string{"drawer.templ", "drawer.go", "defaults.json", "USAGE.md", "artifact.json"} {
		if files[i].name != name {
			t.Fatal("inventory ordering changed")
		}
	}
	dir := filepath.Join(t.TempDir(), "delivery")
	if err := writePrivateDelivery(dir, files, "drawer fixture"); err != nil {
		t.Fatal(err)
	}
	if err := checkPrivateDelivery(dir, files, "drawer fixture", ErrDrawerDeliveryDrift); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateDelivery(dir, files, "drawer fixture"); err == nil {
		t.Fatal("existing output overwritten")
	}
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, []byte("drift"), 0600); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(checkPrivateDelivery(dir, files, "drawer fixture", ErrDrawerDeliveryDrift), ErrDrawerDeliveryDrift) {
			t.Fatal("changed member accepted")
		}
		if err := os.WriteFile(p, f.body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "extra"), []byte("extra"), 0600); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(checkPrivateDelivery(dir, files, "drawer fixture", ErrDrawerDeliveryDrift), ErrDrawerDeliveryDrift) {
		t.Fatal("extra member accepted")
	}
}

func TestDrawerDeliveryVerifiesSourceBeforeIO(t *testing.T) {
	a, err := adaptDrawer(syntheticSource(syntheticDrawer), []byte(syntheticDrawer))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "absent")
	if WriteDrawerDelivery(dir, a, []byte(syntheticDrawer)) == nil || CheckDrawerDelivery(dir, a, []byte(syntheticDrawer)) == nil {
		t.Fatal("unapproved source accepted")
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("replay refusal touched output")
	}
}
