package gallery

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestShellDemoNativeHistoryDoesNotOptIntoMixedVersionRestoration(t *testing.T) {
	for _, page := range []string{"profile", "settings"} {
		for _, status := range []int{200, 409, 422, 503} {
			view := shellDemoView{Page: page, State: defaultShellDemoState(3), Status: status}
			body, err := renderShellDemo(view)
			if err != nil {
				t.Fatal(err)
			}
			root, err := html.Parse(bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			attr := func(n *html.Node, key string) string {
				for _, a := range n.Attr {
					if a.Key == key {
						return a.Val
					}
				}
				return ""
			}
			var editForms, controls int
			var walk func(*html.Node, bool)
			walk = func(n *html.Node, inEditor bool) {
				if n.Type == html.ElementNode && n.Data == "form" {
					for _, a := range n.Attr {
						if a.Key == "data-shell-edit" {
							inEditor = true
							editForms++
						}
					}
					if inEditor && attr(n, "autocomplete") != "off" {
						t.Errorf("%s/%d edit form must opt out of network-history autofill", page, status)
					}
				}
				if inEditor && n.Type == html.ElementNode && (n.Data == "input" || n.Data == "select") && attr(n, "type") != "hidden" {
					controls++
					if policy := attr(n, "autocomplete"); policy != "" && policy != "off" {
						t.Errorf("%s/%d field %s overrides native history policy with %q", page, status, attr(n, "name"), policy)
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c, inEditor)
				}
			}
			walk(root, false)
			wantControls := 1
			if page == "settings" {
				wantControls = 3
			}
			if editForms != 1 || controls != wantControls {
				t.Fatalf("%s/%d forms=%d controls=%d; want 1/%d", page, status, editForms, controls, wantControls)
			}
		}
	}
}

func TestShellDemoPayloadBudgets(t *testing.T) {
	for _, page := range []string{"home", "activity", "profile", "settings", "help", "about"} {
		v := shellDemoView{Page: page, State: defaultShellDemoState(1), CSRF: strings.Repeat("a", 64), Nonce: strings.Repeat("b", 64), Status: 200}
		v.State.DisplayName = strings.Repeat("界", 80)
		v.State.Activity = make([]string, 10)
		for i := range v.State.Activity {
			v.State.Activity[i] = "Profile saved in this synthetic session only."
		}
		body, err := renderShellDemo(v)
		if err != nil || len(body) > 64<<10 {
			t.Fatalf("%s ordinary HTML exceeds 64KiB budget: bytes=%d err=%v", page, len(body), err)
		}
	}
	for _, access := range []string{"editor", "viewer"} {
		v := shellDemoView{Page: "profile", State: defaultShellDemoState(1), CSRF: strings.Repeat("a", 64), Nonce: strings.Repeat("b", 64), Status: 422, Fields: map[string]string{"display_name": strings.Repeat(`"<&`, 2500)}, Errors: map[string]string{"display_name": "Use 1 to 80 characters."}, Notice: "The draft was not saved."}
		v.State.Access = access
		body, err := renderShellDemo(v)
		if err != nil || len(body) > 256<<10 {
			t.Fatalf("rejected HTML exceeds 256KiB budget: bytes=%d err=%v", len(body), err)
		}
	}
	if len(shellDemoJS) > 8<<10 {
		t.Fatalf("draft script exceeds 8KiB budget: %d", len(shellDemoJS))
	}
}

func TestShellDemoAccountDestinationsAgreeAcrossNavigation(t *testing.T) {
	for _, page := range []string{"home", "activity", "profile", "settings", "help", "about"} {
		t.Run(page, func(t *testing.T) {
			body, err := renderShellDemo(shellDemoView{Page: page, State: defaultShellDemoState(1), Status: 200})
			if err != nil {
				t.Fatal(err)
			}
			root := shellDemoTestDOM(t, string(body))
			navs := map[string][]string{}
			current := map[string][]string{}
			shellDemoTestWalk(root, func(n *html.Node) {
				if n.Type != html.ElementNode || n.Data != "nav" {
					return
				}
				label := shellDemoTestAttr(n, "aria-label")
				shellDemoTestWalk(n, func(link *html.Node) {
					if link.Type != html.ElementNode || link.Data != "a" {
						return
					}
					href := shellDemoTestAttr(link, "href")
					for _, want := range []string{"/shell/profile", "/shell/settings", "/shell/help", "/shell/about"} {
						if href == want {
							navs[label] = append(navs[label], href+":"+shellDemoTestText(link))
						}
					}
					if shellDemoTestAttr(link, "aria-current") == "page" {
						current[label] = append(current[label], href)
					}
				})
			})
			want := "/shell/profile:Profile,/shell/settings:Settings,/shell/help:Help,/shell/about:About"
			for _, label := range []string{"Primary", "Primary (mobile)", "Account destinations"} {
				if got := strings.Join(navs[label], ","); got != want {
					t.Errorf("%s account links = %q, want %q", label, got, want)
				}
				wantCurrent := shellDemoURL(page)
				if label == "Account destinations" && (page == "home" || page == "activity") {
					wantCurrent = ""
				}
				if got := strings.Join(current[label], ","); got != wantCurrent {
					t.Errorf("%s current = %q, want %q", label, got, wantCurrent)
				}
			}
			if strings.Contains(string(body), "#ZgotmplZ") {
				t.Fatal("typed destination was rejected by the enclosing template")
			}
		})
	}
}

func TestShellDemoNoticeRetainsOutcomeAndEscaping(t *testing.T) {
	for _, tc := range []struct {
		status      int
		role, title string
	}{{200, "status", "Update"}, {422, "alert", "Changes need attention"}, {409, "alert", "Review a newer saved version"}, {503, "alert", "Changes need attention"}} {
		body, err := renderShellDemo(shellDemoView{Page: "profile", State: defaultShellDemoState(1), Status: tc.status, Notice: `<script>untrusted</script>`})
		if err != nil {
			t.Fatal(err)
		}
		root := shellDemoTestDOM(t, string(body))
		found := 0
		shellDemoTestWalk(root, func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "section" && shellDemoTestAttr(n, "role") == tc.role && shellDemoTestText(n) == tc.title+`<script>untrusted</script>` {
				found++
			}
		})
		if found != 1 || strings.Contains(string(body), `<script>untrusted</script>`) {
			t.Errorf("status %d: wanted one escaped %s notice, found %d", tc.status, tc.role, found)
		}
	}
}
