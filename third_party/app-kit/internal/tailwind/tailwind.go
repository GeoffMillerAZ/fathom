// Package tailwind fetches, verifies, and runs the checksum-pinned Tailwind
// CSS standalone binary (D2: Go + CUE only — no Node anywhere in any kit
// toolchain; CSS is produced by this binary, pinned by SHA-256 per platform).
//
// The pin lives in manifest.cue (embedded). Resolve() downloads the binary
// for the current platform into the user cache on first use, verifies its
// checksum — a mismatch deletes the file and fails hard — and returns the
// path. Run() executes it with stdio inherited, which covers both one-shot
// builds and --watch mode.
package tailwind

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed manifest.cue
var manifestCUE []byte

// Manifest is the decoded form of manifest.cue.
type Manifest struct {
	Version  string            `json:"version"`
	Binaries map[string]Binary `json:"binaries"`
}

// Binary is one platform's pinned release asset.
type Binary struct {
	Asset  string `json:"asset"`
	SHA256 string `json:"sha256"`
}

// LoadManifest parses and validates the embedded manifest.cue (D7: the CUE
// file is the source of truth; this is its runtime validation).
func LoadManifest() (Manifest, error) {
	ctx := cuecontext.New()
	v := ctx.CompileBytes(manifestCUE)
	if err := v.Err(); err != nil {
		return Manifest{}, fmt.Errorf("tailwind: compile manifest.cue: %w", err)
	}
	mv := v.LookupPath(cue.ParsePath("manifest"))
	if err := mv.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("tailwind: manifest.cue invalid: %w", err)
	}
	var m Manifest
	if err := mv.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("tailwind: decode manifest: %w", err)
	}
	return m, nil
}

// Resolve returns the path to a verified Tailwind binary for this platform,
// downloading it into the user cache (e.g. ~/Library/Caches/app-kit) on
// first use. The checksum is verified on every call — a cached binary that
// stops matching the pin is removed and the call fails.
func Resolve(ctx context.Context) (string, error) {
	m, err := LoadManifest()
	if err != nil {
		return "", err
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	bin, ok := m.Binaries[platform]
	if !ok {
		return "", fmt.Errorf("tailwind: no pinned binary for platform %s (manifest %s)", platform, m.Version)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("tailwind: user cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "app-kit", "tailwindcss", m.Version)
	path := filepath.Join(dir, bin.Asset)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := download(ctx, m.Version, bin.Asset, path); err != nil {
			return "", err
		}
	}

	if err := verify(path, bin.SHA256); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// Run resolves the binary and executes it with the given arguments, stdio
// inherited (works for one-shot builds and --watch alike).
func Run(ctx context.Context, args ...string) error {
	path, err := Resolve(ctx)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// download fetches the release asset to path via a temp file, so a partial
// download never poses as a binary.
func download(ctx context.Context, version, asset, path string) error {
	url := fmt.Sprintf("https://github.com/tailwindlabs/tailwindcss/releases/download/%s/%s", version, asset)
	fmt.Fprintf(os.Stderr, "appkit: fetching %s %s…\n", version, asset)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("tailwind: build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("tailwind: download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tailwind: download %s: HTTP %s", url, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("tailwind: cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), asset+".part-*")
	if err != nil {
		return fmt.Errorf("tailwind: temp file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("tailwind: write download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("tailwind: close download: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return fmt.Errorf("tailwind: chmod: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("tailwind: move into cache: %w", err)
	}
	return nil
}

// verify compares the file's SHA-256 against the pinned digest. A mismatch
// is a hard failure (D2): the build must not proceed on an unpinned binary.
func verify(path, wantHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("tailwind: open for verify: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("tailwind: hash: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != wantHex {
		return fmt.Errorf("tailwind: CHECKSUM MISMATCH for %s: got %s, pinned %s — refusing to run", path, got, wantHex)
	}
	return nil
}
