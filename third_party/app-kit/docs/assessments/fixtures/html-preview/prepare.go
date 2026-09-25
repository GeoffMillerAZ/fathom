// Explicit private P02 preparation, never invoked by Gallery or ordinary tests.
// Run from app-kit: go run ./docs/assessments/fixtures/html-preview -kit-root ABSOLUTE_ROOT
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
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
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designattempts"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
	"github.com/geoffmilleraz/app-kit/internal/tailwind"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	preparedTarget  = "go-templ"
	attemptID       = "syntax@902c0695ff832fde0c73bb979cf73aa496098778061dffcff0276c11a6176043"
	sourceSHA       = "f545f273689b9a4312e5d0eeb66c15dc21d7d318e102e37c39b40f494e017e0f"
	fontManifestSHA = "dc7298020ee8a2e2502e7e038e8941c98123a03db4ffc8f52b6ff7fdc41f4655"
	fontRoot        = "data/design-assets/reference-assets/inter/46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441"
	fontPath        = "font-files/InterVariable.woff2"
	// Declared before compilation: total HTML, CSS and embedded font per document.
	maxDocumentBytes = 1 << 20
	templHeader      = "package converted\n\ntempl Specimen() {\n"
	cssInput         = `@import "tailwindcss" source(none);
@source "./original.html";
@custom-variant dark (&:where(.dark, .dark *));
@theme { --font-sans: "InterVariable", sans-serif; }
:root { color-scheme: light; }
:root.dark { color-scheme: dark; }
body { margin: 0; background: #fff; }
.dark body { background: #111827; }
`
	themePolicy = "explicit html.dark class; light body #ffffff; dark body #111827; vendor classes unchanged; no kit token mapping"
)

type fileIdentity struct {
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}
type documentIdentity struct {
	fileIdentity
	StyleSHA256 string `json:"style_sha256"`
}
type previewConfiguration struct {
	TailwindVersion    string       `json:"tailwind_version"`
	TailwindPlatform   string       `json:"tailwind_platform"`
	TailwindSHA256     string       `json:"tailwind_sha256"`
	Font               fileIdentity `json:"font"`
	FontFamily         string       `json:"font_family"`
	FontManifestSHA256 string       `json:"font_manifest_sha256"`
	FontContract       string       `json:"font_contract"`
	CSSInputSHA256     string       `json:"css_input_sha256"`
	CSSOutputSHA256    string       `json:"css_output_sha256"`
	ThemePolicy        string       `json:"theme_policy"`
	MaxDocumentBytes   int          `json:"max_document_bytes"`
}
type previewReceipt struct {
	SchemaVersion  int                         `json:"schema_version"`
	CaseID         string                      `json:"case_id"`
	Source         designconvert.Source        `json:"source"`
	AttemptID      string                      `json:"attempt_id"`
	ArtifactSHA256 string                      `json:"artifact_sha256"`
	Transformer    designconvert.Transformer   `json:"transformer"`
	Configuration  previewConfiguration        `json:"configuration"`
	Documents      map[string]documentIdentity `json:"documents"`
	Verification   string                      `json:"verification"`
	Scope          string                      `json:"scope"`
}

func main() {
	root := flag.String("kit-root", "", "absolute existing app-kit root; explicit private output only")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "P02 preparation refused: arguments")
		os.Exit(2)
	}
	receipt, dir, err := prepare(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// No original, rendered body, CSS, font bytes or compiler diagnostics are logged.
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"receipt_sha256": receipt, "path": dir, "document_count": 4, "verification": "not-verified"})
}

