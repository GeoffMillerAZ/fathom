# Encrypted token records, version 1

GW-04 supplies a persistence port in `foundations/security/tokenization`, pulled by
throughline's restart-stable token requirement (TRACKING F9). The kit ships a memory
reference store; consumers supply durable adapters. Existing `Vault`, `KeyStore`,
`ShreddableVault`, and `NewEncryptedVault` signatures remain valid.

`NewEncryptedVaultWithStore(tokenizer, keys, policy, clock, records, random)` composes
the caller's stores. `random` supplies nonces and defaults to `crypto/rand`; key
generation has the separate `NewMemoryKeyStoreWithRandom` seam. Deterministic minting
uses the tokenizer's existing HMAC derivation. Preserve that secret HMAC key, encrypted
records, namespace keys and shred tombstones across restart. A fresh key store cannot
recover historical records. Never use seeded randomness in production.

`RecordStore.Get` distinguishes absence from operational failure. `PutIfAbsent` must
validate and atomically bind `Token.Value`, returning the winning record without any
overwrite. A durable implementation must commit before acknowledging success; an
ambiguous write error can be retried deterministically to authenticate the winner.
All byte slices crossing the port are defensive copies. There is no plaintext or key
field in `EncryptedRecord`; audit persistence remains a separate caller responsibility.

Version 1 uses AES-256-GCM with a 32-byte namespace key. `Ciphertext` consists of a
12-byte random nonce, encrypted plaintext and a 16-byte authentication tag. Go JSON
encodes that byte slice in base64; CUE validates decoded bytes. Additional authenticated
data is the following concatenation (decimal integers use ASCII, lengths count bytes):

```
"tokenization-record\0" + version + "\0" +
len(value) + "\0" + value + "\0" +
len(class) + "\0" + class + "\0" +
len(namespace) + "\0" + namespace
```

The full token value, class, namespace and version are authenticated. Unsupported
versions, malformed tokens, invalid key lengths and corrupt ciphertext fail closed.
There is no fallback to unauthenticated legacy ciphertext. Before this port the kit's
encrypted records were private in-memory state; existing durable consumer formats
require an explicit migration by their adapter, not silent reinterpretation.

Random-token collisions fail with `ErrTokenCollision`. Deterministic reuse succeeds
only after identity, current key and ciphertext authentication prove the existing
plaintext matches; successful reuse creates no second audit event. A shredded key,
rotated key or corrupt winning record cannot be overwritten. To mint a new generation
after rotation, choose a new namespace. Deterministic tokens deliberately expose
equality. The encrypted deterministic entry point rejects an empty HMAC key;
consumers must supply a high-entropy secret key for resistance to guessing attacks.

Resolution authorizes the stored class. A caller-provided class or nonempty namespace
must match the stored identity; `Parse`'s empty namespace still resolves. Policy denial
precedes key lookup. Every failed resolution and mint is audited denied. Audit logs
are in-memory, append-only and unbounded for the lifetime of an instance; production
hosts own audit export, retention and instance lifetime. MemoryRecordStore defaults to
4096 records / 64 MiB of ciphertext and metadata, refusing new bindings with
ErrRecordLimit at either bound. NewMemoryRecordStoreWithLimits sets explicit limits;
durable adapters own capacity and report refusal as an error.

Shred persists the namespace tombstone in the key adapter; records remain intact.
Vault operations and shredding serialize within one instance. Shared adapters must be
concurrent-safe and atomic, but KeyStore returns key copies: shredding cannot recall
an in-flight copy in another vault or a backup. A host requiring a global revocation
barrier must coordinate those users and backups. No key is cached by EncryptedVault.

Revisit when: a consumer requires key-generation lookup, a global revocation barrier,
or a second encrypted-record format. None is implemented preemptively.
