# Open Sentinel parser feasibility

Bench-only isolated Go module: **glennsarti/sentinel-parser** at
`3a328c0dcfb949ba8e9247b577eb1cac14134a1e`, Apache-2.0, pinned by go.mod/go.sum.
The tagged **v0.0.3** (`4fd6abca97e7`) has identical parser sources; the eight
subsequent commits change only dependency manifests and CI workflows.
No dependency is added to the product module.

The probe counts any error diagnostic, terminal error, nil AST, recovered bad
node, walker error or panic as a file in error. All original `.sentinel` files
are read, including mocks; nothing is rewritten. It also reports mock-path and
other-path counts separately so generated files cannot hide policy failures.
The source manifest hashes sorted `relative path + NUL + content SHA256 + newline`.
Published output contains counts/hashes, without subject content or file lists.
Optional `-details` is for private workspace investigation only.

Use Python 3.13+ for safe archive extraction. Put TMPDIR, GOTMPDIR, GOCACHE and
GOMODCACHE in the lane's cache, GOWORK=off. Check `uptime`; each heavy command
starts below load 6 at niceness 10, with no other lane job running.

```sh
python calibration/oracles/sentinel-open/fetch.py --out workspace/calibration/sentinel-open
nice -n 10 go -C calibration/oracles/sentinel-open test ./...
nice -n 10 go -C calibration/oracles/sentinel-open build -o ../../../workspace/tools/sentinel-open .
nice -n 10 python calibration/oracles/sentinel-open/replay.py \
  --subjects workspace/calibration/sentinel-open --probe workspace/tools/sentinel-open \
  --out workspace/tmp/sentinel-open-replay
```

The fetcher pins original public trees, not a canonical corpus shard:
`hashicorp/terraform-sentinel-policies` at `b0bd9902f1db35f922f7d33ff10c35e88c380f0e`
and `hashicorp/policy-library-gcp-databases-terraform` at
`6f779b6ddc3329c967c2bf7a22807f3a16fee9ac`. These are holdouts only.

This checks feasibility, not parser accuracy against an independent truth.
It is not a support reading and proves no level. No Sentinel runtime is installed,
invoked or used by these scripts. OL-R6 forbids publishing benchmark results from
the EULA-bound runtime; its formatter/evaluator/test output is not an oracle here.