func refused(stage string) error     { return fmt.Errorf("P02 preparation refused: %s", stage) }
func digest(b []byte) string         { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func identity(b []byte) fileIdentity { return fileIdentity{digest(b), len(b)} }

func prepare(root string) (receiptHash, output string, err error) {
	if !filepath.IsAbs(root) {
		return "", "", refused("absolute-kit-root")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", refused("kit-root")
	}
	mod, err := readRegular(root, "go.mod", 64<<10)
	if err != nil || !bytes.HasPrefix(mod, []byte("module github.com/geoffmilleraz/app-kit\n")) || !bytes.Contains(mod, []byte("github.com/a-h/templ v0.3.1020\n")) {
		return "", "", refused("pinned-kit-module")
	}
	assets := filepath.Join(root, "data/design-assets")
	detail, err := designattempts.Read(assets, attemptID)
	if err != nil {
		return "", "", refused("saved-attempt")
	}
	t := detail.Translation
	if detail.Replay != "matches-current" || detail.Entry.ID != attemptID || t.Status != "translated" || detail.Entry.Status != t.Status || t.Target != preparedTarget || detail.Entry.Target != t.Target || t.Verification != "not-verified" || detail.Entry.Verification != "not-verified" || t.Source.SHA256 != sourceSHA || t.Source != detail.Entry.Source || t.Transformer != detail.Entry.Transformer || t.Artifact == nil || t.Artifact.SHA256 != detail.Entry.ArtifactSHA256 || digest([]byte(t.Artifact.Content)) != t.Artifact.SHA256 {
		return "", "", refused("frozen-identity")
	}
	original, err := designassets.ReadFile(assets, t.Source.ArchiveID, t.Source.Path)
	if err != nil || original.ArchiveID != t.Source.ArchiveID || original.Path != t.Source.Path || original.SHA256 != sourceSHA || digest([]byte(original.Content)) != sourceSHA || len(original.Content) != t.Source.Size {
		return "", "", refused("original-integrity")
	}
	if designconvert.VerifyReplay(t, []byte(original.Content)) != nil {
		return "", "", refused("literal-replay")
	}
	if err := safeFragment(original.Content); err != nil {
		return "", "", err
	}
	font, license, fontID, err := readFont(root)
	if err != nil {
		return "", "", err
	}
	bin, pin, err := cachedTailwind()
	if err != nil {
		return "", "", err
	}
	scratch, err := os.MkdirTemp("", "appkit-p02-")
	if err != nil {
		return "", "", refused("private-scratch")
	}
	defer func() {
		if e := os.RemoveAll(scratch); e != nil && err == nil {
			err = refused("private-cleanup")
		}
	}()
	env, err := privateEnv(scratch)
	if err != nil {
		return "", "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	converted, err := compileLiteral(ctx, root, scratch, env, t.Artifact.Content, original.Content)
	if err != nil {
		return "", "", err
	}
	if err := safeFragment(converted); err != nil {
		return "", "", err
	}
	css, err := compileCSS(ctx, scratch, env, bin, original.Content)
	if err != nil {
		return "", "", err
	}
	style := "/* Inter v4.1, upstream SIL Open Font License follows.\n" + string(license) + "\n*/\n@font-face{font-family:InterVariable;font-style:normal;font-weight:100 900;font-display:block;src:url(data:font/woff2;base64," + base64.StdEncoding.EncodeToString(font) + ") format('woff2')}\n" + string(css)
	r := previewReceipt{SchemaVersion: 1, CaseID: "P02-pricing", Source: t.Source, AttemptID: attemptID, ArtifactSHA256: t.Artifact.SHA256, Transformer: t.Transformer, Verification: "not-verified", Scope: "controlled-static-reference", Documents: map[string]documentIdentity{}, Configuration: previewConfiguration{
		TailwindVersion: "v4.3.0", TailwindPlatform: runtime.GOOS + "-" + runtime.GOARCH, TailwindSHA256: pin, Font: fontID, FontFamily: "InterVariable", FontManifestSHA256: fontManifestSHA, FontContract: "unknown: explicit fixture font, not a claim about the original portal font", CSSInputSHA256: digest([]byte(cssInput)), CSSOutputSHA256: digest(css), ThemePolicy: themePolicy, MaxDocumentBytes: maxDocumentBytes,
	}}
	files := map[string][]byte{}
	for _, theme := range []string{"light", "dark"} {
		for _, pane := range []struct{ name, body string }{{"original", original.Content}, {"converted", converted}} {
			name := pane.name + "-" + theme + ".html"
			body, err := document(pane.body, style, theme)
			if err != nil {
				return "", "", err
			}
			files[name] = body
			r.Documents[name] = documentIdentity{identity(body), digest([]byte(style))}
		}
	}
	receipt, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", "", refused("receipt-encoding")
	}
	files["receipt.json"] = append(receipt, '\n')
	receiptHash = digest(files["receipt.json"])
	output, err = publish(root, receiptHash, files)
	return receiptHash, output, err
}

func readFont(root string) ([]byte, []byte, fileIdentity, error) {
	metadata, err := readRegular(root, "docs/assessments/fixtures/catalyst-native/font-assets.json", 64<<10)
	if err != nil || digest(metadata) != fontManifestSHA {
		return nil, nil, fileIdentity{}, refused("font-manifest")
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
		return nil, nil, fileIdentity{}, refused("font-metadata")
	}
	var chosen record
	for _, f := range m.Fonts {
		if f.Path == fontPath {
			if chosen.Path != "" {
				return nil, nil, fileIdentity{}, refused("font-duplicate")
			}
			chosen = f
		}
	}
	if chosen.Path == "" || chosen.Size < 48 || chosen.Size > maxDocumentBytes {
		return nil, nil, fileIdentity{}, refused("font-selection")
	}
	font, err := readRegular(root, fontRoot+"/"+fontPath, chosen.Size)
	if err != nil || identity(font) != chosen.fileIdentity || string(font[:min(4, len(font))]) != "wOF2" || len(font) < 48 || int(binary.BigEndian.Uint32(font[8:12])) != len(font) {
		return nil, nil, fileIdentity{}, refused("font-integrity")
	}
	license, err := readRegular(root, fontRoot+"/LICENSE.txt", 64<<10)
	if err != nil || identity(license) != m.License.fileIdentity || strings.Contains(string(license), "*/") || strings.Contains(strings.ToLower(string(license)), "</style") {
		return nil, nil, fileIdentity{}, refused("font-license")
	}
	return font, license, chosen.fileIdentity, nil
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
	name := filepath.Join(cache, "app-kit/tailwindcss", m.Version, pin.Asset)
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", refused("cached-tailwind-missing")
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 512<<20 {
		return "", "", refused("cached-tailwind-file")
	}
	f, err := os.Open(name)
	if err != nil {
		return "", "", refused("cached-tailwind-read")
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, (512<<20)+1))
	if err != nil || n != info.Size() || hex.EncodeToString(h.Sum(nil)) != pin.SHA256 {
		return "", "", refused("cached-tailwind-checksum")
	}
	return name, pin.SHA256, nil // Never Resolve, download, delete or replace cache entries.
}

