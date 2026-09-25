package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestHostServesPinnedHTMLContextAssetsAndHEAD(t *testing.T) {
	dir, manifestSHA := syntheticFixture(t)
	m, assets, err := loadPinnedAssets(dir, manifestSHA)
	if err != nil {
		t.Fatal(err)
	}
	host, err := newEditorHost("127.0.0.1:43127", m, assets)
	if err != nil {
		t.Fatal(err)
	}

	page := hostRequest(host, http.MethodGet, "/", nil)
	if page.Code != http.StatusOK {
		t.Fatalf("page status = %d", page.Code)
	}
	body := page.Body.String()
	for _, required := range []string{
		`<h1>Client-owned editor</h1>`, `Nothing is saved.`, `id="host-update"`, `>Update context</button>`,
		`id="host-dispose"`, `>Dispose editor</button>`, `id="host-mount"`, `>Mount editor</button>`,
		`<div id="editor-slot"></div>`, `<output id="host-state"`, `<div id="secondary-slot"></div>`,
		`<link rel="stylesheet" href="/assets/base.css">`, `<link rel="stylesheet" href="/assets/editor.css">`,
		`<script type="module" src="/assets/editor.js"></script>`,
		`<script type="application/json" id="editor-config">{"version":1,"instanceId":"primary","label":"Product description","hint":"Local draft only. Nothing is saved.","initialText":"Start typing..."}</script>`,
	} {
		if !strings.Contains(body, required) {
			t.Errorf("page is missing %q", required)
		}
	}
	if strings.Count(body, "<script") != 2 || strings.Contains(body, "<script>") {
		t.Fatalf("page gained an inline executable script: %s", body)
	}
	for _, absent := range []string{"host-morph", "morph-transport-form", "host-fragment", "data-ignore-morph", "morph-client.js"} {
		if strings.Contains(body, absent) {
			t.Fatalf("ordinary page unexpectedly enabled Datastar morph asset %q", absent)
		}
	}
	if response := hostRequest(host, http.MethodGet, "/morph-fragment", nil); response.Code != http.StatusNotFound {
		t.Fatalf("ordinary host exposed morph route: %d", response.Code)
	}
	wantCSP := "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; frame-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; object-src 'none'"
	if got := page.Header().Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("unexpected CSP %q", got)
	}

	contextResponse := hostRequest(host, http.MethodGet, "/context", nil)
	wantContext := `{"version":1,"instanceId":"primary","label":"Updated product description","hint":"Context updated. Your local draft is unchanged."}`
	if contextResponse.Code != http.StatusOK || contextResponse.Body.String() != wantContext || contextResponse.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("context response = %d %q %q", contextResponse.Code, contextResponse.Body.String(), contextResponse.Header().Get("Content-Type"))
	}

	for route, want := range map[string]string{
		"/assets/editor.js":  "export const fixture = true;\n",
		"/assets/base.css":   "body { color: black; }\n",
		"/assets/editor.css": ".editor { min-height: 8rem; }\n",
	} {
		get := hostRequest(host, http.MethodGet, route, nil)
		head := hostRequest(host, http.MethodHead, route, nil)
		if get.Code != http.StatusOK || get.Body.String() != want || head.Code != http.StatusOK || head.Body.Len() != 0 {
			t.Fatalf("asset %s GET/HEAD mismatch: get=%d/%q head=%d/%q", route, get.Code, get.Body.String(), head.Code, head.Body.String())
		}
		if head.Header().Get("Content-Length") != strconv.Itoa(len(want)) || head.Header().Get("Content-Type") != get.Header().Get("Content-Type") || head.Header().Get("ETag") == "" {
			t.Fatalf("asset %s HEAD metadata mismatch: %v", route, head.Header())
		}
	}
	pageHead := hostRequest(host, http.MethodHead, "/", nil)
	contextHead := hostRequest(host, http.MethodHead, "/context", nil)
	if pageHead.Code != http.StatusOK || pageHead.Body.Len() != 0 || pageHead.Header().Get("Content-Length") != strconv.Itoa(len(host.html)) ||
		contextHead.Code != http.StatusOK || contextHead.Body.Len() != 0 || contextHead.Header().Get("Content-Length") != strconv.Itoa(len(host.context)) {
		t.Fatalf("page/context HEAD contract failed: page=%v context=%v", pageHead, contextHead)
	}
}

