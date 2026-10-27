### Security

- The daemon refuses a request whose peer address is not loopback with 403 and the named reason, so a network peer can no longer reach `/rpc` by sending `Host: 127.0.0.1`.
- `fathom daemon install` writes `--addr 127.0.0.1:7575` and refuses a non-loopback `--addr` (exit 2), since remote serving needs token auth, which fathom does not have. The new doctor row `daemon_bind` FAILs on a wide plist or a running non-loopback listener and WARNs on a wide `[daemon].addr`.
