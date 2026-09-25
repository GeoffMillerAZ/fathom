// Command client-editor-host serves a bounded, byte-pinned private editor build
// on an ephemeral loopback origin. It does not install, build, or execute source.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	maxManifestBytes = 1 << 20
	maxFiles         = 128
	maxFileBytes     = 4 << 20
	maxTotalBytes    = 16 << 20
	datastarPath     = "assets/datastar-v1.0.3.js"
	morphClientPath  = "assets/morph-client.js"
	datastarLicense  = "assets/datastar-LICENSE.txt"
	datastarSHA256   = "5d6b7794a50a83d82da962aec5e382f5ae83ac7afbc751f903f7a9c6bd433c65"
	licenseSHA256    = "17da0e216d3c6a819526f5b72a6a44d84d34bcd9beb4191ac6af89c1508ef08f"
	// The pinned MatDash base CSS uses a data:image SVG for native select arrows.
	// Keep that source-compatible exception image-only.
	hostCSP = "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self' data:; connect-src 'self'; frame-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; object-src 'none'"
)

type manifest struct {
	Schema int            `json:"schema"`
	Entry  string         `json:"entry"`
	Styles []string       `json:"styles"`
	Files  []manifestFile `json:"files"`
}

type manifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type pinnedAsset struct {
	body        []byte
	contentType string
	sha256      string
}

type editorHost struct {
	authority string
	origin    string
	html      []byte
	context   []byte
	morph     []byte
	assets    map[string]pinnedAsset
}

