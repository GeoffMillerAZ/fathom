// Command rehearsal proves a private app-kit candidate through a synthetic,
// external consumer. It does not prepare, publish, or certify a release.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	tailwindpin "github.com/geoffmilleraz/app-kit/internal/tailwind"
	"golang.org/x/mod/sumdb/dirhash"
)

const (
	candidateModule = "github.com/geoffmilleraz/app-kit"
	candidateKind   = "maintainer-private-unpublished-rehearsal"
)

var versionPattern = regexp.MustCompile(`^v0\.0\.0-rehearsal\.h[0-9a-f]{64}$`)

//go:embed fixture/go.mod.txt fixture/input.css fixture/main.go
var fixtureFS embed.FS

type stringFlags []string

func (s *stringFlags) String() string { return strings.Join(*s, ",") }
func (s *stringFlags) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type fileRecord struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type exclusion struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type candidateReceipt struct {
	Kind        string       `json:"kind"`
	Module      string       `json:"module"`
	Version     string       `json:"version"`
	SourceHEAD  string       `json:"source_head"`
	TreeSHA256  string       `json:"tree_sha256"`
	ZipSHA256   string       `json:"zip_sha256"`
	ModuleSum   string       `json:"module_sum"`
	GoModSum    string       `json:"go_mod_sum"`
	Files       []fileRecord `json:"files"`
	TotalBytes  int64        `json:"total_bytes"`
	Exclusions  []exclusion  `json:"exclusions,omitempty"`
	Limitations []string     `json:"limitations"`
	Output      string       `json:"output"`
}

type commandRecord struct {
	Args       []string `json:"args"`
	StdoutHash string   `json:"stdout_sha256,omitempty"`
	StderrHash string   `json:"stderr_sha256,omitempty"`
	StdoutFile string   `json:"stdout_file,omitempty"`
	StderrFile string   `json:"stderr_file,omitempty"`
}

type runResult struct {
	Name          string          `json:"name"`
	Commands      []commandRecord `json:"commands"`
	BinarySHA256  string          `json:"binary_sha256"`
	CSSSHA256     string          `json:"css_sha256"`
	HandlerSHA256 string          `json:"handler_sha256"`
	ModuleDir     string          `json:"module_dir"`
}

type report struct {
	Kind               string      `json:"kind"`
	CandidateModule    string      `json:"candidate_module"`
	CandidateVersion   string      `json:"candidate_version"`
	CandidateZIPSHA256 string      `json:"candidate_zip_sha256"`
	CandidateModuleSum string      `json:"candidate_module_sum"`
	CandidateGoModSum  string      `json:"candidate_go_mod_sum"`
	TailwindSHA256     string      `json:"tailwind_sha256"`
	TailwindVersion    string      `json:"tailwind_version"`
	FixtureSHA256      string      `json:"fixture_sha256"`
	Output             string      `json:"output"`
	Runs               []runResult `json:"runs"`
	Identical          bool        `json:"identical"`
	Limitations        []string    `json:"limitations"`
}

type inputs struct {
	receiptPath, candidateProxy, tailwind, output string
	publicProxies                                 []string
	receipt                                       candidateReceipt
	goModSum                                      string
	tailwindSum                                   string
	tailwindVersion                               string
	fixture                                       map[string][]byte
	fixtureSum                                    string
}

