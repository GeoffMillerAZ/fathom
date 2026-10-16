package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

func TestAssetsAdaptFAQRefusals(t *testing.T) {
	root := t.TempDir()
	for _, flags := range [][]string{
		{"-root", root}, {"-root", ""}, {"-root", root, "extra"},
		{"-root", root, "-out="}, {"-root", root, "-check="},
		{"-root", root, "-out", filepath.Join(root, "out"), "-check", root},
		{"-root", root, "-out", filepath.Join(root, "out")},
		{"-id", "arbitrary"}, {"-save"}, {"-target", "go-web"},
	} {
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-faq"}, flags...)...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatal("refusal lost exit/output boundary")
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("refusal wrote output")
	}
}

func TestAssetsAdaptFAQLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("absolute checkout required")
	}
	assets := filepath.Join(root, "data/design-assets")
	_, archiveSHA, _ := strings.Cut(designadapt.FAQArchive, "@")
	sources := []string{filepath.Join(assets, "catalog.json"), filepath.Join(assets, "archives", archiveSHA+".zip")}
	for _, file := range sources {
		before, err := directoryDeliveryFileSHA(file)
		if err != nil {
			t.Fatal("source unavailable")
		}
		t.Cleanup(func() {
			after, err := directoryDeliveryFileSHA(file)
			if err != nil || after != before {
				t.Error("original archive/catalog changed")
			}
		})
	}
	first, e1 := captureAssetsCLI(t, "assets-adapt-faq", "-root", assets)
	second, e2 := captureAssetsCLI(t, "assets-adapt-faq", "-root", assets)
	if e1 != nil || e2 != nil || !bytes.Equal(first, second) {
		t.Fatal("private JSON replay failed; bodies withheld")
	}
	var artifact designadapt.FAQArtifact
	if json.Unmarshal(first, &artifact) != nil || artifact.Verification != "not-verified" {
		t.Fatal("invalid FAQ envelope")
	}
	original, err := designassets.ReadFile(assets, designadapt.FAQArchive, designadapt.FAQMember)
	if err != nil || designadapt.VerifyFAQReplay(artifact, []byte(original.Content)) != nil {
		t.Fatal("source replay failed")
	}
	parent := t.TempDir()
	out := filepath.Join(parent, "delivery")
	invoke := func(want int, flags ...string) {
		t.Helper()
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-faq", "-root", assets}, flags...)...)
		if command.Code(err) != want || len(body) != 0 {
			t.Fatalf("FAQ exit/output mismatch, want %d", want)
		}
	}
	invoke(0, "-out", out)
	entries, err := os.ReadDir(out)
	wantFiles := []string{"USAGE.md", "artifact.json", "defaults.json", "faq.css", "faq.templ", "faq_model.go"}
	if err != nil || len(entries) != len(wantFiles) {
		t.Fatal("wrong delivery inventory")
	}
	for i, entry := range entries {
		if entry.Name() != wantFiles[i] {
			t.Fatal("wrong delivery filename")
		}
		info, err := entry.Info()
		if err != nil || info.Mode() != 0600 {
			t.Fatal("non-private delivery file")
		}
	}
	if info, err := os.Stat(out); err != nil || info.Mode() != os.ModeDir|0700 {
		t.Fatal("non-private delivery directory")
	}
	delivered, err := os.ReadFile(filepath.Join(out, "artifact.json"))
	if err != nil || !bytes.Equal(delivered, first) {
		t.Fatal("delivered receipt differs from JSON default")
	}
	invoke(0, "-check", out)
	invoke(2, "-out", out)
	invoke(0, "-check", out)
	invoke(1, "-check", filepath.Join(parent, "missing"))
	for _, name := range wantFiles {
		file := filepath.Join(out, name)
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, append(append([]byte(nil), body...), '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		invoke(1, "-check", out)
		if err := os.WriteFile(file, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	invoke(0, "-check", out)
	t.Logf("P03 source=%d bytes JSON=%d bytes delivery=%d files; source-preservation check registered", len(original.Content), len(first), len(entries))
}
