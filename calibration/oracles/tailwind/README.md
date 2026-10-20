# tailwind_oracle.cjs — the Tailwind CSS v4 oracle of LB-Q-tailwind (tier A for what the compiler attests)

The tailwindcss v4 compiler's own design system (`__unstable__loadDesignSystem`) is the oracle for the Tailwind
layer of the css dialect. For each `.css` file of a subject it compiles the file with its `@import` closure and
writes the neutral oracle file `indexbench lang-oracle -tool-oracle` reads:

- `definitions`: `theme` (each `--name` of an `@theme` block the compiler registered as a theme key), `utility`
  (`@utility name`, or `name-` for `@utility name-*`, confirmed by `utilities.has`), `variant` (`@custom-variant
  name` and the v4.0 `@variant name (selector);`, confirmed by `variants.has`). `end` is 0: the package exports no
  parser and its theme and utility registries carry no end location, so `range_agreement` is not measured and L1
  is capped at good.
- `calls`: `(owner, name)` pairs. An `@apply` candidate the compiler parses (`parseCandidate`) and compiles
  (`candidatesToCss`) to a custom utility of the file's closure names that utility's definition (a variant prefix
  and `!important` allowed); a `var(--name)` whose name the compiler registered as a theme key names that
  variable. Core utilities and default-theme variables are in the compiler's output and in nobody's source: the
  harness's W2 rule (`cmd/indexbench/langoracle_css.go`) leaves a name defined nowhere in the file's closure out of
  both sides, counted.
- `imports`: every `@import`, resolved to a file of the subject by the same resolver the compiler is given.

Names and lines are the source's: the script reads each file with a small scanner (comments, strings, brackets,
braces) to place them, and counts every candidate the compiler rejects, as `calibration/oracles/lightningcss` does.
Owners are the dialect's own rule: a style rule's declarations to its first selector, a custom property's value to
the property, an `@utility` body to the utility, an `@keyframes` rule's blocks to the rule, anything else to the
file's module symbol `<module>`.

Offline. The compiler's loaders are replaced by this script's: a relative `@import` is read from the subject, the
`tailwindcss` package from the install below, anything else is read as an empty sheet; a plugin or config module
loads as one that does nothing. Each is counted in the subject's disclosures. An `@apply` statement becomes an inert
declaration before the compile (each candidate is then asked of the design system on its own), and a file that does
not import `tailwindcss` is compiled with the import prepended, as the entry sheet that imports it does.

Out of scope: utility classes used in templates (markup, JSX): the oxide scanner's candidates are not measured here.

MIT (tailwindcss, @tailwindcss/node, @tailwindcss/oxide), installed into the toolcache, never vendored:

    npm install --prefix <toolcache>/tailwind tailwindcss@4.3.2 @tailwindcss/node@4.3.2 @tailwindcss/oxide@4.3.2
    NODE_PATH=<toolcache>/tailwind/node_modules node calibration/oracles/tailwind/tailwind_oracle.cjs <subject> [dir...] > <out>.json
    indexbench lang-oracle -dialect tailwind -manifest calibration/oracles/tailwind/manifest.json -subject <subject> \
      -subject-name <name> [-only '<dir>/**'] -tool-oracle <out>.json -out docs/bench/readings/lang/tailwind.json

`manifest.json` is the reading's manifest: `tailwind` is a layer of the css dialect (`host`, `host_variant`), as
React is a layer of the TypeScript and JavaScript dialects, so it has no dialect directory of its own. The files
measured are those the variant's sniff accepts.
