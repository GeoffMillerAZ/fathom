package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	spikeModule        = "github.com/geoffmilleraz/app-kit/spikes/wails-gallery"
	appkitModule       = "github.com/geoffmilleraz/app-kit"
	wailsModule        = "github.com/wailsapp/wails/v3"
	wailsVersion       = "v3.0.0-beta.21"
	wailsSum           = "h1:2jc9E7eT4g3vNQIaEoRP19Uvm4LCuL9OzYPD+Uf+yYk="
	wailsGoModSum      = "h1:/6QR46/nhGCSADHbS++XtDb9dkTnenTHlGskTPRo9S0="
	patchRelative      = "patches/wails-v3.0.0-beta.21-http-only-origin.patch"
	patchSHA256        = "48adbb4904968b6ece05484262a4b8bf936a8c44c7afe53145b71fce19e98f90"
	mediaPatchRelative = "patches/wails-v3.0.0-beta.21-media-denial.patch"
	mediaPatchSHA256   = "859403aeb7ab3a36737f3d0f352209740fd72bb6bf496cdb1510e8011b1b1c35"
	stagedModuleDir    = "wails-v3.0.0-beta.21"
)

var patchedFiles = []string{
	"pkg/application/webview_window_options.go",
	"pkg/application/webview_window_options_test.go",
	"pkg/application/webview_window_darwin.go",
	"pkg/application/webview_window_darwin.h",
	"pkg/application/webview_window_darwin.m",
}

type moduleDownload struct {
	Path, Version, Dir, Sum, GoModSum string
	Error                             string
}

type moduleSelection struct {
	Path, Version string
	Replace       *struct {
		Path, Version string
	}
}

type receipt struct {
	Directory        string            `json:"directory"`
	ModFile          string            `json:"modFile"`
	AppkitVersion    string            `json:"appkitVersion,omitempty"`
	Sum              string            `json:"sum"`
	GoModSum         string            `json:"goModSum"`
	PatchSHA256      string            `json:"patchSHA256"`
	MediaPatchSHA256 string            `json:"mediaPatchSHA256"`
	ChangedFiles     map[string]string `json:"changedFiles"`
}

type runner interface {
	run(context.Context, string, string, []string, ...string) ([]byte, error)
}

type commandRunner struct{}

func (commandRunner) run(ctx context.Context, dir, name string, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd.CombinedOutput()
}

type prepareOptions struct {
	appkitVersion string
}

func parseOptions(args []string) (prepareOptions, error) {
	flags := flag.NewFlagSet("prepare-wails", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var version string
	flags.StringVar(&version, "appkit-version", "", "exact private app-kit candidate version")
	if err := flags.Parse(args); err != nil {
		return prepareOptions{}, err
	}
	if flags.NArg() != 0 {
		return prepareOptions{}, fmt.Errorf("unexpected positional arguments")
	}
	explicit := false
	flags.Visit(func(current *flag.Flag) {
		if current.Name == "appkit-version" {
			explicit = true
		}
	})
	if explicit {
		if err := validateCandidateVersion(version); err != nil {
			return prepareOptions{}, err
		}
	}
	return prepareOptions{appkitVersion: version}, nil
}

func main() {
	options, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "prepare-wails:", err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := prepare(ctx, commandRunner{}, options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "encode staging receipt:", err)
		os.Exit(1)
	}
}

func prepare(ctx context.Context, commands runner, options prepareOptions) (receipt, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return receipt{}, fmt.Errorf("working directory: %w", err)
	}
	return prepareAt(ctx, commands, cwd, options)
}

