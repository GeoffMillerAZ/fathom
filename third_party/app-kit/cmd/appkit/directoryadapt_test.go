package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

func TestAssetsAdaptDirectoryRefusesWithoutWriting(t *testing.T) {
	root := t.TempDir()
	for _, flags := range [][]string{
		{"-root", root}, {"-root", ""}, {"-root", root, "extra"},
		{"-root", root, "-enhanced"}, {"-root", root, "-enhanced=false"}, {"-enhanced=invalid"},
		{"-root", root, "-out", filepath.Join(root, "output")}, {"-root", root, "-check", root},
		{"-root", root, "-out="}, {"-root", root, "-check="},
		{"-root", root, "-out", filepath.Join(root, "output"), "-check", root},
		{"-save"}, {"-id", "arbitrary"}, {"-target", "react"},
	} {
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-directory"}, flags...)...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatal("refusal emitted private body or success")
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("refused command created output")
	}
}

func TestAssetsAdaptDirectoryLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("absolute checkout required")
	}
	assets := filepath.Join(root, "data/design-assets")
	hashFile := func(name string) string {
		t.Helper()
		f, err := os.Open(name)
		if err != nil {
			t.Fatal("private identity unavailable")
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			t.Fatal("private identity read")
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	archiveSHA := strings.Split(designadapt.DirectoryArchive, "@")[1]
	before := map[string]string{}
	for _, name := range []string{"catalog.json", "archives/" + archiveSHA + ".zip"} {
		file := filepath.Join(assets, name)
		before[file] = hashFile(file)
		if strings.HasSuffix(name, ".zip") && before[file] != archiveSHA {
			t.Fatal("archive hash")
		}
	}
	t.Cleanup(func() {
		for file, hash := range before {
			if hashFile(file) != hash {
				t.Error("source/catalog changed")
			}
		}
	})
	first, e1 := captureAssetsCLI(t, "assets-adapt-directory", "-root", assets)
	second, e2 := captureAssetsCLI(t, "assets-adapt-directory", "-root", assets)
	if e1 != nil || e2 != nil || !bytes.Equal(first, second) {
		t.Fatal("private CLI replay failed; bodies withheld")
	}
	explicitBase, explicitErr := captureAssetsCLI(t, "assets-adapt-directory", "-enhanced=false", "-root", assets)
	if explicitErr != nil || !bytes.Equal(first, explicitBase) {
		t.Fatal("default envelope changed when enhancement is explicitly disabled")
	}
	var artifact designadapt.DirectoryArtifact
	if json.Unmarshal(first, &artifact) != nil || artifact.Verification != "not-verified" {
		t.Fatal("invalid evidence envelope")
	}
	original, err := designassets.ReadFile(assets, designadapt.DirectoryArchive, designadapt.DirectoryMember)
	if err != nil || designadapt.VerifyDirectoryReplay(artifact, []byte(original.Content)) != nil {
		t.Fatal("private replay mismatch")
	}
	enhancedFirst, e1 := captureAssetsCLI(t, "assets-adapt-directory", "-root", assets, "-enhanced")
	enhancedSecond, e2 := captureAssetsCLI(t, "assets-adapt-directory", "-enhanced", "-root", assets)
	if e1 != nil || e2 != nil || !bytes.Equal(enhancedFirst, enhancedSecond) {
		t.Fatal("private enhanced CLI replay failed; bodies withheld")
	}
	var enhanced designadapt.DirectoryEnhancedArtifact
	if json.Unmarshal(enhancedFirst, &enhanced) != nil || enhanced.Verification != "not-verified" || !reflect.DeepEqual(enhanced.Base, artifact) {
		t.Fatal("enhanced envelope lost exact default lineage")
	}
	if designadapt.VerifyDirectoryEnhancedReplay(enhanced, []byte(original.Content)) != nil {
		t.Fatal("private enhanced replay mismatch")
	}
}
