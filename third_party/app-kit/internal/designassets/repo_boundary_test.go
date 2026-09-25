package designassets

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const privateDesignAssetsModule = "// Private reference/evidence data, not packages in the app-kit build.\nmodule appkit.invalid/private-design-assets\n"

func TestPrivateDesignAssetsModuleBoundary(t *testing.T) {
	t.Setenv("GO111MODULE", "off")
	markerPath := filepath.Join("..", "..", "data", "design-assets", "go.mod")
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal("read repository private-design-assets marker:", err)
	}
	if string(marker) != privateDesignAssetsModule {
		t.Fatalf("private-design-assets marker must remain minimal; got %q", marker)
	}

	root := t.TempDir()
	writeBoundaryFixture(t, filepath.Join(root, "go.mod"), "module example.invalid/repository\n")
	writeBoundaryFixture(t, filepath.Join(root, "canary", "canary.go"), "package canary\n")
	writeBoundaryFixture(t, filepath.Join(root, "data", "design-assets", "archived", "forensic.go"), "package forensic\n")
	writeBoundaryFixture(t, filepath.Join(root, "data", "design-assets", "go.mod"), string(marker))

	withMarker := goListPackages(t, root)
	wantPublic := []string{"example.invalid/repository/canary"}
	if !slices.Equal(withMarker, wantPublic) {
		t.Fatalf("nested module did not exclude only the private store: got %v, want %v", withMarker, wantPublic)
	}

	if err := os.Remove(filepath.Join(root, "data", "design-assets", "go.mod")); err != nil {
		t.Fatal("remove synthetic boundary marker:", err)
	}
	withoutMarker := goListPackages(t, root)
	wantUnbounded := []string{
		"example.invalid/repository/canary",
		"example.invalid/repository/data/design-assets/archived",
	}
	if !slices.Equal(withoutMarker, wantUnbounded) {
		t.Fatalf("fixture did not prove the marker's exclusion: got %v, want %v", withoutMarker, wantUnbounded)
	}
}

func TestPrivateDesignAssetsGitIgnoreBoundary(t *testing.T) {
	redirectedGitDir := filepath.Join(t.TempDir(), "redirected.git")
	t.Setenv("GIT_DIR", redirectedGitDir)
	t.Setenv("GIT_WORK_TREE", filepath.Join(t.TempDir(), "redirected-work-tree"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "redirected-index"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "redirected-global-config"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "redirected-system-config"))
	t.Setenv("GIT_TEMPLATE_DIR", filepath.Join(t.TempDir(), "redirected-template"))
	ignore, err := os.ReadFile(filepath.Join("..", "..", ".gitignore"))
	if err != nil {
		t.Fatal("read repository ignore rules:", err)
	}
	root := t.TempDir()
	writeBoundaryFixture(t, filepath.Join(root, ".gitignore"), string(ignore))
	writeBoundaryFixture(t, filepath.Join(root, "data", "design-assets", "go.mod"), privateDesignAssetsModule)
	for _, path := range []string{
		"data/design-assets/manifest.json",
		"data/design-assets/source/source.go",
		"data/design-assets/archive/evidence.zip",
	} {
		writeBoundaryFixture(t, filepath.Join(root, filepath.FromSlash(path)), "synthetic private evidence\n")
	}

	emptyTemplate := filepath.Join(root, "empty-git-template")
	if err := os.MkdirAll(emptyTemplate, 0o755); err != nil {
		t.Fatal("create empty Git template:", err)
	}
	runBoundaryCommand(t, root, "git", "init", "-q", "--template="+emptyTemplate)
	if _, err := os.Stat(redirectedGitDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inherited GIT_DIR affected synthetic repository: %v", err)
	}
	assertIgnored := func(path string, want bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "check-ignore", "--no-index", "--quiet", "--", path)
		cmd.Dir = root
		cmd.Env = privateBoundaryGitEnv()
		err := cmd.Run()
		ignored := err == nil
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("git check-ignore %s: %v", path, err)
			}
		}
		if ignored != want {
			t.Fatalf("git ignored %s = %t, want %t", path, ignored, want)
		}
	}
	assertIgnored("data/design-assets/go.mod", false)
	assertIgnored("data/design-assets/manifest.json", true)
	assertIgnored("data/design-assets/source/source.go", true)
	assertIgnored("data/design-assets/archive/evidence.zip", true)
}

func goListPackages(t *testing.T, root string) []string {
	t.Helper()
	out := runBoundaryCommand(t, root, "go", "list", "./...")
	return strings.Fields(string(out))
}

func runBoundaryCommand(t *testing.T, root, name string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = root
	if name == "go" {
		cmd.Env = privateBoundaryGoEnv()
	} else if name == "git" {
		cmd.Env = privateBoundaryGitEnv()
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v: %s", name, strings.Join(args, " "), err, out)
	}
	return out
}

func privateBoundaryGoEnv() []string {
	disabled := map[string]bool{
		"GO111MODULE": true, "GOENV": true, "GOFLAGS": true, "GOPROXY": true,
		"GOSUMDB": true, "GOTOOLCHAIN": true, "GOWORK": true,
	}
	env := make([]string, 0, len(os.Environ())+len(disabled))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !disabled[key] {
			env = append(env, entry)
		}
	}
	return append(env,
		"GO111MODULE=on",
		"GOENV=off",
		"GOFLAGS=",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
	)
}

func privateBoundaryGitEnv() []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(key, "GIT_") {
			env = append(env, entry)
		}
	}
	return append(env,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
	)
}

func writeBoundaryFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal("create fixture directory:", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal("write fixture:", err)
	}
}
