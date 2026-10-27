### Breaking

- The daemon answers loopback peers only, and `serve-http` rebinds a non-loopback `--addr` or `[daemon].addr` (and an installed plist's wide address) to `127.0.0.1`, so a LAN, container or VM client is refused. Run `fathom daemon install --reconcile` from your own terminal; see [the upgrade note](../../upgrades/0.24.0-loopback-offline-go.md).
