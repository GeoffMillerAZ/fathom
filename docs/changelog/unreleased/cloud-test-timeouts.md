### Fixed

- In a Claude Code cloud session, the pre-push gate gets CI's test budget (`GOFLAGS=-timeout=30m`, `FATHOM_TEST_SHARDS_TIMEOUT=30m0s`), so `cmd/fathom`'s race shards no longer fail at go's default 10-minute timeout on the VM's four vCPUs. A timeout you set yourself is kept.
