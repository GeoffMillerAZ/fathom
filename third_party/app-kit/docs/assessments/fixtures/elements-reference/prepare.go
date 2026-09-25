// Explicit private P03/P07/P08/P09 source-native preparation. This is an
// assessment fixture, not a converter, a public API, or a compatibility claim.
//
// Run from app-kit with explicit absolute inputs and a new output path:
//
//	go run ./docs/assessments/fixtures/elements-reference -kit-root /abs/app-kit -runtime-tgz /abs/tailwindplus-elements-1.0.22.tgz -out /abs/new-output
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/tailwind"
	"golang.org/x/net/html"
)

const (
	elementsVersion    = "1.0.22"
	elementsArchiveSHA = "fc684f17d960fb60dadfdf9a0b49ba8fd295af4b8fd1715a26cd20b832ea245b"
	elementsRuntimeSHA = "120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013"
	fontManifestSHA    = "dc7298020ee8a2e2502e7e038e8941c98123a03db4ffc8f52b6ff7fdc41f4655"
	fontRoot           = "data/design-assets/reference-assets/inter/46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441"
	fontPath           = "font-files/InterVariable.woff2"
	maxDocumentBytes   = 1 << 20
	maxArchiveBytes    = 4 << 20
	maxRuntimeBytes    = 1 << 20
	maxLicenseBytes    = 256 << 10
	maxTarEntries      = 256
	marketingArchive   = "marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675"
	applicationArchive = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"
)

var caseSpecs = []caseSpec{
	{ID: "P03-faq", Slug: "p03-faq", ArchiveID: marketingArchive, Path: "html/sections/faq-sections/02-centered-accordion.html", SHA256: "9114b0b981055638eb28fede17eff7bc49c91f40fc32ce758514bfdb7d19f937"},
	{ID: "P07-menu", Slug: "p07-menu", ArchiveID: applicationArchive, Path: "html/elements/dropdowns/01-simple.html", SHA256: "10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a"},
	{ID: "P08-nested-dialog-menu", Slug: "p08-nested-dialog-menu", ArchiveID: applicationArchive, Path: "html/overlays/drawers/07-user-profile-example.html", SHA256: "1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415"},
	{ID: "P09-mobile-nav", Slug: "p09-mobile-nav", ArchiveID: applicationArchive, Path: "html/navigation/navbars/04-simple-with-menu-button-on-left.html", SHA256: "79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7"},
}

var cssInput = `@import "tailwindcss" source(none);
@source "./p03-faq-original.html";
@source "./p07-menu-original.html";
@source "./p08-nested-dialog-menu-original.html";
@source "./p09-mobile-nav-original.html";
@custom-variant dark (&:where(.dark, .dark *));
@font-face { font-family: "InterVariable"; font-style: normal; font-weight: 100 900; font-display: block; src: url("./inter.woff2") format("woff2"); }
@theme { --font-sans: "InterVariable", sans-serif; }
:root { color-scheme: light; background: #fff; }
:root.dark { color-scheme: dark; background: #111827; }
body { box-sizing: border-box; margin: 0; min-height: 100vh; padding: 2rem; background: #fff; font-family: "InterVariable", sans-serif; }
.dark body { background: #111827; }
#specimen { min-width: 0; }
@media (max-width: 640px) { body { padding: 1rem; } }
`

type caseSpec struct {
	ID, Slug, ArchiveID, Path, SHA256 string
}

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

type caseManifest struct {
	ID           string            `json:"id"`
	Source       sourceIdentity    `json:"source"`
	Documents    map[string]string `json:"documents"`
	SourceFile   string            `json:"source_file"`
	RemoteAssets []string          `json:"remote_assets"`
}

type runtimeManifest struct {
	Package        string `json:"package"`
	Version        string `json:"version"`
	ArchiveSHA256  string `json:"archive_sha256"`
	ElementsSHA256 string `json:"elements_sha256"`
	LicenseSHA256  string `json:"license_sha256"`
	PackageSHA256  string `json:"package_sha256"`
}

