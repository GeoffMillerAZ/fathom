# caddyfile — provenance and divergences

LB-Q-caddyfile, design `docs/design/language-breadth.md` §13 (engine `native`, grammar_source
`native:pure-go`). `internal/extractor/confsyntax.go` (the lexer shared with nginx and HAProxy)
reads the Caddyfile's lexical rules from Caddy's documentation ("Caddyfile concepts": tokens, quotes,
heredocs) and its lexer's behaviour (caddyserver/caddy v2.11.6 `caddyconfig/caddyfile/lexer.go`,
Apache-2.0, read, not copied): whitespace separates words; a newline ends a statement unless a
backslash precedes it; `"..."` unescapes `\"` and an escaped newline, `` `...` `` is raw; `#` starts a
comment only at the start of a word; a `<<MARKER` heredoc is one word, its closing marker's
indentation removed from every line, and words after the closing marker continue the statement;
braces are block delimiters only as words of their own, so `{host}`, `{$ENV}` and `{args[0]}` are
ordinary words. The parser groups statements into blocks: a statement whose last word is an unquoted
`{` opens one, a lone unquoted `}` closes it. `internal/extractor/caddyfiledialect.go` walks them.

## What it models

- **The file** (`CaddyfileConfig`, canonical `package`): its base name, line 1 to the last line; its
  root entity, what an import of it resolves to.
- **A site block** (`CaddySite`, `service`): named by its first address as written (`example.com`,
  `:8080`, `{$APP_DOMAIN}`: placeholders are not substituted), signed by all its addresses; a file
  whose first statement (after the global options block) opens no block is one site to its end.
- **A snippet** `(name) { … }` (`CaddySnippet`, `mixin`) and **a named route** `&(name) { … }`
  (`CaddyNamedRoute`, `route`), named by the name inside the parentheses.
- **A named matcher** `@name` (`CaddyMatcher`, `selector`), block or one line, qualified by its site
  (or snippet) and a space.
- **A route** (`CaddyRoute`, `route`): a `handle`, `handle_path`, `handle_errors` or `route` block,
  and a `reverse_proxy`, `php_fastcgi` or `forward_auth` directive, named by the directive and its
  matcher token (`*`, `/path`, `@name`) when it has one, qualified by its site.
- Not modelled: the global options block; other directives.

## References

- **L2 (calls)**: an `import` of a snippet the file declares and an `invoke` of a named route it
  declares, from the enclosing entity (route, site, snippet; the file at the top).
- **L3, imports** (`includes`, `repo-path`): an import that names no snippet is a file or a glob,
  relative to the importing file's directory (Caddy's rule); an absolute path under `/etc/caddy/` is
  read against that directory, any other absolute path recorded with an empty target.
- **L3, route targets** (the route contract, ADR-050: `route_ref`, `route-host`, Raw and Target as
  written): each upstream of a `reverse_proxy`, `php_fastcgi` or `forward_auth` (its arguments after
  the matcher, and its `to` subdirective's), from the route entity.

## Classification

`Caddyfile` by basename and `.caddyfile` by extension (both also fusion fast-path keys);
`**/Caddyfile.*` by glob (`Caddyfile.dev`, `Caddyfile.prose`).

## Limits (LB-Q-caddyfile-rest)

- An import glob names no file: `ResolveDocXrefs` matches a glob against directories.
- `{$ENV}` placeholders are kept as written, in site names and upstreams (the resolver discloses an
  upstream as `variable`).
- `dynamic` upstream modules (SRV, A lookups) name no static target and are not recorded.

## Calibration

`calibration/oracles/caddyfile/` (Caddy v2.11.6: `caddy adapt` acceptance, the reference lexer and
parser, tier A). Subjects: `calibration/corpus.d/caddyfile.yaml`.
