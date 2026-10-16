package designadapt

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
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

//go:embed navbar_routes_test.go.txt
var navbarRoutesTests string

func TestNavbarRepairReplayAndCompiler(t *testing.T) {
	body := []byte(syntheticNavbar)
	base, err := adaptNavbar(syntheticSource(syntheticNavbar), body)
	if err != nil {
		t.Fatal(err)
	}
	a, err := newNavbarRepairArtifact(base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := newNavbarRepairArtifact(base)
	if err != nil || !reflect.DeepEqual(a, b) || !reflect.DeepEqual(a.Base, base) || verifyNavbarRepairReplayWith(a, body, adaptNavbar) != nil {
		t.Fatal("repair replay/base isolation differs")
	}
	for _, field := range []string{"Adapter", "RuleSHA256", "CSSSHA256", "CSS", "RoutesGoSHA256", "RoutesGo", "RoutesTemplSHA256", "RoutesTempl", "WrapperAttribute", "Verification"} {
		t.Run(field, func(t *testing.T) {
			changed := a
			value := reflect.ValueOf(&changed).Elem().FieldByName(field)
			value.SetString(value.String() + "x")
			if verifyNavbarRepairReplayWith(changed, body, adaptNavbar) == nil {
				t.Fatal("mutated repair accepted")
			}
		})
	}
	changed := a
	changed.CSS += "\n/* unreviewed */"
	sum := sha256.Sum256([]byte(changed.CSS))
	changed.CSSSHA256 = hex.EncodeToString(sum[:])
	if verifyNavbarRepairReplayWith(changed, body, adaptNavbar) == nil {
		t.Fatal("self-rehashed repair accepted")
	}
	changed = a
	changed.Base.Defaults = append(bytes.Clone(changed.Base.Defaults), ' ')
	if verifyNavbarRepairReplayWith(changed, body, adaptNavbar) == nil {
		t.Fatal("nested base drift accepted")
	}
	if VerifyNavbarRepairReplay(a, body) == nil {
		t.Fatal("public adapter accepted synthetic source identity")
	}
	if strings.Contains(base.Templ+base.Model, "data-appkit-navbar-repair") || strings.Contains(a.RoutesTempl, "<script") || strings.Contains(a.CSS, ":defined") {
		t.Fatal("implicit repair/runtime ownership introduced")
	}
	runNavbarRepairCompiler(t, a, syntheticNavbar, false, "")
}

func TestNavbarRepairCSSRemainsExplicitAndRuntimeIndependent(t *testing.T) {
	css := navbarRepairCSS
	for strings.Contains(css, "/*") {
		start := strings.Index(css, "/*")
		end := strings.Index(css[start+2:], "*/")
		if end < 0 {
			t.Fatal("unterminated CSS comment")
		}
		css = css[:start] + css[start+2+end+2:]
	}
	for _, block := range strings.Split(css, "}") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		selectors, _, ok := strings.Cut(block, "{")
		if !ok {
			t.Fatal("malformed authored CSS block")
		}
		depth, start := 0, 0
		selectors += ","
		for i, c := range selectors {
			if c == '(' {
				depth++
			}
			if c == ')' {
				depth--
			}
			if c == ',' && depth == 0 {
				if !strings.HasPrefix(strings.TrimSpace(selectors[start:i]), "[data-appkit-navbar-repair]") {
					t.Fatal("unscoped repair selector")
				}
				start = i + 1
			}
		}
		if depth != 0 {
			t.Fatal("malformed selector parentheses")
		}
	}
	for _, forbidden := range []string{"!important", "@import", "url(", ":defined", "position:", "z-index:", "display: none", "visibility: hidden", "overflow: hidden"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("repair introduced hidden ownership/delivery: %s", forbidden)
		}
	}
	for _, required := range []string{"block-size: auto", "min-block-size: 4rem", "flex-wrap: wrap", "outline: 2px solid currentColor", "outline-offset: -2px", "overflow-wrap: anywhere", "details[data-appkit-navbar-routes] > summary"} {
		if !strings.Contains(css, required) {
			t.Fatalf("repair lost authored requirement: %s", required)
		}
	}
}

