package palette_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/palette"
)

// One app-owned route answers both the palette (fragment) and the no-script
// search page (full page) from the same results.
func Example() {
	p := palette.Palette{ID: "cmd", Action: "/search"}
	search := func(q string) palette.Results {
		r := palette.Results{Query: q}
		if strings.Contains("orbit-service", q) {
			r.Groups = []palette.Group{{Heading: "Projects", Results: []palette.Result{{Label: "orbit-service", Href: "/projects/orbit-service"}}}}
		}
		return r
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		w.Header().Add("Vary", palette.FragmentHeader)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if palette.WantsFragment(r) {
			fragment, err := palette.RenderResults(search(q))
			if err != nil {
				http.Error(w, "search failed", http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(w, fragment)
			return
		}
		results := search(q)
		results.HeadingLevel = 2 // beneath the page's h1
		form, err := palette.RenderSearchForm(p, q)
		if err != nil {
			http.Error(w, "search failed", http.StatusInternalServerError)
			return
		}
		fragment, err := palette.RenderResults(results)
		if err != nil {
			http.Error(w, "search failed", http.StatusInternalServerError)
			return
		}
		// An app-owned page shell wraps these, plus the trigger, dialog and script.
		_, _ = io.WriteString(w, "<h1>Search</h1>"+form+fragment)
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=orbit", nil)
	req.Header.Set(palette.FragmentHeader, "1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	fmt.Println(strings.HasPrefix(rec.Body.String(), `<div data-palette-fragment data-palette-state="results"`))

	page := httptest.NewRecorder()
	mux.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/search?q=orbit", nil))
	fmt.Println(strings.HasPrefix(page.Body.String(), "<h1>Search</h1><form role=\"search\""))
	// Output:
	// true
	// true
}
