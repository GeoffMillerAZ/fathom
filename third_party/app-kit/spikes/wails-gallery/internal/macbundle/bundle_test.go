//go:build darwin

package macbundle

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeExecutable(t *testing.T, dir, name, body string) Executable {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o500); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(body))
	return Executable{Path: path, SHA256: fmt.Sprintf("%x", sum)}
}

func fixtureInputs(t *testing.T, parent string) Inputs {
	t.Helper()
	sources := filepath.Join(parent, "sources")
	if err := os.Mkdir(sources, 0o700); err != nil {
		t.Fatal(err)
	}
	return Inputs{
		Launcher:    writeExecutable(t, sources, "launcher", fmt.Sprintf("#!/bin/sh\ntouch %q\n", filepath.Join(parent, "should-not-exist"))),
		Server:      writeExecutable(t, sources, "server", "synthetic gallery server\n"),
		Native:      writeExecutable(t, sources, "native", "synthetic native companion\n"),
		KitRevision: strings.Repeat("a", 40),
		MinMacOS:    "14.0",
	}
}

func assembleFixture(t *testing.T) (string, Inputs, Manifest) {
	t.Helper()
	parent := realTempDir(t)
	input := fixtureInputs(t, parent)
	destination := filepath.Join(parent, "Gallery.app")
	manifest, err := Assemble(destination, input)
	if err != nil {
		t.Fatal(err)
	}
	return destination, input, manifest
}