func prepareAt(ctx context.Context, commands runner, cwd string, options prepareOptions) (receipt, error) {
	spikeGoMod := filepath.Join(cwd, "go.mod")
	spikeGoSum := filepath.Join(cwd, "go.sum")
	goModBytes, err := validateModuleFile(spikeGoMod, spikeModule)
	if err != nil {
		return receipt{}, err
	}
	goSumBytes, err := readRegular(spikeGoSum)
	if err != nil {
		return receipt{}, fmt.Errorf("read spike go.sum: %w", err)
	}
	appkitRoot := ""
	if options.appkitVersion == "" {
		appkitRoot, err = filepath.Abs(filepath.Join(cwd, "..", ".."))
		if err != nil {
			return receipt{}, fmt.Errorf("resolve app-kit checkout: %w", err)
		}
		if _, err := validateModuleFile(filepath.Join(appkitRoot, "go.mod"), appkitModule); err != nil {
			return receipt{}, fmt.Errorf("validate app-kit checkout: %w", err)
		}
	}
	patchPath := filepath.Join(cwd, filepath.FromSlash(patchRelative))
	gotPatchSHA, err := hashRegular(patchPath)
	if err != nil {
		return receipt{}, fmt.Errorf("read containment patch: %w", err)
	}
	if gotPatchSHA != patchSHA256 {
		return receipt{}, fmt.Errorf("patch identity mismatch: got %q, want %q", gotPatchSHA, patchSHA256)
	}
	mediaPatchPath := filepath.Join(cwd, filepath.FromSlash(mediaPatchRelative))
	gotMediaPatchSHA, err := hashRegular(mediaPatchPath)
	if err != nil {
		return receipt{}, fmt.Errorf("read media-denial patch: %w", err)
	}
	if gotMediaPatchSHA != mediaPatchSHA256 {
		return receipt{}, fmt.Errorf("media-denial patch identity mismatch: got %q, want %q", gotMediaPatchSHA, mediaPatchSHA256)
	}

	goEnv := replaceEnvironment(os.Environ(), map[string]string{
		"GOENV":       "off",
		"GOFLAGS":     "",
		"GOPROXY":     "off",
		"GOTOOLCHAIN": "local",
		"GOWORK":      "off",
	})
	if options.appkitVersion != "" {
		out, err := commands.run(ctx, cwd, "go", goEnv, "list", "-mod=readonly", "-m", "-json", appkitModule)
		if err != nil {
			return receipt{}, commandError("read active app-kit module selection", err, out)
		}
		var selected moduleSelection
		if err := json.Unmarshal(out, &selected); err != nil {
			return receipt{}, fmt.Errorf("decode active app-kit module selection: %w", err)
		}
		if err := validateAppkitSelection(selected, options.appkitVersion); err != nil {
			return receipt{}, err
		}
	}
	out, err := commands.run(ctx, cwd, "go", goEnv, "list", "-mod=readonly", "-m", "-json", wailsModule)
	if err != nil {
		return receipt{}, commandError("read active Wails module selection", err, out)
	}
	var selected moduleSelection
	if err := json.Unmarshal(out, &selected); err != nil {
		return receipt{}, fmt.Errorf("decode active Wails module selection: %w", err)
	}
	if err := validateSelection(selected); err != nil {
		return receipt{}, err
	}
	out, err = commands.run(ctx, cwd, "go", goEnv, "mod", "download", "-json", wailsModule+"@"+wailsVersion)
	if err != nil {
		return receipt{}, commandError("download pinned Wails module", err, out)
	}
	var source moduleDownload
	if err := json.Unmarshal(out, &source); err != nil {
		return receipt{}, fmt.Errorf("decode module identity: %w", err)
	}
	if err := validateDownload(source); err != nil {
		return receipt{}, err
	}
	out, err = commands.run(ctx, cwd, "go", goEnv, "mod", "verify")
	if err != nil {
		return receipt{}, commandError("verify module cache", err, out)
	}
	if err := unchanged(spikeGoMod, goModBytes); err != nil {
		return receipt{}, err
	}
	if err := unchanged(spikeGoSum, goSumBytes); err != nil {
		return receipt{}, err
	}

	stageRoot, err := os.MkdirTemp("", "appkit-wails-beta21-")
	if err != nil {
		return receipt{}, fmt.Errorf("create staging directory: %w", err)
	}
	fail := func(err error) (receipt, error) {
		return receipt{}, fmt.Errorf("stage retained at %s: %w", stageRoot, err)
	}
	moduleDir, err := copySource(source.Dir, stageRoot)
	if err != nil {
		return fail(err)
	}
	gitEnv := gitEnvironment(os.Environ())
	for _, patch := range []struct {
		name, path string
	}{
		{name: "containment", path: patchPath},
		{name: "media-denial", path: mediaPatchPath},
	} {
		for _, check := range []bool{true, false} {
			args := []string{"apply", "--whitespace=error-all", patch.path}
			if check {
				args = []string{"apply", "--check", "--whitespace=error-all", patch.path}
			}
			out, err = commands.run(ctx, moduleDir, "git", gitEnv, args...)
			if err != nil {
				return fail(commandError("apply "+patch.name+" patch", err, out))
			}
		}
	}
	modFile := filepath.Join(stageRoot, "go.mod")
	if err := writeExclusive(modFile, goModBytes); err != nil {
		return fail(fmt.Errorf("copy spike go.mod: %w", err))
	}
	if err := writeExclusive(filepath.Join(stageRoot, "go.sum"), goSumBytes); err != nil {
		return fail(fmt.Errorf("copy spike go.sum: %w", err))
	}
	editArgs := []string{"mod", "edit", "-modfile=" + modFile}
	if appkitRoot != "" {
		editArgs = append(editArgs, "-replace="+appkitModule+"="+appkitRoot)
	}
	editArgs = append(editArgs, "-replace="+wailsModule+"="+moduleDir)
	out, err = commands.run(ctx, cwd, "go", goEnv, editArgs...)
	if err != nil {
		return fail(commandError("write temporary module replacements", err, out))
	}
	if options.appkitVersion != "" {
		out, err = commands.run(ctx, cwd, "go", goEnv, "list", "-mod=readonly", "-modfile="+modFile, "-m", "-json", appkitModule)
		if err != nil {
			return fail(commandError("verify staged app-kit module selection", err, out))
		}
		var selected moduleSelection
		if err := json.Unmarshal(out, &selected); err != nil {
			return fail(fmt.Errorf("decode staged app-kit module selection: %w", err))
		}
		if err := validateAppkitSelection(selected, options.appkitVersion); err != nil {
			return fail(err)
		}
	}
	hashes := make(map[string]string, len(patchedFiles))
	for _, name := range patchedFiles {
		hash, err := hashRegular(filepath.Join(moduleDir, filepath.FromSlash(name)))
		if err != nil {
			return fail(fmt.Errorf("hash patched source %s: %w", name, err))
		}
		hashes[name] = hash
	}
	return receipt{
		Directory: stageRoot, ModFile: modFile, AppkitVersion: options.appkitVersion, Sum: source.Sum, GoModSum: source.GoModSum,
		PatchSHA256: patchSHA256, MediaPatchSHA256: mediaPatchSHA256, ChangedFiles: hashes,
	}, nil
}

