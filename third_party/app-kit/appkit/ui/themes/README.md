# Shipped themes

Alternative token sets app-kit ships beyond the kit default
(`cue/tokens/kit.cue` → `appkit/ui/tokens/theme.css`). Each theme is an
independent token set authored against `appkit/ui/tokens/schema.cue` (`#Tokens`)
and generated to a committed `theme.css` an app can `@import` directly.

## synthwave

Neon white + purple on near-black violet — Geoff's brand. **Dark-only**: the
`light` and `dark` values are identical, so it reads as synthwave regardless of
the page's color scheme. Adds one open role, `neon-pink`, used by the kanban
card-title hover (so `text-neon-pink` resolves under this theme).

- Source: `cue/tokens/synthwave/synthwave.cue`
- Generated CSS: `appkit/ui/themes/synthwave/theme.css`

Use it from an app's Tailwind entry instead of the kit default theme:

```css
@import "tailwindcss";
@import ".../appkit/ui/themes/synthwave/theme.css";
@source "...";   /* your templ/components */
```

Regenerate after editing the token set:

```bash
task gen:synthwave
# or:
go run ./cmd/appkit gen \
  --tokens cue/tokens/synthwave/synthwave.cue \
  --css appkit/ui/themes/synthwave/theme.css --dtcg "" --go ""
```