func main() {
	var public stringFlags
	receiptPath := flag.String("receipt", "", "exact frozen candidate receipt.json")
	candidateProxy := flag.String("candidate-proxy", "", "exact candidate file-proxy directory")
	tailwind := flag.String("tailwind", "", "trusted standalone Tailwind executable")
	output := flag.String("out", "", "fresh output directory; retained on success and failure")
	hostSource := flag.String("host-source", "", "opt-in: exact Wails Gallery spike source for independent server/native builds")
	flag.Var(&public, "public-proxy", "public dependency file-proxy directory (repeatable)")
	flag.Parse()
	if flag.NArg() != 0 || *receiptPath == "" || *candidateProxy == "" || *tailwind == "" || *output == "" || len(public) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rehearsal -receipt FILE -candidate-proxy DIR -tailwind FILE -public-proxy DIR [-public-proxy DIR...] -out FRESH_DIR")
		os.Exit(2)
	}
	in, err := validateInputs(*receiptPath, *candidateProxy, *tailwind, public, *output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rehearsal:", err)
		os.Exit(1)
	}
	if err := os.Mkdir(in.output, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "rehearsal: create retained output:", err)
		os.Exit(1)
	}
	if *hostSource != "" {
		if err := rehearseHost(in, *hostSource); err != nil {
			fmt.Fprintln(os.Stderr, "host rehearsal:", err, "retained at", in.output)
			os.Exit(1)
		}
		return
	}

	result := report{
		Kind: "app-kit-private-candidate-consumer-rehearsal-v1", CandidateModule: in.receipt.Module,
		CandidateVersion: in.receipt.Version, CandidateZIPSHA256: in.receipt.ZipSHA256,
		CandidateModuleSum: in.receipt.ModuleSum, CandidateGoModSum: in.receipt.GoModSum,
		TailwindSHA256: in.tailwindSum, TailwindVersion: in.tailwindVersion,
		FixtureSHA256: in.fixtureSum, Output: in.output,
		Limitations: []string{
			"synthetic handler and static CSS evidence only; no browser, runtime UX, accessibility, or real-consumer certification",
			"private unpublished candidate only; no tag, publication, installation, or consumer mutation",
		},
	}
	for i := 1; i <= 2; i++ {
		run, runErr := rehearse(in, fmt.Sprintf("run-%d", i))
		result.Runs = append(result.Runs, run)
		if runErr != nil {
			_ = writeReport(in.output, result)
			fmt.Fprintln(os.Stderr, "rehearsal:", runErr)
			fmt.Fprintln(os.Stderr, "rehearsal: retained artifacts:", in.output)
			os.Exit(1)
		}
	}
	a, b := result.Runs[0], result.Runs[1]
	result.Identical = a.BinarySHA256 == b.BinarySHA256 && a.CSSSHA256 == b.CSSSHA256 && a.HandlerSHA256 == b.HandlerSHA256
	if !result.Identical {
		_ = writeReport(in.output, result)
		fmt.Fprintln(os.Stderr, "rehearsal: independent build outputs differ; retained artifacts:", in.output)
		os.Exit(1)
	}
	if err := writeReport(in.output, result); err != nil {
		fmt.Fprintln(os.Stderr, "rehearsal:", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "rehearsal: write result:", err)
		os.Exit(1)
	}
}

