package gallery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/net/html"
)

var staticExportTestCSS = []byte(`:root{color-scheme:light dark;--ak-color-ink:#111;--ak-color-surface:#fff}`)

func staticExportCanonicalTempDir(t *testing.T) string {
	t.Helper()
	// macOS may supply /var through TMPDIR. Positive fixtures must use the
	// canonical parent required by the exporter; explicit symlink cases below
	// still exercise refusal without depending on the host's temporary root.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestExportStaticContentIsDeterministicAndClosed(t *testing.T) {
	one := filepath.Join(staticExportCanonicalTempDir(t), "one")
	two := filepath.Join(staticExportCanonicalTempDir(t), "two")
	if err := ExportStaticContent(one, staticExportTestCSS, "/"); err != nil {
		t.Fatal(err)
	}
	if err := ExportStaticContent(two, staticExportTestCSS, "/"); err != nil {
		t.Fatal(err)
	}
	first := readStaticExport(t, one)
	second := readStaticExport(t, two)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("two exports differ")
	}
	want := []string{"404-dark.html", "404-light.html", "404.html", "app.css", "index-dark.html", "index-light.html", "index.html", "manifest.json", "profile-dark.html", "profile-light.html", "profile.html"}
	if got := sortedStaticExportNames(first); !reflect.DeepEqual(got, want) {
		t.Fatalf("inventory = %v", got)
	}
	var manifest staticExportManifest
	if err := json.Unmarshal(first["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 || manifest.BasePath != "/" || !reflect.DeepEqual(manifest.ManifestExcludes, []string{"manifest.json"}) || len(manifest.Files) != 10 {
		t.Fatalf("manifest identity = %#v", manifest)
	}
	manifestNames := make([]string, 0, len(manifest.Files))
	for _, entry := range manifest.Files {
		body, ok := first[entry.Path]
		if !ok || entry.Bytes != len(body) {
			t.Fatalf("manifest entry mismatch: %#v", entry)
		}
		digest := sha256.Sum256(body)
		if entry.SHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("manifest digest mismatch: %s", entry.Path)
		}
		manifestNames = append(manifestNames, entry.Path)
	}
	if !sort.StringsAreSorted(manifestNames) {
		t.Fatalf("manifest is not sorted: %v", manifestNames)
	}
	if err := validateStaticExport(first, "/"); err != nil {
		t.Fatal(err)
	}
	for name, body := range first {
		if !strings.HasSuffix(name, ".html") {
			continue
		}
		text := string(body)
		for _, forbidden := range []string{"<script", "<style", "<base", "<form", "href=\"/", "href=\"http", "?theme=", "Gallery navigation", "Browse reusable capabilities", "Specimen state"} {
			if strings.HasPrefix(name, "404") && forbidden == "href=\"/" {
				continue
			}
			if strings.Contains(text, forbidden) {
				t.Errorf("%s contains %q", name, forbidden)
			}
		}
		assertUniqueStaticExportIDs(t, name, body)
		if !strings.Contains(text, `href="#static-reference-main"`) || !strings.Contains(text, `<main id="static-reference-main" tabindex="-1"`) {
			t.Errorf("%s lacks the focusable main skip target", name)
		}
		if strings.Contains(text, "bg-surface-overlay") {
			t.Errorf("%s uses an unavailable semantic utility", name)
		}
	}
	if !bytes.Equal(first["app.css"], staticExportTestCSS) {
		t.Fatal("trusted compiled CSS was not preserved exactly")
	}
}

func TestExportStaticContentDeep404UsesSelectedBasePath(t *testing.T) {
	for _, basePath := range []string{"/", "/reading/"} {
		t.Run(basePath, func(t *testing.T) {
			out := filepath.Join(staticExportCanonicalTempDir(t), "site")
			if err := ExportStaticContent(out, staticExportTestCSS, basePath); err != nil {
				t.Fatal(err)
			}
			files := readStaticExport(t, out)
			if err := validateStaticExport(files, basePath); err != nil {
				t.Fatal(err)
			}
			for _, theme := range []string{"", "-light", "-dark"} {
				body := string(files["404"+theme+".html"])
				for _, want := range []string{`href="` + basePath + `app.css"`, `href="` + basePath + `index` + theme + `.html"`, `aria-current="page"`} {
					if !strings.Contains(body, want) {
						t.Errorf("404%s missing %q", theme, want)
					}
				}
			}
			if strings.Contains(string(files["index.html"]), `href="`+basePath) {
				t.Fatal("ordinary page used an absolute base-path URL")
			}
		})
	}
}

