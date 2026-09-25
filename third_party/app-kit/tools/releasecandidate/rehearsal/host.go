package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/sumdb/dirhash"
)

const hostModule = candidateModule + "/spikes/wails-gallery"

type hostRun struct {
	Run                runResult `json:"execution"`
	NativeSHA256       string    `json:"native_sha256,omitempty"`
	ModuleSHA256       string    `json:"prepared_module_sha256,omitempty"`
	SumSHA256          string    `json:"prepared_sum_sha256,omitempty"`
	NativeModuleSHA256 string    `json:"native_module_sha256,omitempty"`
	PatchedWailsSum    string    `json:"patched_wails_sum,omitempty"`
	NativeMinimumOS    string    `json:"native_minimum_os,omitempty"`
	Error              string    `json:"error,omitempty"`
}

type hostReport struct {
	Kind            string           `json:"kind"`
	Candidate       candidateReceipt `json:"candidate"`
	Source          string           `json:"source"`
	Files           []fileRecord     `json:"host_source_files"`
	HarnessSHA256   string           `json:"harness_sha256"`
	Runs            []hostRun        `json:"runs"`
	SourceUnchanged bool             `json:"source_unchanged"`
	IdenticalServer bool             `json:"identical_server"`
	IdenticalNative bool             `json:"identical_native"`
	Error           string           `json:"error,omitempty"`
	Limitations     []string         `json:"limitations"`
}

// rehearseHost retains the actual spike sources and executed tool before any
// build. The root candidate remains a separate verified artifact; no source
// replacement, installer, native window or signed application is introduced.
func rehearseHost(in inputs, source string) (retErr error) {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	files, inventory, err := readHostSource(source)
	if err != nil {
		return err
	}
	result := hostReport{Kind: "app-kit-private-gallery-host-rehearsal-v1", Candidate: in.receipt,
		Source: source, Files: inventory,
		Limitations: []string{
			"experimental macOS and server build evidence, not signing, installation, native input/media or release certification",
			"builds deny network and fleet source; selected HTTP tests separately permit only loopback networking",
			"embedded Gallery CSS is supplied by the exact kit candidate; no new CSS compilation or browser visual evidence",
			"empty Go caches reuse explicit previously cached dependency file proxies and existing platform compilers",
		},
	}
	defer func() {
		if retErr != nil {
			result.Error = retErr.Error()
		}
		body, err := json.MarshalIndent(result, "", "  ")
		if err == nil {
			err = hostWrite(filepath.Join(in.output, "host-receipt.json"), append(body, '\n'))
		}
		retErr = errors.Join(retErr, err)
	}()
	if err := writeHostFiles(filepath.Join(in.output, "source-snapshot"), files); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	body, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	result.HarnessSHA256 = hashBytes(body)
	if err := hostWrite(filepath.Join(in.output, "executed-rehearsal"), body); err != nil {
		return err
	}
	for i := 1; i <= 2; i++ {
		fmt.Fprintf(os.Stderr, "host rehearsal: independent run %d of 2\n", i)
		run, err := buildHost(in, files, fmt.Sprintf("host-run-%d", i))
		if err != nil {
			run.Error = err.Error()
		}
		result.Runs = append(result.Runs, run)
		if err != nil {
			return err
		}
	}
	after, _, err := readHostSource(source)
	if err != nil {
		return err
	}
	result.SourceUnchanged = reflect.DeepEqual(files, after)
	result.IdenticalServer = result.Runs[0].Run.BinarySHA256 == result.Runs[1].Run.BinarySHA256
	result.IdenticalNative = result.Runs[0].NativeSHA256 == result.Runs[1].NativeSHA256
	if !result.SourceUnchanged || !result.IdenticalServer || !result.IdenticalNative {
		return errors.New("source drift or independent host binary mismatch; inspect retained receipt")
	}
	fmt.Fprintln(os.Stdout, "independent server/native builds and selected HTTP tests passed;", filepath.Join(in.output, "host-receipt.json"))
	return nil
}

