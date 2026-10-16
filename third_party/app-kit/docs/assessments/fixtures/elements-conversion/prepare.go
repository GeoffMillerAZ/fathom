// Explicit private P03/P07/P08/P09 original/compiled preparation. This is an
// assessment fixture, not a public converter API or a compatibility claim.
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

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	sourceManifestSHA = "2fb9ae848b8040f8aee05f74a741253592a53a331201baca299dd813dc121f48"
	maxManifestBytes  = 1 << 20
	maxArtifactBytes  = 8 << 20
	caseCount         = 4
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

type sourceManifest struct {
	SchemaVersion int                     `json:"schema_version"`
	Kind          string                  `json:"kind"`
	Runtime       sourceRuntime           `json:"runtime"`
	Cases         []sourceCase            `json:"cases"`
	Files         map[string]fileIdentity `json:"files"`
	Verification  string                  `json:"verification"`
}

type sourceRuntime struct {
	Package        string `json:"package"`
	Version        string `json:"version"`
	ArchiveSHA256  string `json:"archive_sha256"`
	ElementsSHA256 string `json:"elements_sha256"`
}

type pairCase struct {
	ID                string            `json:"id"`
	Source            sourceIdentity    `json:"source"`
	Receipt           string            `json:"receipt"`
	EmittedTempl      string            `json:"emitted_templ"`
	FormattedTempl    string            `json:"formatted_templ"`
	CompiledHTML      string            `json:"compiled_html"`
	FormattedHTML     string            `json:"formatted_html"`
	Fragments         map[string]string `json:"fragments"`
	SourceDocuments   map[string]string `json:"source_documents"`
	CompiledDocuments map[string]string `json:"compiled_documents"`
	RemoteAssets      []string          `json:"remote_assets"`
}

type pairManifest struct {
	SchemaVersion        int                           `json:"schema_version"`
	Kind                 string                        `json:"kind"`
	SourceManifestSHA256 string                        `json:"source_manifest_sha256"`
	Runtime              designconvert.ElementsRuntime `json:"runtime"`
	Toolchain            pairToolchain                 `json:"toolchain"`
	Cases                []pairCase                    `json:"cases"`
	Files                map[string]fileIdentity       `json:"files"`
	Verification         string                        `json:"verification"`
}

type pairToolchain struct {
	GoVersion     string `json:"go_version"`
	RequiredTempl string `json:"required_templ"`
}

