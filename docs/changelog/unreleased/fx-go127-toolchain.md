### Fixed
- Typed Go loads no longer fail on a Go 1.27 standard library ("method must have no type parameters" in `math/rand/v2`). fathom is now built with Go 1.27.3 (`toolchain go1.27.3` in go.mod; the `go 1.26.2` minimum for source builders is unchanged), so its type checker matches the `go` it drives. Build fathom with Go 1.27 or newer to load Go 1.27 sources.
- Loaders that read compiled package data (the ask spans and layer-rule type checks) also handle Go 1.27: golang.org/x/tools moves to v0.52.0.
