### Security

- The daemon refuses a request whose peer address is not loopback with 403 and the named reason, so a network peer can no longer reach `/rpc` by sending `Host: 127.0.0.1`. A container or VM that reached the host's daemon through a non-loopback address is now refused.
