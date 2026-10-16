package dashboard_test

import (
	"html/template"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/dashboard"
)

var (
	h1Re     = regexp.MustCompile(`<h1\b`)
	topBarRe = regexp.MustCompile(`<header class="flex h-14 [^"]*">(.*?)</header>`)
	mainRe   = regexp.MustCompile(`(?s)<main\b.*</main>`)
)

// topBar returns the shell's top bar markup (the first header, which the shell
// writes before main).
func topBar(t *testing.T, page string) string {
	t.Helper()
	m := topBarRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no top bar in:\n%s", page)
	}
	return m[1]
}

// FATHOM-S0-KIT-01: the shell's top bar always wrote Title as an h1, and
// chrome.RenderPageHeading writes the page's h1, so a page composing both had
// two. Shell.PageHeading hands the h1 to the page: the top bar keeps Title as
// plain text (or nothing), and the page heading inside main is the only h1.
func TestShell_PageHeadingLeavesTheOneH1ToThePage(t *testing.T) {
	heading, err := chrome.RenderPageHeading(chrome.PageHeading{
		Breadcrumbs: []chrome.Crumb{{Label: "Overview", Href: "/"}, {Label: "Plans"}},
		Title:       "Plans",
		Meta:        []string{"12 proven"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Before: composing both yields two h1s. The default is unchanged.
	both := dashboard.RenderShell(dashboard.Shell{Title: "Plans", Nav: "nav", Content: template.HTML(heading)})
	if n := len(h1Re.FindAllString(both, -1)); n != 2 {
		t.Fatalf("default shell + page heading: %d h1, want the documented 2 (top bar and page)", n)
	}
	if !strings.Contains(topBar(t, both), `<h1 class="truncate text-lg font-semibold text-ink">Plans</h1>`) {
		t.Errorf("default top bar no longer writes Title as its h1:\n%s", topBar(t, both))
	}

	for _, title := range []string{"Plans", ""} {
		got := dashboard.RenderShell(dashboard.Shell{
			Title: title, Nav: "nav", Actions: `<button>Search</button>`,
			Content: template.HTML(heading), PageHeading: true,
		})
		if n := len(h1Re.FindAllString(got, -1)); n != 1 {
			t.Errorf("Title %q: %d h1 elements, want exactly the page's one:\n%s", title, n, got)
		}
		if !h1Re.MatchString(mainRe.FindString(got)) {
			t.Errorf("Title %q: the one h1 is not inside main", title)
		}
		bar := topBar(t, got)
		if strings.Contains(bar, "<h1") || strings.Contains(bar, "<h2") {
			t.Errorf("Title %q: the top bar still writes a heading: %s", title, bar)
		}
		// Actions stay at the end of the bar: the leading slot is always
		// written, so justify-between has two children even with no title.
		if !strings.HasPrefix(bar, `<div class="flex min-w-0 items-center gap-3">`) ||
			!strings.HasSuffix(bar, `<div class="flex items-center gap-3"><button>Search</button></div>`) {
			t.Errorf("Title %q: top bar lost its leading slot or its trailing actions: %s", title, bar)
		}
		if title != "" && !strings.Contains(bar, `<p class="truncate text-lg font-semibold text-ink">Plans</p>`) {
			t.Errorf("the top bar dropped the plain-text title: %s", bar)
		}
		if title == "" && strings.Contains(bar, "<p") {
			t.Errorf("an empty Title wrote an empty text element: %s", bar)
		}
	}
}

// The same record's second half: below md the sidebar (and the brand in it) is
// hidden, so a page whose top bar no longer names the page showed no app
// identity at all. Shell.MobileBrand repeats Brand at the start of the top bar
// below md only.
func TestShell_MobileBrandRepeatsTheBrandBelowMd(t *testing.T) {
	brand := template.HTML(`<a href="/" class="min-h-11">acme</a>`)
	got := dashboard.RenderShell(dashboard.Shell{Brand: brand, Nav: "nav", PageHeading: true, MobileBrand: true})
	if n := strings.Count(got, string(brand)); n != 2 {
		t.Fatalf("brand rendered %d times, want 2 (sidebar and mobile top bar)", n)
	}
	bar := topBar(t, got)
	want := `<div class="flex min-w-0 items-center gap-3"><div class="flex shrink-0 items-center md:hidden">` + string(brand) + `</div></div>`
	if !strings.HasPrefix(bar, want) {
		t.Errorf("top bar does not lead with the md:hidden brand:\n got %s\nwant prefix %s", bar, want)
	}

	// Off by default: the brand stays in the sidebar only.
	plain := dashboard.RenderShell(dashboard.Shell{Brand: brand, Nav: "nav", PageHeading: true})
	if n := strings.Count(plain, string(brand)); n != 1 {
		t.Errorf("brand rendered %d times without MobileBrand, want 1", n)
	}
	// Independent of PageHeading: the top bar keeps its h1 after the brand.
	withH1 := dashboard.RenderShell(dashboard.Shell{Title: "Plans", Brand: brand, MobileBrand: true})
	if !strings.Contains(topBar(t, withH1), `md:hidden">`+string(brand)+`</div><h1 class="truncate text-lg font-semibold text-ink">Plans</h1>`) {
		t.Errorf("MobileBrand without PageHeading lost the top-bar h1: %s", topBar(t, withH1))
	}
}

// Neither option set: byte-identical to the shell before the options existed,
// so no consumer's snapshot moves until it opts in.
func TestShell_DefaultTopBarIsUnchanged(t *testing.T) {
	got := dashboard.RenderShell(dashboard.Shell{Title: "T", Actions: "<b>a</b>", Content: "c"})
	const want = `<header class="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-line bg-surface px-4">` +
		`<h1 class="truncate text-lg font-semibold text-ink">T</h1><div class="flex items-center gap-3"><b>a</b></div></header>`
	if !strings.Contains(got, want) {
		t.Errorf("default top bar changed:\n got %s\nwant %s", topBar(t, got), want)
	}
}
