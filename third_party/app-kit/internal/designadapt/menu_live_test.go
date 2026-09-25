package designadapt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

func TestAdaptMenuLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_CONVERSION_KIT_ROOT to the private checkout")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("private checkout must be absolute")
	}
	designRoot := filepath.Join(root, "data/design-assets")
	_, archiveHash, _ := strings.Cut(MenuArchive, "@")
	paths := []string{filepath.Join(designRoot, "catalog.json"), filepath.Join(designRoot, "archives", archiveHash+".zip")}
	identities := make(map[string]string)
	for _, path := range paths {
		hash, err := pricingFileSHA256(path)
		if err != nil {
			t.Fatal("original identity read failed")
		}
		identities[path] = hash
	}
	if identities[paths[1]] != archiveHash {
		t.Fatal("archive hash differs")
	}
	t.Cleanup(func() {
		for _, path := range paths {
			if hash, err := pricingFileSHA256(path); err != nil || hash != identities[path] {
				t.Error("original archive/catalog changed")
			}
		}
	})
	file, err := designassets.ReadFile(designRoot, MenuArchive, MenuMember)
	if err != nil {
		t.Fatal("verified original read failed; source details withheld")
	}
	source := designconvert.Source{ArchiveID: file.ArchiveID, Path: file.Path, SHA256: file.SHA256, Size: len(file.Content)}
	a, err := AdaptMenu(source, []byte(file.Content))
	if err != nil {
		t.Fatal("frozen menu adaptation failed")
	}
	b, err := AdaptMenu(source, []byte(file.Content))
	if err != nil || !reflect.DeepEqual(a, b) || VerifyMenuReplay(a, []byte(file.Content)) != nil {
		t.Fatal("frozen menu replay failed")
	}
	out := t.TempDir()
	if retained := os.Getenv("APPKIT_PROBE_MENU_OUTPUT"); retained != "" {
		out = heroPrivateOutput(t, retained)
	}
	delivery := filepath.Join(out, "delivery")
	if WriteMenuDelivery(delivery, a, []byte(file.Content)) != nil || CheckMenuDelivery(delivery, a, []byte(file.Content)) != nil {
		t.Fatal("private delivery replay failed")
	}
	if WriteMenuDelivery(delivery, a, []byte(file.Content)) == nil {
		t.Fatal("existing delivery was overwritten")
	}
	if !errors.Is(CheckMenuDelivery(filepath.Join(out, "absent"), a, []byte(file.Content)), ErrMenuDeliveryDrift) {
		t.Fatal("missing delivery classification differs")
	}
	if err := os.WriteFile(filepath.Join(out, "original.html"), []byte(file.Content), 0600); err != nil {
		t.Fatal("private original write failed")
	}
	runMenuCompiler(t, a, file.Content, true, out)
	files, err := menuDeliveryFiles(a)
	if err != nil || len(files) != 6 {
		t.Fatal("delivery inventory differs")
	}
	for _, f := range files {
		info, err := os.Stat(filepath.Join(delivery, f.name))
		if err != nil || info.Mode().Perm() != 0600 || info.Size() != int64(len(f.body)) {
			t.Fatal("delivery mode or size differs")
		}
	}
	rendered := make(map[string]string)
	for _, name := range []string{"reference.html", "typed.html", "enhanced.html", "enhanced-long.html", "disabled.html", "links-only.html"} {
		if info, err := os.Stat(filepath.Join(out, name)); err != nil || info.Size() == 0 || info.Mode().Perm() != 0600 {
			t.Fatal("compiled render missing")
		}
		hash, err := pricingFileSHA256(filepath.Join(out, name))
		if err != nil {
			t.Fatal("compiled render identity unavailable")
		}
		rendered[name] = hash
	}
	artifactHash, err := pricingFileSHA256(filepath.Join(delivery, "artifact.json"))
	if err != nil {
		t.Fatal("artifact identity unavailable")
	}
	receipt := map[string]any{"source": a.Source, "rule_sha256": a.RuleSHA256, "syntax": a.Syntax, "runtime": a.Runtime,
		"artifact_sha256": artifactHash, "rendered_sha256": rendered,
		"templ_sha256": a.TemplSHA256, "model_sha256": a.ModelSHA256, "css_sha256": a.CSSSHA256,
		"catalog_sha256": identities[paths[0]], "source_dom": "constant-reference-matched",
		"typed_dom": "explicit-trigger-links-post-csrf-checked", "templ_format_phases": []string{"unformatted", "formatted"},
		"verification": "not-verified", "browser": "not-tested", "consumer_adoption": "not-tested"}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil || os.WriteFile(filepath.Join(out, "compile-receipt.json"), append(encoded, '\n'), 0600) != nil {
		t.Fatal("compiler receipt write failed")
	}
	t.Logf("P07 original/reference DOM, typed compiler and immutable delivery proof retained at %s; no target certification", out)
}
