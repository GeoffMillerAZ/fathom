package gallery

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designevidence"
	"github.com/geoffmilleraz/app-kit/internal/designreceipts"
)

func evidenceImageFixture(t *testing.T) (string, designevidence.Entry, map[string][]byte) {
	t.Helper()
	root, catalog := evidenceFixture(t)
	detail, err := designevidence.Read(root, catalog.List()[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	source, err := designassets.ReadFile(root, detail.Entry.Recorded.Subject.SourceArchive, detail.Entry.SourceMember)
	if err != nil {
		t.Fatal(err)
	}
	content := map[string][]byte{"source": []byte(source.Content), "output": []byte("PRIVATE_OUTPUT_BODY_NOT_FOR_METADATA"), "support": []byte("synthetic tooling"), "observation": []byte("an assertion, not authenticated proof")}
	for _, id := range []string{"capture-a", "capture-b"} {
		im := image.NewNRGBA(image.Rect(0, 0, 5, 3))
		im.Set(0, 0, color.NRGBA{R: uint8(len(id)), G: 120, A: 255})
		var out bytes.Buffer
		if err := png.Encode(&out, im); err != nil {
			t.Fatal(err)
		}
		content[id] = out.Bytes()
	}
	content["unsupported-image"] = []byte(`<svg onload="notExecuted()"></svg>`)
	c, r := detail.Contract, detail.Receipt
	c.Subject.ID = "reference.synthetic-images"
	gate, result := c.Gates["visual"], r.Results["visual"]
	for _, id := range []string{"capture-a", "capture-b", "unsupported-image"} {
		gate.Slots = append(gate.Slots, designreceipts.Slot{ID: id, Role: "image"})
		result.Evidence = append(result.Evidence, designreceipts.Evidence{Asset: designreceipts.Asset{ID: id, SHA256: designreceipts.Digest(content[id]), Bytes: len(content[id])}, Role: "image"})
	}
	c.Gates["visual"], r.Results["visual"] = gate, result
	cb, err := designreceipts.EncodeContract(c)
	if err != nil {
		t.Fatal(err)
	}
	r.ContractSHA256 = designreceipts.Digest(cb)
	rb, err := designreceipts.EncodeReceipt(r)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := designevidence.Publish(root, detail.Entry.SourceMember, cb, rb, designreceipts.Expected{ContractSHA256: r.ContractSHA256, SourceRevision: r.SourceRevision, ArtifactSHA256: r.ArtifactSHA256}, content)
	if err != nil {
		t.Fatal(err)
	}
	return root, entry, content
}

func TestTargetEvidenceImagesExactPrivateBytesAndMetadata(t *testing.T) {
	root, entry, content := evidenceImageFixture(t)
	h := referenceHandler(root)
	before := conversionStoreSnapshot(t, root)
	base := "/target-evidence/" + entry.ID
	for _, id := range []string{"capture-a", "capture-b"} {
		w := referenceRequest(h, "GET", base+"/images/"+id, nil)
		if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), content[id]) {
			t.Fatalf("PNG identity: %d", w.Code)
		}
		if w.Header().Get("Content-Security-Policy") != evidencePNGDocumentCSP {
			t.Fatal("PNG document policy")
		}
		for key, want := range map[string]string{"Content-Type": "image/png", "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY", "Cross-Origin-Resource-Policy": "same-origin", "X-Appkit-Evidence-SHA256": designreceipts.Digest(content[id]), "X-Appkit-Verification": "not-verified"} {
			if w.Header().Get(key) != want {
				t.Fatalf("%s: %q", key, w.Header().Get(key))
			}
		}
	}
	for _, path := range []string{"/api/target-evidence", "/api/target-evidence/" + entry.ID} {
		w := referenceRequest(h, "GET", path, nil)
		if w.Code != 200 {
			t.Fatalf("metadata: %d", w.Code)
		}
		if strings.Contains(w.Body.String(), base64.StdEncoding.EncodeToString(content["capture-a"])) || strings.Contains(w.Body.String(), "PRIVATE_OUTPUT_BODY_NOT_FOR_METADATA") {
			t.Fatal("metadata leaked content")
		}
	}
	page := referenceRequest(h, "GET", base+"?theme=dark&image=capture-b&compare=capture-a&subject_id=reference.synthetic-images", nil)
	if page.Code != 200 {
		t.Fatalf("detail: %d %s", page.Code, page.Body.String())
	}
	for _, want := range []string{"Retained captures", "Compare below (optional)", "Show captures", "not live components", `name="theme" value="dark"`, `name="subject_id" value="reference.synthetic-images"`, `width="5" height="3"`, base + "/images/capture-b", base + "/images/capture-a"} {
		if !strings.Contains(page.Body.String(), want) {
			t.Fatalf("detail missing %q", want)
		}
	}
	if page.Header().Get("Content-Security-Policy") != evidenceImagesCSP {
		t.Fatal("viewer CSP")
	}
	if strings.Index(page.Body.String(), `src="`+base+`/images/capture-b"`) > strings.Index(page.Body.String(), `src="`+base+`/images/capture-a"`) {
		t.Fatal("comparison order changed")
	}
	if !reflect.DeepEqual(before, conversionStoreSnapshot(t, root)) {
		t.Fatal("viewer changed private store")
	}
}