func readHostSource(root string) (map[string][]byte, []fileRecord, error) {
	// Git supplies the explicit visible inventory, not ignored binaries/caches.
	cmd := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ".")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("host source inventory: %w", err)
	}
	names := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	sort.Strings(names)
	files := make(map[string][]byte)
	var records []fileRecord
	var total int
	for _, name := range names {
		if !hostSourceName(name) {
			return nil, nil, fmt.Errorf("unexpected host source path %q", name)
		}
		if _, exists := files[name]; exists {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path {
			return nil, nil, fmt.Errorf("host source must not traverse a symlink: %s", name)
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
			return nil, nil, fmt.Errorf("unsafe or oversized host source: %s", name)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		total += len(body)
		if len(body) > 2<<20 || total > 16<<20 || len(files) >= 200 {
			return nil, nil, errors.New("host source exceeds bounded snapshot")
		}
		files[name] = body
		records = append(records, fileRecord{Path: name, Bytes: int64(len(body)), SHA256: hashBytes(body)})
	}
	for _, name := range []string{"go.mod", "go.sum", "main.go", "host.go", "host_test.go", "native.go", "cmd/prepare-wails/main.go", "patches/wails-v3.0.0-beta.21-http-only-origin.patch", "patches/wails-v3.0.0-beta.21-media-denial.patch"} {
		if len(files[name]) == 0 {
			return nil, nil, fmt.Errorf("host source missing %s", name)
		}
	}
	return files, records, nil
}

func hostSourceName(name string) bool {
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		return false
	}
	dir := filepath.ToSlash(filepath.Dir(name))
	if dir != "." && dir != "internal/hostwire" && dir != "internal/macbundle" &&
		dir != "cmd/prepare-wails" && dir != "cmd/native-probe" &&
		dir != "cmd/bundle-launcher" && dir != "cmd/prepare-bundle" && dir != "patches" {
		return false
	}
	switch filepath.Ext(name) {
	case ".go", ".m", ".h", ".md", ".mod", ".sum", ".patch", ".plist", ".mjs", ".swift":
		return !strings.HasPrefix(filepath.Base(name), ".")
	}
	return false
}

func prepareHostModule(body []byte, version string) ([]byte, error) {
	if !versionPattern.MatchString(version) {
		return nil, errors.New("invalid exact candidate version")
	}
	mod, err := modfile.Parse("go.mod", body, nil)
	if err != nil {
		return nil, err
	}
	if mod.Module == nil || mod.Module.Mod.Path != hostModule {
		return nil, errors.New("not the Gallery host module")
	}
	if len(mod.Replace) != 1 || mod.Replace[0].Old.Path != candidateModule || mod.Replace[0].Old.Version != "" || mod.Replace[0].New.Path != "../.." || mod.Replace[0].New.Version != "" {
		return nil, errors.New("host source must have only the known app-kit development replacement")
	}
	if err := mod.DropReplace(candidateModule, ""); err != nil {
		return nil, err
	}
	if err := mod.AddRequire(candidateModule, version); err != nil {
		return nil, err
	}
	mod.Cleanup()
	return mod.Format()
}

func writeHostFiles(root string, files map[string][]byte) error {
	if err := os.Mkdir(root, 0o700); err != nil {
		return err
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := hostWrite(path, body); err != nil {
			return err
		}
	}
	return nil
}

func hostWrite(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(body)
	return errors.Join(err, f.Close())
}