type cssManifest struct {
	TailwindVersion  string   `json:"tailwind_version"`
	TailwindPlatform string   `json:"tailwind_platform"`
	TailwindSHA256   string   `json:"tailwind_sha256"`
	InputSHA256      string   `json:"input_sha256"`
	OutputSHA256     string   `json:"output_sha256"`
	SourceFiles      []string `json:"source_files"`
	ThemePolicy      string   `json:"theme_policy"`
	MaxDocumentBytes int      `json:"max_document_bytes"`
}

type fontManifest struct {
	Family         string       `json:"family"`
	File           fileIdentity `json:"file"`
	License        fileIdentity `json:"license"`
	ManifestSHA256 string       `json:"manifest_sha256"`
}

type manifest struct {
	SchemaVersion int                     `json:"schema_version"`
	Kind          string                  `json:"kind"`
	Runtime       runtimeManifest         `json:"runtime"`
	Cases         []caseManifest          `json:"cases"`
	Files         map[string]fileIdentity `json:"files"`
	CSS           cssManifest             `json:"css"`
	Font          fontManifest            `json:"font"`
	Verification  string                  `json:"verification"`
}

type sourceReader func(root, archiveID, path string) (*designassets.TextFile, error)

type loadedCase struct {
	spec   caseSpec
	body   []byte
	remote []string
}

type runtimeFiles struct {
	elements, license, packageJSON, archive []byte
}

func main() {
	kitRoot := flag.String("kit-root", "", "absolute existing app-kit root")
	runtimeTGZ := flag.String("runtime-tgz", "", "absolute existing Elements 1.0.22 package tarball")
	out := flag.String("out", "", "absolute new private output directory")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, refused("arguments"))
		os.Exit(2)
	}
	m, err := prepare(*kitRoot, *runtimeTGZ, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Never log source, runtime, CSS, font, license, or document bodies.
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"path": *out, "kind": m.Kind, "case_count": len(m.Cases), "verification": m.Verification,
	})
}

func refused(stage string) error {
	return fmt.Errorf("Elements reference preparation refused: %s", stage)
}
func digest(body []byte) string         { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
func identity(body []byte) fileIdentity { return fileIdentity{SHA256: digest(body), Size: len(body)} }

func prepare(kitRoot, runtimeTGZ, out string) (manifest, error) {
	root, err := validateKitRoot(kitRoot)
	if err != nil {
		return manifest{}, err
	}
	if !filepath.IsAbs(runtimeTGZ) {
		return manifest{}, refused("absolute-runtime-tgz")
	}
	if !filepath.IsAbs(out) || filepath.Clean(out) != out {
		return manifest{}, refused("absolute-new-output")
	}
	if _, err := os.Lstat(out); !errors.Is(err, os.ErrNotExist) {
		return manifest{}, refused("output-exists")
	}

	rt, err := readRuntimeArchive(runtimeTGZ, elementsArchiveSHA)
	if err != nil {
		return manifest{}, err
	}
	if digest(rt.elements) != elementsRuntimeSHA {
		return manifest{}, refused("runtime-elements-integrity")
	}
	cases, err := loadSources(filepath.Join(root, "data/design-assets"), caseSpecs, designassets.ReadFile)
	if err != nil {
		return manifest{}, err
	}
	font, fontLicense, fontID, fontLicenseID, err := readFont(root)
	if err != nil {
		return manifest{}, err
	}
	tailwindBin, tailwindPin, err := cachedTailwind()
	if err != nil {
		return manifest{}, err
	}

	scratch, err := os.MkdirTemp("", "appkit-elements-reference-")
	if err != nil {
		return manifest{}, refused("private-scratch")
	}
	if err := os.Chmod(scratch, 0700); err != nil {
		_ = os.RemoveAll(scratch)
		return manifest{}, refused("private-scratch-mode")
	}
	defer os.RemoveAll(scratch)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	css, err := compileCSS(ctx, scratch, tailwindBin, cases)
	if err != nil {
		return manifest{}, err
	}

	files := map[string][]byte{
		"elements-1.0.22.tgz": rt.archive,
		"elements.js":         rt.elements,
		"elements-LICENSE.md": rt.license,
		"inter.woff2":         font,
		"inter-LICENSE.txt":   fontLicense,
		"reference.css":       css,
	}
	m := manifest{
		SchemaVersion: 1,
		Kind:          "elements-source-reference",
		Runtime:       runtimeManifest{Package: "@tailwindplus/elements", Version: elementsVersion, ArchiveSHA256: elementsArchiveSHA, ElementsSHA256: elementsRuntimeSHA, LicenseSHA256: digest(rt.license), PackageSHA256: digest(rt.packageJSON)},
		Files:         map[string]fileIdentity{},
		CSS:           cssManifest{TailwindVersion: "v4.3.0", TailwindPlatform: runtime.GOOS + "-" + runtime.GOARCH, TailwindSHA256: tailwindPin, InputSHA256: digest([]byte(cssInput)), OutputSHA256: digest(css), SourceFiles: sourceFileNames(caseSpecs), ThemePolicy: "explicit html.dark; fixed host padding/background; source classes unchanged", MaxDocumentBytes: maxDocumentBytes},
		Font:          fontManifest{Family: "InterVariable", File: fontID, License: fontLicenseID, ManifestSHA256: fontManifestSHA},
		Verification:  "not-verified",
	}
	for _, c := range cases {
		sourceName := c.spec.Slug + "-original.html"
		files[sourceName] = slices.Clone(c.body)
		docs := map[string]string{}
		for _, theme := range []string{"light", "dark"} {
			name := c.spec.Slug + "-" + theme + ".html"
			doc, err := document(c.spec.ID, c.body, theme)
			if err != nil {
				return manifest{}, err
			}
			files[name] = doc
			docs[theme] = name
		}
		m.Cases = append(m.Cases, caseManifest{ID: c.spec.ID, Source: sourceIdentity{ArchiveID: c.spec.ArchiveID, Path: c.spec.Path, SHA256: c.spec.SHA256, Size: len(c.body)}, Documents: docs, SourceFile: sourceName, RemoteAssets: c.remote})
	}
	for name, body := range files {
		m.Files[name] = identity(body)
	}
	manifestBody, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return manifest{}, refused("manifest-encoding")
	}
	files["manifest.json"] = append(manifestBody, '\n')
	if err := publishNew(out, files); err != nil {
		return manifest{}, err
	}
	return m, nil
}

