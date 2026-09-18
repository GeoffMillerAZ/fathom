package dashboard

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

// NavMatch selects how an app-owned link participates in current-page matching.
type NavMatch uint8

const (
	// MatchExact matches only the link's path. It is the default.
	MatchExact NavMatch = iota
	// MatchSubtree also matches descendants separated by a slash. The root path
	// remains exact, so a Home link does not select every page.
	MatchSubtree
)

// NavItem is an app-owned destination. Href and Label are escaped by RenderNav.
// Icon is trusted HTML: callers must review it, as with the Shell HTML slots.
// External HTTP(S) links are supported but never marked as the current page.
type NavItem struct {
	Href, Label string
	Icon        template.HTML
	Match       NavMatch
}

// NavGroup groups related destinations under an optional visible heading.
type NavGroup struct {
	Heading string
	Items   []NavItem
}

// MatchPath reports whether a root-relative href matches currentPath. Query
// strings, fragments and trailing slashes do not affect matching; encoded path
// separators remain encoded. External and unsafe URLs never match. Applications
// can reuse this helper for their own navigation compositions.
func MatchPath(href, currentPath string, match NavMatch) bool {
	_, target, safe := navHref(href)
	_, current, currentSafe := navHref(currentPath)
	return safe && currentSafe && matchNavPath(target, current, match)
}

func matchNavPath(target, current string, match NavMatch) bool {
	if target == "" || current == "" {
		return false
	}
	switch match {
	case MatchExact:
		return target == current
	case MatchSubtree:
		return target == current || (target != "/" && strings.HasPrefix(current, target+"/"))
	default:
		return false
	}
}

// navHref accepts root-relative and absolute HTTP(S) URLs. The path is populated
// only for root-relative destinations, keeping external links out of matching.
func navHref(href string) (string, string, bool) {
	if href == "" || strings.TrimSpace(href) != href || strings.Contains(href, `\`) {
		return "", "", false
	}
	for _, r := range href {
		if r < ' ' || r == 0x7f {
			return "", "", false
		}
	}
	u, err := url.Parse(href)
	if err != nil {
		return "", "", false
	}
	if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") && u.Host == "" {
		path := strings.TrimRight(u.EscapedPath(), "/")
		if path == "" {
			path = "/"
		}
		return href, path, true
	}
	if (strings.EqualFold(u.Scheme, "https") || strings.EqualFold(u.Scheme, "http")) && u.Host != "" && u.Opaque == "" {
		return href, "", true
	}
	return "", "", false
}

// RenderNav renders grouped navigation content for Shell.Nav or an app-owned
// nav landmark. It requires the consumer's Tailwind scanner to include this
// package, like RenderShell. It adds no client-side routing or dependencies.
//
// At most one link receives aria-current="page": the longest eligible path wins,
// then MatchExact wins over MatchSubtree, then the first item wins. Empty groups
// are omitted. Unsafe URLs retain their labels as noninteractive disabled items.
// No IDs are generated, so Shell can reuse the markup in its mobile disclosure.
func RenderNav(groups []NavGroup, currentPath string) string {
	_, current, _ := navHref(currentPath)
	bestGroup, bestItem, bestLength := -1, -1, -1
	bestExact := false
	for gi, group := range groups {
		for ii, item := range group.Items {
			_, target, safe := navHref(item.Href)
			if !safe || !matchNavPath(target, current, item.Match) {
				continue
			}
			exact := item.Match == MatchExact
			if len(target) > bestLength || (len(target) == bestLength && exact && !bestExact) {
				bestGroup, bestItem, bestLength, bestExact = gi, ii, len(target), exact
			}
		}
	}

	var b strings.Builder
	for gi, group := range groups {
		if len(group.Items) == 0 {
			continue
		}
		if group.Heading != "" {
			fmt.Fprintf(&b, `<h2 class="px-3 pb-1 pt-4 text-xs font-semibold uppercase tracking-wide text-ink-muted [overflow-wrap:anywhere]">%s</h2>`, esc(group.Heading))
		}
		b.WriteString(`<ul class="flex min-w-0 flex-col gap-1"`)
		if group.Heading != "" {
			fmt.Fprintf(&b, ` aria-label="%s"`, esc(group.Heading))
		}
		b.WriteString(`>`)
		for ii, item := range group.Items {
			href, _, safe := navHref(item.Href)
			active := gi == bestGroup && ii == bestItem
			b.WriteString(`<li class="min-w-0">`)
			classes := `flex min-h-[44px] min-w-0 items-start gap-3 rounded-md px-3 py-2 text-sm `
			if safe {
				classes += `focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus `
				if active {
					classes += `bg-accent-soft font-semibold text-ink`
				} else {
					classes += `font-medium text-ink-muted hover:bg-surface-sunken hover:text-ink`
				}
				fmt.Fprintf(&b, `<a href="%s" class="%s"`, esc(href), classes)
				if active {
					b.WriteString(` aria-current="page"`)
				}
				b.WriteString(`>`)
			} else {
				fmt.Fprintf(&b, `<span aria-disabled="true" class="%sfont-medium text-ink-faint">`, classes)
			}
			if item.Icon != "" {
				fmt.Fprintf(&b, `<span aria-hidden="true" class="mt-0.5 shrink-0">%s</span>`, item.Icon)
			}
			fmt.Fprintf(&b, `<span class="min-w-0 whitespace-normal [overflow-wrap:anywhere]">%s</span>`, esc(item.Label))
			if safe {
				b.WriteString(`</a>`)
			} else {
				b.WriteString(`</span>`)
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ul>`)
	}
	return b.String()
}
