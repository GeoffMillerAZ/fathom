package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

func TestExplicitLiveSourcesCSSAndFont(t *testing.T) {
	root := os.Getenv("APPKIT_ELEMENTS_LIVE_ROOT")
	if root == "" {
		t.Skip("set APPKIT_ELEMENTS_LIVE_ROOT for explicit private source verification")
	}
	cases, err := loadSources(filepath.Join(root, "data/design-assets"), caseSpecs, designassets.ReadFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := readFont(root); err != nil {
		t.Fatal(err)
	}
	bin, _, err := cachedTailwind()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	css, err := compileCSS(ctx, t.TempDir(), bin, cases)
	if err != nil || len(css) == 0 {
		t.Fatal("live CSS closure refused", err)
	}
	for _, c := range cases {
		for _, theme := range []string{"light", "dark"} {
			if _, err := document(c.spec.ID, c.body, theme); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestRuntimeArchiveIntegrityAndClosure(t *testing.T) {
	packageJSON := []byte(`{"name":"@tailwindplus/elements","version":"1.0.22","type":"module","exports":{".":{"import":"./dist/index.js","types":"./dist/index.d.ts"}},"devDependencies":{"synthetic":"1"}}`)
	members := []tarMember{
		{name: "package/README.md", body: []byte("ignored supporting metadata")},
		{name: "package/package.json", body: packageJSON},
		{name: "package/LICENSE.md", body: []byte("synthetic license")},
		{name: "package/dist/index.js", body: []byte("export class SyntheticElement {}\n")},
	}
	tgz := syntheticTGZ(t, members)
	path := filepath.Join(t.TempDir(), "elements.tgz")
	if err := os.WriteFile(path, tgz, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readRuntimeArchive(path, digest(tgz))
	if err != nil {
		t.Fatal(err)
	}
	if string(got.elements) != "export class SyntheticElement {}\n" || string(got.license) != "synthetic license" || !bytes.Equal(got.packageJSON, packageJSON) {
		t.Fatal("selected runtime member bytes changed")
	}
	if !bytes.Equal(got.archive, tgz) {
		t.Fatal("retained runtime archive differs from verified input")
	}
	if _, err := readRuntimeArchive(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("wrong archive digest accepted")
	}

	t.Run("duplicate", func(t *testing.T) {
		bad := syntheticTGZ(t, append(members, tarMember{name: "package/dist/index.js", body: []byte("duplicate")}))
		badPath := filepath.Join(t.TempDir(), "duplicate.tgz")
		if os.WriteFile(badPath, bad, 0600) != nil {
			t.Fatal("write")
		}
		if _, err := readRuntimeArchive(badPath, digest(bad)); err == nil {
			t.Fatal("duplicate selected member accepted")
		}
	})

	t.Run("non-regular", func(t *testing.T) {
		badMembers := slices.Clone(members[:3])
		badMembers = append(badMembers, tarMember{name: "package/dist/index.js", link: "elsewhere", kind: tar.TypeSymlink})
		bad := syntheticTGZ(t, badMembers)
		badPath := filepath.Join(t.TempDir(), "link.tgz")
		if os.WriteFile(badPath, bad, 0600) != nil {
			t.Fatal("write")
		}
		if _, err := readRuntimeArchive(badPath, digest(bad)); err == nil {
			t.Fatal("linked runtime accepted")
		}
	})
}

func TestRuntimePackageRefusesIdentityEntrypointAndImports(t *testing.T) {
	base := map[string]any{"name": "@tailwindplus/elements", "version": elementsVersion, "type": "module", "exports": map[string]any{".": map[string]any{"import": "./dist/index.js", "require": "./dist/index.cjs"}, "./react": map[string]any{"import": "./dist/react/index.js"}}}
	encode := func(p map[string]any) []byte {
		body, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	if err := validateRuntimePackage(encode(base), []byte("export const ok = true\n")); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(map[string]any, *[]byte){
		"name":    func(p map[string]any, _ *[]byte) { p["name"] = "other" },
		"version": func(p map[string]any, _ *[]byte) { p["version"] = "1.0.21" },
		"entrypoint": func(p map[string]any, _ *[]byte) {
			p["exports"] = map[string]any{".": map[string]any{"import": "./dist/other.js"}}
		},
		"static-import":  func(_ map[string]any, js *[]byte) { *js = []byte("import x from './other.js'\n") },
		"dynamic-import": func(_ map[string]any, js *[]byte) { *js = []byte("const x = import('./other.js')\n") },
	} {
		t.Run(name, func(t *testing.T) {
			p := map[string]any{}
			for key, value := range base {
				p[key] = value
			}
			js := []byte("export const ok = true\n")
			mutate(p, &js)
			if validateRuntimePackage(encode(p), js) == nil {
				t.Fatal("unexpected runtime closure accepted")
			}
		})
	}
}

func TestSyntheticSourcePathsAndIntegrity(t *testing.T) {
	specs := make([]caseSpec, 4)
	bodies := map[string]string{}
	for i := range specs {
		body := "<!-- runtime setup is inert: <script src='https://example.invalid/elements.js'></script> --><el-disclosure><img src=\"https://images.example/" + string(rune('a'+i)) + ".jpg\"></el-disclosure>"
		specs[i] = caseSpec{ID: "P" + string(rune('1'+i)), Slug: "p" + string(rune('1'+i)), ArchiveID: "archive@" + strings.Repeat(string(rune('a'+i)), 64), Path: "html/case" + string(rune('1'+i)) + ".html", SHA256: digest([]byte(body))}
		bodies[specs[i].Path] = body
	}
	reader := func(_ string, archiveID, path string) (*designassets.TextFile, error) {
		body := bodies[path]
		return &designassets.TextFile{ArchiveID: archiveID, Path: path, SHA256: digest([]byte(body)), Size: int64(len(body)), Content: body}, nil
	}
	got, err := loadSources("synthetic", specs, reader)
	if err != nil || len(got) != 4 {
		t.Fatal("synthetic source closure refused", err)
	}
	if got[0].remote[0] != "https://images.example/a.jpg" {
		t.Fatal("remote image identity not retained", got[0].remote)
	}

	wrongPath := slices.Clone(specs)
	wrongPath[0].Path = "../escape.html"
	if _, err := loadSources("synthetic", wrongPath, reader); err == nil {
		t.Fatal("escaping source path accepted")
	}
	wrongHash := slices.Clone(specs)
	wrongHash[0].SHA256 = strings.Repeat("0", 64)
	if _, err := loadSources("synthetic", wrongHash, reader); err == nil {
		t.Fatal("source hash mismatch accepted")
	}
}

func TestFragmentSafetyRemoteAssetsAndDocuments(t *testing.T) {
	fragment := `<!-- <script src="https://comment.example/runtime.js"></script> --><el-dialog><button command="show-modal">Open</button><img src="https://images.example/photo.jpg?fit=crop"><a href="#drawer">Local</a></el-dialog>`
	assets, err := inspectFragment(fragment)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(assets, []string{"https://images.example/photo.jpg?fit=crop"}) {
		t.Fatal("remote assets include comments or omit source image", assets)
	}
	for _, bad := range []string{
		`<script type="module" src="elements.js"></script>`,
		`<button onclick="go()">Go</button>`,
		`<a href="java&#x0a;script:alert(1)">Go</a>`,
		`<iframe src="local"></iframe>`,
		`<base href="https://example.test/">`,
		`<img src="data:text/html,bad">`,
	} {
		if _, err := inspectFragment(bad); err == nil {
			t.Fatal("active fragment accepted", bad)
		}
	}

	doc, err := document("P07-menu", []byte(fragment), "dark")
	if err != nil {
		t.Fatal(err)
	}
	for _, exact := range []string{`<html lang="en" class="dark">`, `<link rel="stylesheet" href="reference.css">`, `<script type="module" src="elements.js"></script>`, `<main id="specimen">` + fragment + `</main>`} {
		if !bytes.Contains(doc, []byte(exact)) {
			t.Fatal("document contract missing", exact)
		}
	}
	if bytes.Contains(doc, []byte("Content-Security-Policy")) || bytes.Contains(doc, []byte("<style")) {
		t.Fatal("document embedded policy or style instead of using harness headers and external CSS")
	}
	if _, err := document("P07-menu", []byte(fragment), "system"); err == nil {
		t.Fatal("implicit theme accepted")
	}
	if _, err := document("P07-menu", []byte(strings.Repeat("x", maxDocumentBytes)), "light"); err == nil {
		t.Fatal("document bound not enforced")
	}
}

func TestPublishNewIsPrivateAndNeverClobbers(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "prepared")
	files := map[string][]byte{"manifest.json": []byte("{}\n"), "elements.js": []byte("synthetic")}
	if err := publishNew(out, files); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(out)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("output directory is not private")
	}
	for name := range files {
		info, err := os.Stat(filepath.Join(out, name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("output file is not private", name)
		}
	}
	changed := map[string][]byte{"manifest.json": []byte("changed"), "elements.js": []byte("changed")}
	if err := publishNew(out, changed); err == nil {
		t.Fatal("existing output was clobbered")
	}
	body, err := os.ReadFile(filepath.Join(out, "elements.js"))
	if err != nil || string(body) != "synthetic" {
		t.Fatal("refusal changed published bytes")
	}
}

func TestPublishRequiresPrivateRealParent(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "prepared")
	err := publishNew(out, map[string][]byte{"manifest.json": []byte("{}\n")})
	if err == nil || !strings.Contains(err.Error(), "private-output-parent") {
		t.Fatal("non-private parent accepted", err)
	}
	if _, statErr := os.Lstat(out); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("refused parent acquired output path", statErr)
	}
}

func TestPublishRefusesReplacedDestinationWithoutWritingSentinel(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(root, "sentinel")
	if err := os.Mkdir(sentinel, 0700); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "prepared")
	files := map[string][]byte{"elements.js": []byte("must-not-reach-sentinel"), "manifest.json": []byte("{}\n")}
	err := publishNewWithHook(out, files, func(parent *os.Root, leaf string) error {
		if err := parent.Rename(leaf, "displaced"); err != nil {
			return err
		}
		return parent.Symlink("sentinel", leaf)
	})
	if err == nil || !strings.Contains(err.Error(), "publication-destination-changed") {
		t.Fatal("replacement was not detected before publication", err)
	}
	entries, readErr := os.ReadDir(sentinel)
	if readErr != nil || len(entries) != 0 {
		t.Fatal("publication followed replacement into sentinel", entries, readErr)
	}
	displaced, readErr := os.ReadDir(filepath.Join(root, "displaced"))
	if readErr != nil || len(displaced) != 0 {
		t.Fatal("publication wrote before validating retained destination identity", displaced, readErr)
	}
}

type tarMember struct {
	name string
	body []byte
	link string
	kind byte
}

func syntheticTGZ(t *testing.T, members []tarMember) []byte {
	t.Helper()
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	tw := tar.NewWriter(gz)
	for _, member := range members {
		kind := member.kind
		if kind == 0 {
			kind = tar.TypeReg
		}
		size := int64(len(member.body))
		if kind != tar.TypeReg {
			size = 0
		}
		h := &tar.Header{Name: member.name, Mode: 0600, Size: size, Typeflag: kind, Linkname: member.link}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if size > 0 {
			if _, err := tw.Write(member.body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}
