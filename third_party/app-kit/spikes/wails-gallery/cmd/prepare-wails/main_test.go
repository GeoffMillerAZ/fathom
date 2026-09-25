package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCandidateVersion = "v0.0.0-rehearsal.h0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestParseOptionsRequiresExactPrivateCandidateVersion(t *testing.T) {
	options, err := parseOptions(nil)
	if err != nil || options.appkitVersion != "" {
		t.Fatalf("default options = %+v, %v", options, err)
	}
	options, err = parseOptions([]string{"-appkit-version", testCandidateVersion})
	if err != nil || options.appkitVersion != testCandidateVersion {
		t.Fatalf("candidate options = %+v, %v", options, err)
	}
	for _, args := range [][]string{
		{"-appkit-version"},
		{"-appkit-version="},
		{"-appkit-version=v0.0.0-rehearsal.habc"},
		{"-appkit-version=v0.0.0-rehearsal.h0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef"},
		{"-appkit-version=v1.0.0"},
		{"unexpected"},
	} {
		if _, err := parseOptions(args); err == nil {
			t.Fatalf("invalid options accepted: %q", args)
		}
	}
}

func TestValidateAppkitSelectionRejectsDriftAndReplacement(t *testing.T) {
	valid := moduleSelection{Path: appkitModule, Version: testCandidateVersion}
	if err := validateAppkitSelection(valid, testCandidateVersion); err != nil {
		t.Fatalf("valid app-kit selection refused: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*moduleSelection)
	}{
		{name: "path", mutate: func(got *moduleSelection) { got.Path += "-other" }},
		{name: "version", mutate: func(got *moduleSelection) { got.Version += "0" }},
		{name: "local_replace", mutate: func(got *moduleSelection) {
			got.Replace = &struct{ Path, Version string }{Path: "/tmp/untrusted"}
		}},
		{name: "module_replace", mutate: func(got *moduleSelection) {
			got.Replace = &struct{ Path, Version string }{Path: appkitModule, Version: testCandidateVersion}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := valid
			tc.mutate(&got)
			if err := validateAppkitSelection(got, testCandidateVersion); err == nil {
				t.Fatal("drifted app-kit selection was accepted")
			}
		})
	}
}

func TestValidateDownloadRefusesSourceIdentity(t *testing.T) {
	valid := moduleDownload{
		Path: wailsModule, Version: wailsVersion, Dir: t.TempDir(),
		Sum: wailsSum, GoModSum: wailsGoModSum,
	}
	if err := validateDownload(valid); err != nil {
		t.Fatalf("valid pinned identity refused: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*moduleDownload)
	}{
		{name: "path", mutate: func(got *moduleDownload) { got.Path += "-other" }},
		{name: "version", mutate: func(got *moduleDownload) { got.Version = "v3.0.0-beta.20" }},
		{name: "sum", mutate: func(got *moduleDownload) { got.Sum = "h1:wrong" }},
		{name: "go_mod_sum", mutate: func(got *moduleDownload) { got.GoModSum = "h1:wrong" }},
		{name: "missing_directory", mutate: func(got *moduleDownload) { got.Dir = filepath.Join(t.TempDir(), "missing") }},
		{name: "reported_error", mutate: func(got *moduleDownload) { got.Error = "synthetic refusal" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := valid
			tc.mutate(&got)
			if err := validateDownload(got); err == nil {
				t.Fatal("changed source identity was accepted")
			}
		})
	}
}

func TestValidateSelectionRejectsDriftAndReplacement(t *testing.T) {
	valid := moduleSelection{Path: wailsModule, Version: wailsVersion}
	if err := validateSelection(valid); err != nil {
		t.Fatalf("valid active selection refused: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*moduleSelection)
	}{
		{name: "path", mutate: func(got *moduleSelection) { got.Path += "-other" }},
		{name: "version", mutate: func(got *moduleSelection) { got.Version = "v3.0.0-beta.20" }},
		{name: "local_replace", mutate: func(got *moduleSelection) {
			got.Replace = &struct{ Path, Version string }{Path: "/tmp/untrusted"}
		}},
		{name: "module_replace", mutate: func(got *moduleSelection) {
			got.Replace = &struct{ Path, Version string }{Path: wailsModule, Version: "v3.0.0-beta.20"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := valid
			tc.mutate(&got)
			if err := validateSelection(got); err == nil {
				t.Fatal("drifted active selection was accepted")
			}
		})
	}
}

func TestCopySourceCreatesNewDestinationAndNeverOverwrites(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "source.txt"), []byte("verified source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	destination, err := copySource(source, stage)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(destination, "source.txt")); err != nil || string(got) != "verified source\n" {
		t.Fatalf("copied source = %q, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(destination, "sentinel"), []byte("retain\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := copySource(source, stage); err == nil {
		t.Fatal("existing staging destination was overwritten")
	}
	if got, err := os.ReadFile(filepath.Join(destination, "sentinel")); err != nil || string(got) != "retain\n" {
		t.Fatalf("failed second copy changed destination: %q, %v", got, err)
	}
}

func TestModuleAndPatchInputsAreRegularAndExact(t *testing.T) {
	root := t.TempDir()
	modulePath := filepath.Join(root, "go.mod")
	if err := os.WriteFile(modulePath, []byte("module "+spikeModule+"\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := validateModuleFile(modulePath, spikeModule); err != nil {
		t.Fatal(err)
	}
	if _, err := validateModuleFile(modulePath, appkitModule); err == nil {
		t.Fatal("wrong module identity was accepted")
	}
	target := filepath.Join(root, "patch")
	if err := os.WriteFile(target, []byte("patch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "patch-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := hashRegular(link); err == nil || !strings.Contains(err.Error(), "regular") {
		t.Fatalf("symlink patch was not refused: %v", err)
	}
}

func TestPinnedPatchesAreExactAndMediaContractPreservesDefaultPrompt(t *testing.T) {
	spikeRoot := filepath.Join("..", "..")
	for _, tc := range []struct {
		name, relative, wantSHA string
	}{
		{name: "containment", relative: patchRelative, wantSHA: patchSHA256},
		{name: "media_denial", relative: mediaPatchRelative, wantSHA: mediaPatchSHA256},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(spikeRoot, filepath.FromSlash(tc.relative))
			got, err := hashRegular(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.wantSHA {
				t.Fatalf("patch SHA-256 = %q, want %q", got, tc.wantSHA)
			}
		})
	}

	body, err := readRegular(filepath.Join(spikeRoot, filepath.FromSlash(mediaPatchRelative)))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{
		"@property (nonatomic, readonly) NSUInteger httpOnlyCameraDenials;",
		"@property (nonatomic, readonly) NSUInteger httpOnlyMicrophoneDenials;",
		"@property (nonatomic, readonly) NSUInteger httpOnlyCombinedDenials;",
		"@property (nonatomic, readonly) BOOL httpOnlyMediaOriginMatched;",
		"#if MAC_OS_X_VERSION_MAX_ALLOWED >= 120000",
		"requestMediaCapturePermissionForOrigin:(WKSecurityOrigin *)origin",
		"if (self.httpOnlyOrigin == nil)",
		"decisionHandler(WKPermissionDecisionPrompt);",
		"if (![NSThread isMainThread])",
		"case WKMediaCaptureTypeCamera:",
		"case WKMediaCaptureTypeMicrophone:",
		"case WKMediaCaptureTypeCameraAndMicrophone:",
		"[requestedOrigin isEqualToString:self.httpOnlyOrigin]",
		"decisionHandler(WKPermissionDecisionDeny);",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("media-denial patch is missing contract marker %q", required)
		}
	}
	if strings.Contains(text, "WKPermissionDecisionGrant") {
		t.Fatal("bounded media-denial patch unexpectedly grants media access")
	}
}

func TestWriteExclusiveDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.mod")
	if err := writeExclusive(path, []byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusive(path, []byte("second\n")); err == nil {
		t.Fatal("exclusive writer overwrote existing file")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "first\n" {
		t.Fatalf("existing file changed: %q, %v", got, err)
	}
}

type candidatePrepareRunner struct {
	source               string
	version              string
	appkitSelectionReads int
	editArgs             []string
}

func (runner *candidatePrepareRunner) run(_ context.Context, _ string, name string, _ []string, args ...string) ([]byte, error) {
	if name == "go" && len(args) > 0 && args[0] == "list" {
		module := args[len(args)-1]
		selection := moduleSelection{Path: module}
		switch module {
		case appkitModule:
			selection.Version = runner.version
			runner.appkitSelectionReads++
		case wailsModule:
			selection.Version = wailsVersion
		default:
			return nil, fmt.Errorf("unexpected module selection %q", module)
		}
		return json.Marshal(selection)
	}
	if name == "go" && len(args) >= 2 && args[0] == "mod" && args[1] == "download" {
		return json.Marshal(moduleDownload{
			Path: wailsModule, Version: wailsVersion, Dir: runner.source,
			Sum: wailsSum, GoModSum: wailsGoModSum,
		})
	}
	if name == "go" && len(args) >= 2 && args[0] == "mod" && args[1] == "verify" {
		return nil, nil
	}
	if name == "git" && len(args) > 0 && args[0] == "apply" {
		return nil, nil
	}
	if name == "go" && len(args) >= 2 && args[0] == "mod" && args[1] == "edit" {
		runner.editArgs = append([]string(nil), args...)
		var modFile, replacement string
		for _, arg := range args[2:] {
			if strings.HasPrefix(arg, "-modfile=") {
				modFile = strings.TrimPrefix(arg, "-modfile=")
			}
			if strings.HasPrefix(arg, "-replace="+wailsModule+"=") {
				replacement = strings.TrimPrefix(arg, "-replace=")
			}
		}
		oldPath, newPath, ok := strings.Cut(replacement, "=")
		if modFile == "" || !ok {
			return nil, fmt.Errorf("candidate mod edit omitted its modfile or Wails replacement")
		}
		file, err := os.OpenFile(modFile, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			return nil, err
		}
		_, writeErr := fmt.Fprintf(file, "\nreplace %s => %s\n", oldPath, newPath)
		closeErr := file.Close()
		if writeErr != nil {
			return nil, writeErr
		}
		return nil, closeErr
	}
	return nil, fmt.Errorf("unexpected command: %s %q", name, args)
}

func TestPrepareCandidateUsesExactModuleWithoutSiblingCheckout(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "nested", "spike")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	goMod := "module " + spikeModule + "\n\ngo 1.26.2\n\nrequire (\n\t" + appkitModule + " " + testCandidateVersion + "\n\t" + wailsModule + " " + wailsVersion + "\n)\n"
	if err := os.WriteFile(filepath.Join(cwd, "go.mod"), []byte(goMod), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "go.sum"), []byte("synthetic sum fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spikeRoot := filepath.Join("..", "..")
	for _, relative := range []string{patchRelative, mediaPatchRelative} {
		body, err := os.ReadFile(filepath.Join(spikeRoot, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(cwd, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(root, "wails-source")
	for _, relative := range patchedFiles {
		path := filepath.Join(source, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("synthetic patched Wails source\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	runner := &candidatePrepareRunner{source: source, version: testCandidateVersion}
	receipt, err := prepareAt(t.Context(), runner, cwd, prepareOptions{appkitVersion: testCandidateVersion})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(receipt.Directory) })
	if receipt.AppkitVersion != testCandidateVersion || runner.appkitSelectionReads != 2 {
		t.Fatalf("candidate receipt/selection reads = %q/%d", receipt.AppkitVersion, runner.appkitSelectionReads)
	}
	staged, err := os.ReadFile(receipt.ModFile)
	if err != nil {
		t.Fatal(err)
	}
	stagedText := string(staged)
	if !strings.Contains(stagedText, appkitModule+" "+testCandidateVersion) || strings.Contains(stagedText, "replace "+appkitModule) {
		t.Fatal("staged module lost the exact app-kit requirement or added an app-kit replacement")
	}
	wailsReplacement := false
	for _, arg := range runner.editArgs {
		if strings.HasPrefix(arg, "-replace="+appkitModule+"=") {
			t.Fatal("candidate mod edit added a sibling app-kit replacement")
		}
		if strings.HasPrefix(arg, "-replace="+wailsModule+"=") {
			wailsReplacement = true
		}
	}
	if !wailsReplacement {
		t.Fatal("candidate mod edit omitted the verified Wails source replacement")
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); !os.IsNotExist(err) {
		t.Fatal("candidate fixture unexpectedly supplied a sibling app-kit checkout")
	}
}
