# haproxy — provenance and divergences

LB-Q-haproxy, design `docs/design/language-breadth.md` §13 (engine `native`, grammar_source
`native:pure-go`). `internal/extractor/confsyntax.go` (the lexer shared with nginx and the
Caddyfile) reads HAProxy's line rules, from the configuration manual's "Quoting and escaping"
section (HAProxy 3.4): a newline ends a statement; an unquoted, unescaped `#` starts a comment
anywhere; double quotes take escapes, single quotes nothing; quotes open and close inside a word
(`"a"b` is one word, `ab`); a backslash escapes the next character; a string never spans lines.
`internal/extractor/haproxydialect.go` reads sections: a line whose first word is a section keyword
(`global`, `defaults`, `frontend`, `backend`, `listen`, `peers`, `resolvers`, `userlist`,
`mailers`, `cache`, `program`, `http-errors`, `ring`, `log-forward`, `crt-store`, `traces`, `acme`,
`fcgi-app`) starts one; every other line belongs to it.

## What it models

- **A frontend, a backend, a listen section** (`HAProxyFrontend`, `HAProxyBackend`,
  `HAProxyListen`, canonical `service`): named by the section's name, from its keyword's line to its
  last statement's.
- **A server or server-template** of a backend or listen section (`HAProxyServer`, `target`,
  `<section> <name>`).
- **An acl line** of a proxy section (`HAProxyACL`, `rule`, `<section> <acl>`): each line is one
  (several lines of one name OR together).
- **A use_backend or default_backend line** of a frontend or listen section (`HAProxyRoute`, `route`,
  `<section> <backend>`, signed by the whole line, its condition included).
- Not modelled: `global`, `defaults` and the non-proxy sections; `.if` blocks are read unevaluated.

## References

- **L2 (calls)**: a route to the backend (or listen section) it names, when the file declares one of
  that name; a name built from a sample expression (`%[req.hdr(host),lower]`) is none.
- **L3, route targets** (the route contract, ADR-050: `route_ref`, `route-host`): each server's
  address as written (`app:8080`, `10.0.0.21:8080`, `unix@/run/x.sock`, `${HOST}:80`,
  `[2001:db8::1]:3000`), from its section, so the chain route → backend → deploy key holds. HAProxy
  has no include: a configuration split across files is several `-f` arguments, so there is no
  file-level reference to resolve.

## Classification

`haproxy.cfg` by basename; a `.cfg` only by content sniff (a `global` or `defaults` line, or a
`frontend`, `backend` or `listen` line with a name, at column 0, in the first KiB), so a setup.cfg is
counted ambiguous, never extracted.

## Limits (LB-Q-haproxy-rest)

- A `.cfg` whose first KiB opens with `.if` and an indented section (haproxy/examples/traces.cfg) is
  left ambiguous.
- `.if`/`.elif` conditions are not evaluated: a server in a disabled block is still an entity.

## Calibration

`calibration/oracles/haproxy/` (`haproxy -c -dr -dC0,line`, HAProxy 3.4.6, tier A: the parsed
configuration with source lines). Subjects: `calibration/corpus.d/haproxy.yaml`.
