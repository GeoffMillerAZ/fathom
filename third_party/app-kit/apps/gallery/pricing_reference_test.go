package gallery

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func TestPricingReviewedDescriptorIsConcreteAndRefusesPromotion(t *testing.T) {
	d, err := pricingReferenceDescriptor()
	if err != nil || len(d.Receipt.Documents) != 4 || d.Receipt.Verification != "not-verified" || d.Receipt.Scope != "controlled-static-reference" {
		t.Fatal("reviewed metadata invalid", err)
	}
	for _, mutation := range [][2]string{{`"not-verified"`, `"verified"`}, {`"InterVariable"`, `"InventedPortalFont"`}, {`"original-light.html"`, `"arbitrary-upload.html"`}} {
		src := strings.Replace(string(pricingReferenceCUE), mutation[0], mutation[1], 1)
		if src == string(pricingReferenceCUE) {
			t.Fatal("canary did not mutate metadata", mutation)
		}
		v := cuecontext.New().CompileString(src).LookupPath(cue.ParsePath("pricingReference"))
		if v.Validate(cue.Concrete(true), cue.Final()) == nil {
			t.Fatal("invalid prepared metadata admitted", mutation)
		}
	}
}

func pricingFixture(t *testing.T) (string, pricingDescriptor, map[string][]byte) {
	t.Helper()
	root, catalog := conversionFixture(t)
	entry := conversionEntry(t, catalog, "translated")
	d := pricingDescriptor{Receipt: pricingReceipt{
		SchemaVersion: 1, CaseID: "P02-pricing", Scope: "controlled-static-reference",
		Source: entry.Source, AttemptID: entry.ID, ArtifactSHA256: entry.ArtifactSHA256,
		Transformer: entry.Transformer, Verification: "not-verified", Documents: map[string]pricingFilePin{},
	}}
	bodies := map[string][]byte{}
	style := "body { color: navy; }"
	for _, side := range []string{"original", "converted"} {
		for _, theme := range []string{"light", "dark"} {
			name := side + "-" + theme + ".html"
			body := []byte("<!doctype html><html lang=\"en\" class=\"" + theme + "\"><head><style>" + style + "</style></head><body><p>Synthetic comparison</p></body></html>")
			bodies[name] = body
			d.Receipt.Documents[name] = pricingFilePin{SHA256: fmt.Sprintf("%x", sha256.Sum256(body)), Size: len(body), StyleSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(style)))}
		}
	}
	receipt, err := json.Marshal(d.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	d.ReceiptSHA256, d.ReceiptSize = fmt.Sprintf("%x", sha256.Sum256(receipt)), len(receipt)
	dir := filepath.Join(root, "previews", "p02-pricing", d.ReceiptSHA256)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range bodies {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "receipt.json"), receipt, 0600); err != nil {
		t.Fatal(err)
	}
	return root, d, bodies
}

func pricingHandler(root string, d pricingDescriptor) http.Handler {
	mux := http.NewServeMux()
	registerPricingReferenceWith(root, privateReferenceRegistrar(mux), d, nil)
	return mux
}

func TestPricingReferenceMatchedFramesAndPrivateDocuments(t *testing.T) {
	root, d, bodies := pricingFixture(t)
	before := conversionStoreSnapshot(t, root)
	h := pricingHandler(root, d)
	for _, width := range []string{"390", "1440"} {
		for _, theme := range []string{"light", "dark"} {
			w := referenceRequest(h, "GET", pricingReferenceRoute+"?theme=dark&viewport="+width+"&preview_theme="+theme, nil)
			if w.Code != 200 {
				t.Fatalf("comparison: status=%d body=%s", w.Code, w.Body.String())
			}
			body := w.Body.String()
			for _, want := range []string{`name="theme" value="dark"`, `width="` + width + `" height="1000"`, `original-` + theme + `.html`, `converted-` + theme + `.html`, "not an exact portal baseline", "not perform a purchase"} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Count(body, `<iframe `) != 2 || strings.Count(body, `sandbox=""`) != 2 || strings.Contains(body, "allow-scripts") || strings.Contains(body, "allow-same-origin") || strings.Count(body, "<main") != 1 {
				t.Fatal("comparison structure or sandbox changed")
			}
			if w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-src 'self'") {
				t.Fatal("outer frame policy changed")
			}
		}
	}
	for name, body := range bodies {
		w := referenceRequest(h, "GET", pricingReferenceRoute+"/document/"+name, nil)
		styleHash := sha256.Sum256([]byte("body { color: navy; }"))
		if w.Code != 200 || !reflect.DeepEqual(w.Body.Bytes(), body) || w.Header().Get("X-Prepared-Document-SHA256") != d.Receipt.Documents[name].SHA256 {
			t.Fatal("document identity changed", name)
		}
		for _, want := range []string{"default-src 'none'", "script-src 'none'", "font-src data:", "form-action 'none'", "frame-ancestors 'self'", "; sandbox", "style-src 'sha256-" + base64.StdEncoding.EncodeToString(styleHash[:]) + "'"} {
			if !strings.Contains(w.Header().Get("Content-Security-Policy"), want) {
				t.Errorf("missing CSP directive %s", want)
			}
		}
		if w.Header().Get("X-Frame-Options") != "SAMEORIGIN" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Conversion-Verification") != "not-verified" || w.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Fatal("document privacy/evidence headers changed")
		}
	}
	if !reflect.DeepEqual(before, conversionStoreSnapshot(t, root)) {
		t.Fatal("viewing mutated private inputs")
	}
}

