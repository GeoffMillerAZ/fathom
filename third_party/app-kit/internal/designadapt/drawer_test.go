package designadapt

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
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

// Structural fixture with first-party copy, no purchased classes or SVG geometry.
const syntheticDrawer = `<!-- Fixture -->
<!-- Host supplies runtime -->
<button command="show-modal" commandfor="drawer">Open fixture</button>
<el-dialog><dialog id="drawer"><div tabindex="0"><el-dialog-panel><div>
<div><div><h2 id="slide-over-heading">Fixture heading</h2><div>
<button type="button" command="close" commandfor="drawer"><span></span><span>Close fixture</span><svg viewBox="0 0 24 24"><path d="M0 0" /></svg></button>
</div></div></div>
<!-- Main -->
<div><div><div><div><img src="/fixture.svg" alt="" /></div>
<div><div><div><div><h3>Fixture person</h3><span><span>Fixture online</span></span></div><p>@fixture</p></div>
<div><button type="button">First action</button><button type="button">Second action</button><div><el-dropdown>
<button><span></span><span>Fixture options</span><svg viewBox="0 0 20 20"><path d="M0 0" /></svg></button>
<el-menu anchor="bottom end" popover><div><a href="#">First destination</a><a href="#">Second destination</a></div></el-menu>
</el-dropdown></div></div></div></div></div></div>
<div><dl>
<div><dt>First term</dt><dd><p>Fixture paragraph</p></dd></div>
<div><dt>Second term</dt><dd>Fixture location</dd></div>
<div><dt>Third term</dt><dd>fixture.test</dd></div>
<div><dt>Fourth term</dt><dd><time datetime="2000-02-29">Fixture date</time></dd></div>
</dl></div></div></div></el-dialog-panel></div></dialog></el-dialog>`

//go:embed drawer_model_test.go.txt
var drawerModelTests string

//go:embed drawer_dom_test.go.txt
var drawerDOMTests string

func TestDrawerSourceAndShapeRefusals(t *testing.T) {
	source := syntheticSource(syntheticDrawer)
	if _, err := AdaptDrawer(source, []byte(syntheticDrawer)); err == nil {
		t.Fatal("arbitrary source accepted")
	}
	source.ArchiveID, source.Path, source.SHA256 = DrawerArchive, DrawerMember, DrawerSourceSHA
	if _, err := AdaptDrawer(source, []byte(syntheticDrawer)); err == nil {
		t.Fatal("spoofed source accepted")
	}
	for name, body := range map[string]string{
		"extra-root":      "<div>" + syntheticDrawer + "</div>",
		"dialog-id":       strings.Replace(syntheticDrawer, `id="drawer"`, `id="other"`, 1),
		"dialog-command":  strings.Replace(syntheticDrawer, `command="show-modal"`, `command="show"`, 1),
		"trigger-submit":  strings.Replace(syntheticDrawer, `<button command=`, `<button type="submit" command=`, 1),
		"named-image":     strings.Replace(syntheticDrawer, `alt=""`, `alt="unreviewed"`, 1),
		"non-placeholder": strings.Replace(syntheticDrawer, `href="#"`, `href="/app"`, 1),
		"extra-prose":     syntheticDrawer + "Unexpected text",
		"variant-link":    strings.Replace(syntheticDrawer, `<a href="#">Second`, `<a class="variant" href="#">Second`, 1),
		"variant-detail":  strings.Replace(syntheticDrawer, "<dt>Second", "<dt class=\"variant\">Second", 1),
		"missing-date":    strings.Replace(syntheticDrawer, ` datetime="2000-02-29"`, "", 1),
		"inline-handler":  strings.Replace(syntheticDrawer, `<button command=`, `<button onclick="bad()" command=`, 1),
		"relocated-copy":  strings.Replace(syntheticDrawer, `<span></span><span>Close fixture</span>`, `<span>Close fixture</span><span></span>`, 1),
		"button-command":  strings.Replace(syntheticDrawer, `<button type="button">First action`, `<button type="button" command="close">First action`, 1),
		"button-form":     strings.Replace(syntheticDrawer, `<button type="button">First action`, `<button type="button" form="outside">First action`, 1),
		"button-disabled": strings.Replace(syntheticDrawer, `<button type="button">First action`, `<button type="button" disabled>First action`, 1),
		"paragraph-style": strings.Replace(syntheticDrawer, `<p>Fixture paragraph`, `<p class="unreviewed">Fixture paragraph`, 1),
		"date-style":      strings.Replace(syntheticDrawer, `<time datetime=`, `<time class="unreviewed" datetime=`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			a, err := adaptDrawer(syntheticSource(body), []byte(body))
			if err == nil || a.Templ != "" || a.Model != "" || a.Defaults != nil {
				t.Fatal("unreviewed source emitted output")
			}
		})
	}
}

