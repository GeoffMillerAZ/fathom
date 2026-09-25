package gallery

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	kitdocs "github.com/geoffmilleraz/app-kit/appkit/ui/docs"
)

func staticContentHandler() http.Handler {
	mux := http.NewServeMux()
	registerStaticContent(mux)
	return mux
}

func serveStaticContentRequest(t *testing.T, handler http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, target, nil))
	return recorder
}

func TestStaticContentModelRefusesInvalidInputBeforeOutput(t *testing.T) {
	base := staticContentGuide()
	for name, mutate := range map[string]func(*staticContentModel){
		"blank title":          func(m *staticContentModel) { m.Title = " " },
		"duplicate identifier": func(m *staticContentModel) { m.FAQ[0].ID = m.Sections[0].ID },
		"unsafe destination":   func(m *staticContentModel) { m.FAQ[0].Destination = "https://example.test" },
		"missing destination":  func(m *staticContentModel) { m.FAQ[0].Destination = "absent" },
		"overlong question":    func(m *staticContentModel) { m.FAQ[0].Question = strings.Repeat("x", 513) },
		"control character":    func(m *staticContentModel) { m.Sections[0].Body = "safe\x00unsafe" },
		"too many FAQ entries": func(m *staticContentModel) {
			m.FAQ = append(m.FAQ, m.FAQ...)
			m.FAQ = append(m.FAQ, m.FAQ...)
		},
		"reserved identifier":      func(m *staticContentModel) { m.FAQ[0].ID = "content-guide" },
		"fixed FAQ heading":        func(m *staticContentModel) { m.Sections[0].ID = "content-faq-heading" },
		"fixed state control":      func(m *staticContentModel) { m.FAQ[0].ID = "content-state" },
		"shared main landmark":     func(m *staticContentModel) { m.Sections[0].ID = "gallery-main" },
		"shared navigation":        func(m *staticContentModel) { m.FAQ[0].ID = "gallery-navigation" },
		"invalid section identity": func(m *staticContentModel) { m.Sections[0].ID = "not allowed" },
	} {
		t.Run(name, func(t *testing.T) {
			model := base
			model.Sections = append([]staticContentSection(nil), base.Sections...)
			model.FAQ = append([]staticContentFAQ(nil), base.FAQ...)
			mutate(&model)
			if content, err := buildStaticContent(model, staticContentRequest{State: staticContentReady}); err == nil || content != "" {
				t.Fatalf("buildStaticContent() = %q, %v; want atomic refusal", content, err)
			}
		})
	}
}

func TestStaticContentReservesEveryFixedRenderedID(t *testing.T) {
	response, body := get(t, New(), "/content")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /content = %d, want 200", response.StatusCode)
	}
	ids := regexp.MustCompile(`\bid="([^"]+)"`).FindAllStringSubmatch(body, -1)
	if len(ids) == 0 {
		t.Fatal("rendered baseline has no IDs")
	}
	model := staticContentGuide()
	modelIDs := map[string]bool{}
	for _, section := range model.Sections {
		modelIDs[section.ID] = true
	}
	for _, item := range model.FAQ {
		modelIDs[item.ID] = true
	}
	seen := map[string]bool{}
	for _, match := range ids {
		id := match[1]
		if seen[id] {
			t.Errorf("full-page baseline has duplicate ID %q", id)
		}
		seen[id] = true
		if !modelIDs[id] && !staticContentReservedIDs[id] {
			t.Errorf("fixed rendered ID %q is not reserved", id)
		}
	}
}

func TestStaticContentEscapesHostilePlainText(t *testing.T) {
	model := staticContentGuide()
	model.Sections[0].Body = `<script>alert("no")</script> & text`
	content, err := buildStaticContent(model, staticContentRequest{State: staticContentReady})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "<script>alert") || !strings.Contains(string(content), "&lt;script&gt;") {
		t.Fatalf("hostile text was not escaped: %s", content)
	}
}

