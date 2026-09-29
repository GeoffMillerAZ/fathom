# calibration/corpus.d — one shard per language-breadth queue item

`<id>.yaml`, read by `calibration/clone.sh` alongside `../corpus.yaml` and cloned into
`workspace/calibration/<id>/`. Shape and rules: `../README.md` and
`docs/design/language-breadth.md` §13.3. Delete clones with `task calibrate-clean -- <id>`.

A repo entry may carry `sparse` (a list of gitignore-style patterns) instead of `subpath` when the
files a dialect reads are scattered through a large tree (a tool format's manifests): it is
cloned blob-less and checked out non-cone, so only the matching files land on disk (`go-mod`:
`**/go.mod`, `**/go.sum`, `**/go.work`, `**/go.work.sum`).