func validateKitRoot(root string) (string, error) {
	if !filepath.IsAbs(root) {
		return "", refused("absolute-kit-root")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", refused("kit-root")
	}
	mod, err := readRegular(filepath.Join(resolved, "go.mod"), 64<<10)
	if err != nil || !bytes.HasPrefix(mod, []byte("module github.com/geoffmilleraz/app-kit\n")) {
		return "", refused("kit-module")
	}
	return resolved, nil
}

func loadSources(root string, specs []caseSpec, read sourceReader) ([]loadedCase, error) {
	if len(specs) != 4 {
		return nil, refused("case-closure")
	}
	seen := map[string]bool{}
	result := make([]loadedCase, 0, len(specs))
	for _, spec := range specs {
		if spec.ID == "" || spec.Slug == "" || seen[spec.ID] || seen[spec.Slug] || !strings.HasPrefix(spec.Path, "html/") || filepath.Clean(spec.Path) != spec.Path {
			return nil, refused("case-spec")
		}
		seen[spec.ID], seen[spec.Slug] = true, true
		got, err := read(root, spec.ArchiveID, spec.Path)
		if err != nil || got == nil || got.ArchiveID != spec.ArchiveID || got.Path != spec.Path || got.SHA256 != spec.SHA256 || got.Size <= 0 || got.Size != int64(len(got.Content)) || digest([]byte(got.Content)) != spec.SHA256 {
			return nil, refused("source-integrity-" + spec.ID)
		}
		remote, err := inspectFragment(got.Content)
		if err != nil {
			return nil, err
		}
		result = append(result, loadedCase{spec: spec, body: []byte(got.Content), remote: remote})
	}
	return result, nil
}

