# Encrypted token record upgrade

Existing constructor/interface signatures remain compatible. The stable CUE contract
gains versioned encrypted records; validate the updated schema with its fixtures.

- Both vaults now authorize stored identity, rejecting a caller-supplied class or
  nonempty namespace that does not match. `Parse`'s empty namespace remains supported.
- `NewEncryptedVault` uses a bounded in-memory record store (4096 records, 64 MiB
  of ciphertext/metadata); new bindings at capacity fail with `ErrRecordLimit`.
  Use `NewEncryptedVaultWithStore` and explicit reference-store limits or a durable
  adapter for larger workloads. Never turn a failed insert into successful minting.
- Use `TokenizeDeterministic` with a secret nonempty HMAC key for equality joins.
  Persist that key, namespace keys, shred tombstones and encrypted records. A fresh
  key store cannot recover old records; rotating a key requires a new namespace.
- `EncryptedRecord` v1 authenticates metadata as AAD. Existing consumer ciphertext
  formats must stay in their legacy reader or be explicitly re-encrypted while their
  original keys remain available. Do not reinterpret legacy ciphertext as v1.
- Durable `RecordStore.PutIfAbsent` must commit atomically before acknowledging,
  return the winning record, preserve collisions, and enforce capacity/retention.
  Keys, record storage and audit persistence remain separate consumer responsibilities.

Details and revocation limits: [record contract](../designs/token-records.md).
