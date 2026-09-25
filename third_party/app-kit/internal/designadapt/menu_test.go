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
)

// First-party structure/copy only, no paid classes or icon geometry.
const syntheticMenu = `<!-- Synthetic fixture -->
<!-- Runtime supplied by test host -->
<el-dropdown><button> Fixture menu <svg viewBox="0 0 20 20" aria-hidden="true"><path d="M0 0" /></svg></button>
<el-menu anchor="bottom end" popover><div>
<a href="#">First fixture</a>
<a href="#">Second fixture</a>
<a href="#">Third fixture</a>
<form action="#" method="POST"><button type="submit">Fixture action</button></form>
</div></el-menu></el-dropdown>`

//go:embed menu_model_test.go.txt
var menuModelTests string

//go:embed menu_dom_test.go.txt
var menuDOMTests string

func TestMenuSourceAndShapeRefusals(t *testing.T) {
	source := syntheticSource(syntheticMenu)
	if a, err := AdaptMenu(source, []byte(syntheticMenu)); err == nil || a.Templ != "" {
		t.Fatal("arbitrary source accepted")
	}
	source.ArchiveID, source.Path, source.SHA256 = MenuArchive, MenuMember, MenuSourceSHA
	if _, err := AdaptMenu(source, []byte(syntheticMenu)); err == nil {
		t.Fatal("spoofed source identity accepted")
	}
	for name, body := range map[string]string{
		"extra-root":      "<div>" + syntheticMenu + "</div>",
		"missing-link":    strings.Replace(syntheticMenu, `<a href="#">First fixture</a>`, "", 1),
		"non-placeholder": strings.Replace(syntheticMenu, `href="#"`, `href="/destination"`, 1),
		"form-get":        strings.Replace(syntheticMenu, `method="POST"`, `method="GET"`, 1),
		"submit-reset":    strings.Replace(syntheticMenu, `type="submit"`, `type="reset"`, 1),
		"trigger-submit":  strings.Replace(syntheticMenu, "<button>", `<button type="submit">`, 1),
		"missing-action":  strings.Replace(syntheticMenu, ` action="#"`, "", 1),
		"empty-label":     strings.Replace(syntheticMenu, "First fixture", "", 1),
		"variant-link":    strings.Replace(syntheticMenu, `<a href="#">Second`, `<a href="#" class="different">Second`, 1),
		"inline-handler":  strings.Replace(syntheticMenu, "<button>", `<button onclick="bad()">`, 1),
		"extra-copy":      syntheticMenu + "Extra",
	} {
		t.Run(name, func(t *testing.T) {
			a, err := adaptMenu(syntheticSource(body), []byte(body))
			if err == nil || a.Templ != "" || a.Model != "" || a.Defaults != nil {
				t.Fatal("unsupported source produced output")
			}
		})
	}
}

func TestMenuSyntheticReplayAndCompiler(t *testing.T) {
	a, err := adaptMenu(syntheticSource(syntheticMenu), []byte(syntheticMenu))
	if err != nil {
		t.Fatal(err)
	}
	b, err := adaptMenu(syntheticSource(syntheticMenu), []byte(syntheticMenu))
	if err != nil || !reflect.DeepEqual(a, b) || verifyMenuReplayWith(a, []byte(syntheticMenu), adaptMenu) != nil {
		t.Fatal("replay differs")
	}
	if a.Verification != "not-verified" || a.Adapter != "p07-menu-v1" || a.Runtime.Version != "1.0.22" {
		t.Fatal("availability boundary changed")
	}
	for _, field := range []string{"Adapter", "RuleSHA256", "SyntaxSHA256", "TemplSHA256", "ModelSHA256", "CSSSHA256", "DefaultsSHA256", "Templ", "Model", "CSS", "Verification"} {
		t.Run(field, func(t *testing.T) {
			changed := a
			v := reflect.ValueOf(&changed).Elem().FieldByName(field)
			v.SetString(v.String() + "x")
			if verifyMenuReplayWith(changed, []byte(syntheticMenu), adaptMenu) == nil {
				t.Fatal("changed artifact passed")
			}
		})
	}
	changed := a
	changed.Runtime.ESMSHA256 = strings.Repeat("0", 64)
	if verifyMenuReplayWith(changed, []byte(syntheticMenu), adaptMenu) == nil {
		t.Fatal("changed runtime passed")
	}
	changed = a
	changed.Defaults = append(append(json.RawMessage(nil), a.Defaults...), ' ')
	if verifyMenuReplayWith(changed, []byte(syntheticMenu), adaptMenu) == nil {
		t.Fatal("changed defaults passed")
	}
	runMenuCompiler(t, a, syntheticMenu, false, "")
}

func runMenuCompiler(t *testing.T, a MenuArtifact, source string, private bool, output string) {
	t.Helper()
	dir := t.TempDir()
	fixture := menuDOMTests + "\nconst menuSource = " + strconv.Quote(source) + "\nconst menuDefaults = " + strconv.Quote(string(a.Defaults)) + "\nconst menuOutput = " + strconv.Quote(output) + "\n"
	for name, content := range map[string]string{"menu.templ": a.Templ, "menu.go": a.Model, "menu_test.go": menuModelTests, "menu_dom_test.go": fixture} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
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
			if err := cmd.Run(); err != nil {
				t.Fatal("private compiler proof failed; source diagnostics withheld")
			}
		} else if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("synthetic compiler proof: %v\n%s", err, out)
		}
	}
	for _, phase := range []string{"unformatted", "formatted"} {
		t.Run(phase, func(t *testing.T) {
			if phase == "formatted" {
				run("tool", "templ", "fmt", filepath.Join(dir, "menu.templ"))
			}
			run("tool", "templ", "generate", "-f", filepath.Join(dir, "menu.templ"), "-include-version=false")
			run("test", "-count=1", filepath.Join(dir, "menu.go"), filepath.Join(dir, "menu_templ.go"), filepath.Join(dir, "menu_test.go"), filepath.Join(dir, "menu_dom_test.go"))
		})
	}
}