type preparationResult struct {
	Directory      string `json:"directory"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Cases          int    `json:"cases"`
	Files          int    `json:"files"`
	Verification   string `json:"verification"`
}

type refused string

func (r refused) Error() string { return "elements conversion pair refused: " + string(r) }

func main() {
	kitRoot := flag.String("kit-root", "", "absolute app-kit checkout containing the verified private archives")
	reference := flag.String("reference", "", "absolute prepared Elements source-reference directory")
	outParent := flag.String("out-parent", "", "absolute existing real 0700 parent for a random new private directory")
	flag.Parse()
	if flag.NArg() != 0 || *kitRoot == "" || *reference == "" || *outParent == "" {
		fmt.Fprintln(os.Stderr, "usage: elements-conversion -kit-root ABS -reference ABS -out-parent ABS_EXISTING_0700")
		os.Exit(2)
	}
	result, err := prepare(*kitRoot, *reference, *outParent)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "elements conversion pair: metadata output failed")
		os.Exit(1)
	}
}

func prepare(kitRoot, reference, outParent string) (preparationResult, error) {
	kitRoot, err := validateKitRoot(kitRoot)
	if err != nil {
		return preparationResult{}, err
	}
	manifest, sourceManifestBody, referenceFiles, err := loadReference(reference)
	if err != nil {
		return preparationResult{}, err
	}
	if err := checkToolchain(kitRoot); err != nil {
		return preparationResult{}, err
	}
	parentPath, parent, parentInfo, err := openPrivateParent(outParent)
	if err != nil {
		return preparationResult{}, err
	}
	parent.Close()
	scratch, err := os.MkdirTemp(parentPath, ".elements-conversion-compile-")
	if err != nil {
		return preparationResult{}, refused("scratch-create")
	}
	if err := os.Chmod(scratch, 0700); err != nil {
		_ = os.RemoveAll(scratch)
		return preparationResult{}, refused("scratch-mode")
	}
	defer os.RemoveAll(scratch)

	files := make(map[string][]byte, len(referenceFiles)+30)
	for name, body := range referenceFiles {
		files[name] = slices.Clone(body)
	}
	files["source-manifest.json"] = slices.Clone(sourceManifestBody)
	pair := pairManifest{
		SchemaVersion: 1, Kind: "elements-original-compiled-pair", SourceManifestSHA256: sourceManifestSHA,
		Runtime: designconvert.ElementsRuntime{
			Package: designconvert.ElementsRuntimePackage, Version: designconvert.ElementsRuntimeVersion,
			ESMSHA256: designconvert.ElementsRuntimeESMSHA256, TGZSHA256: designconvert.ElementsRuntimeTGZSHA256,
			Delivery: designconvert.ElementsRuntimeDelivery,
		},
		Toolchain: pairToolchain{GoVersion: runtime.Version(), RequiredTempl: designconvert.RequiredTempl},
		Cases:     []pairCase{}, Files: map[string]fileIdentity{}, Verification: "not-verified",
	}

	for _, item := range manifest.Cases {
		original := referenceFiles[item.SourceFile]
		if len(original) == 0 || digest(original) != item.Source.SHA256 || len(original) != item.Source.Size {
			return preparationResult{}, refused("source-file-identity-" + item.ID)
		}
		actual, err := designassets.ReadFile(filepath.Join(kitRoot, "data", "design-assets"), item.Source.ArchiveID, item.Source.Path)
		if err != nil || actual.ArchiveID != item.Source.ArchiveID || actual.Path != item.Source.Path ||
			actual.SHA256 != item.Source.SHA256 || len(actual.Content) != item.Source.Size || !bytes.Equal([]byte(actual.Content), original) {
			return preparationResult{}, refused("archive-source-identity-" + item.ID)
		}
		source := designconvert.Source{ArchiveID: item.Source.ArchiveID, Path: item.Source.Path, SHA256: item.Source.SHA256, Size: item.Source.Size}
		translation, err := designconvert.TranslateElements(source, original)
		if err != nil || translation.Status != "translated" || translation.Artifact == nil || translation.Verification != "not-verified" {
			return preparationResult{}, refused("translation-" + item.ID)
		}
		if err := designconvert.VerifyElementsReplay(translation, original); err != nil {
			return preparationResult{}, refused("translation-replay-" + item.ID)
		}
		receipt, err := json.MarshalIndent(translation, "", "  ")
		if err != nil {
			return preparationResult{}, refused("receipt-encoding")
		}
		receipt = append(receipt, '\n')
		compiled, formatted, formattedTempl, err := compilePair(kitRoot, filepath.Join(scratch, item.ID), []byte(translation.Artifact.Content), original)
		if err != nil {
			return preparationResult{}, fmt.Errorf("elements conversion pair: private compile for %s failed (%w); compiler output withheld", item.ID, err)
		}
		if !fragmentEqual(original, compiled) || !fragmentEqual(original, formatted) || !fragmentEqual(compiled, formatted) {
			return preparationResult{}, refused("dom-equality-" + item.ID)
		}

		slug := strings.ToLower(strings.TrimPrefix(item.ID, "P"))
		slug = strings.ReplaceAll(slug, "-", "_")
		receiptName := slug + "-receipt.json"
		emittedName := slug + "-emitted.templ"
		formattedTemplName := slug + "-formatted.templ"
		compiledName := slug + "-compiled.html"
		formattedName := slug + "-formatted.html"
		files[receiptName] = receipt
		files[emittedName] = []byte(translation.Artifact.Content)
		files[formattedTemplName] = formattedTempl
		files[compiledName] = compiled
		files[formattedName] = formatted

		compiledDocuments := map[string]string{}
		for _, theme := range []string{"light", "dark"} {
			sourceDocumentName, ok := item.Documents[theme]
			if !ok {
				return preparationResult{}, refused("source-document-" + item.ID)
			}
			sourceDocument := referenceFiles[sourceDocumentName]
			wrapper, err := replaceExactFragment(sourceDocument, original, formatted)
			if err != nil {
				return preparationResult{}, refused("compiled-wrapper-" + item.ID + "-" + theme)
			}
			name := strings.TrimSuffix(sourceDocumentName, ".html") + "-compiled.html"
			files[name] = wrapper
			compiledDocuments[theme] = name
		}
		pair.Cases = append(pair.Cases, pairCase{
			ID: item.ID, Source: item.Source, Receipt: receiptName, EmittedTempl: emittedName,
			FormattedTempl: formattedTemplName, CompiledHTML: compiledName, FormattedHTML: formattedName,
			Fragments:       map[string]string{"source": item.SourceFile, "compiled": formattedName},
			SourceDocuments: mapsClone(item.Documents), CompiledDocuments: compiledDocuments,
			RemoteAssets: slices.Clone(item.RemoteAssets),
		})
	}
	if len(pair.Cases) != caseCount {
		return preparationResult{}, refused("pair-case-closure")
	}
	for name, body := range files {
		pair.Files[name] = identity(body)
	}
	manifestBody, err := json.MarshalIndent(pair, "", "  ")
	if err != nil {
		return preparationResult{}, refused("pair-manifest-encoding")
	}
	manifestBody = append(manifestBody, '\n')
	files["manifest.json"] = manifestBody
	directory, err := publishRandomPrivate(parentPath, parentInfo, files, nil)
	if err != nil {
		return preparationResult{}, err
	}
	return preparationResult{Directory: directory, ManifestSHA256: digest(manifestBody), Cases: len(pair.Cases), Files: len(files), Verification: "not-verified"}, nil
}

func mapsClone(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
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

func loadReference(name string) (sourceManifest, []byte, map[string][]byte, error) {
	var manifest sourceManifest
	if !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return manifest, nil, nil, refused("absolute-reference")
	}
	resolved, err := filepath.EvalSymlinks(name)
	if err != nil {
		return manifest, nil, nil, refused("real-reference")
	}
	name = resolved
	info, err := os.Lstat(name)
	if err != nil || info.Mode() != os.ModeDir|0700 {
		return manifest, nil, nil, refused("private-reference")
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return manifest, nil, nil, refused("reference-open")
	}
	defer root.Close()
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		return manifest, nil, nil, refused("reference-changed")
	}
	body, err := readRootRegular(root, "manifest.json", maxManifestBytes)
	if err != nil || digest(body) != sourceManifestSHA || json.Unmarshal(body, &manifest) != nil {
		return sourceManifest{}, nil, nil, refused("source-manifest")
	}
	if manifest.SchemaVersion != 1 || manifest.Kind != "elements-source-reference" || manifest.Verification != "not-verified" ||
		manifest.Runtime.Package != designconvert.ElementsRuntimePackage || manifest.Runtime.Version != designconvert.ElementsRuntimeVersion ||
		manifest.Runtime.ArchiveSHA256 != designconvert.ElementsRuntimeTGZSHA256 || manifest.Runtime.ElementsSHA256 != designconvert.ElementsRuntimeESMSHA256 ||
		len(manifest.Cases) != caseCount || len(manifest.Files) != 18 {
		return sourceManifest{}, nil, nil, refused("source-manifest-contract")
	}
	files := make(map[string][]byte, len(manifest.Files))
	for filename, want := range manifest.Files {
		if !flatName(filename) || want.Size < 0 || want.Size > maxArtifactBytes {
			return sourceManifest{}, nil, nil, refused("source-file-contract")
		}
		body, err := readRootRegular(root, filename, maxArtifactBytes)
		if err != nil || identity(body) != want {
			return sourceManifest{}, nil, nil, refused("source-file-integrity")
		}
		files[filename] = body
	}
	wantIDs := map[string]bool{"P03-faq": true, "P07-menu": true, "P08-nested-dialog-menu": true, "P09-mobile-nav": true}
	seen := map[string]bool{}
	for _, item := range manifest.Cases {
		if !wantIDs[item.ID] || seen[item.ID] || !flatName(item.SourceFile) || len(item.Documents) != 2 || item.Source.Size <= 0 || item.Source.Size > designconvert.MaxSourceBytes {
			return sourceManifest{}, nil, nil, refused("source-case-contract")
		}
		seen[item.ID] = true
		for _, theme := range []string{"light", "dark"} {
			if !flatName(item.Documents[theme]) {
				return sourceManifest{}, nil, nil, refused("source-document-contract")
			}
		}
	}
	return manifest, body, files, nil
}

func checkToolchain(root string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "tool", "templ", "--version")
	cmd.Dir, cmd.Env = root, offlineGoEnv()
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != strings.TrimPrefix(designconvert.RequiredTempl, "github.com/a-h/templ@") {
		return refused("templ-pin")
	}
	return nil
}

func compilePair(root, directory string, emitted, original []byte) ([]byte, []byte, []byte, error) {
	if err := os.Mkdir(directory, 0700); err != nil {
		return nil, nil, nil, refused("compile-directory")
	}
	compile := func(label string, source []byte, format bool) ([]byte, []byte, error) {
		dir := filepath.Join(directory, label)
		if err := os.Mkdir(dir, 0700); err != nil {
			return nil, nil, refused("compile-step-directory")
		}
		templPath := filepath.Join(dir, "snippet.templ")
		if err := writeExclusive(templPath, source); err != nil {
			return nil, nil, refused("compile-templ-write")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if format {
			if err := runPrivate(ctx, root, dir, "format", "go", "tool", "templ", "fmt", templPath); err != nil {
				return nil, nil, err
			}
		}
		formattedSource, err := readRegular(templPath, maxArtifactBytes)
		if err != nil {
			return nil, nil, refused("compiled-templ-read")
		}
		if err := runPrivate(ctx, root, dir, "generate", "go", "tool", "templ", "generate", "-f", templPath, "-include-version=false"); err != nil {
			return nil, nil, err
		}
		renderPath := filepath.Join(dir, "rendered.html")
		fixture := strings.ReplaceAll(renderFixture, "ORIGINAL_LITERAL", strconv.Quote(string(original)))
		if err := writeExclusive(filepath.Join(dir, "render_test.go"), []byte(fixture)); err != nil {
			return nil, nil, refused("compile-fixture-write")
		}
		env := append(offlineGoEnv(), "APPKIT_ELEMENTS_RENDER_OUT="+renderPath)
		if err := runPrivateEnv(ctx, root, dir, env, "test", "go", "test", "-count=1", filepath.Join(dir, "snippet_templ.go"), filepath.Join(dir, "render_test.go")); err != nil {
			return nil, nil, err
		}
		rendered, err := readRegular(renderPath, maxArtifactBytes)
		if err != nil {
			return nil, nil, refused("compiled-html-read")
		}
		return rendered, formattedSource, nil
	}
	before, _, err := compile("before", emitted, false)
	if err != nil {
		return nil, nil, nil, err
	}
	after, formattedTempl, err := compile("after", emitted, true)
	if err != nil {
		return nil, nil, nil, err
	}
	return before, after, formattedTempl, nil
}

func runPrivate(ctx context.Context, root, logDir, step, name string, args ...string) error {
	return runPrivateEnv(ctx, root, logDir, offlineGoEnv(), step, name, args...)
}

func runPrivateEnv(ctx context.Context, root, logDir string, env []string, step, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = root, env
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Run(); err != nil {
		_ = writeExclusive(filepath.Join(logDir, "compiler-"+step+".log"), log.Bytes())
		return refused("compiler-" + step)
	}
	return nil
}

func offlineGoEnv() []string {
	env := []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "GOPROXY" && key != "GOSUMDB" && key != "GOTOOLCHAIN" && key != "GOFLAGS" && key != "GOWORK" && key != "APPKIT_ELEMENTS_RENDER_OUT" {
			env = append(env, item)
		}
	}
	return append(env, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOFLAGS=-mod=readonly", "GOWORK=off")
}

func replaceExactFragment(document, original, replacement []byte) ([]byte, error) {
	if len(original) == 0 || bytes.Count(document, original) != 1 {
		return nil, refused("wrapper-fragment-cardinality")
	}
	out := bytes.Replace(document, original, replacement, 1)
	if bytes.Count(out, replacement) == 0 {
		return nil, refused("wrapper-replacement")
	}
	return out, nil
}

type nodeShape struct {
	Type      html.NodeType
	Data      string
	Namespace string
	Attr      []html.Attribute
	Children  []nodeShape
}

func fragmentEqual(a, b []byte) bool { return reflect.DeepEqual(fragmentShape(a), fragmentShape(b)) }

func fragmentShape(body []byte) []nodeShape {
	nodes, err := html.ParseFragment(bytes.NewReader(body), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return nil
	}
	var shape func(*html.Node) nodeShape
	shape = func(n *html.Node) nodeShape {
		out := nodeShape{Type: n.Type, Data: n.Data, Namespace: n.Namespace, Attr: slices.Clone(n.Attr)}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			out.Children = append(out.Children, shape(child))
		}
		return out
	}
	out := make([]nodeShape, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, shape(node))
	}
	return out
}

func identity(body []byte) fileIdentity { return fileIdentity{SHA256: digest(body), Size: len(body)} }

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func flatName(name string) bool {
	return name != "" && filepath.Base(name) == name && name != "." && name != ".." && !strings.ContainsAny(name, "\x00/\\")
}

func readRegular(path string, limit int) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 0 || before.Size() > int64(limit) {
		return nil, refused("regular-file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, refused("regular-open")
	}
	after, statErr := file.Stat()
	body, readErr := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	closeErr := file.Close()
	if statErr != nil || !os.SameFile(before, after) || readErr != nil || closeErr != nil || int64(len(body)) != before.Size() {
		return nil, refused("regular-read")
	}
	return body, nil
}

func writeExclusive(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(body)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return refused("private-write")
	}
	return nil
}

func openPrivateParent(name string) (string, *os.Root, os.FileInfo, error) {
	if !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return "", nil, nil, refused("absolute-output-parent")
	}
	resolved, err := filepath.EvalSymlinks(name)
	if err != nil {
		return "", nil, nil, refused("real-output-parent")
	}
	name = resolved
	info, err := os.Lstat(name)
	if err != nil || info.Mode() != os.ModeDir|0700 {
		return "", nil, nil, refused("private-output-parent")
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return "", nil, nil, refused("output-parent-open")
	}
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(info, actual) {
		root.Close()
		return "", nil, nil, refused("output-parent-changed")
	}
	return name, root, info, nil
}

type publicationHook func(parent *os.Root, leaf string) error

func publishRandomPrivate(parentPath string, expectedParent os.FileInfo, files map[string][]byte, hook publicationHook) (string, error) {
	_, parent, currentParent, err := openPrivateParent(parentPath)
	if err != nil {
		return "", err
	}
	defer parent.Close()
	if !os.SameFile(expectedParent, currentParent) {
		return "", refused("output-parent-changed")
	}
	if _, ok := files["manifest.json"]; !ok {
		return "", refused("publication-manifest")
	}
	names := make([]string, 0, len(files))
	for name, body := range files {
		if !flatName(name) || len(body) > maxArtifactBytes {
			return "", refused("publication-file-contract")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	names = slices.DeleteFunc(names, func(name string) bool { return name == "manifest.json" })
	names = append(names, "manifest.json")
	var leaf string
	for range 8 {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return "", refused("publication-random")
		}
		leaf = "elements-conversion-" + hex.EncodeToString(random)
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
		return "", refused("publication-created-directory")
	}
	if hook != nil {
		if err := hook(parent, leaf); err != nil {
			return "", refused("publication-test-hook")
		}
	}
	destination, err := openSameRoot(parent, leaf, created)
	if err != nil {
		return "", err
	}
	defer destination.Close()
	// All writes stay on the retained directory handle. The manifest is the
	// completion marker and is written last. A failure can leave an explicit
	// incomplete 0700 directory. This detects checked-boundary replacement; it
	// does not claim protection against a malicious process with the same UID.
	for _, name := range names {
		if err := writeRootExclusive(destination, name, files[name]); err != nil {
			return "", refused("publication-write")
		}
	}
	if err := verifyRootFiles(destination, files); err != nil {
		return "", err
	}
	published, err := parent.Lstat(leaf)
	if err != nil || !os.SameFile(created, published) {
		return "", refused("publication-destination-changed")
	}
	return filepath.Join(parentPath, leaf), nil
}

func openSameRoot(parent *os.Root, name string, expected os.FileInfo) (*os.Root, error) {
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, refused("publication-open")
	}
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(expected, actual) || actual.Mode() != os.ModeDir|0700 {
		root.Close()
		return nil, refused("publication-destination-changed")
	}
	return root, nil
}

func writeRootExclusive(root *os.Root, name string, body []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(body)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return refused("private-root-write")
	}
	return nil
}

func readRootRegular(root *os.Root, name string, limit int) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Mode() != 0600 || before.Size() < 0 || before.Size() > int64(limit) {
		return nil, refused("root-regular-file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, refused("root-regular-open")
	}
	after, statErr := file.Stat()
	body, readErr := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	closeErr := file.Close()
	if statErr != nil || !os.SameFile(before, after) || after.Mode() != 0600 || readErr != nil || closeErr != nil || int64(len(body)) != before.Size() {
		return nil, refused("root-regular-read")
	}
	return body, nil
}

func verifyRootFiles(root *os.Root, files map[string][]byte) error {
	directory, err := root.Open(".")
	if err != nil {
		return refused("output-closure")
	}
	entries, readErr := directory.ReadDir(len(files) + 1)
	closeErr := directory.Close()
	if readErr != nil && readErr != io.EOF || closeErr != nil || len(entries) != len(files) {
		return refused("output-closure")
	}
	for _, entry := range entries {
		want, ok := files[entry.Name()]
		if !ok || entry.Type()&os.ModeSymlink != 0 {
			return refused("output-entry")
		}
		got, err := readRootRegular(root, entry.Name(), maxArtifactBytes)
		if err != nil || !bytes.Equal(got, want) {
			return refused("output-integrity")
		}
	}
	return nil
}

const renderFixture = `package converted
import (
 "bytes"
 "context"
 "os"
 "reflect"
 "strings"
 "testing"
 "golang.org/x/net/html"
 "golang.org/x/net/html/atom"
)
type nodeShape struct { Type html.NodeType; Data, Namespace string; Attr []html.Attribute; Children []nodeShape }
func shape(n *html.Node) nodeShape {
 s := nodeShape{Type:n.Type, Data:n.Data, Namespace:n.Namespace, Attr:n.Attr}
 for c := n.FirstChild; c != nil; c = c.NextSibling { s.Children = append(s.Children, shape(c)) }
 return s
}
func fragment(t *testing.T, s string) []nodeShape {
 t.Helper()
 nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{Type:html.ElementNode, Data:"div", DataAtom:atom.Div})
 if err != nil { t.Fatal("parse failed") }
 result := []nodeShape{}
 for _, n := range nodes { result = append(result, shape(n)) }
 return result
}
func TestOriginalRenderedDOM(t *testing.T) {
 const source = ORIGINAL_LITERAL
 var out bytes.Buffer
 if err := Specimen().Render(context.Background(), &out); err != nil { t.Fatal("render failed") }
 if !reflect.DeepEqual(fragment(t, source), fragment(t, out.String())) { t.Fatal("namespace-aware DOM/text/attribute/comment mismatch") }
 name := os.Getenv("APPKIT_ELEMENTS_RENDER_OUT")
 f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
 if err != nil { t.Fatal("private render output") }
 if _, err = f.Write(out.Bytes()); err != nil { _ = f.Close(); t.Fatal("private render write") }
 if err = f.Sync(); err != nil { _ = f.Close(); t.Fatal("private render sync") }
 if err = f.Close(); err != nil { t.Fatal("private render close") }
}
`
