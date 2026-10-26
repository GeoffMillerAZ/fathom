# Grammar source patches

`grammars.lock.json` lists each grammar's `patches` in application order as
`{"path":"patches/name.patch","sha256":"<64 lowercase hex>"}`. Paths are
relative to `grammars/`. The build verifies each pinned upstream tarball and
licence, then verifies and applies only the declared patches before optional
generation, and checks the licence again. No directory scan adds patches.

Create unified Git patches against the unpacked upstream grammar root, with
paths such as `a/src/parser.c` and `b/src/parser.c`. `git apply --check` must
succeed from that root. Reversed or context-mismatched patches fail the build.
MPL-2.0 grammar rows cannot declare patches. Patch files are build-time
inputs; they are not embedded in the runtime binary.
