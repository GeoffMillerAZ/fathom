package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/internal/designadapt"
)

func TestAssetsAdaptDirectoryDeliveryInvalidFlags(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"-root", root, "-out="},
		{"-root", root, "-check="},
		{"-root", root, "-out", filepath.Join(root, "out"), "-check", filepath.Join(root, "check")},
		{"-root", root, "-enhanced", "extra"},
	} {
		body, err := captureAssetsCLI(t, append([]string{"assets-adapt-directory"}, args...)...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatal("invalid directory delivery flags did not fail silently with exit 2")
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("invalid directory delivery flags changed the filesystem")
	}
}

func TestAssetsAdaptDirectoryDeliveryLive(t *testing.T) {
	kitRoot := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kitRoot == "" {
		t.Skip("set APPKIT_PROBE_CONVERSION_KIT_ROOT for the private P04 delivery probe")
	}
	if !filepath.IsAbs(kitRoot) {
		t.Fatal("APPKIT_PROBE_CONVERSION_KIT_ROOT must be absolute")
	}
	assets := filepath.Join(kitRoot, "data/design-assets")
	_, archiveSHA, ok := strings.Cut(designadapt.DirectoryArchive, "@")
	if !ok {
		t.Fatal("frozen P04 archive identity is malformed")
	}
	sourcePaths := []string{filepath.Join(assets, "catalog.json"), filepath.Join(assets, "archives", archiveSHA+".zip")}
	sourceBefore := make(map[string][sha256.Size]byte, len(sourcePaths))
	for _, path := range sourcePaths {
		identity, err := directoryDeliveryFileSHA(path)
		if err != nil {
			t.Fatal("private P04 source identity is unavailable")
		}
		sourceBefore[path] = identity
	}
	t.Cleanup(func() {
		for _, path := range sourcePaths {
			after, err := directoryDeliveryFileSHA(path)
			if err != nil || after != sourceBefore[path] {
				t.Error("P04 delivery probe could not reverify unchanged private source/catalog")
			}
		}
	})
	baseJSON, err := captureAssetsCLI(t, "assets-adapt-directory", "-root", assets)
	if err != nil {
		t.Fatal("default P04 JSON generation failed; private output withheld")
	}
	explicitBaseJSON, err := captureAssetsCLI(t, "assets-adapt-directory", "-root", assets, "-enhanced=false")
	if err != nil || !bytes.Equal(baseJSON, explicitBaseJSON) {
		t.Fatal("delivery support changed the default P04 JSON envelope")
	}
	enhancedJSON, err := captureAssetsCLI(t, "assets-adapt-directory", "-root", assets, "-enhanced")
	if err != nil {
		t.Fatal("enhanced P04 JSON generation failed; private output withheld")
	}
	var base designadapt.DirectoryArtifact
	var enhanced designadapt.DirectoryEnhancedArtifact
	if json.Unmarshal(baseJSON, &base) != nil || json.Unmarshal(enhancedJSON, &enhanced) != nil || !reflect.DeepEqual(base, enhanced.Base) {
		t.Fatal("P04 JSON envelopes or base lineage differ")
	}

	parent := t.TempDir()
	export := func(name string, enhanced bool) string {
		t.Helper()
		out := filepath.Join(parent, name)
		args := []string{"assets-adapt-directory", "-root", assets}
		if enhanced {
			args = append(args, "-enhanced")
		}
		body, err := captureAssetsCLI(t, append(args, "-out", out)...)
		if err != nil || len(body) != 0 {
			t.Fatal("P04 delivery export failed or emitted private output")
		}
		return out
	}
	baseOut := export("base", false)
	enhancedOut := export("enhanced", true)
	baseFiles := readDirectoryDelivery(t, baseOut, directoryBaseDeliveryNames)
	enhancedFiles := readDirectoryDelivery(t, enhancedOut, directoryEnhancedDeliveryNames)
	if !bytes.Equal(baseFiles["artifact.json"].body, baseJSON) || !bytes.Equal(enhancedFiles["artifact.json"].body, enhancedJSON) {
		t.Fatal("delivered artifact differs from unchanged JSON envelope")
	}
	if !bytes.Equal(baseFiles["directory.templ"].body, []byte(base.Templ)) || !bytes.Equal(baseFiles["directory.go"].body, []byte(base.Model)) || !bytes.Equal(baseFiles["defaults.json"].body, base.Defaults) {
		t.Fatal("base delivery does not contain exact artifact fields")
	}
	if !bytes.Equal(enhancedFiles["directory.templ"].body, []byte(enhanced.Base.Templ)) || !bytes.Equal(enhancedFiles["directory.go"].body, []byte(enhanced.Base.Model)) || !bytes.Equal(enhancedFiles["directory_enhanced.templ"].body, []byte(enhanced.Templ)) || !bytes.Equal(enhancedFiles["directory_enhanced.go"].body, []byte(enhanced.Model)) || !bytes.Equal(enhancedFiles["directory_enhanced.css"].body, []byte(enhanced.CSS)) || !bytes.Equal(enhancedFiles["defaults.json"].body, enhanced.Base.Defaults) {
		t.Fatal("enhanced delivery does not contain exact base and companion fields")
	}
	for _, tc := range []struct {
		name     string
		dir      string
		enhanced bool
	}{
		{"base", baseOut, false}, {"enhanced", enhancedOut, true},
	} {
		before := readDirectoryDelivery(t, tc.dir, map[bool][]string{false: directoryBaseDeliveryNames, true: directoryEnhancedDeliveryNames}[tc.enhanced])
		args := []string{"assets-adapt-directory", "-root", assets}
		if tc.enhanced {
			args = append(args, "-enhanced")
		}
		body, err := captureAssetsCLI(t, append(args, "-check", tc.dir)...)
		if err != nil || len(body) != 0 || !reflect.DeepEqual(before, readDirectoryDelivery(t, tc.dir, map[bool][]string{false: directoryBaseDeliveryNames, true: directoryEnhancedDeliveryNames}[tc.enhanced])) {
			t.Fatalf("matching %s delivery check failed, emitted output, or mutated files", tc.name)
		}
		assertDirectoryDeliveryExit(t, 2, append(args, "-out", tc.dir)...)
		if !reflect.DeepEqual(before, readDirectoryDelivery(t, tc.dir, map[bool][]string{false: directoryBaseDeliveryNames, true: directoryEnhancedDeliveryNames}[tc.enhanced])) {
			t.Fatalf("%s no-clobber refusal changed the existing delivery", tc.name)
		}
	}
	assertDirectoryDeliveryExit(t, 1, "assets-adapt-directory", "-root", assets, "-check", enhancedOut)
	assertDirectoryDeliveryExit(t, 1, "assets-adapt-directory", "-root", assets, "-enhanced", "-check", baseOut)
	assertDirectoryDeliveryExit(t, 1, "assets-adapt-directory", "-root", assets, "-check", filepath.Join(parent, "missing"))
	tampered := export("tampered", false)
	if err := os.WriteFile(filepath.Join(tampered, "directory.templ"), []byte("changed"), 0o600); err != nil {
		t.Fatal("could not prepare private tamper fixture")
	}
	assertDirectoryDeliveryExit(t, 1, "assets-adapt-directory", "-root", assets, "-check", tampered)
	assertDirectoryDeliveryExit(t, 2, "assets-adapt-directory", "-root", filepath.Join(parent, "missing-source"), "-check", baseOut)
	assertDirectoryDeliveryExit(t, 2, "assets-adapt-directory", "-root", assets, "-out", filepath.Join(parent, "missing-parent", "out"))

	goVersion, templVersion := compileDirectoryDelivery(t, kitRoot, baseOut, false)
	enhancedGoVersion, enhancedTemplVersion := compileDirectoryDelivery(t, kitRoot, enhancedOut, true)
	if goVersion != enhancedGoVersion || templVersion != enhancedTemplVersion {
		t.Fatal("standalone P04 modules resolved different pinned toolchains")
	}
	if !reflect.DeepEqual(baseFiles, readDirectoryDelivery(t, baseOut, directoryBaseDeliveryNames)) || !reflect.DeepEqual(enhancedFiles, readDirectoryDelivery(t, enhancedOut, directoryEnhancedDeliveryNames)) {
		t.Fatal("standalone compilation changed an original immutable delivery")
	}
	t.Logf("private P04 delivery: base_files=%d enhanced_files=%d default_json_bytes=%d enhanced_json_bytes=%d; standalone cached-module-resolution proof compiled with %s templ=%s", len(baseFiles), len(enhancedFiles), len(baseJSON), len(enhancedJSON), goVersion, templVersion)
}