func privateEnv(scratch string) ([]string, error) {
	values := map[string]string{"GOENV": "off", "GOCACHEPROG": "", "GOPROXY": "off", "GOSUMDB": "off", "GONOPROXY": "none", "GOPRIVATE": "none", "GONOSUMDB": "none", "GOVCS": "*:off", "GOTOOLCHAIN": "local", "GOFLAGS": "-mod=readonly", "GOWORK": "off", "CGO_ENABLED": "0", "GOTELEMETRY": "off", "GO_TELEMETRY_CHILD": "", "GO_TELEMETRY_CHILD_UPLOAD": "", "NODE_OPTIONS": "", "NODE_PATH": ""}
	for _, key := range []string{"GOCACHE", "GOTMPDIR", "TMPDIR"} {
		dir := filepath.Join(scratch, strings.ToLower(key))
		if os.Mkdir(dir, 0700) != nil {
			return nil, refused("private-cache")
		}
		values[key] = dir
	}
	// Go 1.26 reports GOTELEMETRY but does not use that environment variable
	// as a mode override. OpenDir/MaybeParent use TEST_TELEMETRY_DIR and read
	// its mode file instead. Never inspect or update the owner's telemetry.
	telemetryDir := filepath.Join(scratch, "telemetry")
	if os.Mkdir(telemetryDir, 0700) != nil || os.WriteFile(filepath.Join(telemetryDir, "mode"), []byte("off\n"), 0600) != nil {
		return nil, refused("private-telemetry")
	}
	values["TEST_TELEMETRY_DIR"] = telemetryDir
	var env []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if _, replace := values[key]; !replace {
			env = append(env, item)
		}
	}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env, nil
}

type boundedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *boundedBuffer) Len() int      { return b.buffer.Len() }
func (b *boundedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, refused("output-bound")
	}
	return b.buffer.Write(p)
}
func run(ctx context.Context, cwd string, env []string, stage, executable string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir, cmd.Env = cwd, env
	output := &boundedBuffer{limit: maxDocumentBytes}
	cmd.Stdout, cmd.Stderr = output, io.Discard
	if cmd.Run() != nil {
		return nil, refused(stage)
	}
	return output.Bytes(), nil
}