func TestDrawerSyntheticReplayAndCompiler(t *testing.T) {
	a, err := adaptDrawer(syntheticSource(syntheticDrawer), []byte(syntheticDrawer))
	if err != nil {
		t.Fatal(err)
	}
	b, err := adaptDrawer(syntheticSource(syntheticDrawer), []byte(syntheticDrawer))
	if err != nil || !reflect.DeepEqual(a, b) || verifyDrawerReplayWith(a, []byte(syntheticDrawer), adaptDrawer) != nil {
		t.Fatal("replay differs")
	}
	if a.Verification != "not-verified" || a.Adapter != "p08-profile-drawer-v1" {
		t.Fatal("availability boundary changed")
	}
	for _, field := range []string{"Adapter", "RuleSHA256", "SyntaxSHA256", "TemplSHA256", "ModelSHA256", "DefaultsSHA256", "Templ", "Model", "Verification"} {
		t.Run(field, func(t *testing.T) {
			changed := a
			v := reflect.ValueOf(&changed).Elem().FieldByName(field)
			v.SetString(v.String() + "x")
			if verifyDrawerReplayWith(changed, []byte(syntheticDrawer), adaptDrawer) == nil {
				t.Fatal("modified artifact accepted")
			}
		})
	}
	changed := a
	changed.Runtime.ESMSHA256 = strings.Repeat("0", 64)
	if verifyDrawerReplayWith(changed, []byte(syntheticDrawer), adaptDrawer) == nil {
		t.Fatal("runtime mutation accepted")
	}
	changed = a
	changed.Defaults = append(append(json.RawMessage(nil), a.Defaults...), ' ')
	if verifyDrawerReplayWith(changed, []byte(syntheticDrawer), adaptDrawer) == nil {
		t.Fatal("defaults byte mutation accepted")
	}
	runDrawerCompiler(t, a, syntheticDrawer, false, "")
}

func runDrawerCompiler(t *testing.T, a DrawerArtifact, source string, private bool, output string) {
	t.Helper()
	dir := t.TempDir()
	fixture := drawerDOMTests + "\nconst drawerSource = " + strconv.Quote(source) + "\nconst drawerDefaults = " + strconv.Quote(string(a.Defaults)) + "\nconst drawerOutput = " + strconv.Quote(output) + "\n"
	for name, body := range map[string]string{"drawer.templ": a.Templ, "drawer.go": a.Model, "drawer_test.go": drawerModelTests, "drawer_dom_test.go": fixture} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal("compiler fixture write failed")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
		if private {
			if body, err := cmd.CombinedOutput(); err != nil {
				if output != "" {
					_ = os.WriteFile(filepath.Join(output, "compiler-failure.txt"), body, 0600)
				}
				t.Fatal("private compiler proof failed; diagnostics retained privately when requested")
			}
		} else if body, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("synthetic compiler proof: %v\n%s", err, body)
		}
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		t.Run(phase, func(t *testing.T) {
			if phase == "formatted" {
				run("tool", "templ", "fmt", filepath.Join(dir, "drawer.templ"))
			}
			run("tool", "templ", "generate", "-f", filepath.Join(dir, "drawer.templ"), "-include-version=false")
			run("test", "-race", "-count=1", filepath.Join(dir, "drawer.go"), filepath.Join(dir, "drawer_templ.go"), filepath.Join(dir, "drawer_test.go"), filepath.Join(dir, "drawer_dom_test.go"))
		})
	}
}

func TestAdaptDrawerLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_CONVERSION_KIT_ROOT to the private checkout")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("private root must be absolute")
	}
	designRoot := filepath.Join(root, "data/design-assets")
	_, archiveHash, _ := strings.Cut(DrawerArchive, "@")
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
	f, err := designassets.ReadFile(designRoot, DrawerArchive, DrawerMember)
	if err != nil {
		t.Fatal("verified original read failed")
	}
	source := designconvert.Source{ArchiveID: f.ArchiveID, Path: f.Path, SHA256: f.SHA256, Size: len(f.Content)}
	a, err := AdaptDrawer(source, []byte(f.Content))
	if err != nil {
		t.Fatal(err)
	}
	b, err := AdaptDrawer(source, []byte(f.Content))
	if err != nil || !reflect.DeepEqual(a, b) || VerifyDrawerReplay(a, []byte(f.Content)) != nil {
		t.Fatal("live replay differs")
	}
	out := t.TempDir()
	if retained := os.Getenv("APPKIT_PROBE_DRAWER_OUTPUT"); retained != "" {
		out = heroPrivateOutput(t, retained)
	}
	delivery := filepath.Join(out, "delivery")
	if WriteDrawerDelivery(delivery, a, []byte(f.Content)) != nil || CheckDrawerDelivery(delivery, a, []byte(f.Content)) != nil {
		t.Fatal("private delivery did not replay")
	}
	if WriteDrawerDelivery(delivery, a, []byte(f.Content)) == nil || !errors.Is(CheckDrawerDelivery(filepath.Join(out, "absent"), a, []byte(f.Content)), ErrDrawerDeliveryDrift) {
		t.Fatal("private existing/missing delivery contract changed")
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"artifact.json": encoded, "original.html": []byte(f.Content), "drawer.templ": []byte(a.Templ), "drawer.go": []byte(a.Model)} {
		if err := os.WriteFile(filepath.Join(out, name), body, 0600); err != nil {
			t.Fatal("private output write failed")
		}
	}
	runDrawerCompiler(t, a, f.Content, true, out)
	if t.Failed() {
		return
	}
	renders := map[string]string{}
	for _, name := range []string{"reference.html", "typed.html", "readonly.html", "long.html", "multiple.html"} {
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
	receipt := map[string]any{"source": a.Source, "rule_sha256": a.RuleSHA256, "syntax": a.Syntax, "runtime": a.Runtime, "rendered_sha256": renders, "artifact_sha256": artifactHash, "templ_sha256": a.TemplSHA256, "model_sha256": a.ModelSHA256, "catalog_sha256": before[paths[0]], "source_dom": "constant-reference-matched", "typed_dom": "explicit-id-name-images-links-details-checked", "templ_format_phases": []string{"unformatted", "formatted"}, "verification": "not-verified", "browser": "not-tested", "consumer_adoption": "not-tested"}
	encoded, err = json.MarshalIndent(receipt, "", "  ")
	if err != nil || os.WriteFile(filepath.Join(out, "compile-receipt.json"), append(encoded, '\n'), 0600) != nil {
		t.Fatal("receipt write failed")
	}
	t.Logf("P08 private compiler/reference and typed binding proof at %s; browser/adoption unproven", out)
}
