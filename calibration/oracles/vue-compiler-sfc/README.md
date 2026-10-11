# vue-compiler-sfc — Vue single-file components' tier-A oracle (ruling W4)

`vue_sfc_oracle.mjs` reads each `.vue` file of a subject with
[`@vue/compiler-sfc`](https://github.com/vuejs/core) 3.5.43 (MIT), the Vue compiler's own SFC parser,
and writes `indexbench lang-oracle -tool-oracle`'s neutral JSON (design §1.8, "W4 for Vue and Svelte";
wave lf7, item vuesvelte, K19):

- `definitions`: the top-level declarations of each `<script>` and `<script setup>` block as the
  compiler parses them (`compileScript`'s `scriptAst` and `scriptSetupAst`, the trees `bindingMetadata` is
  computed from; the compiler's own `@babel/parser` with its plugins when `compileScript` throws, counted):
  `function`, `class`, `constant`, `variable`, `interface`, `type`, `enum`, and `import` for an import
  binding (no definition of the file; the vue manifest maps no oracle kind to it).
- `calls`: `(path, "<module>", name)` for each template element the compiler types a component
  (`tagType` COMPONENT), named `capitalize(camelize(tag))` as `resolveComponent` resolves it; a dotted tag
  as written. `<component :is>` is counted and left out.
- `call_exclusions`: each name a script block calls that no template component of the file names, reason
  `script-call` (a script's calls are TypeScript's or JavaScript's own L2).
- `imports`: each static import or re-export with a relative specifier, resolved by Vite's rule (the path,
  a TypeScript block's `.js` as its `.ts`, Vite's default `resolve.extensions`, a directory's index);
  alias and package specifiers counted, never resolved.
- `disclose`: the counts a reading names.

Tier A for what the compiler attests (W4).

```sh
npm --prefix <tools> install @vue/compiler-sfc@3.5.43
node calibration/oracles/vue-compiler-sfc/vue_sfc_oracle.mjs -tools <tools> -subject <dir> -out <subject>.vue.json
VUE_TOOLS=<tools> node --test calibration/oracles/vue-compiler-sfc/
```

Run in the measuring harness only, never by fathom.