func TestAssembleIsDeterministicAndPlanRelocates(t *testing.T) {
	parent := realTempDir(t)
	input := fixtureInputs(t, parent)
	first := filepath.Join(parent, "First.app")
	second := filepath.Join(parent, "Second.app")
	firstManifest, err := Assemble(first, input)
	if err != nil {
		t.Fatal(err)
	}
	secondManifest, err := Assemble(second, input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstManifest, secondManifest) {
		t.Fatalf("deterministic manifests differ:\n%#v\n%#v", firstManifest, secondManifest)
	}
	for _, rel := range []string{launcherPath, serverPath, nativePath, manifestPath, plistPath} {
		firstBody, err := os.ReadFile(filepath.Join(first, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		secondBody, err := os.ReadFile(filepath.Join(second, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(firstBody, secondBody) {
			t.Errorf("%s differs between assemblies", rel)
		}
	}

	relocated := filepath.Join(parent, "Relocated.app")
	if err := os.Rename(first, relocated); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(relocated, filepath.FromSlash(launcherPath))
	plan, err := Plan(launcher)
	if err != nil {
		t.Fatal(err)
	}
	wantServer := filepath.Join(relocated, filepath.FromSlash(serverPath))
	wantNative := filepath.Join(relocated, filepath.FromSlash(nativePath))
	wantArgs := []string{"-native-host", wantNative, "-native-host-sha256", input.Native.SHA256}
	if plan.Executable != wantServer || !reflect.DeepEqual(plan.Args, wantArgs) || !reflect.DeepEqual(plan.Manifest, firstManifest) {
		t.Fatalf("unexpected relocated plan: %#v", plan)
	}
	if _, err := os.Stat(filepath.Join(parent, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("synthetic executable was run")
	}
	if got := string(mustRead(t, filepath.Join(relocated, filepath.FromSlash(plistPath)))); !strings.Contains(got, "<key>CFBundleExecutable</key><string>gallery-launcher</string>") || !strings.Contains(got, "<key>CFBundlePackageType</key><string>APPL</string>") || !strings.Contains(got, "<key>LSMinimumSystemVersion</key><string>14.0</string>") {
		t.Fatalf("generated plist does not bind fixed metadata:\n%s", got)
	}
}

func TestAssembleRefusesInvalidInputsBeforeDestinationMutation(t *testing.T) {
	tests := map[string]func(t *testing.T, parent string, input *Inputs, destination *string){
		"bad launcher hash": func(_ *testing.T, _ string, input *Inputs, _ *string) {
			input.Launcher.SHA256 = strings.Repeat("0", 64)
		},
		"invalid kit revision": func(_ *testing.T, _ string, input *Inputs, _ *string) {
			input.KitRevision = "working-tree"
		},
		"invalid minimum macOS": func(_ *testing.T, _ string, input *Inputs, _ *string) {
			input.MinMacOS = "014.0"
		},
		"relative destination": func(_ *testing.T, _ string, _ *Inputs, destination *string) {
			*destination = "Gallery.app"
		},
		"wrong destination extension": func(_ *testing.T, parent string, _ *Inputs, destination *string) {
			*destination = filepath.Join(parent, "Gallery.bundle")
		},
		"destination collision": func(t *testing.T, _ string, _ *Inputs, destination *string) {
			if err := os.Mkdir(*destination, 0o700); err != nil {
				t.Fatal(err)
			}
		},
		"symlink input": func(t *testing.T, parent string, input *Inputs, _ *string) {
			link := filepath.Join(parent, "launcher-link")
			if err := os.Symlink(input.Launcher.Path, link); err != nil {
				t.Fatal(err)
			}
			input.Launcher.Path = link
		},
		"group writable input": func(t *testing.T, _ string, input *Inputs, _ *string) {
			if err := os.Chmod(input.Server.Path, 0o720); err != nil {
				t.Fatal(err)
			}
		},
		"oversized sparse input": func(t *testing.T, _ string, input *Inputs, _ *string) {
			if err := os.Chmod(input.Server.Path, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(input.Server.Path, maxExecutable+1); err != nil {
				t.Fatal(err)
			}
		},
		"symlink destination parent": func(t *testing.T, parent string, _ *Inputs, destination *string) {
			realParent := filepath.Join(parent, "real-parent")
			if err := os.Mkdir(realParent, 0o700); err != nil {
				t.Fatal(err)
			}
			linkParent := filepath.Join(parent, "linked-parent")
			if err := os.Symlink(realParent, linkParent); err != nil {
				t.Fatal(err)
			}
			*destination = filepath.Join(linkParent, "Gallery.app")
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			parent := realTempDir(t)
			input := fixtureInputs(t, parent)
			destination := filepath.Join(parent, "Gallery.app")
			mutate(t, parent, &input, &destination)
			_, err := Assemble(destination, input)
			if err == nil {
				t.Fatal("invalid input accepted")
			}
			if name != "destination collision" {
				if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
					t.Fatalf("destination mutated on preflight failure: %v", statErr)
				}
			}
		})
	}
}

func TestPlanRefusesChangedOrMalformedBundle(t *testing.T) {
	tests := map[string]func(t *testing.T, root string){
		"missing executable": func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(serverPath))); err != nil {
				t.Fatal(err)
			}
		},
		"tampered executable": func(t *testing.T, root string) {
			path := filepath.Join(root, filepath.FromSlash(serverPath))
			if err := os.WriteFile(path, []byte("tampered"), 0o700); err != nil {
				t.Fatal(err)
			}
		},
		"group writable executable": func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, filepath.FromSlash(nativePath)), 0o720); err != nil {
				t.Fatal(err)
			}
		},
		"group writable bundle directory": func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, "Contents", "Resources"), 0o720); err != nil {
				t.Fatal(err)
			}
		},
		"group writable staging parent": func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Dir(root), 0o770); err != nil {
				t.Fatal(err)
			}
		},
		"group writable metadata": func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, filepath.FromSlash(manifestPath)), 0o620); err != nil {
				t.Fatal(err)
			}
		},
		"oversized sparse metadata": func(t *testing.T, root string) {
			if err := os.Truncate(filepath.Join(root, filepath.FromSlash(manifestPath)), maxMetadata+1); err != nil {
				t.Fatal(err)
			}
		},
		"symlink executable": func(t *testing.T, root string) {
			path := filepath.Join(root, filepath.FromSlash(nativePath))
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, filepath.FromSlash(serverPath)), path); err != nil {
				t.Fatal(err)
			}
		},
		"truncated manifest": func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(manifestPath)), []byte(`{"schema_version":1`), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"unknown manifest field": func(t *testing.T, root string) {
			path := filepath.Join(root, filepath.FromSlash(manifestPath))
			body := mustRead(t, path)
			body = bytes.Replace(body, []byte(`{"schema_version":1,`), []byte(`{"unknown":true,"schema_version":1,`), 1)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"duplicate manifest field": func(t *testing.T, root string) {
			path := filepath.Join(root, filepath.FromSlash(manifestPath))
			body := mustRead(t, path)
			body = bytes.Replace(body, []byte(`{"schema_version":1,`), []byte(`{"schema_version":1,"schema_version":1,`), 1)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"trailing manifest value": func(t *testing.T, root string) {
			path := filepath.Join(root, filepath.FromSlash(manifestPath))
			body := append(mustRead(t, path), []byte("{}")...)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"noncanonical manifest": func(t *testing.T, root string) {
			path := filepath.Join(root, filepath.FromSlash(manifestPath))
			body := append([]byte(" "), mustRead(t, path)...)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"invalid canonical metadata": func(t *testing.T, root string) {
			path := filepath.Join(root, filepath.FromSlash(manifestPath))
			manifest, err := decodeManifest(mustRead(t, path))
			if err != nil {
				t.Fatal(err)
			}
			manifest.MinMacOS = "14"
			body, err := canonicalManifest(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"changed plist": func(t *testing.T, root string) {
			path := filepath.Join(root, filepath.FromSlash(plistPath))
			body := bytes.Replace(mustRead(t, path), []byte("gallery-launcher"), []byte("gallery-native"), 1)
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"unexpected file": func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "Contents", "unexpected"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			root, _, _ := assembleFixture(t)
			mutate(t, root)
			_, err := Plan(filepath.Join(root, filepath.FromSlash(launcherPath)))
			if err == nil {
				t.Fatal("changed bundle accepted")
			}
		})
	}
}

func TestPlanRequiresAbsoluteLauncherAtFixedLocation(t *testing.T) {
	root, _, _ := assembleFixture(t)
	launcher := filepath.Join(root, filepath.FromSlash(launcherPath))
	if _, err := Plan(filepath.Base(launcher)); err == nil {
		t.Fatal("relative launcher accepted")
	}
	wrong := filepath.Join(root, "Contents", "Resources", "gallery-launcher")
	if err := os.WriteFile(wrong, []byte("wrong"), 0o500); err != nil {
		t.Fatal(err)
	}
	if _, err := Plan(wrong); err == nil {
		t.Fatal("launcher outside fixed location accepted")
	}
}

func TestAssembleRefusesWritableStagingParent(t *testing.T) {
	parent := realTempDir(t)
	input := fixtureInputs(t, parent)
	if err := os.Chmod(parent, 0o770); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(parent, "Gallery.app")
	if _, err := Assemble(destination, input); err == nil {
		t.Fatal("group-writable staging parent accepted")
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination mutated after staging refusal: %v", err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
