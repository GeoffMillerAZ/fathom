package appearance

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

func TestBootstrapReusesRuntimeAndEscapesConfiguration(t *testing.T) {
	key := "ca-theme\"</script><script>alert(1)</script>&\n\u2028\u2029"
	got := BootstrapScript(Options{StorageKey: key})
	if !strings.HasPrefix(got, Script()+"\n") {
		t.Fatal("bootstrap diverges from the deployed runtime")
	}
	if strings.Contains(strings.ToLower(got), "</script") {
		t.Fatal("bootstrap can close its containing script tag")
	}
	config := strings.TrimSuffix(strings.TrimPrefix(got[len(Script()):], "\nwindow.AppKitAppearance.init("), ");\n")
	var decoded Options
	if err := json.Unmarshal([]byte(config), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.StorageKey != key {
		t.Fatalf("storage key did not round-trip: %q", decoded.StorageKey)
	}
	if strings.Count(got, "w.AppKitAppearance =") != 1 {
		t.Fatal("bootstrap duplicates the controller")
	}
}

func TestRenderControlNativeSemanticsAndEscaping(t *testing.T) {
	key := `gg-theme"><script>alert(1)</script>`
	markup := RenderControl(Options{StorageKey: key}) + RenderControl(Options{StorageKey: key})
	doc, err := xhtml.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	var selects, statuses, buttons, controls int
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		attrs := map[string]string{}
		for _, attr := range n.Attr {
			attrs[attr.Key] = attr.Val
		}
		if _, ok := attrs["id"]; ok {
			t.Error("control introduces a shared ID and cannot compose safely")
		}
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "script":
				t.Error("storage key injected script markup")
			case "select":
				selects++
				if n.Parent.Data != "label" {
					t.Error("select lacks an associated native label")
				}
				if _, ok := attrs["disabled"]; !ok {
					t.Error("unmounted control claims working behavior")
				}
				var options []string
				for child := n.FirstChild; child != nil; child = child.NextSibling {
					if child.Data == "option" {
						for _, attr := range child.Attr {
							if attr.Key == "value" {
								options = append(options, attr.Val)
							}
						}
					}
				}
				if strings.Join(options, ",") != "light,dark,system" {
					t.Errorf("appearance choices = %v", options)
				}
			case "button":
				buttons++
				if attrs["type"] != "button" {
					t.Error("retry may submit an app-owned form")
				}
				if _, ok := attrs["hidden"]; !ok {
					t.Error("retry should appear only when needed")
				}
			}
		}
		if attrs["role"] == "status" {
			statuses++
			if attrs["aria-live"] != "polite" || attrs["aria-atomic"] != "true" {
				t.Error("save state is not announced as a complete status")
			}
		}
		if value, ok := attrs["data-appearance-storage-key"]; ok {
			controls++
			if value != key {
				t.Errorf("storage key changed: %q", value)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if selects != 2 || statuses != 2 || buttons != 2 || controls != 2 {
		t.Fatalf("incomplete controls: selects=%d statuses=%d buttons=%d controls=%d", selects, statuses, buttons, controls)
	}
	for _, forbidden := range []string{"style=", "bg-white", "text-black", "dark:", "onclick=", "onchange="} {
		if strings.Contains(markup, forbidden) {
			t.Errorf("renderer contains %q", forbidden)
		}
	}
}

func TestRenderControlHasOneTruthfulScriptFallback(t *testing.T) {
	markup := RenderControl(Options{StorageKey: "ca-theme"})
	doc, err := xhtml.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	var statuses, selects, retries, noscripts int
	var statusText string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		attrs := map[string]string{}
		for _, attr := range n.Attr {
			attrs[attr.Key] = attr.Val
		}
		if attrs["role"] == "status" {
			statuses++
			var text func(*xhtml.Node)
			text = func(node *xhtml.Node) {
				if node.Type == xhtml.TextNode {
					statusText += node.Data
				}
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					text(child)
				}
			}
			text(n)
		}
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "select":
				selects++
				if _, ok := attrs["disabled"]; !ok {
					t.Error("selector must remain disabled until the controller mounts")
				}
			case "button":
				if _, ok := attrs["data-appkit-appearance-retry"]; ok {
					retries++
					if _, ok := attrs["hidden"]; !ok {
						t.Error("retry must remain hidden until the controller reports a recoverable error")
					}
				}
			case "noscript":
				noscripts++
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if statuses != 1 || selects != 1 || retries != 1 {
		t.Fatalf("fallback structure: statuses=%d selects=%d retries=%d", statuses, selects, retries)
	}
	if strings.TrimSpace(statusText) != "Appearance controls require JavaScript." {
		t.Fatalf("fallback status = %q", strings.TrimSpace(statusText))
	}
	if noscripts != 0 || strings.Contains(strings.ToLower(markup), "loading") || strings.Contains(markup, "scripts are disabled") {
		t.Fatalf("renderer retains misleading or duplicate fallback: noscripts=%d markup=%q", noscripts, markup)
	}
}

func TestHandlerDeployment(t *testing.T) {
	h := Handler()
	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/assets/appearance.js", nil))
	if get.Code != http.StatusOK || get.Body.String() != Script() {
		t.Fatalf("GET status=%d body differs=%v", get.Code, get.Body.String() != Script())
	}
	if get.Header().Get("Content-Type") != "application/javascript; charset=utf-8" || get.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unsafe script headers: %v", get.Header())
	}
	if strings.Contains(get.Header().Get("Cache-Control"), "immutable") || !strings.Contains(get.Header().Get("Cache-Control"), "must-revalidate") {
		t.Fatal("stable URL can silently retain an obsolete controller")
	}
	for _, method := range []string{http.MethodHead, http.MethodPost} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(method, "/assets/appearance.js", nil))
		if method == http.MethodHead {
			if rr.Code != http.StatusOK || rr.Body.Len() != 0 || rr.Header().Get("Content-Length") == "" {
				t.Fatalf("HEAD = %d headers=%v body=%q", rr.Code, rr.Header(), rr.Body.String())
			}
		} else if rr.Code != http.StatusMethodNotAllowed || rr.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("POST = %d, allow=%q", rr.Code, rr.Header().Get("Allow"))
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/assets/appearance.js", nil)
	req.Header.Set("If-None-Match", get.Header().Get("ETag"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotModified || rr.Body.Len() != 0 {
		t.Fatalf("conditional GET = %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestBrowserStateMachine(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable: browser state-machine checks were not run; consumers do not require Node")
	}
	cmd := exec.Command(node, "--test", "appearance_test.mjs")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("appearance browser-state checks: %v\n%s", err, output)
	}
}

func ExampleBootstrapScript() {
	// In a real HTML template, put this body in an app-owned, CSP-authorized script
	// tag before loading theme CSS. Do not print it as escaped HTML text.
	_, _ = io.WriteString(io.Discard, BootstrapScript(Options{StorageKey: "ca-theme"}))
}