func TestDatastarMorphOptInHTMLRouteAndRefusals(t *testing.T) {
	dir, manifestSHA := syntheticFixture(t)
	m, assets, err := loadPinnedAssets(dir, manifestSHA)
	if err != nil {
		t.Fatal(err)
	}
	runtimeBody := []byte("pinned runtime bytes are verified by loadPinnedAssets")
	clientBody := []byte("export const morphFixture = true;\n")
	licenseBody := []byte("adjacent pinned MIT license")
	clientSHA := testDigest(clientBody)
	m.Files = append(m.Files,
		manifestFile{Path: datastarPath, SHA256: datastarSHA256, Bytes: int64(len(runtimeBody))},
		manifestFile{Path: morphClientPath, SHA256: clientSHA, Bytes: int64(len(clientBody))},
		manifestFile{Path: datastarLicense, SHA256: licenseSHA256, Bytes: int64(len(licenseBody))},
	)
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	m, err = decodeManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	assets["/"+datastarPath] = pinnedAsset{body: runtimeBody, contentType: "text/javascript; charset=utf-8", sha256: datastarSHA256}
	assets["/"+morphClientPath] = pinnedAsset{body: clientBody, contentType: "text/javascript; charset=utf-8", sha256: clientSHA}
	assets["/"+datastarLicense] = pinnedAsset{body: licenseBody, contentType: "text/plain; charset=utf-8", sha256: licenseSHA256}
	host, err := newEditorHost("127.0.0.1:43130", m, assets)
	if err != nil {
		t.Fatal(err)
	}

	page := hostRequest(host, http.MethodGet, "/", nil)
	pageBody := page.Body.String()
	for _, required := range []string{
		`<button id="host-morph" type="button">Update surrounding context</button>`,
		`<form id="morph-transport-form" hidden></form>`,
		`<section id="host-fragment"><p id="fragment-state">Initial surrounding context.</p><div id="editor-slot" data-ignore-morph></div></section>`,
		`<script type="module" src="/assets/morph-client.js"></script>`,
	} {
		if !strings.Contains(pageBody, required) {
			t.Errorf("morph page is missing %q", required)
		}
	}
	if strings.Count(pageBody, "<script") != 3 || strings.Contains(pageBody, "<script>") {
		t.Fatalf("morph page changed the external/application-json script policy: %s", pageBody)
	}
	if strings.Index(pageBody, `</section><output id="host-state"`) < 0 {
		t.Fatal("host-state must remain outside the morph wrapper")
	}

	const wantFragment = `<section id="host-fragment"><p id="fragment-state">Updated surrounding context.</p><div id="editor-slot" data-ignore-morph></div></section>`
	get := hostRequest(host, http.MethodGet, "/morph-fragment", nil)
	head := hostRequest(host, http.MethodHead, "/morph-fragment", nil)
	for name, response := range map[string]*httptest.ResponseRecorder{"GET": get, "HEAD": head} {
		if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/html; charset=utf-8" ||
			response.Header().Get("Content-Length") != strconv.Itoa(len(wantFragment)) ||
			response.Header().Get("Datastar-Selector") != "#host-fragment" || response.Header().Get("Datastar-Mode") != "outer" {
			t.Fatalf("%s morph response contract failed: status=%d headers=%v", name, response.Code, response.Header())
		}
	}
	if get.Body.String() != wantFragment || head.Body.Len() != 0 {
		t.Fatalf("morph GET/HEAD body mismatch: get=%q head=%q", get.Body.String(), head.Body.String())
	}
	license := hostRequest(host, http.MethodGet, "/"+datastarLicense, nil)
	if license.Code != http.StatusOK || license.Header().Get("Content-Type") != "text/plain; charset=utf-8" || license.Body.String() != string(licenseBody) {
		t.Fatalf("license asset response mismatch: %d %q %q", license.Code, license.Header().Get("Content-Type"), license.Body.String())
	}
	if response := hostRequest(host, http.MethodPost, "/morph-fragment", nil); response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("morph POST refusal = %d Allow %q", response.Code, response.Header().Get("Allow"))
	}
	if response := hostRequest(host, http.MethodGet, "/morph-fragment?extra=1", nil); response.Code != http.StatusNotFound {
		t.Fatalf("morph query refusal = %d", response.Code)
	}
	if response := hostRequest(host, http.MethodGet, "/morph-fragment", http.Header{"Origin": {"https://evil.example"}}); response.Code != http.StatusForbidden {
		t.Fatalf("morph foreign-origin refusal = %d", response.Code)
	}
}

