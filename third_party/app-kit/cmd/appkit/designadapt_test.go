package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

func TestAssetsAdaptPricingRefusesWithoutWriting(t *testing.T) {
	root := t.TempDir()
	for _, flags := range [][]string{
		{"-root", root},
		{"-root", ""},
		{"-root", root, "extra"},
		{"-out="},
		{"-check="},
		{"-out", filepath.Join(root, "export"), "-check", filepath.Join(root, "check")},
		{"-save"},
		{"-id", "arbitrary"},
		{"-target", "react"},
	} {
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-pricing"}, flags...)...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatal("invalid/unavailable input emitted output or success")
		}
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 0 {
		t.Fatal("read-only command created output")
	}
}

func TestAssetsAdaptPricingLive(t *testing.T) {
	kitRoot := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kitRoot == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kitRoot) {
		t.Fatal("absolute checkout required")
	}
	root := filepath.Join(kitRoot, "data/design-assets")
	paths := []string{filepath.Join(root, "catalog.json"), filepath.Join(root, "archives", "8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675.zip")}
	before := make([][]byte, len(paths))
	for i, path := range paths {
		var err error
		before[i], err = os.ReadFile(path)
		if err != nil {
			t.Fatal("source unavailable")
		}
	}
	t.Cleanup(func() {
		for i, path := range paths {
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before[i], after) {
				t.Error("CLI changed private source/index")
			}
		}
	})
	first, e1 := captureAssetsCLI(t, "assets-adapt-pricing", "-root", root)
	second, e2 := captureAssetsCLI(t, "assets-adapt-pricing", "-root", root)
	if e1 != nil || e2 != nil || !bytes.Equal(first, second) {
		t.Fatal("CLI replay failed; private bodies omitted")
	}
	var artifact designadapt.PricingArtifact
	if json.Unmarshal(first, &artifact) != nil {
		t.Fatal("invalid envelope")
	}
	original, err := designassets.ReadFile(root, designadapt.PricingArchive, designadapt.PricingMember)
	if err != nil || designadapt.VerifyPricingReplay(artifact, []byte(original.Content)) != nil {
		t.Fatal("CLI source/replay failed")
	}

	parent := t.TempDir()
	export := func(name string) string {
		t.Helper()
		out := filepath.Join(parent, name)
		body, err := captureAssetsCLI(t, "assets-adapt-pricing", "-root", root, "-out", out)
		if err != nil || len(body) != 0 {
			t.Fatal("private P02 package export failed; response omitted")
		}
		return out
	}
	out1, out2 := export("one"), export("two")
	one, two := readPricingPackage(t, out1), readPricingPackage(t, out2)
	if !reflect.DeepEqual(one, two) {
		t.Fatal("repeat P02 package exports differ")
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(out1); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatal("private P02 package directory mode is not 0700")
		}
		for _, file := range one {
			if file.Mode != 0o600 {
				t.Fatal("private P02 package file mode is not 0600")
			}
		}
	}
	checkBody, checkErr := captureAssetsCLI(t, "assets-adapt-pricing", "-root", root, "-check", out1)
	if checkErr != nil || len(checkBody) != 0 || !reflect.DeepEqual(one, readPricingPackage(t, out1)) {
		t.Fatal("matching P02 package check failed or mutated the package; response omitted")
	}
	beforeCollision := readPricingPackage(t, out1)
	collisionBody, collisionErr := captureAssetsCLI(t, "assets-adapt-pricing", "-root", root, "-out", out1)
	if collisionErr == nil || len(collisionBody) != 0 || !reflect.DeepEqual(beforeCollision, readPricingPackage(t, out1)) {
		t.Fatal("P02 export overwrote or accepted an existing target")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{"missing", func(t *testing.T, dir string) { mustRemove(t, filepath.Join(dir, "pricing.css")) }},
		{"changed", func(t *testing.T, dir string) {
			mustWrite(t, filepath.Join(dir, "pricing.css"), []byte("changed"), 0o600)
		}},
		{"extra", func(t *testing.T, dir string) { mustWrite(t, filepath.Join(dir, "extra.txt"), []byte("extra"), 0o600) }},
		{"nonregular", func(t *testing.T, dir string) {
			path := filepath.Join(dir, "pricing.css")
			mustRemove(t, path)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal("could not plant non-regular package entry")
			}
		}},
		{"symlink", func(t *testing.T, dir string) {
			path := filepath.Join(dir, "pricing.css")
			mustRemove(t, path)
			if err := os.Symlink(filepath.Join(dir, "pricing.go"), path); err != nil {
				t.Skip("symlink unavailable on this host")
			}
		}},
	} {
		t.Run("check-refuses-"+tc.name, func(t *testing.T) {
			dir := filepath.Join(parent, "bad-"+tc.name)
			copyPricingPackage(t, out1, dir)
			tc.mutate(t, dir)
			assertPricingCLIError(t, 1, "-root", root, "-check", dir)
		})
	}
	assertPricingCLIError(t, 1, "-root", root, "-check", filepath.Join(parent, "absent"))
	assertPricingCLIError(t, 2, "-root", filepath.Join(parent, "missing-source"), "-check", out1)
	assertPricingCLIError(t, 2, "-root", root, "-out", filepath.Join(parent, "missing-parent", "package"))
	linkedParent := filepath.Join(t.TempDir(), "linked-parent")
	if err := os.Symlink(parent, linkedParent); err == nil {
		assertPricingCLIError(t, 2, "-root", root, "-out", filepath.Join(linkedParent, "package"))
	}
	t.Logf("private P02 JSON bytes=%d deterministic=true package_files=%d verification=%s; source/catalog unchanged", len(first), len(one), artifact.Verification)
}

var pricingPackageNames = []string{"README.md", "artifact.json", "defaults.json", "pricing.css", "pricing.go", "pricing.templ"}

type pricingPackageFile struct {
	Body []byte
	Mode os.FileMode
}

func readPricingPackage(t *testing.T, dir string) map[string]pricingPackageFile {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != len(pricingPackageNames) {
		t.Fatal("private P02 package inventory differs; bodies omitted")
	}
	names := make([]string, 0, len(entries))
	files := make(map[string]pricingPackageFile, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("private P02 package contains a non-regular entry")
		}
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal("private P02 package file could not be read")
		}
		names = append(names, entry.Name())
		files[entry.Name()] = pricingPackageFile{Body: body, Mode: info.Mode().Perm()}
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, pricingPackageNames) {
		t.Fatal("private P02 package filenames differ")
	}
	return files
}

func copyPricingPackage(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatal("private P02 negative fixture directory could not be created")
	}
	for name, file := range readPricingPackage(t, src) {
		mustWrite(t, filepath.Join(dst, name), file.Body, 0o600)
	}
}

func assertPricingCLIError(t *testing.T, code int, args ...string) {
	t.Helper()
	body, err := captureAssetsCLI(t, append([]string{"assets-adapt-pricing"}, args...)...)
	if command.Code(err) != code || len(body) != 0 {
		t.Fatal("P02 package refusal used the wrong exit class or emitted stdout")
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal("private P02 negative fixture could not be changed")
	}
}

func mustWrite(t *testing.T, path string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, body, mode); err != nil {
		t.Fatal("private P02 negative fixture could not be written")
	}
}
