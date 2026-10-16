package designadapt

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

// First-party structural fixture. No purchased copy, classes or SVG geometry.
const syntheticNavbar = `<!-- Fixture -->
<!-- Host supplies runtime -->
<nav class="fixture-nav"><div><div><div>
<!-- Mobile control -->
<button type="button" command="--toggle" commandfor="mobile-menu"><span></span><span>Toggle fixture navigation</span><svg><path d="M0 0" /></svg><svg><path d="M1 1" /></svg></button>
</div><div><div><img src="/source-light.svg" alt="Fixture brand" /><img src="/source-dark.svg" alt="Fixture brand" /></div><div>
<!-- Desktop links -->
<a href="#" class="desktop-current">First route</a><a href="#" class="desktop-default">Second route</a><a href="#" class="desktop-default">Third route</a><a href="#" class="desktop-default">Fourth route</a>
</div></div><div>
<button type="button"><span></span><span>Fixture notices</span><svg><path d="M2 2" /></svg></button>
<!-- Account -->
<el-dropdown><button><span></span><span>Fixture account</span><img src="/source-avatar.svg" alt="" /></button>
<el-menu anchor="bottom end" popover><a href="#" class="account-link">First account route</a><a href="#" class="account-link">Second account route</a><a href="#" class="account-link">Third account route</a></el-menu>
</el-dropdown></div></div></div>
<el-disclosure id="mobile-menu" hidden><div>
<!-- Mobile links -->
<a href="#" class="mobile-current">First route</a><a href="#" class="mobile-default">Second route</a><a href="#" class="mobile-default">Third route</a><a href="#" class="mobile-default">Fourth route</a>
</div></el-disclosure></nav>`

//go:embed navbar_model_test.go.txt
var navbarModelTests string

//go:embed navbar_dom_test.go.txt
var navbarDOMTests string

func TestNavbarSourceAndShapeRefusals(t *testing.T) {
	source := syntheticSource(syntheticNavbar)
	if _, err := AdaptNavbar(source, []byte(syntheticNavbar)); err == nil {
		t.Fatal("arbitrary source accepted")
	}
	source.ArchiveID, source.Path, source.SHA256 = NavbarArchive, NavbarMember, NavbarSourceSHA
	if _, err := AdaptNavbar(source, []byte(syntheticNavbar)); err == nil {
		t.Fatal("spoofed bytes accepted")
	}
	for name, body := range map[string]string{
		"extra-root":          "<div>" + syntheticNavbar + "</div>",
		"extra-copy":          syntheticNavbar + "Extra text",
		"command":             strings.Replace(syntheticNavbar, `command="--toggle"`, `command="show"`, 1),
		"target":              strings.Replace(syntheticNavbar, `commandfor="mobile-menu"`, `commandfor="other"`, 1),
		"submit":              strings.Replace(syntheticNavbar, `type="button"`, `type="submit"`, 1),
		"placeholder":         strings.Replace(syntheticNavbar, `href="#"`, `href="/real"`, 1),
		"variant":             strings.Replace(syntheticNavbar, `class="desktop-default">Third`, `class="variant">Third`, 1),
		"paired-copy":         strings.Replace(syntheticNavbar, `class="mobile-current">First route`, `class="mobile-current">Other route`, 1),
		"account-alt":         strings.Replace(syntheticNavbar, `alt=""`, `alt="unreviewed"`, 1),
		"brand-alt":           strings.Replace(syntheticNavbar, `alt="Fixture brand"`, `alt="Different brand"`, 1),
		"image-missing":       strings.Replace(syntheticNavbar, `src="/source-light.svg"`, `src=""`, 1),
		"menu-anchor":         strings.Replace(syntheticNavbar, `anchor="bottom end"`, `anchor="top start"`, 1),
		"notification-action": strings.Replace(syntheticNavbar, `<button type="button"><span>`, `<button type="button" form="outside"><span>`, 1),
		"menu-submit":         strings.Replace(syntheticNavbar, `<el-dropdown><button>`, `<el-dropdown><button type="submit">`, 1),
		"landmark-label":      strings.Replace(syntheticNavbar, `<nav class=`, `<nav aria-label="unexpected" class=`, 1),
		"inline-handler":      strings.Replace(syntheticNavbar, `<nav class=`, `<nav onclick="bad()" class=`, 1),
		"relocated-label":     strings.Replace(syntheticNavbar, `<span></span><span>Toggle fixture navigation</span>`, `<span>Toggle fixture navigation</span><span></span>`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			a, err := adaptNavbar(syntheticSource(body), []byte(body))
			if err == nil || a.Templ != "" || a.Model != "" || a.Defaults != nil {
				t.Fatal("unreviewed source emitted output")
			}
		})
	}
}

