package designadapt

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateDeliveryRejectsInvalidInventoryBeforeIO(t *testing.T) {
	marker := privateDeliveryFile{"artifact.json", []byte("{}")}
	for _, files := range [][]privateDeliveryFile{
		nil,
		{{"other.json", nil}},
		{marker, {"later.go", nil}},
		{{"../outside", nil}, marker},
		{{"sub/file", nil}, marker},
		{{"sub\\file", nil}, marker},
		{{"bad\x00name", nil}, marker},
		{{".", nil}, marker},
		{{"..", nil}, marker},
		{{"", nil}, marker},
		{{"same", nil}, {"same", nil}, marker},
		{marker, marker},
	} {
		dir := filepath.Join(t.TempDir(), "uncreated")
		if err := writePrivateDelivery(dir, files, "test delivery"); err == nil {
			t.Fatal("invalid inventory accepted for export")
		}
		if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("invalid inventory created output")
		}
		if err := checkPrivateDelivery(dir, files, "test delivery", ErrDirectoryDeliveryDrift); err == nil || errors.Is(err, ErrDirectoryDeliveryDrift) {
			t.Fatal("invalid inventory treated as content drift")
		}
	}
}
