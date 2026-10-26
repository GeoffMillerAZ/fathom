# CUE wire counterexamples

From the repository root, these must fail with `cue eval ... -c`:

```sh
cue eval cmd/harnessctl/schema.cue -e '#Key & {id:"fixture",publicKey:"wrong",revoked:false}' -c
cue eval cmd/harnessctl/schema.cue -e '#Signature & {sig:"wrong"}' -c
cue eval cmd/harnessctl/schema.cue -e '_time & "2026-02-30T12:00:00Z"' -c
```

With a generated toy fixture, `cue vet ... -d '#Envelope'` accepts its
standard padded envelope. Re-encoding both `payload` and `sig` as canonical
standard raw, URL padded, or URL raw base64 also passes. A payload with
whitespace or both standard `+` and URL `_` alphabet characters fails. The
Go verifier additionally decodes, checks canonical trailing bits and the
32768-byte payload ceiling, then verifies the signature and payload identity.

Checked on 2026-10-24: all three negative expressions failed; generated
standard padded, standard raw, URL padded and URL raw envelopes passed; bad whitespace,
mixed alphabets and invalid calendar date failed.