func validateInputs(receiptPath, candidateProxy, tailwind string, public []string, output string) (inputs, error) {
	if runtime.GOOS != "darwin" {
		return inputs{}, errors.New("macOS sandbox-exec is required")
	}
	var in inputs
	paths := []*string{&receiptPath, &candidateProxy, &tailwind, &output}
	for _, path := range paths {
		absolute, err := filepath.Abs(*path)
		if err != nil {
			return inputs{}, err
		}
		*path = filepath.Clean(absolute)
	}
	// Resolve parent aliases such as macOS /tmp -> /private/tmp. Do not resolve
	// the leaf: existing-output and explicit symlink refusal still apply.
	for _, path := range []*string{&candidateProxy, &output} {
		parent, err := filepath.EvalSymlinks(filepath.Dir(*path))
		if err != nil {
			return inputs{}, fmt.Errorf("resolve input/output parent: %w", err)
		}
		*path = filepath.Join(parent, filepath.Base(*path))
	}
	body, err := os.ReadFile(receiptPath)
	if err != nil {
		return inputs{}, fmt.Errorf("read candidate receipt: %w", err)
	}
	in.receipt, err = decodeCandidateReceipt(body)
	if err != nil {
		return inputs{}, fmt.Errorf("decode candidate receipt: %w", err)
	}
	if in.receipt.Kind != candidateKind || in.receipt.Module != candidateModule || !versionPattern.MatchString(in.receipt.Version) {
		return inputs{}, errors.New("candidate receipt identity is invalid")
	}
	if err := validateReceiptIdentity(in.receipt); err != nil {
		return inputs{}, err
	}
	if in.receipt.SourceHEAD == "" || in.receipt.TreeSHA256 == "" || in.receipt.ZipSHA256 == "" || in.receipt.ModuleSum == "" || in.receipt.GoModSum == "" || in.receipt.TotalBytes <= 0 {
		return inputs{}, errors.New("candidate receipt is incomplete")
	}
	receiptOutput, err := filepath.Abs(in.receipt.Output)
	if err != nil || filepath.Clean(receiptOutput) != filepath.Dir(candidateProxy) || filepath.Base(candidateProxy) != "proxy" {
		return inputs{}, errors.New("explicit candidate proxy does not match receipt output/proxy")
	}
	if _, err := os.Lstat(output); err == nil {
		return inputs{}, errors.New("output directory already exists")
	} else if !os.IsNotExist(err) {
		return inputs{}, fmt.Errorf("inspect output: %w", err)
	}
	if err := requireRealDir(candidateProxy); err != nil {
		return inputs{}, fmt.Errorf("candidate proxy: %w", err)
	}
	for i, path := range public {
		path, err = filepath.Abs(path)
		if err != nil {
			return inputs{}, err
		}
		path = filepath.Clean(path)
		if err := requireRealDir(path); err != nil {
			return inputs{}, fmt.Errorf("public proxy: %w", err)
		}
		public[i] = path
	}
	twInfo, err := os.Lstat(tailwind)
	if err != nil || !twInfo.Mode().IsRegular() || twInfo.Mode()&0o111 == 0 || twInfo.Mode()&os.ModeSymlink != 0 {
		return inputs{}, errors.New("Tailwind path must be an explicit executable regular file, not a symlink")
	}
	in.tailwindSum, err = hashFile(tailwind)
	if err != nil {
		return inputs{}, err
	}
	manifest, err := tailwindpin.LoadManifest()
	if err != nil {
		return inputs{}, fmt.Errorf("load Tailwind manifest: %w", err)
	}
	pin, ok := manifest.Binaries[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok || in.tailwindSum != pin.SHA256 {
		return inputs{}, errors.New("Tailwind executable does not match the embedded platform pin")
	}
	in.tailwindVersion = manifest.Version
	in.fixture, in.fixtureSum, err = embeddedFixture()
	if err != nil {
		return inputs{}, err
	}
	if err := verifyCandidate(candidateProxy, in.receipt); err != nil {
		return inputs{}, err
	}
	in.goModSum, err = moduleFileSum(candidateProxy, in.receipt.Version)
	if err != nil {
		return inputs{}, err
	}
	if in.goModSum != in.receipt.GoModSum {
		return inputs{}, errors.New("candidate proxy go.mod does not match receipt go_mod_sum")
	}
	in.receiptPath, in.candidateProxy, in.tailwind, in.output, in.publicProxies = receiptPath, candidateProxy, tailwind, output, public
	return in, nil
}

func verifyCandidate(proxy string, receipt candidateReceipt) error {
	dir := filepath.Join(proxy, filepath.FromSlash(candidateModule), "@v")
	zipPath := filepath.Join(dir, receipt.Version+".zip")
	zipSum, err := hashFile(zipPath)
	if err != nil || zipSum != receipt.ZipSHA256 {
		return errors.New("candidate ZIP does not match receipt zip_sha256")
	}
	moduleSum, err := dirhash.HashZip(zipPath, dirhash.Hash1)
	if err != nil || moduleSum != receipt.ModuleSum {
		return errors.New("candidate ZIP does not match receipt module_sum")
	}
	if err := verifyZIPInventory(zipPath, receipt); err != nil {
		return err
	}
	byPath := make(map[string]fileRecord, len(receipt.Files))
	var total int64
	for _, file := range receipt.Files {
		if file.Path == "" || file.Bytes < 0 || file.SHA256 == "" || byPath[file.Path].Path != "" {
			return errors.New("candidate receipt file inventory is invalid")
		}
		byPath[file.Path], total = file, total+file.Bytes
	}
	if total != receipt.TotalBytes {
		return errors.New("candidate receipt total_bytes disagrees with file inventory")
	}
	inventoryJSON, err := json.Marshal(receipt.Files)
	if err != nil || hashBytes(inventoryJSON) != receipt.TreeSHA256 {
		return errors.New("candidate receipt tree_sha256 disagrees with its compact file inventory")
	}
	modRecord, ok := byPath["go.mod"]
	if !ok {
		return errors.New("candidate receipt omits go.mod")
	}
	modPath := filepath.Join(dir, receipt.Version+".mod")
	modInfo, err := os.Stat(modPath)
	if err != nil || modInfo.Size() != modRecord.Bytes {
		return errors.New("candidate proxy go.mod size disagrees with receipt")
	}
	modHash, err := hashFile(modPath)
	if err != nil || modHash != modRecord.SHA256 {
		return errors.New("candidate proxy go.mod hash disagrees with receipt")
	}
	infoBody, err := os.ReadFile(filepath.Join(dir, receipt.Version+".info"))
	if err != nil {
		return fmt.Errorf("read candidate info: %w", err)
	}
	var info struct{ Version string }
	if json.Unmarshal(infoBody, &info) != nil || info.Version != receipt.Version {
		return errors.New("candidate proxy info version disagrees with receipt")
	}
	listBody, err := os.ReadFile(filepath.Join(dir, "list"))
	if err != nil || string(listBody) != receipt.Version+"\n" {
		return errors.New("candidate proxy list disagrees with receipt")
	}
	return nil
}

func validateReceiptIdentity(receipt candidateReceipt) error {
	if receipt.Version != "v0.0.0-rehearsal.h"+receipt.TreeSHA256 {
		return errors.New("candidate version does not bind the receipt tree_sha256")
	}
	return nil
}

func verifyZIPInventory(zipPath string, receipt candidateReceipt) error {
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open candidate ZIP: %w", err)
	}
	defer archive.Close()
	if len(archive.File) != len(receipt.Files) {
		return errors.New("candidate ZIP entry count disagrees with receipt inventory")
	}
	prefix := candidateModule + "@" + receipt.Version + "/"
	records := make(map[string]fileRecord, len(receipt.Files))
	for _, record := range receipt.Files {
		records[record.Path] = record
	}
	seen := make(map[string]bool, len(archive.File))
	for _, file := range archive.File {
		if !strings.HasPrefix(file.Name, prefix) {
			return fmt.Errorf("candidate ZIP entry has wrong module identity: %s", file.Name)
		}
		path := strings.TrimPrefix(file.Name, prefix)
		record, ok := records[path]
		if !ok || seen[path] || file.FileInfo().IsDir() || int64(file.UncompressedSize64) != record.Bytes {
			return fmt.Errorf("candidate ZIP entry disagrees with receipt inventory: %s", path)
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || hashBytes(body) != record.SHA256 {
			return fmt.Errorf("candidate ZIP entry content disagrees with receipt: %s", path)
		}
		seen[path] = true
	}
	return nil
}

func rehearse(in inputs, name string) (runResult, error) {
	result := runResult{Name: name}
	root := filepath.Join(in.output, name)
	work := filepath.Join(root, "consumer")
	for _, dir := range []string{work, filepath.Join(root, "gopath"), filepath.Join(root, "module-cache"), filepath.Join(root, "build-cache")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return result, err
		}
	}
	mainBody, modTemplate, cssBody := in.fixture["main.go"], in.fixture["go.mod.txt"], in.fixture["input.css"]
	modBody := bytes.ReplaceAll(modTemplate, []byte("__APPKIT_VERSION__"), []byte(in.receipt.Version))
	for path, body := range map[string][]byte{"main.go": mainBody, "go.mod": modBody, "input.css": cssBody} {
		if err := os.WriteFile(filepath.Join(work, path), body, 0o600); err != nil {
			return result, err
		}
	}
	goSum := fmt.Sprintf("%s %s %s\n%s %s/go.mod %s\ngithub.com/rivo/uniseg v0.4.7 h1:WUdvkW8uEhrYfLC4ZzdpI2ztxP1I582+49Oc5Mq64VQ=\ngithub.com/rivo/uniseg v0.4.7/go.mod h1:FN3SvrM+Zdj16jyLfmOkMNblXMcoc8DfTHruCPUcx88=\n", candidateModule, in.receipt.Version, in.receipt.ModuleSum, candidateModule, in.receipt.Version, in.goModSum)
	if err := os.WriteFile(filepath.Join(work, "go.sum"), []byte(goSum), 0o600); err != nil {
		return result, err
	}

	runner := sandboxRunner(in, root, work)
	if err := runner.expectRefusal(&result, "/usr/bin/head", "-c", "1", "/Volumes/m2ext/code/geoffmilleraz/app-kit/go.mod"); err != nil {
		return result, fmt.Errorf("%s source denial canary: %w", name, err)
	}
	networkScript := `import socket,sys
try:
 s=socket.socket()
 s.connect(("127.0.0.1",9))
except PermissionError: sys.exit(0)
except OSError as e: print(e,file=sys.stderr); sys.exit(1)
sys.exit(1)`
	if _, err := runner.run(&result, "/usr/bin/python3", "-c", networkScript); err != nil {
		return result, fmt.Errorf("%s network denial canary: %w", name, err)
	}
	downloadBody, err := runner.run(&result, "go", "mod", "download", "-json", candidateModule+"@"+in.receipt.Version)
	if err != nil {
		return result, fmt.Errorf("%s candidate download: %w", name, err)
	}
	var download struct {
		Path, Version, Sum, GoModSum string
		Error                        json.RawMessage
	}
	if err := json.Unmarshal(downloadBody, &download); err != nil || len(download.Error) != 0 || download.Path != candidateModule || download.Version != in.receipt.Version || download.Sum != in.receipt.ModuleSum || download.GoModSum != in.receipt.GoModSum {
		return result, fmt.Errorf("%s Go download identity/sums disagree with candidate receipt", name)
	}
	moduleDirBody, err := runner.run(&result, "go", "list", "-mod=readonly", "-m", "-f", "{{.Dir}}", candidateModule)
	if err != nil {
		return result, fmt.Errorf("%s module resolution: %w", name, err)
	}
	moduleDir := strings.TrimSpace(string(moduleDirBody))
	moduleCache := filepath.Join(root, "module-cache")
	if rel, err := filepath.Rel(moduleCache, moduleDir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return result, errors.New("resolved app-kit outside fresh module cache")
	}
	result.ModuleDir = moduleDir
	for _, dir := range []string{"analytics", "dashboard", "vizframe"} {
		info, err := os.Stat(filepath.Join(moduleDir, "appkit", "ui", dir))
		if err != nil || !info.IsDir() {
			return result, fmt.Errorf("selected candidate package missing: %s", dir)
		}
	}
	candidateSources := []string{
		"appkit/ui/analytics/analytics.go", "appkit/ui/analytics/csv.go", "appkit/ui/analytics/render.go",
		"appkit/ui/dashboard/nav.go", "appkit/ui/dashboard/shell.go", "appkit/ui/vizframe/vizframe.go",
	}
	sources := make([]string, 0, len(candidateSources))
	for _, source := range candidateSources {
		info, err := os.Stat(filepath.Join(moduleDir, source))
		if err != nil || !info.Mode().IsRegular() {
			return result, fmt.Errorf("selected candidate source missing: %s", source)
		}
		sources = append(sources, fmt.Sprintf("@source %q;", filepath.Join(moduleDir, source)))
	}
	if err := os.WriteFile(filepath.Join(work, "kit.sources.css"), []byte(strings.Join(sources, "\n")+"\n"), 0o600); err != nil {
		return result, err
	}
	if _, err := runner.run(&result, "/bin/cp", filepath.Join(moduleDir, "appkit/ui/tokens/theme.css"), filepath.Join(work, "theme.css")); err != nil {
		return result, fmt.Errorf("%s theme copy: %w", name, err)
	}
	if _, err := runner.run(&result, "go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-o", "consumer-bin", "."); err != nil {
		return result, fmt.Errorf("%s consumer build: %w", name, err)
	}
	handlerBody, err := runner.run(&result, filepath.Join(work, "consumer-bin"))
	if err != nil {
		return result, fmt.Errorf("%s handler assertions: %w", name, err)
	}
	if _, err := runner.run(&result, in.tailwind, "-i", "input.css", "-o", "app.css"); err != nil {
		return result, fmt.Errorf("%s Tailwind build: %w", name, err)
	}
	css, err := os.ReadFile(filepath.Join(work, "app.css"))
	if err != nil {
		return result, err
	}
	for _, selector := range []string{".bg-surface-sunken", ".text-ink", ".border-line", ".min-h-\\[44px\\]", ".overflow-x-auto", ".tabular-nums"} {
		if !hasCSSRule(css, selector) {
			return result, fmt.Errorf("%s generated CSS omitted required selector %s", name, selector)
		}
	}
	if !bytes.Contains(css, []byte("--ak-color-accent:")) {
		return result, fmt.Errorf("%s generated CSS omitted selected theme variables", name)
	}
	if hasCSSRule(css, ".bg-fuchsia-950") {
		return result, fmt.Errorf("%s generated CSS scanned the fixture-only poison selector", name)
	}
	result.BinarySHA256, err = hashFile(filepath.Join(work, "consumer-bin"))
	if err != nil {
		return result, err
	}
	result.CSSSHA256 = hashBytes(css)
	result.HandlerSHA256 = hashBytes(handlerBody)
	return result, nil
}

