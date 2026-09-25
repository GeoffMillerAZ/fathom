// Explicit private P03 typed FAQ preparation. It writes only a random private
// evidence directory and never mutates the reference, original archive, or delivery.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
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
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	sourceManifestSHA = "2fb9ae848b8040f8aee05f74a741253592a53a331201baca299dd813dc121f48"
	faqArchive        = "marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675"
	faqMember         = "html/sections/faq-sections/02-centered-accordion.html"
	faqSourceSHA      = "9114b0b981055638eb28fede17eff7bc49c91f40fc32ce758514bfdb7d19f937"
	maxFileBytes      = 8 << 20
)

type fileIdentity struct {
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}
type sourceIdentity struct {
	ArchiveID string `json:"archive_id"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	Size      int    `json:"size"`
}
type sourceCase struct {
	ID           string            `json:"id"`
	Source       sourceIdentity    `json:"source"`
	Documents    map[string]string `json:"documents"`
	SourceFile   string            `json:"source_file"`
	RemoteAssets []string          `json:"remote_assets"`
}
type sourceRuntime struct {
	Package        string `json:"package"`
	Version        string `json:"version"`
	ArchiveSHA256  string `json:"archive_sha256"`
	ElementsSHA256 string `json:"elements_sha256"`
}
type sourceManifest struct {
	SchemaVersion int                     `json:"schema_version"`
	Kind          string                  `json:"kind"`
	Runtime       sourceRuntime           `json:"runtime"`
	Cases         []sourceCase            `json:"cases"`
	Files         map[string]fileIdentity `json:"files"`
	Verification  string                  `json:"verification"`
}
type artifactIdentity struct {
	Adapter      string `json:"adapter"`
	SHA256       string `json:"sha256"`
	SourceSHA256 string `json:"source_sha256"`
	Verification string `json:"verification"`
}
type toolchainIdentity struct {
	Go    string `json:"go"`
	Templ string `json:"templ"`
}
type preparedManifest struct {
	SchemaVersion        int              `json:"schema_version"`
	Kind                 string           `json:"kind"`
	SourceManifestSHA256 string           `json:"source_manifest_sha256"`
	Source               sourceIdentity   `json:"source"`
	Artifact             artifactIdentity `json:"artifact"`
	Runtime              struct {
		SourceReference designconvert.ElementsRuntime  `json:"source_reference"`
		NativeDetails   *designconvert.ElementsRuntime `json:"native_details"`
	} `json:"runtime"`
	Toolchain    toolchainIdentity            `json:"toolchain"`
	Documents    map[string]map[string]string `json:"documents"`
	Files        map[string]fileIdentity      `json:"files"`
	Verification string                       `json:"verification"`
}
type preparationResult struct {
	Directory      string `json:"directory"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Files          int    `json:"files"`
	Verification   string `json:"verification"`
}
type refused string

func (r refused) Error() string { return "FAQ preparation refused: " + string(r) }

func main() {
	kitRoot := flag.String("kit-root", "", "absolute app-kit checkout with verified private archives")
	reference := flag.String("reference", "", "absolute prepared Elements source-reference directory")
	outParent := flag.String("out-parent", "", "absolute existing real 0700 parent")
	flag.Parse()
	if flag.NArg() != 0 || *kitRoot == "" || *reference == "" || *outParent == "" {
		fmt.Fprintln(os.Stderr, "usage: faq-adapter -kit-root ABS -reference ABS -out-parent ABS_EXISTING_0700")
		os.Exit(2)
	}
	result, err := prepare(*kitRoot, *reference, *outParent)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, refused("metadata-output"))
		os.Exit(1)
	}
}

