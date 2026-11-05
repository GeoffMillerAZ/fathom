package lang_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// LL-23 (design section 8, row 4): a module outside this repository builds
// and runs against the door, offline. The consumer under testdata/consumer
// carries its own tidied go.mod and go.sum, with a replace directive that points
// at this tree. The test copies the three files into a scratch directory, with
// the replace rewritten to the absolute path of this tree, and builds with
// -mod=readonly and the proxy off: a dependency the module cache lacks, or a
// go.mod that needs an update, fails the build. The module cache needs only what
// `go mod download` of the root module fetches, which a fresh cache holds.
//
// A change to the root module's requirements that moves a version the consumer
// pins fails TestOutsideModuleMatchesRoot with the command that regenerates
// the consumer's go.mod and go.sum.
const regenerateConsumer = "cd pkg/lang/testdata/consumer && GOFLAGS=-mod=mod go mod tidy (with the module proxy reachable)"

func TestOutsideModuleBuilds(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum", "main.go"} {
		b, err := os.ReadFile(filepath.Join("testdata", "consumer", name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "go.mod" {
			if !strings.Contains(string(b), replaceLine) {
				t.Fatalf("the consumer's go.mod lacks %q", replaceLine)
			}
			b = []byte(strings.Replace(string(b), replaceLine, "replace github.com/geoffmilleraz/fathom => "+root, 1))
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	env := append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=-mod=readonly", "GOTOOLCHAIN=local")
	bin := filepath.Join(dir, "consumer")
	build := exec.Command("go", "build", "-ldflags=-s -w", "-o", bin, ".")
	build.Dir, build.Env = dir, env
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the outside module does not build offline against this tree: %v\n%s\nRegenerate its go.mod and go.sum with: %s", err, out, regenerateConsumer)
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

// replaceLine is the consumer's committed replace directive: a relative path to
// this tree that the test rewrites to an absolute one.
const replaceLine = "replace github.com/geoffmilleraz/fathom => ../../../.."

// requirements reads a go.mod's "go" version and its requirement lines.
func requirements(t *testing.T, path string) (goVersion string, req map[string]string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	req = map[string]string{}
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "//", 2)[0])
		switch {
		case strings.HasPrefix(line, "go "):
			goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		case line == "require (":
			in = true
		case line == ")":
			in = false
		case strings.HasPrefix(line, "require "):
			if f := strings.Fields(strings.TrimPrefix(line, "require ")); len(f) == 2 {
				req[f[0]] = f[1]
			}
		case in:
			if f := strings.Fields(line); len(f) == 2 {
				req[f[0]] = f[1]
			}
		}
	}
	return goVersion, req
}

// The consumer's go.mod pins the versions the root module does, and the same
// go version, so a root change that the consumer has not followed fails here
// with the command that fixes it, not later as a build error.
func TestOutsideModuleMatchesRoot(t *testing.T) {
	rootGo, rootReq := requirements(t, filepath.Join("..", "..", "go.mod"))
	conGo, conReq := requirements(t, filepath.Join("testdata", "consumer", "go.mod"))
	if conGo != rootGo {
		t.Errorf("the consumer's go.mod says go %s, the root's says go %s; regenerate: %s", conGo, rootGo, regenerateConsumer)
	}
	for mod, v := range conReq {
		if mod == "github.com/geoffmilleraz/fathom" {
			continue
		}
		if want, ok := rootReq[mod]; ok && want != v {
			t.Errorf("the consumer pins %s %s, the root requires %s; regenerate: %s", mod, v, want, regenerateConsumer)
		}
	}
}
