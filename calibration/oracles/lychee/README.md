# lychee — HTML's tier-A local link targets (L3)

`lychee_oracle.mjs` converts a [lychee](https://github.com/lycheeverse/lychee)
0.24.2 (MIT OR Apache-2.0; the release binary
`lychee-aarch64-apple-darwin.tar.gz`, sha256
`c9d3740ea2d891854d37116c9fba840f37b6e7c89d330e7db84ac333631c4977`) `--offline`
report into lang-oracle's neutral `imports` (wave lf5, item html): each local
link lychee resolved (`success_map`) is `(path, line, to_path)`, the line its
own span gives, `to_path` the subject file its `file://` URL names with the
query and fragment dropped; a directory link counts as lychee was told to
accept it (`--index-files index.html,index.htm`) and names the first index file
in that order. Tier A only for what it attests (design §1.3): local file
targets. A link it could not resolve, one outside the subject and a remote one
name no file and are counted in `disclose`, never truth. lang-oracle scores the
pairs whose target is a measured file as `import_resolution` and, with no SCIP
index, as the file-edge truth (LB-F57).

```sh
node calibration/oracles/lychee/lychee_oracle.mjs -list <subject> > <list>
(cd <subject> && lychee --offline --no-progress -v --format json \
   --root-dir "$(pwd)" --index-files index.html,index.htm \
   --files-from <list> > <lychee.json>)
node calibration/oracles/lychee/lychee_oracle.mjs -subject <subject> -lychee <lychee.json> -version 0.24.2 -out <subject>.lychee.json
node --test calibration/oracles/lychee/
```

Run in the measuring harness only, never by fathom.
