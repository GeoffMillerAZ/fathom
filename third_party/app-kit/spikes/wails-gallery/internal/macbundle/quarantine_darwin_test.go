//go:build darwin

package macbundle

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestQuarantinedInputsAndBundleExecutablesAreRefused(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		parent := realTempDir(t)
		input := fixtureInputs(t, parent)
		if err := os.Chmod(input.Launcher.Path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := unix.Setxattr(input.Launcher.Path, "com.apple.quarantine", []byte("0081;synthetic;appkit;"), 0); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(parent, "Gallery.app")
		if _, err := Assemble(destination, input); err == nil {
			t.Fatal("quarantined source accepted")
		}
		if _, err := os.Lstat(destination); !os.IsNotExist(err) {
			t.Fatalf("destination mutated after quarantine refusal: %v", err)
		}
	})

	t.Run("bundle", func(t *testing.T) {
		root, _, _ := assembleFixture(t)
		if err := unix.Setxattr(root, "com.apple.quarantine", []byte("0081;synthetic;appkit;"), 0); err != nil {
			t.Fatal(err)
		}
		if _, err := Plan(filepath.Join(root, filepath.FromSlash(launcherPath))); err == nil {
			t.Fatal("quarantined bundled executable accepted")
		}
	})
}

func TestFIFOInputIsRefusedWithoutOpening(t *testing.T) {
	parent := realTempDir(t)
	input := fixtureInputs(t, parent)
	fifo := filepath.Join(parent, "server-fifo")
	if err := unix.Mkfifo(fifo, 0o500); err != nil {
		t.Fatal(err)
	}
	input.Server = Executable{Path: fifo, SHA256: input.Server.SHA256}
	destination := filepath.Join(parent, "Gallery.app")
	if _, err := Assemble(destination, input); err == nil {
		t.Fatal("FIFO input accepted")
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination mutated after FIFO refusal: %v", err)
	}
}

func TestQuarantinedSourceAncestorIsRefused(t *testing.T) {
	parent := realTempDir(t)
	input := fixtureInputs(t, parent)
	// The marker is on an ancestor, not the selected executable or its immediate parent.
	marker := []byte("0081;synthetic;appkit;")
	if err := unix.Setxattr(parent, "com.apple.quarantine", marker, 0); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(parent, "Gallery.app")
	if _, err := Assemble(destination, input); err == nil {
		t.Fatal("executable from quarantined ancestor accepted")
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination mutated after ancestor quarantine refusal: %v", err)
	}
	if size, err := unix.Getxattr(parent, "com.apple.quarantine", nil); err != nil || size != len(marker) {
		t.Fatalf("quarantine marker changed: size=%d err=%v", size, err)
	}
}

func TestMetadataFIFOIsRefusedWithoutOpening(t *testing.T) {
	root, _, _ := assembleFixture(t)
	manifest := filepath.Join(root, filepath.FromSlash(manifestPath))
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Plan(filepath.Join(root, filepath.FromSlash(launcherPath))); err == nil {
		t.Fatal("FIFO metadata accepted")
	}
}
