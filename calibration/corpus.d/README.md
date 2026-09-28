# calibration/corpus.d — one shard per language-breadth queue item

`<id>.yaml`, read by `calibration/clone.sh` alongside `../corpus.yaml` and cloned into
`workspace/calibration/<id>/`. Shape and rules: `../README.md` and
`docs/design/language-breadth.md` §13.3. Delete clones with `task calibrate-clean -- <id>`.