func inspectFragment(body string) ([]string, error) {
	if len(body) == 0 || len(body) > maxDocumentBytes {
		return nil, refused("fragment-bound")
	}
	z := html.NewTokenizer(strings.NewReader(body))
	remote := map[string]bool{}
	for {
		t := z.Next()
		if t == html.ErrorToken {
			if z.Err() == io.EOF {
				break
			}
			return nil, refused("fragment-parse")
		}
		if t == html.DoctypeToken {
			return nil, refused("fragment-document")
		}
		if t != html.StartTagToken && t != html.SelfClosingTagToken && t != html.EndTagToken {
			continue
		}
		tok := z.Token()
		tag := strings.ToLower(tok.Data)
		if tag == "script" || tag == "base" || tag == "iframe" || tag == "frame" || tag == "frameset" {
			return nil, refused("fragment-active-element")
		}
		for _, a := range tok.Attr {
			key := strings.ToLower(a.Key)
			if strings.HasPrefix(key, "on") || key == "srcdoc" {
				return nil, refused("fragment-active-attribute")
			}
			if executableURL(a.Val) {
				return nil, refused("fragment-executable-url")
			}
			for _, found := range remoteURLs(a.Val) {
				remote[found] = true
			}
		}
	}
	assets := make([]string, 0, len(remote))
	for item := range remote {
		assets = append(assets, item)
	}
	sort.Strings(assets)
	return assets, nil
}

func executableURL(value string) bool {
	compact := strings.Map(func(r rune) rune {
		if r <= ' ' || r == 0x7f {
			return -1
		}
		return r
	}, strings.ToLower(value))
	for _, scheme := range []string{"javascript:", "vbscript:", "data:", "file:", "blob:"} {
		if strings.HasPrefix(compact, scheme) {
			return true
		}
	}
	return false
}

func remoteURLs(value string) []string {
	var found []string
	for _, field := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '\'' || r == '"' || r == '(' || r == ')' || r == ',' || r == ' ' || r == '\t' || r == '\r' || r == '\n'
	}) {
		candidate := strings.Trim(field, "\\")
		if strings.HasPrefix(candidate, "//") {
			found = append(found, "https:"+candidate)
			continue
		}
		u, err := url.Parse(candidate)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			found = append(found, candidate)
		}
	}
	return found
}

func readRuntimeArchive(path, expectedSHA string) (runtimeFiles, error) {
	body, err := readAbsoluteRegular(path, maxArchiveBytes)
	if err != nil || digest(body) != expectedSHA {
		return runtimeFiles{}, refused("runtime-archive-integrity")
	}
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return runtimeFiles{}, refused("runtime-gzip")
	}
	tr := tar.NewReader(gz)
	wanted := map[string]int{"package/dist/index.js": maxRuntimeBytes, "package/LICENSE.md": maxLicenseBytes, "package/package.json": 64 << 10}
	got := map[string][]byte{}
	count := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			_ = gz.Close()
			return runtimeFiles{}, refused("runtime-tar")
		}
		count++
		if count > maxTarEntries {
			_ = gz.Close()
			return runtimeFiles{}, refused("runtime-entry-bound")
		}
		limit, selected := wanted[h.Name]
		if !selected {
			continue
		}
		if _, duplicate := got[h.Name]; duplicate || h.Typeflag != tar.TypeReg || h.Size < 1 || h.Size > int64(limit) {
			_ = gz.Close()
			return runtimeFiles{}, refused("runtime-member")
		}
		member, err := io.ReadAll(io.LimitReader(tr, int64(limit)+1))
		if err != nil || int64(len(member)) != h.Size {
			_ = gz.Close()
			return runtimeFiles{}, refused("runtime-member-read")
		}
		got[h.Name] = member
	}
	if err := gz.Close(); err != nil || len(got) != len(wanted) {
		return runtimeFiles{}, refused("runtime-closure")
	}
	r := runtimeFiles{elements: got["package/dist/index.js"], license: got["package/LICENSE.md"], packageJSON: got["package/package.json"], archive: body}
	if err := validateRuntimePackage(r.packageJSON, r.elements); err != nil {
		return runtimeFiles{}, err
	}
	return r, nil
}

