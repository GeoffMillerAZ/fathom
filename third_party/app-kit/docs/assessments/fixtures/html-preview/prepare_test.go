package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

func TestExactDOMOracle(t *testing.T) {
	const original = `<section class="bg-white dark:bg-gray-900"><!--kept--><h2>Exact text</h2><svg viewBox="0 0 20 20"><path d="M1 2"/></svg></section>`
	if !sameDOM(original, original) {
		t.Fatal("identical DOM refused")
	}
	for name, mutated := range map[string]string{
		"text":       strings.Replace(original, "Exact text", "Changed text", 1),
		"dark-class": strings.Replace(original, " dark:bg-gray-900", "", 1),
		"comment":    strings.Replace(original, "<!--kept-->", "", 1),
		"namespace":  strings.ReplaceAll(strings.ReplaceAll(original, "<svg ", "<div "), "</svg>", "</div>"),
		"whitespace": strings.Replace(original, "Exact text", "Exact  text", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if sameDOM(original, mutated) {
				t.Fatal("planted DOM mismatch accepted")
			}
		})
	}
}

func TestPreparedTargetMatchesActualTranslator(t *testing.T) {
	body := []byte("<section><p>Synthetic pricing</p></section>\n")
	source := designconvert.Source{ArchiveID: "marketing@" + strings.Repeat("a", 64), Path: "html/pricing.html", SHA256: digest(body), Size: len(body)}
	translation, err := designconvert.Translate(source, body)
	if err != nil || translation.Status != "translated" || translation.Target != preparedTarget {
		t.Fatalf("preparer target disagrees with translator: target=%q status=%q err=%v", translation.Target, translation.Status, err)
	}
}

func TestFragmentResourceRefusals(t *testing.T) {
	for _, body := range []string{`<div><a href="#">Choose</a><svg data-slot="icon" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><path fill="currentColor" d="M1 2"/></svg></div>`, `<p class="dark:bg-gray-900">Static</p>`} {
		if err := safeFragment(body); err != nil {
			t.Fatal("synthetic static fragment refused", err)
		}
	}
	for _, body := range []string{
		`<a href="https://example.test/">External</a>`, `<a href="//example.test/">External</a>`, `<a href="/local">Navigation</a>`,
		`<a href="javascript:alert(1)">Execute</a>`, `<a href="#" ping="https://example.test/">Ping</a>`,
		`<img src="https://example.test/i">`, `<script>alert(1)</script>`, `<iframe srcdoc="hello"></iframe>`,
		`<form action="#"><button>Submit</button></form>`, `<meta http-equiv="refresh" content="0;url=https://example.test/">`,
		`<link rel="stylesheet" href="https://example.test/a.css">`, `<style>@import 'https://example.test/a.css';</style>`,
		`<p onclick="alert(1)">Handler</p>`, `<p style="background:url(https://example.test/i)">Resource</p>`,
		`<svg><use href="#x"/></svg>`, `<svg><path fill="url(https://example.test/i)"/></svg>`,
		`<div class="bg-[url(https://example.test/i)]">CSS resource</div>`, `<!doctype html><p>Document</p>`,
	} {
		if safeFragment(body) == nil {
			t.Fatal("active/resource-bearing fragment accepted")
		}
	}
}

func TestDocumentBoundsAndWrapper(t *testing.T) {
	const original = `<div class="dark:bg-gray-900"><a href="#">Choose</a></div>`
	const style = "body{font-family:InterVariable}"
	a, err := document(original, style, "light")
	if err != nil {
		t.Fatal(err)
	}
	b, err := document(original, style, "light")
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("identical wrapper inputs differ")
	}
	if !bytes.Contains(a, []byte("<body>"+original+"</body>")) || !bytes.Contains(a, []byte("default-src 'none'")) || !bytes.Contains(a, []byte("form-action 'none'")) {
		t.Fatal("body or CSP changed")
	}
	if _, err := document(original, style, "system"); err == nil {
		t.Fatal("implicit theme accepted")
	}
	if _, err := document(original, strings.Repeat("x", maxDocumentBytes), "dark"); err == nil {
		t.Fatal("total document bound not enforced")
	}
	if _, err := document(original, "</style><script>bad</script>", "dark"); err == nil {
		t.Fatal("style boundary accepted")
	}
	if _, err := document(`<form></form>`, style, "light"); err == nil {
		t.Fatal("active document body accepted")
	}
	out := &boundedBuffer{limit: 4}
	if _, err := out.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := out.Write([]byte("5")); err == nil || out.Len() != 4 {
		t.Fatal("stdout bound failed")
	}
	copyOut := &boundedBuffer{limit: 4}
	if _, err := io.Copy(copyOut, io.LimitReader(strings.NewReader("12345"), 5)); err == nil || copyOut.Len() > 4 {
		t.Fatal("io.Copy bypassed stdout bound through an embedded ReaderFrom")
	}
}

func TestPrivateBuildEnvironment(t *testing.T) {
	t.Setenv("GOCACHE", "/outside/cache")
	t.Setenv("GOCACHEPROG", "external-cache-program")
	t.Setenv("GOENV", "/outside/goenv")
	t.Setenv("GONOPROXY", "*")
	scratch := t.TempDir()
	env, err := privateEnv(scratch)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, item := range env {
		key, value, _ := strings.Cut(item, "=")
		if _, ok := values[key]; ok {
			t.Fatal("duplicate child environment key")
		}
		values[key] = value
	}
	for _, key := range []string{"GOCACHE", "GOTMPDIR", "TMPDIR"} {
		if values[key] != filepath.Join(scratch, strings.ToLower(key)) {
			t.Fatal("ambient cache/temp path survived")
		}
		info, err := os.Stat(values[key])
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal("child cache is not private")
		}
	}
	for key, want := range map[string]string{"GOENV": "off", "GOCACHEPROG": "", "GOPROXY": "off", "GONOPROXY": "none", "GOVCS": "*:off", "GOWORK": "off", "CGO_ENABLED": "0", "GO_TELEMETRY_CHILD": "", "GO_TELEMETRY_CHILD_UPLOAD": "", "TEST_TELEMETRY_DIR": filepath.Join(scratch, "telemetry")} {
		if values[key] != want {
			t.Fatal("child execution boundary drifted", key)
		}
	}
}