func TestNavbarSyntheticReplayAndCompiler(t *testing.T) {
	a, err := adaptNavbar(syntheticSource(syntheticNavbar), []byte(syntheticNavbar))
	if err != nil {
		t.Fatal(err)
	}
	b, err := adaptNavbar(syntheticSource(syntheticNavbar), []byte(syntheticNavbar))
	if err != nil || !reflect.DeepEqual(a, b) || verifyNavbarReplayWith(a, []byte(syntheticNavbar), adaptNavbar) != nil {
		t.Fatal("replay differs")
	}
	if a.Verification != "not-verified" || a.Adapter != "p09-responsive-navbar-v1" {
		t.Fatal("availability boundary changed")
	}
	for _, field := range []string{"Adapter", "RuleSHA256", "SyntaxSHA256", "TemplSHA256", "ModelSHA256", "DefaultsSHA256", "Templ", "Model", "Verification"} {
		t.Run(field, func(t *testing.T) {
			changed := a
			v := reflect.ValueOf(&changed).Elem().FieldByName(field)
			v.SetString(v.String() + "x")
			if verifyNavbarReplayWith(changed, []byte(syntheticNavbar), adaptNavbar) == nil {
				t.Fatal("modified artifact accepted")
			}
		})
	}
	changed := a
	changed.Runtime.ESMSHA256 = strings.Repeat("0", 64)
	if verifyNavbarReplayWith(changed, []byte(syntheticNavbar), adaptNavbar) == nil {
		t.Fatal("runtime mutation accepted")
	}
	changed = a
	changed.Defaults = append(append(json.RawMessage(nil), a.Defaults...), ' ')
	if verifyNavbarReplayWith(changed, []byte(syntheticNavbar), adaptNavbar) == nil {
		t.Fatal("defaults byte mutation accepted")
	}
	runNavbarCompiler(t, a, syntheticNavbar, false, "")
}

