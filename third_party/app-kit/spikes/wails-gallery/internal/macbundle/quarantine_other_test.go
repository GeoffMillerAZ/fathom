//go:build !darwin

package macbundle

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssemblyFailsClosedWithoutMacOSQuarantineInspection(t *testing.T) {
	parent, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	makeExecutable := func(name string) Executable {
		path := filepath.Join(parent, name)
		body := []byte("synthetic executable " + name)
		if err := os.WriteFile(path, body, 0o500); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		return Executable{Path: path, SHA256: fmt.Sprintf("%x", sum)}
	}
	input := Inputs{
		Launcher:    makeExecutable("launcher"),
		Server:      makeExecutable("server"),
		Native:      makeExecutable("native"),
		KitRevision: strings.Repeat("a", 40),
		MinMacOS:    "14.0",
	}
	destination := filepath.Join(parent, "Gallery.app")
	if _, err := Assemble(destination, input); err == nil || !strings.Contains(err.Error(), "quarantine inspection unsupported") {
		t.Fatalf("non-Darwin assembly did not fail closed: %v", err)
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination mutated on unsupported host: %v", err)
	}
}
