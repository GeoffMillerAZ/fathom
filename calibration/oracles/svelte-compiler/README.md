# svelte-compiler — Svelte components' tier-A oracle (ruling W4)

`svelte_oracle.mjs` reads each `.svelte` file of a subject with `svelte/compiler`'s `parse`
([svelte](https://github.com/sveltejs/svelte) 5.57.2, MIT; the modern AST), the Svelte compiler's own
parser, and writes `indexbench lang-oracle -tool-oracle`'s neutral JSON by the rules
`../vue-compiler-sfc/README.md` gives for Vue (design §1.8, "W4 for Vue and Svelte"; wave lf7, item
vuesvelte, K19), whose helpers it imports:

- `definitions`: the top-level declarations of the instance and module scripts (`ast.instance.content`,
  `ast.module.content`, the compiler's ESTree programs, TypeScript read by its acorn plugin).
- `calls`: `(path, "<module>", name)` for each `Component` node of the template (a capitalized or dotted
  tag, its name as written; the pair compares its last segment, M2). `<svelte:component>` and
  `<svelte:self>` are counted and left out.
- `call_exclusions`, `imports`, `disclose`: as the Vue oracle's. A file the compiler refuses to parse has
  no truth: counted and named.

```sh
npm --prefix <tools> install svelte@5.57.2
node calibration/oracles/svelte-compiler/svelte_oracle.mjs -tools <tools> -subject <dir> -out <subject>.svelte.json
SVELTE_TOOLS=<tools> node --test calibration/oracles/svelte-compiler/
```

Run in the measuring harness only, never by fathom.