func validateCandidateVersion(version string) error {
	const prefix = "v0.0.0-rehearsal.h"
	digest := strings.TrimPrefix(version, prefix)
	if digest == version || len(digest) != sha256.Size*2 {
		return fmt.Errorf("app-kit version must be %s followed by 64 lowercase hexadecimal characters", prefix)
	}
	for _, character := range digest {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return fmt.Errorf("app-kit version must be %s followed by 64 lowercase hexadecimal characters", prefix)
			}
		}
	}
	return nil
}

func validateAppkitSelection(got moduleSelection, version string) error {
	if got.Path != appkitModule || got.Version != version {
		return fmt.Errorf("active app-kit selection mismatch: path=%q version=%q", got.Path, got.Version)
	}
	if got.Replace != nil {
		return fmt.Errorf("active app-kit selection must not have a replacement: path=%q version=%q", got.Replace.Path, got.Replace.Version)
	}
	return nil
}

func validateDownload(got moduleDownload) error {
	if got.Error != "" {
		return fmt.Errorf("module download reported: %s", got.Error)
	}
	if got.Path != wailsModule || got.Version != wailsVersion || got.Sum != wailsSum || got.GoModSum != wailsGoModSum {
		return fmt.Errorf("pinned Wails identity mismatch: path=%q version=%q sum=%q goModSum=%q", got.Path, got.Version, got.Sum, got.GoModSum)
	}
	info, err := os.Lstat(got.Dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("verified Wails source directory is unavailable or unsafe: %q: %w", got.Dir, err)
	}
	return nil
}

func validateSelection(got moduleSelection) error {
	if got.Path != wailsModule || got.Version != wailsVersion {
		return fmt.Errorf("active Wails selection mismatch: path=%q version=%q", got.Path, got.Version)
	}
	if got.Replace != nil {
		return fmt.Errorf("active Wails selection must not have a replacement: path=%q version=%q", got.Replace.Path, got.Replace.Version)
	}
	return nil
}

func validateModuleFile(path, want string) ([]byte, error) {
	body, err := readRegular(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	line, _, _ := strings.Cut(string(body), "\n")
	if line != "module "+want {
		return nil, fmt.Errorf("%s module line is %q, want %q", path, line, "module "+want)
	}
	return body, nil
}

func copySource(source, stageRoot string) (string, error) {
	destination := filepath.Join(stageRoot, stagedModuleDir)
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("staged module destination already exists: %s", destination)
	}
	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		return "", fmt.Errorf("copy Wails source after go mod verify: %w", err)
	}
	return destination, nil
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	return os.ReadFile(path)
}

func hashRegular(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeExclusive(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func unchanged(path string, before []byte) error {
	after, err := readRegular(path)
	if err != nil {
		return fmt.Errorf("re-read %s: %w", path, err)
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("source module file changed during preparation: %s", path)
	}
	return nil
}

func replaceEnvironment(environ []string, values map[string]string) []string {
	result := make([]string, 0, len(environ)+len(values))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := values[key]; !replace {
			result = append(result, entry)
		}
	}
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

func gitEnvironment(environ []string) []string {
	result := make([]string, 0, len(environ)+3)
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(key, "GIT_") {
			result = append(result, entry)
		}
	}
	return append(result,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
	)
}

func commandError(action string, err error, output []byte) error {
	const max = 2048
	if len(output) > max {
		output = output[len(output)-max:]
	}
	return fmt.Errorf("%s: %w: %s", action, err, strings.TrimSpace(string(output)))
}
