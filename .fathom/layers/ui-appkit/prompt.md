# ui-appkit — compile prompt

You are compiling the app-kit UI doctrine into obligations: short, cited,
glob-scoped rules served to an agent at the moment it edits a UI file.

Source corpus: app-kit's decision docs (the kit decisions D1–D16 via the
appkit MCP / docs — ADR-0017's layered doctrine governs where it refines
earlier slogans), its README/CLAUDE.md UI pipeline sections, and the
consuming repo's own UI doctrine sections where present.

Emit a claim ONLY when it is: (1) a rule an agent could VIOLATE in a single
edit (not architecture narrative); (2) citable to a specific decision or doc
section; (3) scopeable to file globs (markup/render files, css/token files,
package manifests). Prefer the imperative, name the decision id, keep each
claim under 40 words. The six load-bearing families, in priority order:

1. Corpus-first: search the licensed corpus (appkit MCP) before writing any
   markup; hand-writing an existing component is the named failure.
2. Role tokens: palette→role mapping; dark: color variants dropped; no
   inline colors — every color is `var(--ak-color-<role>)` or a role
   utility (ADR-0017 §2); focus → ring-focus.
3. Visibility: the @source line contract for every rendered L3b kit
   package (L3a viz renderers need no scanner — don't demand @source
   for them).
4. Build hygiene, as ADR-0017 §3 splits it: CSS only via the pinned
   Tailwind binary (`appkit css`); no `package.json` in the kit repo or
   any repo whose primary artifact is a Go binary/handler tree;
   kit-authored browser JS only at the L1 escape-hatch rung
   (dependency-free classic script, one global, //go:embed);
   third-party JS only vendored + SHA-256-recorded. A Node-built app
   consuming generated kit artifacts (theme.css, motion.css, L2 scripts)
   is COMPLIANT — never emit a claim against it.
5. Kits-first (D13): a second-app component moves up into appkit/ui/.
6. Divergence discipline (ADR-0017 §2): a deliberate departure from a kit
   component carries a `DIVERGENCE:` header and a row in the kit's
   docs/divergence.md — undocumented divergence is the violation, not
   divergence itself.

Reject: styling opinions without a decision id, claims that would forbid a
non-Go consumer from taking a layer prefix (ADR-0017: stopping at any layer
is complete adoption), and anything already expressed as a pre-edit guard
verbatim (guards and obligations complement, not duplicate: guards carry
the short warning, obligations carry the citable rule).
