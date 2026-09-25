package gallery

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"regexp"
	"strings"
	"testing"
)

func TestPricingComparisonWorkspaceModesAndFallback(t *testing.T) {
	root, d, _ := pricingFixture(t)
	h := pricingHandler(root, d)
	for _, layout := range []string{"flip", "side", "stack"} {
		for _, scale := range []string{"fit", "actual"} {
			for _, active := range []string{"original", "converted"} {
				path := pricingReferenceRoute + "?layout=" + layout + "&scale=" + scale + "&active=" + active + "&viewport=1440&preview_theme=dark&theme=light"
				w := referenceRequest(h, "GET", path, nil)
				if w.Code != 200 {
					t.Fatalf("%s: %d", path, w.Code)
				}
				body := w.Body.String()
				for _, want := range []string{`<!doctype html>`, `data-layout="` + layout + `"`, `data-scale="` + scale + `"`, `data-active="` + active + `"`, `<noscript>`, `method="get"`, `Above / below`, `Side by side`, `Enter fullscreen`, `Preview scroll positions are independent.`, `width="1440" height="1000"`, `data-theme="light"`} {
					if !strings.Contains(body, want) {
						t.Errorf("%s missing %q", path, want)
					}
				}
				for _, side := range []string{"original", "converted"} {
					prefix := `id="comparison-` + side + `" `
					wantHidden := layout == "flip" && side != active
					if strings.Contains(body, prefix+"hidden") != wantHidden {
						t.Errorf("%s: hidden state for %s", path, side)
					}
				}
				if strings.Contains(body, "max-w-6xl") || strings.Count(body, "<main") != 1 || strings.Contains(body, "ZgotmplZ") {
					t.Fatal("workspace wrapped, landmark duplicated or template rejected")
				}
			}
		}
	}
	w := referenceRequest(h, "GET", pricingReferenceRoute, nil)
	if !strings.Contains(w.Body.String(), `data-layout="flip" data-active="original" data-scale="fit"`) {
		t.Fatal("default should show one full-width fitted preview")
	}
	if !strings.Contains(w.Body.String(), `id="comparison-scale-label">100% · fit controls not initialized</output>`) {
		t.Fatal("failed or blocked scripts must not silently label actual-size output as fitted")
	}
}

func TestPricingComparisonScriptAdmissionIsExactAndOuterOnly(t *testing.T) {
	root, d, _ := pricingFixture(t)
	h := pricingHandler(root, d)
	js := referenceRequest(h, "GET", pricingReferenceRoute+"/workspace.js", nil)
	css := referenceRequest(h, "GET", pricingReferenceRoute+"/workspace.css", nil)
	if js.Code != 200 || css.Code != 200 || js.Body.String() != string(pricingCompareJS) || css.Body.String() != string(pricingCompareCSS) {
		t.Fatal("workspace assets differ")
	}
	sum := sha256.Sum256(js.Body.Bytes())
	digest := "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
	w := referenceRequest(h, "GET", pricingReferenceRoute, nil)
	// html/template correctly entity-escapes '+' in base64 attribute values;
	// compare the browser-decoded attribute, not its serialized spelling.
	if !strings.Contains(html.UnescapeString(w.Body.String()), `integrity="`+digest+`"`) || !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src '"+digest+"'") {
		t.Fatal("script body, SRI and CSP disagree")
	}
	for _, forbidden := range []string{"unsafe-inline", "unsafe-eval", "script-src 'self'"} {
		if strings.Contains(w.Header().Get("Content-Security-Policy"), forbidden) {
			t.Fatalf("overbroad script policy: %s", forbidden)
		}
	}
	changed := append([]byte(nil), js.Body.Bytes()...)
	changed[0] ^= 1
	mutated := sha256.Sum256(changed)
	if strings.Contains(w.Header().Get("Content-Security-Policy"), base64.StdEncoding.EncodeToString(mutated[:])) {
		t.Fatal("changed script admitted by header")
	}
	for _, path := range []string{pricingReferenceRoute + "/document/original-light.html", pricingReferenceRoute + "?layout=invalid", pricingReferenceRoute + "/workspace.js", pricingReferenceRoute + "/workspace.css"} {
		response := referenceRequest(h, "GET", path, nil)
		if strings.Contains(response.Header().Get("Content-Security-Policy"), digest) {
			t.Fatal("parent script admission leaked", path)
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private asset caching changed", path)
		}
	}
	for _, forbidden := range []string{"contentDocument", "contentWindow", "postMessage", "fetch(", "localStorage", "innerHTML"} {
		if strings.Contains(js.Body.String(), forbidden) {
			t.Errorf("workspace gained frame/data access: %s", forbidden)
		}
	}
	for _, role := range []string{"surface", "ink", "accent", "focus", "line"} {
		if !strings.Contains(css.Body.String(), "var(--ak-color-"+role+")") {
			t.Errorf("missing semantic role: %s", role)
		}
	}
	if !strings.Contains(css.Body.String(), "scrollbar-gutter: stable") {
		t.Fatal("fit can oscillate when changing content height adds/removes scrollbar width")
	}
}

func TestPricingComparisonRootGutterIsStable(t *testing.T) {
	rootRule := regexp.MustCompile(`(?m)^html\s*\{[^}]*\}`).Find(pricingCompareCSS)
	if !strings.Contains(string(rootRule), "scrollbar-gutter: stable") {
		t.Fatal("root scrollbar width changes available pane width as fitted content changes page height")
	}
}

func TestPricingComparisonWorkspaceRefusesInvalidViewAndAssetQueries(t *testing.T) {
	root, d, _ := pricingFixture(t)
	h := pricingHandler(root, d)
	for _, query := range []string{"layout=", "layout=overlay", "layout=flip&layout=stack", "scale=100", "scale=", "scale=fit&scale=actual", "active=other", "active=", "active=original&active=converted", "layout%7Cscale=flip"} {
		w := referenceRequest(h, "GET", pricingReferenceRoute+"?"+query, nil)
		if w.Code != 400 {
			t.Errorf("%s admitted: %d", query, w.Code)
		}
		assertPricingDenied(t, w)
	}
	for _, asset := range []string{"workspace.js", "workspace.css"} {
		for _, tc := range []struct {
			suffix, method string
			headers        map[string]string
			code           int
		}{
			{"?v=1", "GET", nil, 400}, {"", "POST", nil, 405}, {"", "HEAD", nil, 405}, {"", "GET", map[string]string{"Origin": "https://example.com"}, 403},
		} {
			w := referenceRequest(h, tc.method, pricingReferenceRoute+"/"+asset+tc.suffix, tc.headers)
			if w.Code != tc.code {
				t.Error(fmt.Sprintf("%s %s: got %d want %d", tc.method, asset, w.Code, tc.code))
			}
			assertPricingDenied(t, w)
		}
		w := referenceRequest(h, "GET", pricingReferenceRoute+"/%77"+asset[1:], nil)
		if w.Code != 400 {
			t.Fatal("escaped asset alias admitted")
		}
	}
}
