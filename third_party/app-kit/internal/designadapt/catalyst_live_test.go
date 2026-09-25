package designadapt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

// Explicit licensed-input rehearsal, never an implicit dependency of source-only
// tests. This compiles the complete form with REAL model/view helpers (not stubs).
// It does not certify browser fidelity, HTTP behavior, or consumer integration.
func TestCatalystPrivateCompiledForm(t *testing.T) {
	packet := os.Getenv("APPKIT_CATALYST_SOURCE_PACKET")
	if packet == "" {
		t.Skip("select explicit private Catalyst source packet")
	}
	if !filepath.IsAbs(packet) || filepath.Clean(packet) != packet {
		t.Fatal("private packet must be an explicit clean absolute path")
	}
	info, err := os.Lstat(packet)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("private packet must be a real existing directory")
	}
	read := func(name string, limit int64) []byte {
		t.Helper()
		if filepath.Base(name) != name {
			t.Fatal("non-flat private input")
		}
		p := filepath.Join(packet, name)
		st, err := os.Lstat(p)
		if err != nil || !st.Mode().IsRegular() || st.Size() > limit {
			t.Fatalf("invalid private input %s", name)
		}
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	referenceBytes := read("reference.json", 1<<20)
	if catalystDigest(referenceBytes) != "94d3ab4c145e33bf29af750918429833d833fff3f9ac8820e1314c7f2c67f53d" {
		t.Fatal("private source reference drift")
	}
	type filePin struct {
		Name, SHA256 string
		Bytes        int
	}
	var reference struct {
		Default                               filePin `json:"default_html"`
		Errors                                filePin `json:"error_html"`
		Countries, Defaults, Currencies, Plan filePin
		Assets                                []struct {
			URL, MIME string
			File      filePin
		}
		Licenses []filePin
		Frames   []struct{ Screenshot filePin }
	}
	if err := json.Unmarshal(referenceBytes, &reference); err != nil {
		t.Fatal(err)
	}
	pinned := func(pin filePin) []byte {
		t.Helper()
		body := read(pin.Name, 16<<20)
		if len(body) != pin.Bytes || catalystDigest(body) != pin.SHA256 {
			t.Fatalf("private input drift: %s", pin.Name)
		}
		return body
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("private compiled form: %v\n%s", err, out)
		}
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	run(root, "tool", "cue", "vet", "-d", "#Reference", filepath.Join(root, "docs/assessments/fixtures/catalyst-settings-adapter/reference.cue"), filepath.Join(packet, "reference.json"))
	body := pinned(reference.Default)
	errors := pinned(reference.Errors)
	for _, pin := range []filePin{reference.Plan, reference.Countries, reference.Defaults, reference.Currencies} {
		pinned(pin)
	}
	for _, asset := range reference.Assets {
		pinned(asset.File)
	}
	for _, pin := range reference.Licenses {
		pinned(pin)
	}
	for _, frame := range reference.Frames {
		pinned(frame.Screenshot)
	}
	first, err := AdaptCatalystForm(body)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AdaptCatalystForm(body)
	if err != nil || first != second {
		t.Fatal("private full form did not replay exactly")
	}
	errorDOM, err := html.Parse(bytes.NewReader(errors))
	if err != nil {
		t.Fatal(err)
	}
	var errorClass string
	errorNodes := 0
	for _, p := range catalystElements(errorDOM, "p") {
		if catalystAttr(p, "data-slot") == "error" {
			selected := catalystAttr(p, "class")
			if errorClass != "" && errorClass != selected {
				t.Fatal("source error styles disagree")
			}
			errorClass = selected
			errorNodes++
		}
	}
	if errorClass == "" || errorNodes != 2 {
		t.Fatal("source error appearance unavailable")
	}
	dir := t.TempDir()
	if parent := os.Getenv("APPKIT_CATALYST_BUILD_PARENT"); parent != "" {
		st, err := os.Lstat(parent)
		if !filepath.IsAbs(parent) || err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			t.Fatal("build parent must be an explicit existing private directory")
		}
		dir, err = os.MkdirTemp(parent, "p10-go-")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("retained private generated output (not certified): %s", dir)
	}
	write := func(name string, body []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("settings.templ", []byte(first))
	for source, target := range map[string]string{
		"catalyst_model.go.txt": "policy.go", "catalyst_view.go.txt": "view.go",
		"catalyst_native.go.txt": "catalyst_native.go", "catalyst_native.css": "catalyst_native.css",
	} {
		code, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		write(target, code)
	}
	write("go.mod", []byte("module example.test/p10\n\ngo 1.26.2\n\nrequire github.com/a-h/templ v0.3.1020\n"))
	sums, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	write("go.sum", sums)
	tests, err := os.ReadFile("catalyst_runtime_test.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	constants := "\nconst sourceCountries = " + strconv.Quote(string(pinned(reference.Countries))) + "\nconst sourceDefaults = " + strconv.Quote(string(pinned(reference.Defaults))) + "\nconst sourceCurrencies = " + strconv.Quote(string(pinned(reference.Currencies))) + "\nconst sourceErrorClass = " + strconv.Quote(errorClass) + "\n"
	write("settings_test.go", append(tests, []byte(constants)...))
	serverDir := filepath.Join(dir, "cmd", "server")
	if err := os.MkdirAll(serverDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"server.go", "server_test.go"} {
		code, err := os.ReadFile(filepath.Join(root, "docs/assessments/fixtures/catalyst-settings-adapter", name+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(serverDir, name), code, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("cmd/server/appearance.go", []byte("package main\nconst sourceErrorClass = "+strconv.Quote(errorClass)+"\n"))
	controller, err := os.ReadFile("catalyst_enhance.mjs")
	if err != nil {
		t.Fatal(err)
	}
	write("cmd/server/controller.mjs", controller)
	// Resolve the kit-pinned tool through its modfile, but generate inside the
	// private module. Otherwise templ bakes the random parent path into errors.
	run(dir, "tool", "-modfile", filepath.Join(root, "go.mod"), "templ", "generate", "-f", "settings.templ", "-include-version=false")
	generatedGo, err := os.ReadFile(filepath.Join(dir, "settings_templ.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := checkCatalystGeneratedLocations(generatedGo); err != nil {
		t.Fatal(err)
	}
	run(dir, "test", "-race", "-count=1", "-v", "./...")
	run(dir, "build", "-trimpath", "-buildvcs=false", "-o", filepath.Join(dir, "fixture"), "./cmd/server")
	for _, name := range []string{"rendered-default.html", "rendered-errors.html"} {
		rendered, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := checkCatalystNativeStructure(rendered); err != nil {
			t.Fatal(err)
		}
	}
	// Freeze identities of the preparatory generated files. This is not a target
	// receipt, complete source closure, export API, or browser verification claim.
	var records []string
	for _, name := range []string{"settings.templ", "settings_templ.go", "policy.go", "view.go", "catalyst_native.go", "catalyst_native.css", "settings_test.go", "go.mod", "go.sum", "rendered-default.html", "rendered-errors.html", "cmd/server/server.go", "cmd/server/server_test.go", "cmd/server/appearance.go", "cmd/server/controller.mjs", "fixture"} {
		code, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, fmt.Sprintf("%s  %s", catalystDigest(code), name))
	}
	write("SHA256SUMS", []byte(strings.Join(records, "\n")+"\n"))
	t.Logf("complete native form compiles and scoped runtime assertions pass; source %s; no browser/certification claim", CatalystFormSourceSHA256)
}

func checkCatalystGeneratedLocations(body []byte) error {
	file, err := parser.ParseFile(token.NewFileSet(), "generated.go", body, 0)
	if err != nil {
		return err
	}
	locations := 0
	var mismatch bool
	ast.Inspect(file, func(n ast.Node) bool {
		literal, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		kind, ok := literal.Type.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		owner, ok := kind.X.(*ast.Ident)
		if !ok || owner.Name != "templ" || kind.Sel.Name != "Error" {
			return true
		}
		for _, element := range literal.Elts {
			pair, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := pair.Key.(*ast.Ident)
			if !ok || key.Name != "FileName" {
				continue
			}
			locations++
			value, ok := pair.Value.(*ast.BasicLit)
			if !ok || value.Kind != token.STRING {
				mismatch = true
				continue
			}
			name, err := strconv.Unquote(value.Value)
			if err != nil || name != "settings.templ" {
				mismatch = true
			}
		}
		return true
	})
	if locations == 0 || mismatch {
		return fmt.Errorf("generated templ locations must use stable module-relative settings.templ")
	}
	return nil
}

func TestCatalystGeneratedLocationCanaries(t *testing.T) {
	for _, test := range []struct {
		source string
		valid  bool
	}{
		{`package sample; var _ = templ.Error{FileName:"settings.templ"}`, true},
		{`package sample; var _ = templ.Error{FileName:"data/design-assets/random/settings.templ"}`, false},
		{`package sample; var _ = templ.Error{FileName:variable}`, false},
		{`package sample`, false},
	} {
		if err := checkCatalystGeneratedLocations([]byte(test.source)); (err == nil) != test.valid {
			t.Fatalf("location admission: %v", err)
		}
	}
}

func checkCatalystNativeStructure(body []byte) error {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return err
	}
	forms := catalystElements(doc, "form")
	if len(forms) != 2 || !catalystHasAttr(forms[0], "data-p10-settings") || !catalystHasAttr(forms[1], "data-p10-reset") || !catalystHasAttr(forms[1], "hidden") {
		return fmt.Errorf("complete form missing")
	}
	ids := map[string]*html.Node{}
	var nodes []*html.Node
	var duplicate string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			nodes = append(nodes, n)
			if id := catalystAttr(n, "id"); id != "" {
				if ids[id] != nil {
					duplicate = id
				}
				ids[id] = n
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if duplicate != "" {
		return fmt.Errorf("duplicate rendered ID: %s", duplicate)
	}
	for _, n := range nodes {
		for _, key := range []string{"aria-describedby", "aria-labelledby", "for", "form"} {
			for _, id := range strings.Fields(catalystAttr(n, key)) {
				if ids[id] == nil {
					return fmt.Errorf("broken rendered %s reference %s", key, id)
				}
			}
		}
		if catalystHasAttr(n, "disabled") && (n.Data == "fieldset" || n.Data == "input" || n.Data == "textarea" || n.Data == "select" || n.Data == "button") {
			return fmt.Errorf("native form control remains disabled")
		}
		if n.Data == "select" {
			selected := 0
			for _, option := range catalystElements(n, "option") {
				if catalystHasAttr(option, "selected") {
					selected++
				}
			}
			if selected != 1 {
				return fmt.Errorf("native select %s has %d selected choices", catalystAttr(n, "name"), selected)
			}
		}
	}
	var submits []*html.Node
	for _, button := range catalystElements(forms[0], "button") {
		if catalystAttr(button, "type") == "submit" && catalystAttr(button, "form") == "" {
			submits = append(submits, button)
		}
	}
	if len(submits) != 1 || catalystAttr(submits[0], "value") != "validate" {
		return fmt.Errorf("implicit submission can discard draft")
	}
	return nil
}

func TestCatalystNativeStructureCanaries(t *testing.T) {
	valid := `<form data-p10-settings><fieldset class="disabled:opacity-50"><input id="name" name="name" aria-describedby="name-error"><p id="name-error" hidden></p><select><option selected>Example</option></select><button type="submit" value="reset" form="reset-form">Reset</button><button type="submit" value="validate">Validate</button></fieldset></form><form hidden data-p10-reset id="reset-form"></form>`
	if err := checkCatalystNativeStructure([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(valid, "<fieldset ", "<fieldset disabled ", 1),
		strings.Replace(valid, "<input ", "<input disabled ", 1),
		strings.Replace(valid, `id="name-error"`, `id="missing"`, 1),
		strings.Replace(valid, `<p id="name-error" hidden></p>`, `<p id="name" hidden></p>`, 1),
		strings.Replace(valid, "<option selected>", "<option>", 1),
		strings.Replace(valid, ` form="reset-form"`, "", 1),
	} {
		if err := checkCatalystNativeStructure([]byte(bad)); err == nil {
			t.Fatal("broken native structure passed")
		}
	}
}
