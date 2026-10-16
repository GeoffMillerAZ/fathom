package dashboard_test

import (
	"fmt"
	"html/template"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/dashboard"
)

func TestAccountDestinationsGroupOrderDefaultsAndPreservation(t *testing.T) {
	account := dashboard.AccountDestinations{
		Profile:  dashboard.NavItem{Href: "/account", Icon: template.HTML(`<svg data-icon="profile"></svg>`), Match: dashboard.MatchSubtree},
		Settings: dashboard.NavItem{},
		Help:     dashboard.NavItem{Label: "Support"},
		About:    dashboard.NavItem{Href: "javascript:alert(1)", Icon: template.HTML(`<span>i</span>`), Match: dashboard.NavMatch(99)},
	}
	want := dashboard.NavGroup{
		Heading: "Your <account>",
		Items: []dashboard.NavItem{
			{Href: "/account", Label: "Profile", Icon: template.HTML(`<svg data-icon="profile"></svg>`), Match: dashboard.MatchSubtree},
			{Label: "Support"},
			{Href: "javascript:alert(1)", Label: "About", Icon: template.HTML(`<span>i</span>`), Match: dashboard.NavMatch(99)},
		},
	}
	if got := account.Group("Your <account>"); !reflect.DeepEqual(got, want) {
		t.Fatalf("Group() = %#v, want %#v", got, want)
	}

	first := account.Group("Your account")
	first.Items[0].Href = "/mutated"
	second := account.Group("Your account")
	if second.Items[0].Href != "/account" || account.Profile.Href != "/account" {
		t.Fatal("returned group aliases a prior result or mutates its input")
	}
}

func TestAccountDestinationsGroupOmitsOnlyUnsupportedDestinations(t *testing.T) {
	group := (dashboard.AccountDestinations{
		Profile:  dashboard.NavItem{Icon: template.HTML(`<svg></svg>`)},
		Settings: dashboard.NavItem{Match: dashboard.MatchSubtree},
		Help:     dashboard.NavItem{Label: "Help desk"},
		About:    dashboard.NavItem{Href: "/about"},
	}).Group("")
	if want := []dashboard.NavItem{{Label: "Help desk"}, {Href: "/about", Label: "About"}}; !reflect.DeepEqual(group.Items, want) {
		t.Fatalf("Items = %#v, want %#v", group.Items, want)
	}
}

func TestAccountDestinationsComposeWithRenderNav(t *testing.T) {
	group := (dashboard.AccountDestinations{
		Profile: dashboard.NavItem{Href: "/account", Match: dashboard.MatchSubtree},
		Help:    dashboard.NavItem{Href: "/help"},
		About:   dashboard.NavItem{Href: "javascript:alert(1)"},
	}).Group(`Your <account> & help`)

	html := dashboard.RenderNav([]dashboard.NavGroup{group}, "/account/details")
	for _, want := range []string{
		`Your &lt;account&gt; &amp; help`,
		`aria-label="Your &lt;account&gt; &amp; help"`,
		`href="/account"`,
		`aria-current="page"`,
		`aria-disabled="true"`,
		`>Profile</span>`,
		`>Help</span>`,
		`>About</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("RenderNav output missing %q: %s", want, html)
		}
	}
	if strings.Contains(html, `href="javascript:`) {
		t.Fatal("unsafe destination became an interactive link")
	}
}

func TestRenderNoticeKindsAndDefaults(t *testing.T) {
	tests := []struct {
		name  string
		kind  dashboard.NoticeKind
		role  string
		tone  string
		title string
	}{
		{name: "status zero value", kind: dashboard.NoticeStatus, role: "status", tone: "border-line bg-surface-raised", title: "Update"},
		{name: "error", kind: dashboard.NoticeError, role: "alert", tone: "border-danger bg-danger-soft", title: "Changes need attention"},
		{name: "conflict", kind: dashboard.NoticeConflict, role: "alert", tone: "border-danger bg-danger-soft", title: "Review required"},
		{name: "unknown fails attention", kind: dashboard.NoticeKind(255), role: "alert", tone: "border-danger bg-danger-soft", title: "Changes need attention"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html := dashboard.RenderNotice(dashboard.Notice{Kind: tt.kind, Body: "Saved values remain unchanged."})
			for _, want := range []string{
				`<section role="` + tt.role + `" tabindex="-1" class="mb-6 rounded-md border ` + tt.tone + ` p-4">`,
				`<h2 class="text-sm font-semibold">` + tt.title + `</h2>`,
				`<p class="mt-2 max-w-prose text-sm">Saved values remain unchanged.</p>`,
			} {
				if !strings.Contains(html, want) {
					t.Errorf("RenderNotice output missing %q: %s", want, html)
				}
			}
		})
	}
}

func TestRenderNoticeEmptyAndEscaped(t *testing.T) {
	if got := dashboard.RenderNotice(dashboard.Notice{Kind: dashboard.NoticeError, Title: "Ignored", Body: ""}); got != "" {
		t.Fatalf("empty body rendered %q", got)
	}

	html := dashboard.RenderNotice(dashboard.Notice{
		Title: `<img src=x onerror="alert(1)">`,
		Body:  `<script>alert("body")</script> & retry`,
	})
	for _, want := range []string{
		`&lt;img src=x onerror=&#34;alert(1)&#34;&gt;`,
		`&lt;script&gt;alert(&#34;body&#34;)&lt;/script&gt; &amp; retry`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("escaped notice missing %q: %s", want, html)
		}
	}
	for _, forbidden := range []string{"<script", "<form", " action=", "onclick=", "javascript:", "style=", "rgb(", "rgba(", "hsl(", "hsla("} {
		if strings.Contains(html, forbidden) {
			t.Errorf("notice invented executable/form behavior or a literal color %q: %s", forbidden, html)
		}
	}
	if regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`).MatchString(html) {
		t.Errorf("notice emitted a literal hex color: %s", html)
	}
}

func ExampleAccountDestinations_Group() {
	group := (dashboard.AccountDestinations{
		Profile: dashboard.NavItem{Href: "/account/profile"},
		Help:    dashboard.NavItem{Href: "/help", Label: "Support"},
	}).Group("Your account")

	fmt.Println(group.Heading)
	for _, item := range group.Items {
		fmt.Println(item.Label, item.Href)
	}
	// Output:
	// Your account
	// Profile /account/profile
	// Support /help
}
