# tags_oracle.py — tier-C oracle: an upstream tags.scm on an independent runtime

ADR-048 amendment D5: a language dialect with no SCIP producer (bash)
measures L2 call pairs against its grammar's upstream tags query, run by
py-tree-sitter (the C runtime) — never fathom's own wazero engine. Tier C
caps the level at good.

tree-sitter/tree-sitter-bash v0.25.1 (the lock's pin, a06c2e44) ships no
queries/tags.scm (only highlights.scm), so `bash-tags.scm` is the nearest
licence-allowed upstream: Aider-AI/aider's bash-tags.scm @ ff191aff
(Apache-2.0), verbatim under a provenance header. Disclosure: the bash
dialect's own tags.scm was derived from the same file, so the oracle is
independent in runtime, not in query shape.

    python3 -m venv <tools>/pyts && <tools>/pyts/bin/pip install tree-sitter==0.25.2 tree-sitter-bash==0.25.1
    <tools>/pyts/bin/python tags_oracle.py --lang bash --query bash-tags.scm --ext .sh --ext .bash --shebang bash --shebang sh <clone>/bats-core > out/bash-bats-core.json
    ... --defs ...   # definitions instead of calls (a secondary L1 check)

Lua (LB-Q-lua) runs the grammar's own upstream tags query, `lua-tags.scm` (tree-sitter-grammars/tree-sitter-lua
v0.5.0 `queries/tags.scm`, MIT, verbatim under a header), with py-tree-sitter 0.25.2 and tree-sitter-lua 0.5.0:

    <tools>/pyts/bin/pip install tree-sitter==0.25.2 tree-sitter-lua==0.5.0
    <tools>/pyts/bin/python tags_oracle.py --lang lua --query lua-tags.scm --ext .lua <clone>/kong > out/lua-kong-calls.json
    # Kong/kong 3.9.2 (kong/): 587 files / 10,996 pairs, nvim-telescope/telescope.nvim v0.2.2: 75 / 2,991; 0 parse errors

That query's `@definition.function` and `@definition.method` both enclose calls, and its captures are a plain
`@name` (aider's are `@name.definition.function`); the script reads either. Upstream marks the whole
`table_constructor` as the definition of its function-valued fields, so a call written in a sibling field of a
callback table is paired with the first function field's name: the oracle and the dialect disagree there by design.

Call pairs: each `@reference.call` command name with the innermost
enclosing `@definition.function` (top-level calls skipped; a variable
assignment is not a callable and never encloses). 2026-09-28: bats-core
70 candidate files / 725 pairs, nvm 277 / 1,770 (extensionless shebang
test scripts included; the harness drops files fathom does not read).

Ruby (LB-Q-ruby) runs the grammar's own upstream `queries/tags.scm` and `queries/locals.scm` (tree-sitter/tree-sitter-ruby
v0.23.1, `71bd32fb`, MIT, verbatim under headers as `ruby-tags.scm` and `ruby-locals.scm`) with py-tree-sitter 0.25.2 and
tree-sitter-ruby 0.23.1:

    <tools>/pyts/bin/pip install tree-sitter==0.25.2 tree-sitter-ruby==0.23.1
    <tools>/pyts/bin/python tags_oracle.py --lang ruby --query ruby-tags.scm --locals ruby-locals.scm \
        --ext .rb --ext .rake --ext .ru --basename Rakefile <clone>/sinatra > out/ruby-sinatra-calls.json
    # sinatra/sinatra v4.2.1: 151 files / 3,326 pairs, jekyll/jekyll v4.4.1: 165 / 5,137; 0 parse errors

