package designreceipts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestNavbarOverlayEvidencePrepare creates one private retrospective packet.
// It does not publish to the evidence store. Every input and output root is an
// explicit maintainer selection; ordinary package tests skip this preparation.
func TestNavbarOverlayEvidencePrepare(t *testing.T) {
	out, requested := os.LookupEnv("APPKIT_NAVBAR_OVERLAY_OUT")
	if !requested {
		t.Skip("set explicit private output and reviewed P09 input roots")
	}
	info, err := os.Lstat(out)
	if !filepath.IsAbs(out) || err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		t.Fatal("existing private 0700 output root required")
	}
	roots := map[string]string{
		"kit":             os.Getenv("APPKIT_NAVBAR_OVERLAY_KIT"),
		"runtime":         os.Getenv("APPKIT_NAVBAR_OVERLAY_RUNTIME"),
		"navbar":          os.Getenv("APPKIT_NAVBAR_OVERLAY_BASE"),
		"runtime-proof":   os.Getenv("APPKIT_NAVBAR_OVERLAY_RUNTIME_PROOF"),
		"delivery":        os.Getenv("APPKIT_NAVBAR_OVERLAY_DELIVERY"),
		"delivery-parent": os.Getenv("APPKIT_NAVBAR_OVERLAY_DELIVERY_PARENT"),
		"delivery-run":    os.Getenv("APPKIT_NAVBAR_OVERLAY_DELIVERY_RUN"),
	}
	read := func(root, name string) ([]byte, error) { return readOverlayFile(roots[root], name) }

	body, summary, err := buildOverlayBundle(navbarOverlaySpec(), read)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Name != "navbar" || summary.Images != 114 {
		t.Fatalf("unexpected export summary: %+v", summary)
	}
	again, againSummary, err := buildOverlayBundle(navbarOverlaySpec(), read)
	if err != nil || string(again) != string(body) || againSummary != summary {
		t.Fatalf("preparation is not deterministic: %v", err)
	}

	r, err := os.OpenRoot(out)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	f, err := r.OpenFile("navbar.json", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := f.Write(body)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("write: %v; close: %v", writeErr, closeErr)
	}
	meta, _ := json.Marshal(summary)
	t.Log(string(meta))
}