func buildHost(in inputs, original map[string][]byte, name string) (result hostRun, retErr error) {
	result.Run.Name = name
	root := filepath.Join(in.output, name)
	for _, dir := range []string{root, filepath.Join(root, "gopath"), filepath.Join(root, "module-cache"), filepath.Join(root, "build-cache")} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return result, err
		}
	}
	files := make(map[string][]byte, len(original))
	for name, body := range original {
		files[name] = body
	}
	var err error
	files["go.mod"], err = prepareHostModule(files["go.mod"], in.receipt.Version)
	if err != nil {
		return result, err
	}
	files["go.sum"] = append(bytes.Clone(files["go.sum"]), []byte(fmt.Sprintf("\n%s %s %s\n%s %s/go.mod %s\n", candidateModule, in.receipt.Version, in.receipt.ModuleSum, candidateModule, in.receipt.Version, in.receipt.GoModSum))...)
	work := filepath.Join(root, "host")
	if err := writeHostFiles(work, files); err != nil {
		return result, err
	}
	s := sandboxRunner(in, root, work)
	s.retainLogs = true
	if err := s.expectRefusal(&result.Run, "/usr/bin/head", "-c", "1", "/Volumes/m2ext/code/geoffmilleraz/app-kit/go.mod"); err != nil {
		return result, err
	}
	canary := `import errno,socket
s=socket.socket()
try: s.connect(('127.0.0.1',9))
except PermissionError: pass
else: raise RuntimeError('network not explicitly denied')`
	if _, err := s.run(&result.Run, "/usr/bin/python3", "-c", canary); err != nil {
		return result, err
	}
	if _, err := s.run(&result.Run, "go", "mod", "download", candidateModule+"@"+in.receipt.Version, "github.com/wailsapp/wails/v3@v3.0.0-beta.21"); err != nil {
		return result, err
	}
	out, err := s.run(&result.Run, "go", "list", "-mod=readonly", "-m", "-json", candidateModule)
	if err != nil {
		return result, err
	}
	var selection struct {
		Path, Version, Dir, Sum, GoModSum string
		Replace                           *json.RawMessage
	}
	if err := json.Unmarshal(out, &selection); err != nil {
		return result, err
	}
	if selection.Path != candidateModule || selection.Version != in.receipt.Version || selection.Replace != nil || selection.Sum != in.receipt.ModuleSum || selection.GoModSum != in.receipt.GoModSum || !strings.HasPrefix(selection.Dir, filepath.Join(root, "module-cache")+string(filepath.Separator)) {
		return result, errors.New("host did not resolve the exact candidate in its private cache")
	}
	result.Run.ModuleDir = selection.Dir
	// The preparer verifies the complete selected module graph with GOPROXY=off.
	// Resolve graph metadata from the local proxies first; unlike download-all,
	// this does not demand archives/info for unused platform implementations.
	if _, err := s.run(&result.Run, "go", "mod", "graph"); err != nil {
		return result, err
	}
	out, err = s.run(&result.Run, "go", "run", "-mod=readonly", "./cmd/prepare-wails", "-appkit-version", in.receipt.Version)
	if err != nil {
		return result, err
	}
	var prepared struct{ Directory, ModFile, AppkitVersion string }
	if err := json.Unmarshal(out, &prepared); err != nil {
		return result, err
	}
	if !strings.HasPrefix(prepared.Directory, root+string(filepath.Separator)) || prepared.ModFile != filepath.Join(prepared.Directory, "go.mod") || prepared.AppkitVersion != in.receipt.Version {
		return result, errors.New("preparer receipt escaped run or lost candidate identity")
	}
	result.ModuleSHA256, err = hashFile(prepared.ModFile)
	if err != nil {
		return result, err
	}
	result.SumSHA256, err = hashFile(filepath.Join(prepared.Directory, "go.sum"))
	if err != nil {
		return result, err
	}
	// Preserve the preparer's original receipt and absolute-path module file.
	// A separately hash-checked copy gives both cold roots the same relative
	// replacement identity; -trimpath alone does not remove Go build-info paths.
	wailsSource := filepath.Join(prepared.Directory, "wails-v3.0.0-beta.21")
	stableWails := filepath.Join(root, "patched-wails")
	if err := os.CopyFS(stableWails, os.DirFS(wailsSource)); err != nil {
		return result, err
	}
	beforeSum, err := dirhash.HashDir(wailsSource, "wails", dirhash.Hash1)
	if err != nil {
		return result, err
	}
	result.PatchedWailsSum, err = dirhash.HashDir(stableWails, "wails", dirhash.Hash1)
	if err != nil || result.PatchedWailsSum != beforeSum {
		return result, errors.New("stable-path Wails copy differs from prepared source")
	}
	modBytes, err := os.ReadFile(prepared.ModFile)
	if err != nil {
		return result, err
	}
	modBytes, err = hostNativeModule(modBytes, wailsSource)
	if err != nil {
		return result, err
	}
	nativeModFile := filepath.Join(root, "native.mod")
	if err := hostWrite(nativeModFile, modBytes); err != nil {
		return result, err
	}
	sumBytes, err := os.ReadFile(filepath.Join(prepared.Directory, "go.sum"))
	if err != nil {
		return result, err
	}
	if err := hostWrite(filepath.Join(root, "native.sum"), sumBytes); err != nil {
		return result, err
	}
	result.NativeModuleSHA256 = hashBytes(modBytes)
	server := filepath.Join(root, "gallery-server")
	if _, err := s.run(&result.Run, "go", "build", "-mod=readonly", "-tags=server", "-trimpath", "-buildvcs=false", "-o", server, "."); err != nil {
		return result, err
	}
	result.Run.BinarySHA256, err = hashFile(server)
	if err != nil {
		return result, err
	}
	// Native compilation is separate from the cgo-free browser/server artifact.
	native := s
	native.env = append(append([]string{}, s.env...), "CGO_ENABLED=1", "MACOSX_DEPLOYMENT_TARGET=14.0")
	nativeBin := filepath.Join(root, "gallery-native")
	if _, err := native.run(&result.Run, "go", "build", "-mod=readonly", "-modfile="+nativeModFile, "-tags=appkit_contained", "-trimpath", "-buildvcs=false", "-ldflags=-extldflags=-mmacosx-version-min=14.0", "-o", nativeBin, "."); err != nil {
		return result, err
	}
	result.NativeSHA256, err = hashFile(nativeBin)
	if err != nil {
		return result, err
	}
	out, err = s.run(&result.Run, "/usr/bin/otool", "-l", nativeBin)
	if err != nil {
		return result, err
	}
	if err := checkNativeMinimumOS(string(out)); err != nil {
		return result, err
	}
	result.NativeMinimumOS = "14.0"
	// Existing actual-handler tests need a socket, unlike builds. Retain a
	// distinct policy: fleet sources and non-loopback networking remain denied.
	local := s
	local.prefix = append([]string{}, s.prefix...)
	local.prefix[2] += "\n(allow network-inbound (local ip \"localhost:*\"))\n(allow network-outbound (remote ip \"localhost:*\"))"
	externalCanary := `import socket
s=socket.socket();s.settimeout(0.2)
try: s.connect(('192.0.2.1',9))
except PermissionError: pass
else: raise RuntimeError('non-loopback network not explicitly denied')`
	if _, err := local.run(&result.Run, "/usr/bin/python3", "-c", externalCanary); err != nil {
		return result, err
	}
	if _, err := local.run(&result.Run, "go", "test", "-mod=readonly", "-tags=server", "-count=1", "-v", "-run=^Test(HostGalleryPOSTAndSSE|HostCancelledStartCloses|ServerBuildDoesNotImportNativeRuntime)$", "."); err != nil {
		return result, err
	}
	if _, err := s.run(&result.Run, "go", "test", "-mod=readonly", "-tags=server", "-count=1", "./internal/hostwire", "./cmd/prepare-wails"); err != nil {
		return result, err
	}
	return result, nil
}

