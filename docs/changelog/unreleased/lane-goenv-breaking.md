### Breaking

- The go commands fathom runs on its own now run offline (ADR-083), so a module cache that lacks a repository's dependencies leaves the Go typed pass degraded: ingest exits 1 with "ingest degraded" and names the module. Run `go mod download` in each Go module, or set `[ingest] go_network = true` in `~/.fathom/config.toml`; see [the upgrade note](../../upgrades/0.24.0-loopback-offline-go.md).