func directoryDeliveryFileSHA(path string) ([sha256.Size]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

var directoryBaseDeliveryNames = []string{"README.md", "artifact.json", "defaults.json", "directory.go", "directory.templ"}
var directoryEnhancedDeliveryNames = []string{"README.md", "artifact.json", "defaults.json", "directory.go", "directory.templ", "directory_enhanced.css", "directory_enhanced.go", "directory_enhanced.templ"}

type directoryDeliveryFile struct {
	body []byte
	mode os.FileMode
}

func readDirectoryDelivery(t *testing.T, dir string, want []string) map[string]directoryDeliveryFile {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != len(want) {
		t.Fatal("P04 delivery inventory differs; private bodies withheld")
	}
	names := make([]string, 0, len(entries))
	files := make(map[string]directoryDeliveryFile, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			t.Fatal("P04 delivery contains a non-regular entry")
		}
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal("P04 delivery entry could not be read")
		}
		names = append(names, entry.Name())
		files[entry.Name()] = directoryDeliveryFile{body, info.Mode().Perm()}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatal("P04 delivery file mode is not 0600")
		}
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, want) {
		t.Fatal("P04 delivery filenames differ")
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatal("P04 delivery directory mode is not 0700")
		}
	}
	return files
}

func assertDirectoryDeliveryExit(t *testing.T, code int, args ...string) {
	t.Helper()
	body, err := captureAssetsCLI(t, args...)
	if command.Code(err) != code || len(body) != 0 {
		t.Fatalf("P04 delivery refusal used exit %d or emitted stdout", code)
	}
}