func TestEffectivePrivateTelemetryMode(t *testing.T) {
	// Simulate inherited upload-enabled telemetry using only synthetic paths.
	// Reading an owner's mode is neither necessary nor permitted for this check.
	inherited := t.TempDir()
	if err := os.WriteFile(filepath.Join(inherited, "mode"), []byte("on\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_TELEMETRY_DIR", inherited)
	t.Setenv("GOTELEMETRY", "on")
	t.Setenv("GO_TELEMETRY_CHILD", "1")
	t.Setenv("GO_TELEMETRY_CHILD_UPLOAD", "1")
	scratch := t.TempDir()
	env, err := privateEnv(scratch)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := run(ctx, scratch, env, "effective-telemetry-mode", "go", "env", "-json", "GOTELEMETRY", "GOTELEMETRYDIR")
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]string
	if json.Unmarshal(out, &actual) != nil || actual["GOTELEMETRY"] != "off" || actual["GOTELEMETRYDIR"] != filepath.Join(scratch, "telemetry") {
		t.Fatal("Go subprocess did not use private effective off mode")
	}
	if _, err := run(ctx, scratch, env, "effective-tool-telemetry", "go", "tool", "compile", "-V"); err != nil {
		t.Fatal(err)
	}
	for dir, want := range map[string]string{inherited: "on\n", filepath.Join(scratch, "telemetry"): "off\n"} {
		entries, err := os.ReadDir(dir)
		mode, readErr := os.ReadFile(filepath.Join(dir, "mode"))
		info, statErr := os.Stat(filepath.Join(dir, "mode"))
		if err != nil || readErr != nil || statErr != nil || len(entries) != 1 || entries[0].Name() != "mode" || string(mode) != want || info.Mode().Perm() != 0600 {
			t.Fatal("telemetry command created counters/reports or changed a synthetic mode")
		}
	}
}

func TestPrivatePublicationNoClobber(t *testing.T) {
	root := t.TempDir()
	if os.MkdirAll(filepath.Join(root, "data/design-assets"), 0700) != nil {
		t.Fatal("synthetic root")
	}
	files := map[string][]byte{"receipt.json": []byte("{}\n"), "original-light.html": []byte("synthetic")}
	hash := digest(files["receipt.json"])
	dir, err := publish(root, hash, files)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := publish(root, hash, files); err != nil || again != dir {
		t.Fatal("identical publication was not a no-op")
	}
	changed := map[string][]byte{"receipt.json": files["receipt.json"], "original-light.html": []byte("changed")}
	if _, err := publish(root, hash, changed); err == nil {
		t.Fatal("conflicting immutable output replaced")
	}
	body, err := os.ReadFile(filepath.Join(dir, "original-light.html"))
	if err != nil || string(body) != "synthetic" {
		t.Fatal("conflict changed original output")
	}
	if os.WriteFile(filepath.Join(dir, "extra.html"), []byte("extra"), 0600) != nil {
		t.Fatal("plant closure mismatch")
	}
	if verifyExisting(dir, files) == nil {
		t.Fatal("unexpected bundle file accepted")
	}
}

func TestSyntheticPinnedCompileAndDOM(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	env, err := privateEnv(scratch)
	if err != nil {
		t.Fatal(err)
	}
	const original = `<section class="bg-white dark:bg-gray-900"><!--kept--><p>Exact text &amp; spacing</p><a href="#">Choose</a><svg viewBox="0 0 20 20"><path d="M1 2"/></svg></section>`
	translation, err := designconvert.HTMLToTempl([]byte(original))
	if err != nil || translation.Templ == "" {
		t.Fatal("synthetic literal rejected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	out, err := compileLiteral(ctx, root, scratch, env, translation.Templ, original)
	if err != nil || !sameDOM(original, out) {
		t.Fatal("synthetic pinned compile/DOM failed", err)
	}
	if _, err := compileLiteral(ctx, root, scratch, env, translation.Templ, strings.Replace(original, "Exact text", "Wrong text", 1)); err == nil || !strings.Contains(err.Error(), "DOM-unformatted") {
		t.Fatal("compiled output escaped independent original oracle")
	}
}

func TestSyntheticCachedCSS(t *testing.T) {
	bin, _, err := cachedTailwind()
	if err != nil {
		if strings.HasSuffix(err.Error(), ": cached-tailwind-missing") {
			t.Skip("cached pinned Tailwind absent; no download attempted")
		}
		t.Fatal(err)
	}
	scratch := t.TempDir()
	env, err := privateEnv(scratch)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	css, err := compileCSS(ctx, scratch, env, bin, `<div class="bg-white dark:bg-gray-900 text-indigo-600 p-8">Synthetic</div>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{".bg-white", ".dark\\:bg-gray-900", ".dark *", "InterVariable"} {
		if !strings.Contains(string(css), required) {
			t.Fatal("fixture CSS lost explicit class/theme/font contract", required)
		}
	}
}