func TestDatastarMorphReservedSetAndPinsAreRejected(t *testing.T) {
	base := manifest{
		Schema: 1,
		Entry:  "assets/editor.js",
		Styles: []string{"assets/base.css"},
		Files: []manifestFile{
			{Path: "assets/editor.js", SHA256: strings.Repeat("a", 64), Bytes: 1},
			{Path: "assets/base.css", SHA256: strings.Repeat("b", 64), Bytes: 1},
		},
	}
	reserved := []manifestFile{
		{Path: datastarPath, SHA256: datastarSHA256, Bytes: 1},
		{Path: morphClientPath, SHA256: strings.Repeat("c", 64), Bytes: 1},
		{Path: datastarLicense, SHA256: licenseSHA256, Bytes: 1},
	}
	for count := 1; count < len(reserved); count++ {
		candidate := cloneManifest(t, base)
		candidate.Files = append(candidate.Files, reserved[:count]...)
		assertManifestRejected(t, candidate)
	}
	badRuntime := cloneManifest(t, base)
	badRuntime.Files = append(badRuntime.Files, reserved...)
	badRuntime.Files[len(badRuntime.Files)-3].SHA256 = strings.Repeat("d", 64)
	assertManifestRejected(t, badRuntime)
	badLicense := cloneManifest(t, base)
	badLicense.Files = append(badLicense.Files, reserved...)
	badLicense.Files[len(badLicense.Files)-1].SHA256 = strings.Repeat("e", 64)
	assertManifestRejected(t, badLicense)
}