func TestStaticContentRoutesAndStates(t *testing.T) {
	handler := staticContentHandler()
	ready := serveStaticContentRequest(t, handler, http.MethodGet, "/content")
	if ready.Code != http.StatusOK {
		t.Fatalf("GET /content = %d, want 200", ready.Code)
	}
	for _, want := range []string{
		`<main id="gallery-main"`, `Choose a profile before you integrate a capability`,
		`<details id="profile"`, `href="/content/choosing-a-profile"`, `href="/catalog"`, `href="/quality"`,
		`min-h-11`, `focus-visible:outline-2`,
		`<form action="/content" method="get"`, `name="state"`, `Synthetic unavailable state`,
	} {
		if !strings.Contains(ready.Body.String(), want) {
			t.Errorf("ready content missing %q", want)
		}
	}
	expectedOutline := kitdocs.RenderToCPanel([]kitdocs.Heading{
		{Level: 2, ID: "content-guide", Text: "Read this first"},
		{Level: 2, ID: "start-with-task", Text: "Start with the task"},
		{Level: 2, ID: "keep-policy-local", Text: "Keep policy local"},
		{Level: 2, ID: "content-faq", Text: "Questions before reuse"},
	}, kitdocs.Options{})
	if !strings.Contains(ready.Body.String(), expectedOutline) {
		t.Fatal("ready content did not retain the exact native ToC panel")
	}
	if got := strings.Count(ready.Body.String(), "<main "); got != 1 {
		t.Fatalf("main landmarks = %d, want exactly one", got)
	}
	ids := regexp.MustCompile(`\bid="([^"]+)"`).FindAllStringSubmatch(ready.Body.String(), -1)
	seen := map[string]bool{}
	for _, match := range ids {
		if seen[match[1]] {
			t.Fatalf("duplicate rendered id %q", match[1])
		}
		seen[match[1]] = true
	}
	for _, id := range []string{"content-guide", "start-with-task", "keep-policy-local", "content-faq"} {
		if !seen[id] || !strings.Contains(ready.Body.String(), `href="#`+id+`"`) {
			t.Errorf("outline target %q is missing or broken", id)
		}
	}
	for _, want := range []string{`id="content-guide" tabindex="-1"`, `id="start-with-task" tabindex="-1"`, `id="keep-policy-local" tabindex="-1"`, `id="content-faq" tabindex="-1"`} {
		if !strings.Contains(ready.Body.String(), want) {
			t.Errorf("outline target is not programmatically focusable: %q", want)
		}
	}
	for _, target := range []string{"content-guide", "start-with-task", "keep-policy-local", "content-faq"} {
		attrs := regexp.MustCompile(`id="` + target + `"[^>]*`).FindString(ready.Body.String())
		for _, class := range []string{"focus-visible:outline-2", "focus-visible:outline-focus", "focus-visible:outline-offset-2"} {
			if !strings.Contains(attrs, class) {
				t.Errorf("fragment target %s lacks %s", target, class)
			}
		}
	}
	if strings.Contains(ready.Body.String(), `tabindex="0"`) {
		t.Fatal("static outline targets must not enter the ordinary Tab order")
	}
	if strings.Contains(ready.Body.String(), "<script") {
		t.Fatal("normal static content must not load a script")
	}

	article := serveStaticContentRequest(t, handler, http.MethodGet, "/content/choosing-a-profile?theme=dark")
	if article.Code != http.StatusOK || !strings.Contains(article.Body.String(), `data-theme="dark"`) || !strings.Contains(article.Body.String(), `href="/content?theme=dark"`) || !strings.Contains(article.Body.String(), `href="/content/choosing-a-profile?theme=light"`) || !strings.Contains(article.Body.String(), "Static brochure or documentation") || !strings.Contains(article.Body.String(), "Ordinary authoritative application") || !strings.Contains(article.Body.String(), "Local-heavy editing") {
		t.Fatalf("article did not preserve the local dark reading path: %d %s", article.Code, article.Body.String())
	}

	for state, title := range map[string]string{"empty": "No guide entries are available", "unavailable": "The guide is temporarily unavailable"} {
		recorder := serveStaticContentRequest(t, handler, http.MethodGet, "/content?state="+state+"&theme=dark")
		wantStatus := http.StatusOK
		if state == "unavailable" {
			wantStatus = http.StatusServiceUnavailable
		}
		if recorder.Code != wantStatus || !strings.Contains(recorder.Body.String(), title) || !strings.Contains(recorder.Body.String(), `href="/content?theme=dark"`) || !strings.Contains(recorder.Body.String(), `href="/content?state=`+state+`&amp;theme=light"`) {
			t.Errorf("state %s did not render its labeled recovery: %d %s", state, recorder.Code, recorder.Body.String())
		}
	}
	if strings.Contains(ready.Body.String(), "Can a reference become a shipped feature?") || !strings.Contains(ready.Body.String(), "Does a reference prove a feature is ready?") {
		t.Fatal("reference readiness wording drifted")
	}
}

func TestStaticContentRefusesUnsupportedRequestsAndRecoversUnknownDestination(t *testing.T) {
	handler := staticContentHandler()
	for _, target := range []string{"/content?state=ready&state=empty", "/content?unknown=x", "/content?theme=evil", "/content?state=other"} {
		if recorder := serveStaticContentRequest(t, handler, http.MethodGet, target); recorder.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", target, recorder.Code)
		}
	}
	post := serveStaticContentRequest(t, handler, http.MethodPost, "/content")
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != http.MethodGet {
		t.Errorf("POST /content = %d Allow=%q, want 405 GET", post.Code, post.Header().Get("Allow"))
	}
	unknown := serveStaticContentRequest(t, handler, http.MethodGet, "/content/not-a-guide?theme=dark")
	if unknown.Code != http.StatusNotFound || !strings.Contains(unknown.Body.String(), "Content destination unavailable") || !strings.Contains(unknown.Body.String(), `href="/content?theme=dark"`) {
		t.Errorf("unknown destination did not give truthful themed recovery: %d %s", unknown.Code, unknown.Body.String())
	}
	articleState := serveStaticContentRequest(t, handler, http.MethodGet, "/content/choosing-a-profile?state=empty")
	if articleState.Code != http.StatusBadRequest {
		t.Errorf("article state = %d, want 400", articleState.Code)
	}
}