func validateRuntimePackage(packageJSON, elements []byte) error {
	var p struct {
		Name    string                     `json:"name"`
		Version string                     `json:"version"`
		Type    string                     `json:"type"`
		Exports map[string]json.RawMessage `json:"exports"`
	}
	if json.Unmarshal(packageJSON, &p) != nil {
		return refused("runtime-package-json")
	}
	if p.Name != "@tailwindplus/elements" || p.Version != elementsVersion || p.Type != "module" {
		return refused("runtime-package-identity")
	}
	root, ok := p.Exports["."]
	if !ok {
		return refused("runtime-package-entrypoint")
	}
	entry, err := rootImport(root)
	if err != nil || entry != "./dist/index.js" {
		return refused("runtime-package-entrypoint")
	}
	lower := strings.ToLower(string(elements))
	if strings.Contains(lower, "sourcemappingurl=") {
		return refused("runtime-source-map-closure")
	}
	for _, line := range strings.Split(string(elements), "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "import ") || strings.HasPrefix(trim, "import{") || strings.HasPrefix(trim, "import\"") || strings.Contains(trim, "import(") || strings.HasPrefix(trim, "export ") && strings.Contains(trim, " from ") {
			return refused("runtime-module-closure")
		}
	}
	return nil
}

func rootImport(raw json.RawMessage) (string, error) {
	var direct string
	if json.Unmarshal(raw, &direct) == nil {
		return direct, nil
	}
	var conditions map[string]json.RawMessage
	if json.Unmarshal(raw, &conditions) != nil {
		return "", refused("runtime-package-entrypoint")
	}
	selected, ok := conditions["import"]
	if !ok {
		return "", refused("runtime-package-entrypoint")
	}
	if json.Unmarshal(selected, &direct) == nil {
		return direct, nil
	}
	var nested map[string]json.RawMessage
	if json.Unmarshal(selected, &nested) != nil {
		return "", refused("runtime-package-entrypoint")
	}
	selected, ok = nested["default"]
	if !ok || json.Unmarshal(selected, &direct) != nil {
		return "", refused("runtime-package-entrypoint")
	}
	return direct, nil
}

func document(caseID string, fragment []byte, theme string) ([]byte, error) {
	if theme != "light" && theme != "dark" {
		return nil, refused("document-theme")
	}
	if _, err := inspectFragment(string(fragment)); err != nil {
		return nil, err
	}
	themeClass := "light"
	if theme == "dark" {
		themeClass = "dark"
	}
	doc := []byte("<!doctype html>\n<html lang=\"en\" class=\"" + themeClass + "\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>" + caseID + " Elements source reference</title><link rel=\"stylesheet\" href=\"reference.css\"><script type=\"module\" src=\"elements.js\"></script></head><body><main id=\"specimen\">" + string(fragment) + "</main></body></html>\n")
	if len(doc) > maxDocumentBytes {
		return nil, refused("document-bound")
	}
	return doc, nil
}

func sourceFileNames(specs []caseSpec) []string {
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Slug+"-original.html")
	}
	return names
}

func compileCSS(ctx context.Context, scratch, bin string, cases []loadedCase) ([]byte, error) {
	if len(cases) != 4 {
		return nil, refused("css-source-closure")
	}
	for _, c := range cases {
		if err := writeExclusive(filepath.Join(scratch, c.spec.Slug+"-original.html"), c.body); err != nil {
			return nil, refused("css-source")
		}
	}
	if err := writeExclusive(filepath.Join(scratch, "input.css"), []byte(cssInput)); err != nil {
		return nil, refused("css-input")
	}
	var first []byte
	for _, name := range []string{"first.css", "second.css"} {
		cmd := exec.CommandContext(ctx, bin, "-i", "input.css", "-o", name, "--minify")
		cmd.Dir, cmd.Stdout, cmd.Stderr = scratch, io.Discard, io.Discard
		if err := cmd.Run(); err != nil {
			return nil, refused("tailwind-build")
		}
		css, err := readRegular(filepath.Join(scratch, name), maxDocumentBytes)
		if err != nil || len(css) == 0 || bytes.Contains(bytes.ToLower(css), []byte("@source")) || bytes.Contains(bytes.ToLower(css), []byte("@import")) {
			return nil, refused("css-output")
		}
		if first == nil {
			first = css
		} else if !bytes.Equal(first, css) {
			return nil, refused("css-repeatability")
		}
	}
	return first, nil
}

func cachedTailwind() (string, string, error) {
	m, err := tailwind.LoadManifest()
	if err != nil || m.Version != "v4.3.0" {
		return "", "", refused("tailwind-pin")
	}
	pin, ok := m.Binaries[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		return "", "", refused("tailwind-platform")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", "", refused("tailwind-cache")
	}
	path := filepath.Join(cache, "app-kit/tailwindcss", m.Version, pin.Asset)
	body, err := readRegular(path, 512<<20)
	if err != nil {
		return "", "", refused("cached-tailwind-missing")
	}
	if digest(body) != pin.SHA256 {
		return "", "", refused("cached-tailwind-checksum")
	}
	return path, pin.SHA256, nil
}