func compileLiteral(ctx context.Context, root, scratch string, env []string, converted, original string) (string, error) {
	if !strings.HasPrefix(converted, templHeader) {
		return "", refused("templ-header")
	}
	// Only the initial package declaration changes; all template bytes stay intact.
	converted = "package main" + converted[len("package converted"):]
	if os.WriteFile(filepath.Join(scratch, "snippet.templ"), []byte(converted), 0600) != nil || os.WriteFile(filepath.Join(scratch, "runner.go"), []byte(runner), 0600) != nil {
		return "", refused("private-templ-write")
	}
	var first string
	for _, phase := range []string{"unformatted", "formatted"} {
		if phase == "formatted" {
			if _, err := run(ctx, root, env, "templ-format", "go", "tool", "templ", "fmt", "-w", "2", scratch); err != nil {
				return "", err
			}
		}
		if _, err := run(ctx, root, env, "templ-generate-"+phase, "go", "tool", "templ", "generate", "-path", scratch, "-w", "2", "-include-version=false"); err != nil {
			return "", err
		}
		binary := filepath.Join(scratch, "render")
		if _, err := run(ctx, root, env, "templ-compile-"+phase, "go", "build", "-o", binary, filepath.Join(scratch, "snippet_templ.go"), filepath.Join(scratch, "runner.go")); err != nil {
			return "", err
		}
		out, err := run(ctx, scratch, env, "templ-render-"+phase, binary)
		if err != nil {
			return "", err
		}
		if !sameDOM(original, string(out)) {
			return "", refused("DOM-" + phase)
		}
		if phase == "unformatted" {
			first = string(out)
		} else if first != string(out) {
			return "", refused("formatted-render-bytes")
		}
	}
	return first, nil
}

const runner = `package main
import("context"; "os")
func main(){ if Specimen().Render(context.Background(),os.Stdout)!=nil { os.Exit(1) } }
`

type nodeShape struct {
	Type            html.NodeType
	Data, Namespace string
	Attr            []html.Attribute
	Children        []nodeShape
}

func shape(n *html.Node) nodeShape {
	s := nodeShape{Type: n.Type, Data: n.Data, Namespace: n.Namespace, Attr: n.Attr}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s.Children = append(s.Children, shape(c))
	}
	return s
}
func fragment(s string) ([]nodeShape, error) {
	nodes, err := html.ParseFragment(strings.NewReader(s), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return nil, err
	}
	result := []nodeShape{}
	for _, n := range nodes {
		result = append(result, shape(n))
	}
	return result, nil
}
func sameDOM(a, b string) bool {
	left, e1 := fragment(a)
	right, e2 := fragment(b)
	return e1 == nil && e2 == nil && reflect.DeepEqual(left, right)
}

// Refuse resources/behavior rather than rewriting the independently retained body.
// This allowlist is deliberately only a static pricing-fragment viewer contract.
func safeFragment(body string) error {
	if len(body) == 0 || len(body) > designconvert.MaxSourceBytes {
		return refused("fragment-bound")
	}
	tags := strings.Fields("div section header footer main article h1 h2 h3 h4 p span a ul ol li dl dt dd strong em b i small br hr svg g path title desc")
	attrs := strings.Fields("class id role data-slot aria-hidden aria-label aria-labelledby aria-describedby xmlns viewbox fill stroke stroke-width stroke-linecap stroke-linejoin fill-rule clip-rule d width height")
	z := html.NewTokenizer(strings.NewReader(body))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if z.Err() == io.EOF {
				return nil
			}
			return refused("fragment-parse")
		}
		if tt == html.DoctypeToken {
			return refused("fragment-document")
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken && tt != html.EndTagToken {
			continue
		}
		token := z.Token()
		if !slices.Contains(tags, token.Data) {
			return refused("fragment-element")
		}
		for _, a := range token.Attr {
			if a.Key == "href" && token.Data == "a" && strings.HasPrefix(a.Val, "#") && !strings.ContainsAny(a.Val, "\x00\r\n\t ") {
				continue
			}
			if !slices.Contains(attrs, a.Key) {
				return refused("fragment-attribute")
			}
			lower := strings.ToLower(a.Val)
			if strings.Contains(lower, "url(") || strings.Contains(lower, "url\\") || strings.Contains(lower, "@import") || strings.ContainsAny(lower, "\x00\r") {
				return refused("fragment-resource")
			}
			if a.Key == "xmlns" && (token.Data != "svg" || a.Val != "http://www.w3.org/2000/svg") {
				return refused("fragment-namespace")
			}
		}
	}
}