type startupRecord struct {
	Origin      string `json:"origin"`
	ManifestSHA string `json:"manifestSHA"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "client-editor-host:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("client-editor-host", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	assetsDir := flags.String("assets", "", "absolute private asset directory")
	manifestSHA := flags.String("manifest-sha", "", "expected manifest.json SHA-256")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *assetsDir == "" || *manifestSHA == "" {
		return errors.New("require -assets ABSOLUTE_PRIVATE_DIR -manifest-sha SHA and no positional arguments")
	}

	m, assets, err := loadPinnedAssets(*assetsDir, *manifestSHA)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("bind loopback: %w", err)
	}
	authority := listener.Addr().String()
	host, err := newEditorHost(authority, m, assets)
	if err != nil {
		_ = listener.Close()
		return err
	}
	record, _ := json.Marshal(startupRecord{Origin: host.origin, ManifestSHA: *manifestSHA})
	if _, err := fmt.Fprintln(stdout, string(record)); err != nil {
		_ = listener.Close()
		return fmt.Errorf("write startup record: %w", err)
	}

	server := &http.Server{
		Handler:           host,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(listener)
	}()

	select {
	case err := <-serveDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-signalContext.Done():
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		_ = server.Close()
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveDone; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

func loadPinnedAssets(dir, wantManifestSHA string) (manifest, map[string]pinnedAsset, error) {
	var empty manifest
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return empty, nil, errors.New("assets directory must be an absolute clean path")
	}
	if !validSHA(wantManifestSHA) {
		return empty, nil, errors.New("manifest SHA-256 must be 64 lowercase hexadecimal characters")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return empty, nil, errors.New("assets directory must be a private non-symlink directory")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return empty, nil, errors.New("open private assets directory")
	}
	defer root.Close()
	openedInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(info, openedInfo) {
		return empty, nil, errors.New("assets directory changed while opening")
	}

	manifestBody, err := readRootRegular(root, "manifest.json", maxManifestBytes)
	if err != nil {
		return empty, nil, fmt.Errorf("read manifest: %w", err)
	}
	if digest(manifestBody) != wantManifestSHA {
		return empty, nil, errors.New("manifest SHA-256 mismatch")
	}
	m, err := decodeManifest(manifestBody)
	if err != nil {
		return empty, nil, err
	}

	assets := make(map[string]pinnedAsset, len(m.Files))
	for _, file := range m.Files {
		body, err := readRootRegular(root, file.Path, maxFileBytes)
		if err != nil {
			return empty, nil, fmt.Errorf("read listed asset %q: %w", file.Path, err)
		}
		if int64(len(body)) != file.Bytes || digest(body) != file.SHA256 {
			return empty, nil, fmt.Errorf("listed asset %q failed size or SHA-256 verification", file.Path)
		}
		assets["/"+file.Path] = pinnedAsset{body: body, contentType: assetContentType(file.Path), sha256: file.SHA256}
	}
	return m, assets, nil
}

func decodeManifest(body []byte) (manifest, error) {
	var m manifest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return manifest{}, errors.New("manifest is not strict JSON for schema 1")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return manifest{}, err
	}
	if m.Schema != 1 || len(m.Files) == 0 || len(m.Files) > maxFiles || len(m.Styles) == 0 {
		return manifest{}, errors.New("manifest shape or cardinality is invalid")
	}
	if !validAssetPath(m.Entry) || path.Ext(m.Entry) != ".js" {
		return manifest{}, errors.New("manifest entry must be a normalized listed JavaScript path")
	}
	seen := make(map[string]bool, len(m.Files))
	var total int64
	for _, file := range m.Files {
		if !validAssetPath(file.Path) || seen[file.Path] || !validSHA(file.SHA256) || file.Bytes < 0 || file.Bytes > maxFileBytes {
			return manifest{}, errors.New("manifest contains an invalid file record")
		}
		seen[file.Path] = true
		if total > maxTotalBytes-file.Bytes {
			return manifest{}, errors.New("manifest file total exceeds 16 MiB")
		}
		total += file.Bytes
	}
	if !seen[m.Entry] {
		return manifest{}, errors.New("manifest entry is not listed in files")
	}
	styles := make(map[string]bool, len(m.Styles))
	for _, style := range m.Styles {
		if !validAssetPath(style) || path.Ext(style) != ".css" || styles[style] || !seen[style] {
			return manifest{}, errors.New("manifest style is invalid, duplicated, or not listed in files")
		}
		styles[style] = true
	}
	reserved := 0
	for _, name := range []string{datastarPath, morphClientPath, datastarLicense} {
		if seen[name] {
			reserved++
		}
	}
	if reserved != 0 && reserved != 3 {
		return manifest{}, errors.New("Datastar morph assets must be present as one complete reserved set")
	}
	if reserved == 3 {
		files := make(map[string]manifestFile, len(m.Files))
		for _, file := range m.Files {
			files[file.Path] = file
		}
		if files[datastarPath].SHA256 != datastarSHA256 {
			return manifest{}, errors.New("Datastar morph runtime does not match the pinned v1.0.3 bundle")
		}
		if files[datastarLicense].SHA256 != licenseSHA256 {
			return manifest{}, errors.New("Datastar morph license does not match the adjacent MIT license")
		}
	}
	return m, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("manifest must contain exactly one JSON value")
	}
	return nil
}

func validAssetPath(name string) bool {
	if name == "" || name != path.Clean(name) || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "//") {
		return false
	}
	for _, character := range []byte(name) {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '/' || character == '-' || character == '_' || character == '.') {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return false
		}
	}
	switch path.Ext(name) {
	case ".js", ".css", ".woff2", ".png", ".jpg", ".txt":
		return true
	default:
		return false
	}
}

func validSHA(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func readRootRegular(root *os.Root, name string, limit int64) ([]byte, error) {
	if name != "manifest.json" && !validAssetPath(name) {
		return nil, errors.New("invalid relative asset path")
	}
	parts := strings.Split(name, "/")
	for index := range parts {
		candidate := strings.Join(parts[:index+1], "/")
		info, err := root.Lstat(candidate)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("missing path or symlink refused")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, errors.New("asset parent is not a directory")
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, errors.New("asset is not a regular file")
		}
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, errors.New("open regular file")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("regular file exceeds its byte limit")
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, errors.New("read exceeds its byte limit")
	}
	return body, nil
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func assetContentType(name string) string {
	switch path.Ext(name) {
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".woff2":
		return "font/woff2"
	case ".png":
		return "image/png"
	case ".jpg":
		return "image/jpeg"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		panic("validated asset has unsupported extension")
	}
}

func newEditorHost(authority string, m manifest, assets map[string]pinnedAsset) (*editorHost, error) {
	if authority == "" || strings.ContainsAny(authority, " /\\\t\r\n@?#%") {
		return nil, errors.New("invalid bound authority")
	}
	boundHost, boundPort, err := net.SplitHostPort(authority)
	portNumber, portErr := strconv.Atoi(boundPort)
	boundIP := net.ParseIP(boundHost)
	if err != nil || portErr != nil || strconv.Itoa(portNumber) != boundPort || portNumber < 1 || portNumber > 65535 || boundIP == nil || !boundIP.IsLoopback() {
		return nil, errors.New("bound authority must be a canonical loopback address and port")
	}
	h := &editorHost{
		authority: authority,
		origin:    "http://" + authority,
		context:   []byte(`{"version":1,"instanceId":"primary","label":"Updated product description","hint":"Context updated. Your local draft is unchanged."}`),
		assets:    assets,
	}
	enabled := morphEnabled(m)
	if enabled {
		for _, required := range []struct {
			path string
			sha  string
		}{{datastarPath, datastarSHA256}, {morphClientPath, ""}, {datastarLicense, licenseSHA256}} {
			asset, ok := assets["/"+required.path]
			if !ok || (required.sha != "" && asset.sha256 != required.sha) {
				return nil, errors.New("pinned Datastar morph asset set is unavailable")
			}
		}
		h.morph = []byte(`<section id="host-fragment"><p id="fragment-state">Updated surrounding context.</p><div id="editor-slot" data-ignore-morph></div></section>`)
	}
	h.html = renderHTML(m, enabled)
	return h, nil
}

func morphEnabled(m manifest) bool {
	for _, file := range m.Files {
		if file.Path == datastarPath {
			return true
		}
	}
	return false
}

func renderHTML(m manifest, morph bool) []byte {
	var document strings.Builder
	document.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>Client-owned editor</title>")
	for _, style := range m.Styles {
		document.WriteString(`<link rel="stylesheet" href="/` + html.EscapeString(style) + `">`)
	}
	document.WriteString(`</head><body><main><h1>Client-owned editor</h1><p>Local draft only. Nothing is saved.</p><noscript>This editor requires JavaScript. Nothing is saved.</noscript><div><button id="host-update" type="button">Update context</button><button id="host-dispose" type="button">Dispose editor</button><button id="host-mount" type="button">Mount editor</button>`)
	if morph {
		document.WriteString(`<button id="host-morph" type="button">Update surrounding context</button>`)
	}
	document.WriteString(`</div>`)
	if morph {
		document.WriteString(`<form id="morph-transport-form" hidden></form><section id="host-fragment"><p id="fragment-state">Initial surrounding context.</p><div id="editor-slot" data-ignore-morph></div></section>`)
	} else {
		document.WriteString(`<div id="editor-slot"></div>`)
	}
	document.WriteString(`<output id="host-state" aria-live="polite"></output><div id="secondary-slot"></div><script type="application/json" id="editor-config">{"version":1,"instanceId":"primary","label":"Product description","hint":"Local draft only. Nothing is saved.","initialText":"Start typing..."}</script><script type="module" src="/` + html.EscapeString(m.Entry) + `"></script>`)
	if morph {
		document.WriteString(`<script type="module" src="/assets/morph-client.js"></script>`)
	}
	document.WriteString(`</main></body></html>`)
	return []byte(document.String())
}

func (h *editorHost) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setHostHeaders(w.Header())
	if !h.requestAllowed(r) {
		writeResponse(w, r, http.StatusForbidden, "text/plain; charset=utf-8", []byte("request refused\n"), "")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeResponse(w, r, http.StatusMethodNotAllowed, "text/plain; charset=utf-8", []byte("method not allowed\n"), "")
		return
	}
	if r.URL == nil || r.URL.IsAbs() || r.URL.Host != "" || r.URL.Fragment != "" || r.URL.RawQuery != "" || r.URL.EscapedPath() != r.URL.Path {
		writeResponse(w, r, http.StatusNotFound, "text/plain; charset=utf-8", []byte("not found\n"), "")
		return
	}
	switch r.URL.Path {
	case "/":
		writeResponse(w, r, http.StatusOK, "text/html; charset=utf-8", h.html, "")
	case "/context":
		writeResponse(w, r, http.StatusOK, "application/json; charset=utf-8", h.context, "")
	case "/morph-fragment":
		if h.morph == nil {
			writeResponse(w, r, http.StatusNotFound, "text/plain; charset=utf-8", []byte("not found\n"), "")
			return
		}
		w.Header().Set("Datastar-Selector", "#host-fragment")
		w.Header().Set("Datastar-Mode", "outer")
		writeResponse(w, r, http.StatusOK, "text/html; charset=utf-8", h.morph, "")
	default:
		asset, ok := h.assets[r.URL.Path]
		if !ok {
			writeResponse(w, r, http.StatusNotFound, "text/plain; charset=utf-8", []byte("not found\n"), "")
			return
		}
		writeResponse(w, r, http.StatusOK, asset.contentType, asset.body, `"sha256-`+asset.sha256+`"`)
	}
}

func (h *editorHost) requestAllowed(r *http.Request) bool {
	if r.Host != h.authority {
		return false
	}
	origins, hasOrigin := r.Header["Origin"]
	if hasOrigin && (len(origins) != 1 || origins[0] != h.origin) {
		return false
	}
	sites, hasSite := r.Header["Sec-Fetch-Site"]
	return !hasSite || (len(sites) == 1 && (sites[0] == "same-origin" || sites[0] == "none"))
}

func setHostHeaders(header http.Header) {
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", hostCSP)
	header.Set("Cross-Origin-Opener-Policy", "same-origin")
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}

func writeResponse(w http.ResponseWriter, r *http.Request, status int, contentType string, body []byte, etag string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}