func decodeCandidateReceipt(body []byte) (candidateReceipt, error) {
	var receipt candidateReceipt
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&receipt); err != nil {
		return candidateReceipt{}, err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return candidateReceipt{}, errors.New("receipt must contain exactly one JSON value")
	}
	return receipt, nil
}

func hasCSSRule(css []byte, selector string) bool {
	return regexp.MustCompile(regexp.QuoteMeta(selector) + `\s*\{`).Match(css)
}

type sandbox struct {
	root, work string
	env        []string
	prefix     []string
	timeout    time.Duration
	retainLogs bool
}

func sandboxRunner(in inputs, root, work string) sandbox {
	proxyURLs := []string{fileURL(in.candidateProxy)}
	for _, proxy := range in.publicProxies {
		proxyURLs = append(proxyURLs, fileURL(proxy))
	}
	profile := `(version 1)
(allow default)
(deny network*)
(deny file-read* (subpath "/Volumes/m2ext/code/geoffmilleraz"))
(deny file-read* (subpath "/Users/geoff/code/geoffmilleraz"))`
	env := []string{
		"PATH=/etc/profiles/per-user/geoff/bin:/usr/bin:/bin:/usr/sbin:/sbin",
		"GOENV=off", "GOWORK=off", "GOTOOLCHAIN=local", "GOVCS=off", "CGO_ENABLED=0",
		"GOPATH=" + filepath.Join(root, "gopath"), "GOMODCACHE=" + filepath.Join(root, "module-cache"),
		"GOCACHE=" + filepath.Join(root, "build-cache"), "GOPROXY=" + strings.Join(proxyURLs, ","),
		"GOSUMDB=sum.golang.org", "GONOSUMDB=" + candidateModule, "TMPDIR=" + root,
	}
	return sandbox{root: root, work: work, env: env, prefix: []string{"/usr/bin/sandbox-exec", "-p", profile, "/usr/bin/env", "-i"}, timeout: 5 * time.Minute}
}