func compileCSS(ctx context.Context, scratch string, env []string, bin, original string) ([]byte, error) {
	if os.WriteFile(filepath.Join(scratch, "original.html"), []byte(original), 0600) != nil || os.WriteFile(filepath.Join(scratch, "input.css"), []byte(cssInput), 0600) != nil {
		return nil, refused("css-input")
	}
	var first []byte
	for _, name := range []string{"one.css", "two.css"} {
		if _, err := run(ctx, scratch, env, "tailwind-build", bin, "-i", "input.css", "-o", name, "--minify"); err != nil {
			return nil, err
		}
		css, err := readRegular(scratch, name, maxDocumentBytes)
		if err != nil {
			return nil, refused("css-output")
		}
		lower := strings.ToLower(string(css))
		for _, forbidden := range []string{"url(", "@import", "</style", "@source"} {
			if strings.Contains(lower, forbidden) {
				return nil, refused("css-resource")
			}
		}
		if first == nil {
			first = css
		} else if !bytes.Equal(first, css) {
			return nil, refused("css-repeatability")
		}
	}
	return first, nil
}

func document(body, style, theme string) ([]byte, error) {
	if theme != "light" && theme != "dark" {
		return nil, refused("document-theme")
	}
	if err := safeFragment(body); err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToLower(style), "</style") {
		return nil, refused("style-boundary")
	}
	styleSum := sha256.Sum256([]byte(style))
	csp := "default-src 'none'; style-src 'sha256-" + base64.StdEncoding.EncodeToString(styleSum[:]) + "'; font-src data:; base-uri 'none'; form-action 'none'"
	doc := []byte("<!doctype html>\n<html lang=\"en\" class=\"" + theme + "\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><meta http-equiv=\"Content-Security-Policy\" content=\"" + csp + "\"><title>P02 pricing: controlled static reference</title><style>" + style + "</style></head><body>" + body + "</body></html>\n")
	if len(doc) > maxDocumentBytes {
		return nil, refused("document-bound")
	}
	return doc, nil
}

func selectedPath(root, relative string) (string, error) {
	if !filepath.IsLocal(relative) {
		return "", refused("local-path")
	}
	current := root
	for _, part := range strings.Split(filepath.ToSlash(relative), "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", refused("regular-path")
		}
	}
	return current, nil
}
func readRegular(root, relative string, limit int) ([]byte, error) {
	name, err := selectedPath(root, relative)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, refused("regular-file-bound")
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, refused("regular-file-read")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, refused("regular-file-changed")
	}
	body, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(body) > limit || int64(len(body)) != info.Size() {
		return nil, refused("regular-file-read-bound")
	}
	return body, nil
}

func publish(root, hash string, files map[string][]byte) (string, error) {
	parent, err := selectedPath(root, "data/design-assets")
	if err != nil {
		return "", refused("private-output-root")
	}
	for _, part := range []string{"previews", "p02-pricing"} {
		parent = filepath.Join(parent, part)
		if err := os.Mkdir(parent, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", refused("private-output-directory")
		}
		info, err := os.Lstat(parent)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
			return "", refused("private-output-mode")
		}
	}
	lock, err := os.OpenFile(filepath.Join(parent, ".prepare.lock"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", refused("publication-lock")
	}
	lock.Close()
	defer os.Remove(filepath.Join(parent, ".prepare.lock"))
	target := filepath.Join(parent, hash)
	if _, err := os.Lstat(target); err == nil {
		return target, verifyExisting(target, files)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", refused("existing-output")
	}
	stage, err := os.MkdirTemp(parent, ".prepare-")
	if err != nil {
		return "", refused("publication-stage")
	}
	defer os.RemoveAll(stage)
	for name, body := range files {
		f, err := os.OpenFile(filepath.Join(stage, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return "", refused("publication-file")
		}
		_, writeErr := f.Write(body)
		syncErr := f.Sync()
		closeErr := f.Close()
		if writeErr != nil || syncErr != nil || closeErr != nil {
			return "", refused("publication-write")
		}
	}
	if err := os.Rename(stage, target); err != nil {
		return "", refused("publication-rename")
	}
	return target, verifyExisting(target, files)
}
func verifyExisting(dir string, files map[string][]byte) error {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return refused("existing-output-directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != len(files) {
		return refused("existing-output-closure")
	}
	for name, want := range files {
		got, err := readRegular(dir, name, maxDocumentBytes)
		info, statErr := os.Lstat(filepath.Join(dir, name))
		if err != nil || statErr != nil || info.Mode().Perm() != 0600 || !bytes.Equal(got, want) {
			return refused("existing-output-conflict")
		}
	}
	return nil
}
