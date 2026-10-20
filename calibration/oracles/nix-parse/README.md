# nix-parse -- the Nix dialect's oracle (LB-Q-nix, tier C)

The reference parser is the oracle (design §18.3: an oracle independent of the extractor; never fathom's code
path, never its tree-sitter grammar). `nix-instantiate --parse` runs as a separate process per file and prints
the parsed expression back as Nix text, fully parenthesised, attribute sets merged and sorted, every path made
absolute. This program reads that text back (`ast.go`) and writes `indexbench lang-oracle`'s tool-oracle JSON:

```
go build -o <scratch>/nix-parse .
<scratch>/nix-parse -subject <checkout> -files-out <files.txt> [-flake] > workspace/tmp/oracle/nix-<subject>.json
indexbench lang-oracle -dialect nix -subject <checkout> -ctags /nonexistent \
  -tool-oracle workspace/tmp/oracle/nix-<subject>.json -out docs/bench/readings/lang/nix.json
```

Read-only use of the host's nix (2.34): `--parse` evaluates nothing, builds nothing and fetches nothing; `-flake`
adds `nix flake show --json --offline --no-write-lock-file` on the subject root (an input already in the store at
the lock's hash needs no fetch). `NIX_CONFIG` keeps substituters off.

| Family | Source | What it gives |
|---|---|---|
| Gate | `nix-instantiate --parse <file>` | a file the parser refuses (a syntax error, or an unbound variable, which `--parse` also resolves) is out of every family and counted |
| Definitions | `lex.go` and `src.go` for names and lines; the parse attests each name | the root expression's own names (design: dialects/nix/tags.scm header): the file's expression peeled through at most three wrappers (function, let, with), then its let bindings, its attribute set's keys at depth one (a dotted key is its first attribute) and its functions' formals; `kind` from the parse: function (a lambda value), option (the key `options`, or an application of mkOption / mkEnableOption / mkPackageOption), package (an application of callPackage / mkDerivation / a build* helper), else attribute (a set key) or variable (a let binding), and param |
| References | the parse | the head of every application inside a definition's value, by bare name (`lib.mkIf` is `mkIf`); the printer's `__sub`, `__mul` and `__findFile` (an operator or a search path) are no call |
| Imports | the lexer's path literals, attested against the parse's absolute paths | every `./` or `../` literal that is plain text and names a `.nix` file of the subject or a directory holding `default.nix`; a flake input `url = "./dir"` (and `"path:./dir"`) naming a directory holding `flake.nix` |
| Cross references | the parse | a name selected from an imported file and applied, in the scope Nix reads: `u = import ./u.nix; ... u.f 1`, `(import ./u.nix).f 1`, `inherit (import ./u.nix) f;` then `f 1`; the imported file's root set must define the name; a let binding or function argument of the same name shadows |

Limits, disclosed in the reading: the reference parse carries no positions, so a definition's line is located by
the lexer and the parse only attests that the name exists (a name the parse does not list is dropped and counted,
`sites_unattested`); the printed text loses whether the author wrote a pair of parentheses, so a value written
`(mkOption { })` reads as an option here and as an attribute to the grammar; the cross-reference truth is the
`import`-and-select shape, the one Nix scopes statically (a module-system `imports` list or a `callPackage` path
brings no name into scope, and neither is a reference); `with import ./x.nix;` and a dynamic path are not read.
