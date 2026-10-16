package gallery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/tailwind"
)

const galleryTailwindImport = `@import "tailwindcss" source(none);`

func TestGalleryCSSDisablesImplicitSourceDiscovery(t *testing.T) {
	input, err := os.ReadFile("ui/input.css")
	if err != nil {
		t.Fatal(err)
	}
	imports := regexp.MustCompile(`@import\s+["']tailwindcss["'][^;]*;`).FindAll(input, -1)
	if len(imports) != 1 || string(imports[0]) != galleryTailwindImport {
		t.Fatalf("Gallery CSS must contain exactly %q; got %q", galleryTailwindImport, imports)
	}
	if strings.Count(string(input), galleryTailwindImport) != 1 {
		t.Fatalf("Gallery CSS must contain one exact guarded Tailwind import")
	}
	if !strings.Contains(string(input), `@source "../*.js";`) {
		t.Fatal("Gallery-owned browser scripts must participate in the explicit source scan")
	}
}

func TestGalleryCSSClosureWithPinnedCompiler(t *testing.T) {
	if os.Getenv("APPKIT_CSS_CLOSURE") != "1" {
		t.Skip("set APPKIT_CSS_CLOSURE=1 with APPKIT_CSS_COMPILER=/absolute/pinned/tailwindcss")
	}
	compiler := os.Getenv("APPKIT_CSS_COMPILER")
	if !filepath.IsAbs(compiler) {
		t.Fatal("APPKIT_CSS_COMPILER must be an absolute path")
	}
	manifest, err := tailwind.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	pin, ok := manifest.Binaries[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		t.Fatalf("no Tailwind pin for %s-%s", runtime.GOOS, runtime.GOARCH)
	}
	gotHash := galleryCSSFileSHA256(t, compiler)
	if gotHash != pin.SHA256 {
		t.Fatalf("APPKIT_CSS_COMPILER sha256=%s, want pinned %s", gotHash, pin.SHA256)
	}

	evidenceParent := os.Getenv("APPKIT_CSS_EVIDENCE_ROOT")
	if !filepath.IsAbs(evidenceParent) {
		t.Fatal("APPKIT_CSS_EVIDENCE_ROOT must be an existing absolute private directory")
	}
	root, err := os.MkdirTemp(evidenceParent, "appkit-gallery-css-closure.")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retained CSS closure inputs and outputs: %s", root)
	// Keep test canaries out of the Gallery's own Go-source utility scan.
	writeClosureFile(t, root, "selected/page.html", `<main class="`+"min-"+"h-["+`41rem]"></main>`+"\n")
	guarded := galleryTailwindImport + "\n" + `@source "./selected";` + "\n"
	implicit := `@import "tailwindcss";` + "\n" + `@source "./selected";` + "\n"
	writeClosureFile(t, root, "guarded.css", guarded)
	writeClosureFile(t, root, "implicit.css", implicit)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	guardedBefore := compileClosureCSS(t, ctx, compiler, root, "guarded.css", "guarded-before.css")
	implicitBefore := compileClosureCSS(t, ctx, compiler, root, "implicit.css", "implicit-before.css")
	writeClosureFile(t, root, "unselected/canary.html", `<aside class="`+"min-"+"w-["+`1937px]"></aside>`+"\n")
	guardedAfter := compileClosureCSS(t, ctx, compiler, root, "guarded.css", "guarded-after.css")
	implicitAfter := compileClosureCSS(t, ctx, compiler, root, "implicit.css", "implicit-after.css")

	selected, leak := []byte(`.min-h-\[41rem\]`), []byte(`.min-w-\[1937px\]`)
	if !bytes.Contains(guardedBefore, selected) || !bytes.Contains(guardedAfter, selected) {
		t.Fatal("explicit selected source was not compiled")
	}
	if !bytes.Equal(guardedBefore, guardedAfter) || bytes.Contains(guardedAfter, leak) {
		t.Fatal("unselected canary changed source(none) output")
	}
	if bytes.Equal(implicitBefore, implicitAfter) || bytes.Contains(implicitBefore, leak) || !bytes.Contains(implicitAfter, leak) {
		t.Fatal("implicit-discovery control did not admit only the added canary")
	}
	receipt := fmt.Sprintf("compiler=%s\nversion=%s\nsha256=%s\nguarded_before=%x\nguarded_after=%x\nimplicit_before=%x\nimplicit_after=%x\n",
		compiler, manifest.Version, gotHash, sha256.Sum256(guardedBefore), sha256.Sum256(guardedAfter), sha256.Sum256(implicitBefore), sha256.Sum256(implicitAfter))
	writeClosureFile(t, root, "receipt.txt", receipt)
}

func compileClosureCSS(t *testing.T, ctx context.Context, compiler, root, input, output string) []byte {
	t.Helper()
	cmd := exec.CommandContext(ctx, compiler, "--cwd", root, "-i", input, "-o", output, "--minify")
	cmd.WaitDelay = 2 * time.Second
	if diagnostics, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile %s: %v\n%s", input, err, diagnostics)
	}
	b, err := os.ReadFile(filepath.Join(root, output))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeClosureFile(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func galleryCSSFileSHA256(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