func runNavbarRepairCompiler(t *testing.T, a NavbarRepairArtifact, source string, private bool, output string) {
	t.Helper()
	dir := t.TempDir()
	dom := navbarDOMTests + "\nconst navbarSource = " + strconv.Quote(source) + "\nconst navbarDefaults = " + strconv.Quote(string(a.Base.Defaults)) + "\nconst navbarOutput = " + strconv.Quote(output) + "\n"
	routes := navbarRoutesTests + "\nconst navbarRepairOutput = " + strconv.Quote(output) + "\n"
	files := map[string]string{"navbar.templ": a.Base.Templ, "navbar.go": a.Base.Model, "navbar_test.go": navbarModelTests, "navbar_dom_test.go": dom, "navbar_routes.go": a.RoutesGo, "navbar_routes.templ": a.RoutesTempl, "navbar_routes_test.go": routes}
	for name, body := range files {
		if os.WriteFile(filepath.Join(dir, name), []byte(body), 0600) != nil {
			t.Fatal("compiler input write failed")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	run := func(t *testing.T, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
		if body, err := cmd.CombinedOutput(); err != nil {
			if private {
				if output != "" {
					_ = os.WriteFile(filepath.Join(output, "compiler-failure.txt"), body, 0600)
				}
				t.Fatal("private repair compilation failed; body withheld")
			}
			t.Fatalf("synthetic repair compilation failed: %v\n%s", err, body)
		}
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		t.Run(phase, func(t *testing.T) {
			for _, name := range []string{"navbar.templ", "navbar_routes.templ"} {
				if phase == "formatted" {
					run(t, "tool", "templ", "fmt", filepath.Join(dir, name))
				}
				run(t, "tool", "templ", "generate", "-f", filepath.Join(dir, name), "-include-version=false")
			}
			args := []string{"test", "-race", "-count=1"}
			for _, name := range []string{"navbar.go", "navbar_templ.go", "navbar_test.go", "navbar_dom_test.go", "navbar_routes.go", "navbar_routes_templ.go", "navbar_routes_test.go"} {
				args = append(args, filepath.Join(dir, name))
			}
			run(t, args...)
		})
	}
}

func TestNavbarRepairLive(t *testing.T) {
	kit := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kit == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kit) {
		t.Fatal("absolute private checkout required")
	}
	designRoot := filepath.Join(kit, "data/design-assets")
	_, archiveHash, _ := strings.Cut(NavbarArchive, "@")
	before := map[string]string{}
	for _, name := range []string{"catalog.json", filepath.Join("archives", archiveHash+".zip")} {
		p := filepath.Join(designRoot, name)
		hash, err := pricingFileSHA256(p)
		if err != nil {
			t.Fatal("source identity read failed")
		}
		before[p] = hash
	}
	t.Cleanup(func() {
		for file, hash := range before {
			if got, err := pricingFileSHA256(file); err != nil || got != hash {
				t.Error("source/catalog changed")
			}
		}
	})
	original, err := designassets.ReadFile(designRoot, NavbarArchive, NavbarMember)
	if err != nil {
		t.Fatal("verified original unavailable")
	}
	source := designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(original.Content)}
	a, err := AdaptNavbarRepair(source, []byte(original.Content))
	if err != nil || VerifyNavbarRepairReplay(a, []byte(original.Content)) != nil {
		t.Fatal("private repair replay failed")
	}
	base, err := json.Marshal(a.Base)
	if err != nil {
		t.Fatal(err)
	}
	base = append(base, '\n')
	sum := sha256.Sum256(base)
	if hex.EncodeToString(sum[:]) != "5559fb79f3d06d312df0a6a7bafe216815b5f34499b28a804862291d3f8cb642" {
		t.Fatal("frozen five-file base artifact drifted")
	}
	out := t.TempDir()
	if requested := os.Getenv("APPKIT_PROBE_NAVBAR_REPAIR_OUTPUT"); requested != "" {
		out = heroPrivateOutput(t, requested)
	}
	artifact, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	artifact = append(artifact, '\n')
	for name, body := range map[string][]byte{"artifact.json": artifact, "base-artifact.json": base, "original.html": []byte(original.Content), "navbar_repair.css": []byte(a.CSS), "navbar_routes.go": []byte(a.RoutesGo), "navbar_routes.templ": []byte(a.RoutesTempl), "navbar.templ": []byte(a.Base.Templ), "navbar.go": []byte(a.Base.Model)} {
		if os.WriteFile(filepath.Join(out, name), body, 0600) != nil {
			t.Fatal("private candidate write failed")
		}
	}
	runNavbarRepairCompiler(t, a, original.Content, true, out)
	if t.Failed() {
		return
	}
	renders := map[string]string{}
	for _, name := range []string{"reference.html", "typed.html", "minimal.html", "long.html", "multiple.html", "repair-typed.html", "repair-long.html", "repair-minimal.html", "repair-maximum.html", "repair-multiple.html"} {
		h, err := pricingFileSHA256(filepath.Join(out, name))
		if err != nil {
			t.Fatal("render missing")
		}
		renders[name] = h
	}
	hash := sha256.Sum256(artifact)
	receipt := map[string]any{"kind": "private-p09-repair-compiler", "artifact_sha256": hex.EncodeToString(hash[:]), "base_artifact_sha256": hex.EncodeToString(sum[:]), "rendered_sha256": renders, "source_unchanged": before, "phases": []string{"unformatted", "formatted"}, "verification": "not-verified", "browser": "not-tested", "delivery": "internal-candidate-not-cli"}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil || os.WriteFile(filepath.Join(out, "compile-receipt.json"), append(encoded, '\n'), 0600) != nil {
		t.Fatal("receipt write failed")
	}
	if baseline := os.Getenv("APPKIT_PROBE_NAVBAR_REPAIR_BASELINE"); baseline != "" {
		const frozenSHA = "91171695b41fe40b9e2982d834520ac1ef508a1ba6777bc4a924ee5d3c796a61"
		if !filepath.IsAbs(baseline) {
			t.Fatal("absolute frozen compiler receipt required")
		}
		if h, err := pricingFileSHA256(baseline); err != nil || h != frozenSHA {
			t.Fatal("frozen compiler receipt differs")
		}
		body, err := os.ReadFile(baseline)
		if err != nil {
			t.Fatal("frozen compiler receipt unavailable")
		}
		var frozen struct {
			Artifact string            `json:"artifact_sha256"`
			Renders  map[string]string `json:"rendered_sha256"`
		}
		if json.Unmarshal(body, &frozen) != nil || frozen.Artifact != hex.EncodeToString(hash[:]) || len(frozen.Renders) != 8 {
			t.Fatal("supplemental candidate no longer matches frozen repair")
		}
		for name, want := range frozen.Renders {
			if renders[name] != want {
				t.Fatal("frozen render changed during supplemental acceptance")
			}
		}
		fixtureHash := sha256.Sum256([]byte(navbarRoutesTests))
		supplement := map[string]any{
			"kind": "private-p09-repair-acceptance-fixtures", "artifact_sha256": frozen.Artifact,
			"repair_compile_receipt_sha256": frozenSHA,
			"rendered_sha256":               map[string]string{"repair-maximum.html": renders["repair-maximum.html"], "repair-multiple.html": renders["repair-multiple.html"]},
			"verification":                  "not-certified", "fixture_tests_sha256": hex.EncodeToString(fixtureHash[:]),
		}
		encoded, err = json.MarshalIndent(supplement, "", "  ")
		if err != nil || os.WriteFile(filepath.Join(out, "acceptance-fixtures.json"), append(encoded, '\n'), 0600) != nil {
			t.Fatal("supplemental fixture receipt write failed")
		}
		if h, err := pricingFileSHA256(baseline); err != nil || h != frozenSHA {
			t.Fatal("frozen compiler receipt changed")
		}
	}
	t.Logf("P09 repair private compiler candidate: %s; browser/adoption unproven", out)
}