func TestTargetEvidenceImagesRefuseUnrelatedBytesAndUnsafeRequests(t *testing.T) {
	root, entry, _ := evidenceImageFixture(t)
	h := referenceHandler(root)
	base := "/target-evidence/" + entry.ID
	path := base + "/images/capture-a"
	for _, method := range []string{"POST", "HEAD", "PUT", "DELETE"} {
		if w := referenceRequest(h, method, path, nil); w.Code != 405 {
			t.Fatalf("method %s: %d", method, w.Code)
		}
	}
	for _, headers := range []map[string]string{{"Origin": "https://example.invalid"}, {"Sec-Fetch-Site": "cross-site"}, {"Host": "example.invalid"}} {
		if w := referenceRequest(h, "GET", path, headers); w.Code != 403 || w.Header().Get("Content-Type") == "image/png" {
			t.Fatalf("private boundary: %d", w.Code)
		}
	}
	for _, id := range []string{"source", "output", "support", "observation", "missing"} {
		if w := referenceRequest(h, "GET", base+"/images/"+id, nil); w.Code != 404 {
			t.Fatalf("unrelated %s: %d", id, w.Code)
		}
	}
	for _, query := range []string{"unknown=x", "theme=purple", "theme=dark&theme=light", "image=capture-a"} {
		if w := referenceRequest(h, "GET", path+"?"+query, nil); w.Code != 400 {
			t.Fatalf("image query %s: %d", query, w.Code)
		}
	}
	for _, suffix := range []string{"?image=missing", "?compare=output", "?image=../source", "?image=a&image=b"} {
		w := referenceRequest(h, "GET", base+suffix, nil)
		if w.Code != 400 || strings.Contains(w.Header().Get("Content-Security-Policy"), "img-src") {
			t.Fatalf("selection %s: %d", suffix, w.Code)
		}
	}
	for _, path := range []string{"/api/target-evidence?image=capture-a", "/target-evidence?compare=capture-a", "/api/target-evidence/" + entry.ID + "?image=capture-a", base + "/images/capture%2Da"} {
		if w := referenceRequest(h, "GET", path, nil); w.Code != 400 {
			t.Fatalf("scope/alias %s: %d", path, w.Code)
		}
	}
	if w := referenceRequest(h, "GET", base+"/images/unsupported-image", nil); w.Code != 500 || strings.Contains(w.Body.String(), "<svg") || strings.Contains(w.Header().Get("Content-Security-Policy"), "unsafe-inline") {
		t.Fatal("unsupported bytes served")
	}
	w := referenceRequest(h, "GET", base+"?image=unsupported-image", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "capture could not pass") || !strings.Contains(w.Body.String(), designreceipts.Digest([]byte(`<svg onload="notExecuted()"></svg>`))) || !strings.Contains(w.Body.String(), "Nine recorded gates") || strings.Contains(w.Body.String(), `src="`+base+`/images/unsupported-image"`) {
		t.Fatal("bad image prevented metadata recovery or was displayed")
	}
	for _, path := range []string{"/references", "/target-evidence", "/api/target-evidence/" + entry.ID} {
		if w := referenceRequest(h, "GET", path, nil); strings.Contains(w.Header().Get("Content-Security-Policy"), "img-src") {
			t.Fatal("unrelated CSP expanded")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "evidence", "entries", strings.TrimPrefix(entry.ID, "evidence@")+".json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if w := referenceRequest(h, "GET", path, nil); w.Code != 500 || w.Header().Get("Content-Type") == "image/png" {
		t.Fatalf("tamper: %d", w.Code)
	}
}

func TestCatalogReferencesLinkExactPrivateEvidenceWithoutChangingProfiles(t *testing.T) {
	h := New()
	for _, id := range []string{"hero", "pricing", "faq", "directory", "form", "navigation", "menu", "drawer"} {
		capID := "reference." + id + "-adapter"
		w := referenceRequest(h, http.MethodGet, "/catalog/"+capID+"?theme=dark", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "/target-evidence?subject_id="+url.QueryEscape(capID)+"&amp;theme=dark") {
			t.Fatalf("catalog %s: %d", id, w.Code)
		}
		if !strings.Contains(w.Body.String(), "Records may be absent") || strings.Contains(w.Body.String(), "Open live specimen") {
			t.Fatal("reference promoted to live specimen")
		}
	}
}