func compileDirectoryDelivery(t *testing.T, repo, delivery string, enhanced bool) (string, string) {
	t.Helper()
	build := t.TempDir()
	names := []string{"directory.go", "directory.templ"}
	constructor := "NewDirectoryNavigation"
	variant := "base"
	if enhanced {
		names = append(names, "directory_enhanced.go", "directory_enhanced.templ")
		constructor = "NewDirectoryEnhanced"
		variant = "enhanced"
	}
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(delivery, name))
		if err != nil || os.WriteFile(filepath.Join(build, name), body, 0o600) != nil {
			t.Fatal("could not create isolated P04 build copy")
		}
	}
	defaults, err := os.ReadFile(filepath.Join(delivery, "defaults.json"))
	if err != nil {
		t.Fatal("could not read delivered P04 defaults")
	}
	fixture := fmt.Sprintf(`package converted
import("bytes";"context";"encoding/json";"testing")
const defaultsJSON=%s
func TestDeliveredDirectoryRenders(t *testing.T){var d Directory;if json.Unmarshal([]byte(defaultsJSON),&d)!=nil{t.Fatal("defaults")};d.AddURL="/new";for i:=range d.Rows{d.Rows[i].EditURL="/edit"};c,e:=%s(d);if e!=nil{t.Fatal("constructor")};var b bytes.Buffer;if c.Render(context.Background(),&b)!=nil||b.Len()==0{t.Fatal("render")}}
`, strconv.Quote(string(defaults)), constructor)
	fixturePath := filepath.Join(build, "delivery_test.go")
	if err := os.WriteFile(fixturePath, []byte(fixture), 0o600); err != nil {
		t.Fatal("could not create isolated P04 build test")
	}
	fixtureRoot := filepath.Join(repo, "docs/assessments/fixtures/pricing-adapter")
	readFixture := func(name string) []byte {
		t.Helper()
		path := filepath.Join(fixtureRoot, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > 32<<10 {
			t.Fatal("standalone module metadata fixture is unsafe")
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal("standalone module metadata fixture is unreadable")
		}
		return body
	}
	moduleBody := readFixture("consumer.mod.txt")
	const pricingModule = "module example.invalid/pricing-proof"
	if bytes.Count(moduleBody, []byte(pricingModule)) != 1 {
		t.Fatal("standalone module template identity differs")
	}
	moduleBody = bytes.Replace(moduleBody, []byte(pricingModule), []byte("module example.invalid/directory-delivery-"+variant), 1)
	if bytes.Contains(moduleBody, []byte("github.com/geoffmilleraz/app-kit")) || bytes.Contains(moduleBody, []byte("\nreplace ")) || bytes.Contains(moduleBody, []byte("\nreplace (")) {
		t.Fatal("standalone module fixture contains an app-kit dependency or replacement")
	}
	sumBody := readFixture("consumer.sum.txt")
	for name, body := range map[string][]byte{"go.mod": moduleBody, "go.sum": sumBody} {
		if err := os.WriteFile(filepath.Join(build, name), body, 0o600); err != nil {
			t.Fatal("could not materialize standalone P04 module metadata")
		}
	}
	immutableNames := append(append([]string{}, names...), "delivery_test.go", "go.mod", "go.sum")
	immutable := make(map[string][sha256.Size]byte, len(immutableNames))
	for _, name := range immutableNames {
		identity, err := directoryDeliveryFileSHA(filepath.Join(build, name))
		if err != nil {
			t.Fatal("could not identify standalone P04 module input")
		}
		immutable[name] = identity
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	env := directoryDeliveryOfflineGoEnv()
	verifyModules := exec.CommandContext(ctx, "go", "mod", "verify")
	verifyModules.Dir, verifyModules.Env = build, env
	if verifyModules.Run() != nil {
		t.Fatal("standalone cached module checksums could not be verified")
	}
	preflight := exec.CommandContext(ctx, "go", "test", "./...")
	preflight.Dir, preflight.Env = build, env
	preflightOutput, preflightErr := preflight.CombinedOutput()
	if preflightErr == nil || !bytes.Contains(preflightOutput, []byte("undefined: directoryNavigation")) {
		t.Fatal("standalone pre-generation canary did not fail for absent generated code")
	}
	templates := []string{"directory.templ"}
	if enhanced {
		templates = append(templates, "directory_enhanced.templ")
	}
	for _, name := range templates {
		cmd := exec.CommandContext(ctx, "go", "tool", "templ", "generate", "-f", name, "-include-version=false")
		cmd.Dir, cmd.Env = build, env
		if cmd.Run() != nil {
			t.Fatal("pinned templ generation failed; private diagnostics withheld")
		}
	}
	generated, err := filepath.Glob(filepath.Join(build, "*_templ.go"))
	if err != nil || len(generated) != len(templates) {
		t.Fatal("isolated P04 generated source inventory missing")
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", "^$")
	cmd.Dir, cmd.Env = build, env
	if cmd.Run() != nil {
		t.Fatal("isolated P04 delivery compile failed; private diagnostics withheld")
	}
	cmd = exec.CommandContext(ctx, "go", "test", "-count=1", "-run", "^TestDeliveredDirectoryRenders$")
	cmd.Dir, cmd.Env = build, env
	if cmd.Run() != nil {
		t.Fatal("isolated P04 delivery render failed; private diagnostics withheld")
	}
	resolvedRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal("app-kit checkout identity could not be resolved")
	}
	repoParent := filepath.Clean(filepath.Dir(resolvedRepo)) + string(filepath.Separator)
	inspectClosure := func(target, requiredModule string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-f", "{{if .Module}}{{.Module.Path}}\t{{.Dir}}{{end}}", target)
		cmd.Dir, cmd.Env = build, env
		dependencyOutput, err := cmd.Output()
		if err != nil {
			t.Fatal("standalone P04 dependency closure could not be inspected")
		}
		rows, foundRequired := 0, false
		for _, line := range bytes.Split(dependencyOutput, []byte{'\n'}) {
			fields := bytes.SplitN(line, []byte{'\t'}, 2)
			if len(fields) != 2 {
				continue
			}
			rows++
			modulePath := string(fields[0])
			foundRequired = foundRequired || modulePath == requiredModule
			dir, err := filepath.EvalSymlinks(string(fields[1]))
			if err != nil {
				t.Fatal("standalone dependency directory identity could not be resolved")
			}
			dir = filepath.Clean(dir)
			if modulePath == "github.com/geoffmilleraz/app-kit" || strings.HasPrefix(dir+string(filepath.Separator), repoParent) {
				t.Fatal("standalone P04 consumer or templ compiler resolved app-kit or a sibling checkout")
			}
		}
		if rows == 0 || !foundRequired {
			t.Fatal("standalone dependency closure omitted its required module")
		}
	}
	inspectClosure("./...", "example.invalid/directory-delivery-"+variant)
	inspectClosure("github.com/a-h/templ/cmd/templ", "github.com/a-h/templ")
	goMod := exec.CommandContext(ctx, "go", "env", "GOMOD")
	goMod.Dir, goMod.Env = build, env
	goModPath, err := goMod.Output()
	actualGoMod, resolveErr := filepath.EvalSymlinks(strings.TrimSpace(string(goModPath)))
	wantGoMod, wantResolveErr := filepath.EvalSymlinks(filepath.Join(build, "go.mod"))
	if err != nil || resolveErr != nil || wantResolveErr != nil || actualGoMod != wantGoMod {
		t.Fatal("P04 compile did not use its standalone module")
	}
	for _, name := range immutableNames {
		after, err := directoryDeliveryFileSHA(filepath.Join(build, name))
		if err != nil || after != immutable[name] {
			t.Fatal("standalone compilation changed module metadata or delivered source bytes")
		}
	}
	version := exec.CommandContext(ctx, "go", "version")
	version.Dir, version.Env = build, env
	goVersion, err := version.Output()
	if err != nil {
		t.Fatal("standalone Go version unavailable")
	}
	templ := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Version}}", "github.com/a-h/templ")
	templ.Dir, templ.Env = build, env
	templVersion, err := templ.Output()
	if err != nil {
		t.Fatal("standalone templ version unavailable")
	}
	return strings.TrimSpace(string(goVersion)), strings.TrimSpace(string(templVersion))
}

func directoryDeliveryOfflineGoEnv() []string {
	env := make([]string, 0, len(os.Environ())+5)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "GOENV" && key != "GOPROXY" && key != "GOSUMDB" && key != "GOPRIVATE" && key != "GONOPROXY" && key != "GONOSUMDB" && key != "GOINSECURE" && key != "GOTOOLCHAIN" && key != "GOFLAGS" && key != "GOWORK" {
			env = append(env, item)
		}
	}
	return append(env, "GOENV=off", "GOPROXY=off", "GOSUMDB=sum.golang.org", "GOPRIVATE=", "GONOPROXY=", "GONOSUMDB=", "GOINSECURE=", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
}