func (s sandbox) run(result *runResult, command string, args ...string) ([]byte, error) {
	full := append(append(append([]string{}, s.prefix...), s.env...), command)
	full = append(full, args...)
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, full[0], full[1:]...)
	cmd.Dir = s.work
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	record := commandRecord{Args: full, StdoutHash: hashBytes(stdout.Bytes()), StderrHash: hashBytes(stderr.Bytes())}
	if s.retainLogs {
		record.StdoutFile = filepath.Join(s.root, fmt.Sprintf("command-%02d.stdout", len(result.Commands)+1))
		record.StderrFile = filepath.Join(s.root, fmt.Sprintf("command-%02d.stderr", len(result.Commands)+1))
		err = errors.Join(err, hostWrite(record.StdoutFile, stdout.Bytes()), hostWrite(record.StderrFile, stderr.Bytes()))
	}
	result.Commands = append(result.Commands, record)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return stdout.Bytes(), fmt.Errorf("%s: exceeded %s", command, s.timeout)
		}
		return stdout.Bytes(), fmt.Errorf("%s: %w: %s", strings.Join(append([]string{command}, args...), " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func (s sandbox) expectRefusal(result *runResult, command string, args ...string) error {
	full := append(append(append([]string{}, s.prefix...), s.env...), command)
	full = append(full, args...)
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, full[0], full[1:]...)
	cmd.Dir = s.work
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result.Commands = append(result.Commands, commandRecord{Args: full, StdoutHash: hashBytes(stdout.Bytes()), StderrHash: hashBytes(stderr.Bytes())})
	if err == nil || !strings.Contains(strings.ToLower(stderr.String()), "operation not permitted") {
		return fmt.Errorf("read was not explicitly refused: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func requireRealDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("must be a real directory, not a symlink")
	}
	return nil
}

func embeddedFixture() (map[string][]byte, string, error) {
	names := []string{"go.mod.txt", "input.css", "main.go"}
	files := make(map[string][]byte, len(names))
	h := sha256.New()
	for _, name := range names {
		body, err := fixtureFS.ReadFile("fixture/" + name)
		if err != nil {
			return nil, "", fmt.Errorf("read embedded fixture %s: %w", name, err)
		}
		files[name] = body
		_, _ = io.WriteString(h, name)
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(body)
		_, _ = h.Write([]byte{0})
	}
	return files, fmt.Sprintf("%x", h.Sum(nil)), nil
}

func moduleFileSum(proxy, version string) (string, error) {
	body, err := os.ReadFile(filepath.Join(proxy, filepath.FromSlash(candidateModule), "@v", version+".mod"))
	if err != nil {
		return "", err
	}
	return dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil })
}

func fileURL(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func hashFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashBytes(body), nil
}

func hashBytes(body []byte) string { return fmt.Sprintf("%x", sha256.Sum256(body)) }

func writeReport(root string, value report) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	tmp := filepath.Join(root, "rehearsal-receipt.json.tmp")
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(root, "rehearsal-receipt.json"))
}
