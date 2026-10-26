# Independent Catalog Kit consumer

`run.sh` proves that an external Go module can consume Catalog Kit through its
public imports. It creates a temporary module with an explicit local `replace`
pointing at the checkout under test. It never writes a module file or build scratch
into this checkout. The CUE fixture imports the kit's exported schema and supplies
two layers, a profile, a receiver policy, and a decision identity.

Prerequisites: Go 1.26, CUE 0.16-compatible CLI, and a local agentic-kit checkout.
The example has no third-party Go dependencies. Replay from any directory:

```sh
/absolute/path/to/agentic-kit/examples/catalog-consumer/run.sh
```

The runner sets `GOWORK=off`, `GOPROXY=off`, and `GOSUMDB=off`; set `TMPDIR` to an
existing directory outside the checkout to choose scratch. It validates and exports
the CUE fixture, builds the consumer in the temporary module, executes it, and
checks that changed definition bytes refuse before a prepared packet is emitted.
A successful run prints one deterministic JSON object with the selected stable key,
resolved order, sealed packet digest, and reuse key.

The fixture captures local evidence text and uses a placeholder decision identity.
The resulting key is a binding demonstration, not a provider call, judgment,
evidence authenticity proof, or permission to reuse a judgment. The consumer must
validate its own typed domain definition and evidence scope in production.