func runNavbarCompiler(t *testing.T, a NavbarArtifact, source string, private bool, output string) {
	t.Helper()
	dir := t.TempDir()
	fixture := navbarDOMTests + "\nconst navbarSource = " + strconv.Quote(source) + "\nconst navbarDefaults = " + strconv.Quote(string(a.Defaults)) + "\nconst navbarOutput = " + strconv.Quote(output) + "\n"
	for name, body := range map[string]string{"navbar.templ": a.Templ, "navbar.go": a.Model, "navbar_test.go": navbarModelTests, "navbar_dom_test.go": fixture} {
		if os.WriteFile(filepath.Join(dir, name), []byte(body), 0600) != nil {
			t.Fatal("compiler fixture write failed")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
		if body, err := cmd.CombinedOutput(); err != nil {
			if private {
				if output != "" {
					_ = os.WriteFile(filepath.Join(output, "compiler-failure.txt"), body, 0600)
				}
				t.Fatal("private compiler proof failed; diagnostics retained privately when requested")
			}
			t.Fatalf("synthetic compiler proof: %v\n%s", err, body)
		}
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		t.Run(phase, func(t *testing.T) {
			if phase == "formatted" {
				run("tool", "templ", "fmt", filepath.Join(dir, "navbar.templ"))
			}
			run("tool", "templ", "generate", "-f", filepath.Join(dir, "navbar.templ"), "-include-version=false")
			run("test", "-race", "-count=1", filepath.Join(dir, "navbar.go"), filepath.Join(dir, "navbar_templ.go"), filepath.Join(dir, "navbar_test.go"), filepath.Join(dir, "navbar_dom_test.go"))
		})
	}
}

func TestAdaptNavbarLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_CONVERSION_KIT_ROOT to the private checkout")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("private root must be absolute")
	}
	designRoot := filepath.Join(root, "data/design-assets")
	_, archiveHash, _ := strings.Cut(NavbarArchive, "@")
	paths := []string{filepath.Join(designRoot, "catalog.json"), filepath.Join(designRoot, "archives", archiveHash+".zip")}
	before := map[string]string{}
	for _, p := range paths {
		h, err := pricingFileSHA256(p)
		if err != nil {
			t.Fatal("source identity read failed")
		}
		before[p] = h
	}
	if before[paths[1]] != archiveHash {
		t.Fatal("archive mismatch")
	}
	t.Cleanup(func() {
		for p, h := range before {
			if got, err := pricingFileSHA256(p); err != nil || got != h {
				t.Error("original source/catalog changed")
			}
		}
	})
	f, err := designassets.ReadFile(designRoot, NavbarArchive, NavbarMember)
	if err != nil {
		t.Fatal("verified original read failed")
	}
	source := designconvert.Source{ArchiveID: f.ArchiveID, Path: f.Path, SHA256: f.SHA256, Size: len(f.Content)}
	a, err := AdaptNavbar(source, []byte(f.Content))
	if err != nil {
		t.Fatal(err)
	}
	b, err := AdaptNavbar(source, []byte(f.Content))
	if err != nil || !reflect.DeepEqual(a, b) || VerifyNavbarReplay(a, []byte(f.Content)) != nil {
		t.Fatal("live replay differs")
	}
	out := t.TempDir()
	if retained := os.Getenv("APPKIT_PROBE_NAVBAR_OUTPUT"); retained != "" {
		out = heroPrivateOutput(t, retained)
	}
	delivery := filepath.Join(out, "delivery")
	if WriteNavbarDelivery(delivery, a, []byte(f.Content)) != nil || CheckNavbarDelivery(delivery, a, []byte(f.Content)) != nil {
		t.Fatal("private delivery did not replay")
	}
	if WriteNavbarDelivery(delivery, a, []byte(f.Content)) == nil {
		t.Fatal("existing private delivery overwritten")
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"artifact.json": encoded, "original.html": []byte(f.Content), "navbar.templ": []byte(a.Templ), "navbar.go": []byte(a.Model)} {
		if os.WriteFile(filepath.Join(out, name), body, 0600) != nil {
			t.Fatal("private output write failed")
		}
	}
	runNavbarCompiler(t, a, f.Content, true, out)
	if t.Failed() {
		return
	}
	renders := map[string]string{}
	for _, name := range []string{"reference.html", "typed.html", "minimal.html", "long.html", "multiple.html"} {
		h, err := pricingFileSHA256(filepath.Join(out, name))
		if err != nil {
			t.Fatal("render missing")
		}
		renders[name] = h
	}
	artifactHash, err := pricingFileSHA256(filepath.Join(delivery, "artifact.json"))
	if err != nil {
		t.Fatal("delivery artifact hash unavailable")
	}
	receipt := map[string]any{"source": a.Source, "rule_sha256": a.RuleSHA256, "syntax": a.Syntax, "runtime": a.Runtime, "rendered_sha256": renders, "artifact_sha256": artifactHash, "templ_sha256": a.TemplSHA256, "model_sha256": a.ModelSHA256, "catalog_sha256": before[paths[0]], "source_dom": "constant-reference-matched", "typed_dom": "explicit-id-name-images-current-dual-links-and-optional-destinations-checked", "templ_format_phases": []string{"unformatted", "formatted"}, "verification": "not-verified", "browser": "not-tested", "consumer_adoption": "not-tested"}
	encoded, err = json.MarshalIndent(receipt, "", "  ")
	if err != nil || os.WriteFile(filepath.Join(out, "compile-receipt.json"), append(encoded, '\n'), 0600) != nil {
		t.Fatal("receipt write failed")
	}
	t.Logf("P09 private compiler/reference and typed binding proof at %s; browser/adoption unproven", out)
}