func readFont(root string) ([]byte, []byte, fileIdentity, fileIdentity, error) {
	metadata, err := readRegular(filepath.Join(root, "docs/assessments/fixtures/catalyst-native/font-assets.json"), 64<<10)
	if err != nil || digest(metadata) != fontManifestSHA {
		return nil, nil, fileIdentity{}, fileIdentity{}, refused("font-manifest")
	}
	type record struct {
		Path string `json:"path"`
		fileIdentity
	}
	var m struct {
		Fonts   []record `json:"fonts"`
		License record   `json:"license"`
	}
	if json.Unmarshal(metadata, &m) != nil || m.License.Path != "LICENSE.txt" {
		return nil, nil, fileIdentity{}, fileIdentity{}, refused("font-metadata")
	}
	var chosen record
	for _, f := range m.Fonts {
		if f.Path == fontPath {
			if chosen.Path != "" {
				return nil, nil, fileIdentity{}, fileIdentity{}, refused("font-duplicate")
			}
			chosen = f
		}
	}
	if chosen.Path == "" || chosen.Size < 48 || chosen.Size > maxDocumentBytes {
		return nil, nil, fileIdentity{}, fileIdentity{}, refused("font-selection")
	}
	font, err := readRegular(filepath.Join(root, fontRoot, fontPath), chosen.Size)
	if err != nil || identity(font) != chosen.fileIdentity || len(font) < 12 || string(font[:4]) != "wOF2" || int(binary.BigEndian.Uint32(font[8:12])) != len(font) {
		return nil, nil, fileIdentity{}, fileIdentity{}, refused("font-integrity")
	}
	license, err := readRegular(filepath.Join(root, fontRoot, "LICENSE.txt"), 64<<10)
	if err != nil || identity(license) != m.License.fileIdentity {
		return nil, nil, fileIdentity{}, fileIdentity{}, refused("font-license")
	}
	return font, license, chosen.fileIdentity, m.License.fileIdentity, nil
}

func readAbsoluteRegular(path string, limit int) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, refused("absolute-regular-path")
	}
	return readRegular(path, limit)
}

func readRegular(path string, limit int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || info.Size() > int64(limit) {
		return nil, refused("regular-file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, refused("regular-file-open")
	}
	body, readErr := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	closeErr := f.Close()
	if readErr != nil || closeErr != nil || int64(len(body)) != info.Size() {
		return nil, refused("regular-file-read")
	}
	return body, nil
}

func writeExclusive(path string, body []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(body)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return refused("private-write")
	}
	return nil
}

func publishNew(out string, files map[string][]byte) error {
	return publishNewWithHook(out, files, nil)
}

type publicationHook func(parent *os.Root, leaf string) error