func prepare(kitRoot, reference, outParent string) (preparationResult, error) {
	root, err := validateKitRoot(kitRoot)
	if err != nil {
		return preparationResult{}, err
	}
	sourceManifestValue, sourceManifestBody, item, referenceFiles, err := loadReference(reference)
	if err != nil {
		return preparationResult{}, err
	}
	_ = sourceManifestValue
	parentPath, parent, parentInfo, err := openPrivateParent(outParent)
	if err != nil {
		return preparationResult{}, err
	}
	parent.Close()
	scratch, err := os.MkdirTemp(parentPath, ".faq-adapter-build-")
	if err != nil {
		return preparationResult{}, refused("scratch-create")
	}
	if err := os.Chmod(scratch, 0700); err != nil {
		_ = os.RemoveAll(scratch)
		return preparationResult{}, refused("scratch-mode")
	}
	defer os.RemoveAll(scratch)

	assets := filepath.Join(root, "data", "design-assets")
	original, err := designassets.ReadFile(assets, faqArchive, faqMember)
	if err != nil || original.ArchiveID != faqArchive || original.Path != faqMember || original.SHA256 != faqSourceSHA ||
		len(original.Content) != item.Source.Size || !bytes.Equal([]byte(original.Content), referenceFiles[item.SourceFile]) {
		return preparationResult{}, refused("independent-source-identity")
	}
	source := designconvert.Source{ArchiveID: faqArchive, Path: faqMember, SHA256: faqSourceSHA, Size: len(original.Content)}
	artifact, err := designadapt.AdaptFAQ(source, []byte(original.Content))
	if err != nil || artifact.Verification != "not-verified" || artifact.Adapter != "p03-faq-v1" {
		return preparationResult{}, refused("adapt")
	}
	if err := designadapt.VerifyFAQReplay(artifact, []byte(original.Content)); err != nil {
		return preparationResult{}, refused("replay")
	}

	delivery := filepath.Join(scratch, "delivery")
	if err := designadapt.WriteFAQDelivery(delivery, artifact, []byte(original.Content)); err != nil {
		return preparationResult{}, refused("delivery-write")
	}
	if err := designadapt.CheckFAQDelivery(delivery, artifact, []byte(original.Content)); err != nil {
		return preparationResult{}, refused("delivery-check")
	}
	artifactBody, err := readRegular(filepath.Join(delivery, "artifact.json"), maxFileBytes)
	if err != nil {
		return preparationResult{}, err
	}

	fixtureTemplate, err := readRegular(filepath.Join(root, "docs", "assessments", "fixtures", "faq-adapter", "firstparty_fixture_test.go.txt"), maxFileBytes)
	if err != nil {
		return preparationResult{}, refused("fixture-template")
	}
	defaults := artifact.Defaults
	fixture := string(fixtureTemplate)
	if strings.Count(fixture, "SOURCE_LITERAL") != 1 || strings.Count(fixture, "DEFAULTS_LITERAL") != 1 {
		return preparationResult{}, refused("fixture-placeholders")
	}
	fixture = strings.Replace(fixture, "SOURCE_LITERAL", strconv.Quote(original.Content), 1)
	fixture = strings.Replace(fixture, "DEFAULTS_LITERAL", strconv.Quote(string(defaults)), 1)
	rawReference, rawNative, rawDeps, templVersion, err := compileStandalone(root, delivery, filepath.Join(scratch, "raw"), []byte(fixture), false)
	if err != nil {
		return preparationResult{}, err
	}
	formattedReference, formattedNative, formattedDeps, _, err := compileStandalone(root, delivery, filepath.Join(scratch, "formatted"), []byte(fixture), true)
	if err != nil {
		return preparationResult{}, err
	}
	if !fragmentEqual([]byte(original.Content), rawReference) || !fragmentEqual(rawReference, formattedReference) || !fragmentEqual(rawNative, formattedNative) {
		return preparationResult{}, refused("compiled-dom")
	}
	if strings.Contains(rawDeps, "github.com/geoffmilleraz/app-kit") || strings.Contains(rawDeps, root) || rawDeps != formattedDeps || !strings.Contains(rawDeps, "github.com/a-h/templ") {
		return preparationResult{}, refused("standalone-dependencies")
	}
	if err := designadapt.CheckFAQDelivery(delivery, artifact, []byte(original.Content)); err != nil {
		return preparationResult{}, refused("delivery-mutated")
	}

	files := map[string][]byte{
		"source-manifest.json": slices.Clone(sourceManifestBody), "source.html": []byte(original.Content),
		"reference.html": rawReference, "reference-formatted.html": formattedReference,
		"native.html": rawNative, "native-formatted.html": formattedNative,
		"elements.js": slices.Clone(referenceFiles["elements.js"]), "reference.css": slices.Clone(referenceFiles["reference.css"]),
		"inter.woff2": slices.Clone(referenceFiles["inter.woff2"]), "faq.css": []byte(artifact.CSS),
		"artifact.json": artifactBody, "defaults.json": slices.Clone(defaults),
	}
	for _, name := range []string{"faq.templ", "faq_model.go", "USAGE.md"} {
		body, readErr := readRegular(filepath.Join(delivery, name), maxFileBytes)
		if readErr != nil {
			return preparationResult{}, refused("delivery-inventory")
		}
		files[name] = body
	}
	documents := map[string]map[string]string{}
	for _, lane := range []string{"source", "reference", "reference_formatted", "native", "native_formatted"} {
		documents[lane] = map[string]string{}
	}
	for _, theme := range []string{"light", "dark"} {
		sourceDoc := referenceFiles[item.Documents[theme]]
		sourceName := "source-" + theme + ".html"
		files[sourceName] = slices.Clone(sourceDoc)
		documents["source"][theme] = sourceName
		for _, lane := range []struct{ key, fragment string }{{"reference", "reference.html"}, {"reference_formatted", "reference-formatted.html"}} {
			body, replaceErr := replaceExact(sourceDoc, []byte(original.Content), files[lane.fragment])
			if replaceErr != nil {
				return preparationResult{}, replaceErr
			}
			name := strings.ReplaceAll(lane.key, "_", "-") + "-" + theme + ".html"
			files[name] = body
			documents[lane.key][theme] = name
		}
		for _, lane := range []struct{ key, fragment string }{{"native", "native.html"}, {"native_formatted", "native-formatted.html"}} {
			name := strings.ReplaceAll(lane.key, "_", "-") + "-" + theme + ".html"
			files[name] = nativeDocument(theme, files[lane.fragment])
			documents[lane.key][theme] = name
		}
	}
	m := preparedManifest{SchemaVersion: 1, Kind: "p03-faq-typed-preparation", SourceManifestSHA256: sourceManifestSHA,
		Source: item.Source, Artifact: artifactIdentity{Adapter: artifact.Adapter, SHA256: digest(artifactBody), SourceSHA256: artifact.Source.SHA256, Verification: artifact.Verification},
		Runtime: artifact.Runtime, Toolchain: toolchainIdentity{Go: runtime.Version(), Templ: templVersion}, Documents: documents,
		Files: map[string]fileIdentity{}, Verification: "not-verified"}
	for name, body := range files {
		m.Files[name] = identity(body)
	}
	if err := validatePreparedTopology(m, files, item); err != nil {
		return preparationResult{}, err
	}
	manifestBody, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return preparationResult{}, refused("manifest-encode")
	}
	manifestBody = append(manifestBody, '\n')
	files["manifest.json"] = manifestBody
	directory, err := publishRandomPrivate(parentPath, parentInfo, files)
	if err != nil {
		return preparationResult{}, err
	}
	return preparationResult{Directory: directory, ManifestSHA256: digest(manifestBody), Files: len(files), Verification: "not-verified"}, nil
}

