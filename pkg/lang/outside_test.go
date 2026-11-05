package lang_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// LL-23 (design section 8, row 4): a module outside this repository builds
// and runs against the door, offline. The consumer under testdata/consumer is
// copied next to a go.mod written here, with a replace directive that points at
// this tree. The build may use the local module cache and nothing else: the
// proxy is off, so a dependency the cache lacks fails the build.
func TestOutsideModuleBuilds(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	rootMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	goVersion := regexp.MustCompile(`(?m)^go (\S+)$`).FindSubmatch(rootMod)
	if goVersion == nil {
		t.Fatal("go.mod names no go version")
	}
	src, err := os.ReadFile(filepath.Join("testdata", "consumer", "main.go"))
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	mod := "module example.com/consumer\n\ngo " + string(goVersion[1]) + "\n\n" +
		"require github.com/geoffmilleraz/fathom v0.0.0\n\n" +
		"replace github.com/geoffmilleraz/fathom => " + root + "\n"
	for name, body := range map[string][]byte{"go.mod": []byte(mod), "main.go": src} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// -mod=mod lets the build add the dependency requirements and go.sum
	// lines the replaced module needs, from the module cache alone.
	env := append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local")
	bin := filepath.Join(dir, "consumer")
	build := exec.Command("go", "build", "-ldflags=-s -w", "-o", bin, ".")
	build.Dir, build.Env = dir, env
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the outside module does not build offline against this tree: %v\n%s", err, out)
	}

	run := exec.Command(bin)
	run.Dir = dir
	var stderr bytes.Buffer
	run.Stderr = &stderr
	out, err := run.Output()
	if err != nil {
		t.Fatalf("the outside binary failed: %v\n%s", err, stderr.String())
	}
	var res lang.Result
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("the outside binary printed no Result: %v\n%s", err, out)
	}
	if res.Dialect != "terraform" || res.Skipped != "" {
		t.Fatalf("outside read: dialect %q skipped %q", res.Dialect, res.Skipped)
	}
	var found bool
	for _, s := range res.Symbols {
		found = found || (s.Kind == "TerraformResource" && s.Name == "aws_s3_bucket.b")
	}
	if !found {
		t.Fatalf("the outside read lost the resource: %+v", res.Symbols)
	}
}
