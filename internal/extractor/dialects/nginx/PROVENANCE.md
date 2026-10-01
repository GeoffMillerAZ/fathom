# nginx — provenance and divergences

LB-Q-nginx, design `docs/design/language-breadth.md` §13 (engine `native`, grammar_source
`native:pure-go`). `internal/extractor/confsyntax.go` is one small lexer shared with the HAProxy and
Caddyfile dialects; for nginx it follows crossplane's token rules (nginxinc/crossplane, Apache-2.0,
the lexer was read, not copied): whitespace separates words, `{ } ;` are words of their own outside
quotes, a quote opens a string only at the start of a word (`\"` unescaped inside it, other escapes
kept as written), `#` starts a comment only at the start of a word (`(?:#.*#)` in a regex location is
a word), `${name}` keeps its braces, and the body of a `*_by_lua_block` directive is one opaque word
(Lua strings, long brackets and `--` comments may hold braces). `internal/extractor/nginxdialect.go`
walks the statement tree into a `DocResult`.

## Grammar considered and not used

opa-oz/tree-sitter-nginx v1.0.1 (commit `47ade644d754cce57974aac44d2c9450e823d4f4`, tarball sha256
`64c563e478c57aebd4d722574300239689eb6e6b5eaacbc07d91c309282dee18`): `indexbench lang-pin -tree`
verdict MIT (basis LICENSE), not AGPL, not refused. Not used: the tags engine cannot emit a route
target or an include's cross-file reference (its L3 is `imports.scm` through `resolvestrategy.go`),
and a routing format targets L3 (owner, 2026-10-02).

## What it models

- **The file** (`NginxConfig`, canonical `package`): named by its base name, line 1 to the last
  line; every file's root entity, what an include of it resolves to.
- **A server block** (`NginxServer`, `service`): named by its first non-empty `server_name`, else its
  first `listen` argument, else `server`; signed by its listen and server_name directives. Two blocks
  of one name (the HTTP-to-HTTPS pair) are two entities on two lines with one symbol key.
- **A location** (`NginxLocation`, `route`): named by its arguments as written, modifier included
  (`/api/`, `~* \.(png|jpg)$`, `= /healthz`, `@fallback`), qualified by its server's name and a
  space (`example.com /api/`); a nested location is qualified by the server, not the outer location;
  a location outside any server (a snippet) is unqualified.
- **An upstream** (`NginxUpstream`, `service`) and **each of its servers** (`NginxUpstreamServer`,
  `target`, `<upstream> <address>`).
- Not entities: the children of `map`, `geo`, `types`, `split_clients`, `charset_map` and `match`
  (data); `if` and `limit_except` blocks (their directives belong to the location around them).

## References

- **L2 (calls)**: a `proxy_pass`, `grpc_pass`, `fastcgi_pass`, `uwsgi_pass`, `scgi_pass` or
  `memcached_pass` whose host, written without a port, names an upstream of the same file, from its
  location (else its server, in `stream`; else the file). With a port nginx does not use the upstream
  block (it resolves the name), so that pass is a route target instead.
- **L3, includes** (`includes`, `repo-path`): nginx resolves a relative include against the
  configuration prefix, the directory of the main nginx.conf, which one file cannot see. A file named
  nginx.conf is the prefix: one candidate in its own directory. Any other file records one candidate
  per directory from its own up to the repository root; `fusion.ResolveDocXrefs` keeps those that are
  files of the repository. An absolute path under `/etc/nginx/`, `/usr/local/nginx/conf/`,
  `/usr/local/etc/nginx/` or `/opt/homebrew/etc/nginx/` is read against the same directories; any
  other absolute path is recorded with an empty target.
- **L3, route targets** (the route contract, ADR-050: `route_ref`, `route-host`, Raw and Target the
  argument as written): a `*_pass` whose host names no upstream of the file, or names one with a port,
  from its location (server, file); each upstream server's address, from the upstream entity (so the
  chain location → upstream → deploy key holds). Variables, unix sockets and IP literals are recorded
  as written; `fusion.ResolveRouteRefs` (LB-F96) resolves or discloses them.

## Classification

`nginx.conf` by basename; `**/sites-enabled/*` and `**/sites-available/*` by glob (Debian's
extensionless vhost files; an Apache vhost there would be read as nginx, and yields only its file
entity: Apache has no `;`); a `.conf` only by content sniff (a block keyword followed by `{`, or a
known nginx directive ending in `;`, in the first KiB), so a supervisord.conf, redis.conf or php-fpm
www.conf is counted ambiguous, never extracted. `conf.d/*.conf` is not a glob claim for the same
reason (supervisord and httpd use conf.d too).

## Limits (LB-Q-nginx-rest)

- A `.conf` whose first KiB is only comments is left ambiguous (12 of h5bp's 38 files).
- An include glob (`conf.d/*.conf`) names no file: `ResolveDocXrefs` matches a glob against
  directories.
- A file the dialect claims by no name is not read: awesome-compose's extensionless `proxy/conf`,
  docker-nginx's envsubst `*.conf.template`.
- An upstream declared in another file of the same configuration is read as a route target (one file
  cannot see it).
- A Go-template or consul-template file (`{{range}}`) is read as nginx and yields what its braces
  happen to delimit.

## Calibration

`calibration/oracles/nginx/` (crossplane 0.5.8, tier C, with `nginx -T` 1.31.6 as the tier-A
cross-check of the included file set). Subjects: `calibration/corpus.d/nginx.yaml`.
