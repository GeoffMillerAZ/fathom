### Fixed

- A route in a configuration file (a Traefik, Envoy or Caddy upstream, any route-host reference) now joins the compose or Kubernetes service of its nearest ancestor directory before falling back to every same-named service in the repository, so a config beside a root compose file no longer links to unrelated overlay stacks.
- Traefik: a router's `service: x@file` and `middlewares: [y@file]` (the file provider's suffix) now name the `x` and `y` declared in the file.