func validatePreparedTopology(m preparedManifest, files map[string][]byte, source sourceCase) error {
	if m.SchemaVersion != 1 || m.Kind != "p03-faq-typed-preparation" || m.Verification != "not-verified" ||
		m.SourceManifestSHA256 != sourceManifestSHA || m.Source != source.Source || m.Source.SHA256 != faqSourceSHA ||
		m.Artifact.Adapter != "p03-faq-v1" || m.Artifact.Verification != "not-verified" || m.Artifact.SourceSHA256 != faqSourceSHA ||
		m.Runtime.SourceReference.Package != designconvert.ElementsRuntimePackage || m.Runtime.SourceReference.Version != designconvert.ElementsRuntimeVersion ||
		m.Runtime.SourceReference.ESMSHA256 != designconvert.ElementsRuntimeESMSHA256 || m.Runtime.SourceReference.TGZSHA256 != designconvert.ElementsRuntimeTGZSHA256 ||
		m.Runtime.SourceReference.Delivery != designconvert.ElementsRuntimeDelivery || m.Runtime.NativeDetails != nil {
		return refused("prepared-topology")
	}
	seen := map[string]bool{}
	for _, lane := range []string{"source", "reference", "reference_formatted", "native", "native_formatted"} {
		pair, ok := m.Documents[lane]
		if !ok || len(pair) != 2 {
			return refused("prepared-documents")
		}
		for _, theme := range []string{"light", "dark"} {
			name := pair[theme]
			if !flatName(name) || len(files[name]) == 0 || seen[name] {
				return refused("prepared-document-binding")
			}
			seen[name] = true
		}
	}
	if len(m.Files) != len(files) {
		return refused("prepared-file-closure")
	}
	for name, body := range files {
		if !flatName(name) || m.Files[name] != identity(body) {
			return refused("prepared-file-identity")
		}
	}
	original := files["source.html"]
	for _, theme := range []string{"light", "dark"} {
		sourceDoc := files[m.Documents["source"][theme]]
		if len(original) == 0 || bytes.Count(sourceDoc, original) != 1 || !bytes.Contains(sourceDoc, []byte("reference.css")) ||
			!bytes.Contains(sourceDoc, []byte("elements.js")) || bytes.Contains(sourceDoc, []byte("faq.css")) {
			return refused("prepared-source-wrapper")
		}
		for _, lane := range []struct{ key, fragment string }{{"reference", "reference.html"}, {"reference_formatted", "reference-formatted.html"}} {
			want, err := replaceExact(sourceDoc, original, files[lane.fragment])
			if err != nil || !bytes.Equal(files[m.Documents[lane.key][theme]], want) {
				return refused("prepared-reference-wrapper")
			}
		}
		for _, lane := range []struct{ key, fragment string }{{"native", "native.html"}, {"native_formatted", "native-formatted.html"}} {
			got, fragment := files[m.Documents[lane.key][theme]], files[lane.fragment]
			if !bytes.Equal(got, nativeDocument(theme, fragment)) || bytes.Contains(got, []byte("<script")) || bytes.Count(got, fragment) != 1 {
				return refused("prepared-native-wrapper")
			}
		}
	}
	return nil
}

