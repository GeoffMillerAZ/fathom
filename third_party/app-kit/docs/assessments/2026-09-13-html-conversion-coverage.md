# Official HTML structural inventory — 2026-09-13

Status: **read-only inventory complete; no conversion or compatibility result**.
This supplements the [frozen twelve-case pilot](2026-09-13-conversion-pilot.md) and
[conversion pipeline](../design/conversion-pipeline.md). The denominator is **all 657
HTML members**, including interactive and difficult cases; none were excluded.

## Inputs and integrity

Private catalog revision: `7c7c0ab69b86349f1bb8d1df865f7cf8eb4910542c6fe8340252c6593facaab6`.
Archives remain under ignored `data/design-assets/archives/<sha256>.zip`.

| Alias | Exact archive ID | HTML members | All regular members checked |
|---|---|---:|---:|
| A | `application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8` | 364 | 1,092 |
| E | `ecommerce@79c4900dedc217e4a1086f25009522430bfe3e494fb5097402b9bb16b0549757` | 114 | 342 |
| M | `marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675` | 179 | 537 |
| Total | Three official UI-block archives | **657** | **1,971** |

All three archive SHA-256s matched the catalog. All 1,971 regular members were read
through ZIP CRC verification and matched catalog SHA-256 and byte size. The 657 HTML
members total 7,512,098 bytes across 93 archive/category/subcategory folders.
Identity-set SHA-256: `d57b19cec955cc89c094e304ab7bb6d39ec1188fc88b0d6cb4c99f50ec0580b0`.
This hashes the UTF-8 JSON list of `{archive_id,path,sha256}` records sorted by
`(archive_id,path)`, with sorted object keys, compact separators and ASCII escaping.

An ephemeral standard-library Python ZIP/HTML scan processed source only in memory.
`HTMLParser(convert_charrefs=False)` completed for 657/657 members without exceptions;
raw start-tag spellings were also inspected for SVG case. This permissive parser is
**not** a browser HTML5 tree builder, validator, sanitizer or target compiler.
No extraction, installation, network request, rendering or paid-source output occurred.

## Structural feature counts

Cells count files containing a feature, not occurrences. Rows overlap; A/E/M are the
archives above. The candidate partition is descriptive, not a target-pass numerator.

| Observed feature | A /364 | E /114 | M /179 | Total /657 |
|---|---:|---:|---:|---:|
| Custom `el-*` elements | 102 | 50 | 38 | 190 |
| Actual `<script>` elements, all inline `type=module` | 2 | 0 | 0 | 2 |
| Actual external `<script src>` | 0 | 0 | 0 | 0 |
| Commented Elements loader instructions | 102 | 50 | 38 | 190 |
| Neither actual script nor custom element | 261 | 64 | 141 | 466 |
| Inline `on*` event attributes | 0 | 0 | 0 | 0 |
| Defined directive-prefix attributes | 0 | 0 | 0 | 0 |
| Inline `style` attributes | 5 | 8 | 34 | 47 |
| Actual `<style>` elements | 0 | 0 | 0 | 0 |
| SVG / mixed-case SVG attributes | 228 | 72 | 112 | 412 |
| Mixed-case SVG tags | 0 | 0 | 11 | 11 |
| SVG `foreignObject` | 0 | 0 | 2 | 2 |
| Comments (1,609 comment nodes total) | 170 | 74 | 50 | 294 |
| Named character references | 14 | 48 | 61 | 123 |
| Numeric character references | 1 | 30 | 17 | 48 |
| Literal braces in text | 3 | 0 | 0 | 3 |
| Remote resource attributes / CSS URLs | 106 | 106 | 102 | 314 |
| Remote navigation/action links | 1 | 0 | 0 | 1 |
| Empty or `#` placeholder links | 161 | 89 | 118 | 368 |
| Native `dialog` | 45 | 38 | 29 | 112 |
| Forms | 30 | 52 | 20 | 102 |
| Explicit `dark:` classes | 359 | 0 | 177 | 536 |

Directive prefixes checked: `@`, `:`, `x-`, `v-`, `data-on`, `data-bind`, `data-signals`,
`data-computed`, `data-init`. Zero matches does not imply absence of dynamic behavior.
No `javascript:`, `vbscript:` or `data:` scheme was observed in the checked URL
attributes (`href`, `src`, `action`, `formaction`, `xlink:href`); this is not a safety audit.
Resource counting covers `src`/`srcset`/`poster`, resource-link `href` and style URLs,
not instructional comments; host file counts overlap: `tailwindcss.com` 220,
`images.unsplash.com` 164. Assets were not fetched or pinned.

## Conversion-test requirements and exact edge inputs

The 22 custom tag names include dialog/drawer, disclosure, menu/popover, selection,
command-palette and tab families. File counts include `el-dialog` 112, `el-menu` 62,
`el-disclosure` 40, `el-tab-group`/`el-tab-list`/`el-tab-panels` 33 each, `el-select` 11
and `el-command-palette` 8. The runtime URL in 190 files is **inside HTML comments**:
`https://cdn.jsdelivr.net/npm/@tailwindplus/elements@1` is a mutable major selector,
not an executing or pinned runtime. Preserving these elements alone cannot earn behavior proof.

Preserve SVG namespaces/casing: `viewBox` 411 files, `preserveAspectRatio` 13,
`patternUnits` 11, `gradientUnits` 4, `gradientTransform` 3, `patternTransform` 1;
tags include `clipPath` 2, `foreignObject` 2, `linearGradient` 1 and `radialGradient` 9.
Preserve comments, entities, literal braces, style values, ARIA/command references and
presence semantics: `popover` 115, `hidden` 74, `checked` 57, `required` 33,
`disabled` 11, `selected` 12; empty `data-*` attributes also occur. Do not silently
delete behavior, normalize utility classes or treat 466 script/custom-element-free files as certified static conversions.

| Edge input | Exact archive-relative member | Member SHA-256 |
|---|---|---|
| A: inline module, checkbox table | `html/lists/tables/17-with-checkboxes.html` | `ca78a3891000c4b59846bf2819a0dec65ee1f98cc203f28f27d624372e09745f` |
| A: inline module, command palette | `html/navigation/command-palettes/09-with-footer.html` | `f477b5fd6f92bf34e4cb2bfa6625381dd650152d1cdc2a8a4b1abb2710d49d82` |
| M: SVG/HTML namespace boundary | `html/sections/heroes/08-with-phone-mockup.html` | `a4913be74dff4bffcee425ebd0e47d1f9ce9cb3a4734cbe013033cf2c1994e2d` |
| A: real panel tabs, unlike pilot P06 navigation | `html/forms/textareas/05-with-preview-button.html` | `6009226bd36b9d9184960cb25abbd2fac649e179e6baa4a720b373d7143058bf` |

This assessment establishes byte identity and syntax-feature coverage only: **0/657
conversion results or source-native visual baselines are established here**. Tailwind
patch/config, fonts, runtime closure, responsive fidelity, focus/keyboard behavior,
accessibility, lifecycle and performance remain unverified. Keep originals available;
syntax acceptance, family proof and target compatibility require separate receipts.