func TestPricingReferenceRefusalsKeepFrameDenial(t *testing.T) {
	root, d, _ := pricingFixture(t)
	h := pricingHandler(root, d)
	for _, tc := range []struct {
		path, method string
		headers      map[string]string
		status       int
	}{
		{pricingReferenceRoute + "?viewport=400", "GET", nil, 400},
		{pricingReferenceRoute + "?viewport=390&viewport=1440", "GET", nil, 400},
		{pricingReferenceRoute + "?preview_theme=", "GET", nil, 400},
		{pricingReferenceRoute + "?x=1", "GET", nil, 400},
		{pricingReferenceRoute + "?theme=%zz", "GET", nil, 400},
		{pricingReferenceRoute + "/document/original-light.html?theme=dark", "GET", nil, 400},
		{pricingReferenceRoute + "/document/%6friginal-light.html", "GET", nil, 400},
		{pricingReferenceRoute + "/document/unknown.html", "GET", nil, 400},
		{pricingReferenceRoute, "POST", nil, 405},
		{pricingReferenceRoute, "GET", map[string]string{"Origin": "https://example.com"}, 403},
		{pricingReferenceRoute, "GET", map[string]string{"Host": "example.com"}, 403},
	} {
		w := referenceRequest(h, tc.method, tc.path, tc.headers)
		if w.Code != tc.status {
			t.Errorf("%s %s: %d, want %d", tc.method, tc.path, w.Code, tc.status)
		}
		assertPricingDenied(t, w)
	}
	r := httptest.NewRequest("GET", pricingReferenceRoute, nil)
	r.Host, r.RemoteAddr = "127.0.0.1:8765", "192.0.2.1:54321"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("non-loopback peer admitted")
	}
	assertPricingDenied(t, w)
}

func assertPricingDenied(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-src 'self'") || strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'sha256-") {
		t.Fatal("error weakened frame policy")
	}
}

func TestPricingReferenceReceiptBindsAllAdmittedMetadata(t *testing.T) {
	for _, field := range []string{"source", "artifact", "transformer", "document", "style", "configuration", "scope", "verification", "receipt-bytes", "document-bytes", "missing-document", "symlink-document"} {
		t.Run(field, func(t *testing.T) {
			root, d, _ := pricingFixture(t)
			dir := filepath.Join(root, "previews", "p02-pricing", d.ReceiptSHA256)
			doc := filepath.Join(dir, "original-light.html")
			switch field {
			case "source":
				d.Receipt.Source.Path = "different.html"
			case "artifact":
				d.Receipt.ArtifactSHA256 = strings.Repeat("0", 64)
			case "transformer":
				d.Receipt.Transformer.ID = "different"
			case "document", "style":
				pin := d.Receipt.Documents["original-light.html"]
				if field == "document" {
					pin.Size++
				} else {
					pin.StyleSHA256 = strings.Repeat("0", 64)
				}
				d.Receipt.Documents["original-light.html"] = pin
			case "configuration":
				d.Receipt.Configuration.FontFamily = "different"
			case "scope":
				d.Receipt.Scope = "certified"
			case "verification":
				d.Receipt.Verification = "verified"
			case "receipt-bytes":
				if err := os.WriteFile(filepath.Join(dir, "receipt.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			case "document-bytes":
				body, err := os.ReadFile(doc)
				if err != nil {
					t.Fatal(err)
				}
				body[len(body)-1] ^= 1
				if err := os.WriteFile(doc, body, 0600); err != nil {
					t.Fatal(err)
				}
			case "missing-document", "symlink-document":
				if err := os.Remove(doc); err != nil {
					t.Fatal(err)
				}
				if field == "symlink-document" {
					if err := os.Symlink(filepath.Join(dir, "converted-light.html"), doc); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, path := range []string{pricingReferenceRoute, pricingReferenceRoute + "/document/original-light.html"} {
				w := referenceRequest(pricingHandler(root, d), "GET", path, nil)
				if w.Code != 409 {
					t.Fatalf("%s: changed fixture admitted: %d", path, w.Code)
				}
				assertPricingDenied(t, w)
				if !strings.Contains(w.Body.String(), "Prepared comparison unavailable") {
					t.Fatal("missing actionable recovery")
				}
			}
		})
	}
}

func TestPricingReadFileRefusesSizeAndType(t *testing.T) {
	root, d, _ := pricingFixture(t)
	r, err := os.OpenRoot(filepath.Join(root, "previews", "p02-pricing", d.ReceiptSHA256))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	pin := d.Receipt.Documents["original-light.html"]
	for _, limit := range []int{0, pin.Size - 1} {
		if _, err := readPricingFile(r, "original-light.html", pin, limit); err == nil {
			t.Fatal("size limit ignored")
		}
	}
	if _, err := readPricingFile(r, ".", pin, 1<<20); err == nil {
		t.Fatal("directory accepted")
	}
}