func hostNativeModule(body []byte, expectedWailsPath string) ([]byte, error) {
	mod, err := modfile.Parse("go.mod", body, nil)
	if err != nil {
		return nil, err
	}
	const wails = "github.com/wailsapp/wails/v3"
	if len(mod.Replace) != 1 || mod.Replace[0].Old.Path != wails || mod.Replace[0].Old.Version != "" || mod.Replace[0].New.Path != expectedWailsPath || mod.Replace[0].New.Version != "" {
		return nil, errors.New("prepared module has an unexpected replacement")
	}
	if err := mod.AddReplace(wails, "", "../patched-wails", ""); err != nil {
		return nil, err
	}
	return mod.Format()
}

func checkNativeMinimumOS(loadCommands string) error {
	// Inspect the linked Mach-O contract, not the requested compiler environment.
	blocks := strings.Split(loadCommands, "Load command ")
	count := 0
	for _, block := range blocks {
		fields := strings.Fields(block)
		for i, field := range fields {
			if field == "LC_BUILD_VERSION" {
				count++
				found := false
				for j := i + 1; j+1 < len(fields); j++ {
					if fields[j] == "minos" {
						found = fields[j+1] == "14.0"
						break
					}
				}
				if !found {
					return errors.New("native binary does not declare requested macOS 14.0 minimum")
				}
			}
		}
	}
	if count != 1 {
		return errors.New("native binary must have exactly one LC_BUILD_VERSION")
	}
	return nil
}