func TestAssetsArePinnedAndTamperingIsRejected(t *testing.T) {
	dir, manifestSHA := syntheticFixture(t)
	m, assets, err := loadPinnedAssets(dir, manifestSHA)
	if err != nil {
		t.Fatal(err)
	}
	host, err := newEditorHost("127.0.0.1:43128", m, assets)
	if err != nil {
		t.Fatal(err)
	}
	assetPath := filepath.Join(dir, "assets", "editor.js")
	tampered := []byte("export const fixture = true;\n")
	tampered[len(tampered)-2] = 'X'
	if err := os.WriteFile(assetPath, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	response := hostRequest(host, http.MethodGet, "/assets/editor.js", nil)
	if response.Body.String() != "export const fixture = true;\n" {
		t.Fatalf("live handler reread changed disk bytes: %q", response.Body.String())
	}
	if _, _, err := loadPinnedAssets(dir, manifestSHA); err == nil {
		t.Fatal("fresh load accepted a tampered listed asset")
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	manifestBody, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(manifestBody, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadPinnedAssets(dir, manifestSHA); err == nil {
		t.Fatal("load accepted a manifest whose pinned bytes changed")
	}
}

func TestManifestShapePathsAndLimitsAreRejected(t *testing.T) {
	valid := manifest{
		Schema: 1,
		Entry:  "assets/editor.js",
		Styles: []string{"assets/base.css", "assets/editor.css"},
		Files: []manifestFile{
			{Path: "assets/editor.js", SHA256: strings.Repeat("a", 64), Bytes: 1},
			{Path: "assets/base.css", SHA256: strings.Repeat("b", 64), Bytes: 1},
			{Path: "assets/editor.css", SHA256: strings.Repeat("c", 64), Bytes: 1},
		},
	}
	mutations := map[string]func(*manifest){
		"schema":             func(m *manifest) { m.Schema = 2 },
		"missing entry":      func(m *manifest) { m.Entry = "assets/missing.js" },
		"entry extension":    func(m *manifest) { m.Entry = "assets/base.css" },
		"missing style":      func(m *manifest) { m.Styles[0] = "assets/missing.css" },
		"duplicate style":    func(m *manifest) { m.Styles[1] = m.Styles[0] },
		"duplicate file":     func(m *manifest) { m.Files[1].Path = m.Files[0].Path },
		"parent path":        func(m *manifest) { m.Files[0].Path, m.Entry = "../editor.js", "../editor.js" },
		"absolute path":      func(m *manifest) { m.Files[0].Path, m.Entry = "/editor.js", "/editor.js" },
		"unclean path":       func(m *manifest) { m.Files[0].Path, m.Entry = "assets/./editor.js", "assets/./editor.js" },
		"backslash path":     func(m *manifest) { m.Files[0].Path, m.Entry = `assets\editor.js`, `assets\editor.js` },
		"non ASCII path":     func(m *manifest) { m.Files[0].Path, m.Entry = "assets/éditor.js", "assets/éditor.js" },
		"URL delimiter path": func(m *manifest) { m.Files[0].Path, m.Entry = "assets/editor?.js", "assets/editor?.js" },
		"unsupported type":   func(m *manifest) { m.Files[0].Path, m.Entry = "assets/editor.mjs", "assets/editor.mjs" },
		"uppercase SHA":      func(m *manifest) { m.Files[0].SHA256 = strings.Repeat("A", 64) },
		"short SHA":          func(m *manifest) { m.Files[0].SHA256 = "abc" },
		"negative size":      func(m *manifest) { m.Files[0].Bytes = -1 },
		"large file":         func(m *manifest) { m.Files[0].Bytes = maxFileBytes + 1 },
		"empty styles":       func(m *manifest) { m.Styles = nil },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := cloneManifest(t, valid)
			mutate(&candidate)
			body, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeManifest(body); err == nil {
				t.Fatalf("accepted invalid manifest: %s", body)
			}
		})
	}

	tooMany := cloneManifest(t, valid)
	tooMany.Files = make([]manifestFile, maxFiles+1)
	for index := range tooMany.Files {
		tooMany.Files[index] = manifestFile{Path: fmt.Sprintf("assets/f%d.js", index), SHA256: strings.Repeat("a", 64), Bytes: 1}
	}
	tooMany.Entry = tooMany.Files[0].Path
	tooMany.Styles = []string{tooMany.Files[1].Path}
	tooMany.Files[1].Path = "assets/base.css"
	assertManifestRejected(t, tooMany)

	overTotal := cloneManifest(t, valid)
	overTotal.Files = []manifestFile{
		{Path: "assets/editor.js", SHA256: strings.Repeat("a", 64), Bytes: maxFileBytes},
		{Path: "assets/base.css", SHA256: strings.Repeat("b", 64), Bytes: maxFileBytes},
		{Path: "assets/editor.css", SHA256: strings.Repeat("c", 64), Bytes: maxFileBytes},
		{Path: "assets/font.woff2", SHA256: strings.Repeat("d", 64), Bytes: maxFileBytes},
		{Path: "assets/image.png", SHA256: strings.Repeat("e", 64), Bytes: 1},
	}
	assertManifestRejected(t, overTotal)

	for name, body := range map[string][]byte{
		"unknown top-level field": []byte(`{"schema":1,"entry":"assets/editor.js","styles":["assets/base.css"],"files":[{"path":"assets/editor.js","sha256":"` + strings.Repeat("a", 64) + `","bytes":1},{"path":"assets/base.css","sha256":"` + strings.Repeat("b", 64) + `","bytes":1}],"extra":true}`),
		"unknown file field":      []byte(`{"schema":1,"entry":"assets/editor.js","styles":["assets/base.css"],"files":[{"path":"assets/editor.js","sha256":"` + strings.Repeat("a", 64) + `","bytes":1,"extra":true},{"path":"assets/base.css","sha256":"` + strings.Repeat("b", 64) + `","bytes":1}]}`),
		"trailing JSON":           []byte(`{"schema":1} {}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeManifest(body); err == nil {
				t.Fatal("accepted non-strict JSON")
			}
		})
	}
}

func TestSymlinksAndNonPrivateRootsAreRejected(t *testing.T) {
	dir, manifestSHA := syntheticFixture(t)
	assetPath := filepath.Join(dir, "assets", "editor.js")
	targetPath := filepath.Join(dir, "editor-target.js")
	if err := os.Rename(assetPath, targetPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetPath, assetPath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadPinnedAssets(dir, manifestSHA); err == nil {
		t.Fatal("accepted a symlinked listed asset")
	}

	dir, manifestSHA = syntheticFixture(t)
	alias := filepath.Join(t.TempDir(), "assets-link")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadPinnedAssets(alias, manifestSHA); err == nil {
		t.Fatal("accepted a symlinked assets root")
	}

	dir, manifestSHA = syntheticFixture(t)
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadPinnedAssets(dir, manifestSHA); err == nil {
		t.Fatal("accepted a non-private assets root")
	}
}

func TestHostRejectsForeignRequestsMethodsAndUnknownRoutes(t *testing.T) {
	dir, manifestSHA := syntheticFixture(t)
	m, assets, err := loadPinnedAssets(dir, manifestSHA)
	if err != nil {
		t.Fatal(err)
	}
	host, err := newEditorHost("127.0.0.1:43129", m, assets)
	if err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		response := hostRequest(host, method, "/", nil)
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("%s = %d Allow %q", method, response.Code, response.Header().Get("Allow"))
		}
	}
	for _, target := range []string{"/missing", "/assets/missing.js", "/context?extra=1", "/assets/editor.js?v=1", "/assets/%65ditor.js"} {
		if response := hostRequest(host, http.MethodGet, target, nil); response.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d", target, response.Code)
		}
	}

	badHosts := []string{"", "localhost:43129", "127.0.0.1:43130", "127.0.0.1.example:43129", "127.0.0.1@evil.example"}
	for _, authority := range badHosts {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Host = authority
		response := httptest.NewRecorder()
		host.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("Host %q = %d", authority, response.Code)
		}
	}
	badHeaders := []http.Header{
		{"Origin": {"https://evil.example"}},
		{"Origin": {"null"}},
		{"Origin": {"http://127.0.0.1:43130"}},
		{"Origin": {host.origin, host.origin}},
		{"Sec-Fetch-Site": {"cross-site"}},
		{"Sec-Fetch-Site": {"same-site"}},
		{"Sec-Fetch-Site": {"none", "same-origin"}},
	}
	for _, headers := range badHeaders {
		if response := hostRequest(host, http.MethodGet, "/", headers); response.Code != http.StatusForbidden {
			t.Fatalf("headers %v = %d", headers, response.Code)
		}
	}
	for _, headers := range []http.Header{nil, {"Origin": {host.origin}}, {"Sec-Fetch-Site": {"same-origin"}}, {"Sec-Fetch-Site": {"none"}}} {
		if response := hostRequest(host, http.MethodGet, "/", headers); response.Code != http.StatusOK {
			t.Fatalf("accepted headers %v = %d", headers, response.Code)
		}
	}
	for _, response := range []*httptest.ResponseRecorder{
		hostRequest(host, http.MethodGet, "/", http.Header{"Origin": {"https://evil.example"}}),
		hostRequest(host, http.MethodPost, "/", nil),
		hostRequest(host, http.MethodGet, "/missing", nil),
	} {
		if response.Header().Get("Content-Security-Policy") != hostCSP || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("refusal lost security headers: %v", response.Header())
		}
	}
}

func syntheticFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	bodies := map[string][]byte{
		"assets/editor.js":  []byte("export const fixture = true;\n"),
		"assets/base.css":   []byte("body { color: black; }\n"),
		"assets/editor.css": []byte(".editor { min-height: 8rem; }\n"),
	}
	m := manifest{Schema: 1, Entry: "assets/editor.js", Styles: []string{"assets/base.css", "assets/editor.css"}}
	for _, name := range []string{"assets/editor.js", "assets/base.css", "assets/editor.css"} {
		body := bodies[name]
		m.Files = append(m.Files, manifestFile{Path: name, SHA256: testDigest(body), Bytes: int64(len(body))})
		fullPath := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifestBody, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifestBody, 0600); err != nil {
		t.Fatal(err)
	}
	return dir, testDigest(manifestBody)
}

func hostRequest(host *editorHost, method, target string, headers http.Header) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	request.Host = host.authority
	request.Header = headers.Clone()
	response := httptest.NewRecorder()
	host.ServeHTTP(response, request)
	return response
}

func testDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func cloneManifest(t *testing.T, value manifest) manifest {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var cloned manifest
	if err := json.Unmarshal(body, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func assertManifestRejected(t *testing.T, value manifest) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeManifest(body); err == nil {
		t.Fatalf("accepted invalid manifest: %s", body)
	}
}