func TestStaticExportValidatorRejectsBrokenOrExecutableDocuments(t *testing.T) {
	mutations := map[string]func(map[string][]byte){
		"form": func(files map[string][]byte) {
			files["index.html"] = bytes.Replace(files["index.html"], []byte("</main>"), []byte("<form></form></main>"), 1)
		},
		"duplicate id": func(files map[string][]byte) {
			files["index.html"] = bytes.Replace(files["index.html"], []byte("</main>"), []byte(`<p id="content-title"></p></main>`), 1)
		},
		"missing fragment": func(files map[string][]byte) {
			files["index.html"] = bytes.Replace(files["index.html"], []byte("#content-profile-title"), []byte("#missing-fragment"), 1)
		},
		"escaped path": func(files map[string][]byte) {
			files["index.html"] = bytes.Replace(files["index.html"], []byte(`href="profile.html`), []byte(`href="../profile.html`), 1)
		},
		"nonlocal URL": func(files map[string][]byte) {
			files["index.html"] = bytes.Replace(files["index.html"], []byte(`href="profile.html`), []byte(`href="https://example.invalid/profile.html`), 1)
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			files, err := buildStaticExport(staticExportTestCSS, "/")
			if err != nil {
				t.Fatal(err)
			}
			mutate(files)
			if err := validateStaticExport(files, "/"); err == nil {
				t.Fatal("invalid static document accepted")
			}
		})
	}
}

func TestExportStaticContentRefusesInputsAtomically(t *testing.T) {
	parent := staticExportCanonicalTempDir(t)
	cases := []struct {
		name string
		out  string
		css  []byte
	}{
		{name: "relative", out: "relative-output", css: staticExportTestCSS},
		{name: "empty CSS", out: filepath.Join(parent, "empty"), css: nil},
		{name: "invalid UTF-8 CSS", out: filepath.Join(parent, "utf8"), css: []byte{0xff}},
		{name: "oversized CSS", out: filepath.Join(parent, "large"), css: bytes.Repeat([]byte("x"), staticContentCSSLimit+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ExportStaticContent(tc.out, tc.css, "/"); err == nil {
				t.Fatal("invalid export accepted")
			}
			if filepath.IsAbs(tc.out) {
				if _, err := os.Lstat(tc.out); !os.IsNotExist(err) {
					t.Fatalf("refusal changed output: %v", err)
				}
			}
		})
	}
	existing := filepath.Join(parent, "existing")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(existing, "keep")
	if err := os.WriteFile(marker, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ExportStaticContent(existing, staticExportTestCSS, "/"); err == nil {
		t.Fatal("existing target accepted")
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "unchanged" {
		t.Fatalf("existing target changed: %q %v", body, err)
	}
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := ExportStaticContent(link, staticExportTestCSS, "/"); err == nil {
		t.Fatal("symlink target accepted")
	}
	linkedParent := filepath.Join(parent, "parent-link")
	if err := os.Symlink(parent, linkedParent); err != nil {
		t.Fatal(err)
	}
	if err := ExportStaticContent(filepath.Join(linkedParent, "child"), staticExportTestCSS, "/"); err == nil {
		t.Fatal("symlinked parent accepted")
	}
}

func TestExportStaticContentRefusesInvalidBasePathsBeforeCreation(t *testing.T) {
	for i, basePath := range []string{"", "reading/", "/reading", "//reading/", "/reading//deep/", "/reading/../", "/reading/%2e/", "/reading/?x/", "/reading/#x/", "/read ing/", "/réading/"} {
		out := filepath.Join(staticExportCanonicalTempDir(t), fmt.Sprintf("site-%d", i))
		if err := ExportStaticContent(out, staticExportTestCSS, basePath); err == nil {
			t.Errorf("invalid base path accepted: %q", basePath)
		}
		if _, err := os.Lstat(out); !os.IsNotExist(err) {
			t.Fatalf("base path refusal changed output: %q: %v", basePath, err)
		}
	}
}

func readStaticExport(t *testing.T, root string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			t.Fatalf("non-regular output: %s", entry.Name())
		}
		body, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if !utf8.Valid(body) {
			t.Fatalf("invalid UTF-8 output: %s", entry.Name())
		}
		files[entry.Name()] = body
	}
	return files
}

func assertUniqueStaticExportIDs(t *testing.T, name string, body []byte) {
	t.Helper()
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			for _, attr := range node.Attr {
				if attr.Key == "id" {
					if seen[attr.Val] {
						t.Errorf("%s duplicates id %q", name, attr.Val)
					}
					seen[attr.Val] = true
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
}