func publishNewWithHook(out string, files map[string][]byte, afterCreate publicationHook) error {
	if !filepath.IsAbs(out) || filepath.Clean(out) != out {
		return refused("absolute-new-output")
	}
	parentPath, err := filepath.EvalSymlinks(filepath.Dir(out))
	if err != nil {
		return refused("output-parent")
	}
	leaf := filepath.Base(out)
	if leaf == "." || leaf == ".." || leaf == string(filepath.Separator) {
		return refused("output-name")
	}
	parentInfo, err := os.Lstat(parentPath)
	if err != nil || parentInfo.Mode() != os.ModeDir|0700 {
		return refused("private-output-parent")
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return refused("output-parent-open")
	}
	defer parent.Close()
	actualParent, err := parent.Stat(".")
	if err != nil || !os.SameFile(parentInfo, actualParent) {
		return refused("output-parent-changed")
	}
	if _, err := parent.Lstat(leaf); !errors.Is(err, os.ErrNotExist) {
		return refused("output-exists")
	}
	names := make([]string, 0, len(files))
	for name := range files {
		if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
			return refused("publication-name")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	stageLeaf, err := makePrivateStage(parent)
	if err != nil {
		return err
	}
	stageInfo, err := parent.Lstat(stageLeaf)
	if err != nil || stageInfo.Mode() != os.ModeDir|0700 {
		return refused("publication-stage")
	}
	stage, err := openSameRoot(parent, stageLeaf, stageInfo)
	if err != nil {
		return err
	}
	defer func() {
		_ = stage.Close()
		removeSameDirectory(parent, stageLeaf, stageInfo)
	}()
	for _, name := range names {
		if err := writeRootExclusive(stage, name, files[name]); err != nil {
			return refused("publication-file")
		}
	}
	if err := verifyRootFiles(stage, files); err != nil {
		return err
	}
	// Mkdir is the no-clobber publication point: unlike Rename, it cannot replace
	// a concurrently-created empty directory. The retained parent/destination
	// handles confine writes and identity checks within a real 0700 parent. This
	// detects replacement at the checked boundaries; it is not protection from a
	// malicious process running as the same user. A write failure leaves an
	// explicit incomplete private directory; manifest.json is written last.
	if err := parent.Mkdir(leaf, 0700); err != nil {
		return refused("publication-create")
	}
	created, err := parent.Lstat(leaf)
	if err != nil || created.Mode() != os.ModeDir|0700 {
		return refused("publication-created-directory")
	}
	if afterCreate != nil {
		if err := afterCreate(parent, leaf); err != nil {
			return refused("publication-test-hook")
		}
	}
	destination, err := openSameRoot(parent, leaf, created)
	if err != nil {
		return err
	}
	defer destination.Close()
	ordered := slices.DeleteFunc(slices.Clone(names), func(name string) bool { return name == "manifest.json" })
	ordered = append(ordered, "manifest.json")
	for _, name := range ordered {
		body, err := readRootRegular(stage, name, maxDocumentBytes)
		if err != nil {
			return refused("publication-stage-read")
		}
		if err := writeRootExclusive(destination, name, body); err != nil {
			return refused("publication-write")
		}
	}
	if err := verifyRootFiles(destination, files); err != nil {
		return err
	}
	published, err := parent.Lstat(leaf)
	if err != nil || !os.SameFile(created, published) {
		return refused("publication-destination-changed")
	}
	return nil
}

func makePrivateStage(parent *os.Root) (string, error) {
	for range 8 {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return "", refused("publication-random")
		}
		name := ".elements-reference-" + hex.EncodeToString(random)
		if err := parent.Mkdir(name, 0700); err == nil {
			return name, nil
		} else if !errors.Is(err, os.ErrExist) {
			return "", refused("publication-stage")
		}
	}
	return "", refused("publication-stage-collision")
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

func removeSameDirectory(parent *os.Root, name string, expected os.FileInfo) {
	actual, err := parent.Lstat(name)
	if err == nil && os.SameFile(expected, actual) {
		_ = parent.RemoveAll(name)
	}
}

func writeRootExclusive(root *os.Root, name string, body []byte) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(body)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return refused("private-write")
	}
	return nil
}

func readRootRegular(root *os.Root, name string, limit int) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Mode() != 0600 || before.Size() < 0 || before.Size() > int64(limit) {
		return nil, refused("root-regular-file")
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, refused("root-regular-open")
	}
	after, statErr := f.Stat()
	body, readErr := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	closeErr := f.Close()
	if statErr != nil || !os.SameFile(before, after) || after.Mode() != 0600 || readErr != nil || closeErr != nil || int64(len(body)) != before.Size() {
		return nil, refused("root-regular-read")
	}
	return body, nil
}

func verifyRootFiles(root *os.Root, files map[string][]byte) error {
	dir, err := root.Open(".")
	if err != nil {
		return refused("output-closure")
	}
	entries, readErr := dir.ReadDir(len(files) + 1)
	closeErr := dir.Close()
	if readErr != nil && readErr != io.EOF || closeErr != nil || len(entries) != len(files) {
		return refused("output-closure")
	}
	for _, entry := range entries {
		want, ok := files[entry.Name()]
		if !ok || entry.Type()&os.ModeSymlink != 0 {
			return refused("output-entry")
		}
		got, err := readRootRegular(root, entry.Name(), maxDocumentBytes)
		if err != nil || !bytes.Equal(got, want) {
			return refused("output-integrity")
		}
	}
	return nil
}
