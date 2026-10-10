# react-docgen oracle: React components, props, hooks and their edges (tier C)

`react_oracle.mjs` is the oracle `indexbench lang-react` scores the React kinds of the TypeScript and
JavaScript dialects against (LB-F379, `docs/bench/readings/lang/react.json`). It shares no code with
fathom's extractor and is never imported by fathom.

| Family | Truth | Reads |
|---|---|---|
| L1 definitions | components (`component`, `class_component`) and props (`prop`, named `Owner.prop`) | react-docgen-typescript 2.4.0 (MIT): `Parser.getComponentInfo` decides what a component is, `getPropsInfo` what its props are |
| L2 calls | a JSX element or a hook call, from its enclosing definition to the component or hook | the TypeScript checker resolving the tag or callee |
| L3 file edges, imports | any reference to a component or hook from another file; an import declaration that binds one | the TypeScript checker, `ts.resolveModuleName` |

## Why tier C

docgen's component test is a type heuristic (any function of one parameter, any class whose instances have
`props`), so its population is a heuristic's, and the edges are graded over that population. Tier C caps every
level at good (design §1.3); the reading says so.

## What the program decides, each counted in the output

- **Which declarations are offered**: every module-scope function, class and variable declaration, not only a
  file's exports (docgen's own walk), so an unexported component is truth too; a re-export is followed to the
  declaration, so a component is keyed once, where it is declared.
- **Excluded** (`excluded`): a lower-case name (React reads a lower-case JSX tag as an HTML element); a
  declaration whose call signature returns no React node and that has no render method (`Request`, a fetch
  helper of one parameter); a `createContext` object (React 19's types make it renderable as its provider);
  a prop with no declaration in the subject (a library's inherited props, `theme` and `as` on a styled
  component); a prop with no property or method signature (a mapped type such as cva's `VariantProps`).
- **Added** (`added`): a capitalized function returning a React node that docgen's type test says nothing of:
  one with no parameter (docgen documents one only with a doc comment) and one whose annotation has no call
  signature of its own (`const Home: NextPage = () => …`, a union of a function and a class type). Its props
  are docgen's own reading of its first parameter.
- **Options**: Storybook's (literal enum values, optional props without `undefined`), with every prop kept:
  docgen's table hides an undocumented `children` and Storybook's filter drops a prop whose first declaration
  is a library's, both display choices; the subject test grades each prop at its own declaration instead.
- **Hooks** (`hooks`): in-repo functions named `use[A-Z0-9]` (React's naming rule), the targets of L2 hook
  pairs. docgen grades no hook, so L1 has none.
- **Names** (`names`): each (file, name as written) at which a reference resolves to a component or hook; the
  harness counts a fathom call pair only when its callee is such a name, so an imported `Link` from
  `next/link` is not the subject's own `Link`.

## Run

```sh
T=<scratch>/npm   # npm --prefix $T install react-docgen-typescript@2 typescript@5
# each subject in an oracle checkout with its own dependencies installed, scripts off
(cd <otel-demo>/src/frontend && npm ci --ignore-scripts)
(cd <bulletproof-react>/apps/react-vite && npm install --ignore-scripts --legacy-peer-deps)
(cd <next.js>/examples/<ex> && npm install --ignore-scripts --legacy-peer-deps)   # with-typescript, blog-starter, with-redux
node react_oracle.mjs --tools $T --app <otel-demo>/src/frontend --repo <otel-demo> --out otel.json
node react_oracle.mjs --tools $T --app <bulletproof-react>/apps/react-vite --repo <bulletproof-react> --out bulletproof.json
node react_oracle.mjs --tools $T --app <next.js>/examples/<ex> --repo <next.js> --out next-<ex>.json   # three, merged into one file
indexbench lang-react -subject <otel-demo checkout> -subject-name otel-demo -oracle otel.json \
  -only 'src/frontend/**' -out docs/bench/readings/lang/react.json
```

The measured checkout is a second clone without `node_modules`; `-only` names the app. Run under the wave's
runtime lock; outputs stay out of the repository. `-baseline-kinds` scores the kinds the dialects gave React's
definitions before LB-F377 (a function, a class, no prop), the step-0 reading.
