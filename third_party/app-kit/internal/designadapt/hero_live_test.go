package designadapt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

// This opt-in probe reads the owner's frozen source without modifying its
// archive/catalog. Compiler output is withheld to avoid leaking licensed copy.
func TestAdaptHeroLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_CONVERSION_KIT_ROOT to an existing private app-kit checkout")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("private checkout path must be absolute")
	}
	designRoot := filepath.Join(root, "data/design-assets")
	_, archiveHash, _ := strings.Cut(HeroArchive, "@")
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
	file, err := designassets.ReadFile(designRoot, HeroArchive, HeroMember)
	if err != nil {
		t.Fatal("verified original read failed; source/error details withheld")
	}
	source := designconvert.Source{ArchiveID: file.ArchiveID, Path: file.Path, SHA256: file.SHA256, Size: len(file.Content)}
	a, err := AdaptHero(source, []byte(file.Content))
	if err != nil {
		t.Fatal("frozen hero adaptation failed")
	}
	b, err := AdaptHero(source, []byte(file.Content))
	if err != nil || !reflect.DeepEqual(a, b) || VerifyHeroReplay(a, []byte(file.Content)) != nil {
		t.Fatal("frozen hero replay failed")
	}
	for _, mutate := range []func(*HeroArtifact){
		func(a *HeroArtifact) { a.SourceAssets.ImageURL += "?changed" },
		func(a *HeroArtifact) { a.Source.Size++ },
		func(a *HeroArtifact) { a.Templ += " " },
		func(a *HeroArtifact) { a.Verification = "verified" },
	} {
		changed := a
		mutate(&changed)
		if VerifyHeroReplay(changed, []byte(file.Content)) == nil {
			t.Fatal("changed source/artifact accepted")
		}
	}
	out := t.TempDir()
	if retained := os.Getenv("APPKIT_PROBE_HERO_OUTPUT"); retained != "" {
		out = heroPrivateOutput(t, retained)
	}
	if err := WriteHeroDelivery(filepath.Join(out, "delivery"), a, []byte(file.Content)); err != nil {
		t.Fatal("private delivery failed")
	}
	if err := CheckHeroDelivery(filepath.Join(out, "delivery"), a, []byte(file.Content)); err != nil {
		t.Fatal("private delivery replay failed")
	}
	if err := os.WriteFile(filepath.Join(out, "original.html"), []byte(file.Content), 0600); err != nil {
		t.Fatal("private original specimen write failed")
	}
	runHeroCompiler(t, a, file.Content, true, out)
	for _, name := range []string{"reference.html", "typed.html", "enhanced.html", "typed-long.html", "enhanced-long.html"} {
		if info, err := os.Stat(filepath.Join(out, name)); err != nil || info.Size() == 0 || info.Mode().Perm() != 0600 {
			t.Fatal("private compiled render missing")
		}
	}
	receipt := map[string]any{
		"source": a.Source, "rule_sha256": a.RuleSHA256, "syntax": a.Syntax,
		"templ_sha256": a.TemplSHA256, "model_sha256": a.ModelSHA256, "css_sha256": a.CSSSHA256,
		"catalog_sha256": identities[paths[0]], "source_dom": "matched-except-six-declared-urls",
		"templ_format_phases": []string{"unformatted", "formatted"}, "verification": "not-verified",
		"assets": "source-urls-preserved-not-fetched", "browser": "not-tested", "consumer_adoption": "not-tested",
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil || os.WriteFile(filepath.Join(out, "compile-receipt.json"), append(encoded, '\n'), 0600) != nil {
		t.Fatal("private compiler receipt write failed")
	}
	t.Logf("P01 source/compiler/DOM/private-delivery proof retained at %s; no browser or target certification", out)
}

func heroPrivateOutput(t *testing.T, dir string) string {
	t.Helper()
	clean := filepath.Clean(dir)
	resolved, err := filepath.EvalSymlinks(clean)
	tempRoot, tempErr := filepath.EvalSymlinks(os.TempDir())
	rel, relErr := filepath.Rel(tempRoot, resolved)
	info, statErr := os.Stat(clean)
	entries, readErr := os.ReadDir(clean)
	if !filepath.IsAbs(dir) || err != nil || tempErr != nil || relErr != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || statErr != nil || !info.IsDir() || info.Mode().Perm() != 0700 || readErr != nil || len(entries) != 0 {
		t.Fatal("retained hero output must be an existing empty private directory beneath system temp")
	}
	// A macOS /var alias may be present in the system temp prefix. Reject links
	// within the task-owned suffix rather than rejecting that platform alias.
	if clean != filepath.Join(os.TempDir(), rel) && clean != resolved {
		t.Fatal("retained hero output may not traverse a task-owned symbolic link")
	}
	return resolved
}