func compileStandalone(root, delivery, build string, fixture []byte, format bool) ([]byte, []byte, string, string, error) {
	if err := os.Mkdir(build, 0700); err != nil {
		return nil, nil, "", "", refused("build-directory")
	}
	for _, name := range []string{"faq.templ", "faq_model.go", "faq.css", "defaults.json"} {
		body, err := readRegular(filepath.Join(delivery, name), maxFileBytes)
		if err != nil {
			return nil, nil, "", "", refused("build-input")
		}
		if err := writeExclusive(filepath.Join(build, name), body); err != nil {
			return nil, nil, "", "", refused("build-copy")
		}
	}
	module, err := readRegular(filepath.Join(root, "docs", "assessments", "fixtures", "pricing-adapter", "consumer.mod.txt"), maxFileBytes)
	if err != nil {
		return nil, nil, "", "", err
	}
	module = bytes.Replace(module, []byte("module example.invalid/pricing-proof\n"), []byte("module example.invalid/faq-proof\n"), 1)
	sum, err := readRegular(filepath.Join(root, "docs", "assessments", "fixtures", "pricing-adapter", "consumer.sum.txt"), maxFileBytes)
	if err != nil {
		return nil, nil, "", "", err
	}
	for name, body := range map[string][]byte{"go.mod": module, "go.sum": sum, "fixture_test.go": fixture} {
		if err := writeExclusive(filepath.Join(build, name), body); err != nil {
			return nil, nil, "", "", err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := runCommand(ctx, build, offlineGoEnv(), "go", "mod", "verify"); err != nil {
		return nil, nil, "", "", refused("module-verify")
	}
	if format {
		if err := runCommand(ctx, build, offlineGoEnv(), "go", "tool", "templ", "fmt", "faq.templ"); err != nil {
			return nil, nil, "", "", refused("templ-format")
		}
	}
	version, err := commandOutput(ctx, build, offlineGoEnv(), "go", "tool", "templ", "version")
	if err != nil {
		return nil, nil, "", "", refused("templ-version")
	}
	if strings.TrimSpace(string(version)) != strings.TrimPrefix(designconvert.RequiredTempl, "github.com/a-h/templ@") {
		return nil, nil, "", "", refused("templ-pin")
	}
	if err := runCommand(ctx, build, offlineGoEnv(), "go", "tool", "templ", "generate", "-f", "faq.templ", "-include-version=false"); err != nil {
		return nil, nil, "", "", refused("templ-generate")
	}
	referenceOut, nativeOut := filepath.Join(build, "rendered-reference.html"), filepath.Join(build, "rendered-native.html")
	env := append(offlineGoEnv(), "APPKIT_FAQ_REFERENCE_OUT="+referenceOut, "APPKIT_FAQ_NATIVE_OUT="+nativeOut)
	_, err = commandOutput(ctx, build, env, "go", "test", "-count=1", ".")
	if err != nil {
		return nil, nil, "", "", refused("standalone-test")
	}
	reference, err := readRegular(referenceOut, maxFileBytes)
	if err != nil {
		return nil, nil, "", "", err
	}
	native, err := readRegular(nativeOut, maxFileBytes)
	if err != nil {
		return nil, nil, "", "", err
	}
	deps, err := commandOutput(ctx, build, offlineGoEnv(), "go", "list", "-deps", "-f", "{{.ImportPath}} {{.Dir}}", ".")
	if err != nil {
		return nil, nil, "", "", refused("dependency-list")
	}
	stableDeps, err := standaloneDependencies(string(deps), root, build)
	if err != nil {
		return nil, nil, "", "", err
	}
	return reference, native, stableDeps, strings.TrimSpace(string(version)), nil
}

// Only the generated module's own directory differs between the two builds.
// Preserve every dependency path so an external resolution change still fails replay.
func standaloneDependencies(deps, root, build string) (string, error) {
	if strings.Contains(deps, "github.com/geoffmilleraz/app-kit") || strings.Contains(deps, root) {
		return "", refused("standalone-dependencies")
	}
	lines := strings.Split(strings.TrimSuffix(deps, "\n"), "\n")
	own := 0
	for i, line := range lines {
		pkg, dir, ok := strings.Cut(line, " ")
		if !ok || pkg == "" || !filepath.IsAbs(dir) {
			return "", refused("dependency-record")
		}
		if pkg == "example.invalid/faq-proof" {
			if dir != build {
				return "", refused("generated-module-resolution")
			}
			own++
			lines[i] = pkg + " <generated-module>"
		}
	}
	if own != 1 {
		return "", refused("generated-module-resolution")
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func loadReference(name string) (sourceManifest, []byte, sourceCase, map[string][]byte, error) {
	var m sourceManifest
	if !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return m, nil, sourceCase{}, nil, refused("absolute-reference")
	}
	resolved, err := filepath.EvalSymlinks(name)
	if err != nil {
		return m, nil, sourceCase{}, nil, refused("real-reference")
	}
	info, err := os.Lstat(resolved)
	if err != nil || info.Mode() != os.ModeDir|0700 {
		return m, nil, sourceCase{}, nil, refused("private-reference")
	}
	body, err := readRegular(filepath.Join(resolved, "manifest.json"), 1<<20)
	if err != nil || digest(body) != sourceManifestSHA || json.Unmarshal(body, &m) != nil {
		return sourceManifest{}, nil, sourceCase{}, nil, refused("source-manifest")
	}
	if m.SchemaVersion != 1 || m.Kind != "elements-source-reference" || m.Verification != "not-verified" ||
		m.Runtime.Package != designconvert.ElementsRuntimePackage || m.Runtime.Version != designconvert.ElementsRuntimeVersion ||
		m.Runtime.ArchiveSHA256 != designconvert.ElementsRuntimeTGZSHA256 || m.Runtime.ElementsSHA256 != designconvert.ElementsRuntimeESMSHA256 || len(m.Files) != 18 {
		return sourceManifest{}, nil, sourceCase{}, nil, refused("source-manifest-contract")
	}
	all := map[string][]byte{}
	for file, want := range m.Files {
		if !flatName(file) || want.Size < 0 || want.Size > maxFileBytes {
			return sourceManifest{}, nil, sourceCase{}, nil, refused("reference-file-contract")
		}
		got, readErr := readRegular(filepath.Join(resolved, file), maxFileBytes)
		if readErr != nil || identity(got) != want {
			return sourceManifest{}, nil, sourceCase{}, nil, refused("reference-file-integrity")
		}
		all[file] = got
	}
	var found []sourceCase
	for _, item := range m.Cases {
		if item.ID == "P03-faq" {
			found = append(found, item)
		}
	}
	if len(found) != 1 || found[0].Source.ArchiveID != faqArchive || found[0].Source.Path != faqMember || found[0].Source.SHA256 != faqSourceSHA ||
		!flatName(found[0].SourceFile) || len(found[0].Documents) != 2 || len(found[0].RemoteAssets) != 0 {
		return sourceManifest{}, nil, sourceCase{}, nil, refused("p03-binding")
	}
	for _, required := range []string{found[0].SourceFile, found[0].Documents["light"], found[0].Documents["dark"], "elements.js", "reference.css", "inter.woff2"} {
		if len(all[required]) == 0 {
			return sourceManifest{}, nil, sourceCase{}, nil, refused("p03-file")
		}
	}
	return m, body, found[0], all, nil
}

func validateKitRoot(root string) (string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return "", refused("absolute-kit-root")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", refused("real-kit-root")
	}
	mod, err := readRegular(filepath.Join(resolved, "go.mod"), 64<<10)
	if err != nil || !bytes.HasPrefix(mod, []byte("module github.com/geoffmilleraz/app-kit\n")) {
		return "", refused("kit-module")
	}
	return resolved, nil
}

func nativeDocument(theme string, fragment []byte) []byte {
	return []byte(`<!doctype html><html lang="en" class="` + theme + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Private FAQ native review</title><link rel="stylesheet" href="reference.css"><link rel="stylesheet" href="faq.css"></head><body><main id="specimen">` + string(fragment) + `</main></body></html>`)
}

func replaceExact(document, original, replacement []byte) ([]byte, error) {
	if len(original) == 0 || bytes.Count(document, original) != 1 {
		return nil, refused("wrapper-fragment-cardinality")
	}
	return bytes.Replace(document, original, replacement, 1), nil
}

type nodeShape struct {
	Type            html.NodeType
	Data, Namespace string
	Attr            []html.Attribute
	Children        []nodeShape
}

func fragmentShape(body []byte) []nodeShape {
	nodes, err := html.ParseFragment(bytes.NewReader(body), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return nil
	}
	var shape func(*html.Node) nodeShape
	shape = func(n *html.Node) nodeShape {
		out := nodeShape{Type: n.Type, Data: n.Data, Namespace: n.Namespace, Attr: slices.Clone(n.Attr)}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			out.Children = append(out.Children, shape(c))
		}
		return out
	}
	out := make([]nodeShape, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, shape(n))
	}
	return out
}
func fragmentEqual(a, b []byte) bool    { return reflect.DeepEqual(fragmentShape(a), fragmentShape(b)) }
func digest(body []byte) string         { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
func identity(body []byte) fileIdentity { return fileIdentity{SHA256: digest(body), Size: len(body)} }
func flatName(name string) bool {
	return name != "" && filepath.Base(name) == name && name != "." && name != ".." && !strings.ContainsAny(name, "\x00/\\")
}

func offlineGoEnv() []string {
	env := []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "GOPROXY" && key != "GOSUMDB" && key != "GOTOOLCHAIN" && key != "GOFLAGS" && key != "GOWORK" && key != "APPKIT_FAQ_REFERENCE_OUT" && key != "APPKIT_FAQ_NATIVE_OUT" {
			env = append(env, item)
		}
	}
	return append(env, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
}
func runCommand(ctx context.Context, dir string, env []string, name string, args ...string) error {
	_, err := commandOutput(ctx, dir, env, name, args...)
	return err
}
func commandOutput(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.Bytes(), err
}

func readRegular(path string, limit int) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 0 || before.Size() > int64(limit) {
		return nil, refused("regular-file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, refused("regular-open")
	}
	after, statErr := f.Stat()
	body, readErr := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	closeErr := f.Close()
	if statErr != nil || !os.SameFile(before, after) || readErr != nil || closeErr != nil || int64(len(body)) != before.Size() {
		return nil, refused("regular-read")
	}
	return body, nil
}
func writeExclusive(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, w := f.Write(body)
	s := f.Sync()
	c := f.Close()
	return errors.Join(w, s, c)
}
func openPrivateParent(name string) (string, *os.Root, os.FileInfo, error) {
	if !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return "", nil, nil, refused("absolute-output-parent")
	}
	resolved, err := filepath.EvalSymlinks(name)
	if err != nil {
		return "", nil, nil, refused("real-output-parent")
	}
	info, err := os.Lstat(resolved)
	if err != nil || info.Mode() != os.ModeDir|0700 {
		return "", nil, nil, refused("private-output-parent")
	}
	root, err := os.OpenRoot(resolved)
	if err != nil {
		return "", nil, nil, refused("output-parent-open")
	}
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		root.Close()
		return "", nil, nil, refused("output-parent-changed")
	}
	return resolved, root, info, nil
}
func publishRandomPrivate(parentPath string, expected os.FileInfo, files map[string][]byte) (string, error) {
	_, parent, current, err := openPrivateParent(parentPath)
	if err != nil {
		return "", err
	}
	defer parent.Close()
	if !os.SameFile(expected, current) {
		return "", refused("output-parent-changed")
	}
	if _, ok := files["manifest.json"]; !ok {
		return "", refused("publication-manifest")
	}
	names := make([]string, 0, len(files))
	for name, body := range files {
		if !flatName(name) || len(body) > maxFileBytes {
			return "", refused("publication-file-contract")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	names = slices.DeleteFunc(names, func(n string) bool { return n == "manifest.json" })
	names = append(names, "manifest.json")
	var leaf string
	for range 8 {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", refused("publication-random")
		}
		leaf = "faq-adapter-" + hex.EncodeToString(b)
		if err := parent.Mkdir(leaf, 0700); err == nil {
			break
		} else if !errors.Is(err, os.ErrExist) {
			return "", refused("publication-create")
		}
		leaf = ""
	}
	if leaf == "" {
		return "", refused("publication-collision")
	}
	created, err := parent.Lstat(leaf)
	if err != nil || created.Mode() != os.ModeDir|0700 {
		return "", refused("publication-directory")
	}
	destination, err := parent.OpenRoot(leaf)
	if err != nil {
		return "", refused("publication-open")
	}
	defer destination.Close()
	actual, err := destination.Stat(".")
	if err != nil || !os.SameFile(created, actual) {
		return "", refused("publication-changed")
	}
	for _, name := range names {
		f, err := destination.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return "", refused("publication-write")
		}
		_, w := f.Write(files[name])
		s := f.Sync()
		c := f.Close()
		if errors.Join(w, s, c) != nil {
			return "", refused("publication-write")
		}
	}
	listing, err := destination.Open(".")
	if err != nil {
		return "", refused("publication-closure")
	}
	entries, readErr := listing.ReadDir(len(files) + 1)
	closeErr := listing.Close()
	if readErr != nil && readErr != io.EOF || closeErr != nil || len(entries) != len(files) {
		return "", refused("publication-closure")
	}
	for _, entry := range entries {
		want, ok := files[entry.Name()]
		if !ok || entry.Type()&os.ModeSymlink != 0 {
			return "", refused("publication-entry")
		}
		got, err := readRegular(filepath.Join(parentPath, leaf, entry.Name()), maxFileBytes)
		if err != nil || !bytes.Equal(got, want) {
			return "", refused("publication-integrity")
		}
	}
	return filepath.Join(parentPath, leaf), nil
}