That query counts every constant and every identifier that is not a local variable as a `@reference.call` (`(#is-not? local)`
is what py-tree-sitter leaves to its caller), so `--locals` applies the locals query the way tree-sitter-tags does: a name that
a `@local.definition` in an enclosing `@local.scope` (or the file's root scope) binds at or before the reference is a variable,
a `(setter (identifier) @ignore)` is dropped, and each name node yields one tag, the first pattern to claim it (a class's own
name is its definition, never also a reference). The script's tag set was checked equal to `tree-sitter tags` 0.26.11's
(`--config-path` naming a directory that holds the grammar checkout) on both subjects: 20,073 and 16,825 tags, none in one and not
the other. The CLI prints only a tag's name span, not the definition's, so it cannot pair a call with its enclosing method; that is
why the pairing runs here. A pair is a call with the innermost enclosing `@definition.method` (`def`, `def self.x`, `alias`);
a call in a class body has no such method and is not a pair. Disclosure: the dialect takes every constant and the method name of
every `call` node but not a bare identifier, so the oracle's non-local identifiers (about 13% of its pairs: `params`, `settings`,
a reader called inside its class) are the dialect's L2 recall misses; the dialect also lists `super` as a call, which upstream's
pattern does not.


JavaScript (LB-F68) runs the grammar's own upstream `queries/tags.scm` (tree-sitter/tree-sitter-javascript v0.25.0,
`44c892e0`, the lock's pin, MIT, verbatim under a header as `javascript-tags.scm`) with py-tree-sitter 0.25.2 and
tree-sitter-javascript 0.25.0:

    <tools>/pyts/bin/pip install tree-sitter==0.25.2 tree-sitter-javascript==0.25.0
    <tools>/pyts/bin/python tags_oracle.py --lang javascript --query javascript-tags.scm \
        --ext .js --ext .jsx --ext .mjs --ext .cjs <clone>/axios > out/javascript-axios-calls.json
    # axios/axios cdcfd214: 165 files / 4,621 pairs (its dist/ bundles included: the harness keeps the 989 pairs on
    # files fathom reads), expressjs/express df0abc93: 152 / 630; 0 parse errors

scip-typescript is no L2 oracle on untyped JavaScript (it resolves few call targets to callable symbols: 305 truth pairs
for 1,385 fathom pairs), so this is the JavaScript calls truth. Its `@definition.function` and `@definition.method` both
enclose calls; a call's callee is the upstream `@name` (an identifier other than `require`, or a member call's property).
Disclosure: the javascript dialect's definition shapes follow this query (class and function declarations, methods,
arrow-function and function-expression bindings, member assignments), so the oracle is independent in runtime, not in
query shape, where they agree. They do not agree on scope: upstream also defines a function-local binding, a named
function expression, an object-literal pair or method and an assignment inside a function, which the dialect leaves
out since LB-Q-javascript-rest (scip-typescript defines none of them as a module or member symbol), so upstream pairs a call written
in one with that inner name and the dialect with the definition around it. `indexbench lang-oracle` takes calls from one
oracle per subject, so this file is scored in a run without `-scip` (`-tool-oracle out/javascript-<subject>-calls.json`),
and the reading carries its numbers in a note beside scip-typescript's L1, L3 and L4.

Scala (LB-Q-scala) runs the grammar's own upstream `queries/tags.scm` (tree-sitter/tree-sitter-scala v0.26.2, `b931fcc3`,
the lock's pin, MIT, verbatim under a header as `scala-tags.scm`) with py-tree-sitter 0.25.2 and tree-sitter-scala 0.26.2,
as a cross-check only:

    <tools>/pyts/bin/pip install tree-sitter==0.25.2 tree-sitter-scala==0.26.2
    <tools>/pyts/bin/python -I tags_oracle.py --lang scala --query scala-tags.scm --ext .scala --ext .sc <clone>/cats > out/scala-cats-calls.json
    # typelevel/cats v2.13.0: 835 files / 4,005 pairs (1 with a parse error), http4s/http4s v0.23.38: 667 / 2,883

Upstream's only call pattern is `(call_expression (identifier) @name)`, a plain `f(x)`: a member call (`xs.map(f)`,
`F.pure(a)`), most of Scala's calls, is in no pair it writes, so against it the dialect reads call_recall .9887 and
call_precision .3896 (scored beside the scalameta definitions, 2026-10-10). The scala reading grades L2 against the
scalameta dump's call pairs instead (`calibration/oracles/scalameta`, tier B, which reads a member call), and records
this run in its notes.
