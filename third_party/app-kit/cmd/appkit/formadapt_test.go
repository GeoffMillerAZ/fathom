package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

func TestAssetsAdaptFormRefusals(t *testing.T) {
	root := t.TempDir()
	for _, flags := range [][]string{
		{"-root", root}, {"-root", ""}, {"-root", root, "extra"},
		{"-root", root, "-out="}, {"-root", root, "-check="},
		{"-root", root, "-out", filepath.Join(root, "out"), "-check", root},
		{"-root", root, "-out", filepath.Join(root, "out")},
		{"-root", root, "-check", root}, {"-id", "arbitrary"}, {"-enhanced"},
	} {
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-form"}, flags...)...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatal("refusal lost exit/output boundary")
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("refusal wrote output")
	}
}

func TestAssetsAdaptFormLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("absolute checkout required")
	}
	assets := filepath.Join(root, "data/design-assets")
	_, archiveSHA, _ := strings.Cut(designadapt.FormArchive, "@")
	sources := []string{filepath.Join(assets, "catalog.json"), filepath.Join(assets, "archives", archiveSHA+".zip")}
	before := map[string][32]byte{}
	for _, file := range sources {
		hash, err := directoryDeliveryFileSHA(file)
		if err != nil {
			t.Fatal("source unavailable")
		}
		before[file] = hash
	}
	t.Cleanup(func() {
		for file, hash := range before {
			got, err := directoryDeliveryFileSHA(file)
			if err != nil || got != hash {
				t.Error("source/catalog changed or unreadable")
			}
		}
	})
	first, e1 := captureAssetsCLI(t, "assets-adapt-form", "-root", assets)
	second, e2 := captureAssetsCLI(t, "assets-adapt-form", "-root", assets)
	if e1 != nil || e2 != nil || !bytes.Equal(first, second) {
		t.Fatal("private replay failed; bodies withheld")
	}
	var artifact designadapt.FormArtifact
	if json.Unmarshal(first, &artifact) != nil || artifact.Verification != "not-verified" {
		t.Fatal("bad envelope")
	}
	original, err := designassets.ReadFile(assets, designadapt.FormArchive, designadapt.FormMember)
	if err != nil || designadapt.VerifyFormReplay(artifact, []byte(original.Content)) != nil {
		t.Fatal("source replay failed")
	}
	for _, field := range []string{"Adapter", "RuleSHA256", "SyntaxSHA256", "TemplSHA256", "ModelSHA256", "DefaultsSHA256", "Templ", "Model", "Verification"} {
		changed := artifact
		reflect.ValueOf(&changed).Elem().FieldByName(field).SetString("tampered")
		if designadapt.VerifyFormReplay(changed, []byte(original.Content)) == nil {
			t.Fatalf("accepted changed %s", field)
		}
	}
	parent := t.TempDir()
	out := filepath.Join(parent, "delivery")
	invoke := func(want int, flags ...string) {
		t.Helper()
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-form", "-root", assets}, flags...)...)
		if command.Code(err) != want || len(body) != 0 {
			t.Fatalf("exit/output mismatch want %d", want)
		}
	}
	invoke(0, "-out", out)
	wantFiles := []string{"README.md", "artifact.json", "defaults.json", "form.go", "form.templ"}
	entries, err := os.ReadDir(out)
	if err != nil || len(entries) != len(wantFiles) {
		t.Fatal("wrong delivery inventory")
	}
	for i, entry := range entries {
		if entry.Name() != wantFiles[i] {
			t.Fatal("wrong delivery name")
		}
	}
	delivered, err := os.ReadFile(filepath.Join(out, "artifact.json"))
	if err != nil || !bytes.Equal(delivered, first) {
		t.Fatal("delivery differs from JSON default")
	}
	invoke(0, "-check", out)
	invoke(2, "-out", out)
	invoke(0, "-check", out)
	invoke(1, "-check", filepath.Join(parent, "missing"))
	if err := os.WriteFile(filepath.Join(out, "form.go"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	invoke(1, "-check", out)
	t.Logf("P05 source=%d bytes JSON=%d bytes delivery=%d files; original/catalog unchanged check registered", len(original.Content), len(first), len(entries))
}
