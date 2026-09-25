package dashboard

import "fmt"

// AccountDestinations is an experimental presentation recipe, not an account
// service. Applications supply only destinations their routing and authorization
// policy permits. Use the same group with RenderNav in sidebar and account menus;
// omitting a destination does not authorize or forbid access to its HTTP handler.
//
// Like NavItem, Icon is trusted HTML. Labels and URLs are escaped by RenderNav,
// not by this model. Custom link markup must separately validate destinations;
// escaping alone does not reproduce RenderNav's disabled-item contract. Existing
// Shell and NavGroup composition remains available.
type AccountDestinations struct {
	Profile, Settings, Help, About NavItem
}

// Group returns a fresh group in Profile, Settings, Help, About order. A destination
// with neither Href nor Label is omitted. A supplied URL with no label uses its
// English role name; localized or app-specific labels stay caller-owned. A label
// without a URL remains available for RenderNav's disabled-item treatment.
func (a AccountDestinations) Group(heading string) NavGroup {
	group := NavGroup{Heading: heading}
	for _, entry := range []struct {
		item  NavItem
		label string
	}{{a.Profile, "Profile"}, {a.Settings, "Settings"}, {a.Help, "Help"}, {a.About, "About"}} {
		if entry.item.Href == "" && entry.item.Label == "" {
			continue
		}
		if entry.item.Label == "" {
			entry.item.Label = entry.label
		}
		group.Items = append(group.Items, entry.item)
	}
	return group
}

// NoticeKind describes presentation, never HTTP status, authorization or whether
// a mutation succeeded. The app owns that determination and the message it shows.
type NoticeKind uint8

const (
	NoticeStatus NoticeKind = iota
	NoticeError
	NoticeConflict
)

// Notice is an experimental plain-text feedback recipe. A status is announced
// politely; errors and conflicts use alert semantics. It supplies no retry action,
// focus movement, draft retention, save logic or live-region update mechanism.
type Notice struct {
	Kind        NoticeKind
	Title, Body string
}

// RenderNotice renders escaped feedback using the dashboard's L3b token utilities.
// Scan appkit/ui/dashboard/*.go when building CSS. An empty body emits nothing;
// unknown kinds use attention treatment instead of silently becoming success.
// Callers rendering a changed live page still own focus and announcement testing.
func RenderNotice(n Notice) string {
	if n.Body == "" {
		return ""
	}
	role, tone, title := "alert", "border-danger bg-danger-soft", "Changes need attention"
	switch n.Kind {
	case NoticeStatus:
		role, tone, title = "status", "border-line bg-surface-raised", "Update"
	case NoticeConflict:
		title = "Review required"
	}
	if n.Title != "" {
		title = n.Title
	}
	return fmt.Sprintf(`<section role="%s" tabindex="-1" class="mb-6 rounded-md border %s p-4"><h2 class="text-sm font-semibold">%s</h2><p class="mt-2 max-w-prose text-sm">%s</p></section>`, role, tone, esc(title), esc(n.Body))
}
