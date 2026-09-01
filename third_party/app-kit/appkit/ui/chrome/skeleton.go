package chrome

import (
	"fmt"
	"strings"
)

// Skeleton renders n shimmer placeholder bars for a loading state (pure CSS via
// Tailwind's animate-pulse; no JS). The last bar is shorter to suggest a final
// partial line. n <= 0 renders a single bar.
func Skeleton(n int) string {
	if n <= 0 {
		n = 1
	}
	var b strings.Builder
	b.WriteString(`<div class="space-y-2" role="status" aria-busy="true" aria-label="Loading">`)
	for i := 0; i < n; i++ {
		w := "w-full"
		if i == n-1 && n > 1 {
			w = "w-2/3"
		}
		fmt.Fprintf(&b, `<div class="h-4 %s rounded bg-surface-sunken animate-pulse"></div>`, w)
	}
	b.WriteString(`<span class="sr-only">Loading…</span></div>`)
	return b.String()
}

// SkeletonCard renders a card-shaped loading placeholder (a title bar + body
// lines inside the standard card surface).
func SkeletonCard(lines int) string {
	if lines <= 0 {
		lines = 3
	}
	var b strings.Builder
	b.WriteString(`<div class="rounded-lg border border-line bg-surface-raised p-5" role="status" aria-busy="true" aria-label="Loading">`)
	b.WriteString(`<div class="h-5 w-1/3 rounded bg-surface-sunken animate-pulse"></div>`)
	b.WriteString(`<div class="mt-4 space-y-2">`)
	for i := 0; i < lines; i++ {
		w := "w-full"
		if i == lines-1 && lines > 1 {
			w = "w-1/2"
		}
		fmt.Fprintf(&b, `<div class="h-4 %s rounded bg-surface-sunken animate-pulse"></div>`, w)
	}
	b.WriteString(`</div><span class="sr-only">Loading…</span></div>`)
	return b.String()
}
